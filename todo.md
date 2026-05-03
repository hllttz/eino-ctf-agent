# TODO：Eino CTF Agent 后续增强清单

本文档记录当前后端 MVP 之后最值得继续补充的能力。优先级按“能否明显提升可用性、是否降低后续开发成本、是否符合 CTF Agent 场景”排序。

## P0：继续收敛到 Eino 原生组件和 Graph

### 目标

凡是 Eino 或 eino-ext 已经提供的组件能力，优先用框架组件表达，而不是在业务层重新定义同类抽象。

### 为什么要做

项目目标是 Eino 驱动的本地知识库 Agent。重复定义 Embedding、Retriever、Indexer、Tool、Graph 等概念，会让后续接入 Eino callback、trace、compose graph、tool node 和 ReAct Agent 变得别扭。

### 当前状态

- LLM 已使用 Eino `model.BaseChatModel`。
- Embedding 已收敛为 Eino `embedding.Embedder`，DashScope/Qwen 走 eino-ext OpenAI-compatible embedding 组件。
- KnowledgeRetriever 已实现 Eino `retriever.Retriever`，返回 `schema.Document`。
- SQLite VectorStore 仍保留为本地 MVP store adapter，因为当前需求是本地 SQLite JSON 向量存储，暂未直接使用 Milvus/Redis/Elasticsearch 等 eino-ext 外部向量库。

### 详细任务

- 将 Markdown 入库链路逐步收敛为 `Parser/Transformer/Indexer` 风格。
- 为本地 SQLite VectorStore 增加 Eino `indexer.Indexer` 适配器。
- 将 Simple RAG 从 service 手写编排迁移到 Eino `compose.Graph` 或 `compose.Chain`。
- Tool-Augmented Agent 阶段使用 Eino `tool.BaseTool` 和 `compose.ToolsNode`。
- 保持业务 service 只负责应用语义，不直接实现框架已有的抽象。

### 验收标准

- Chat/RAG 主链路可以通过 Eino Graph 编译运行。
- Retriever、Indexer、Tool 都可作为 Eino node 接入。
- 业务层不再定义与 Eino 同名同义的接口。
- 本地 SQLite 只作为存储适配器，不向上泄露实现细节。

## P0：前端完整接入现有后端接口

### 目标

让网页端可以直接使用当前后端能力：上传 Markdown、查看索引状态、删除文档、流式聊天、显示 citation 和命中的 skills。

### 为什么要做

当前后端已经提供了核心接口，但前端仍有不少 TODO。补完前端后，项目才能从“后端 API 可用”变成“真实用户可操作的本地知识库助手”。

### 涉及模块

- `web/src/api/chat.ts`
- `web/src/api/knowledge.ts`
- `web/src/api/skills.ts`
- `web/src/views/ChatView.vue`
- `web/src/views/KnowledgeView.vue`
- `web/src/views/SkillsView.vue`
- `web/src/components/CitationPanel.vue`
- `web/src/components/SkillBadge.vue`

### 详细任务

- 实现 `streamChatMessage()`，解析 `message_delta`、`citation`、`skill_used`、`error`、`done`。
- 实现 `uploadDocument()` 和 `deleteDocument()`。
- 知识库页面展示 `pending/parsing/chunking/embedding/indexed/failed` 状态。
- 上传后轮询 `/api/knowledge/documents`，直到状态变为 `indexed` 或 `failed`。
- 聊天页展示引用来源和命中的 skills。
- 对网络错误、后端错误、索引失败给出明确提示。

### 验收标准

- 浏览器中可以上传 `.md` 文件。
- 上传后可以看到状态从 `pending` 变化到 `indexed`。
- 聊天回答可以流式显示。
- RAG 回答可以展示 citation。
- 命中 skill 时可以看到 skill badge。

## P0：Mock LLM / Mock Embedder 开发模式

### 目标

在没有真实 `DEEPSEEK_API_KEY` 或 `DASHSCOPE_API_KEY` 的情况下，也能启动服务、跑测试、调前端和验证 RAG 链路。

### 为什么要做

外部模型 API 会带来成本、网络不稳定和测试不可复现问题。Mock 能显著提升本地开发效率。

