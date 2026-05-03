package embedding

import (
	"fmt"

	"eino_ctf_agent/internal/config"
)

func NewEmbedder(cfg *config.Config) (Embedder, error) {
	switch cfg.Embedding.Provider {
	case "dashscope", "qwen":
		return NewQwenEmbedder(cfg)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Embedding.Provider)
	}
}
