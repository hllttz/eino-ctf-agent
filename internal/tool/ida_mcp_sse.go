package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// MCP tool 名称常量。真实 IDA MCP (zeromcp) 工具名与预期可能不同，集中在此管理。
const (
	mcpToolListFunctions     = "list_functions"
	mcpToolDecompileFunction = "decompile_function"
	mcpToolListStrings       = "list_strings"
	mcpToolGetXrefsTo        = "get_xrefs_to"
)

// 各工具的候选名 fallback 列表。首选名失败时按顺序尝试。
// zeromcp 等 MCP 实现可能使用不同命名，例如 ida_functions 而非 list_functions。
// 候选名按 zeromcp 常见命名优先排序。真实环境可用 tools/list 确认。
// zeromcp 真实工具: list_funcs, lookup_funcs, func_query。
// export_funcs 需要 addrs 参数，不作为无参数函数列表候选。
var functionsToolCandidates = []string{
	"list_funcs", "lookup_funcs", "func_query",
	"list_functions", "ida_functions", "functions", "get_functions",
}
var decompileToolCandidates = []string{"decompile_func", "decompile_function", "ida_decompile", "decompile"}
var stringsToolCandidates = []string{"list_strings", "ida_strings", "strings", "get_strings"}
var xrefsToolCandidates = []string{"get_xrefs_to", "ida_xrefs", "xrefs", "get_xrefs"}
var disasmToolCandidates = []string{"disasm", "insn_query"}

// 各工具的候选参数键名列表。不同 IDA MCP 实现可能使用不同的参数名，
// 例如有的用 limit 有的用 queries 有的用 count。
// 顺序表示优先级——优先使用更通用的命名。

// isFunctionListTool 判断远端工具名是否为函数列表工具。
// 三层判定：
//  1. 白名单精确匹配 → true
//  2. 名字含 func/function 关键词 → 继续检查黑名单
//  3. 黑名单（即使含关键词也不是函数列表工具）→ false
func isFunctionListTool(name string) bool {
	lower := strings.ToLower(name)

	if isExactFunctionListTool(lower) {
		return true
	}

	if strings.Contains(lower, "func") || strings.Contains(lower, "function") {
		if isNonFunctionTool(lower) {
			return false
		}
		return true
	}

	return false
}

// isExactFunctionListTool 白名单精确匹配（不区分大小写）。
func isExactFunctionListTool(name string) bool {
	exact := map[string]bool{
		"list_funcs":     true,
		"lookup_funcs":   true,
		"func_query":     true,
		"list_functions": true,
		"ida_functions":  true,
		"functions":      true,
		"get_functions":  true,
	}
	return exact[name]
}

// isNonFunctionTool 黑名单——即使名字含 func/function 也不是函数列表工具。
// 同时覆盖所有已知的非函数列表工具，用于 tools/list 自愈阶段过滤。
func isNonFunctionTool(name string) bool {
	blacklist := map[string]bool{
		// 状态/配置类
		"server_health": true,
		"server_warmup": true,
		"config":        true,
		"get_config":    true,
		"ida_status":    true,
		"ida_health":    true,
		"health":        true,
		"ping":          true,
		"echo":          true,
		// IDB/二进制操作
		"int_convert":        true,
		"idb_save":           true,
		"get_bytes":          true,
		"get_original_bytes": true,
		// 非函数列表查询
		"imports":       true,
		"imports_query": true,
		"list_globals":  true,
		"entity_query":  true,
		"find_regex":    true,
		// 反编译/反汇编
		"decompile":          true,
		"decompile_func":     true,
		"decompile_function": true,
		"disasm":             true,
		// 交叉引用
		"xrefs_to":   true,
		"xrefs_from": true,
		// 分析类（名字可能含 function 但不是函数列表）
		"analyze_component": true,
		"diff_before_after": true,
		"trace_data_flow":   true,
		// 含 func/function 关键词但不是函数列表工具
		"func_profile":     true,
		"export_funcs":     true,
		"define_func":      true,
		"analyze_function": true,
	}
	return blacklist[name]
}

