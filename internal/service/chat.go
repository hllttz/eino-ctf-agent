package service

import (
	"context"
	"fmt"
	"log"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	appmodel "eino_ctf_agent/internal/model"
)

// ChatService 处理聊天业务逻辑。
// 只依赖 Eino 的 BaseChatModel 抽象接口，不直接依赖 DeepSeek 包。
type ChatService struct {
	chatModel einomodel.BaseChatModel
}

// NewChatService 创建 ChatService 实例。
func NewChatService(chatModel einomodel.BaseChatModel) *ChatService {
	return &ChatService{
		chatModel: chatModel,
	}
}

// Chat 执行非流式对话，返回完整回复。
func (s *ChatService) Chat(ctx context.Context, req *appmodel.ChatRequest) (string, error) {
	// 将前端消息格式转换为 Eino schema.Message
	messages := make([]*schema.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, &schema.Message{
			Role:    toSchemaRole(msg.Role),
			Content: msg.Content,
		})
	}

	// 调用 LLM Generate
	resp, err := s.chatModel.Generate(ctx, messages)
	if err != nil {
		log.Printf("[ERROR] LLM Generate failed: %v", err)
		return "", fmt.Errorf("LLM generate failed: %w", err)
	}

	return resp.Content, nil
}

// toSchemaRole 将字符串角色转换为 Eino 的 RoleType。
func toSchemaRole(role string) schema.RoleType {
	switch role {
	case "user":
		return schema.User
	case "assistant":
		return schema.Assistant
	case "system":
		return schema.System
	case "tool":
		return schema.Tool
	default:
		return schema.User
	}
}
