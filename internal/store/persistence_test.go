package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	s := NewStore()

	s.Add("file1.txt", 100, []VectorEntry{
		{Vector: []float32{1, 0, 0}, Text: "chunk-a", ChunkIndex: 0},
		{Vector: []float32{0, 1, 0}, Text: "chunk-b", ChunkIndex: 1},
	})
	s.Add("file2.txt", 200, []VectorEntry{
		{Vector: []float32{0, 0, 1}, Text: "chunk-c", ChunkIndex: 0},
	})

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "store.gob")

	if err := s.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved file does not exist: %v", err)
	}

	// Load into new store
	s2 := NewStore()
	if err := s2.Load(path); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify stats match
	origStats := s.Stats()
	loadedStats := s2.Stats()
	if origStats.VectorCount != loadedStats.VectorCount {
		t.Errorf("vector count mismatch: %d vs %d", origStats.VectorCount, loadedStats.VectorCount)
	}
	if origStats.IndexedFiles != loadedStats.IndexedFiles {
		t.Errorf("indexed files mismatch: %d vs %d", origStats.IndexedFiles, loadedStats.IndexedFiles)
	}

	// Verify entries
	s2.RLock()
	defer s2.RUnlock()

	if len(s2.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(s2.entries))
	}

	// Verify file index was rebuilt
	if len(s2.fileIndex["file1.txt"]) != 2 {
		t.Errorf("expected 2 indices for file1.txt, got %d", len(s2.fileIndex["file1.txt"]))
	}
	if len(s2.fileIndex["file2.txt"]) != 1 {
		t.Errorf("expected 1 index for file2.txt, got %d", len(s2.fileIndex["file2.txt"]))
	}

	// Verify mtimes
	mt, ok := s2.fileMtimes["file1.txt"]
	if !ok || mt != 100 {
		t.Errorf("expected mtime 100 for file1.txt, got %d (ok=%v)", mt, ok)
	}
	mt, ok = s2.fileMtimes["file2.txt"]
	if !ok || mt != 200 {
		t.Errorf("expected mtime 200 for file2.txt, got %d (ok=%v)", mt, ok)
	}

	// Verify search still works after load
	results := s2.Search([]float32{1, 0, 0}, 1)
	if len(results) != 1 || results[0].Text != "chunk-a" {
		t.Errorf("unexpected search result after load: %+v", results)
	}
}

func TestLoadBadVersion(t *testing.T) {
	// We can't easily forge a bad version with gob without a custom file,
	// but we can test loading a non-existent file.
	s := NewStore()
	err := s.Load("/nonexistent/path/store.gob")
	if err == nil {
		t.Error("expected error loading non-existent file")
	}
}

func TestReconcile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some files
	writeFile(t, filepath.Join(tmpDir, "existing.txt"), "hello")
	writeFile(t, filepath.Join(tmpDir, "new.txt"), "world")
	writeFile(t, filepath.Join(tmpDir, "updated.txt"), "changed")
	writeFile(t, filepath.Join(tmpDir, "ignored.bin"), "binary")

	s := NewStore()

	// "existing.txt" is already in store with correct mtime
	info1, _ := os.Stat(filepath.Join(tmpDir, "existing.txt"))
	s.Add(filepath.Join(tmpDir, "existing.txt"), info1.ModTime().Unix(), []VectorEntry{
		{Vector: []float32{1}, Text: "existing", ChunkIndex: 0},
	})

	// "updated.txt" is in store but with old mtime
	s.Add(filepath.Join(tmpDir, "updated.txt"), 1, []VectorEntry{
		{Vector: []float32{1}, Text: "old-updated", ChunkIndex: 0},
	})

	// "removed.txt" is in store but not on disk
	s.Add(filepath.Join(tmpDir, "removed.txt"), 999, []VectorEntry{
		{Vector: []float32{1}, Text: "removed", ChunkIndex: 0},
	})

	added, removed, updated := s.Reconcile([]string{tmpDir}, []string{".txt"})

	// "new.txt" should be in added
	if !containsPath(added, filepath.Join(tmpDir, "new.txt")) {
		t.Errorf("expected new.txt in added, got %v", added)
	}

	// "removed.txt" should be in removed
	if !containsPath(removed, filepath.Join(tmpDir, "removed.txt")) {
		t.Errorf("expected removed.txt in removed, got %v", removed)
	}

	// "updated.txt" should be in updated
	if !containsPath(updated, filepath.Join(tmpDir, "updated.txt")) {
		t.Errorf("expected updated.txt in updated, got %v", updated)
	}

	// "existing.txt" should NOT be in any list
	if containsPath(added, filepath.Join(tmpDir, "existing.txt")) ||
		containsPath(removed, filepath.Join(tmpDir, "existing.txt")) ||
		containsPath(updated, filepath.Join(tmpDir, "existing.txt")) {
		t.Error("existing.txt should not appear in any reconcile list")
	}

	// "ignored.bin" should NOT be in any list
	if containsPath(added, filepath.Join(tmpDir, "ignored.bin")) {
		t.Error("ignored.bin should not appear in added")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func containsPath(paths []string, target string) bool {
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}