### 涉及模块

- `internal/llm/`
- `internal/embedding/`
- `configs/config.example.yaml`
- `internal/config/config.go`

### 详细任务

- 增加 `llm.provider: mock`。
- 增加 `embedding.provider: mock`。
- Mock ChatModel 返回可预测文本，例如回显最后一个 user message。
- Mock Embedder 使用稳定 hash 生成固定维度向量。
- 测试环境默认使用 mock provider。

### 验收标准

- 不配置 API Key 时，可以通过 mock 配置启动服务。
- `go test ./...` 不依赖外部网络。
- 上传文档、检索、聊天链路可以在 mock 模式下跑通。

## P1：PDF 文本型文件入库

### 目标

支持文本型 PDF 上传、解析、切分、向量化和检索。不做 OCR，不支持扫描版 PDF。

### 为什么要做

本地知识库通常不只有 Markdown。PDF 是 CTF writeup、论文、工具手册、比赛资料中非常常见的格式。

### 涉及模块

- `internal/parser/pdf.go`
- `internal/service/knowledge.go`
- `internal/handler/knowledge.go`
- `internal/model/citation.go`

### 详细任务

- 选择 PDF 文本提取库。
- 按页提取文本并保留 `page_number`。
- PDF chunk metadata 写入页码。
- 文档上传接口允许 `.pdf`。
- 扫描版或无法提取文本的 PDF 状态置为 `failed`，并写明“不支持 OCR 或未提取到文本”。

### 验收标准

- 上传文本型 PDF 后状态变为 `indexed`。
- 提问相关内容可以检索到 PDF chunk。
- citation 中包含页码。
- 扫描版 PDF 不导致服务崩溃。

## P1：文档去重与重新索引

### 目标

避免重复上传同一文档造成重复 embedding，并支持在配置变化或模型变化后重建索引。

### 为什么要做

Embedding 有成本，重复索引浪费明显。后续调整 chunk size、embedding model、score threshold 时，需要可控地重建索引。

### 涉及模块

- `internal/model/document.go`
- `internal/store/document_repo.go`
- `internal/service/knowledge.go`
- `internal/handler/knowledge.go`

### 详细任务

- 上传时计算文件 SHA256。
- `documents` 表增加 `content_hash`、`embedding_model`、`embedding_dimension`。
- 同 hash 文档默认拒绝重复上传，或返回已有文档。
- 增加 `POST /api/knowledge/documents/:id/reindex`。
- 重新索引前删除旧 chunks 和 vectors。

### 验收标准

- 重复上传同一文件不会重复生成向量。
- 调用 reindex 后 chunk_count 和 vectors 更新。
- reindex 失败时文档状态为 `failed`，旧数据不污染新索引。

## P1：统一错误响应和结构化日志

### 目标

统一 API 错误格式，引入 request_id，记录关键耗时，避免日志泄露 API Key。

### 为什么要做

RAG 链路涉及上传、数据库、embedding、LLM、SSE，多环节出错时需要快速定位。统一错误和日志是后续稳定迭代的地基。

### 涉及模块

- `internal/errors/`
- `internal/pkg/response/`
- `internal/middleware/logger.go`
- `internal/middleware/recovery.go`
- `internal/handler/*`

### 详细任务

- 定义业务错误码，例如 `invalid_request`、`index_failed`、`embedding_failed`、`llm_failed`。
- Handler 统一使用 response helper。
- 请求日志包含 `request_id`、method、path、status、duration。
- panic 由 recovery 中间件捕获并返回统一错误。
- 日志中禁止输出 API Key。

### 验收标准

- 所有 API 错误响应结构一致。
- panic 不会导致服务进程退出。
- 每个请求日志都能看到 request_id 和耗时。

## P2：知识库检索增强

### 目标

在纯向量检索基础上增加关键词召回和更稳的排序，为 CTF 场景中的函数名、报错、flag 片段、文件名等精确关键词提高召回率。

### 为什么要做

CTF 资料里经常出现短 token、函数名、命令、错误字符串。纯 embedding 对这类内容不一定稳定。

### 涉及模块

- `internal/retriever/`
- `internal/vectorstore/`
- `internal/store/chunk_repo.go`

