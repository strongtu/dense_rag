package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPollWatcher_Scan(t *testing.T) {
	dir := t.TempDir()

	// Create test files.
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "b.docx"), []byte("world"), 0644)
	os.WriteFile(filepath.Join(dir, "c.pdf"), []byte("skip"), 0644)
	os.WriteFile(filepath.Join(dir, "~$temp.docx"), []byte("temp"), 0644)

	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0755)
	os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("nested"), 0644)

	pw := NewPollWatcher(dir, func(string, EventOp) {}, nil)
	snap := pw.scan()

	// Should include a.txt, b.docx, nested.txt but NOT c.pdf or ~$temp.docx.
	if len(snap) != 3 {
		t.Errorf("expected 3 files in snapshot, got %d", len(snap))
		for k := range snap {
			t.Logf("  %s", k)
		}
	}

	if _, ok := snap[filepath.Join(dir, "a.txt")]; !ok {
		t.Error("missing a.txt in snapshot")
	}
	if _, ok := snap[filepath.Join(dir, "b.docx")]; !ok {
		t.Error("missing b.docx in snapshot")
	}
	if _, ok := snap[filepath.Join(sub, "nested.txt")]; !ok {
		t.Error("missing sub/nested.txt in snapshot")
	}
	if _, ok := snap[filepath.Join(dir, "c.pdf")]; ok {
		t.Error("c.pdf should not be in snapshot")
	}
	if _, ok := snap[filepath.Join(dir, "~$temp.docx")]; ok {
		t.Error("~$temp.docx should not be in snapshot")
	}
}

func TestPollWatcher_Diff_Added(t *testing.T) {
	pw := NewPollWatcher("/tmp", func(string, EventOp) {}, nil)
	pw.snapshot = map[string]fileState{
		"/tmp/a.txt": {ModTime: 100, Size: 10},
	}

	current := map[string]fileState{
		"/tmp/a.txt": {ModTime: 100, Size: 10},
		"/tmp/b.txt": {ModTime: 200, Size: 20},
	}

	events := pw.diff(current)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].path != "/tmp/b.txt" || events[0].op != OpCreateModify {
		t.Errorf("expected add event for b.txt, got %+v", events[0])
	}
}

func TestPollWatcher_Diff_Modified(t *testing.T) {
	pw := NewPollWatcher("/tmp", func(string, EventOp) {}, nil)
	pw.snapshot = map[string]fileState{
		"/tmp/a.txt": {ModTime: 100, Size: 10},
	}

	current := map[string]fileState{
		"/tmp/a.txt": {ModTime: 200, Size: 15},
	}

	events := pw.diff(current)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].path != "/tmp/a.txt" || events[0].op != OpCreateModify {
		t.Errorf("expected modify event for a.txt, got %+v", events[0])
	}
}

func TestPollWatcher_Diff_Deleted(t *testing.T) {
	pw := NewPollWatcher("/tmp", func(string, EventOp) {}, nil)
	pw.snapshot = map[string]fileState{
		"/tmp/a.txt": {ModTime: 100, Size: 10},
		"/tmp/b.txt": {ModTime: 200, Size: 20},
	}

	current := map[string]fileState{
		"/tmp/a.txt": {ModTime: 100, Size: 10},
	}

	events := pw.diff(current)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].path != "/tmp/b.txt" || events[0].op != OpDelete {
		t.Errorf("expected delete event for b.txt, got %+v", events[0])
	}
}

func TestPollWatcher_Diff_NoChanges(t *testing.T) {
	pw := NewPollWatcher("/tmp", func(string, EventOp) {}, nil)
	pw.snapshot = map[string]fileState{
		"/tmp/a.txt": {ModTime: 100, Size: 10},
	}

	current := map[string]fileState{
		"/tmp/a.txt": {ModTime: 100, Size: 10},
	}

	events := pw.diff(current)
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestPollWatcher_Lifecycle(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	events := make(map[string]EventOp)

	pw := NewPollWatcher(dir, func(path string, op EventOp) {
		mu.Lock()
		events[path] = op
		mu.Unlock()
	}, nil)
	// Use a short interval for testing.
	pw.interval = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	go pw.Start(ctx)

	// Create a file and wait for a poll cycle.
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	op, found := events[testFile]
	mu.Unlock()

	if !found {
		t.Error("expected event for test.txt")
	} else if op != OpCreateModify {
		t.Errorf("expected OpCreateModify, got %v", op)
	}

	// Stop the watcher.
	cancel()
	pw.debouncer.Stop()
	pw.pool.Shutdown()
}

func TestPollWatcher_InitialSnapshot(t *testing.T) {
	dir := t.TempDir()

	// Create a file before the watcher starts.
	existing := filepath.Join(dir, "existing.txt")
	os.WriteFile(existing, []byte("data"), 0644)

	// Build snapshot to simulate reconciliation.
	snap := BuildSnapshot(dir)

	var mu sync.Mutex
	events := make(map[string]EventOp)

	pw := NewPollWatcher(dir, func(path string, op EventOp) {
		mu.Lock()
		events[path] = op
		mu.Unlock()
	}, snap)
	pw.interval = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	go pw.Start(ctx)

	// Wait for a poll cycle — existing file should NOT trigger an event.
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	_, found := events[existing]
	mu.Unlock()

	if found {
		t.Error("existing file should not trigger event when initialized with snapshot")
	}

	cancel()
	pw.debouncer.Stop()
	pw.pool.Shutdown()
}

func TestBuildSnapshot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "b.pdf"), []byte("skip"), 0644)

	snap := BuildSnapshot(dir)
	if len(snap) != 1 {
		t.Errorf("expected 1 entry in snapshot, got %d", len(snap))
	}
	if _, ok := snap[filepath.Join(dir, "a.txt")]; !ok {
		t.Error("missing a.txt in snapshot")
	}
}
