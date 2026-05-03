package retriever

import (
	"context"
	"fmt"
	"strings"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"eino_ctf_agent/internal/vectorstore"
)

const (
	MetaDocumentID  = "document_id"
	MetaFilename    = "filename"
	MetaFileType    = "file_type"
	MetaChunkIndex  = "chunk_index"
	MetaHeadingPath = "heading_path"
	MetaPageNumber  = "page_number"
)

type KnowledgeRetriever struct {
	embedder    einoembedding.Embedder
	vectorStore vectorstore.VectorStore
	defaultTopK int
}

func NewKnowledgeRetriever(embedder einoembedding.Embedder, vectorStore vectorstore.VectorStore) *KnowledgeRetriever {
	return &KnowledgeRetriever{
		embedder:    embedder,
		vectorStore: vectorStore,
		defaultTopK: 5,
	}
}

func (r *KnowledgeRetriever) Retrieve(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}

	topK := r.defaultTopK
	options := einoretriever.GetCommonOptions(&einoretriever.Options{TopK: &topK, Embedding: r.embedder}, opts...)
	if options.Embedding == nil {
		return nil, fmt.Errorf("retriever embedding is nil")
	}

	vectors, err := options.Embedding.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("embedding response count mismatch")
	}

	searchOpts := vectorstore.SearchOptions{}
	if options.TopK != nil {
		searchOpts.TopK = *options.TopK
	}
	if options.ScoreThreshold != nil {
		searchOpts.ScoreThreshold = *options.ScoreThreshold
	}

	results, err := r.vectorStore.Search(ctx, vectors[0], searchOpts)
	if err != nil {
		return nil, err
	}

	docs := make([]*schema.Document, 0, len(results))
	for _, result := range results {
		record := result.Record
		doc := &schema.Document{
			ID:      record.ChunkID,
			Content: record.Content,
			MetaData: map[string]any{
				MetaDocumentID:  record.DocumentID,
				MetaFilename:    record.Filename,
				MetaFileType:    record.FileType,
				MetaChunkIndex:  record.ChunkIndex,
				MetaHeadingPath: record.HeadingPath,
				MetaPageNumber:  record.PageNumber,
			},
		}
		doc.WithScore(result.Score)
		docs = append(docs, doc)
	}
	return docs, nil
}
