# 项目阶段规格说明

## 1. 阶段体系说明

本项目有两套阶段命名，来源于不同规划时期：

| 体系 | 来源 | 范围 |
|------|------|------|
| **plan.md 原始阶段** | `plan.md`，项目启动时制定的 0-10 阶段计划 | 项目基础能力路线：项目骨架 → 聊天 → 流式 → 前端 → Embedding → 知识库 → RAG → Skills → Agent → 部署 |
| **Agent 扩展阶段** | Phase 5/6 实际开发时引入 | ReAct Agent 和 CTF 工具能力路线：Agent 主链路 → CTF 本地工具 → IDA MCP 工具 |

**命名冲突**：plan.md 的 Phase 6 是「Skills 读取与路由」，Agent Phase 6 是「IDA MCP 只读二进制分析工具链」。为避免混淆，后续阶段扩展统一使用 **Agent Phase N** 命名。

**关系**：Agent 扩展阶段不是替代 plan.md，而是在 plan.md Phase 7/7.5 完成后的能力延伸。两者共同构成当前项目能力。

## 2. 当前项目总览

```
Go + Gin 后端
├── 配置管理 (config.yaml + .env)
├── ChatModel (DeepSeek V4 Pro / Mock，OpenAI 兼容适配)
├── Embedder (Qwen text-embedding-v4 / Mock，DashScope)
├── Redis 向量存储 (RediSearch HNSW)
├── 知识库 (Markdown/PDF 解析/切块/向量化/检索)
├── Skills 系统 (YAML front-matter + trigger 匹配)
├── 普通聊天 + SSE 流式输出
├── Simple RAG 模式 (检索 → prompt → 生成)
├── ReAct Agent 模式 (Eino React Agent + 13 个工具)
├── 可观测性 (traceID / X-Request-ID / conversation_id / 自定义 Logger/Recovery)
├── CTF 本地分析工具链 (8 个工具)
├── IDA MCP 只读分析工具 (6 个工具，SSE transport 已实现)
├── Mock LLM / Mock Embedder (provider: mock)
├── Vue 3 前端 (聊天/知识库/Skills 占位)
└── internal/agent 包 (TODO 骨架，ReAct 逻辑在 service/chat.go)
```

## 3. 原始 Phase 0-10 状态表

| 阶段 | 目标 | 状态 | 关键文件 |
|------|------|------|----------|
| Phase 0 | 项目初始化与配置 | ✅ 已完成 | `cmd/server/main.go`, `internal/config/config.go`, `configs/config.yaml` |
| Phase 1 | DeepSeek 普通聊天 | ✅ 已完成 | `internal/llm/deepseek.go`, `internal/service/chat.go`, `internal/handler/chat.go` |
| Phase 2 | SSE 流式输出 | ✅ 已完成 | `internal/handler/chat.go` (Stream handler), `internal/pkg/sse/writer.go` |
| Phase 2.5 | 最小聊天前端 | ✅ 已完成 | `web/src/views/ChatView.vue`, `web/src/api/chat.ts` |
| Phase 3 | Qwen Embedding 接入 | ✅ 已完成 | `internal/embedding/qwen.go`, `internal/embedding/factory.go` |
| Phase 4A | Markdown 知识库入库 | ✅ 已完成 | `internal/knowledge/` (markdown.go, redis.go, components.go, service.go, metadata.go) |
| Phase 4B | PDF 知识库入库 | ✅ 已完成 | `internal/knowledge/pdf.go`, `internal/knowledge/service.go`（enqueuePDFIndex/indexPDF） |
| Phase 5 | Simple RAG Chain | ✅ 已完成 | `internal/service/rag.go`, `internal/prompt/rag_prompt.go` |
| Phase 5.5 | 知识库管理前端 | ❌ 未完成 | 前端 Store 已定义，View 为占位符 |
| Phase 6 | Skills 读取与路由 | ✅ 已完成 | `internal/skill/` (model.go, loader.go, registry.go, router.go), `internal/tool/skill_reader.go` |
| Phase 7 | Tool-Augmented Agent | ✅ 已完成 | `internal/tool/knowledge_search.go`, `internal/tool/registry.go`, `internal/service/chat.go` (react path) |
| Phase 7.5 | ReAct Agent 增强 | ✅ 已完成 | `internal/service/chat.go` (react.NewAgent, streamToolCallChecker) |
| Phase 8 | 前端完整整合 | ❌ 未完成 | `web/` 下 View 为占位符 |
| Phase 9 | 日志、错误处理与测试 | 🔶 已实现，部分接入 | traceID/X-Request-ID 已实现；错误码 + 统一响应 helper 已实现；自定义 Logger/Recovery 中间件已实现并接入 main.go；handler 层渐进迁移中（skill.go 已完成） |
| Phase 10 | Docker 部署与项目文档 | ❌ 未完成 | — |