var functionsParamCandidates = []string{"limit", "queries", "count", "max_results"}
var decompileParamCandidates = []string{"address", "addr", "name", "function_name", "target"}
var stringsParamCandidates = []string{"limit", "queries", "count", "max_results"}
var xrefsParamCandidates = []string{"address", "addr", "target"}
var disasmParamCandidates = []string{"address", "start", "addr"}

// buildAltArgSets 基于原始参数值和候选键名列表构建多组参数映射。
// 返回的切片首元素为原始参数（如果原始键在候选列表中），后续元素按候选键顺序排列。
func buildAltArgSets(primaryArgs map[string]any, keyCandidates []string) []map[string]any {
	// 提取原始值
	var value any
	for _, v := range primaryArgs {
		value = v
		break
	}

	seen := make(map[string]bool)
	var sets []map[string]any
	for _, key := range keyCandidates {
		if seen[key] {
			continue
		}
		seen[key] = true
		sets = append(sets, map[string]any{key: value})
	}
	return sets
}

// jsonRPCRequest JSON-RPC 请求。
type jsonRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// jsonRPCResponse JSON-RPC 响应。
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// sseEvent 解析后的 SSE 事件。
type sseEvent struct {
	Event string
	Data  string
	ID    string
}

// sseTransport 管理单次 MCP SSE 通信。
// 每次 IDA 工具调用都建立新连接：GET SSE → 读 endpoint event → POST JSON-RPC → 读 SSE response → 关闭。
type sseTransport struct {
	sseEndpoint string
	timeout     time.Duration
	httpClient  *http.Client
	hostHeader  string // IDA_MCP_HOST_HEADER 覆盖 HTTP Host 头；为空则不覆盖

	mu    sync.Mutex
	reqID int
}

func newSSETransport(endpoint string, timeout time.Duration, hostHeader string) *sseTransport {
	return &sseTransport{
		sseEndpoint: endpoint,
		timeout:     timeout,
		hostHeader:  hostHeader,
		// http.Client 不设 Timeout：SSE 是长连接，Client.Timeout 会中断 resp.Body 的持续读取。
		// 超时控制由 context.WithTimeout 在 callMCPTool 中完成。
		httpClient: &http.Client{Timeout: 0},
	}
}

// callMCPTool 通过 MCP SSE 协议调用 IDA 工具。
// 每次调用建立新连接：GET SSE → 单 goroutine 解析事件流 → 读 endpoint event →
// 解析相对 POST URL → 安全校验 → POST JSON-RPC → 读 message event → 返回 result。
func (t *sseTransport) callMCPTool(ctx context.Context, toolName string, arguments map[string]any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	// Step 1: 建立 SSE 连接
	resp, err := t.httpClient.Do(mustNewGetRequest(ctx, t.sseEndpoint, t.hostHeader))
	if err != nil {
		return nil, fmt.Errorf("sse connect: %w", err)
	}
	defer resp.Body.Close()

	// 单 goroutine 解析 SSE 流，所有事件从同一 channel 读取
	events := t.parseSSE(ctx, resp.Body)

	// Step 2: 读 endpoint event，获取 POST 地址（可能是相对路径如 /sse?session=xxx）
	rawPostURL, err := t.readNextEvent(ctx, events, "endpoint")
	if err != nil {
		return nil, fmt.Errorf("read endpoint event: %w", err)
	}

	// Step 3: 解析相对路径并校验 POST endpoint 安全
	resolvedPostURL, err := resolveAndValidatePostEndpoint(rawPostURL, t.sseEndpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid MCP post endpoint: %w", err)
	}

	// Step 4: 发送 JSON-RPC tools/call 请求
	// 每次 callMCPTool 创建独立 SSE 连接，reqID 固定为 1
	reqID := 1

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      toolName,
			"arguments": arguments,
		},
	}
	reqBody, _ := json.Marshal(rpcReq)

	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, resolvedPostURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("create json-rpc request: %w", err)
	}
	postReq.Header.Set("Content-Type", "application/json")
	if t.hostHeader != "" {
		postReq.Host = t.hostHeader
	}

	postResp, err := t.httpClient.Do(postReq)
	if err != nil {
		return nil, fmt.Errorf("json-rpc post: %w", err)
	}
	postResp.Body.Close()

	// Step 5: 从同一 SSE stream 读取 message event，按 id 匹配
	return t.readMessageResponse(ctx, events, reqID)
}

