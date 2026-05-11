# Phase 4：Agent 可观测性最小增强

## 目标

为 React Agent 请求链路增加可观测性：trace ID、关键链路日志、conversation_id 追踪。

## request_id / trace_id 设计

- 每请求生成 8 字节随机 hex 编码（16 字符），函数 `newTraceID()` 定义在 `internal/service/chat.go`
- 在 `Chat()` / `Stream()` 入口生成，透传给 `reactChat()` / `reactStream()` 的所有日志
- 缺失 header 时自动生成，不依赖客户端传入

日志格式：
```
[TRACE] <traceID> Chat mode=react messages=3 conversation="conv-abc"
[TRACE] <traceID> reactChat start
[TRACE] <traceID> reactChat done reply=123chars
```

## conversation_id 当前语义

- `ChatRequest.ConversationID` 字段已在结构体中定义（`json:"conversation_id,omitempty"`）
- **当前仅用于日志追踪**，在 entry 日志中输出
- **不代表服务端已保存会话状态**
- 缺失或为空字符串时兼容旧请求，不影响任何行为

## 多轮对话 messages 的传递方式

- 多轮对话依赖**客户端每次请求传入完整 messages 历史**
- `ToSchemaMessages()` 将全部 `ChatMessage` 转为 Eino `schema.Message`
- `buildAgentInput()` 不会丢弃历史消息：system prompt 前置，原始 messages 全部保留
- 验证：`internal/model/message_test.go::TestToSchemaMessages_KeepsAllMessages` — 4 轮消息全部保留

## 服务端会话持久化

**当前不支持。** 每次 HTTP 请求是独立的：

1. `agent.Generate()` 或 `agent.Stream()` 内部，Eino ReAct Graph 在单次请求中通过 `state.Messages` 累积消息
2. HTTP 响应返回后，Graph state 销毁
3. conversation_id 可用于后续扩展（如 Redis 存储会话历史），但当前不实现

## Agent 初始化失败策略

- 启动期：`InitAgent()` 在 main.go 中显式调用，失败时输出 `[WARN]` 但**不阻止服务启动**
- simple_rag 模式在 Agent 初始化失败时仍可用
- 运行时：`sync.Once` 缓存初始化结果，不会再重试

## 已知风险

1. sync.Once 错误不可恢复 — 启动期已通过 `InitAgent()` 部分缓解
2. 流式 checker 消费中间 chunk — Eino Graph 架构决定，最终回复由 tool 执行后的模型再生层输出
3. ChatStream.Skills/Citations 在 react path 为空 — 前端未消费
4. buildAgentInput 的 ctx 参数未使用 — 预留给后续 context 传递

## 测试覆盖

| 测试 | 文件 | 验证内容 |
|------|------|---------|
| TestLastUserMessage_MultiTurn | model/message_test.go | 多轮消息取最后 user |
| TestLastUserMessage_NoUserFallback | model/message_test.go | 无 user 时退回到最后一条 |
| TestLastUserMessage_Empty | model/message_test.go | nil/空切片返回空 |
| TestToSchemaMessages_KeepsAllMessages | model/message_test.go | 4 轮消息全部保留 |
| TestChatRequest_ConversationID_Empty | model/message_test.go | 空 ID 兼容 |
| TestChatRequest_ConversationID_Present | model/message_test.go | ID 正确赋值 |
| TestNewTraceID_NonEmpty | service/chat_test.go | trace ID 长度 16 |
| TestNewTraceID_Unique | service/chat_test.go | 100 次生成无重复 |
| TestValidateChatRequest_Nil/Empty/Valid | service/chat_test.go | 请求校验边界 |

## 后续 Phase 5 建议

**会话持久化（conversation storage）**：基于 conversation_id 将 Eino state.Messages 持久化到 Redis，实现真正的服务端多轮对话。不改 Agent 逻辑、不新增工具。
