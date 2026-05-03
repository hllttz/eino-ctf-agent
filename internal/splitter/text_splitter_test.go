package splitter

import (
	"strings"
	"testing"

	"eino_ctf_agent/internal/parser"
)

func TestSplitMarkdownBlocks(t *testing.T) {
	s, err := NewTextSplitter(10, 2)
	if err != nil {
		t.Fatalf("NewTextSplitter returned error: %v", err)
	}
	chunks := s.SplitMarkdownBlocks([]parser.MarkdownBlock{
		{HeadingPath: "A", Content: strings.Repeat("a", 25)},
	})
	if len(chunks) != 3 {
		t.Fatalf("len(chunks) = %d, want 3", len(chunks))
	}
	for _, chunk := range chunks {
		if chunk.HeadingPath != "A" {
			t.Fatalf("heading path = %q, want A", chunk.HeadingPath)
		}
		if len([]rune(chunk.Content)) > 10 {
			t.Fatalf("chunk length = %d, want <= 10", len([]rune(chunk.Content)))
		}
	}
}

func TestNewTextSplitterRejectsInvalidOverlap(t *testing.T) {
	if _, err := NewTextSplitter(10, 10); err == nil {
		t.Fatal("NewTextSplitter returned nil error for invalid overlap")
	}
}
