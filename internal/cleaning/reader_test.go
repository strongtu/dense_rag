package cleaning

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTxt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "Hello, world!\nThis is a test file."
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadTxt(path)
	if err != nil {
		t.Fatalf("ReadTxt returned error: %v", err)
	}
	if got != content {
		t.Errorf("ReadTxt = %q, want %q", got, content)
	}
}

func TestReadTxt_NotFound(t *testing.T) {
	_, err := ReadTxt("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestReadFile_Txt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "some text content"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if got != content {
		t.Errorf("ReadFile = %q, want %q", got, content)
	}
}

func TestReadFile_UnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pdf")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadFile(path)
	if err == nil {
		t.Error("expected error for unsupported extension, got nil")
	}
}

func TestStripMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "image removal",
			input: "text ![alt](http://example.com/img.png) more",
			want:  "text  more",
		},
		{
			name:  "link to text",
			input: "see [click here](http://example.com) for info",
			want:  "see click here for info",
		},
		{
			name:  "bold removal",
			input: "this is **bold** text",
			want:  "this is bold text",
		},
		{
			name:  "italic removal",
			input: "this is *italic* text",
			want:  "this is italic text",
		},
		{
			name:  "header removal",
			input: "## Section Title\nBody text",
			want:  "Section Title\nBody text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
