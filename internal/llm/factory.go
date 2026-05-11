package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"

	"eino_ctf_agent/internal/config"
)

// NewChatModel 创建 ChatModel（BaseChatModel 接口），兼容现有调用方。
func NewChatModel(ctx context.Context, cfg *config.Config) (model.BaseChatModel, error) {
	switch cfg.LLM.Provider {
	case "deepseek":
		return NewDeepSeekChatModel(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
	}
}

// NewToolCallingChatModel 创建支持 Tool Calling 的 ChatModel。
// 用于 Agent 模式（ReAct），模型可自动决策调用工具。
func NewToolCallingChatModel(ctx context.Context, cfg *config.Config) (model.ToolCallingChatModel, error) {
	switch cfg.LLM.Provider {
	case "deepseek":
		return NewDeepSeekToolCallingModel(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.LLM.Provider)
	}
}
