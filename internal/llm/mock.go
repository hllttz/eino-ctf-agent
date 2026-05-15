package llm

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// mock 流式输出参数：每段 1~3 个词，段间延迟 120~250ms。
// 长文本 + 小分段 + 足够延迟确保浏览器端可观察到逐段流式效果。
const (
	mockChunkWordsMin = 1
	mockChunkWordsMax = 3
	mockDelayMin      = 120 * time.Millisecond
	mockDelayMax      = 250 * time.Millisecond
)

// MockChatModel 用于单元测试和本地开发的模拟聊天模型。
// 不依赖外部API，返回可预测的回复文本。
type MockChatModel struct {
	tools []*schema.ToolInfo
}

// compile-time interface check
var (
	_ model.BaseChatModel        = (*MockChatModel)(nil)
	_ model.ToolCallingChatModel = (*MockChatModel)(nil)
)

// NewMockChatModel 创建 MockChatModel。
func NewMockChatModel() *MockChatModel {
	return &MockChatModel{}
}

// Generate 返回模拟回复消息（非流式）。
func (m *MockChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage(mockFullText(), nil), nil
}

// Stream 返回模拟流式输出：将长文本拆成 20+ 个小段，段间加延迟。
// 使用 schema.Pipe 实现真流式，前端可观察到逐段出现的效果。
func (m *MockChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	words := strings.Fields(mockFullText())
	sr, sw := schema.Pipe[*schema.Message](len(words))

	go func() {
		defer sw.Close()

		i := 0
		chunkIdx := 0
		for i < len(words) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// 每段取 1~3 个词（最后一段可能更少）
			n := mockChunkWordsMin + (chunkIdx*7+3)%(mockChunkWordsMax-mockChunkWordsMin+1)
			end := i + n
			if end > len(words) {
				end = len(words)
			}

			chunk := strings.Join(words[i:end], " ")
			// 非最后一段追加空格以保持阅读连贯
			if end < len(words) {
				chunk += " "
			}

			sw.Send(schema.AssistantMessage(chunk, nil), nil)

			// 段间延迟：120~250ms，不同段延迟不同
			delay := mockDelayMin + time.Duration((chunkIdx*31+11)%131)*time.Millisecond
			time.Sleep(delay)

			i = end
			chunkIdx++
		}
	}()

	return sr, nil
}

// WithTools 返回绑定工具后的新实例。
func (m *MockChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &MockChatModel{tools: tools}, nil
}

// mockFullText 返回 200+ 字符的模拟回复文本。
// 拆分成小段后在浏览器端可观察到明显流式效果。
func mockFullText() string {
	return "Hello! I'm a mock language model response designed to demonstrate " +
		"real-time streaming output in the browser. Each chunk of text arrives " +
		"with a small delay, simulating how a large language model generates " +
		"tokens one by one during inference. This makes the conversation feel " +
		"more natural and interactive, rather than waiting for the entire " +
		"response to complete before displaying anything to the user."
}
