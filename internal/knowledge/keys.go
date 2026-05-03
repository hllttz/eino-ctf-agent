package knowledge

import (
	"strings"

	"eino_ctf_agent/internal/config"
)

func KeyPrefix(cfg *config.Config) string {
	prefix := cfg.Redis.KeyPrefix
	if prefix == "" {
		prefix = "eino_ctf_agent:"
	}
	if !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}
	return prefix
}

func DocumentSetKey(cfg *config.Config) string {
	return KeyPrefix(cfg) + "documents"
}

func DocumentKey(cfg *config.Config, id string) string {
	return KeyPrefix(cfg) + "doc:" + id
}

func ChunkKeyPrefix(cfg *config.Config) string {
	return KeyPrefix(cfg) + "chunk:"
}

func ChunkKey(cfg *config.Config, chunkID string) string {
	return ChunkKeyPrefix(cfg) + chunkID
}

func VectorField(cfg *config.Config) string {
	if cfg.Redis.VectorField != "" {
		return cfg.Redis.VectorField
	}
	return RedisVectorField
}
