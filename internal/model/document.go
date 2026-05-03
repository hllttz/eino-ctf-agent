package model

// Document 表示知识库中的一个文档元数据。
// Phase 4A 实现。
type Document struct {
	ID           string `json:"id"`
	Filename     string `json:"filename"`
	FileType     string `json:"file_type"`     // markdown, pdf
	Status       string `json:"status"`        // pending, parsing, chunking, embedding, indexed, failed
	ChunkCount   int    `json:"chunk_count"`
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}
