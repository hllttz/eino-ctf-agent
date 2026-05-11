package knowledge

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

var markdownHeadingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)

// MarkdownBlock Markdown解析后的逻辑块，包含标题路径和正文。
type MarkdownBlock struct {
	HeadingPath string
	Content     string
}

// TextChunk 文本切分后的片段，用于向量化与检索。
type TextChunk struct {
	HeadingPath string
	Content     string
}

// TextSplitter 文本分割器，按字符数滑动窗口切分文本块。
type TextSplitter struct {
	ChunkSize int
	Overlap   int
}

func ParseMarkdown(content string) ([]MarkdownBlock, error) {
	content = stripFrontMatter(strings.ReplaceAll(content, "\r\n", "\n"))
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("markdown content is empty")
	}

	var blocks []MarkdownBlock
	var headings [6]string
	var current strings.Builder
	currentHeading := ""

	flush := func() {
		text := strings.TrimSpace(current.String())
		if text == "" {
			current.Reset()
			return
		}
		blocks = append(blocks, MarkdownBlock{
			HeadingPath: currentHeading,
			Content:     text,
		})
		current.Reset()
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	const maxLineSize = 1024 * 1024
	scanner.Buffer(make([]byte, 64*1024), maxLineSize)

	for scanner.Scan() {
		line := scanner.Text()
		if match := markdownHeadingPattern.FindStringSubmatch(line); match != nil {
			flush()
			level := len(match[1])
			title := strings.TrimSpace(match[2])
			headings[level-1] = title
			for i := level; i < len(headings); i++ {
				headings[i] = ""
			}
			currentHeading = joinHeadings(headings[:])
			current.WriteString(line)
			current.WriteByte('\n')
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan markdown: %w", err)
	}
	flush()

	if len(blocks) == 0 {
		blocks = append(blocks, MarkdownBlock{Content: strings.TrimSpace(content)})
	}
	return blocks, nil
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

func (s *TextSplitter) SplitMarkdownBlocks(blocks []MarkdownBlock) []TextChunk {
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

func stripFrontMatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	rest := content[len("---\n"):]
	if idx := strings.Index(rest, "\n---\n"); idx >= 0 {
		return rest[idx+len("\n---\n"):]
	}
	return content
}

func joinHeadings(headings []string) string {
	parts := make([]string, 0, len(headings))
	for _, heading := range headings {
		if heading != "" {
			parts = append(parts, heading)
		}
	}
	return strings.Join(parts, " > ")
}