## 4. 已完成阶段规格

### Phase 0：项目初始化与配置

- **目标**：搭建 Go + Vue 单仓骨架，统一配置管理
- **实现范围**：
  - Go module 初始化，Vue 3 + Vite 项目
  - `configs/config.yaml` + `.env` 双层配置
  - Gin 健康检查 `GET /health`
  - `.gitignore` 排除 API Key、构建产物
- **关键文件**：`cmd/server/main.go`, `internal/config/config.go`, `configs/config.yaml`, `web/`
- **关键流程**：`main.go` → `config.Load()` → applyDefaults → validate → 启动 Gin
- **不包含**：Docker、CI/CD、多环境配置
- **验收方式**：`go build ./...` 成功，`go run ./cmd/server` 启动，`GET /health` 返回 200
- **遗留风险**：`applyDefaults()` 对 `== 0` 判断未设置，Temperature/ScoreThreshold 等 0 值会被覆盖（已知语义歧义）

### Phase 1：DeepSeek 普通聊天

- **目标**：完成最基础的 DeepSeek 对话，不涉及 RAG/Skills/Agent
- **实现范围**：
  - DeepSeek ChatModel 封装（OpenAI 兼容协议）
  - LLM factory 支持 provider 切换
  - `POST /api/chat` 同步非流式返回
  - 多轮消息传入
- **关键文件**：`internal/llm/deepseek.go`, `internal/llm/factory.go`, `internal/service/chat.go`, `internal/model/message.go`
- **关键流程**：HTTP POST → handler 解析 → ChatService.Chat() → ChatModel.Generate() → 返回 reply
- **不包含**：流式输出、RAG、工具调用
- **验收方式**：curl 调用 `/api/chat` 收到完整回复，API Key 错误返回明确错误
- **遗留风险**：ChatService 中 ChatModel 和 ToolCallingChatModel 是两个独立实例（Eino React Agent 要求）

### Phase 2：SSE 流式输出

- **目标**：流式输出，前端逐步显示回复
- **实现范围**：
  - `POST /api/chat/stream` SSE endpoint
  - `internal/pkg/sse/writer.go` SSE 写入工具
  - 事件序列：`message_delta` → `citation` → `skill_used` → `done`
  - 前端 `fetch + ReadableStream` 消费 SSE
- **关键文件**：`internal/handler/chat.go` (Stream handler), `internal/pkg/sse/writer.go`, `web/src/api/chat.ts`
- **关键流程**：ChatService.Stream() → ChatModel.Stream() → StreamReader → SSE Writer 逐块写入
- **不包含**：原生 EventSource、WebSocket
- **验收方式**：curl 看到 SSE event stream，前端逐步显示文本
- **遗留风险**：反向代理可能缓存 SSE 流；Gin response buffering 需 Flusher 支持

### Phase 3：Qwen Embedding 接入

- **目标**：Qwen Embedding 封装，为知识库入库和检索做准备
- **实现范围**：
  - DashScope OpenAI 兼容 embedding API
  - 默认模型 `text-embedding-v4`，维度 1024
  - `EmbedStrings(ctx, texts)` 批量接口
  - 空文本拦截
