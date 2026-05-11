package model

import "github.com/cloudwego/eino/schema"

// ChatRequest 聊天请求结构体。
// ConversationID 当前仅用于日志追踪和未来持久化扩展，不代表服务端已保存会话状态。
// 多轮对话依赖客户端在每次请求中传入完整 messages 历史。
// 缺失或为空时兼容旧请求，不影响任何行为。
type ChatRequest struct {
	ConversationID string        `json:"conversation_id,omitempty"`
	Messages       []ChatMessage `json:"messages" binding:"required,min=1"`
}

// ChatMessage 单条聊天消息。
type ChatMessage struct {
	Role    string `json:"role" binding:"required,oneof=user assistant system"`
	Content string `json:"content" binding:"required"`
}

// ChatResponse 聊天响应结构体。
type ChatResponse struct {
	Reply     string     `json:"reply"`
	Citations []Citation `json:"citations,omitempty"`
	Skills    []SkillRef `json:"skills,omitempty"`
}

// ErrorResponse HTTP错误响应结构体。
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SkillRef 技能引用信息，用于API响应中标识匹配到的技能。
type SkillRef struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// LastUserMessage 从消息列表中提取最后一条 user 消息的内容。
// 如果没有 user 消息，退回到最后一条消息。
func LastUserMessage(messages []ChatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	if len(messages) == 0 {
		return ""
	}
	return messages[len(messages)-1].Content
}

// ToSchemaMessages 将内部 ChatMessage 切片转换为 Eino schema.Message 切片。
func ToSchemaMessages(messages []ChatMessage) []*schema.Message {
	out := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, &schema.Message{
			Role:    ToSchemaRole(msg.Role),
			Content: msg.Content,
		})
	}
	return out
}

// ToSchemaRole 将字符串角色名转换为 Eino schema.RoleType。
func ToSchemaRole(role string) schema.RoleType {
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
