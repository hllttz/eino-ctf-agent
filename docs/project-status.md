# 项目状态

最后更新：2026-05-14

## 已完成模块

| 模块 | 关键文件 | 说明 |
|------|----------|------|
| 配置管理 | `internal/config/config.go` | YAML + .env 双层配置，默认值 + 校验 |
| HTTP 路由 | `internal/router/router.go` | Gin 路由注册，含 CORS |
| DeepSeek ChatModel | `internal/llm/deepseek.go`, `factory.go` | BaseChatModel + ToolCallingChatModel 双实例 |
| Qwen Embedding | `internal/embedding/qwen.go`, `factory.go` | DashScope OpenAI 兼容 embedding |
| RAG 知识库（Markdown） | `internal/knowledge/` | 解析/切块/Indexer/Retriever/Metadata |
| RAG 知识库（PDF） | `internal/knowledge/pdf.go` | ledongthuc/pdf 提取文本，按页切分 |
| RAG 编排 | `internal/service/rag.go` | retrieve → filter → skills → prompt → generate |
| Skills 系统 | `internal/skill/` | YAML front-matter 加载/注册/关键词路由 |
| Skills API | `internal/handler/skill.go` | list/get/reload |
| ReAct Agent | `internal/service/chat.go` | Eino react.NewAgent + 13 工具 |
| Tool Registry | `internal/tool/registry.go` | 并发安全 map 注册表 |
| CTF 本地工具（8个） | `internal/tool/` | file_info/reader, command_executor, python_runner, encoding_decoder, crypto_helper, remote_interactor, archive_tool |
| IDA MCP 工具（6个） | `internal/tool/ida_mcp_*.go` | ida_status/functions/decompile/strings/xrefs/disasm |
| SSE 流式输出 | `internal/pkg/sse/writer.go` | 4 事件序列（skill_used → citation → message_delta → done） |
| traceID / 可观测性 | `internal/service/chat.go` | 8 字节 hex traceID，context 注入 |
| Mock LLM | `internal/llm/mock.go` | BaseChatModel + ToolCallingChatModel，返回固定文本 |
| Mock Embedder | `internal/embedding/mock.go` | FNV-1a 哈希生成确定性向量 |
| 健康检查 | `internal/handler/health_handler.go` | GET /health |
| 文档元数据 CRUD | `internal/knowledge/metadata.go` | Redis Hash/Set |

## 已实现但未完全接入

| 模块 | 已有代码 | 接入状态 |
|------|----------|----------|
| 业务错误码 | `internal/errors/errors.go`（13 错误码 + AppError） | 已实现 |
| 统一响应 helper | `internal/pkg/response/response.go`（OK/Error/...） | 仅 handler/skill.go 已接入 |
| 自定义 Logger | `internal/middleware/logger.go`（traceID + API Key 脱敏） | 已接入 main.go |
| 自定义 Recovery | `internal/middleware/recovery.go`（panic 捕获 + 统一错误） | 已接入 main.go |
| 其余 Handler | `handler/chat.go`, `handler/knowledge.go` | 仍使用 model.ErrorResponse，待迁移 |

## 未完成模块

| 模块 | 说明 |
|------|------|
| `internal/agent/` | 5 个文件均为 TODO 骨架注释。ReAct 主逻辑在 `service/chat.go` |
| 前端 | 所有 View/Component 为占位符，仅部分 API 调用已连接（listDocuments, listSkills） |
| Docker 部署 | 有 docker-compose.yml（仅 Redis Stack），无后端 Dockerfile |
| Eino Graph 化 | 当前 RAG 为手工编排，未使用 Eino Graph/Chain |
| 文档去重 | SHA256 hash + reindex API 未实现 |
| 混合检索 | BM25 + rerank 未实现 |
| Agent trace 持久化 | 未实现 |
| 会话持久化 | conversation_id 仅日志用途，不持久化 |

## 暂缓模块

| 模块 | 原因 |
|------|------|
| 自动 exploit 生成 | 需沙箱隔离、权限控制、审计日志 |
| 远程 pwn / socket 交互 | 需完整沙箱 |
| Docker 沙箱 | 需安全策略评审 |
| MCP server 端 | 当前只需 client 端 |

## 文档与代码不一致修正说明

以下内容在 `todo.md` 或 `project-phase-spec.md` 中标记不准确，本次以代码实际状态修正：

| 文档标记 | 实际状态 | 修正 |
|----------|----------|------|
| Phase 4B PDF 入库「未完成」 | `knowledge/pdf.go` + `service.go` 已实现 | 已修正为 ✅ |
| Phase 9 统一错误/中间件「TODO 骨架」 | errors.go / response.go / logger.go / recovery.go 均已实现 | 已修正为 🔶 已实现部分接入 |
| Agent Phase 6 IDA MCP「SSE transport 未完成」 | `ida_mcp_sse.go` 已实现完整 SSE transport | 已修正为 ✅ |
| `middleware/logger.go`「TODO 骨架」 | 已完整实现 traceID + API Key 脱敏 | 已修正 |
| `middleware/recovery.go`「TODO 骨架」 | 已完整实现 panic 恢复 + 统一错误 | 已修正 |
| `internal/agent` | 仍是 TODO 骨架 | 无变化 |