- **关键文件**：`internal/embedding/qwen.go`, `internal/embedding/factory.go`, `internal/embedding/embedder.go`
- **关键流程**：Embedder.EmbedStrings() → DashScope API → `[][]float64` 返回
- **不包含**：本地 embedding 模型、其他 provider
- **验收方式**：输入中文文本返回 1024 维向量；空文本不发送 API 请求
- **遗留风险**：DashScope endpoint 国内外不同；模型更新后维度变化

### Phase 4A：Markdown 知识库入库

- **目标**：Markdown 文档上传、解析、切块、向量化、检索
- **实现范围**：
  - Markdown 解析（heading 层级追踪生成 `A > B > C` 路径）
  - 字符级切块（chunk_size=512, overlap=64）
  - Eino Redis Indexer 向量写入
  - Eino Redis Retriever 向量检索
  - Redis Hash/Set 存储文档元数据和 chunk 数据
  - 文档状态机：`pending → parsing → chunking → embedding → indexed / failed`
  - 后台 goroutine 异步索引（最多 2 并发）
  - `POST /api/knowledge/upload`, `GET /api/knowledge/documents`, `DELETE /api/knowledge/documents/:id`
- **关键文件**：`internal/knowledge/markdown.go`, `internal/knowledge/redis.go`, `internal/knowledge/components.go`, `internal/knowledge/metadata.go`, `internal/knowledge/service.go`, `internal/knowledge/keys.go`
- **关键流程**：HTTP Upload → 保存原文件 → Redis 元数据（pending）→ 后台 goroutine 解析切块 → Embedder 生成向量 → Eino Redis Indexer 写入 → 状态更新为 indexed
- **不包含**：PDF 解析、文档去重、rerank、混合检索
- **验收方式**：上传 md 文件后状态变为 indexed；文档列表显示 filename/status/chunk_count；删除时同步清理 chunks
- **遗留风险**：大文件同步索引可能超时（已通过 2 并发 goroutine 限制缓解）；chunk 语义边界不够精确

### Phase 5（plan.md）：Simple RAG Chain

- **目标**：确定性 RAG 问答链路：检索 → prompt → 回答
- **实现范围**：
  - `RAGService`：Retriever 检索 → score threshold 过滤 → Skill Router 匹配 → BuildRAGSystemPrompt → ChatModel 生成
  - `<doc>` XML 块注入 prompt
  - `<skill>` XML 块注入 prompt（按 priority 排序）
  - 流式和非流式两种输出
  - 无检索结果时明确告知「当前知识库中没有找到直接依据」
- **关键文件**：`internal/service/rag.go`, `internal/prompt/rag_prompt.go`
- **关键流程**：query → Embedder → Retriever.Retrieve() → filter → match skills → BuildRAGSystemPrompt → ChatModel.Generate/Stream
- **不包含**：Agent 工具调用、混合检索、rerank
- **验收方式**：上传文档后提问相关内容能检索到对应 chunk；回答体现知识库内容；不相关问题不胡乱引用
- **遗留风险**：Retriever score threshold 和 RAGConfig.ScoreThreshold 可能存在重复过滤（已记录在 suggestions.md）

### Phase 6（plan.md）：Skills 读取与路由

- **目标**：Skills 加载、匹配、读取，不强制接入 Agent
- **实现范围**：
  - YAML front-matter + Markdown body 格式解析
  - SkillRegistry（加载、ListAll、GetByName、Reload）
  - SkillRouter：大小写不敏感 trigger 关键词匹配，按 priority 排序，最多 max_active_skills 个
  - SkillReader Tool（白名单校验 skill_name，不读取任意文件）
  - Skills API：列表、详情、reload
  - Skills 注入 RAG/Agent system prompt
