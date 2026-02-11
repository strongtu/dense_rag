package cleaning

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zakahan/docx2md"
)

// ReadTxt reads a file as UTF-8 text and returns its contents.
func ReadTxt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read txt %s: %w", path, err)
	}
	return string(data), nil
}

// ReadDocx converts a .docx file to markdown, then strips markdown syntax
// to produce clean plain text. This removes images, tables, formulas,
// headers/footers, bold/italic markers, and links.
func ReadDocx(path string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "docx2md-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	_, md, err := docx2md.DocxConvert(path, tmpDir)
	if err != nil {
		return "", fmt.Errorf("read docx %s: %w", path, err)
	}
	return stripMarkdown(md), nil
}

// ReadFile dispatches to the appropriate reader based on file extension.
func ReadFile(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt":
		return ReadTxt(path)
	case ".docx":
		return ReadDocx(path)
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// stripMarkdown removes common markdown syntax from text.
var (
	reImage      = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reLink       = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reBoldItal3  = regexp.MustCompile(`\*{3}(.+?)\*{3}`)
	reBold       = regexp.MustCompile(`\*{2}(.+?)\*{2}`)
	reItalic     = regexp.MustCompile(`\*(.+?)\*`)
	reUBoldItal3 = regexp.MustCompile(`_{3}(.+?)_{3}`)
	reUBold      = regexp.MustCompile(`_{2}(.+?)_{2}`)
	reUItalic    = regexp.MustCompile(`_(.+?)_`)
	reHeader     = regexp.MustCompile(`(?m)^#{1,6}\s+`)
)

func stripMarkdown(text string) string {
	// Remove images first (before links, since images use similar syntax)
	text = reImage.ReplaceAllString(text, "")
	// Replace links with just the link text
	text = reLink.ReplaceAllString(text, "$1")
	// Remove bold/italic markers (process longest markers first)
	text = reBoldItal3.ReplaceAllString(text, "$1")
	text = reBold.ReplaceAllString(text, "$1")
	text = reItalic.ReplaceAllString(text, "$1")
	text = reUBoldItal3.ReplaceAllString(text, "$1")
	text = reUBold.ReplaceAllString(text, "$1")
	text = reUItalic.ReplaceAllString(text, "$1")
	// Remove header markers
	text = reHeader.ReplaceAllString(text, "")
	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)
	return text
}
