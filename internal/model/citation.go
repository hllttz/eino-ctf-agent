package model

// Citation 检索结果引用，记录检索命中片段的来源和相似度。
type Citation struct {
	DocumentID string  `json:"document_id,omitempty"`
	Filename   string  `json:"filename"`
	ChunkIndex int     `json:"chunk_index"`
	Score      float64 `json:"score"`
	PageNumber int     `json:"page_number,omitempty"`
}
