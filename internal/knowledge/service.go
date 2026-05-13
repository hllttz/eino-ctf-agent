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

// Service 知识库服务，管理文档上传、异步索引和删除的全生命周期。
type Service struct {
	cfg          *config.Config
	redisClient  *goredis.Client
	indexer      einoindexer.Indexer
	documentRepo *MetadataRepo
	// 并发索引控制信号量（缓冲通道容量2），限制同时执行embedding的文档数。
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
	if !isAllowedFilename(filename) {
		return nil, fmt.Errorf("unsupported file type: %s (allowed: .md, .markdown, .pdf)", filepath.Ext(filename))
	}

	content, err := io.ReadAll(io.LimitReader(reader, 20<<20))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return nil, fmt.Errorf("file is empty")
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}

	fileType := "markdown"
	if isPDFFilename(filename) {
		fileType = "pdf"
	}

	storageDir := filepath.Join(s.cfg.Storage.DocsDir, fileType)
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s dir: %w", fileType, err)
	}

	safeName := sanitizeFilename(filename)
	sourcePath := filepath.Join(storageDir, id+"_"+safeName)
	if err := os.WriteFile(sourcePath, content, 0o644); err != nil {
		return nil, fmt.Errorf("save file: %w", err)
	}

	doc := &model.Document{
		ID:         id,
		Filename:   filepath.Base(filename),
		FileType:   fileType,
		SourcePath: sourcePath,
		Status:     model.DocumentStatusPending,
	}
	if err := s.documentRepo.Create(ctx, doc); err != nil {
		return nil, err
	}

	if fileType == "pdf" {
		s.enqueuePDFIndex(doc, string(content))
	} else {
		s.enqueueMarkdownIndex(doc, string(content))
	}
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

// enqueuePDFIndex 将 PDF 文档放入后台索引队列。
func (s *Service) enqueuePDFIndex(doc *model.Document, content string) {
	go func() {
		s.indexLimiter <- struct{}{}
		defer func() { <-s.indexLimiter }()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := s.indexPDF(ctx, doc, content); err != nil {
			failMsg := err.Error()
			_ = s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusFailed, failMsg)
		}
	}()
}

// indexPDF 将 PDF 文本按页解析、复用现有 splitter 切块、embedding → Redis index。
// 每页文本先作为独立单元交给 TextSplitter，切出的 chunk 继承 page_number metadata。
func (s *Service) indexPDF(ctx context.Context, doc *model.Document, content string) error {
	if err := s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusParsing, ""); err != nil {
		return err
	}
	pages, err := ParsePDF(strings.NewReader(content))
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

	// 每页文本作为独立 MarkdownBlock 交给 splitter，保留 page_number。
	// page_number 通过 HeadingPath 字段传递（"Page N"），复用现有 splitter/chunk 流程。
	// 后续可改为显式 metadata 传递，避免解析 HeadingPath。
	var allChunks []TextChunk
	for _, page := range pages {
		pageChunks := textSplitter.splitText("", page.Text)
		for i := range pageChunks {
			pageChunks[i].HeadingPath = fmt.Sprintf("Page %d", page.PageNumber)
		}
		allChunks = append(allChunks, pageChunks...)
	}
	if len(allChunks) == 0 {
		return fmt.Errorf("pdf file produced no chunks")
	}

	if err := s.documentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusEmbedding, ""); err != nil {
		return err
	}
	if err := s.deleteDocumentChunks(ctx, doc.ID, doc.ChunkCount); err != nil {
		return err
	}

	docs := make([]*schema.Document, 0, len(allChunks))
	for i, chunk := range allChunks {
		chunkID := fmt.Sprintf("%s:%d", doc.ID, i)
		pageNum := 0
		// 从 HeadingPath "Page N" 中提取页码
		if n, parseErr := fmt.Sscanf(chunk.HeadingPath, "Page %d", &pageNum); n != 1 || parseErr != nil {
			pageNum = 0
		}
		docs = append(docs, &schema.Document{
			ID:      chunkID,
			Content: chunk.Content,
			MetaData: map[string]any{
				MetaDocumentID:  doc.ID,
				MetaFilename:    doc.Filename,
				MetaFileType:    doc.FileType,
				MetaChunkIndex:  i,
				MetaHeadingPath: chunk.HeadingPath,
				MetaPageNumber:  pageNum,
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

// isAllowedFilename 检查上传文件是否为支持的类型（Markdown 或 PDF）。
func isAllowedFilename(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".md" || ext == ".markdown" || ext == ".pdf"
}

func isPDFFilename(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".pdf"
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
