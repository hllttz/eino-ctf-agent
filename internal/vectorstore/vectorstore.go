package vectorstore

import "context"

type VectorStore interface {
	Upsert(ctx context.Context, records []VectorRecord) error
	Search(ctx context.Context, query []float64, opts SearchOptions) ([]SearchResult, error)
	DeleteByDocumentID(ctx context.Context, documentID string) error
	Close() error
}

type VectorRecord struct {
	ID          string
	DocumentID  string
	ChunkID     string
	Filename    string
	FileType    string
	ChunkIndex  int
	HeadingPath string
	Content     string
	PageNumber  int
	Embedding   []float64
}

type SearchResult struct {
	Record VectorRecord
	Score  float64
}
