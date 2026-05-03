# API 接口文档

> 本文档会随各 Phase 实现逐步更新。

## 基础

### 健康检查 ✅ Phase 0

```
GET /health
Response: { "status": "ok" }
```

---

## 聊天

### 非流式聊天 ✅ Phase 1

```
POST /api/chat
Body: { "messages": [{ "role": "user", "content": "..." }] }
Response: { "reply": "..." }
```

### 流式聊天 (TODO Phase 2)

```
POST /api/chat/stream
Body: { "messages": [{ "role": "user", "content": "..." }] }
Response: SSE event stream
  event: message_delta  data: { "content": "..." }
  event: done           data: {}
  event: error          data: { "message": "..." }
```

---

## 知识库 (TODO Phase 4A/5.5)

### 上传文档

```
POST /api/knowledge/upload
Content-Type: multipart/form-data
Field: file (.md / .pdf)
Response: { "id": "...", "filename": "...", "status": "pending" }
```

### 文档列表

```
GET /api/knowledge/documents
Response: [{ "id": "...", "filename": "...", "status": "indexed", "chunk_count": 12 }]
```

### 删除文档

```
DELETE /api/knowledge/documents/:id
Response: 204 No Content
```

---

## Skills (TODO Phase 6/8)

### Skill 列表

```
GET /api/skills
Response: [{ "name": "...", "description": "...", "triggers": [...] }]
```

### Skill 详情

```
GET /api/skills/:name
Response: { "name": "...", "description": "...", "body": "..." }
```

### 重新加载 Skills

```
POST /api/skills/reload
Response: { "count": 5 }
```
