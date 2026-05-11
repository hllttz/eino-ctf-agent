package service

import (
	"testing"

	appmodel "eino_ctf_agent/internal/model"
)

func TestNewTraceID_NonEmpty(t *testing.T) {
	id := newTraceID()
	if len(id) != 16 {
		t.Errorf("expected 16 hex chars, got %d: %q", len(id), id)
	}
}

func TestNewTraceID_Unique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for range 100 {
		id := newTraceID()
		if seen[id] {
			t.Errorf("duplicate trace ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestValidateChatRequest_Nil(t *testing.T) {
	err := validateChatRequest(nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

func TestValidateChatRequest_EmptyMessages(t *testing.T) {
	err := validateChatRequest(&appmodel.ChatRequest{})
	if err == nil {
		t.Error("expected error for empty messages")
	}
}

func TestValidateChatRequest_EmptyContent(t *testing.T) {
	// 使用 model 包的类型
	err := validateChatRequest(&appmodel.ChatRequest{
		Messages: []appmodel.ChatMessage{
			{Role: "user", Content: ""},
		},
	})
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestValidateChatRequest_Valid(t *testing.T) {
	err := validateChatRequest(&appmodel.ChatRequest{
		ConversationID: "conv-test",
		Messages: []appmodel.ChatMessage{
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
