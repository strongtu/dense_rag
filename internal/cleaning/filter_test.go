package cleaning

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSupportedFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"document.txt", true},
		{"document.TXT", true},
		{"document.Txt", true},
		{"report.docx", true},
		{"report.DOCX", true},
		{"report.Docx", true},
		{"image.png", false},
		{"data.pdf", false},
		{"archive.zip", false},
		{"noext", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsSupportedFile(tt.path)
			if got != tt.want {
				t.Errorf("IsSupportedFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsFileTooLarge(t *testing.T) {
	dir := t.TempDir()

	// Create a small file.
	small := filepath.Join(dir, "small.txt")
	if err := os.WriteFile(small, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	tooLarge, err := IsFileTooLarge(small)
	if err != nil {
		t.Fatalf("IsFileTooLarge returned error: %v", err)
	}
	if tooLarge {
		t.Error("small file should not be too large")
	}
}

func TestIsFileTooLarge_NonExistent(t *testing.T) {
	_, err := IsFileTooLarge("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestIsFileTooLarge_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")

	// Create a sparse file that appears larger than MaxFileSize.
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxFileSize + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	tooLarge, err := IsFileTooLarge(path)
	if err != nil {
		t.Fatalf("IsFileTooLarge returned error: %v", err)
	}
	if !tooLarge {
		t.Error("large file should be too large")
	}
}
