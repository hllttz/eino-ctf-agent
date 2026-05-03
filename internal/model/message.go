package model

type ChatRequest struct {
	ConversationID string        `json:"conversation_id,omitempty"`
	Messages       []ChatMessage `json:"messages" binding:"required,min=1"`
}

type ChatMessage struct {
	Role    string `json:"role" binding:"required,oneof=user assistant system"`
	Content string `json:"content" binding:"required"`
}

type ChatResponse struct {
	Reply     string     `json:"reply"`
	Citations []Citation `json:"citations,omitempty"`
	Skills    []SkillRef `json:"skills,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type SkillRef struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}
