# Agent Phase 6 — IDA MCP 交接总结

> 生成日期：2026-05-14
> 分支：`agent-phase6-ida-mcp`
> 状态：ida_functions 已能通过 list_funcs 返回真实函数列表，核心调用链已验证通过

---

## 1. 本阶段最终解决的问题

### 1.1 网络层（前三轮修复，已稳定）
- WSL → Windows 跨系统 SSE 连接：`IDA_MCP_HOST_HEADER` 覆盖 Host 头
- ReAct handler 工具调用链：工具注册、InvokableTool 接口适配

### 1.2 参数构造（第4轮修复）
- **问题**：`buildAltArgSets` 把 `limit=200`（int）机械复制到 `queries` 键，zeromcp 需要 string
- **修复**：`Functions()` 改用工具感知的 `buildFunctionsArgSets(limit)`，为每种工具生成类型正确的参数
  - `list_funcs` / `lookup_funcs` → `{"queries": ""}`, `{"queries": "*"}`, `{}`
  - `func_query` → `{"query": ""}`, `{"query": "*"}`, `{"queries": ""}`, `{"queries": "*"}`, `{}`
  - `list_functions` / `ida_functions` / `functions` / `get_functions` → `{"limit": N}`, `{"count": N}`, `{"max_results": N}`, `{}`
- `export_funcs` 已从候选移除（需要 addrs 参数）

### 1.3 结果解析（第5轮修复）
- **问题**：`extractFunctionNamesFromArray` 不支持 zeromcp 的嵌套结构（`data` 数组、`fn` 对象）
- **修复**：重写为递归提取，支持 `data` 字段内嵌数组、`fn` 字段内嵌对象；跳过含 `error` 字段非 null 的元素；新增去重

### 1.4 语义校验（第3轮修复）
- **问题**：远端返回 server_health/status JSON 被当作函数列表返回
- **修复**：`parseFunctionsResult` 加入 `isServerHealthJSON` / `isToolsListJSON` 检测；`validateFunctionsResult` 在 fallback 中拒绝非函数列表结果

### 1.5 自愈过滤（第4-5轮修复）
- **问题**：tools/list 自愈路径误选 `server_health`、`analyze_component`、`trace_data_flow`、`func_profile` 等无关工具
- **修复**：自愈路径改为 `isExactFunctionListTool` 严格白名单（仅 7 个精确工具名），拒绝关键词模糊匹配

---

## 2. 当前可运行环境配置

```bash
# WSL2 内执行：
export IDA_MCP_URL="http://172.26.192.1:13338/sse"
export IDA_MCP_HOST_HEADER="127.0.0.1:13338"
export NO_PROXY="localhost,127.0.0.1,::1,172.26.192.1"
export no_proxy="$NO_PROXY"

# 启动后端：
cd /home/winter/workspace/eino-ctf-agent
go run ./cmd/server
```

**IDA MCP 服务在 Windows 侧运行**（IDA Pro + zeromcp 插件，端口 13338）。WSL 通过 `172.26.192.1`（Windows 宿主私有 IP）访问。

`.env.example` 中已配置：
```
IDA_MCP_URL=http://172.26.192.1:13338/sse
IDA_MCP_HOST_HEADER=127.0.0.1:13338
```

---

## 3. 关键配置说明

| 配置项 | 值 | 说明 |
|--------|-----|------|
| IDA MCP URL | `http://172.26.192.1:13338/sse` | Windows 宿主 IP + zeromcp SSE endpoint |
| Host Header | `127.0.0.1:13338` | zeromcp 校验 Host 为 127.0.0.1:13338 |
| endpoint 校验 | `validateIDAEndpoint` | 允许 localhost/127.0.0.1/RFC 1918 私有 IP；拒绝公网 IP |
| POST endpoint 校验 | `resolveAndValidatePostEndpoint` | SSE endpoint event 返回的相对/绝对 URL 均需通过安全性校验 |
| 超时 | `IDA_MCP_TIMEOUT_SECONDS`（默认 5s） | 每次 SSE 调用独立 context.WithTimeout |

