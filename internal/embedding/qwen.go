package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"eino_ctf_agent/internal/config"
)

type QwenEmbedder struct {
	apiKey    string
	baseURL   string
	model     string
	dimension int
	batchSize int
	client    *http.Client
}

func NewQwenEmbedder(cfg *config.Config) (*QwenEmbedder, error) {
	apiKey := cfg.GetEmbeddingAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("embedding API key not set: environment variable %s is empty", cfg.Embedding.APIKeyEnv)
	}
	batchSize := cfg.Embedding.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}
	return &QwenEmbedder{
		apiKey:    apiKey,
		baseURL:   strings.TrimRight(cfg.Embedding.BaseURL, "/"),
		model:     cfg.Embedding.Model,
		dimension: cfg.Embedding.Dimension,
		batchSize: batchSize,
		client:    &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (e *QwenEmbedder) Dimension() int {
	return e.dimension
}

func (e *QwenEmbedder) EmbedStrings(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("embedding input is empty")
	}

	all := make([][]float64, 0, len(texts))
	for start := 0; start < len(texts); start += e.batchSize {
		end := start + e.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		vectors, err := e.embedBatch(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		all = append(all, vectors...)
	}
	return all, nil
}

func (e *QwenEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("embedding input at index %d is empty", i)
		}
	}

	payload := embeddingRequest{
		Model: e.model,
		Input: texts,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding API returned %d: %s", resp.StatusCode, compactBody(respBody))
	}

	var decoded embeddingResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(decoded.Data) != len(texts) {
		return nil, fmt.Errorf("embedding response count mismatch: got %d, want %d", len(decoded.Data), len(texts))
	}

	vectors := make([][]float64, len(texts))
	for _, item := range decoded.Data {
		if item.Index < 0 || item.Index >= len(texts) {
			return nil, fmt.Errorf("embedding response contains invalid index %d", item.Index)
		}
		if e.dimension > 0 && len(item.Embedding) != e.dimension {
			return nil, fmt.Errorf("embedding dimension mismatch: got %d, want %d", len(item.Embedding), e.dimension)
		}
		vectors[item.Index] = item.Embedding
	}
	return vectors, nil
}

func compactBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		return text[:500] + "..."
	}
	return text
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}
