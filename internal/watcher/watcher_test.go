package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"dense-rag/internal/config"
)

func testConfig(watchDir string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.WatchDir = watchDir
	return cfg
}

func TestDebouncerCoalesces(t *testing.T) {
	var mu sync.Mutex
	calls := make(map[string]int)

	d := NewDebouncer(50 * time.Millisecond)
	defer d.Stop()

	cb := func(key string) {
		mu.Lock()
		calls[key]++
		mu.Unlock()
	}

	// Trigger same key 5 times rapidly — should coalesce to 1 call.
	for i := 0; i < 5; i++ {
		d.Debounce("fileA", func() { cb("fileA") })
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce window to fire.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := calls["fileA"]
	mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 debounced call, got %d", count)
	}
}

func TestDebouncerDifferentKeys(t *testing.T) {
	var mu sync.Mutex
	calls := make(map[string]int)

	d := NewDebouncer(50 * time.Millisecond)
	defer d.Stop()

	d.Debounce("a", func() {
		mu.Lock()
		calls["a"]++
		mu.Unlock()
	})
	d.Debounce("b", func() {
		mu.Lock()
		calls["b"]++
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if calls["a"] != 1 || calls["b"] != 1 {
		t.Errorf("expected 1 call each, got a=%d b=%d", calls["a"], calls["b"])
	}
}

func TestDebouncerStop(t *testing.T) {
	called := false
	d := NewDebouncer(50 * time.Millisecond)
	d.Debounce("x", func() { called = true })
	d.Stop()
	time.Sleep(100 * time.Millisecond)
	if called {
		t.Error("callback should not fire after Stop")
	}
}

func TestPoolConcurrency(t *testing.T) {
	p := NewPool(2)

	var mu sync.Mutex
	running := 0
	maxRunning := 0
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		p.Submit(func() {
			defer wg.Done()
			mu.Lock()
			running++
			if running > maxRunning {
				maxRunning = running
			}
			mu.Unlock()

			time.Sleep(20 * time.Millisecond)

			mu.Lock()
			running--
			mu.Unlock()
		})
	}

	wg.Wait()
	p.Shutdown()

	if maxRunning > 2 {
		t.Errorf("expected max 2 concurrent workers, observed %d", maxRunning)
	}
}

func TestPoolShutdown(t *testing.T) {
	p := NewPool(2)
	done := make(chan struct{})

	p.Submit(func() {
		time.Sleep(50 * time.Millisecond)
		close(done)
	})

	p.Shutdown()

	select {
	case <-done:
		// ok — work completed before shutdown returned
	default:
		t.Error("Shutdown returned before work completed")
	}
}

func TestWatcherCreateModify(t *testing.T) {
	tmpDir := t.TempDir()

	var mu sync.Mutex
	events := make(map[string]EventOp)

	processFn := func(path string, op EventOp) {
		mu.Lock()
		events[path] = op
		mu.Unlock()
	}

	w, err := NewWatcher(testConfig(tmpDir), processFn)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	defer func() {
		cancel()
		w.debouncer.Stop()
		w.fsw.Close()
		w.pool.Shutdown()
	}()

	// Give the watcher time to set up.
	time.Sleep(100 * time.Millisecond)

	// Create a .txt file.
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait for debounce + processing.
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	op, found := events[testFile]
	mu.Unlock()

	if !found {
		t.Error("expected create event for test.txt")
	} else if op != OpCreateModify {
		t.Errorf("expected OpCreateModify, got %v", op)
	}
}

func TestWatcherDelete(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create a file before the watcher starts.
	testFile := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var mu sync.Mutex
	events := make(map[string]EventOp)

	processFn := func(path string, op EventOp) {
		mu.Lock()
		events[path] = op
		mu.Unlock()
	}

	w, err := NewWatcher(testConfig(tmpDir), processFn)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	defer func() {
		cancel()
		w.debouncer.Stop()
		w.fsw.Close()
		w.pool.Shutdown()
	}()

	time.Sleep(100 * time.Millisecond)

	// Delete the file.
	os.Remove(testFile)

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	op, found := events[testFile]
	mu.Unlock()

	if !found {
		t.Error("expected delete event for existing.txt")
	} else if op != OpDelete {
		t.Errorf("expected OpDelete, got %v", op)
	}
}

func TestWatcherIgnoresUnsupported(t *testing.T) {
	tmpDir := t.TempDir()

	var mu sync.Mutex
	events := make(map[string]EventOp)

	processFn := func(path string, op EventOp) {
		mu.Lock()
		events[path] = op
		mu.Unlock()
	}

	w, err := NewWatcher(testConfig(tmpDir), processFn)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	defer func() {
		cancel()
		w.debouncer.Stop()
		w.fsw.Close()
		w.pool.Shutdown()
	}()

	time.Sleep(100 * time.Millisecond)

	// Create a .go file (unsupported).
	unsupported := filepath.Join(tmpDir, "main.go")
	os.WriteFile(unsupported, []byte("package main"), 0644)

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	_, found := events[unsupported]
	mu.Unlock()

	if found {
		t.Error("should not have received event for unsupported file type")
	}
}

func TestWatcherNewSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()

	var mu sync.Mutex
	events := make(map[string]EventOp)

	processFn := func(path string, op EventOp) {
		mu.Lock()
		events[path] = op
		mu.Unlock()
	}

	w, err := NewWatcher(testConfig(tmpDir), processFn)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go w.Start(ctx)
	defer func() {
		cancel()
		w.debouncer.Stop()
		w.fsw.Close()
		w.pool.Shutdown()
	}()

	time.Sleep(100 * time.Millisecond)

	// Create a subdirectory and a file inside it.
	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	time.Sleep(200 * time.Millisecond) // let watcher register the new dir

	subFile := filepath.Join(subDir, "nested.txt")
	os.WriteFile(subFile, []byte("nested content"), 0644)

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	_, found := events[subFile]
	mu.Unlock()

	if !found {
		t.Error("expected event for file created in new subdirectory")
	}
}
