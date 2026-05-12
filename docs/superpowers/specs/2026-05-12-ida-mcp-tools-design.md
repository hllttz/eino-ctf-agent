# Phase 6: IDA MCP 只读二进制分析工具链 — 设计

## 目标

让 Agent 在遇到 ELF / PE / Mach-O 等二进制文件时，可以结合本地工具和 IDA MCP 做初步逆向分析。

最小闭环：
1. `file_info` 判断文件类型
2. `command_executor` 执行 file / strings / readelf / objdump 等基础命令
3. 如果 IDA MCP 可用，调用 IDA MCP 获取函数、字符串、反编译结果、交叉引用
4. 汇总文件类型、架构、保护机制、关键字符串、可疑函数、潜在漏洞点

## 非目标

- 完整异步 SSE session 管理（留到后续阶段）
- 自动重连、启动/控制 IDA GUI、执行 IDA Python
- patch binary、remote pwn、socket 交互、自动 exploit

## 接口抽象

```go
// IDAMCPClient 封装对 IDA MCP 服务的只读分析调用。
// 工具层只依赖此接口，不关心底层是 SSE、HTTP 还是 Mock 传输。
type IDAMCPClient interface {
    Status(ctx context.Context) (*IDAMCPStatus, error)
    Functions(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error)
    Decompile(ctx context.Context, target string) (*IDAMCPDecompileResult, error)
    Strings(ctx context.Context, limit int) (*IDAMCPStringsResult, error)
    Xrefs(ctx context.Context, target string) (*IDAMCPXrefsResult, error)
}
```

## 两种实现

| 实现 | 文件 | 用途 |
|------|------|------|
| `RealMCPClient` | `ida_mcp_client.go` | 生产：endpoint 安全校验 + Status 真实探测；其余方法返回 transport pending 错误 |
| `MockMCPClient` | `ida_mcp_client.go` | 测试：返回预设数据 |

### RealMCPClient 当前阶段能力

- `NewRealMCPClient(endpoint, timeout)`：校验 endpoint（只允许 localhost/127.0.0.1），非法返回 nil+error
- `Status(ctx)`：HTTP GET 探测 endpoint 是否可达，收到响应头即 close Body，不读完整 SSE stream
- 其余 4 个方法返回 `"transport not fully implemented; IDA MCP SSE client pending"`

### MockMCPClient

- 所有方法返回预设数据
- 测试代码可注入自定义返回值

## 5 个工具

| 工具名 | 接口方法 | 输入 | 输出 |
|--------|---------|------|------|
| `ida_status` | `Status(ctx)` | 无 | `available`, `endpoint`, `error`, `truncated` |
| `ida_functions` | `Functions(ctx, limit)` | `limit`（默认200） | `functions[]`（JSON 文本）, `total`, `truncated`, `error` |
| `ida_decompile` | `Decompile(ctx, target)` | `target`（函数名/地址） | `code`（伪代码文本）, `truncated`, `error` |
| `ida_strings` | `Strings(ctx, limit)` | `limit`（默认200） | `strings[]`（JSON 文本）, `total`, `truncated`, `error` |
| `ida_xrefs` | `Xrefs(ctx, target)` | `target`（函数名/地址） | `xrefs[]`（JSON 文本）, `truncated`, `error` |

所有输出均以 JSON 文本形式返回，经过 100KB 截断。

## 配置

```go
// 环境变量，有默认值
IDA_MCP_ENDPOINT        = "http://127.0.0.1:13337/sse"
IDA_MCP_TIMEOUT_SECONDS = 5
```

## 安全边界

- endpoint 只允许 `localhost` 或 `127.0.0.1`（IPv4 only，::1 当前不支持，注释说明）
- 拒绝 `0.0.0.0`、远程 IP、远程域名
- 所有请求有 timeout
- 所有输出 100KB 截断 + `truncated=true`
- 只读分析，不 patch、不执行脚本、不控制调试器
- 日志不打印完整反编译文本
- client 不可用时返回错误，不 panic
- main.go 中 IDA MCP 不可用时不影响服务启动

## 工具注册与 Prompt

- main.go：读环境变量 → 创建 RealMCPClient → endpoint 非法记 warning → 注册 5 个工具
- Agent prompt 追加 IDA 工具说明：
  - 先 `file_info` / `command_executor` 做基础分析
  - 使用 IDA 工具前先调用 `ida_status`
  - 不可用时回退到 strings / readelf / objdump
  - 不要一次性反编译所有函数

## 文件变更

| 文件 | 操作 |
|------|------|
| `internal/tool/ida_mcp_client.go` | 新增：接口 + RealMCPClient + MockMCPClient + endpoint 校验 |
| `internal/tool/ida_mcp_tools.go` | 新增：5 个工具构造函数 |
| `internal/tool/ida_mcp_test.go` | 新增：Mock 驱动测试 |
| `cmd/server/main.go` | 修改：创建 client → 注册 IDA 工具 |
| `internal/prompt/agent_prompt.go` | 修改：追加 IDA 工具说明 |

## 测试覆盖

| 测试 | 内容 |
|------|------|
| `TestIDAEndpointDefault` | 默认 endpoint = `http://127.0.0.1:13337/sse` |
| `TestIDAEndpointRejectRemote` | 拒绝远程 IP/域名 |
| `TestIDAEndpointAllowLocalhost` | 允许 localhost / 127.0.0.1 变体 |
| `TestIDAStatusAvailable` | Mock 返回 available=true |
| `TestIDAStatusUnavailable` | Mock 返回 available=false |
| `TestIDAFunctionsTruncated` | 输出超 100KB 截断 |
| `TestIDADecompileNormal` | 正常反编译返回 |
| `TestIDAXrefs` | 交叉引用返回 |
| `TestIDATransportPending` | transport 未实现时返回 pending 错误 |
| `TestRegistryHasIDATools` | 5 个工具在 Registry 中存在 |
| `TestPromptContainsIDATools` | prompt 包含 IDA 工具名 |
| `TestMockStatusNoReadBody` | Status 探测不读取完整 SSE body |