**不修改的内容**：WSL 网络、Windows 防火墙、IDA Pro 插件配置、portproxy、Agent 架构。

---

## 4. ida_functions 调用链

```
用户请求 "调用 ida_functions"
  → ReAct Agent (Eino)
    → NewIDAFunctionsTool() [注册名: ida_functions]
      → IDAFunctionsInput{Limit: 200}
        → idaClient.Functions(ctx, 200)
          → RealMCPClient.Functions()
            │
            ├─[主迭代] 按 buildFunctionsArgSets(200) 生成的工具-参数组依次尝试：
            │   Group 1 (list_funcs, lookup_funcs):
            │     {"queries": ""} → {"queries": "*"} → {}
            │   Group 2 (func_query):
            │     {"query": ""} → {"query": "*"} → {"queries": ""} → {"queries": "*"} → {}
            │   Group 3 (list_functions, ida_functions, functions, get_functions):
            │     {"limit": 200} → {"count": 200} → {"max_results": 200} → {}
            │
            │   每个调用：
            │     → isToolNotFound(err) → break（跳过该工具组）
            │     → isParamErrorText(text) → continue（下一个参数集）
            │     → validateFunctionsResult(text) → fail → continue
            │     → OK → parseFunctionsResult(text) → 返回函数列表
            │
            └─[自愈] 所有候选失败：
                → transport.listMCPTools()
                → isExactFunctionListTool(name) 过滤（严格白名单）
                → diffTools 排除已尝试候选
                → functionsFallbackArgSets() 重试
                  → validateFunctionsResult 语义校验
                  → OK → parseFunctionsResult → 返回函数列表
```

---

## 5. fallback 策略总览

### 5.1 工具名 fallback
候选顺序（`functionsToolCandidates`）：
```
list_funcs → lookup_funcs → func_query
→ list_functions → ida_functions → functions → get_functions
```

### 5.2 参数 fallback（工具感知）
不再使用通用的 `buildAltArgSets` 机械复制值。每种工具使用类型正确的参数值。

### 5.3 tools/list 自愈
- 过滤：**仅 `isExactFunctionListTool`** 白名单精确匹配（7 个工具名）
- 新候选：排除已在主迭代中尝试过的工具名
- 参数：`functionsFallbackArgSets()` = `{"queries": ""}`, `{"queries": "*"}`, `{}`
- 结果必须通过 `validateFunctionsResult` 语义校验

### 5.4 黑名单（自愈阶段过滤）
`isNonFunctionTool` 包含 30+ 工具：`server_health`, `analyze_component`, `diff_before_after`, `trace_data_flow`, `func_profile`, `export_funcs`, `define_func`, `analyze_function`, `decompile`, `disasm`, `xrefs_to`, `xrefs_from`, `imports`, `list_globals` 等。

---

## 6. zeromcp 真实返回结构

### list_funcs
```json
[{"data": [{"addr": "0x140001000", "name": "_dynamic_initializer", "size": "0xc"}]}]
```

### lookup_funcs
```json
[{"query": "*", "fn": {"addr": "0x140001000", "name": "_dynamic_initializer", "size": "0xc"}, "error": null}]
```

### func_query
```json
[{"data": [{"addr": "0x140001000", "name": "init_routine", "size": "0xc", "has_type": false}]}]
```

---

## 7. parseFunctionsResult 当前支持的返回格式

| 格式 | 处理方式 |
|------|----------|
| JSON 字符串数组 `["main", "check"]` | 直接提取 |
| JSON 对象数组 `[{"name":"main", "addr":"0x401000"}]` | 提取 name + addr，输出 `"main 0x401000"` |
| data 嵌套 `[{"data": [{...}]}]` | 递归进入 data 数组 |
| fn 嵌套 `[{"fn": {...}}]` | 提取 fn 对象 |
| error 字段 `[{"fn": null, "error": "msg"}]` | 跳过（error 非 null） |
| JSON 对象含 functions/funcs/items/results 键 | 从对应数组提取 |
| server_health JSON `{"status":"ok","uptime_sec":...}` | 拒绝（isServerHealthJSON） |
| tools/list JSON `{"tools":[...]}` | 拒绝（isToolsListJSON） |
| 纯文本多行 | 按行解析，过滤错误行 |

