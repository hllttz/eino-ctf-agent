package vectorstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(ctx context.Context, path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create vector db dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open vector db: %w", err)
	}
	store := &SQLiteStore{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) Upsert(ctx context.Context, records []VectorRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin vector transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO vectors (
			id, document_id, chunk_id, filename, file_type, chunk_index,
			heading_path, content, page_number, embedding, dimension, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare vector upsert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, record := range records {
		if len(record.Embedding) == 0 {
			return fmt.Errorf("vector record %s has empty embedding", record.ID)
		}
		embeddingJSON, err := json.Marshal(record.Embedding)
		if err != nil {
			return fmt.Errorf("marshal vector %s: %w", record.ID, err)
		}
		if _, err = stmt.ExecContext(
			ctx,
			record.ID,
			record.DocumentID,
			record.ChunkID,
			record.Filename,
			record.FileType,
			record.ChunkIndex,
			record.HeadingPath,
			record.Content,
			record.PageNumber,
			string(embeddingJSON),
			len(record.Embedding),
			now,
		); err != nil {
			return fmt.Errorf("upsert vector %s: %w", record.ID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit vector transaction: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Search(ctx context.Context, query []float64, opts SearchOptions) ([]SearchResult, error) {
	if len(query) == 0 {
		return nil, fmt.Errorf("query vector is empty")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, document_id, chunk_id, filename, file_type, chunk_index,
		       heading_path, content, page_number, embedding
		FROM vectors
	`)
	if err != nil {
		return nil, fmt.Errorf("query vectors: %w", err)
	}
	defer rows.Close()

	records := make([]VectorRecord, 0)
	for rows.Next() {
		var record VectorRecord
		var embeddingJSON string
		if err := rows.Scan(
			&record.ID,
			&record.DocumentID,
			&record.ChunkID,
			&record.Filename,
			&record.FileType,
			&record.ChunkIndex,
			&record.HeadingPath,
			&record.Content,
			&record.PageNumber,
			&embeddingJSON,
		); err != nil {
			return nil, fmt.Errorf("scan vector: %w", err)
		}
		if err := json.Unmarshal([]byte(embeddingJSON), &record.Embedding); err != nil {
			return nil, fmt.Errorf("decode vector %s: %w", record.ID, err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate vectors: %w", err)
	}

	return RankRecords(records, query, opts), nil
}

func (s *SQLiteStore) DeleteByDocumentID(ctx context.Context, documentID string) error {
	if documentID == "" {
		return fmt.Errorf("document id is empty")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM vectors WHERE document_id = ?`, documentID); err != nil {
		return fmt.Errorf("delete vectors for document %s: %w", documentID, err)
	}
	return nil
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS vectors (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			chunk_id TEXT NOT NULL,
			filename TEXT NOT NULL,
			file_type TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			heading_path TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			page_number INTEGER NOT NULL DEFAULT 0,
			embedding TEXT NOT NULL,
			dimension INTEGER NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_vectors_document_id ON vectors(document_id);
		CREATE INDEX IF NOT EXISTS idx_vectors_file_type ON vectors(file_type);
	`); err != nil {
		return fmt.Errorf("migrate vector db: %w", err)
	}
	return nil
}