// readNextEvent 从 SSE channel 读取指定类型的第一个事件，返回其 data 字段。
func (t *sseTransport) readNextEvent(ctx context.Context, events <-chan sseEvent, eventType string) (string, error) {
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return "", fmt.Errorf("sse stream closed before %q event", eventType)
			}
			if ev.Event == eventType {
				return strings.TrimSpace(ev.Data), nil
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// readMessageResponse 从 SSE channel 读取 message 事件，按 id 匹配 JSON-RPC 响应。
func (t *sseTransport) readMessageResponse(ctx context.Context, events <-chan sseEvent, expectedID int) (json.RawMessage, error) {
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return nil, fmt.Errorf("sse stream closed before message response id=%d", expectedID)
			}
			if ev.Event != "message" {
				continue
			}
			var resp jsonRPCResponse
			if err := json.Unmarshal([]byte(strings.TrimSpace(ev.Data)), &resp); err != nil {
				continue
			}
			if resp.ID != expectedID {
				continue
			}
			if resp.Error != nil {
				return nil, fmt.Errorf("mcp rpc error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			return resp.Result, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// parseSSE 解析 SSE stream 并发送解析后的事件到 channel。
// goroutine 在 reader 关闭或 ctx 取消时退出。
func (t *sseTransport) parseSSE(ctx context.Context, reader io.Reader) <-chan sseEvent {
	ch := make(chan sseEvent, 8)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		var current sseEvent
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			line := scanner.Text()
			if line == "" || line == "\r" {
				// 空行表示事件结束
				if current.Event != "" || current.Data != "" {
					select {
					case ch <- current:
					case <-ctx.Done():
						return
					}
					current = sseEvent{}
				}
				continue
			}
			// 忽略注释行
			if strings.HasPrefix(line, ":") {
				continue
			}
			// 解析 field:value
			if strings.HasPrefix(line, "event:") {
				current.Event = strings.TrimSpace(line[len("event:"):])
			} else if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(line[len("data:"):])
				if current.Data != "" {
					current.Data += "\n" + data // 多行 data 拼接
				} else {
					current.Data = data
				}
			} else if strings.HasPrefix(line, "id:") {
				current.ID = strings.TrimSpace(line[len("id:"):])
			}
		}
	}()
	return ch
}

// resolveAndValidatePostEndpoint 解析并校验 MCP POST endpoint。
// endpoint event 返回的相对路径基于 SSE endpoint 解析为完整 URL，再校验安全性。
// 允许 127.0.0.1 / localhost / RFC 1918 私有 IP，拒绝 0.0.0.0、公网 IP、远程域名。
// 返回解析后的完整 URL。
func resolveAndValidatePostEndpoint(rawURL, sseEndpoint string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse post endpoint: %w", err)
	}

	// 相对路径基于 SSE endpoint 解析为完整 URL
	if u.Host == "" {
		base, _ := url.Parse(sseEndpoint)
		resolved := base.ResolveReference(u)
		return resolveAndValidatePostEndpoint(resolved.String(), sseEndpoint)
	}

	// 安全校验：允许 localhost 和私有 IP
	host := u.Hostname()
	if host != "localhost" && host != "127.0.0.1" {
		ip := net.ParseIP(host)
		if ip == nil || !isPrivateIP(ip) {
			return "", fmt.Errorf("MCP post endpoint must be localhost or a private IP, got %s", host)
		}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("MCP post endpoint scheme must be http or https, got %q", u.Scheme)
	}
	return u.String(), nil
}