- **关键文件**：`internal/skill/model.go`, `internal/skill/loader.go`, `internal/skill/registry.go`, `internal/skill/router.go`, `internal/tool/skill_reader.go`, `internal/handler/skill.go`, `internal/prompt/agent_prompt.go`
- **关键流程**：Loader 扫描 data/skills/ → Registry 加载 → Router.Match(query) → 匹配到的 skills 注入 prompt
- **不包含**：Skills 向量索引（有意不进入普通知识库）
- **验收方式**：启动时加载 enabled skills；输入包含 trigger 时匹配对应 skill；非法 skill_name 被拒绝
- **遗留风险**：中文关键词可能误触发；skill body 过长导致 prompt 膨胀（已通过 trimSkillBody 限制）

### Phase 7：Tool-Augmented Agent

- **目标**：在 Simple RAG 稳定后，引入工具增强 Agent
- **实现范围**：
  - `knowledge_search` Tool（封装 Eino Retriever）
  - `skill_reader` Tool（封装 SkillReader）
  - Tool Registry（并发安全 map）
  - `agent.mode` 配置切换：`simple_rag` / `react`
  - ChatService 双模式路由
- **关键文件**：`internal/tool/knowledge_search.go`, `internal/tool/skill_reader.go`, `internal/tool/registry.go`, `internal/service/chat.go`
- **关键流程**：Chat()/Stream() → 读 agent.mode → simple_rag 走 RAGService / react 走 reactChat()
- **不包含**：Eino Graph 编排（有意保持手工编排作为过渡方案）
- **验收方式**：Agent 能调用 knowledge_search 和 skill_reader；配置可关闭 Agent 回退 Simple RAG
- **遗留风险**：模型可能选错工具；工具输出过长导致 prompt 膨胀

### Phase 7.5：ReAct Agent 增强

- **目标**：Eino ReAct Agent 集成，多步推理和工具调用循环
- **实现范围**：
  - Eino `react.NewAgent()` 集成
  - `ToolCallingChatModel`（DeepSeek，与普通 ChatModel 分离）
  - `streamToolCallChecker`：自定义 checker 处理 DeepSeek thinking 模式的工具调用
  - `MaxStep` 配置（默认 5）
  - `ShowToolCalls` 配置
  - Agent 惰性初始化（`sync.Once`）
- **关键文件**：`internal/service/chat.go` (reactChat, reactStream, getReactAgent, streamToolCallChecker)
- **关键流程**：react.NewAgent(ToolCallingModel + Tools) → agent.Generate/Stream() → reasoning → tool call → observation → answer
- **不包含**：Agent trace 持久化、debug 模式 UI
- **验收方式**：复杂问题分多步调用工具；max_steps 生效；普通问题不过度复杂
- **遗留风险**：sync.Once 错误不可恢复；流式 checker 消费中间 chunk（Eino Graph 架构决定）

### Agent Phase 1-4：ReAct Agent 主链路与可观测性

- **目标**：Agent 基础能力 + 可观测性最小增强
- **实现范围**：
  - Eino ReAct Agent 双工具链（knowledge_search + skill_reader）
  - `X-Request-ID` header 透传（handler 提取或生成 → context 注入）
  - `traceID`：每请求 8 字节随机 hex 编码（16 字符）
  - `conversation_id`：在 entry 日志中追踪，当前不持久化
  - 多轮对话：客户端每次请求传入完整 messages 历史，`ToSchemaMessages()` 保留全部消息
  - Agent 初始化失败不阻止服务启动（`[WARN]` 日志）
  - `ChatStream` 流式读取器封装：预检索引用 + 匹配技能 + SSE 事件序列
- **关键文件**：`internal/service/chat.go`, `internal/model/message.go`, `internal/handler/chat.go`
- **关键流程**：Handler 提取 X-Request-ID → ContextWithTraceID → ChatService.Chat/Stream → buildAgentInput → Agent
- **不包含**：服务端会话持久化、Agent trace UI
- **验收方式**：X-Request-ID 透传验证；多轮消息全部保留测试；traceID 唯一性测试
- **遗留风险**：sync.Once 错误不可恢复；conversation_id 仅日志用途；ChatStream.Skills/Citations 在 react path 为空

