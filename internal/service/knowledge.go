package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"eino_ctf_agent/internal/config"
	"eino_ctf_agent/internal/embedding"
	"eino_ctf_agent/internal/model"
	"eino_ctf_agent/internal/parser"
	"eino_ctf_agent/internal/splitter"
	"eino_ctf_agent/internal/store"
	"eino_ctf_agent/internal/vectorstore"
)

type KnowledgeService struct {
	cfg          *config.Config
	embedder     embedding.Embedder
	vectorStore  vectorstore.VectorStore
	documentRepo *store.DocumentRepo
	chunkRepo    *store.ChunkRepo
	indexLimiter chan struct{}
}

func NewKnowledgeService(
	cfg *config.Config,
	embedder embedding.Embedder,
	vectorStore vectorstore.VectorStore,
	documentRepo *store.DocumentRepo,
	chunkRepo *store.ChunkRepo,
) *KnowledgeService {
	return &KnowledgeService{
		cfg:          cfg,
		embedder:     embedder,
		vectorStore:  vectorStore,
		documentRepo: documentRepo,
		chunkRepo:    chunkRepo,
		indexLimiter: make(chan struct{}, 2),
	}
}

func (s *KnowledgeService) UploadMarkdown(ctx context.Context, filename string, reader io.Reader) (*model.Document, error) {
	if s.embedder == nil || s.vectorStore == nil {
		return nil, fmt.Errorf("knowledge service is not fully initialized")
	}
	if !isMarkdownFilename(filename) {
		return nil, fmt.Errorf("only markdown files are supported in MVP")
	}

	content, err := io.ReadAll(io.LimitReader(reader, 20<<20))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return nil, fmt.Errorf("markdown file is empty")
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}
	markdownDir := filepath.Join(s.cfg.Storage.DocsDir, "markdown")
	if err := os.MkdirAll(markdownDir, 0o755); err != nil {
		return nil, fmt.Errorf("create markdown dir: %w", err)
	}

	safeName := sanitizeFilename(filename)
	sourcePath := filepath.Join(markdownDir, id+"_"+safeName)
	if err := os.WriteFile(sourcePath, content, 0o644); err != nil {
		return nil, fmt.Errorf("save markdown file: %w", err)
	}

	doc := &model.Document{
		ID:         id,
		Filename:   filepath.Base(filename),
		FileType:   "markdown",
		SourcePath: sourcePath,
		Status:     model.DocumentStatusPending,
	}
	if err := s.documentRepo.Create(ctx, doc); err != nil {
		return nil, err
	}

	s.enqueueMarkdownIndex(doc, string(content))
	return doc, nil
}

func (s *KnowledgeService) ListDocuments(ctx context.Context) ([]model.Document, error) {
	return s.documentRepo.List(ctx)
}

func (s *KnowledgeService) DeleteDocument(ctx context.Context, id string) error {
	doc, err := s.documentRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.vectorStore.DeleteByDocumentID(ctx, id); err != nil {
		return err
	}
	if err := s.chunkRepo.DeleteByDocumentID(ctx, id); err != nil {
		return err
	}
	if err := s.documentRepo.Delete(ctx, id); err != nil {
		return err
	}
	if doc.SourcePath != "" {
		if err := os.Remove(doc.SourcePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove source file: %w", err)
		}
	}
	return nil
}

func (s *KnowledgeService) enqueueMarkdownIndex(doc *model.Document, content string) {
	go func() {
		s.indexLimiter <- struct{}{}
		defer func() { <-s.indexLimiter }()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := s.indexMarkdown(ctx, doc, content); err != nil {
			failMsg := err.Error()
			_ = s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusFailed, failMsg)
		}
	}()
}

func (s *KnowledgeService) indexMarkdown(ctx context.Context, doc *model.Document, content string) error {
	if err := s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusParsing, ""); err != nil {
		return err
	}
	blocks, err := parser.ParseMarkdown(content)
	if err != nil {
		return err
	}

	if err := s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusChunking, ""); err != nil {
		return err
	}
	textSplitter, err := splitter.NewTextSplitter(s.cfg.RAG.ChunkSize, s.cfg.RAG.ChunkOverlap)
	if err != nil {
		return err
	}
	textChunks := textSplitter.SplitMarkdownBlocks(blocks)
	if len(textChunks) == 0 {
		return fmt.Errorf("markdown file produced no chunks")
	}

	texts := make([]string, len(textChunks))
	for i, chunk := range textChunks {
		texts[i] = chunk.Content
	}

	if err := s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusEmbedding, ""); err != nil {
		return err
	}
	vectors, err := s.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return err
	}
	if len(vectors) != len(textChunks) {
		return fmt.Errorf("embedding count mismatch: got %d, want %d", len(vectors), len(textChunks))
	}

	chunks := make([]store.Chunk, len(textChunks))
	records := make([]vectorstore.VectorRecord, len(textChunks))
	for i, chunk := range textChunks {
		chunkID := fmt.Sprintf("%s:%d", doc.ID, i)
		chunks[i] = store.Chunk{
			ID:          chunkID,
			DocumentID:  doc.ID,
			Filename:    doc.Filename,
			FileType:    doc.FileType,
			ChunkIndex:  i,
			HeadingPath: chunk.HeadingPath,
			Content:     chunk.Content,
		}
		records[i] = vectorstore.VectorRecord{
			ID:          chunkID,
			DocumentID:  doc.ID,
			ChunkID:     chunkID,
			Filename:    doc.Filename,
			FileType:    doc.FileType,
			ChunkIndex:  i,
			HeadingPath: chunk.HeadingPath,
			Content:     chunk.Content,
			Embedding:   vectors[i],
		}
	}

	if err := s.chunkRepo.CreateMany(ctx, chunks); err != nil {
		return err
	}
	if err := s.vectorStore.Upsert(ctx, records); err != nil {
		return err
	}
	if err := s.documentRepo.UpdateChunkCount(ctx, doc.ID, len(chunks)); err != nil {
		return err
	}
	return s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusIndexed, "")
}

func isMarkdownFilename(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".md" || ext == ".markdown"
}

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeFilename(filename string) string {
	name := filepath.Base(filename)
	name = unsafeFilenameChars.ReplaceAllString(name, "_")
	name = strings.Trim(name, "._")
	if name == "" {
		return "document.md"
	}
	return name
}

func newID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}
