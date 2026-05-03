package model

// ChatRequest 是 POST /api/chat 和 POST /api/chat/stream 的请求体。
type ChatRequest struct {
	ConversationID string        `json:"conversation_id,omitempty"`
	Messages       []ChatMessage `json:"messages" binding:"required,min=1"`
}

// ChatMessage 是前端发送的单条消息。
type ChatMessage struct {
	Role    string `json:"role" binding:"required,oneof=user assistant system"`
	Content string `json:"content" binding:"required"`
}

// ChatResponse 是 POST /api/chat 的非流式响应体。
type ChatResponse struct {
	Reply string `json:"reply"`
}

// ErrorResponse 是统一错误响应格式。
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}
