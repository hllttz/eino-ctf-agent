package model

const (
	DocumentStatusPending   = "pending"
	DocumentStatusParsing   = "parsing"
	DocumentStatusChunking  = "chunking"
	DocumentStatusEmbedding = "embedding"
	DocumentStatusIndexed   = "indexed"
	DocumentStatusFailed    = "failed"
)

// Document 文档元数据，记录上传文档的索引状态和基础信息。
type Document struct {
	ID           string `json:"id"`
	Filename     string `json:"filename"`
	FileType     string `json:"file_type"`
	SourcePath   string `json:"-"`
	Status       string `json:"status"`
	ChunkCount   int    `json:"chunk_count"`
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}
