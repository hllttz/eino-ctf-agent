# 架构说明

## 总体调用链路

```
HTTP Request
  → Gin Router
  → Handler（请求解析/响应写入，薄层）
  → Service（业务编排） ← Config（配置）
      ├─ Simple RAG：RAGService → Retriever → Skill Router → Prompt → ChatModel
      └─ ReAct Agent：ChatService → react.NewAgent(ToolCallingChatModel + Tools)
           Agent 循环：reasoning → tool_call → observation → ... → final_answer
  → Redis（向量检索 + 文档元数据）
```

## 核心启动链路（cmd/server/main.go）

1. 加载 `configs/config.yaml` + `.env`
2. 创建 Eino ChatModel（DeepSeek 或 Mock）
3. 创建 Eino ToolCallingChatModel（Agent 独占，与普通 ChatModel 分离）
4. 创建 Eino Embedder（Qwen DashScope 或 Mock）
5. 连接 Redis Stack，Ping + `EnsureVectorIndex`（RediSearch HNSW）
6. 创建 Eino Redis Indexer / Retriever → knowledge.Service
7. 加载 `data/skills/` → skill.Registry → skill.Router
8. 注册 13 个 Agent 工具到 tool.Registry
9. 组装 RAGService → ChatService → Handler → Gin Router → 启动

## 目录结构

```
eino_ctf_agent/
├── cmd/server/main.go           # 入口：组装全链路
├── configs/                     # YAML 配置文件
├── data/
│   ├── docs/                    # 上传的文档原文件
│   └── skills/                  # Skills .md 文件
├── docs/                        # 项目文档
├── internal/
│   ├── agent/                   # TODO 骨架：规划中 Agent Runner（当前未使用）
│   ├── config/                  # YAML 加载 + 默认值 + 校验
│   ├── embedding/               # Embedder 工厂（qwen / mock）
│   ├── errors/                  # 业务错误码 + AppError 结构体
│   ├── handler/                 # HTTP Handler（薄层）
│   ├── knowledge/               # 知识库：解析/切块/Redis/Indexer/Retriever
│   ├── llm/                     # ChatModel 工厂（deepseek / mock）
│   ├── middleware/               # CORS / Logger / Recovery
│   ├── model/                   # 请求/响应模型 + 消息工具函数
│   ├── pkg/
│   │   ├── response/            # 统一响应 helper（OK/Error/...）
│   │   ├── security/            # Skill 名校验、安全路径拼接
│   │   └── sse/                 # SSE Writer
│   ├── prompt/                  # RAG/Agent System Prompt 构建
│   ├── router/                  # Gin 路由注册
│   ├── service/                 # ChatService / RAGService / SkillService
│   ├── skill/                   # Skill model / loader / registry / router
│   └── tool/                    # Agent 工具实现（13 个工具）
└── web/                         # Vue 3 前端（当前为占位状态）
```

## 关键设计

### ReAct Agent 主逻辑位置

当前 ReAct Agent 的核心逻辑在 **`internal/service/chat.go`** 中：

- `reactChat()` — 同步 React Agent 调用
- `reactStream()` — 流式 React Agent 调用
- `getReactAgent()` — 惰性初始化 Eino React Agent（sync.Once）
- `streamToolCallChecker()` — DeepSeek thinking 模式工具调用检测

**`internal/agent/` 包当前是 TODO 骨架**，5 个文件均只有注释：
- `runner.go` — "TODO Phase 7: Agent Runner 接口定义"
- `simple_rag_runner.go` — "TODO Phase 5: Simple RAG Runner"
- `tool_agent_runner.go` — "TODO Phase 7: Tool-Augmented Agent Runner"
- `react_runner.go` — "TODO Phase 7.5: ReAct Agent Runner"
- `trace.go` — "TODO Phase 7.5: Agent 调用追踪"

后续重构方向是将 `service/chat.go` 中的 ReAct 逻辑迁入 `internal/agent/`，但当前不做。

### 双模式路由

`agent.mode` 配置控制聊天行为：

- `simple_rag`：检索 → Skill 匹配 → Prompt 构建 → ChatModel 生成（不走 Agent）
- `react`：Eino ReAct Agent 多步推理 + 工具调用

两个模式共用同一套 Tool Registry 和 Skill Router。

### Redis Key 约定

默认 key prefix 为 `eino_ctf_agent:`：

```
eino_ctf_agent:documents               # 文档 ID 集合（Set）
eino_ctf_agent:doc:<id>                # 文档元数据（Hash）
eino_ctf_agent:chunk:<doc_id>:<n>      # Chunk 内容 + 向量（Hash）
idx:eino_ctf_agent_chunks              # RediSearch HNSW 向量索引
```

### 文档入库链路

```
HTTP Upload → 保存原文件 → Redis 元数据(pending)
→ 后台 goroutine（最多2并发）
  → 解析(Markdown/PDF) → 切块(chunk_size=512, overlap=64)
  → Embedder 生成向量 → Eino Redis Indexer 写入
  → 状态更新为 indexed / failed
```

### 安全边界（工具执行）

- 所有路径限制在工作目录内（`resolvePath` + `isPathSafe`）
- 禁止绝对路径和 `../` 穿越
- 命令执行使用 27 个 allowlist
- Python 执行：最小环境变量 + timeout（默认5s，最大20s）+ 输出截断（100KB）
- IDA MCP endpoint 只允许 `127.0.0.1` 或 `localhost`

### 设计原则

1. 优先使用 Eino/eino-ext 已有组件，不重复造轮子
2. Handler 保持薄层，业务逻辑在 Service / Tool 层
3. 知识库实现集中在 `internal/knowledge`，不分散为 parser/splitter/store 碎片包
4. Redis 是当前唯一存储，后续扩展优先使用 Eino 组件或薄适配层
