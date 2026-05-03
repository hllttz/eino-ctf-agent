package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"

	"eino_ctf_agent/internal/config"
)

// NewChatModel 根据配置中的 provider 创建对应的 ChatModel。
// 目前只支持 deepseek（通过 OpenAI-compatible API）。
// 后续可扩展支持其他 provider，例如 openai、ollama。
func NewChatModel(ctx context.Context, cfg *config.Config) (model.BaseChatModel, error) {
	switch cfg.LLM.Provider {
	case "deepseek":
		return NewDeepSeekChatModel(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
	}
}