### Agent Phase 5：CTF 本地分析工具最小闭环

- **目标**：让 Agent 能读取文件、执行命令、运行 Python、解码编码
- **实现范围**：
  - `file_info`：文件存在性、大小、类型判断（30+ 扩展名分类）
  - `file_reader`：读取文本/源码文件，max_bytes 限制（默认 1MB）
  - `command_executor`：执行 27 个 allowlist 命令，路径参数校验，tar/unzip 仅允许列表模式
  - `python_runner`：最小环境变量（PATH + PYTHONNOUSERSITE），timeout + 输出截断
  - `encoding_decoder`：hex / base64 / url / rot13 / binary 解码
  - 安全边界：所有路径限制在工作目录内、禁止绝对路径和 `../` 穿越、timeout 默认 5s 最大 20s、输出 100KB 截断
- **关键文件**：`internal/tool/safe_util.go`, `internal/tool/file_info.go`, `internal/tool/file_reader.go`, `internal/tool/command_executor.go`, `internal/tool/python_runner.go`, `internal/tool/encoding_decoder.go`, `internal/tool/ctf_tools_test.go`
- **关键流程**：Agent 调用工具 → resolvePath 校验 → 执行 → truncateOutput → 返回 truncated 标记
- **不包含**：网络访问、Docker 沙箱、远程 exploit、socket 交互
- **验收方式**：43 个单元测试覆盖安全边界、输出截断、路径防逃逸、命令白名单、tar/unzip 限制
- **遗留风险**：command_executor 中 tar/unzip 列表模式校验依赖 flag 字符串匹配；python_runner 无 OS 级网络隔离

### Agent Phase 6：IDA MCP 只读二进制分析工具链 ✅ 已完成

- **目标**：让 Agent 在遇到二进制文件时可以结合 IDA MCP 做初步逆向分析
- **实现范围**：
  - `IDAMCPClient` 接口抽象（工具层不依赖传输层）
  - `RealMCPClient`：endpoint 安全校验 + 完整 SSE transport 实现（GET SSE → endpoint event → POST JSON-RPC → 读 SSE response）
  - `MockMCPClient`：测试用，所有方法返回预设数据
  - `DisabledMCPClient`：配置异常时使用，不阻止服务启动
  - 6 个工具：`ida_status`、`ida_functions`、`ida_decompile`、`ida_strings`、`ida_xrefs`、`ida_disasm`
  - endpoint 只允许 `127.0.0.1` 或 `localhost`（IPv4 only）
  - SSE transport 含 fallback 机制（wsl-conn-fix.patch 兼容性问题处理）
- **关键文件**：`internal/tool/ida_mcp_client.go`, `internal/tool/ida_mcp_tools.go`, `internal/tool/ida_mcp_sse.go`, `internal/tool/ida_mcp_test.go`
- **关键流程**：main.go 读环境变量 → NewRealMCPClient（校验 endpoint）→ SetIDAClient → 注册 6 个工具 → Agent 调用 ida_status → 按需调用其他 ida_* 工具
- **不包含**：`::1` IPv6、自动重连
- **验收方式**：单元测试覆盖 endpoint 校验、Status 探测、Mock 工具正常返回、输出截断、Registry 注册
- **遗留风险**：SSE stream 错误处理依赖 IDA MCP 服务端行为

## 5. 未完成或部分完成阶段

### Phase 4B：PDF 知识库入库 ✅ 已完成

- **原计划目标**：支持文本型 PDF 上传、解析、切分、向量化
- **实现范围**：
  - 使用 `ledongthuc/pdf` 库提取 PDF 文本
  - 按页提取文本，保留 `page_number` 元数据
  - `POST /api/knowledge/upload` 允许 `.pdf` 文件
  - 异步索引（`enqueuePDFIndex` / `indexPDF`）
  - 扫描版或无法提取文本的 PDF 标记为 `failed`
