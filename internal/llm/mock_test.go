package llm

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestMockStreamMultipleChunks(t *testing.T) {
	m := NewMockChatModel()
	ctx := context.Background()

	sr, err := m.Stream(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	defer sr.Close()

	var chunks []string
	start := time.Now()
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}
		chunks = append(chunks, chunk.Content)
	}
	elapsed := time.Since(start)

	// 至少 10 个 chunk
	if len(chunks) < 10 {
		t.Errorf("expected at least 10 chunks, got %d: %v", len(chunks), chunks)
	}

	// 总耗时大于 1 秒
	if elapsed < 1*time.Second {
		t.Errorf("expected at least 1s delay, got %v", elapsed)
	}

	// 拼接后内容不为空
	full := strings.Join(chunks, "")
	if len(full) == 0 {
		t.Error("expected non-empty concatenated content")
	}

	t.Logf("chunks=%d, elapsed=%v, len=%d", len(chunks), elapsed, len(full))
}

func TestMockStreamContextCancel(t *testing.T) {
	m := NewMockChatModel()
	ctx, cancel := context.WithCancel(context.Background())

	sr, err := m.Stream(ctx, []*schema.Message{
		schema.UserMessage("hello"),
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	defer sr.Close()

	// 读第一个 chunk 后立即取消
	_, err = sr.Recv()
	if err != nil {
		t.Fatalf("first Recv failed: %v", err)
	}
	cancel()

	// 继续读，期望在有限时间内收到 EOF
	deadline := time.After(2 * time.Second)
	chunkCount := 1
recvLoop:
	for {
		select {
		case <-deadline:
			t.Errorf("timeout waiting for stream to close after cancel, got %d chunks", chunkCount)
			return
		default:
		}
		_, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break recvLoop
		}
		if err != nil {
			t.Fatalf("Recv after cancel failed: %v", err)
		}
		chunkCount++
	}

	t.Logf("received %d chunks before stream closed after cancel", chunkCount)
}
