# 常见问题排查

## Redis Stack 和普通 Redis 的区别

本项目使用 Eino Redis Indexer/Retriever，底层依赖 RediSearch 模块的向量检索能力（`FT.CREATE` 和 `FT.SEARCH` 命令）。

- **Redis Stack**：包含 RediSearch、RedisJSON、RedisGraph 等模块，开箱即用
- **普通 Redis**：不包含 RediSearch，执行 `FT.CREATE` 会报错

本地开发必须使用 Redis Stack。推荐通过 `make redis-up` 启动 Docker Redis Stack。

## FT.INFO / FT.CREATE unknown command

```text
[FATAL] failed to prepare redis vector index: FT.CREATE unknown command
```

**原因**：连接的是普通 Redis Server，缺少 RediSearch 模块。

**解决**：
1. 确认使用 Redis Stack：`redis-cli MODULE LIST | grep search`
2. 或使用 Docker：`make redis-up`
3. 清理已有的错误数据：`redis-cli FLUSHDB`（仅开发环境）

## API Key 缺失

```text
[FATAL] failed to create chat model: LLM API key not set: environment variable DEEPSEEK_API_KEY is empty
```

**原因**：`.env` 文件未配置或配置的 Key 无效。

**解决方式一（配置真实 Key）**：
```bash
cp .env.example .env
# 编辑 .env，填入 DEEPSEEK_API_KEY 和 DASHSCOPE_API_KEY
```

**解决方式二（使用 Mock 模式）**：
```yaml
# configs/config.yaml
llm:
  provider: mock
embedding:
  provider: mock
```

Mock 模式下无需任何 API Key，所有接口均可使用。

## 端口被占用

```text
[FATAL] server failed: listen tcp 0.0.0.0:8080: bind: address already in use
```

**原因**：8080 端口已被占用。

**解决**：修改 `configs/config.yaml` 中 `server.port` 为其他端口，或终止占用进程。

## WSL / Docker 常见问题

### Redis Stack 无法启动

```bash
docker compose ps  # 检查容器状态
docker compose logs redis-stack  # 查看 Redis 日志
```

### WSL2 中 Redis 端口无法从宿主机访问

在 `docker-compose.yml` 中已经绑定了 `127.0.0.1`，默认只允许本机访问。如需跨主机访问，修改为：

```yaml
ports:
  - "6379:6379"    # 移除 127.0.0.1 前缀（注意安全风险）
```

### Docker 服务未运行

```bash
sudo service docker start   # WSL2 Ubuntu
# 或
sudo systemctl start docker
```

## IDA MCP 是可选能力

启动时如看到以下日志，不影响基础功能：

```text
[WARN] IDA MCP endpoint invalid (): ... — IDA tools will be unavailable
```

这是正常情况。IDA MCP 需要额外的 IDA Pro + MCP 插件环境。不用 IDA 分析功能时可以忽略。

## 前端目前仍为占位状态

如果访问 `http://localhost:5173` 看到的页面功能不完整——这是正常的。前端 Vue 3 工程的 View 和 Component 当前为占位状态。后端 API 完全可用，可以通过 curl 或 Postman 测试。

## 配置文件未找到

```text
[FATAL] failed to load config: read config file configs/config.yaml: ...
```

**解决**：
```bash
cp configs/config.example.yaml configs/config.yaml
# 然后按需修改配置
```

## go test 网络相关失败

如果测试中涉及网络调用（如 LLM API），确保：
1. 使用 mock provider 配置
2. 或设置有效的 API Key 环境变量

当前大部分测试是纯单元测试，不需要外部依赖。`internal/tool` 包的测试可能涉及文件系统操作，确保 `/tmp` 目录可写。