// mcpToolCallResult MCP tools/call 返回的 content 结构。
type mcpToolCallResult struct {
	Content []mcpContentItem `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

type mcpContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// extractMCPText 从 MCP tools/call 的 JSON-RPC result 中提取文本内容。
// 兼容 MCP content 数组和纯文本字符串两种格式。
func extractMCPText(result json.RawMessage) (string, error) {
	var callResult mcpToolCallResult
	if err := json.Unmarshal(result, &callResult); err == nil && len(callResult.Content) > 0 {
		var parts []string
		for _, item := range callResult.Content {
			if item.Type == "text" && item.Text != "" {
				parts = append(parts, item.Text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n"), nil
		}
	}

	var text string
	if err := json.Unmarshal(result, &text); err == nil {
		return text, nil
	}

	return string(result), nil
}

// callMCPToolText 调用 MCP 工具并通过 extractMCPText 提取文本。
func (t *sseTransport) callMCPToolText(ctx context.Context, toolName string, arguments map[string]any) (string, error) {
	result, err := t.callMCPTool(ctx, toolName, arguments)
	if err != nil {
		return "", err
	}
	return extractMCPText(result)
}

// isToolNotFound 判断错误是否为远端工具不存在（可重试）。
// 同时处理 JSON-RPC error 和 result content 文本两种形式。
// 匹配: Method/Tool not found, Unknown method/tool, not registered, -32601.
func isToolNotFound(err error) bool {
	if err == nil {
		return false
	}
	return isToolNotFoundText(err.Error())
}

// isToolNotFoundText 检测文本是否表示远端工具不存在。
// 支持多种 MCP 实现的错误措辞差异：zeromcp 用 "Method not found"，
// 某些实现用 "Tool not found"、"Unknown tool"、"not registered" 等。
func isToolNotFoundText(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "not found") || strings.Contains(lower, "not_found") {
		return true // Method/Tool/anything not found
	}
	if strings.Contains(lower, "unknown") && (strings.Contains(lower, "method") || strings.Contains(lower, "tool")) {
		return true
	}
	if strings.Contains(lower, "not registered") {
		return true
	}
	return false
}

// isParamError 判断错误是否为参数错误（可重试不同参数名）。
// 同时处理 JSON-RPC error 和 result content 文本两种形式。
func isParamError(err error) bool {
	if err == nil {
		return false
	}
	return isParamErrorText(err.Error())
}

// isParamErrorText 检测文本是否表示参数错误。
// 远端 MCP 工具可能因参数名不匹配返回此类错误（如 "Invalid params: missing required parameters"）。
func isParamErrorText(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "invalid param") ||
		strings.Contains(lower, "missing required parameter") ||
		strings.Contains(lower, "unexpected parameter") ||
		strings.Contains(lower, "unknown parameter") ||
		strings.Contains(lower, "bad parameter")
}

// resultValidator 验证 MCP 工具返回的文本是否在语义上正确。
// 返回 nil 表示通过验证；返回 error 表示语义不匹配，应继续 fallback。
type resultValidator func(text string) error

// callMCPToolWithFallback 按候选名列表依次尝试调用 MCP 工具。
// 遇到 Method not found 时尝试下一个候选名；遇到参数错误时尝试参数名重试（同一工具名、不同参数键）。
// 所有候选都失败时，调用 tools/list 获取真实可用工具名列表并包含在错误信息中。
//
// IDA MCP (zeromcp) 把 "Method not found" 放在 tools/call 的 result content 中返回
// （JSON-RPC 层面不报 error），因此 err==nil 时也需要检查 result 文本是否包含该错误。
// 同理，参数错误（如 "Invalid params: missing required parameters: ['queries']"）也通过 content 返回。
func (t *sseTransport) callMCPToolWithFallback(ctx context.Context, candidates []string, arguments map[string]any) (json.RawMessage, error) {
	return t.callMCPToolWithFullFallback(ctx, candidates, arguments, nil, nil)
}

// callMCPToolWithFullFallback 同时支持工具名 fallback 和参数名 fallback。
// paramKeyCandidates 为可选的参数键名候选列表（用于参数 fallback）。nil 表示不进行参数 fallback。
// validator 为可选的结果语义校验器。nil 表示不校验。校验失败时继续尝试下一个候选。
//
// 可重试的错误类型：
//   - 工具不存在（Method/Tool not found, Unknown tool, not registered）
//   - 参数错误（Invalid params, missing required parameter 等）
//   - validator 校验失败（仅当 validator 非 nil 时）
//
// 不可重试的错误：连接失败、超时、非 MCP 错误等直接返回。
func (t *sseTransport) callMCPToolWithFullFallback(ctx context.Context, toolCandidates []string, arguments map[string]any, paramKeyCandidates []string, validator resultValidator) (json.RawMessage, error) {
	logicalName := ""
	if len(toolCandidates) > 0 {
		logicalName = toolCandidates[0]
	}

	// 构建完整参数集：主参数 + 候选键名替代参数
	argSets := []map[string]any{arguments}
	if len(paramKeyCandidates) > 0 {
		altSets := buildAltArgSets(arguments, paramKeyCandidates)
		if len(altSets) > 1 {
			argSets = append(argSets, altSets[1:]...)
		}
	}
	argSets = append(argSets, map[string]any{}) // 空参数兜底

	var lastErr error
	paramFailedTools := make(map[string]bool) // 工具名存在但参数名不匹配

	for i, toolName := range toolCandidates {
		log.Printf("[ida-mcp-call] logical=%s remote=%s args=%v attempt=%d",
			logicalName, toolName, argKeyString(arguments), i+1)

		result, err := t.callMCPTool(ctx, toolName, arguments)
		if err == nil {
			text, _ := extractMCPText(result)
			log.Printf("[ida-mcp-call] remote=%s response_prefix=%q",
				toolName, truncateStr(text, 300))

			if isToolNotFoundText(text) {
				log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=tool_not_found_in_result", logicalName, toolName)
				lastErr = fmt.Errorf("tool %q not found (in result): %s", toolName, text)
			} else if isParamErrorText(text) {
				log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=param_error", logicalName, toolName)
				paramFailedTools[toolName] = true
				lastErr = fmt.Errorf("param error for tool %q: %s", toolName, text)
				if len(argSets) > 1 {
					if altResult, ok := t.tryArgSets(ctx, logicalName, toolName, argSets[1:], validator); ok {
						return altResult, nil
					}
				}
			} else if validator != nil {
				if valErr := validator(text); valErr != nil {
					log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=validator_rejected: %v", logicalName, toolName, valErr)
					lastErr = fmt.Errorf("tool %q result invalid: %w", toolName, valErr)
					continue
				}
				log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=true", logicalName, toolName)
				return result, nil
			} else {
				log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=true", logicalName, toolName)
				return result, nil
			}
		} else {
			log.Printf("[ida-mcp-call] remote=%s error=%v", toolName, err)

			lastErr = err
			if isToolNotFound(err) {
				log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=tool_not_found", logicalName, toolName)
			} else if isParamError(err) {
				log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=param_error", logicalName, toolName)
				paramFailedTools[toolName] = true
				if len(argSets) > 1 {
					if altResult, ok := t.tryArgSets(ctx, logicalName, toolName, argSets[1:], validator); ok {
						return altResult, nil
					}
				}
			} else {
				return nil, err
			}
		}

		if i >= len(toolCandidates)-1 {
			for toolName := range paramFailedTools {
				if altResult, ok := t.tryArgSets(ctx, logicalName, toolName, argSets[1:], validator); ok {
					return altResult, nil
				}
			}
		}
	}

	// 所有候选都失败：调用 tools/list 获取真实可用工具名。
	tools, listErr := t.listMCPTools(ctx)
	if listErr == nil && len(tools) > 0 {
		newCandidates := diffToolsFiltered(tools, toolCandidates, isNonFunctionTool)
		if len(newCandidates) > 0 {
			for _, name := range newCandidates {
				for _, argSet := range argSets {
					log.Printf("[ida-mcp-call] logical=%s remote=%s(args_from_tools/list) args=%v",
						logicalName, name, argKeyString(argSet))
					result, retryErr := t.callMCPTool(ctx, name, argSet)
					if retryErr != nil {
						if !isToolNotFound(retryErr) {
							lastErr = retryErr
						}
						continue
					}
					text, _ := extractMCPText(result)
					if isToolNotFoundText(text) || isParamErrorText(text) {
						continue
					}
					if validator != nil {
						if valErr := validator(text); valErr != nil {
							log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=validator_rejected: %v", logicalName, name, valErr)
							continue
						}
					}
					log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=true (self-healed)", logicalName, name)
					return result, nil
				}
			}
			paramTried := ""
			if len(paramFailedTools) > 0 && len(argSets) > 1 {
				var paramKeys []string
				for _, s := range argSets {
					for k := range s {
						paramKeys = append(paramKeys, k)
					}
				}
				paramTried = fmt.Sprintf("; param keys tried: %v", paramKeys)
			}
			return nil, fmt.Errorf("MCP tool name not found; tried: %v; available tools: %v%s: %w",
				toolCandidates, tools, paramTried, lastErr)
		}
		availableMsg := ""
		if listErr != nil {
			availableMsg = fmt.Sprintf("; tools/list also failed: %v", listErr)
		} else {
			availableMsg = "; tools/list returned no tools"
		}
		return nil, fmt.Errorf("MCP tool name not found; tried: %v%s: %w",
			toolCandidates, availableMsg, lastErr)
	}
	return nil, fmt.Errorf("MCP tool name not found; tried: %v; tools/list also failed: %v: %w",
		toolCandidates, listErr, lastErr)
}

// tryArgSets 用同一工具名尝试多组参数，返回首个成功结果。
// 跳过返回 tool-not-found 或 param-error 的参数集。
// logicalName 只用于日志；validator 非 nil 时对结果做语义校验。
func (t *sseTransport) tryArgSets(ctx context.Context, logicalName string, toolName string, argSets []map[string]any, validator resultValidator) (json.RawMessage, bool) {
	for _, argSet := range argSets {
		log.Printf("[ida-mcp-call] logical=%s remote=%s(param_retry) args=%v",
			logicalName, toolName, argKeyString(argSet))
		result, err := t.callMCPTool(ctx, toolName, argSet)
		if err != nil {
			if isToolNotFound(err) || isParamError(err) {
				continue
			}
			continue // 非可重试错误也应跳过，不阻止其他参数集
		}
		text, _ := extractMCPText(result)
		if isToolNotFoundText(text) || isParamErrorText(text) {
			log.Printf("[ida-mcp-call] remote=%s(param_retry) response_prefix=%q -> skipped", toolName, truncateStr(text, 200))
			continue
		}
		if validator != nil {
			if valErr := validator(text); valErr != nil {
				log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=validator_rejected: %v", logicalName, toolName, valErr)
				continue
			}
		}
		log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=true (param retry)", logicalName, toolName)
		return result, true
	}
	return nil, false
}

// callMCPToolTextWithFallback 是 callMCPToolWithFallback 的文本提取版本。
func (t *sseTransport) callMCPToolTextWithFallback(ctx context.Context, candidates []string, arguments map[string]any) (string, error) {
	result, err := t.callMCPToolWithFallback(ctx, candidates, arguments)
	if err != nil {
		return "", err
	}
	return extractMCPText(result)
}

// callMCPToolTextWithParamFallback 是支持参数名 fallback 的文本提取版本。
func (t *sseTransport) callMCPToolTextWithParamFallback(ctx context.Context, toolCandidates []string, arguments map[string]any, paramKeyCandidates []string) (string, error) {
	result, err := t.callMCPToolWithFullFallback(ctx, toolCandidates, arguments, paramKeyCandidates, nil)
	if err != nil {
		return "", err
	}
	return extractMCPText(result)
}

// callMCPToolTextWithParamFallbackAndValidator 是支持参数 fallback + 结果语义校验的文本提取版本。
func (t *sseTransport) callMCPToolTextWithParamFallbackAndValidator(ctx context.Context, toolCandidates []string, arguments map[string]any, paramKeyCandidates []string, validator resultValidator) (string, error) {
	result, err := t.callMCPToolWithFullFallback(ctx, toolCandidates, arguments, paramKeyCandidates, validator)
	if err != nil {
		return "", err
	}
	return extractMCPText(result)
}

// listMCPTools 调用 MCP tools/list 获取可用工具名列表并记录 schema 信息。
// 创建独立 sseTransport 确保 reqID 从 1 开始，不受调用方 reqID 影响。
func (t *sseTransport) listMCPTools(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	resp, err := t.httpClient.Do(mustNewGetRequest(ctx, t.sseEndpoint, t.hostHeader))
	if err != nil {
		return nil, fmt.Errorf("sse connect: %w", err)
	}
	defer resp.Body.Close()

	events := t.parseSSE(ctx, resp.Body)
	rawPostURL, err := t.readNextEvent(ctx, events, "endpoint")
	if err != nil {
		return nil, fmt.Errorf("read endpoint event: %w", err)
	}
	resolvedPostURL, err := resolveAndValidatePostEndpoint(rawPostURL, t.sseEndpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid MCP post endpoint: %w", err)
	}

	listReqID := 1

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      listReqID,
		Method:  "tools/list",
	}
	reqBody, _ := json.Marshal(rpcReq)

	postReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, resolvedPostURL, strings.NewReader(string(reqBody)))
	postReq.Header.Set("Content-Type", "application/json")
	if t.hostHeader != "" {
		postReq.Host = t.hostHeader
	}
	postResp, err := t.httpClient.Do(postReq)
	if err != nil {
		return nil, fmt.Errorf("tools/list post: %w", err)
	}
	postResp.Body.Close()

	result, err := t.readMessageResponse(ctx, events, listReqID)
	if err != nil {
		return nil, fmt.Errorf("tools/list response: %w", err)
	}

	var listResult struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			InputSchema any    `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &listResult); err != nil {
		return nil, fmt.Errorf("parse tools/list: %w", err)
	}

	names := make([]string, len(listResult.Tools))
	for i, tool := range listResult.Tools {
		names[i] = tool.Name
		// 记录函数相关工具的 schema 信息供诊断
		if isPotentialFuncTool(tool.Name) {
			schemaJSON, _ := json.Marshal(tool.InputSchema)
			log.Printf("[ida-mcp-list] tool=%s desc=%q schema=%s", tool.Name, tool.Description, truncateStr(string(schemaJSON), 200))
		}
	}
	log.Printf("[ida-mcp-list] total_tools=%d tools=%v", len(names), names)
	return names, nil
}

