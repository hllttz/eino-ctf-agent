package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"eino_ctf_agent/internal/model"
)

type DocumentRepo struct {
	db *sql.DB
}

func NewDocumentRepo(db *DB) *DocumentRepo {
	return &DocumentRepo{db: db.SQL}
}

func (r *DocumentRepo) Create(ctx context.Context, doc *model.Document) error {
	now := time.Now().UTC().Format(time.RFC3339)
	doc.CreatedAt = now
	doc.UpdatedAt = now
	if doc.Status == "" {
		doc.Status = model.DocumentStatusPending
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO documents (
			id, filename, file_type, source_path, status, chunk_count,
			error_message, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, doc.ID, doc.Filename, doc.FileType, doc.SourcePath, doc.Status, doc.ChunkCount, doc.ErrorMessage, doc.CreatedAt, doc.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create document %s: %w", doc.ID, err)
	}
	return nil
}

func (r *DocumentRepo) Get(ctx context.Context, id string) (*model.Document, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, filename, file_type, source_path, status, chunk_count,
		       error_message, created_at, updated_at
		FROM documents WHERE id = ?
	`, id)
	doc := &model.Document{}
	if err := row.Scan(
		&doc.ID,
		&doc.Filename,
		&doc.FileType,
		&doc.SourcePath,
		&doc.Status,
		&doc.ChunkCount,
		&doc.ErrorMessage,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document not found: %s", id)
		}
		return nil, fmt.Errorf("get document %s: %w", id, err)
	}
	return doc, nil
}

func (r *DocumentRepo) List(ctx context.Context) ([]model.Document, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, filename, file_type, source_path, status, chunk_count,
		       error_message, created_at, updated_at
		FROM documents
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	docs := make([]model.Document, 0)
	for rows.Next() {
		var doc model.Document
		if err := rows.Scan(
			&doc.ID,
			&doc.Filename,
			&doc.FileType,
			&doc.SourcePath,
			&doc.Status,
			&doc.ChunkCount,
			&doc.ErrorMessage,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate documents: %w", err)
	}
	return docs, nil
}

func (r *DocumentRepo) UpdateStatus(ctx context.Context, id, status, errorMessage string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE documents
		SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`, status, errorMessage, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update document status %s: %w", id, err)
	}
	return nil
}

func (r *DocumentRepo) UpdateChunkCount(ctx context.Context, id string, chunkCount int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE documents
		SET chunk_count = ?, updated_at = ?
		WHERE id = ?
	`, chunkCount, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update document chunk count %s: %w", id, err)
	}
	return nil
}

func (r *DocumentRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM documents WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete document %s: %w", id, err)
	}
	return nil
}
