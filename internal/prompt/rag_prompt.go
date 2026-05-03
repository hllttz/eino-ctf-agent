package prompt

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"

	"eino_ctf_agent/internal/retriever"
	"eino_ctf_agent/internal/skill"
)

const ragSystemIntro = `你是一个本地知识库问答助手。请优先基于提供的知识库上下文回答。
如果上下文没有直接依据，必须明确说明“当前知识库中没有找到直接依据”，再给出谨慎的通用建议。
回答要清晰、可操作，涉及不确定信息时说明依据不足。`

func BuildRAGSystemPrompt(docs []*schema.Document, activeSkills []skill.Skill, maxContextChunks int) string {
	var b strings.Builder
	b.WriteString(ragSystemIntro)
	appendSkillContext(&b, activeSkills)
	appendRetrievedContext(&b, docs, maxContextChunks)
	return b.String()
}

func appendSkillContext(b *strings.Builder, activeSkills []skill.Skill) {
	if len(activeSkills) == 0 {
		return
	}

	b.WriteString("\n\n[Active Skills]\n")
	b.WriteString("以下 Skills 是任务方法论和操作流程，只能作为解题步骤指导，不等同于知识库事实依据。\n\n")
	for _, s := range activeSkills {
		b.WriteString(fmt.Sprintf(
			"<skill name=%q title=%q priority=%d>\nDescription: %s\n%s\n</skill>\n\n",
			s.Name,
			s.Title,
			s.Priority,
			s.Description,
			trimSkillBody(s.Body, s.MaxTokens),
		))
	}
}

func appendRetrievedContext(b *strings.Builder, docs []*schema.Document, maxContextChunks int) {
	b.WriteString("\n\n[Retrieved Context]\n")
	if len(docs) == 0 {
		b.WriteString("当前知识库中没有找到直接依据。")
		return
	}
	if maxContextChunks <= 0 || maxContextChunks > len(docs) {
		maxContextChunks = len(docs)
	}
	for i := 0; i < maxContextChunks; i++ {
		doc := docs[i]
		b.WriteString(fmt.Sprintf(
			"<doc source=%q document_id=%q chunk=%d score=%.4f heading=%q>\n%s\n</doc>\n\n",
			metadataString(doc, retriever.MetaFilename),
			metadataString(doc, retriever.MetaDocumentID),
			metadataInt(doc, retriever.MetaChunkIndex),
			doc.Score(),
			metadataString(doc, retriever.MetaHeadingPath),
			doc.Content,
		))
	}
}

func metadataString(doc *schema.Document, key string) string {
	if doc == nil || doc.MetaData == nil {
		return ""
	}
	value, ok := doc.MetaData[key]
	if !ok {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func metadataInt(doc *schema.Document, key string) int {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	switch value := doc.MetaData[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func trimSkillBody(body string, maxTokens int) string {
	body = strings.TrimSpace(body)
	if maxTokens <= 0 {
		maxTokens = 2000
	}
	// Rough token budget: keep the logic deterministic and dependency-free.
	maxRunes := maxTokens * 2
	runes := []rune(body)
	if len(runes) <= maxRunes {
		return body
	}
	return string(runes[:maxRunes]) + "\n..."
}