输出格式：`"name addr"`（如 `"main 0x140001000"`），去重保持插入顺序。

---

## 8. 已修改文件列表

| 文件 | 改动量 | 说明 |
|------|--------|------|
| `internal/tool/ida_mcp_sse.go` | ~830 行（新建） | SSE transport、MCP JSON-RPC、工具名/参数 fallback、tools/list、isFunctionListTool 体系、diffToolsFiltered、辅助函数 |
| `internal/tool/ida_mcp_sse_test.go` | ~1270 行（新建） | SSE transport 测试、fallback 测试、参数 fallback 测试、Host header 测试 |
| `internal/tool/ida_mcp_client.go` | +540 行 | RealMCPClient、parseFunctionsResult、extractFunctionNamesFromArray、extractNameAndAddr、validateFunctionsResult、isServerHealthJSON、Functions()、buildFunctionsArgSets 等 |
| `internal/tool/ida_mcp_tools.go` | +2 行 | Eino InvokableTool 注册（ida_status/ida_functions/ida_decompile/ida_strings/ida_xrefs） |
| `internal/tool/ida_mcp_test.go` | +420 行 | endpoint 校验、MockMCPClient、SSE 集成测试、Host header 测试、DisabledMCPClient |
| `internal/tool/ida_mcp_semantic_test.go` | ~760 行（新建） | parseFunctionsResult 语义校验、zeromcp 格式解析、validateFunctionsResult、fallback 过滤测试 |
| `internal/prompt/agent_prompt.go` | +2 行 | Agent prompt 微调 |
| `.env.example` | +9 行 | IDA MCP 环境变量模板 |

---

## 9. 已通过的测试

```
ok  eino_ctf_agent/internal/handler   0.012s
ok  eino_ctf_agent/internal/knowledge 0.011s
ok  eino_ctf_agent/internal/model     0.008s
ok  eino_ctf_agent/internal/prompt    0.008s
ok  eino_ctf_agent/internal/service   0.008s
ok  eino_ctf_agent/internal/skill     0.004s
ok  eino_ctf_agent/internal/tool      11.204s
```

**零失败**。tool 包内包含 55+ 测试用例，覆盖：
- endpoint 安全性校验（白名单/黑名单）
- SSE transport 协议（endpoint event、JSON-RPC、超时、ID 匹配）
- 工具名 fallback（Method not found、result content 错误）
- 参数 fallback（含空参数兜底）
- tools/list 自愈
- Host header 覆盖（GET/POST/tools/list/fallback 持久性）
- parseFunctionsResult 语义校验（health/status/tools-list 拒绝、zeromcp 格式接受）
- validateFunctionsResult
- isFunctionListTool / isExactFunctionListTool / isNonFunctionTool
- Registry 层 Eino InvokableTool 集成
- DisabledMCPClient

---

## 10. 当前真实验证命令

```bash
# 启动：
export IDA_MCP_URL="http://172.26.192.1:13338/sse"
export IDA_MCP_HOST_HEADER="127.0.0.1:13338"
export NO_PROXY="localhost,127.0.0.1,::1,172.26.192.1"
export no_proxy="$NO_PROXY"
go run ./cmd/server

# 测试：
curl --noproxy '*' -s -X POST http://127.0.0.1:8080/api/chat \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: ida-functions-e2e" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "只调用 ida_functions 一次，返回当前 IDA 函数列表。不要调用 ida_status，不要调用其他工具，不要重试。"
      }
    ],
    "agent": {
      "mode": "react"
    }
  }'
```

**预期**：
- 日志出现 `[ida-mcp-call] logical=ida_functions remote=list_funcs args={queries=}`
- 日志出现 `[ida-mcp-fallback] logical=ida_functions remote=list_funcs success=true`
- 返回中包含 `0x140001000` 和对应函数名
- 不再出现 `queries=200`（int 值）、`server_health`、`analyze_component`、`func_profile` 等

