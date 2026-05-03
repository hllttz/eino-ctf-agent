package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"eino_ctf_agent/internal/config"
)

// NewDeepSeekChatModel 根据配置创建 DeepSeek ChatModel。
// DeepSeek 提供 OpenAI-compatible API，所以使用 eino-ext 的 openai 组件。
// 业务层不应直接调用此函数，应通过 NewChatModel factory 创建。
func NewDeepSeekChatModel(ctx context.Context, cfg *config.Config) (model.BaseChatModel, error) {
	apiKey := cfg.GetLLMAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("LLM API key not set: environment variable %s is empty", cfg.LLM.APIKeyEnv)
	}

	temperature := float32(cfg.LLM.Temperature)
	maxTokens := cfg.LLM.MaxTokens

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:    apiKey,
		BaseURL:   cfg.LLM.BaseURL,
		Model:     cfg.LLM.Model,
		MaxTokens: &maxTokens,
		Temperature: &temperature,
		Timeout:   30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create DeepSeek ChatModel: %w", err)
	}

	return chatModel, nil
}
