# 项目结构说明

当前后端已从 SQLite MVP 收敛为 Redis + Eino 原生组件：

- Embedding 使用 Eino `embedding.Embedder`。
- 知识库写入使用 `eino-ext/components/indexer/redis`。
- 知识库检索使用 `eino-ext/components/retriever/redis`。
- 文档元数据也存入 Redis Hash/Set。
- Markdown 解析、切块、Redis 元数据、Indexer/Retriever 组装统一放在 `internal/knowledge`，避免 `internal` 下出现过多碎片包。

> 注意：向量检索依赖 Redis Search/Vector Search，普通 Redis Server 不包含 `FT.CREATE`/`FT.SEARCH`。本地建议使用 Redis Stack 或带 RediSearch 模块的 Redis。

## 目录结构

```text
eino_ctf_agent/
├── cmd/server/main.go                 # 程序入口，组装配置、模型、Redis、HTTP 路由
├── configs/                           # 配置文件
├── data/                              # 上传文档与 Skills
├── docs/                              # 项目文档
├── internal/
│   ├── agent/                         # Agent Runner
│   ├── config/                        # 配置加载和默认值
│   ├── embedding/                     # Eino Embedder 工厂
│   ├── handler/                       # HTTP Handler
│   ├── knowledge/                     # 知识库核心：Markdown、Redis 元数据、Eino Redis Indexer/Retriever
│   ├── llm/                           # Eino ChatModel 工厂
│   ├── middleware/                    # CORS / Logger / Recovery
│   ├── model/                         # HTTP 请求/响应模型
│   ├── prompt/                        # RAG / Agent / Skill Prompt
│   ├── router/                        # Gin 路由注册
│   ├── service/                       # Chat / RAG / Skill 应用服务
│   ├── skill/                         # Skill 加载、注册、路由
│   ├── tool/                          # Eino Tool 扩展点
│   └── pkg/                           # SSE、响应、安全等内部工具
└── web/                               # Vue 3 + Vite 前端
```

## 核心链路

### 文档入库

```text
HTTP Upload
  → knowledge.Service.UploadMarkdown
  → 保存原始 Markdown 到 data/docs/markdown
  → Redis 记录 document 元数据，状态 pending
  → 后台 goroutine 异步解析和切块
  → Eino Redis Indexer 生成 embedding 并写入 Redis Hash
  → Redis document 状态更新为 indexed/failed
```

### RAG 检索

```text
ChatRequest
  → service.RAGService
  → Eino Redis Retriever
  → schema.Document + metadata
  → Skill Router 注入匹配到的 Skill
  → BuildRAGSystemPrompt
  → Eino ChatModel Generate/Stream
```

## Redis Key 约定

默认 `redis.key_prefix` 为 `eino_ctf_agent:`：

```text
eino_ctf_agent:documents               # 文档 ID 集合
eino_ctf_agent:doc:<document_id>       # 文档元数据 Hash
eino_ctf_agent:chunk:<document_id>:<n> # chunk 内容、metadata、embedding Hash
idx:eino_ctf_agent_chunks              # RediSearch 向量索引
```

## 配置重点

```yaml
storage:
  docs_dir: ./data/docs
  skills_dir: ./data/skills

redis:
  addr: 127.0.0.1:6379
  username: ""
  password_env: REDIS_PASSWORD
  db: 0
  key_prefix: eino_ctf_agent:
  index: idx:eino_ctf_agent_chunks
  vector_field: vector_content
  distance_threshold: 0
  dialect: 2
```

## 设计原则

1. 已有 Eino/eino-ext 组件优先，不重复造轮子。
2. `service` 层只表达应用语义，不实现 embedding、indexer、retriever 这类框架已有抽象。
3. 知识库相关实现集中在 `internal/knowledge`，避免 `parser/splitter/store/vectorstore/retriever` 分散扩张。
4. Redis 是当前唯一知识库存储；后续如接入 Milvus、Qdrant、Elasticsearch，也应优先使用对应 Eino 组件或薄适配层。

