# 快速开始指南

## 1. 环境要求

- **Go** 1.22+
- **Docker**（用于运行 Redis Stack）
- **可选**：DeepSeek API Key（LLM）、DashScope API Key（Embedding）

不使用 Docker 时，也可自行安装 [Redis Stack](https://redis.io/docs/latest/operate/oss_and_stack/install/install-stack/)。

## 2. 配置 .env

```bash
cp .env.example .env
```

### 使用真实 API（生产/完整功能）

编辑 `.env`，填入有效的 API Key：

```ini
DEEPSEEK_API_KEY=sk-your-deepseek-api-key
DASHSCOPE_API_KEY=sk-your-dashscope-api-key
REDIS_PASSWORD=
```

### 使用 Mock 模式（本地开发/测试）

不需要任何 API Key。编辑 `configs/config.yaml`（或使用 `configs/config.example.yaml`）修改：

```yaml
llm:
  provider: mock          # 原是 deepseek

embedding:
  provider: mock          # 原是 dashscope
```

Mock LLM 返回固定文本，Mock Embedder 生成确定性向量。完整功能链路可跑通。

## 3. 启动 Redis Stack

```bash
make redis-up
```

验证 Redis 是否正常：

```bash
docker compose exec redis-stack redis-cli PING
# 应返回 PONG
```

RedisInsight 管理界面：http://localhost:8001

## 4. 启动后端

```bash
# 使用默认配置
make run

# 或使用 example 配置
make run-example
```

启动成功时日志类似：

```
[INFO] starting server at 0.0.0.0:8080
```

## 5. 验证接口

### 健康检查

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

### 同步聊天

```bash
curl -s -X POST http://localhost:8080/api/chat \
  -H 'Content-Type: application/json' \
  -d '{"messages":[{"role":"user","content":"hello"}]}'
# {"reply":"This is a mock response from the LLM.","citations":null,"skills":null}
```

### 流式聊天（SSE）

```bash
curl -N -X POST http://localhost:8080/api/chat/stream \
  -H 'Content-Type: application/json' \
  -d '{"messages":[{"role":"user","content":"hello"}]}'
```

SSE 事件输出：

```
event:skill_used
data:{"skills":null}

event:message_delta
data:{"content":"This is a mock streaming response."}

event:done
data:{}
```

### 上传文档

```bash
echo "# Test Doc\n\nHello World" > /tmp/test.md
curl -X POST http://localhost:8080/api/knowledge/upload \
  -F "file=@/tmp/test.md"
```

### 文档列表

```bash
curl http://localhost:8080/api/knowledge/documents
```

## 6. 常见启动失败原因

| 现象 | 原因 | 解决 |
|------|------|------|
| `[FATAL] failed to connect redis` | Redis Stack 未启动 | `make redis-up` |
| `FT.CREATE unknown command` | 使用了普通 Redis 而非 Redis Stack | 见 [troubleshooting.md](troubleshooting.md) |
| `LLM API key not set` | `.env` 中未配置 API Key | 配置 `.env` 或切换为 mock provider |
| `no such file: configs/config.yaml` | 配置文件缺失 | `cp configs/config.example.yaml configs/config.yaml` |
| IDA MCP 相关 WARN 日志 | IDA MCP 是可选组件 | 不影响基础功能，见 [troubleshooting.md](troubleshooting.md) |
