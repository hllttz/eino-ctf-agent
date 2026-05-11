package knowledge

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/cloudwego/eino/schema"
	goredis "github.com/redis/go-redis/v9"

	"eino_ctf_agent/internal/config"
	"eino_ctf_agent/internal/model"
)

const (
	RedisContentField = "content"
	RedisVectorField  = "vector_content"

	MetaDocumentID  = "document_id"
	MetaFilename    = "filename"
	MetaFileType    = "file_type"
	MetaChunkIndex  = "chunk_index"
	MetaHeadingPath = "heading_path"
	MetaPageNumber  = "page_number"
	MetaSourcePath  = "source_path"
)

const (
	docFieldID           = "id"
	docFieldFilename     = "filename"
	docFieldFileType     = "file_type"
	docFieldSourcePath   = "source_path"
	docFieldStatus       = "status"
	docFieldChunkCount   = "chunk_count"
	docFieldErrorMessage = "error_message"
	docFieldCreatedAt    = "created_at"
	docFieldUpdatedAt    = "updated_at"
)

var ErrDocumentNotFound = errors.New("document not found")

// MetadataRepo 文档元数据仓库，基于Redis Hash存储文档的元信息。
type MetadataRepo struct {
	client *goredis.Client
	cfg    *config.Config
}

func NewMetadataRepo(client *goredis.Client, cfg *config.Config) *MetadataRepo {
	return &MetadataRepo{client: client, cfg: cfg}
}

func (r *MetadataRepo) Create(ctx context.Context, doc *model.Document) error {
	now := time.Now().UTC().Format(time.RFC3339)
	doc.CreatedAt = now
	doc.UpdatedAt = now
	if doc.Status == "" {
		doc.Status = model.DocumentStatusPending
	}

	pipe := r.client.Pipeline()
	pipe.HSet(ctx, DocumentKey(r.cfg, doc.ID), documentHash(doc)...)
	pipe.SAdd(ctx, DocumentSetKey(r.cfg), doc.ID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("create document metadata %s: %w", doc.ID, err)
	}
	return nil
}

func (r *MetadataRepo) Get(ctx context.Context, id string) (*model.Document, error) {
	values, err := r.client.HGetAll(ctx, DocumentKey(r.cfg, id)).Result()
	if err != nil {
		return nil, fmt.Errorf("get document metadata %s: %w", id, err)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrDocumentNotFound, id)
	}
	return documentFromHash(values), nil
}

func (r *MetadataRepo) List(ctx context.Context) ([]model.Document, error) {
	ids, err := r.client.SMembers(ctx, DocumentSetKey(r.cfg)).Result()
	if err != nil {
		return nil, fmt.Errorf("list document ids: %w", err)
	}

	docs := make([]model.Document, 0, len(ids))
	for _, id := range ids {
		doc, err := r.Get(ctx, id)
		if err != nil {
			if errors.Is(err, ErrDocumentNotFound) {
				continue
			}
			return nil, err
		}
		docs = append(docs, *doc)
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].CreatedAt > docs[j].CreatedAt
	})
	return docs, nil
}

func (r *MetadataRepo) UpdateStatus(ctx context.Context, id, status, errorMessage string) error {
	if err := r.client.HSet(ctx, DocumentKey(r.cfg, id),
		docFieldStatus, status,
		docFieldErrorMessage, errorMessage,
		docFieldUpdatedAt, time.Now().UTC().Format(time.RFC3339),
	).Err(); err != nil {
		return fmt.Errorf("update document status %s: %w", id, err)
	}
	return nil
}

func (r *MetadataRepo) UpdateChunkCount(ctx context.Context, id string, chunkCount int) error {
	if err := r.client.HSet(ctx, DocumentKey(r.cfg, id),
		docFieldChunkCount, chunkCount,
		docFieldUpdatedAt, time.Now().UTC().Format(time.RFC3339),
	).Err(); err != nil {
		return fmt.Errorf("update document chunk count %s: %w", id, err)
	}
	return nil
}

func (r *MetadataRepo) Delete(ctx context.Context, id string) error {
	pipe := r.client.Pipeline()
	pipe.SRem(ctx, DocumentSetKey(r.cfg), id)
	pipe.Del(ctx, DocumentKey(r.cfg, id))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("delete document metadata %s: %w", id, err)
	}
	return nil
}

func MetadataString(doc *schema.Document, key string) string {
	if doc == nil || doc.MetaData == nil {
		return ""
	}
	value, ok := doc.MetaData[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func MetadataInt(doc *schema.Document, key string) int {
	if doc == nil || doc.MetaData == nil {
		return 0
	}
	switch value := doc.MetaData[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		n, err := parseInt(value)
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

func documentHash(doc *model.Document) []any {
	return []any{
		docFieldID, doc.ID,
		docFieldFilename, doc.Filename,
		docFieldFileType, doc.FileType,
		docFieldSourcePath, doc.SourcePath,
		docFieldStatus, doc.Status,
		docFieldChunkCount, doc.ChunkCount,
		docFieldErrorMessage, doc.ErrorMessage,
		docFieldCreatedAt, doc.CreatedAt,
		docFieldUpdatedAt, doc.UpdatedAt,
	}
}

func documentFromHash(values map[string]string) *model.Document {
	chunkCount, _ := parseInt(values[docFieldChunkCount])
	return &model.Document{
		ID:           values[docFieldID],
		Filename:     values[docFieldFilename],
		FileType:     values[docFieldFileType],
		SourcePath:   values[docFieldSourcePath],
		Status:       values[docFieldStatus],
		ChunkCount:   chunkCount,
		ErrorMessage: values[docFieldErrorMessage],
		CreatedAt:    values[docFieldCreatedAt],
		UpdatedAt:    values[docFieldUpdatedAt],
	}
}

func parseInt(value string) (int, error) {
	return strconv.Atoi(value)
}
