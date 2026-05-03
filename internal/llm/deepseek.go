package llm

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"eino_ctf_agent/internal/config"
)

func NewDeepSeekChatModel(ctx context.Context, cfg *config.Config) (model.BaseChatModel, error) {
	apiKey := cfg.GetLLMAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("LLM API key not set: environment variable %s is empty", cfg.LLM.APIKeyEnv)
	}

	temperature := float32(cfg.LLM.Temperature)
	maxTokens := cfg.LLM.MaxTokens

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:      apiKey,
		BaseURL:     cfg.LLM.BaseURL,
		Model:       cfg.LLM.Model,
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
		Timeout:     60 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("create DeepSeek ChatModel: %w", err)
	}
	return chatModel, nil
}
