# IDA MCP 链路验证记录

> 验证日期：2026-05-14 | 分支：agent-phase6-ida-mcp

## 验证目标

确认 ReAct Agent 在真实环境中正确执行 `ida_functions → ida_decompile` 工具调用链。

## curl 验证命令

```bash
curl -s -X POST http://127.0.0.1:8080/api/chat \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: final-ida-test-006" \
  -d '{
    "messages": [
      {"role": "user", "content": "用IDA MCP工具：先调用 ida_functions 获取函数列表，然后选一个函数（优先main）调用 ida_decompile 反编译。严格只做这两步，不额外操作，不调用 ida_status。"}
    ]
  }'
```

## 实际工具调用链

服务器日志中确认的完整调用链：

```
14:22:33 [TRACE] reactChat start
14:22:40 [ida-react-tool] logical=ida_functions handler=IDAMCPClient.Functions
14:22:40 [ida-mcp-call] logical=ida_functions remote=list_funcs args={queries=}
14:22:40 [ida-mcp-fallback] logical=ida_functions remote=list_funcs success=true   ✅

14:22:40 [ida-mcp-call] logical=decompile_func remote=decompile_func args={address=main}
14:22:40 [ida-mcp-call] logical=decompile_func remote=decompile_function args={address=main}
14:22:40 [ida-mcp-call] logical=decompile_func remote=ida_decompile args={address=main}
14:22:40 [ida-mcp-call] logical=decompile_func remote=decompile args={address=main}
14:22:40 [ida-mcp-call] logical=decompile_func remote=decompile(param_retry) args={addr=main}
14:22:40 [ida-mcp-fallback] logical=decompile_func remote=decompile success=true (param retry)  ✅

14:23:09 [TRACE] reactChat done
```

关键发现：
- **ida_functions** 最终映射到远端 `list_funcs` 工具（zeromcp 真实工具名），参数 `queries=`
- **ida_decompile** 最终映射到远端 `decompile` 工具，经参数名 fallback `address`→`addr` 后成功
- 单次调用耗时约 36 秒（含 fallback 重试）

## ida_functions 输出摘要

- 返回 **100 个函数**，未截断
- MinGW 编译的 Windows PE（`_main` 初始化，`__mingw_*` 运行时）
- 关键入口函数：`main` @ `0x140001688`、`WinMainCRTStartup`、`mainCRTStartup`
- I/O 函数：`scanf` @ `0x1400015e0`、`printf` @ `0x140001634`

## main 函数反编译结果

```c
int __fastcall main(int argc, const char **argv, const char **envp) {
  int v4;
  _DWORD v5[3];
  _main(*(_QWORD *)&argc, argv, envp);
  v5[2] = 1;
  printf(&Format);
  scanf("%d%d", v5, &v4);
  JUMPOUT(0x1400016D9LL);
}
```

分析要点：
- `scanf` 读取两个整数后 `JUMPOUT` 间接跳转（控制流混淆 / jump table）
- IDA 警告栈帧分配复杂，反编译结果可能不完整
- 后续需 `ida_disasm` 查看 `0x1400016D9` 处的实际指令

## WSL 网关 IP 漂移注意事项

WSL2 中 Windows 宿主机 IP 会随重启变化，当前 IP 通过以下方式获取：

```bash
cat /etc/resolv.conf | grep nameserver | awk '{print $2}'
```

**不要将动态命令写入 `.env` 文件**。推荐方式：

### 方案 A：启动脚本

在启动脚本（如 `run_with_ida_mcp.sh`）中动态获取 IP：

```bash
#!/bin/bash
export IDA_MCP_URL="http://$(cat /etc/resolv.conf | grep nameserver | awk '{print $2}'):13338/sse"
export IDA_MCP_HOST_HEADER=127.0.0.1:13338
export NO_PROXY=localhost,127.0.0.1,::1,$(cat /etc/resolv.conf | grep nameserver | awk '{print $2}')
exec go run ./cmd/server -config ./configs/config.yaml
```

### 方案 B：固定 IP（当前使用）

```bash
# 在 .env 中手动设置当前 WSL 网关 IP
IDA_MCP_URL=http://172.27.112.1:13338/sse
IDA_MCP_HOST_HEADER=127.0.0.1:13338
```

注意：WSL 重启后 IP 可能变化，需重新检查并更新。
