package cleaning

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaxFileSize is the maximum supported file size (20 MB).
var MaxFileSize int64 = 20 * 1024 * 1024

// IsSupportedFile returns true if the file extension is .txt or .docx (case
// insensitive) and the file is not a Word temporary file (~$*.docx).
func IsSupportedFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, "~$") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".txt" || ext == ".docx"
}

// IsFileTooLarge returns true if the file at path exceeds MaxFileSize.
func IsFileTooLarge(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	return info.Size() > MaxFileSize, nil
}
