package model

// Citation 表示 RAG 回答中的引用来源。
// Phase 5 实现。
type Citation struct {
	Filename   string  `json:"filename"`
	ChunkIndex int     `json:"chunk_index"`
	Score      float64 `json:"score"`
	PageNumber int     `json:"page_number,omitempty"` // PDF 专用
}
