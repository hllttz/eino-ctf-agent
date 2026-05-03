package retriever

import (
	"context"
	"fmt"
	"strings"

	"eino_ctf_agent/internal/embedding"
	"eino_ctf_agent/internal/vectorstore"
)

type KnowledgeRetriever struct {
	embedder    embedding.Embedder
	vectorStore vectorstore.VectorStore
}

func NewKnowledgeRetriever(embedder embedding.Embedder, vectorStore vectorstore.VectorStore) *KnowledgeRetriever {
	return &KnowledgeRetriever{embedder: embedder, vectorStore: vectorStore}
}

func (r *KnowledgeRetriever) Retrieve(ctx context.Context, query string, opts vectorstore.SearchOptions) ([]vectorstore.SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	vectors, err := r.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("embedding response count mismatch")
	}
	return r.vectorStore.Search(ctx, vectors[0], opts)
}
