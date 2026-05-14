# API 接口文档

基础地址：`http://localhost:8080`

---

## 健康检查

### GET /health

**响应** `200 OK`

```json
{"status": "ok"}
```

---

## 聊天

### POST /api/chat

同步聊天，支持 Simple RAG 和 ReAct Agent 两种模式（由 `agent.mode` 配置控制）。

**请求**

```json
{
  "conversation_id": "optional-trace-id",
  "messages": [
    {"role": "user", "content": "hello"}
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `messages` | array | 是 | 消息列表，至少 1 条 |
| `messages[].role` | string | 是 | `user` / `assistant` / `system` |
| `messages[].content` | string | 是 | 消息内容 |
| `conversation_id` | string | 否 | 会话追踪 ID |

**响应** `200 OK`

```json
{
  "reply": "回复文本...",
  "citations": [
    {
      "content": "引用的知识库文本...",
      "filename": "example.md",
      "chunk_index": 0,
      "score": 0.85
    }
  ],
  "skills": [
    {
      "name": "web_recon",
      "title": "Web信息收集",
      "description": "收集目标Web信息"
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `reply` | string | 模型回复文本 |
| `citations` | array | 知识库引用列表（RAG 模式） |
| `skills` | array | 匹配到的 Skills 列表 |

**错误响应**

```json
{
  "error": "chat_error",
  "message": "具体错误信息"
}
```

常见错误码：`invalid_request`、`chat_error`、`llm_failed`。

### POST /api/chat/stream

流式聊天（SSE），事件序列如下：

```
event: skill_used
data: {"skills": [{"name": "web_recon", ...}]}

event: citation
data: {"content": "...", "filename": "...", "chunk_index": 0, "score": 0.85}

event: message_delta
data: {"content": "逐块返回的文本..."}

event: done
data: {}
```

错误时会发送：

```
event: error
data: {"error": "stream_error", "message": "..."}

event: done
data: {}
```

**请求体格式** 同 `POST /api/chat`。

**curl 示例**

```bash
curl -N -X POST http://localhost:8080/api/chat/stream \
  -H 'Content-Type: application/json' \
  -d '{"messages":[{"role":"user","content":"介绍一下这个项目"}]}'
```

---

## 知识库

### POST /api/knowledge/upload

上传文档（支持 `.md`、`.pdf`）。

**请求** `multipart/form-data`

| 字段 | 类型 | 说明 |
|------|------|------|
| `file` | file | 文档文件 |

**响应** `202 Accepted`

```json
{
  "id": "abc123",
  "filename": "writeup.md",
  "file_type": "markdown",
  "status": "pending",
  "chunk_count": 0,
  "created_at": "2026-05-14T10:00:00Z"
}
```

文档状态流转：`pending → parsing → chunking → embedding → indexed`（失败时为 `failed`）。

### GET /api/knowledge/documents

获取所有文档列表。

**响应** `200 OK`

```json
[
  {
    "id": "abc123",
    "filename": "writeup.md",
    "file_type": "markdown",
    "status": "indexed",
    "chunk_count": 12,
    "created_at": "2026-05-14T10:00:00Z"
  }
]
```

### DELETE /api/knowledge/documents/:id

删除文档及其所有 chunk 数据。

**响应** `204 No Content`

---

## Skills

### GET /api/skills

获取所有 Skills 列表。

**响应** `200 OK`

```json
[
  {
    "name": "web_recon",
    "title": "Web 信息收集",
    "description": "收集目标 Web 信息",
    "triggers": ["web", "http", "扫描"],
    "enabled": true,
    "priority": 10
  }
]
```

### GET /api/skills/:name

获取指定 Skill 详情（含完整 body）。

**响应** `200 OK`

```json
{
  "name": "web_recon",
  "title": "Web 信息收集",
  "description": "收集目标 Web 信息",
  "body": "# Web 信息收集\n\n## 步骤\n...",
  "triggers": ["web", "http"],
  "priority": 10
}
```

**错误**

| 状态码 | 错误码 | 说明 |
|--------|--------|------|
| 400 | `invalid_skill_name` | Skill 名称格式非法 |
| 404 | `skill_not_found` | Skill 不存在 |

### POST /api/skills/reload

重新从 `data/skills/` 目录加载所有 Skills。

**响应** `200 OK`

```json
{"status": "ok"}
```

**错误**

| 状态码 | 错误码 | 说明 |
|--------|--------|------|
| 500 | `reload_skills_failed` | 重新加载失败 |
