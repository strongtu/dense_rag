package cleaning

import (
	"strings"
	"testing"
)

func TestChunkText_Empty(t *testing.T) {
	chunks := ChunkText("", "test.txt", DefaultChunkSize, DefaultOverlap)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}

func TestChunkText_SingleChunk(t *testing.T) {
	text := "Short text."
	chunks := ChunkText(text, "test.txt", DefaultChunkSize, DefaultOverlap)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text != text {
		t.Errorf("chunk text = %q, want %q", chunks[0].Text, text)
	}
	if chunks[0].FilePath != "test.txt" {
		t.Errorf("chunk filePath = %q, want %q", chunks[0].FilePath, "test.txt")
	}
	if chunks[0].Index != 0 {
		t.Errorf("chunk index = %d, want 0", chunks[0].Index)
	}
	if chunks[0].Offset != 0 {
		t.Errorf("chunk offset = %d, want 0", chunks[0].Offset)
	}
}

func TestChunkText_MultipleChunks(t *testing.T) {
	// Create text with paragraphs that together exceed chunkSize.
	para1 := strings.Repeat("a", 300)
	para2 := strings.Repeat("b", 300)
	text := para1 + "\n\n" + para2

	chunks := ChunkText(text, "file.txt", 512, 64)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	// First chunk should contain para1.
	if !strings.Contains(chunks[0].Text, para1) {
		t.Error("first chunk should contain para1")
	}
	// Second chunk should contain para2.
	if !strings.Contains(chunks[1].Text, para2) {
		t.Error("second chunk should contain para2")
	}
}

func TestChunkText_LargeParagraphOverlap(t *testing.T) {
	// Single paragraph larger than chunkSize — should be split with overlap.
	text := strings.Repeat("x", 1000)
	chunks := ChunkText(text, "big.txt", 400, 50)

	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks for 1000 chars with size=400, got %d", len(chunks))
	}

	// Verify indices are sequential.
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk[%d].Index = %d, want %d", i, c.Index, i)
		}
	}

	// Verify overlap: end of chunk[0] should overlap with start of chunk[1].
	overlap0 := chunks[0].Text[len(chunks[0].Text)-50:]
	start1 := chunks[1].Text[:50]
	if overlap0 != start1 {
		t.Error("expected overlap between chunk 0 and chunk 1")
	}
}

func TestChunkText_ParagraphSplitting(t *testing.T) {
	// Multiple small paragraphs that fit within a single chunk.
	text := "Hello.\n\nWorld.\n\nFoo.\n\nBar."
	chunks := ChunkText(text, "test.txt", 512, 64)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for small paragraphs, got %d", len(chunks))
	}
	// The chunk should contain all paragraph texts joined.
	for _, word := range []string{"Hello.", "World.", "Foo.", "Bar."} {
		if !strings.Contains(chunks[0].Text, word) {
			t.Errorf("chunk should contain %q", word)
		}
	}
}

func TestChunkText_DefaultParams(t *testing.T) {
	text := strings.Repeat("y", 100)
	chunks := ChunkText(text, "f.txt", 0, -1)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk with default params for short text, got %d", len(chunks))
	}
}
