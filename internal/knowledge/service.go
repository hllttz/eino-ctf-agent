package knowledge

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

	einoindexer "github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	goredis "github.com/redis/go-redis/v9"

	"eino_ctf_agent/internal/config"
	"eino_ctf_agent/internal/model"
)

type Service struct {
	cfg          *config.Config
	redisClient  *goredis.Client
	indexer      einoindexer.Indexer
	documentRepo *MetadataRepo
	indexLimiter chan struct{}
}

func NewService(cfg *config.Config, redisClient *goredis.Client, indexer einoindexer.Indexer, documentRepo *MetadataRepo) *Service {
	return &Service{
		cfg:          cfg,
		redisClient:  redisClient,
		indexer:      indexer,
		documentRepo: documentRepo,
		indexLimiter: make(chan struct{}, 2),
	}
}

func (s *Service) UploadMarkdown(ctx context.Context, filename string, reader io.Reader) (*model.Document, error) {
	if s.indexer == nil || s.redisClient == nil {
		return nil, fmt.Errorf("knowledge service is not fully initialized")
	}
	if !isMarkdownFilename(filename) {
		return nil, fmt.Errorf("only markdown files are supported")
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

func (s *Service) ListDocuments(ctx context.Context) ([]model.Document, error) {
	return s.documentRepo.List(ctx)
}

func (s *Service) DeleteDocument(ctx context.Context, id string) error {
	doc, err := s.documentRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.deleteDocumentChunks(ctx, id, doc.ChunkCount); err != nil {
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

func (s *Service) enqueueMarkdownIndex(doc *model.Document, content string) {
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

func (s *Service) indexMarkdown(ctx context.Context, doc *model.Document, content string) error {
	if err := s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusParsing, ""); err != nil {
		return err
	}
	blocks, err := ParseMarkdown(content)
	if err != nil {
		return err
	}

	if err := s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusChunking, ""); err != nil {
		return err
	}
	textSplitter, err := NewTextSplitter(s.cfg.RAG.ChunkSize, s.cfg.RAG.ChunkOverlap)
	if err != nil {
		return err
	}
	textChunks := textSplitter.SplitMarkdownBlocks(blocks)
	if len(textChunks) == 0 {
		return fmt.Errorf("markdown file produced no chunks")
	}

	if err := s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusEmbedding, ""); err != nil {
		return err
	}
	if err := s.deleteDocumentChunks(ctx, doc.ID, doc.ChunkCount); err != nil {
		return err
	}

	docs := make([]*schema.Document, 0, len(textChunks))
	for i, chunk := range textChunks {
		chunkID := fmt.Sprintf("%s:%d", doc.ID, i)
		docs = append(docs, &schema.Document{
			ID:      chunkID,
			Content: chunk.Content,
			MetaData: map[string]any{
				MetaDocumentID:  doc.ID,
				MetaFilename:    doc.Filename,
				MetaFileType:    doc.FileType,
				MetaChunkIndex:  i,
				MetaHeadingPath: chunk.HeadingPath,
				MetaPageNumber:  0,
				MetaSourcePath:  doc.SourcePath,
			},
		})
	}

	if _, err := s.indexer.Store(ctx, docs); err != nil {
		return err
	}
	if err := s.documentRepo.UpdateChunkCount(ctx, doc.ID, len(docs)); err != nil {
		return err
	}
	return s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusIndexed, "")
}

func (s *Service) deleteDocumentChunks(ctx context.Context, docID string, chunkCount int) error {
	keys := make(map[string]struct{}, chunkCount)
	for i := 0; i < chunkCount; i++ {
		keys[ChunkKey(s.cfg, fmt.Sprintf("%s:%d", docID, i))] = struct{}{}
	}

	iter := s.redisClient.Scan(ctx, 0, ChunkKeyPrefix(s.cfg)+docID+":*", 100).Iterator()
	for iter.Next(ctx) {
		keys[iter.Val()] = struct{}{}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("scan redis chunks for document %s: %w", docID, err)
	}
	if len(keys) == 0 {
		return nil
	}

	chunkKeys := make([]string, 0, len(keys))
	for key := range keys {
		chunkKeys = append(chunkKeys, key)
	}
	if err := s.redisClient.Del(ctx, chunkKeys...).Err(); err != nil {
		return fmt.Errorf("delete redis chunks for document %s: %w", docID, err)
	}
	return nil
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
