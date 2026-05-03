package embedding

import (
	"context"
	"fmt"
	"time"

	openaiembedding "github.com/cloudwego/eino-ext/components/embedding/openai"

	"eino_ctf_agent/internal/config"
)

func NewQwenEmbedder(ctx context.Context, cfg *config.Config) (Embedder, error) {
	apiKey := cfg.GetEmbeddingAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("embedding API key not set: environment variable %s is empty", cfg.Embedding.APIKeyEnv)
	}

	dimensions := cfg.Embedding.Dimension
	embedder, err := openaiembedding.NewEmbedder(ctx, &openaiembedding.EmbeddingConfig{
		APIKey:     apiKey,
		BaseURL:    cfg.Embedding.BaseURL,
		Model:      cfg.Embedding.Model,
		Dimensions: &dimensions,
		Timeout:    60 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("create Qwen embedder: %w", err)
	}
	return embedder, nil
}
