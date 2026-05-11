# Go 后端架构与代码质量审查建议

## 一、总体结论

**项目整体架构清晰**：handler（薄）→ service（编排）→ Eino 组件 的分层符合 CLAUDE.md 的设计意图。Eino ChatModel / Embedder / Indexer / Retriever 均已正确集成，Redis 向量检索链路完整。

**三个突出问题**：

1. **全项目零函数文档注释** — 39 个 .go 文件中，有实际业务代码的约 26 个文件，没有任何一个导出函数有 Go doc 注释。仅 `rag_prompt.go:70` 有一行英文注释。
2. **存在一处明确重复造轮子** — `filterRetrievedDocs` 对 Retriever 已按 `WithScoreThreshold` 过滤的结果做了二次相同阈值的过滤，完全冗余。
3. **多处命名/职责小问题** — 不影响运行但影响可读性，集中于 `metadata.go`、`rag.go`、`skill/registry.go`。

---

## 二、可以改用 Eino 原生能力的逻辑清单

### 2.1 确定应改（高置信度）

| # | 位置 | 当前实现 | 问题 | 建议 |
|---|------|----------|------|------|
| 1 | `internal/service/rag.go:77` `filterRetrievedDocs` | 对检索结果用 `cfg.RAG.ScoreThreshold` 做二次过滤 | `einoretriever.WithScoreThreshold` 在 `rag.go:72` 已经传入相同阈值，Redis Retriever 内部已过滤。二次过滤是纯粹冗余代码 | **删除 `filterRetrievedDocs` 调用和函数定义**（约 10 行） |

### 2.2 建议改但有注意事项

| # | 位置 | 当前实现 | Eino 替代方案 | 风险 / 争议 |
|---|------|----------|---------------|-------------|
| 2 | `internal/knowledge/markdown.go` `ParseMarkdown` + `TextSplitter` | 自实现 Markdown 解析、heading 追踪、字符级切分 | Eino 有 `components/document` 包提供 Loader + Splitter | **不建议改**。当前实现有特殊逻辑：heading 层级追踪生成 `A > B > C` 路径、Front matter 剥离、自定义 chunk 重叠策略。Eino 的通用 splitter 无法直接产出 `HeadingPath` 语义。强行改用反而需要更多适配代码 |
| 3 | `internal/service/rag.go` 整体 `buildMessages` 方法 | 手工编排 retrieve → filter → match skills → build prompt → generate | Eino `compose.Graph` / `compose.Chain` 可将这些步骤建模为节点和边 | **暂不建议改**。CLAUDE.md 已注明这是 TODO 规划（Phase 7 Agent Runner），当前 simple_rag 模式是有意为之的过渡方案。等 Agent 模式落地时统一重构更合理 |

### 2.3 不应改

| # | 位置 | 理由 |
|---|------|------|
| 4 | SSE Writer (`internal/pkg/sse/writer.go`) | Eino 不提供 HTTP SSE 输出层。`schema.StreamReader` 已正确使用 |
| 5 | Skill 匹配/路由 (`internal/skill/router.go`) | 纯业务逻辑，Eino 无等效组件 |
| 6 | `toSchemaRole` / `toSchemaMessages` (`internal/service/chat.go`) | 薄适配层，Eino 不会替你转换自定义 model 到 schema.Message |

---

## 三、注释不规范的文件和函数清单

### 3.1 完全无注释的实际代码文件（26 个）

以下文件中有导出函数/类型，但**没有任何 Go doc 注释**：

