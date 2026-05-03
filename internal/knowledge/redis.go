package knowledge

import (
	"context"
	"fmt"
	"strings"

	goredis "github.com/redis/go-redis/v9"

	"eino_ctf_agent/internal/config"
)

func NewRedisClient(cfg *config.Config) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:          cfg.Redis.Addr,
		Username:      cfg.Redis.Username,
		Password:      cfg.GetRedisPassword(),
		DB:            cfg.Redis.DB,
		Protocol:      2,
		UnstableResp3: true,
	})
}

func EnsureVectorIndex(ctx context.Context, client *goredis.Client, cfg *config.Config) error {
	if _, err := client.FTInfo(ctx, cfg.Redis.Index).Result(); err == nil {
		return nil
	} else if isRedisSearchUnavailable(err) {
		return fmt.Errorf("redis vector search requires Redis Stack or RediSearch module: %w", err)
	} else if !isMissingIndex(err) {
		return fmt.Errorf("inspect redis index %s: %w", cfg.Redis.Index, err)
	}

	_, err := client.FTCreate(
		ctx,
		cfg.Redis.Index,
		&goredis.FTCreateOptions{
			OnHash: true,
			Prefix: []interface{}{
				ChunkKeyPrefix(cfg),
			},
		},
		&goredis.FieldSchema{FieldName: RedisContentField, FieldType: goredis.SearchFieldTypeText},
		&goredis.FieldSchema{FieldName: MetaDocumentID, FieldType: goredis.SearchFieldTypeTag},
		&goredis.FieldSchema{FieldName: MetaFilename, FieldType: goredis.SearchFieldTypeText},
		&goredis.FieldSchema{FieldName: MetaFileType, FieldType: goredis.SearchFieldTypeTag},
		&goredis.FieldSchema{FieldName: MetaChunkIndex, FieldType: goredis.SearchFieldTypeNumeric, Sortable: true},
		&goredis.FieldSchema{FieldName: MetaHeadingPath, FieldType: goredis.SearchFieldTypeText},
		&goredis.FieldSchema{FieldName: MetaPageNumber, FieldType: goredis.SearchFieldTypeNumeric},
		&goredis.FieldSchema{
			FieldName: VectorField(cfg),
			FieldType: goredis.SearchFieldTypeVector,
			VectorArgs: &goredis.FTVectorArgs{
				HNSWOptions: &goredis.FTHNSWOptions{
					Type:           "FLOAT32",
					Dim:            cfg.Embedding.Dimension,
					DistanceMetric: "COSINE",
				},
			},
		},
	).Result()
	if err != nil {
		if isIndexAlreadyExists(err) {
			return nil
		}
		if isRedisSearchUnavailable(err) {
			return fmt.Errorf("redis vector search requires Redis Stack or RediSearch module: %w", err)
		}
		return fmt.Errorf("create redis vector index %s: %w", cfg.Redis.Index, err)
	}
	return nil
}

func isMissingIndex(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown index") ||
		strings.Contains(msg, "no such index") ||
		strings.Contains(msg, "index does not exist")
}

func isIndexAlreadyExists(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "index already exists") ||
		strings.Contains(msg, "already exists")
}

func isRedisSearchUnavailable(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "module") && strings.Contains(msg, "search")
}