// isPotentialFuncTool 判断工具名是否可能为函数列表工具。
func isPotentialFuncTool(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "func") || strings.Contains(lower, "funcs") ||
		strings.Contains(lower, "function") || strings.Contains(lower, "functions")
}

// mustNewGetRequest 创建 GET 请求，辅助函数。
// hostHeader 非空时设置 req.Host 覆盖 HTTP Host 头（用于 WSL→Windows 跨系统访问场景）。
func mustNewGetRequest(ctx context.Context, url string, hostHeader string) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "text/event-stream")
	if hostHeader != "" {
		req.Host = hostHeader
	}
	return req
}

// diffTools 返回在 tools 中但不在 candidates 中的工具名。
func diffTools(tools, candidates []string) []string {
	return diffToolsFiltered(tools, candidates, nil)
}

// diffToolsFiltered 返回在 tools 中但不在 candidates 中的工具名，并通过 filter 排除过滤函数返回 true 的工具。
// filter 为 nil 时不过滤。
func diffToolsFiltered(tools, candidates []string, filter func(string) bool) []string {
	seen := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		seen[c] = true
	}
	var diff []string
	var filteredOut []string
	for _, t := range tools {
		if seen[t] {
			continue
		}
		if filter != nil && filter(t) {
			filteredOut = append(filteredOut, t)
			continue
		}
		diff = append(diff, t)
	}
	if len(filteredOut) > 0 {
		log.Printf("[ida-mcp-list] filtered_irrelevant_tools=%v", filteredOut)
	}
	return diff
}

// tryCandidates 尝试候选名列表直到找到可用的。
func (t *sseTransport) tryCandidates(ctx context.Context, candidates []string, arguments map[string]any) (json.RawMessage, error) {
	for _, name := range candidates {
		result, err := t.callMCPTool(ctx, name, arguments)
		if err == nil {
			text, _ := extractMCPText(result)
			if isToolNotFoundText(text) {
				continue
			}
			return result, nil
		}
		if isToolNotFound(err) {
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("no tool found in: %v", candidates)
}

// argKeyString 返回参数的 key=value 字符串，用于日志。
func argKeyString(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// truncateStr 截断字符串到 maxLen 字符，用于日志。
func truncateStr(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
