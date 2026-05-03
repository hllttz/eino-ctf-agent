package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Chunk struct {
	ID          string
	DocumentID  string
	Filename    string
	FileType    string
	ChunkIndex  int
	HeadingPath string
	Content     string
	PageNumber  int
}

type ChunkRepo struct {
	db *sql.DB
}

func NewChunkRepo(db *DB) *ChunkRepo {
	return &ChunkRepo{db: db.SQL}
}

func (r *ChunkRepo) CreateMany(ctx context.Context, chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin chunk transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks (
			id, document_id, filename, file_type, chunk_index,
			heading_path, content, page_number, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare chunk insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, chunk := range chunks {
		if _, err = stmt.ExecContext(
			ctx,
			chunk.ID,
			chunk.DocumentID,
			chunk.Filename,
			chunk.FileType,
			chunk.ChunkIndex,
			chunk.HeadingPath,
			chunk.Content,
			chunk.PageNumber,
			now,
		); err != nil {
			return fmt.Errorf("insert chunk %s: %w", chunk.ID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit chunk transaction: %w", err)
	}
	return nil
}

func (r *ChunkRepo) DeleteByDocumentID(ctx context.Context, documentID string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM chunks WHERE document_id = ?`, documentID); err != nil {
		return fmt.Errorf("delete chunks for document %s: %w", documentID, err)
	}
	return nil
}
