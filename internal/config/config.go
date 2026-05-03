package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是项目的顶层配置结构，与 config.example.yaml 完全对应。
// Phase 0 只使用 Server 部分，其余字段在后续 Phase 中逐步启用。
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	LLM       LLMConfig       `yaml:"llm"`
	Embedding EmbeddingConfig `yaml:"embedding"`
	RAG       RAGConfig       `yaml:"rag"`
	Storage   StorageConfig   `yaml:"storage"`
	Skills    SkillsConfig    `yaml:"skills"`
	Agent     AgentConfig     `yaml:"agent"`
}

// ServerConfig 定义 HTTP 服务参数。
type ServerConfig struct {
	Host string     `yaml:"host"`
	Port int        `yaml:"port"`
	CORS CORSConfig `yaml:"cors"`
}

// CORSConfig 定义跨域允许的来源列表。
type CORSConfig struct {
	AllowOrigins []string `yaml:"allow_origins"`
}

// LLMConfig 定义 DeepSeek ChatModel 参数。
type LLMConfig struct {
	Provider      string  `yaml:"provider"`
	Model         string  `yaml:"model"`
	FallbackModel string  `yaml:"fallback_model"`
	BaseURL       string  `yaml:"base_url"`
	APIKeyEnv     string  `yaml:"api_key_env"`
	Thinking      bool    `yaml:"thinking"`
	Temperature   float64 `yaml:"temperature"`
	MaxTokens     int     `yaml:"max_tokens"`
}

// EmbeddingConfig 定义 Qwen Embedding 参数。
type EmbeddingConfig struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
	Dimension int    `yaml:"dimension"`
	BatchSize int    `yaml:"batch_size"`
}

// RAGConfig 定义 RAG 检索参数。
type RAGConfig struct {
	TopK             int     `yaml:"top_k"`
	ScoreThreshold   float64 `yaml:"score_threshold"`
	ChunkSize        int     `yaml:"chunk_size"`
	ChunkOverlap     int     `yaml:"chunk_overlap"`
	MaxContextChunks int     `yaml:"max_context_chunks"`
}

// StorageConfig 定义存储路径。
type StorageConfig struct {
	DocsDir    string `yaml:"docs_dir"`
	SkillsDir  string `yaml:"skills_dir"`
	MetadataDB string `yaml:"metadata_db"`
	VectorDB   string `yaml:"vector_db"`
}

// SkillsConfig 定义 Skills 系统参数。
type SkillsConfig struct {
	Enabled         bool `yaml:"enabled"`
	MaxActiveSkills int  `yaml:"max_active_skills"`
	AllowReload     bool `yaml:"allow_reload"`
}

// AgentConfig 定义 Agent 运行模式。
type AgentConfig struct {
	Mode          string `yaml:"mode"`
	MaxSteps      int    `yaml:"max_steps"`
	ShowToolCalls bool   `yaml:"show_tool_calls"`
}

// Load 从指定路径加载 YAML 配置文件，并应用默认值。
// 配置文件路径查找顺序：参数 path → 环境变量 CONFIG_PATH → 默认 configs/config.yaml。
func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("CONFIG_PATH")
	}
	if path == "" {
		path = "configs/config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	cfg.applyDefaults()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// applyDefaults 为未设置的字段填充合理默认值。
func (c *Config) applyDefaults() {
	// Server defaults
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if len(c.Server.CORS.AllowOrigins) == 0 {
		c.Server.CORS.AllowOrigins = []string{"http://localhost:5173"}
	}

	// LLM defaults
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

	// Embedding defaults
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

	// RAG defaults
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

	// Storage defaults
	if c.Storage.DocsDir == "" {
		c.Storage.DocsDir = "./data/docs"
	}
	if c.Storage.SkillsDir == "" {
		c.Storage.SkillsDir = "./data/skills"
	}
	if c.Storage.MetadataDB == "" {
		c.Storage.MetadataDB = "./metadata_db/app.sqlite"
	}
	if c.Storage.VectorDB == "" {
		c.Storage.VectorDB = "./vector_db/vector.sqlite"
	}

	// Skills defaults
	if c.Skills.MaxActiveSkills == 0 {
		c.Skills.MaxActiveSkills = 3
	}

	// Agent defaults
	if c.Agent.Mode == "" {
		c.Agent.Mode = "simple_rag"
	}
	if c.Agent.MaxSteps == 0 {
		c.Agent.MaxSteps = 5
	}
}

// validate 校验关键配置字段。Phase 0 只校验 Server 字段。
func (c *Config) validate() error {
	var errs []string

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Sprintf("server.port must be 1-65535, got %d", c.Server.Port))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// GetLLMAPIKey 从环境变量中获取 LLM API Key。
func (c *Config) GetLLMAPIKey() string {
	return os.Getenv(c.LLM.APIKeyEnv)
}

// GetEmbeddingAPIKey 从环境变量中获取 Embedding API Key。
func (c *Config) GetEmbeddingAPIKey() string {
	return os.Getenv(c.Embedding.APIKeyEnv)
}

// Addr 返回 "host:port" 格式的监听地址。
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
