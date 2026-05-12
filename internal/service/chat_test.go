package service

import (
	"context"
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

func TestContextWithTraceID_Present(t *testing.T) {
	ctx := ContextWithTraceID(context.Background(), "req-hdr-abc123")
	got, ok := TraceIDFromContext(ctx)
	if !ok {
		t.Fatal("expected trace ID to be present in context")
	}
	if got != "req-hdr-abc123" {
		t.Errorf("expected req-hdr-abc123, got %q", got)
	}
}

func TestTraceIDFromContext_Missing(t *testing.T) {
	_, ok := TraceIDFromContext(context.Background())
	if ok {
		t.Error("expected no trace ID in empty context")
	}
}

func TestTraceID_HeaderFlow(t *testing.T) {
	// 模拟 handler → service 流转：header 存在时透传，缺失时生成
	ctx := ContextWithTraceID(context.Background(), "x-req-456")
	id1, ok := TraceIDFromContext(ctx)
	if !ok || id1 != "x-req-456" {
		t.Fatalf("context trace ID mismatch: got=%q ok=%v", id1, ok)
	}

	// 缺失 header 时 newTraceID 生成
	id2 := newTraceID()
	if len(id2) != 16 {
		t.Errorf("generated trace ID wrong length: %d", len(id2))
	}

	// 两次生成应不同
	id3 := newTraceID()
	if id2 == id3 {
		t.Errorf("two generated trace IDs should differ")
	}
}
