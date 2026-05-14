package embedding

import (
	"context"
	"math"

	einoembedding "github.com/cloudwego/eino/components/embedding"

	"eino_ctf_agent/internal/config"
)

// MockEmbedder 用于单元测试和本地开发的模拟嵌入模型。
// 使用稳定哈希为每个文本生成固定维度向量，不依赖外部API。
type MockEmbedder struct {
	dimension int
}

// compile-time interface check
var _ Embedder = (*MockEmbedder)(nil)

// NewMockEmbedder 创建 MockEmbedder。
func NewMockEmbedder(cfg *config.Config) *MockEmbedder {
	dim := cfg.Embedding.Dimension
	if dim <= 0 {
		dim = 1024
	}
	return &MockEmbedder{dimension: dim}
}

// EmbedStrings 为每个输入文本生成固定维度的模拟向量。
// 同一文本多次调用返回相同向量（确定性）。
func (e *MockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...einoembedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i, text := range texts {
		result[i] = e.mockEmbed(text)
	}
	return result, nil
}

// mockEmbed 对单个文本生成固定维度向量。
// 基于文本哈希生成确定性向量，值在[-1,1]区间。
func (e *MockEmbedder) mockEmbed(text string) []float64 {
	h := hashString(text)
	vec := make([]float64, e.dimension)
	for i := range vec {
		h = h*1103515245 + 12345
		vec[i] = math.Sin(float64(h)) * 0.5
	}
	return vec
}

// hashString 计算字符串的简单64位哈希（FNV-1a变体）。
func hashString(s string) uint64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}
