package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	SQL *sql.DB
}

func Open(ctx context.Context, path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create metadata db dir: %w", err)
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open metadata db: %w", err)
	}
	db := &DB{SQL: sqlDB}
	if err := db.migrate(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	if db == nil || db.SQL == nil {
		return nil
	}
	return db.SQL.Close()
}

func (db *DB) migrate(ctx context.Context) error {
	if _, err := db.SQL.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if _, err := db.SQL.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			filename TEXT NOT NULL,
			file_type TEXT NOT NULL,
			source_path TEXT NOT NULL,
			status TEXT NOT NULL,
			chunk_count INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL,
			filename TEXT NOT NULL,
			file_type TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			heading_path TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			page_number INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			FOREIGN KEY(document_id) REFERENCES documents(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_chunks_document_id ON chunks(document_id);
	`); err != nil {
		return fmt.Errorf("migrate metadata db: %w", err)
	}
	return nil
}
