# TODO：Eino CTF Agent 后续增强清单

本文档记录当前 Redis + Eino 原生组件架构之后，还值得继续补充的能力。优先级按“是否影响可用性、是否降低后续维护成本、是否贴合 CTF Agent 场景”排序。

## 当前已完成的关键调整

- SQLite 元数据和 SQLite VectorStore 已移除。
- 知识库写入改为 Eino Redis Indexer。
- 知识库检索改为 Eino Redis Retriever。
- 文档元数据改为 Redis Hash/Set。
- Markdown 解析、切块、Redis 元数据、Indexer/Retriever 组装已收敛到 `internal/knowledge`。
- RAG 服务只依赖 Eino `retriever.Retriever` 接口。
- Skill 注入已接入 RAG Prompt，流式响应会返回命中的 Skill。

## P0：Redis Stack / RediSearch 本地开发环境 ✅ 已完成

### 目标

提供一套明确的本地启动方式，确保 Redis 支持 `FT.CREATE`、`FT.SEARCH` 和向量检索。

### 为什么要做

普通 Redis Server 不包含 Redis Search/Vector Search。当前代码已经使用 Eino Redis Indexer/Retriever，运行时需要 Redis Stack 或加载 RediSearch 模块的 Redis。

### 详细任务

- 增加 `docker-compose.yml`，使用 Redis Stack 镜像。
- 文档中写清楚 WSL、本机、Docker 三种连接方式。
- 启动脚本或 README 中加入 `redis-cli FT._LIST` 检查。
- 服务启动失败时继续保留清晰错误：需要 Redis Stack / RediSearch。

### 验收标准

- 新机器按文档可一次启动 Redis Stack。
- `go run ./cmd/server` 可以自动创建向量索引。
- 普通 Redis 缺少 RediSearch 时错误信息清晰。

### 实现记录（2026-05-14）

- ✅ 已新增 `docker-compose.yml`（Redis Stack + RedisInsight，端口 6379/8001）
- ✅ 已新增 `Makefile`（含 `redis-up` / `redis-down` 命令）

## P0：Eino Graph 化 RAG 主链路

### 目标

把当前 `service.RAGService` 中的手工编排逐步迁移到 Eino `compose.Graph` 或 `compose.Chain`。

### 为什么要做

项目目标是 Eino 驱动的本地知识库 Agent。Graph 化后可以更自然地接入 callback、trace、retriever node、tool node 和后续 ReAct Agent。

### 详细任务

- 将 Retriever、Prompt Builder、ChatModel 组合为 Eino Graph。
- 为 Skill Router 增加一个独立节点或前置 transformer。
- 保留现有 HTTP API，不影响前端。
- 给 Graph 增加基础单元测试。

### 验收标准

- RAG 请求可以通过 Eino Graph 跑通。
- Retriever / ChatModel callback 可以被 Eino trace 捕获。
- `service` 层只负责调用 Graph，不再手写主流程细节。

## P0：Mock LLM / Mock Embedder 开发模式 ✅ 已完成

### 目标

没有真实 `DEEPSEEK_API_KEY` 或 `DASHSCOPE_API_KEY` 时，也能启动服务、跑测试、调前端。

### 为什么要做

外部模型 API 会带来成本、网络不稳定和测试不可复现问题。Mock 能显著提升本地开发效率。

### 详细任务

- 增加 `llm.provider: mock`。
- 增加 `embedding.provider: mock`。
- Mock ChatModel 返回可预测文本。
- Mock Embedder 使用稳定 hash 生成固定维度向量。
- 测试环境默认使用 mock provider。

### 验收标准

- 不配置 API Key 也可以启动后端。
- `go test ./...` 不依赖外部网络。
- 上传文档、检索、聊天链路可以在 mock 模式跑通。

### 实现记录（2026-05-14）

- ✅ `internal/llm/mock.go`：MockChatModel 实现 BaseChatModel + ToolCallingChatModel
- ✅ `internal/embedding/mock.go`：MockEmbedder 使用 FNV-1a 哈希生成确定性向量
- ✅ 工厂函数已接入 `mock` provider

## P1：文档去重与重新索引

### 目标

避免重复上传同一文档造成重复 embedding，并支持配置或模型变化后的重建索引。

### 详细任务

- 上传时计算文件 SHA256。
- Redis 文档元数据增加 `content_hash`、`embedding_model`、`embedding_dimension`。
- 同 hash 文档默认拒绝重复上传或返回已有文档。
- 增加 `POST /api/knowledge/documents/:id/reindex`。
- reindex 前删除旧 chunk keys，再重新走 Eino Redis Indexer。

### 验收标准

- 重复上传同一文件不会生成重复向量。
- reindex 后 `chunk_count` 和 Redis chunk keys 更新正确。
- reindex 失败时文档状态为 `failed`，错误可见。

