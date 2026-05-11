package model

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestLastUserMessage_MultiTurn(t *testing.T) {
	messages := []ChatMessage{
		{Role: "user", Content: "第一问"},
		{Role: "assistant", Content: "第一答"},
		{Role: "user", Content: "第二问"},
	}
	got := LastUserMessage(messages)
	if got != "第二问" {
		t.Errorf("expected 第二问, got %q", got)
	}
}

func TestLastUserMessage_SingleUser(t *testing.T) {
	messages := []ChatMessage{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "唯一用户消息"},
	}
	got := LastUserMessage(messages)
	if got != "唯一用户消息" {
		t.Errorf("expected 唯一用户消息, got %q", got)
	}
}

func TestLastUserMessage_NoUserFallback(t *testing.T) {
	messages := []ChatMessage{
		{Role: "system", Content: "system prompt"},
		{Role: "assistant", Content: "assistant only"},
	}
	got := LastUserMessage(messages)
	if got != "assistant only" {
		t.Errorf("expected fallback to last message, got %q", got)
	}
}

func TestLastUserMessage_Empty(t *testing.T) {
	got := LastUserMessage(nil)
	if got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
	got = LastUserMessage([]ChatMessage{})
	if got != "" {
		t.Errorf("expected empty string for empty slice, got %q", got)
	}
}

func TestToSchemaMessages_KeepsAllMessages(t *testing.T) {
	messages := []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "follow-up"},
	}
	out := ToSchemaMessages(messages)
	if len(out) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(out))
	}
	if out[0].Role != schema.System || out[0].Content != "sys" {
		t.Errorf("message 0 mismatch: role=%v content=%q", out[0].Role, out[0].Content)
	}
	if out[3].Role != schema.User || out[3].Content != "follow-up" {
		t.Errorf("message 3 mismatch: role=%v content=%q", out[3].Role, out[3].Content)
	}
}

func TestChatRequest_ConversationID_Empty(t *testing.T) {
	req := ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	}
	if req.ConversationID != "" {
		t.Errorf("expected empty conversation_id, got %q", req.ConversationID)
	}
	// 缺失 conversation_id 时，json omitempty 和结构体零值兼容旧请求。
	if len(req.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(req.Messages))
	}
}

func TestChatRequest_ConversationID_Present(t *testing.T) {
	req := ChatRequest{
		ConversationID: "conv-abc-123",
		Messages:       []ChatMessage{{Role: "user", Content: "hello"}},
	}
	if req.ConversationID != "conv-abc-123" {
		t.Errorf("expected conv-abc-123, got %q", req.ConversationID)
	}
	if len(req.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(req.Messages))
	}
}
