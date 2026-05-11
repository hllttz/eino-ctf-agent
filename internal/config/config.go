package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 应用根配置，YAML配置文件反序列化入口，聚合所有子模块配置。
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	LLM       LLMConfig       `yaml:"llm"`
	Embedding EmbeddingConfig `yaml:"embedding"`
	RAG       RAGConfig       `yaml:"rag"`
	Storage   StorageConfig   `yaml:"storage"`
	Redis     RedisConfig     `yaml:"redis"`
	Skills    SkillsConfig    `yaml:"skills"`
	Agent     AgentConfig     `yaml:"agent"`
}

// ServerConfig HTTP服务器配置。
type ServerConfig struct {
	Host string     `yaml:"host"`
	Port int        `yaml:"port"`
	CORS CORSConfig `yaml:"cors"`
}

// CORSConfig 跨域资源共享配置。
type CORSConfig struct {
	AllowOrigins []string `yaml:"allow_origins"`
}

// LLMConfig 大语言模型配置，控制模型选择、连接参数和生成行为。
type LLMConfig struct {
	Provider      string `yaml:"provider"`
	Model         string `yaml:"model"`
	FallbackModel string `yaml:"fallback_model"`
	BaseURL       string `yaml:"base_url"`
	APIKeyEnv     string `yaml:"api_key_env"`
	// 是否启用DeepSeek思考模式（思维链推理）。
	Thinking    bool    `yaml:"thinking"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

// EmbeddingConfig 向量嵌入模型配置。
type EmbeddingConfig struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
	Dimension int    `yaml:"dimension"`
	BatchSize int    `yaml:"batch_size"`
}

// RAGConfig RAG检索与上下文构建参数配置。
type RAGConfig struct {
	TopK int `yaml:"top_k"`
	// 检索相似度分数阈值（0~1），低于此分数的结果被过滤。注意与RedisConfig.DistanceThreshold的原始向量距离概念不同。
	ScoreThreshold   float64 `yaml:"score_threshold"`
	ChunkSize        int     `yaml:"chunk_size"`
	ChunkOverlap     int     `yaml:"chunk_overlap"`
	MaxContextChunks int     `yaml:"max_context_chunks"`
}

// StorageConfig 本地文件存储路径配置。
type StorageConfig struct {
	DocsDir   string `yaml:"docs_dir"`
	SkillsDir string `yaml:"skills_dir"`
}

// RedisConfig Redis连接与RediSearch向量索引配置。
type RedisConfig struct {
	Addr        string `yaml:"addr"`
	Username    string `yaml:"username"`
	PasswordEnv string `yaml:"password_env"`
	DB          int    `yaml:"db"`
	KeyPrefix   string `yaml:"key_prefix"`
	Index       string `yaml:"index"`
	VectorField string `yaml:"vector_field"`
	// 向量原始距离阈值（非分数），用于RediSearch预过滤。与RAGConfig.ScoreThreshold的相似度分数概念互补。
	DistanceThreshold float64 `yaml:"distance_threshold"`
	// RediSearch查询方言版本，默认2。
	Dialect int `yaml:"dialect"`
}

// SkillsConfig Skills技能系统配置。
type SkillsConfig struct {
	Enabled         bool `yaml:"enabled"`
	MaxActiveSkills int  `yaml:"max_active_skills"`
	AllowReload     bool `yaml:"allow_reload"`
}

// AgentConfig Agent运行模式与行为控制配置。
type AgentConfig struct {
	Mode          string `yaml:"mode"`
	MaxSteps      int    `yaml:"max_steps"`
	ShowToolCalls bool   `yaml:"show_tool_calls"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}
	if path == "" {
		path = "configs/config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if len(c.Server.CORS.AllowOrigins) == 0 {
		c.Server.CORS.AllowOrigins = []string{"http://localhost:5173"}
	}

	if c.LLM.Provider == "" {
		c.LLM.Provider = "deepseek"
	}
	if c.LLM.Model == "" {
		c.LLM.Model = "deepseek-v4-flash"
	}
	if c.LLM.BaseURL == "" {
		c.LLM.BaseURL = "https://api.deepseek.com"
	}
	if c.LLM.APIKeyEnv == "" {
		c.LLM.APIKeyEnv = "DEEPSEEK_API_KEY"
	}
	if c.LLM.Temperature == 0 {
		c.LLM.Temperature = 0.7
	}
	if c.LLM.MaxTokens == 0 {
		c.LLM.MaxTokens = 4096
	}

	if c.Embedding.Provider == "" {
		c.Embedding.Provider = "dashscope"
	}
	if c.Embedding.Model == "" {
		c.Embedding.Model = "text-embedding-v4"
	}
	if c.Embedding.BaseURL == "" {
		c.Embedding.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if c.Embedding.APIKeyEnv == "" {
		c.Embedding.APIKeyEnv = "DASHSCOPE_API_KEY"
	}
	if c.Embedding.Dimension == 0 {
		c.Embedding.Dimension = 1024
	}
	if c.Embedding.BatchSize == 0 {
		c.Embedding.BatchSize = 10
	}

	if c.RAG.TopK == 0 {
		c.RAG.TopK = 5
	}
	if c.RAG.ScoreThreshold == 0 {
		c.RAG.ScoreThreshold = 0.35
	}
	if c.RAG.ChunkSize == 0 {
		c.RAG.ChunkSize = 512
	}
	if c.RAG.ChunkOverlap == 0 {
		c.RAG.ChunkOverlap = 64
	}
	if c.RAG.MaxContextChunks == 0 {
		c.RAG.MaxContextChunks = 5
	}

	if c.Storage.DocsDir == "" {
		c.Storage.DocsDir = "./data/docs"
	}
	if c.Storage.SkillsDir == "" {
		c.Storage.SkillsDir = "./data/skills"
	}

	if c.Redis.Addr == "" {
		c.Redis.Addr = "127.0.0.1:6379"
	}
	if c.Redis.PasswordEnv == "" {
		c.Redis.PasswordEnv = "REDIS_PASSWORD"
	}
	if c.Redis.KeyPrefix == "" {
		c.Redis.KeyPrefix = "eino_ctf_agent:"
	}
	if c.Redis.Index == "" {
		c.Redis.Index = "idx:eino_ctf_agent_chunks"
	}
	if c.Redis.VectorField == "" {
		c.Redis.VectorField = "vector_content"
	}
	if c.Redis.Dialect == 0 {
		c.Redis.Dialect = 2
	}

	if c.Skills.MaxActiveSkills == 0 {
		c.Skills.MaxActiveSkills = 3
	}

	if c.Agent.Mode == "" {
		c.Agent.Mode = "simple_rag"
	}
	if c.Agent.MaxSteps == 0 {
		c.Agent.MaxSteps = 5
	}
}

func (c *Config) validate() error {
	var errs []string
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Sprintf("server.port must be 1-65535, got %d", c.Server.Port))
	}
	if c.RAG.ChunkOverlap >= c.RAG.ChunkSize {
		errs = append(errs, "rag.chunk_overlap must be smaller than rag.chunk_size")
	}
	if c.Embedding.Dimension <= 0 {
		errs = append(errs, "embedding.dimension must be positive")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (c *Config) GetLLMAPIKey() string {
	return os.Getenv(c.LLM.APIKeyEnv)
}

func (c *Config) GetEmbeddingAPIKey() string {
	return os.Getenv(c.Embedding.APIKeyEnv)
}

func (c *Config) GetRedisPassword() string {
	if c.Redis.PasswordEnv == "" {
		return ""
	}
	return os.Getenv(c.Redis.PasswordEnv)
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