```
internal/config/config.go          — Load, Config, 所有 Config 子结构体, GetLLMAPIKey 等 6 个导出方法
internal/llm/factory.go            — NewChatModel
internal/llm/deepseek.go           — NewDeepSeekChatModel
internal/embedding/factory.go      — NewEmbedder
internal/embedding/qwen.go         — NewQwenEmbedder
internal/embedding/embedder.go     — Embedder 类型别名
internal/knowledge/keys.go         — KeyPrefix, DocumentSetKey, DocumentKey, ChunkKeyPrefix, ChunkKey, VectorField
internal/knowledge/redis.go        — NewRedisClient, EnsureVectorIndex
internal/knowledge/components.go   — NewRedisIndexer, NewRedisRetriever, RedisDocumentToSchema
internal/knowledge/metadata.go     — MetadataRepo, ErrDocumentNotFound, NewMetadataRepo, MetadataString, MetadataInt
internal/knowledge/markdown.go     — ParseMarkdown, NewTextSplitter, MarkdownBlock, TextChunk, TextSplitter
internal/knowledge/service.go      — Service, NewService, UploadMarkdown, ListDocuments, DeleteDocument
internal/service/chat.go           — ChatService, ChatStream, NewChatService, Chat, Stream
internal/service/rag.go            — RAGService, NewRAGService, Generate, Stream
internal/service/skill.go          — SkillService, NewSkillService, List, Get, Reload
internal/prompt/rag_prompt.go      — BuildRAGSystemPrompt（唯一有注释的：L70 有一行英文）
internal/handler/chat.go           — ChatHandler, NewChatHandler, Chat, Stream
internal/handler/knowledge.go      — KnowledgeHandler, NewKnowledgeHandler, Upload, ListDocuments, DeleteDocument
internal/handler/skill.go          — SkillHandler, NewSkillHandler, List, Get, Reload
internal/handler/health_handler.go — HealthHandler, NewHealthHandler, Health
internal/skill/model.go            — Skill, WithoutBody
internal/skill/registry.go         — Registry, NewRegistry, Reload, ListAll, GetByName, MustGetByName
internal/skill/loader.go           — Loader, NewLoader, LoadAll, Load
internal/skill/router.go           — Router, NewRouter, Match
internal/tool/skill_reader.go      — SkillReader, NewSkillReader, Read
internal/middleware/cors.go        — CORS
internal/router/router.go          — Setup
internal/pkg/sse/writer.go         — Writer, NewWriter, SetHeaders, Event
internal/pkg/security/security.go  — ValidSkillName, SafeJoin, ForbiddenSensitiveFile
```

### 3.2 仅有 TODO 注释的骨架文件（14 个）

这些文件要么是空文件，要么只有 `// TODO Phase X: ...` 占位符，属于规划中但未实现的模块：

```
internal/agent/runner.go
internal/agent/simple_rag_runner.go
internal/agent/tool_agent_runner.go
internal/agent/react_runner.go
internal/agent/trace.go
internal/tool/knowledge_search.go
internal/tool/registry.go
internal/prompt/skill_prompt.go
internal/prompt/agent_prompt.go
internal/middleware/logger.go
internal/middleware/recovery.go
internal/pkg/response/response.go
internal/errors/errors.go
internal/handler/chat_stream.go         (空文件)
```

### 3.3 `rag_prompt.go:70` — 唯一存在注释的行

```go
// Rough token budget: keep the logic deterministic and dependency-free.
```

这是英文注释。项目规范要求中文注释，应改为中文。

---

## 四、命名与可读性问题清单

### 4.1 命名问题

