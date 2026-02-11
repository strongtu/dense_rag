package store

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

// SchemaVersion is the version of the persistence format.
const SchemaVersion = 1

// persistData is the serialization envelope for gob encoding.
type persistData struct {
	Version    int
	Entries    []VectorEntry
	FileMtimes map[string]int64
}

// Save writes the store contents to disk at the given path.
// It writes to a temporary file first and renames for atomicity.
func (s *Store) Save(path string) error {
	s.RLock()
	defer s.RUnlock()

	data := persistData{
		Version:    SchemaVersion,
		Entries:    s.entries,
		FileMtimes: s.fileMtimes,
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".store-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	enc := gob.NewEncoder(tmp)
	if err := enc.Encode(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("encode store: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// Load reads the store contents from disk, replacing any existing data.
func (s *Store) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open store file: %w", err)
	}
	defer f.Close()

	var data persistData
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&data); err != nil {
		return fmt.Errorf("decode store: %w", err)
	}

	if data.Version != SchemaVersion {
		return fmt.Errorf("unsupported schema version %d (expected %d)", data.Version, SchemaVersion)
	}

	s.Lock()
	defer s.Unlock()

	s.entries = data.Entries
	s.fileMtimes = data.FileMtimes
	if s.fileMtimes == nil {
		s.fileMtimes = make(map[string]int64)
	}

	// Rebuild fileIndex from entries.
	s.fileIndex = make(map[string][]int)
	for i, e := range s.entries {
		s.fileIndex[e.FilePath] = append(s.fileIndex[e.FilePath], i)
	}

	return nil
}
