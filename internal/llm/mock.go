package llm

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
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

// Generate 返回模拟回复消息。
func (m *MockChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("This is a mock response from the LLM.", nil), nil
}

// Stream 返回包含单条消息的流式读取器。
func (m *MockChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg := schema.AssistantMessage("This is a mock streaming response.", nil)
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

// WithTools 返回绑定工具后的新实例。
func (m *MockChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &MockChatModel{tools: tools}, nil
}
