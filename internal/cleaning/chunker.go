package cleaning

import "strings"

// Chunk represents a segment of text from a file.
type Chunk struct {
	Text     string
	FilePath string
	Index    int
	Offset   int
}

const (
	DefaultChunkSize = 512
	DefaultOverlap   = 64
)

// ChunkText splits text into chunks of approximately chunkSize characters
// with overlap between consecutive chunks.
//
// It first splits the text by double newlines (paragraphs), then accumulates
// paragraphs into chunks up to chunkSize. If a single paragraph exceeds
// chunkSize, it is split by chunkSize with overlap.
func ChunkText(text, filePath string, chunkSize, overlap int) []Chunk {
	if len(text) == 0 {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = 0
	}

	// Short text produces a single chunk.
	if len(text) <= chunkSize {
		return []Chunk{{
			Text:     text,
			FilePath: filePath,
			Index:    0,
			Offset:   0,
		}}
	}

	paragraphs := splitParagraphs(text)

	var chunks []Chunk
	var buf strings.Builder
	bufOffset := 0 // character offset where current buffer started in original text
	currentOffset := 0

	for _, para := range paragraphs {
		paraLen := len(para)

		if paraLen > chunkSize {
			// Flush current buffer first if non-empty.
			if buf.Len() > 0 {
				chunks = append(chunks, Chunk{
					Text:     buf.String(),
					FilePath: filePath,
					Index:    len(chunks),
					Offset:   bufOffset,
				})
				buf.Reset()
			}
			// Split the large paragraph by chunkSize with overlap.
			subChunks := splitLargeParagraph(para, currentOffset, filePath, len(chunks), chunkSize, overlap)
			chunks = append(chunks, subChunks...)
			currentOffset += paraLen
			bufOffset = currentOffset
			continue
		}

		// Check if adding this paragraph would exceed chunkSize.
		newLen := buf.Len()
		if newLen > 0 {
			newLen += 2 // for the "\n\n" separator
		}
		newLen += paraLen

		if newLen > chunkSize {
			// Flush current buffer.
			if buf.Len() > 0 {
				chunks = append(chunks, Chunk{
					Text:     buf.String(),
					FilePath: filePath,
					Index:    len(chunks),
					Offset:   bufOffset,
				})
				buf.Reset()
			}
			bufOffset = currentOffset
		}

		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		} else {
			bufOffset = currentOffset
		}
		buf.WriteString(para)
		currentOffset += paraLen
	}

	// Flush remaining buffer.
	if buf.Len() > 0 {
		chunks = append(chunks, Chunk{
			Text:     buf.String(),
			FilePath: filePath,
			Index:    len(chunks),
			Offset:   bufOffset,
		})
	}

	return chunks
}

// splitParagraphs splits text by double newlines and returns non-empty paragraphs.
// It also tracks the original character offsets consumed.
func splitParagraphs(text string) []string {
	raw := strings.Split(text, "\n\n")
	paragraphs := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			paragraphs = append(paragraphs, p)
		}
	}
	return paragraphs
}

// splitLargeParagraph splits a single paragraph that exceeds chunkSize into
// chunks with overlap.
func splitLargeParagraph(para string, baseOffset int, filePath string, startIndex, chunkSize, overlap int) []Chunk {
	var chunks []Chunk
	pos := 0
	for pos < len(para) {
		end := pos + chunkSize
		if end > len(para) {
			end = len(para)
		}
		chunks = append(chunks, Chunk{
			Text:     para[pos:end],
			FilePath: filePath,
			Index:    startIndex + len(chunks),
			Offset:   baseOffset + pos,
		})
		if end == len(para) {
			break
		}
		pos = end - overlap
	}
	return chunks
}
