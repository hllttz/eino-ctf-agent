package parser

import "testing"

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