- **关键文件**：`internal/knowledge/pdf.go`, `internal/knowledge/service.go`

### Phase 5.5：知识库管理前端

- **原计划目标**：知识库上传、列表、删除、状态展示
- **当前缺口**：前端 Store 已定义 API 调用，View 为占位符
- **建议后续实现**：FileUploader 组件 + DocumentList 组件 + 上传后轮询状态
- **优先级**：P2（后端 API 已可用，前端可独立推进）

### Phase 8：前端完整整合

- **原计划目标**：整合聊天、知识库、Skills、citation、工具调用展示
- **当前缺口**：所有 View 为占位符，仅 `listDocuments()` 和 `listSkills()` API 调用已连接
- **建议后续实现**：ChatView 流式渲染 → KnowledgeView 上传/列表 → SkillsView 列表/详情 → CitationPanel → ToolCallPanel
- **优先级**：P2（后端能力已就绪，前端逐个页面推进）

### Phase 9：日志、错误处理与测试 🔶 已实现，部分接入

- **原计划目标**：统一错误格式、request_id、核心测试、可观测性
- **已实现**：
  - traceID、X-Request-ID、conversation_id（见 Agent Phase 1-4）
  - `middleware/cors.go`：CORS 中间件已实现并接入
  - `internal/errors/errors.go`：13 个业务错误码 + AppError 结构体
  - `internal/pkg/response/response.go`：OK/Created/NoContent/Error/ErrorRaw helpers
  - `middleware/logger.go`：traceID + API Key 脱敏 + 结构化日志
  - `middleware/recovery.go`：panic 捕获 + 统一错误响应格式
  - `main.go`：已切换为 `middleware.Logger()` / `middleware.Recovery()`（替换 Gin 内置）
  - Mock LLM / Mock Embedder 开发模式（`llm/mock.go` + `embedding/mock.go`）
- **当前缺口**：
  - handler 层仅 `skill.go` 已接入 `pkg/response`，其余 handler 仍使用 `model.ErrorResponse`
  - handler 集成测试未编写
- **优先级**：P1（基础已就绪，渐进迁移即可）

### Phase 10：Docker 部署与项目文档

- **原计划目标**：docker-compose up 一键启动
- **当前缺口**：无 Dockerfile、docker-compose.yml、Makefile
- **建议后续实现**：后端 Dockerfile + docker-compose（Redis Stack + 后端 + 前端）+ README 快速开始
- **优先级**：P2（本地开发期暂不需要，但接近演示期应优先）

## 6. Agent 扩展阶段说明

Agent 扩展阶段是在 plan.md Phase 7.5（ReAct Agent）完成后的能力延伸。

| 阶段 | 主题 | 工具数 | 状态 |
|------|------|--------|------|
| Agent Phase 1-4 | ReAct Agent 主链路与可观测性 | 2 | ✅ 已完成 |
| Agent Phase 5 | CTF 本地分析工具最小闭环 | 5 | ✅ 已完成 |
| Agent Phase 6 | IDA MCP 只读二进制分析工具链 | 6 | ✅ 已完成（含 SSE transport + ida_disasm） |

默认合法 endpoint 下，Agent 共注册 **13 个工具**，按来源分布：

```
knowledge_search    (Phase 7)           ← 真实可用
skill_reader        (Phase 7)           ← 真实可用
file_info           (Agent Phase 5)     ← 真实可用
file_reader         (Agent Phase 5)     ← 真实可用
command_executor    (Agent Phase 5)     ← 真实可用
python_runner       (Agent Phase 5)     ← 真实可用
encoding_decoder    (Agent Phase 5)     ← 真实可用
crypto_helper       (Phase 7)           ← 真实可用
remote_interactor   (Phase 8)           ← 真实可用
archive_tool        (Phase 9)           ← 真实可用
ida_status          (Agent Phase 6)     ← 真实可用（探测 endpoint 是否可达）
ida_functions       (Agent Phase 6)     ← 真实可用（SSE transport）
ida_decompile       (Agent Phase 6)     ← 真实可用（SSE transport）
ida_strings         (Agent Phase 6)     ← 真实可用（SSE transport）
ida_xrefs           (Agent Phase 6)     ← 真实可用（SSE transport）
ida_disasm          (Agent Phase 6)     ← 真实可用（SSE transport）
```

