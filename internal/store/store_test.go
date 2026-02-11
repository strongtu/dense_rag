package store

import (
	"math"
	"testing"
)

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float32{1, 2, 3}
	score := CosineSimilarity(a, a)
	if math.Abs(float64(score)-1.0) > 1e-6 {
		t.Errorf("expected ~1.0 for identical vectors, got %f", score)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	score := CosineSimilarity(a, b)
	if math.Abs(float64(score)) > 1e-6 {
		t.Errorf("expected ~0.0 for orthogonal vectors, got %f", score)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	score := CosineSimilarity(a, b)
	if math.Abs(float64(score)+1.0) > 1e-6 {
		t.Errorf("expected ~-1.0 for opposite vectors, got %f", score)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	score := CosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("expected 0 for zero vector, got %f", score)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	score := CosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("expected 0 for different-length vectors, got %f", score)
	}
}

func TestAddAndSearch(t *testing.T) {
	s := NewStore()

	entries := []VectorEntry{
		{Vector: []float32{1, 0, 0}, Text: "chunk-a", ChunkIndex: 0},
		{Vector: []float32{0, 1, 0}, Text: "chunk-b", ChunkIndex: 1},
		{Vector: []float32{0, 0, 1}, Text: "chunk-c", ChunkIndex: 2},
	}
	s.Add("file1.txt", 100, entries)

	// Query closest to chunk-a
	results := s.Search([]float32{0.9, 0.1, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Text != "chunk-a" {
		t.Errorf("expected top result chunk-a, got %s", results[0].Text)
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("expected results sorted descending by score")
	}
}

func TestSearch_TopKLargerThanEntries(t *testing.T) {
	s := NewStore()
	s.Add("f.txt", 1, []VectorEntry{
		{Vector: []float32{1, 0}, Text: "only", ChunkIndex: 0},
	})
	results := s.Search([]float32{1, 0}, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestSearch_EmptyStore(t *testing.T) {
	s := NewStore()
	results := s.Search([]float32{1, 0}, 5)
	if results != nil {
		t.Errorf("expected nil results for empty store, got %v", results)
	}
}

func TestRemove(t *testing.T) {
	s := NewStore()

	s.Add("file1.txt", 100, []VectorEntry{
		{Vector: []float32{1, 0}, Text: "a", ChunkIndex: 0},
	})
	s.Add("file2.txt", 200, []VectorEntry{
		{Vector: []float32{0, 1}, Text: "b", ChunkIndex: 0},
	})

	s.Remove("file1.txt")

	stats := s.Stats()
	if stats.VectorCount != 1 {
		t.Errorf("expected 1 vector after remove, got %d", stats.VectorCount)
	}
	if stats.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file after remove, got %d", stats.IndexedFiles)
	}

	// Search should only return file2 content
	results := s.Search([]float32{1, 0}, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Text != "b" {
		t.Errorf("expected remaining entry to be 'b', got %s", results[0].Text)
	}
}

func TestAddReplacesPreviousEntries(t *testing.T) {
	s := NewStore()

	s.Add("file1.txt", 100, []VectorEntry{
		{Vector: []float32{1, 0}, Text: "old", ChunkIndex: 0},
	})
	s.Add("file1.txt", 200, []VectorEntry{
		{Vector: []float32{0, 1}, Text: "new", ChunkIndex: 0},
	})

	stats := s.Stats()
	if stats.VectorCount != 1 {
		t.Errorf("expected 1 vector after replace, got %d", stats.VectorCount)
	}

	results := s.Search([]float32{0, 1}, 1)
	if results[0].Text != "new" {
		t.Errorf("expected 'new', got %s", results[0].Text)
	}
}

func TestStats(t *testing.T) {
	s := NewStore()

	stats := s.Stats()
	if stats.VectorCount != 0 || stats.IndexedFiles != 0 {
		t.Errorf("expected empty stats, got %+v", stats)
	}

	s.Add("file1.txt", 100, []VectorEntry{
		{Vector: []float32{1, 0}, Text: "a", ChunkIndex: 0},
		{Vector: []float32{0, 1}, Text: "b", ChunkIndex: 1},
	})
	s.Add("file2.txt", 200, []VectorEntry{
		{Vector: []float32{1, 1}, Text: "c", ChunkIndex: 0},
	})

	stats = s.Stats()
	if stats.VectorCount != 3 {
		t.Errorf("expected 3 vectors, got %d", stats.VectorCount)
	}
	if stats.IndexedFiles != 2 {
		t.Errorf("expected 2 indexed files, got %d", stats.IndexedFiles)
	}
	if stats.StoreSizeBytes <= 0 {
		t.Errorf("expected positive store size, got %d", stats.StoreSizeBytes)
	}

	s.Remove("file1.txt")
	stats = s.Stats()
	if stats.VectorCount != 1 {
		t.Errorf("expected 1 vector after remove, got %d", stats.VectorCount)
	}
	if stats.IndexedFiles != 1 {
		t.Errorf("expected 1 indexed file after remove, got %d", stats.IndexedFiles)
	}
}

func TestFileMtime(t *testing.T) {
	s := NewStore()

	_, ok := s.FileMtime("missing.txt")
	if ok {
		t.Error("expected ok=false for missing file")
	}

	s.Add("file1.txt", 12345, []VectorEntry{
		{Vector: []float32{1}, Text: "a", ChunkIndex: 0},
	})

	mt, ok := s.FileMtime("file1.txt")
	if !ok {
		t.Error("expected ok=true for existing file")
	}
	if mt != 12345 {
		t.Errorf("expected mtime 12345, got %d", mt)
	}
}
