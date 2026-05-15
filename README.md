# Eino CTF Agent

基于 [Eino](https://github.com/cloudwego/eino) 框架的 CTF 辅助分析 Agent，支持 RAG 知识库检索、ReAct 工具调用和 IDA Pro 二进制分析。

## 核心能力

- **RAG 知识库**：Markdown / PDF 文档上传、向量化、语义检索
- **ReAct Agent**：多步推理 + 13 个工具（文件分析、命令执行、Python 运行、密码学辅助、远程交互、IDA 分析）
- **Skills 系统**：YAML front-matter 技能文件，关键词触发匹配
- **Mock 模式**：不依赖外部 API 即可本地开发和测试

## 技术栈

| 层 | 技术 |
|---|------|
| 语言 | Go 1.22+ |
| HTTP 框架 | Gin |
| AI 框架 | Eino (CloudWeGo) |
| LLM | DeepSeek V4 Pro（OpenAI 兼容，支持 Mock） |
| Embedding | Qwen text-embedding-v4（DashScope，支持 Mock） |
| 向量存储 | Redis Stack（RediSearch HNSW） |
| 前端 | Vue 3 + Vite（当前为占位状态） |

## 快速启动

```bash
# 1. 安装依赖
go mod download

# 2. 配置环境
cp .env.example .env
# 编辑 .env，填入 API Key（或使用 Mock 模式跳过）

# 3. 启动 Redis Stack
make redis-up

# 4. 启动后端
make run
```

使用 Mock 模式（无需 API Key）：
```bash
# 使用预置 Mock 配置启动
make run-mock
```

### 启动前端（开发联调）

```bash
cd web
npm install
npm run dev
```

前端 dev server 运行在 `http://localhost:5173`，API 请求通过 Vite proxy 自动转发到后端 `:8080`。

详细步骤见 [docs/quickstart.md](docs/quickstart.md)。

## 核心接口

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/health` | 健康检查 |
| `POST` | `/api/chat` | 同步聊天（支持 RAG 和 ReAct 模式） |
| `POST` | `/api/chat/stream` | 流式聊天（SSE） |
| `POST` | `/api/knowledge/upload` | 上传文档（.md / .pdf） |
| `GET` | `/api/knowledge/documents` | 文档列表 |
| `DELETE` | `/api/knowledge/documents/:id` | 删除文档 |
| `GET` | `/api/skills` | Skills 列表 |
| `GET` | `/api/skills/:name` | Skills 详情 |
| `POST` | `/api/skills/reload` | 重新加载 Skills |

完整 API 文档见 [docs/api.md](docs/api.md)。

## 当前项目状态

| 模块 | 状态 |
|------|------|
| 配置管理 + HTTP 服务 | ✅ 完成 |
| 普通聊天 + SSE 流式 | ✅ 完成 |
| RAG 知识库（Markdown + PDF） | ✅ 完成 |
| Skills 系统 | ✅ 完成 |
| ReAct Agent（13 工具） | ✅ 完成 |
| CTF 本地分析工具链 | ✅ 完成 |
| IDA MCP 分析工具 | ✅ 完成 |
| Mock LLM / Embedder | ✅ 完成 |
| 统一错误响应 + 自定义中间件 | 🔶 已实现，部分接入 |
| `internal/agent` 包 | ❌ TODO 骨架（ReAct 逻辑在 service/chat.go） |
| 前端 | ❌ 占位状态 |
| Docker 部署 | ❌ 未完成 |

详见 [docs/project-status.md](docs/project-status.md)。

## 文档索引

| 文档 | 说明 |
|------|------|
| [docs/quickstart.md](docs/quickstart.md) | 快速开始指南 |
| [docs/architecture.md](docs/architecture.md) | 架构说明 |
| [docs/api.md](docs/api.md) | API 接口文档 |
| [docs/project-status.md](docs/project-status.md) | 项目状态详情 |
| [docs/project-phase-spec.md](docs/project-phase-spec.md) | 阶段规格说明 |
| [docs/troubleshooting.md](docs/troubleshooting.md) | 常见问题排查 |

## 命令速查

```bash
make run          # 启动后端
make test         # 运行测试
make fmt          # 代码格式化
make vet          # 静态分析
make redis-up     # 启动 Redis Stack
make redis-down   # 停止 Redis Stack
```
