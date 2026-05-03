package knowledge

import (
	"context"
	"fmt"
	"math"
	"strconv"

	redisindexer "github.com/cloudwego/eino-ext/components/indexer/redis"
	redisretriever "github.com/cloudwego/eino-ext/components/retriever/redis"
	einoembedding "github.com/cloudwego/eino/components/embedding"
	einoindexer "github.com/cloudwego/eino/components/indexer"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	goredis "github.com/redis/go-redis/v9"

	"eino_ctf_agent/internal/config"
)

func NewRedisIndexer(ctx context.Context, client *goredis.Client, cfg *config.Config, embedder einoembedding.Embedder) (einoindexer.Indexer, error) {
	return redisindexer.NewIndexer(ctx, &redisindexer.IndexerConfig{
		Client:           client,
		KeyPrefix:        ChunkKeyPrefix(cfg),
		DocumentToHashes: documentToHashes(cfg),
		BatchSize:        cfg.Embedding.BatchSize,
		Embedding:        embedder,
	})
}

func NewRedisRetriever(ctx context.Context, client *goredis.Client, cfg *config.Config, embedder einoembedding.Embedder) (einoretriever.Retriever, error) {
	return redisretriever.NewRetriever(ctx, &redisretriever.RetrieverConfig{
		Client:            client,
		Index:             cfg.Redis.Index,
		VectorField:       VectorField(cfg),
		DistanceThreshold: redisDistanceThreshold(cfg),
		Dialect:           cfg.Redis.Dialect,
		ReturnFields: []string{
			RedisContentField,
			MetaDocumentID,
			MetaFilename,
			MetaFileType,
			MetaChunkIndex,
			MetaHeadingPath,
			MetaPageNumber,
			redisretriever.SortByDistanceAttributeName,
		},
		DocumentConverter: RedisDocumentToSchema,
		TopK:              cfg.RAG.TopK,
		Embedding:         embedder,
	})
}

func RedisDocumentToSchema(ctx context.Context, raw goredis.Document) (*schema.Document, error) {
	content, ok := raw.Fields[RedisContentField]
	if !ok {
		return nil, fmt.Errorf("redis document %s missing %s field", raw.ID, RedisContentField)
	}

	doc := &schema.Document{
		ID:      raw.ID,
		Content: content,
		MetaData: map[string]any{
			MetaDocumentID:  raw.Fields[MetaDocumentID],
			MetaFilename:    raw.Fields[MetaFilename],
			MetaFileType:    raw.Fields[MetaFileType],
			MetaChunkIndex:  parseInt(raw.Fields[MetaChunkIndex]),
			MetaHeadingPath: raw.Fields[MetaHeadingPath],
			MetaPageNumber:  parseInt(raw.Fields[MetaPageNumber]),
		},
	}

	if distance, ok := raw.Fields[redisretriever.SortByDistanceAttributeName]; ok {
		if score, err := scoreFromDistance(distance); err == nil {
			doc.WithScore(score)
		}
	}
	return doc, nil
}

func documentToHashes(cfg *config.Config) func(ctx context.Context, doc *schema.Document) (*redisindexer.Hashes, error) {
	return func(ctx context.Context, doc *schema.Document) (*redisindexer.Hashes, error) {
		if doc.ID == "" {
			return nil, fmt.Errorf("document id is required for redis indexer")
		}

		vectorField := VectorField(cfg)
		fields := map[string]redisindexer.FieldValue{
			RedisContentField: {
				Value:    doc.Content,
				EmbedKey: vectorField,
			},
		}
		for key, value := range doc.MetaData {
			if key == RedisContentField || key == vectorField {
				continue
			}
			fields[key] = redisindexer.FieldValue{Value: value}
		}

		return &redisindexer.Hashes{
			Key:         doc.ID,
			Field2Value: fields,
		}, nil
	}
}

func redisDistanceThreshold(cfg *config.Config) *float64 {
	if cfg.Redis.DistanceThreshold <= 0 {
		return nil
	}
	return &cfg.Redis.DistanceThreshold
}

func scoreFromDistance(value string) (float64, error) {
	distance, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	score := 1 - distance
	return math.Max(0, math.Min(1, score)), nil
}