### 详细任务

- 增加简单 BM25 或 SQLite FTS5。
- 先向量召回 top 20，再关键词召回 top 20。
- 合并去重并计算混合分数。
- 配置化权重，例如 `vector_score * 0.7 + keyword_score * 0.3`。

### 验收标准

- 查询函数名、命令、错误码时能稳定命中相关 chunk。
- 不相关文档不会因为关键词噪声大量混入。
- 检索结果仍能返回 score 和 citation。

## P2：CTF 安全小工具

### 目标

提供一批安全、确定性、无副作用的小工具，后续可接入 Tool-Augmented Agent。

### 为什么要做

CTF 解题常需要 hash、编码识别、字符串提取、entropy 分析等基础操作。这些工具不需要让 Agent 执行任意命令，安全边界更清晰。

### 涉及模块

- `internal/tool/`
- `internal/handler/`
- `internal/service/`

### 详细任务

- Hash：MD5、SHA1、SHA256。
- 编解码：base64、hex、url encode/decode。
- 文件信息：大小、扩展名、MIME、magic bytes。
- 文本分析：可打印字符串提取、字符频率、entropy。
- 对上传文件大小做限制。

### 验收标准

- 工具接口不执行 shell 命令。
- 输入非法时返回明确错误。
- 大文件不会导致内存失控。

## P2：Tool-Augmented Agent 模式

### 目标

在 Simple RAG 稳定后，让模型在受控范围内调用 `knowledge_search`、`skill_reader` 和 CTF 小工具。

### 为什么要做

Simple RAG 是确定性的，适合 MVP；但复杂 CTF 问题常需要先识别题型、读取 skill、检索资料、再做工具分析。工具增强 Agent 能更自然地串联这些步骤。

### 涉及模块

- `internal/agent/`
- `internal/tool/knowledge_search.go`
- `internal/tool/skill_reader.go`
- `internal/service/chat.go`

### 详细任务

- 实现 Agent Runner 接口。
- 根据 `agent.mode` 在 `simple_rag` 和 `tool_agent` 之间切换。
- 工具调用次数受 `agent.max_steps` 限制。
- SSE 输出 `tool_call` 事件。
- 工具失败时让模型可以基于错误继续回答。

### 验收标准

- 文档问题会调用 knowledge search。
- 逆向、pwn、crypto 等问题会调用 skill reader。
- 工具调用不会无限循环。
- 配置切回 `simple_rag` 后行为保持稳定。

## P3：多会话与消息持久化

### 目标

保存会话历史，允许前端恢复历史聊天，避免刷新页面后上下文丢失。

### 为什么要做

CTF 解题通常是连续过程，用户需要围绕同一个题目多轮追问。持久化会话可以保留上下文和引用。

### 涉及模块

- `internal/store/message_repo.go`
- `internal/model/message.go`
- `internal/service/chat.go`
- `web/src/stores/chat.ts`

### 详细任务

- 增加 `conversations` 和 `messages` 表。
- `/api/chat` 和 `/api/chat/stream` 写入用户消息和 assistant 回复。
- 增加会话列表、详情、删除接口。
- 前端支持选择历史会话。

### 验收标准

- 刷新页面后可以恢复历史会话。
- 删除会话不会删除知识库文档。
- 会话消息顺序稳定。

## P3：Docker 和文档补齐

### 目标

提供可复现的一键启动方式，并把配置、API、Skills 格式写清楚。

### 为什么要做

项目后续如果要给他人使用或迁移环境，Docker 和文档会显著降低上手成本。

### 涉及模块

- `Dockerfile`
- `docker-compose.yaml`
- `README.md`
- `docs/api.md`
- `docs/architecture.md`
- `docs/skills.md`

### 详细任务

- 后端 Dockerfile。
- 可选前端构建并由 Gin 托管静态文件。
- docker-compose 挂载 `data/`、`metadata_db/`、`vector_db/`。
- README 写快速开始、环境变量、常见问题。
- docs 写 API 请求/响应示例。

### 验收标准

- `docker-compose up --build` 可启动。
- 重启容器后知识库数据仍在。
- 新用户按 README 可以从零跑通。
