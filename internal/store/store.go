package store

import (
	"sort"
	"sync"
)

// VectorEntry represents a single embedded chunk stored in the vector store.
type VectorEntry struct {
	Vector     []float32
	Text       string
	FilePath   string
	ChunkIndex int
}

// SearchResult is returned by Search with the matched text and relevance score.
type SearchResult struct {
	Text     string
	FilePath string
	Score    float32
}

// StoreStats holds summary statistics about the store.
type StoreStats struct {
	VectorCount    int
	IndexedFiles   int
	StoreSizeBytes int64
}

// Store is an in-memory vector store with file-level indexing.
type Store struct {
	sync.RWMutex
	entries    []VectorEntry
	fileIndex  map[string][]int   // filePath → indices into entries
	fileMtimes map[string]int64   // filePath → mtime unix timestamp
}

// NewStore creates a new empty Store.
func NewStore() *Store {
	return &Store{
		entries:    nil,
		fileIndex:  make(map[string][]int),
		fileMtimes: make(map[string]int64),
	}
}

// Add inserts new entries for the given file path. Any previous entries for the
// same file are removed first. mtime is stored for reconciliation.
func (s *Store) Add(filePath string, mtime int64, newEntries []VectorEntry) {
	s.Lock()
	defer s.Unlock()

	s.removeUnsafe(filePath)

	startIdx := len(s.entries)
	indices := make([]int, len(newEntries))
	for i, e := range newEntries {
		e.FilePath = filePath
		s.entries = append(s.entries, e)
		indices[i] = startIdx + i
	}

	s.fileIndex[filePath] = indices
	s.fileMtimes[filePath] = mtime
}

// Remove deletes all entries associated with the given file path.
func (s *Store) Remove(filePath string) {
	s.Lock()
	defer s.Unlock()

	s.removeUnsafe(filePath)
}

// removeUnsafe removes entries for filePath and rebuilds the fileIndex.
// Must be called while holding the write lock.
func (s *Store) removeUnsafe(filePath string) {
	if _, ok := s.fileIndex[filePath]; !ok {
		return
	}

	// Filter out entries belonging to filePath.
	filtered := make([]VectorEntry, 0, len(s.entries))
	for _, e := range s.entries {
		if e.FilePath != filePath {
			filtered = append(filtered, e)
		}
	}
	s.entries = filtered

	delete(s.fileIndex, filePath)
	delete(s.fileMtimes, filePath)

	// Rebuild entire fileIndex from scratch.
	for k := range s.fileIndex {
		delete(s.fileIndex, k)
	}
	for i, e := range s.entries {
		s.fileIndex[e.FilePath] = append(s.fileIndex[e.FilePath], i)
	}
}

// Search returns the top-k entries most similar to the query vector, sorted
// descending by cosine similarity score.
func (s *Store) Search(query []float32, topK int) []SearchResult {
	s.RLock()
	defer s.RUnlock()

	if len(s.entries) == 0 || topK <= 0 {
		return nil
	}

	type scored struct {
		idx   int
		score float32
	}

	scores := make([]scored, len(s.entries))
	for i, e := range s.entries {
		scores[i] = scored{idx: i, score: CosineSimilarity(query, e.Vector)}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if topK > len(scores) {
		topK = len(scores)
	}

	results := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		e := s.entries[scores[i].idx]
		results[i] = SearchResult{
			Text:     e.Text,
			FilePath: e.FilePath,
			Score:    scores[i].score,
		}
	}

	return results
}

// Stats returns summary statistics about the store.
func (s *Store) Stats() StoreStats {
	s.RLock()
	defer s.RUnlock()

	var sizeBytes int64
	for _, e := range s.entries {
		sizeBytes += int64(len(e.Vector)*4 + len(e.Text) + len(e.FilePath))
	}

	return StoreStats{
		VectorCount:    len(s.entries),
		IndexedFiles:   len(s.fileIndex),
		StoreSizeBytes: sizeBytes,
	}
}

// FileMtime returns the stored modification time for the given file path.
func (s *Store) FileMtime(filePath string) (int64, bool) {
	s.RLock()
	defer s.RUnlock()

	mt, ok := s.fileMtimes[filePath]
	return mt, ok
}

// HasIndexedFile reports whether the given path is an indexed file (for safe document fetch).
func (s *Store) HasIndexedFile(filePath string) bool {
	s.RLock()
	defer s.RUnlock()

	_, ok := s.fileIndex[filePath]
	return ok
}