| # | 文件:行 | 问题 | 建议 |
|---|---------|------|------|
| 1 | `internal/model/message.go` | 文件名 `message.go` 但包含 `ChatRequest`, `ChatResponse`, `ErrorResponse`, `SkillRef`，不止 message | 重命名为 `chat.go`，或将 `ErrorResponse` 和 `SkillRef` 拆分 |
| 2 | `internal/skill/registry.go:58` `MustGetByName` | Go 惯例中 `MustXXX` 表示 panic on error，但这个函数返回 `(Skill, error)` | 重命名为 `GetByNameOrError` 或直接去掉此方法（仅 `tool/skill_reader.go` 一处调用，可以用 `GetByName`） |
| 3 | `internal/embedding/embedder.go` | 仅一行类型别名 `type Embedder = einoembedding.Embedder` | 考虑删除此文件，直接在 `factory.go` 和 `qwen.go` 中使用 `einoembedding.Embedder`。当前增加了一层无意义的间接 |
| 4 | `internal/knowledge/metadata.go` | 文件名 `metadata.go` 暗示 Redis 文档元数据，但其中的 `MetadataString` / `MetadataInt` 是通用的 `schema.Document` metadata 读取工具 | 这些工具函数可以考虑移到 `internal/model` 或单独文件 `doc_helpers.go` |
| 5 | `internal/knowledge/components.go` | 文件名 `components.go` 太泛，实际内容是 Redis Indexer / Retriever 工厂 + 文档转换 | 建议重命名为 `redis_components.go` |
| 6 | `internal/service/rag.go:104` `filterRetrievedDocs` 参数名 `docs` | 与 `schema.Document` 类型名重名，且与项目内部 `model.Document` 容易混淆 | 改为 `results` 或 `chunks` |
| 7 | `internal/service/rag.go:129` `toSkillRefs` 循环变量 `s` | 接收者也是 `s`（RAGService），循环内 `s` 遮蔽外层概念，容易误读 | 改为 `sk` 或 `skill` |
| 8 | `internal/service/chat.go:71` `toSchemaMessages` 参数 `messages` | 参数类型是 `[]appmodel.ChatMessage`，返回值是 `[]*schema.Message`，两者都叫 messages | 参数改为 `chatMessages`，或返回值变量改为 `out`（已是） |

### 4.2 可读性问题

| # | 文件:行 | 问题 | 建议 |
|---|---------|------|------|
| 9 | `internal/service/rag.go:77` | 冗余过滤（见 2.1）降低代码意图清晰度 | 删除 |
| 10 | `internal/knowledge/metadata.go:197` `parseInt` | 解析失败静默返回 0，调用方无法区分 "chunk index 真的为 0" 和 "解析出错" | 至少改为返回 `(int, error)` 让调用方决策，或加日志 |
| 11 | `internal/knowledge/components.go:114` `scoreFromDistance` | 函数名暗示通用距离→分数转换，但公式 `1 - distance` 仅对 COSINE 距离正确，却无注释说明 | 加注释说明这是 COSINE distance 转 similarity，或将函数改名为 `cosineDistanceToScore` |
| 12 | `internal/config/config.go:114` `applyDefaults` | 全字段用 `== 0` 判断"未设置"，对 `Temperature`、`ScoreThreshold`、`DistanceThreshold` 这些 0 是合法值的字段存在语义歧义 | 短期不改（保持最小改动），但应加注释说明"当前设计下 0 视为使用默认值" |
| 13 | `internal/knowledge/service.go:110` `enqueueMarkdownIndex` | 后台 goroutine 的 context 用 `context.Background()` + 30min timeout，与请求 context 脱钩是设计意图但无注释 | 加一行注释说明"索引与请求生命周期解耦" |
| 14 | `internal/handler/chat.go:67-77` | 流式 handler 中先发 skills、再发 citations、再循环 chunk，循环内有 `select` 检查 context 取消。这个顺序的业务含义（为什么 skill_used 先于 citation）无注释 | 加简短注释说明 SSE 事件发送顺序的设计意图 |
| 15 | `internal/handler/knowledge.go:43` | `doc != nil` 时返回 `doc` 而非 `ErrorResponse`，这与其他 handler 的错误处理模式不一致 | 统一错误响应格式，或加注释说明为何 Upload 特殊处理 |

### 4.3 职责边界问题

| # | 位置 | 问题 | 建议 |
|---|------|------|------|
| 16 | `internal/service/rag.go:139` `lastUserMessage` | 通用消息工具函数放在 RAG service 中 | 移到 `internal/model/message.go` |
| 17 | `internal/service/chat.go:71-95` `toSchemaMessages` / `toSchemaRole` | 模型转换逻辑放在 service 层 | 移到 `internal/model/message.go` 或独立 `internal/adapter` 包 |
| 18 | `internal/knowledge/keys.go:36` `VectorField` | Redis key/field 生成函数中夹了一个向量字段名获取函数 | 移到 `metadata.go`（与 `RedisVectorField` 常量相邻） |
| 19 | `internal/handler/chat_stream.go` | 空文件 | 删除，或合并到 `chat.go` |

