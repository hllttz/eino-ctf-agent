package knowledge

import (
	"strings"
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestParseMarkdownHeadings(t *testing.T) {
	blocks, err := ParseMarkdown("# A\nintro\n\n## B\ndetail")
	if err != nil {
		t.Fatalf("ParseMarkdown returned error: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(blocks))
	}
	if blocks[0].HeadingPath != "A" {
		t.Fatalf("first heading = %q, want A", blocks[0].HeadingPath)
	}
	if blocks[1].HeadingPath != "A > B" {
		t.Fatalf("second heading = %q, want A > B", blocks[1].HeadingPath)
	}
}

func TestParseMarkdownRejectsEmpty(t *testing.T) {
	if _, err := ParseMarkdown("  \n\t"); err == nil {
		t.Fatal("ParseMarkdown returned nil error for empty content")
	}
}

func TestSplitMarkdownBlocks(t *testing.T) {
	s, err := NewTextSplitter(10, 2)
	if err != nil {
		t.Fatalf("NewTextSplitter returned error: %v", err)
	}
	chunks := s.SplitMarkdownBlocks([]MarkdownBlock{
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

func TestRedisDocumentToSchemaSetsScore(t *testing.T) {
	doc, err := RedisDocumentToSchema(nil, goredis.Document{
		ID: "chunk-1",
		Fields: map[string]string{
			RedisContentField: "hello",
			MetaDocumentID:    "doc-1",
			MetaFilename:      "a.md",
			MetaFileType:      "markdown",
			MetaChunkIndex:    "2",
			MetaHeadingPath:   "A",
			MetaPageNumber:    "0",
			"distance":        "0.25",
		},
	})
	if err != nil {
		t.Fatalf("RedisDocumentToSchema returned error: %v", err)
	}
	if doc.Score() != 0.75 {
		t.Fatalf("score = %v, want 0.75", doc.Score())
	}
	if MetadataInt(doc, MetaChunkIndex) != 2 {
		t.Fatalf("chunk index = %v, want 2", MetadataInt(doc, MetaChunkIndex))
	}
}
