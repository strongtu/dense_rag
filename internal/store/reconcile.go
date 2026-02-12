package store

import (
	"os"
	"path/filepath"
	"strings"
)

// Reconcile walks the given directories and compares the files on disk with
// the store state. It returns lists of files that need to be added, removed,
// or updated. It does NOT perform any mutations on the store.
func (s *Store) Reconcile(watchDirs []string, supportedExts []string) (added, removed, updated []string) {
	extSet := make(map[string]bool, len(supportedExts))
	for _, ext := range supportedExts {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extSet[strings.ToLower(ext)] = true
	}

	// Track which files on disk we see.
	seen := make(map[string]bool)

	for _, watchDir := range watchDirs {
		filepath.Walk(watchDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip files we can't stat
			}
			if info.IsDir() {
				return nil
			}

			// Skip Word temp files.
			base := filepath.Base(path)
			if strings.HasPrefix(base, "~$") {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if !extSet[ext] {
				return nil
			}

			seen[path] = true
			mtime := info.ModTime().Unix()

			storedMtime, exists := s.FileMtime(path)
			if !exists {
				added = append(added, path)
			} else if mtime != storedMtime {
				updated = append(updated, path)
			}

			return nil
		})
	}

	// Check for files in store that are no longer on disk.
	s.RLock()
	for filePath := range s.fileMtimes {
		if !seen[filePath] {
			removed = append(removed, filePath)
		}
	}
	s.RUnlock()

	return added, removed, updated
}
