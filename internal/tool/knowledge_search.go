package tool

import (
	"context"
	"fmt"
	"strings"

	einoretriever "github.com/cloudwego/eino/components/retriever"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"eino_ctf_agent/internal/knowledge"
)

// KnowledgeSearchInput 知识库检索工具的输入参数。
type KnowledgeSearchInput struct {
	Query string `json:"query" jsonschema:"description=search query string to find relevant documents"`
}

// KnowledgeSearchOutput 知识库检索工具的输出结果。
type KnowledgeSearchOutput struct {
	Results string `json:"results" jsonschema:"description=retrieved document chunks with source and score"`
}

// NewKnowledgeSearchTool 创建知识库检索工具，包装 Eino Retriever。
// topK 和 scoreThreshold 控制检索精度与数量。
func NewKnowledgeSearchTool(retriever einoretriever.Retriever, topK int, scoreThreshold float64) (einotool.InvokableTool, error) {
	return utils.InferTool[KnowledgeSearchInput, KnowledgeSearchOutput](
		"knowledge_search",
		"Search the local knowledge base for relevant document chunks. "+
			"Use this when the user asks about topics that may be covered in uploaded documents. "+
			"Returns formatted text with source filename, heading path, and relevance score for each result.",
		func(ctx context.Context, input KnowledgeSearchInput) (KnowledgeSearchOutput, error) {
			docs, err := retriever.Retrieve(
				ctx,
				input.Query,
				einoretriever.WithTopK(topK),
				einoretriever.WithScoreThreshold(scoreThreshold),
			)
			if err != nil {
				return KnowledgeSearchOutput{}, fmt.Errorf("knowledge search failed: %w", err)
			}
			return KnowledgeSearchOutput{Results: formatRetrievedDocs(docs)}, nil
		},
	)
}

func formatRetrievedDocs(docs []*schema.Document) string {
	if len(docs) == 0 {
		return "未找到相关知识库内容。"
	}
	var b strings.Builder
	for i, doc := range docs {
		b.WriteString(fmt.Sprintf(
			"[%d] source=%q heading=%q score=%.4f\n%s\n\n",
			i+1,
			knowledge.MetadataString(doc, knowledge.MetaFilename),
			knowledge.MetadataString(doc, knowledge.MetaHeadingPath),
			doc.Score(),
			doc.Content,
		))
	}
	return b.String()
}