---

## 五、建议修改优先级

### P0 — 应立即修复（影响正确性/有明显冗余）

1. **删除 `filterRetrievedDocs`**（`internal/service/rag.go:104-115`）及其调用（`:77`）— 纯冗余代码

### P1 — 建议近期修复（影响可读性/可维护性）

2. **给所有导出函数/类型加中文 Go doc 注释** — 覆盖全部 26 个有实际代码的文件
3. **重命名 `MustGetByName`** → `GetByNameOrError`（`internal/skill/registry.go:58`）
4. **`parseInt` 改为不静默吞错误**（`internal/knowledge/metadata.go:197`）
5. **移动 `lastUserMessage`** 到 `internal/model/message.go`
6. **移动 `toSchemaMessages` / `toSchemaRole`** 到合适位置
7. **删除空文件** `internal/handler/chat_stream.go`

### P2 — 可择机改进（改善结构但不紧急）

8. 重命名 `internal/model/message.go` → `chat.go`
9. 重命名 `internal/knowledge/components.go` → `redis_components.go`
10. 删除或内联 `internal/embedding/embedder.go` 的类型别名
11. `scoreFromDistance` 加注释或改名
12. Config `applyDefaults` 零值歧义加注释

### P3 — 等待合适时机（与 Phase 7/8/9 一起做）

13. `internal/service/rag.go` 手工编排 → Eino Graph/Chain — 等 Agent Runner 实现时统一重构
14. TODO 骨架文件（14 个）— 按 Phase 规划逐个实现
15. Markdown 解析是否改用 Eino document loader — 需要评估 HeadingPath 需求是否可放弃

---

## 六、涉及文件列表

### 需要修改的文件（P0+P1）

| 文件 | 修改类型 |
|------|----------|
| `internal/service/rag.go` | 删除 `filterRetrievedDocs`，移动 `lastUserMessage`、`toSkillRefs` |
| `internal/service/chat.go` | 移动 `toSchemaMessages`/`toSchemaRole`，添加注释 |
| `internal/model/message.go` | 接收移动过来的工具函数，添加注释 |
| `internal/skill/registry.go` | 重命名 `MustGetByName` |
| `internal/knowledge/metadata.go` | `parseInt` 改为不静默吞错误 |
| `internal/handler/chat_stream.go` | 删除空文件 |

### 需要添加注释的文件（全部 26 个有实际代码的文件）

```
internal/config/config.go
internal/llm/factory.go
internal/llm/deepseek.go
internal/embedding/factory.go
internal/embedding/qwen.go
internal/embedding/embedder.go
internal/knowledge/keys.go
internal/knowledge/redis.go
internal/knowledge/components.go
internal/knowledge/metadata.go
internal/knowledge/markdown.go
internal/knowledge/service.go
internal/service/chat.go
internal/service/rag.go
internal/service/skill.go
internal/prompt/rag_prompt.go
internal/handler/chat.go
internal/handler/knowledge.go
internal/handler/skill.go
internal/handler/health_handler.go
internal/skill/model.go
internal/skill/registry.go
internal/skill/loader.go
internal/skill/router.go
internal/tool/skill_reader.go
internal/middleware/cors.go
internal/router/router.go
internal/pkg/sse/writer.go
internal/pkg/security/security.go
```

### 有争议/待评估的文件

| 文件 | 争议点 |
|------|--------|
| `internal/knowledge/markdown.go` | 是否改用 Eino document loader — **建议不改**，HeadingPath 是定制需求 |
| `internal/knowledge/components.go` | 文件名是否改名 — 低优先级 |
| `internal/embedding/embedder.go` | 类型别名是否删除 — 需确认是否有其他包依赖此别名 |
