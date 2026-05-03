package service

import (
	"context"
	"fmt"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	appmodel "eino_ctf_agent/internal/model"
)

type ChatService struct {
	chatModel  einomodel.BaseChatModel
	ragService *RAGService
}

type ChatStream struct {
	Reader    *schema.StreamReader[*schema.Message]
	Citations []appmodel.Citation
	Skills    []appmodel.SkillRef
}

func NewChatService(chatModel einomodel.BaseChatModel, ragService *RAGService) *ChatService {
	return &ChatService{chatModel: chatModel, ragService: ragService}
}

func (s *ChatService) Chat(ctx context.Context, req *appmodel.ChatRequest) (*appmodel.ChatResponse, error) {
	if err := validateChatRequest(req); err != nil {
		return nil, err
	}
	if s.ragService != nil {
		return s.ragService.Generate(ctx, req)
	}

	messages := toSchemaMessages(req.Messages)
	resp, err := s.chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM generate failed: %w", err)
	}
	return &appmodel.ChatResponse{Reply: resp.Content}, nil
}

func (s *ChatService) Stream(ctx context.Context, req *appmodel.ChatRequest) (*ChatStream, error) {
	if err := validateChatRequest(req); err != nil {
		return nil, err
	}
	if s.ragService != nil {
		return s.ragService.Stream(ctx, req)
	}

	reader, err := s.chatModel.Stream(ctx, toSchemaMessages(req.Messages))
	if err != nil {
		return nil, fmt.Errorf("LLM stream failed: %w", err)
	}
	return &ChatStream{Reader: reader}, nil
}

func validateChatRequest(req *appmodel.ChatRequest) error {
	if req == nil || len(req.Messages) == 0 {
		return fmt.Errorf("messages are required")
	}
	for i, msg := range req.Messages {
		if msg.Content == "" {
			return fmt.Errorf("message %d content is empty", i)
		}
	}
	return nil
}

func toSchemaMessages(messages []appmodel.ChatMessage) []*schema.Message {
	out := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, &schema.Message{
			Role:    toSchemaRole(msg.Role),
			Content: msg.Content,
		})
	}
	return out
}

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
