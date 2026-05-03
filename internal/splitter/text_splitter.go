package splitter

import (
	"fmt"
	"strings"

	"eino_ctf_agent/internal/parser"
)

type TextChunk struct {
	HeadingPath string
	Content     string
}

type TextSplitter struct {
	ChunkSize int
	Overlap   int
}

func NewTextSplitter(chunkSize, overlap int) (*TextSplitter, error) {
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be positive")
	}
	if overlap < 0 {
		return nil, fmt.Errorf("chunk overlap cannot be negative")
	}
	if overlap >= chunkSize {
		return nil, fmt.Errorf("chunk overlap must be smaller than chunk size")
	}
	return &TextSplitter{ChunkSize: chunkSize, Overlap: overlap}, nil
}

func (s *TextSplitter) SplitMarkdownBlocks(blocks []parser.MarkdownBlock) []TextChunk {
	chunks := make([]TextChunk, 0, len(blocks))
	for _, block := range blocks {
		text := strings.TrimSpace(block.Content)
		if text == "" {
			continue
		}
		chunks = append(chunks, s.splitText(block.HeadingPath, text)...)
	}
	return chunks
}

func (s *TextSplitter) splitText(headingPath, text string) []TextChunk {
	runes := []rune(text)
	if len(runes) <= s.ChunkSize {
		return []TextChunk{{HeadingPath: headingPath, Content: text}}
	}

	step := s.ChunkSize - s.Overlap
	chunks := make([]TextChunk, 0, len(runes)/step+1)
	for start := 0; start < len(runes); start += step {
		end := start + s.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunkText := strings.TrimSpace(string(runes[start:end]))
		if chunkText != "" {
			chunks = append(chunks, TextChunk{
				HeadingPath: headingPath,
				Content:     chunkText,
			})
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}
