# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## Commands

```bash
# 后端
go build ./...                          # 编译检查
go run ./cmd/server                     # 启动后端（默认读取 configs/config.yaml）
go run ./cmd/server -config ./configs/config.example.yaml  # 指定配置文件
go test ./...                           # 运行全部测试（纯单元测试，不需要外部依赖）

# 前端
cd web && npm install                   # 安装依赖
cd web && npm run dev                   # 启动前端开发服务器（:5173，API 代理到 :8080）
cd web && npm run build                 # 生产构建（vue-tsc 类型检查 + vite build）
```

项目没有 Makefile、Dockerfile 或 docker-compose（todo.md 中规划为 P0）。没有 lint 脚本——直接依赖 `go build` / `vue-tsc` 的类型检查。

## Architecture

```
请求 → Gin Router → Handler（薄层） → Service（业务编排） → Eino 组件 / Redis
```

### 核心启动链路（cmd/server/main.go）

1. 加载 `configs/config.yaml` + `.env`
2. 创建 **Eino ChatModel**（DeepSeek，OpenAI 兼容适配）
3. 创建 **Eino Embedder**（Qwen DashScope，OpenAI 兼容适配）
4. 连接 **Redis**（go-redis v9），Ping 检查，`EnsureVectorIndex` 创建 RediSearch HNSW 向量索引
5. 创建 **Eino Redis Indexer** / **Eino Redis Retriever**（包装 embedder）
6. 创建 `MetadataRepo` → `knowledge.Service`
7. 加载 `data/skills/` → `skill.Registry` → `skill.Router`
8. 组装 `RAGService` → `ChatService` → Handler → Gin Engine → 启动

### 关键依赖关系

- **ChatService** 依赖 ChatModel + 可选的 RAGService（按 `agent.mode` 决定）
- **RAGService** 依赖 Eino Retriever + SkillRouter → 构建 system prompt → 调用 ChatModel
- **knowledge.Service** 负责文档上传 + 异步索引（后台 goroutine，最多 2 并发）
- **Eino Indexer** 在 `Store` 时自动调用 Embedder 生成向量，写入 Redis Hash
- **Eino Retriever** 在 `Retrieve` 时自动对 query 调用 Embedder，执行 RediSearch 向量检索
- **ChatStream**（`service/chat.go`）封装流式读取器 + 预检索引用 + 匹配技能，SSE handler 按 `skill_used → citation → message_delta → done` 顺序发送事件

### 当前模式：Simple RAG（命令式编排）

没有使用 Eino Graph/Chain API。`RAGService` 中手工调用：retrieve → filter → match skills → build prompt → generate。Agent Runner（simple_rag / tool_agent / react）都是 TODO 骨架。

### Redis 是唯一存储

- 向量索引：`FT.CREATE idx:eino_ctf_agent_chunks`（HNSW + COSINE）
- 文档元数据：`eino_ctf_agent:doc:<id>`（Redis Hash）
- 文档列表：`eino_ctf_agent:documents`（Redis Set）
- Chunk 数据：`eino_ctf_agent:chunk:<document_id>:<n>`（Redis Hash，含 vector blob）
- **必须使用 Redis Stack 或带 RediSearch 模块的 Redis**，普通 Redis 缺少 `FT.CREATE`/`FT.SEARCH`

### Skills 系统

Markdown 文件放在 `data/skills/`，格式为 YAML front-matter + Markdown body。`skill.Router` 通过大小写不敏感的 trigger 关键词匹配，按 priority 排序，最多返回 N 个。Skills 不进入向量索引，而是直接注入 system prompt。SkillReader 只接受 skill_name（白名单校验），不接受任意路径。

### 模块职责速查

| 包 | 职责 |
|---|---|
| `internal/config` | YAML 加载 + 默认值 + 环境变量读取 |
| `internal/llm` | ChatModel 工厂（目前仅 deepseek） |
| `internal/embedding` | Embedder 工厂（目前仅 dashscope/qwen） |
| `internal/knowledge` | Markdown 解析/切块、Redis 元数据、Eino Indexer/Retriever 组装、文档 service |
| `internal/skill` | Skill model、loader、registry、router |
| `internal/prompt` | RAG/Agent/Skill system prompt 构建 |
| `internal/service` | ChatService、RAGService、SkillService |
| `internal/handler` | Gin handler（chat + stream/knowledge/skill/health） |
| `internal/model` | HTTP 请求/响应结构体、Document/Citation 模型、消息工具函数（LastUserMessage、ToSchemaMessages、ToSchemaRole） |
| `internal/pkg/sse` | SSE Writer（SetHeaders + Event 写入 + Flush） |
| `internal/pkg/security` | Skill 名称校验、安全路径拼接、敏感文件拦截 |
| `internal/middleware` | CORS、请求日志、panic 恢复 |
| `internal/router` | 路由注册（Gin 路由树组装） |
| `internal/errors` | 统一错误类型（TODO 骨架） |
| `internal/agent` | Agent Runner 骨架（TODO） |
| `internal/tool` | Eino Tool 扩展点（大部分 TODO，仅 SkillReader 部分实现） |

### 配置默认值约定

`config.go` 的 `applyDefaults()` 对所有字段用 `== 0` 判断"未设置"，包括 `Temperature`、`ScoreThreshold`、`DistanceThreshold` 等 0 也可以是合法业务值的字段。当前实现约定下，显式设为 0 会被默认值覆盖——这是已知语义歧义，不作为 bug，读写时需注意。

### 前端状态

Vue 3 + Vite + Pinia + Vue Router，`web/` 目录。目前所有 View 都是 "待实现" 占位符——只有 `listDocuments()` 和 `listSkills()` API 调用已连接。Store 定义了数据结构，但组件只渲染骨架。

## Rules

1. 先理解现有代码，再修改。
2. 复杂任务先给出目标、涉及文件、实现思路、验证方式。
3. 优先最小改动，不做无关重构。
4. handler 保持薄，业务逻辑放到 service / agent / tool 层。
5. Prompt、Tool、检索、流式输出尽量解耦。
6. 新增工具时，明确输入、输出、用途、调用时机和错误处理。
7. 不伪造工具结果，不假装测试通过。
8. 修改后说明改动文件、原因和验证方法；没跑测试就明确写未运行测试。
9. 默认用中文说明，代码风格保持与项目一致。

## Engineering Focus

优先保证：Agent 调用链清晰、Tool 输入输出结构化、错误可追踪、流式输出稳定、检索结果与工具结果分离。

## Workflow

1. 阅读需求和相关代码
2. 确认涉及文件
3. 先计划，后实现
4. 做最小可验证修改
5. 格式化 / 测试 / 总结
