package embedding

import (
	"context"
	"fmt"

	"eino_ctf_agent/internal/config"
)

func NewEmbedder(ctx context.Context, cfg *config.Config) (Embedder, error) {
	switch cfg.Embedding.Provider {
	case "dashscope", "qwen":
		return NewQwenEmbedder(ctx, cfg)
	case "mock":
		return NewMockEmbedder(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Embedding.Provider)
	}
}