---

## 11. 后续可继续优化的点

### 11.1 自愈路径中的"已尝试工具"重试
当前 `allFunctionsCandidates()` 包含所有候选。如果某工具在主迭代中因 "Method not found" 失败，但 tools/list 显示它存在，自愈路径会因为 `contains(allCandidates, name)` 跳过它。可改为：对 tools/list 返回的白名单工具，即使已尝试过也重新用不同参数重试。

### 11.2 非函数工具的 fallback 同理收紧
`callMCPToolWithFullFallback`（decompile/strings/xrefs 使用）的自愈路径仍用 `isNonFunctionTool` 过滤（黑名单）。可同样改为更严格的白名单机制。

### 11.3 ida_strings / ida_decompile / ida_xrefs 的 zeromcp 适配
目前 decompile 候选为 `decompile_func → decompile_function → ida_decompile → decompile`，参数候选为 `address → addr → name → function_name → target`。zeromcp 可能需要 `decompile` + `addr` 或其他组合。应参考 `ida_functions` 的模式做工具感知的参数构造。

### 11.4 list_funcs schema 自动适配
当前 `listMCPTools` 已记录 function 相关工具的 `inputSchema` 到日志。可以在此基础上自动生成参数集（而非硬编码 `buildFunctionsArgSets`）。

### 11.5 并发调用优化
当前 fallback 是串行的（每次创建新 SSE 连接）。对于互不依赖的工具名候选，可以并发尝试。

---

## 12. 给新 Claude Code 对话的继续开发 prompt

```
systemRole:
你是资深 Go 后端工程师，熟悉 CloudWeGo Eino、MCP tools/call、IDA MCP / zeromcp。

项目状态：
- 分支 agent-phase6-ida-mcp
- WSL 网络、Host header、ReAct handler 均已打通并验证通过
- ida_functions 已能通过 list_funcs 返回真实函数列表（{queries: ""} 参数）
- 不要再修改网络、Host header、IDA 插件配置或 Agent 架构

当前代码位置：
- IDA MCP 客户端：internal/tool/ida_mcp_client.go（RealMCPClient、parseFunctionsResult、Functions）
- SSE transport：internal/tool/ida_mcp_sse.go（MCP JSON-RPC、fallback、tools/list）
- Eino 工具注册：internal/tool/ida_mcp_tools.go
- 测试：internal/tool/ida_mcp_test.go、ida_mcp_sse_test.go、ida_mcp_semantic_test.go

已验证通过的调用链：
用户请求 → ReAct Agent → ida_functions 工具
  → RealMCPClient.Functions()
    → buildFunctionsArgSets 生成工具感知参数
    → list_funcs + {queries: ""} → 成功返回函数列表
    → parseFunctionsResult 解析 zeromcp 嵌套格式
    → 返回函数名 + 地址列表

关键设计决策：
1. Functions() 不使用通用的 callMCPToolWithFullFallback，而是直接迭代 buildFunctionsArgSets
2. 参数构造是工具感知的：list_funcs 用 string queries，传统工具用 int limit
3. 自愈路径仅用 isExactFunctionListTool 严格白名单（7个工具名）
4. 结果必须通过 validateFunctionsResult 语义校验
5. export_funcs 不在函数列表候选（需要 addrs）

阅读 HANDOFF.md 了解完整上下文后再开始修改代码。
```

---

## 快速验证命令

```bash
# 编译
go build ./...

# 全量测试
go test ./... -count=1

# 仅 tool 包测试
go test ./internal/tool/... -v -count=1

# 启动服务
export IDA_MCP_URL="http://172.26.192.1:13338/sse"
export IDA_MCP_HOST_HEADER="127.0.0.1:13338"
export NO_PROXY="localhost,127.0.0.1,::1,172.26.192.1"
export no_proxy="$NO_PROXY"
go run ./cmd/server
```
