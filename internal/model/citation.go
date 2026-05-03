package model

type Citation struct {
	DocumentID string  `json:"document_id,omitempty"`
	Filename   string  `json:"filename"`
	ChunkIndex int     `json:"chunk_index"`
	Score      float64 `json:"score"`
	PageNumber int     `json:"page_number,omitempty"`
}
