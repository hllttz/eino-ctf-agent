package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"

	"eino_ctf_agent/internal/config"
)

func NewChatModel(ctx context.Context, cfg *config.Config) (model.BaseChatModel, error) {
	switch cfg.LLM.Provider {
	case "deepseek":
		return NewDeepSeekChatModel(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
	}
}
