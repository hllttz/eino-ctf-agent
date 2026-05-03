package parser

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

var markdownHeadingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)

type MarkdownBlock struct {
	HeadingPath string
	Content     string
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