> endpoint 非法时不注册 RealMCPClient，改用 DisabledMCPClient，各工具返回配置错误。
> 注意：当前 main.go 注册了全部 13 个工具，但 Agent Phase 6 文档标注 6 个 IDA 工具。

## 7. 当前架构边界

### 不应强行 Eino 化的部分

| 模块 | 原因 |
|------|------|
| Gin handler / router | HTTP 层，Eino 不提供等价组件 |
| Config loading (YAML + env) | 基础设施，与应用编排无关 |
| Request validation | 业务逻辑，Eino 无等价组件 |
| Trace / logging | 可观测性基础设施 |
| 文件路径安全校验 (`security.SafeJoin`) | 应用安全策略 |
| 命令执行安全 (`validatePathArgs`, allowlist) | CTF 特化安全逻辑 |
| HTTP response / SSE output | Eino 不提供 HTTP 输出层 |
| Tool Registry (`map[string]einotool.BaseTool`) | 薄封装层，已正确对接 Eino ToolsConfig |

### 应优先使用 Eino 的部分

| 模块 | Eino 组件 |
|------|----------|
| ChatModel | `einomodel.BaseChatModel` / `ToolCallingChatModel` |
| Embedder | `einoembedding.Embedder` |
| Tool | `einotool.InvokableTool` (通过 `utils.InferTool[I,O]()` 创建) |
| ReAct Agent | `react.NewAgent()` |
| Retriever | `einoretriever.Retriever` |
| Indexer | Eino Redis Indexer |
| Schema messages | `schema.Message`, `schema.Document` |
| Prompt 编排 | `prompt.BuildAgentSystemPrompt()` / `BuildRAGSystemPrompt()` |

## 8. 后续阶段建议

### P0：必须先修

1. ~~**统一错误响应格式**：实现 `internal/errors/errors.go` + `internal/pkg/response/response.go`~~ ✅ 已完成（2026-05-14）
2. **配置语义歧义**：`applyDefaults()` 对 `== 0` 的 Temperature/ScoreThreshold 覆盖问题（已知设计选择，暂不修改）

### P1：下一阶段建议

1. **会话持久化**：基于 conversation_id 将 Eino state.Messages 持久化到 Redis
2. **Agent trace 持久化**：记录 reasoning → tool call → observation → answer 调用链
3. **Handler 层渐进迁移**：将 `chat.go`、`knowledge.go` 的错误响应迁移到 `pkg/response`

### P2：之后可以做

1. **前端完整整合** (Phase 5.5 + Phase 8)：ChatView 流式、KnowledgeView 上传/列表、SkillsView、citation/tool_call 展示
2. **Docker 部署** (Phase 10)：完善 Dockerfile + README 快速开始指南
3. **混合检索**：向量 + BM25 + rerank
4. **Agent 包重构**：将 `service/chat.go` 中的 ReAct 逻辑迁移到 `internal/agent/`

### P3：暂不建议

1. **自动 exploit 生成**：需前置沙箱隔离、权限控制、审计日志
2. **远程 pwn / socket 交互**：需前置完整沙箱
3. **Docker 沙箱**：需前置安全策略评审
4. **MCP server 端实现**：当前只需 client 端

## 9. 验收命令

```bash
# 格式化
gofmt -l .

# 编译
go build ./...

# 静态分析
go vet ./...

# 全部测试（纯单元，不需要外部依赖）
go test ./...
```

---

> 最后更新：2026-05-14  
> 文档版本：v1.1  
> 本次更新：工程收敛 — 中间件接入、Mock 实现、文档同步、docker-compose/Makefile  
> 下次更新触发：下一阶段完成后或架构变更时