## P1：PDF 文本型文件入库 ✅ 已完成

### 目标

支持文本型 PDF 上传、解析、切分、向量化和检索。不做 OCR。

### 详细任务

- 选择稳定的 PDF 文本提取库。
- 按页提取文本并保留 `page_number`。
- PDF chunk metadata 写入 Redis。
- 上传接口允许 `.pdf`。
- 扫描版或无法提取文本的 PDF 标记为 `failed`。

### 验收标准

- 文本型 PDF 上传后状态变为 `indexed`。
- citation 中包含页码。
- 扫描版 PDF 不导致服务崩溃。

### 实现记录（Phase 4B，2026-05）

- ✅ `internal/knowledge/pdf.go`：使用 `ledongthuc/pdf` 库提取文本
- ✅ `internal/knowledge/service.go`：`enqueuePDFIndex` / `indexPDF` 异步索引
- ✅ `.pdf` 已在 `isAllowedFilename` 中允许
- ✅ 按页提取文本，保留 `page_number` 元数据

## P1：前端完整接入当前后端

### 目标

网页端可以直接使用上传、状态查询、删除、流式聊天、citation 和 Skill 命中展示。

### 详细任务

- 完成 `streamChatMessage()`，解析 `message_delta`、`citation`、`skill_used`、`error`、`done`。
- 完成 `uploadDocument()` 和 `deleteDocument()`。
- 知识库页面展示 `pending/parsing/chunking/embedding/indexed/failed` 状态。
- 上传后轮询 `/api/knowledge/documents`，直到 `indexed` 或 `failed`。
- 聊天页展示 citation 和 skill badge。

### 验收标准

- 浏览器中可以上传 `.md` 文件。
- 上传后可以看到异步索引状态变化。
- RAG 回答可以展示引用来源。
- 命中 Skill 时前端可见。

## P1：结构化日志和统一错误响应 🔶 已实现，部分接入

### 目标

统一 API 错误格式，引入 request id，记录 RAG 链路关键耗时。

### 详细任务

- 定义业务错误码，如 `invalid_request`、`index_failed`、`embedding_failed`、`llm_failed`。
- Handler 统一使用 response helper。
- 请求日志包含 `request_id`、`method`、`path`、`status`、`duration`。
- panic 由 recovery 中间件捕获并返回统一错误。
- 日志中禁止输出 API Key。

### 验收标准

- 所有 API 错误响应结构一致。
- 每个请求日志都能看到 request id 和耗时。
- panic 不会导致服务进程退出。

### 实现记录（2026-05-14）

- ✅ `internal/errors/errors.go`：13 个业务错误码 + AppError 结构体
- ✅ `internal/pkg/response/response.go`：OK/Created/NoContent/Error/ErrorRaw helpers
- ✅ `internal/middleware/logger.go`：traceID + API Key 脱敏 + 结构化日志
- ✅ `internal/middleware/recovery.go`：panic 捕获 + 统一错误响应
- ✅ `main.go`：已切换为 `middleware.Logger()` / `middleware.Recovery()`
- ✅ `handler/skill.go`：已接入 `pkg/response` 统一响应
- 🔶 其余 handler（chat.go, knowledge.go）仍使用 `model.ErrorResponse`，待渐进迁移

## P2：混合检索和过滤

### 目标

在 Redis 向量检索基础上补充更强的关键词/过滤能力，提高 CTF writeup、工具参数、报错文本类查询的命中率。

### 详细任务

- 利用 RediSearch TEXT/TAG/NUMERIC 字段实现文件类型、文档 ID、页码过滤。
- 设计混合检索策略：向量 TopK + 关键词召回 + rerank。
- 针对 CTF 场景加入常见关键词保真策略，例如 CVE、函数名、命令、flag 格式。

### 验收标准

- 精确错误字符串、函数名、命令参数能稳定召回。
- 可按文档或文件类型过滤。
- 检索结果排序比纯向量更稳。

## P2：Agent 工具化知识库检索 ✅ 已完成

### 目标

把知识库检索封装为 Eino Tool，供 Tool Agent / ReAct Agent 主动调用。

### 详细任务

- 实现 `internal/tool/knowledge_search.go`。
- Tool 内部复用 Eino Redis Retriever。
- 返回结构化结果：content、filename、chunk_index、score。
- 在 Agent Runner 中注册该 Tool。

### 验收标准

- Agent 可以按需调用知识库搜索。
- Tool 返回结果可被模型继续推理。
- 不重复实现 retriever 逻辑。

### 实现记录

- ✅ `internal/tool/knowledge_search.go`：已实现，复用 Eino Redis Retriever
- ✅ 已在 `main.go` 中注册为 `knowledge_search` 工具
- ✅ 返回结构化结果含 content/filename/chunk_index/score

