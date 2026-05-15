package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// IDAMCPStatus Status 探测结果。
type IDAMCPStatus struct {
	Available bool   `json:"available"`
	Endpoint  string `json:"endpoint"`
	Error     string `json:"error,omitempty"`
}

// IDAMCPFunctionsResult 函数列表查询结果。
type IDAMCPFunctionsResult struct {
	Functions []string `json:"functions"`
	Total     int      `json:"total"`
	Error     string   `json:"error,omitempty"`
}

// IDAMCPDecompileResult 反编译结果。
type IDAMCPDecompileResult struct {
	Code  string `json:"code"`
	Error string `json:"error,omitempty"`
}

// IDAMCPStringsResult 字符串查询结果。
type IDAMCPStringsResult struct {
	Strings []string `json:"strings"`
	Total   int      `json:"total"`
	Error   string   `json:"error,omitempty"`
}

// IDAMCPXrefsResult 交叉引用查询结果。
type IDAMCPXrefsResult struct {
	Xrefs []string `json:"xrefs"`
	Error string   `json:"error,omitempty"`
}

// IDAMCPDisasmResult 反汇编结果。
type IDAMCPDisasmResult struct {
	Instructions string `json:"instructions"`
	Error        string `json:"error,omitempty"`
}

// IDAMCPClient 封装对 IDA MCP 服务的只读分析调用。
// 工具层只依赖此接口，不关心底层是 SSE、HTTP 还是 Mock 传输。
type IDAMCPClient interface {
	Status(ctx context.Context) (*IDAMCPStatus, error)
	Functions(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error)
	Decompile(ctx context.Context, target string) (*IDAMCPDecompileResult, error)
	Strings(ctx context.Context, limit int) (*IDAMCPStringsResult, error)
	Xrefs(ctx context.Context, target string) (*IDAMCPXrefsResult, error)
	Disasm(ctx context.Context, address string, end string, count int) (*IDAMCPDisasmResult, error)
}

// 默认配置
const (
	defaultIDAEndpoint        = "http://127.0.0.1:13338/sse"
	defaultIDATimeoutSec      = 5
	idaTransportPendingSuffix = " transport not fully implemented; IDA MCP SSE client pending"
)

// validateIDAEndpoint 校验 IDA MCP endpoint：允许 localhost 和 RFC 1918 私有 IP。
// 拒绝 0.0.0.0、公网 IP、远程域名、空 scheme、非 http/https scheme。
// 允许私有 IP 是为了支持 WSL→Windows 跨系统访问场景。
func validateIDAEndpoint(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid IDA MCP endpoint URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("IDA MCP endpoint scheme must be http or https, got %q", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("IDA MCP endpoint has empty host")
	}

	if host == "localhost" || host == "127.0.0.1" {
		return raw, nil
	}

	if host == "0.0.0.0" {
		return "", fmt.Errorf("IDA MCP endpoint 0.0.0.0 not allowed, use localhost or a private IP")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return raw, nil
		}
		return "", fmt.Errorf("IDA MCP endpoint must be localhost, 127.0.0.1, or a private IP address, got %s", host)
	}
	return "", fmt.Errorf("IDA MCP endpoint must be localhost, 127.0.0.1, or a private IP address, got %s", host)
}

// isPrivateIP 判断是否为 RFC 1918 私有 IPv4 地址。
func isPrivateIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 {
			return true // 10.0.0.0/8
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true // 172.16.0.0/12
		}
		if ip4[0] == 192 && ip4[1] == 168 {
			return true // 192.168.0.0/16
		}
	}
	return false
}

// RealMCPClient IDA MCP 生产客户端。
type RealMCPClient struct {
	endpoint   string
	timeout    time.Duration
	hostHeader string // IDA_MCP_HOST_HEADER 覆盖 HTTP Host 头；为空则不覆盖
}

// NewRealMCPClient 创建 IDA MCP 客户端，校验 endpoint 安全性，读取 IDA_MCP_HOST_HEADER。
// endpoint 非法时返回 nil + error，调用方应记录 warning 并使用 disabled client。
func NewRealMCPClient(endpoint string, timeoutSec int) (*RealMCPClient, error) {
	validated, err := validateIDAEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	t := time.Duration(timeoutSec) * time.Second
	return &RealMCPClient{
		endpoint:   validated,
		timeout:    t,
		hostHeader: strings.TrimSpace(os.Getenv("IDA_MCP_HOST_HEADER")),
	}, nil
}

// Status 探测 IDA MCP 服务是否可达。
// SSE 是长连接，收到响应头后立即关闭 body，不读取完整 stream。
// 必须受 timeout 控制。
func (c *RealMCPClient) Status(ctx context.Context) (*IDAMCPStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return &IDAMCPStatus{Available: false, Endpoint: c.endpoint, Error: err.Error()}, nil
	}
	if c.hostHeader != "" {
		req.Host = c.hostHeader
	}

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return &IDAMCPStatus{Available: false, Endpoint: c.endpoint, Error: err.Error()}, nil
	}
	// 收到响应头即认为 reachable，立即关闭 body，不读取 SSE stream
	resp.Body.Close()

	return &IDAMCPStatus{Available: true, Endpoint: c.endpoint}, nil
}

// Functions 通过 MCP SSE transport 获取函数列表。
// 使用工具感知的参数构造：不同远端工具使用类型正确的参数值。
// list_funcs/lookup_funcs → queries="" / queries="*" 等字符串参数
// func_query → query=""/query="*"/queries=""/queries="*"
// list_functions/ida_functions/... → limit=N/count=N 等整数参数
// export_funcs 不参与（需要 addrs 参数）。
// 语义校验确保不把 server health/status 当函数列表返回。
func (c *RealMCPClient) Functions(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error) {
	transport := newSSETransport(c.endpoint, c.timeout, c.hostHeader)
	const logicalName = "ida_functions"

	for _, group := range buildFunctionsArgSets(limit) {
		for _, toolName := range group.toolNames {
			for _, argSet := range group.argSets {
				log.Printf("[ida-mcp-call] logical=%s remote=%s args=%v", logicalName, toolName, argKeyString(argSet))
				result, err := transport.callMCPTool(ctx, toolName, argSet)
				if err != nil {
					if isToolNotFound(err) {
						log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=tool_not_found", logicalName, toolName)
						break // 工具不存在 → 跳过整个工具组
					}
					log.Printf("[ida-mcp-call] remote=%s error=%v", toolName, err)
					continue
				}
				text, _ := extractMCPText(result)
				log.Printf("[ida-mcp-call] remote=%s response_prefix=%q", toolName, truncateStr(text, 200))

				if isToolNotFoundText(text) {
					log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=tool_not_found_in_result", logicalName, toolName)
					break
				}
				if isParamErrorText(text) {
					continue
				}
				if valErr := validateFunctionsResult(text); valErr != nil {
					log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=validator_rejected: %v", logicalName, toolName, valErr)
					continue
				}
				log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=true", logicalName, toolName)
				return parseFunctionsResult(text)
			}
		}
	}

	// 所有候选失败：tools/list 自愈。仅白名单精确匹配，拒绝通过关键词匹配的无关工具。
	tools, listErr := transport.listMCPTools(ctx)
	if listErr == nil && len(tools) > 0 {
		allCandidates := allFunctionsCandidates()
		var accepted, rejected []string
		for _, name := range tools {
			if isExactFunctionListTool(strings.ToLower(name)) {
				accepted = append(accepted, name)
			} else {
				rejected = append(rejected, name)
			}
		}
		log.Printf("[ida-mcp-list-filter] logical=%s accepted=%v rejected=%v", logicalName, accepted, rejected)

		for _, name := range accepted {
			if contains(allCandidates, name) {
				continue
			}
			for _, argSet := range functionsFallbackArgSets() {
				log.Printf("[ida-mcp-call] logical=%s remote=%s(args_from_tools/list) args=%v", logicalName, name, argKeyString(argSet))
				result, err := transport.callMCPTool(ctx, name, argSet)
				if err != nil {
					continue
				}
				text, _ := extractMCPText(result)
				if isToolNotFoundText(text) || isParamErrorText(text) {
					continue
				}
				if valErr := validateFunctionsResult(text); valErr != nil {
					log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=false reason=validator_rejected: %v", logicalName, name, valErr)
					continue
				}
				log.Printf("[ida-mcp-fallback] logical=%s remote=%s success=true (self-healed)", logicalName, name)
				return parseFunctionsResult(text)
			}
		}
	}

	return &IDAMCPFunctionsResult{Error: "ida_functions: all candidates exhausted"}, nil
}

// contains checks if a string slice contains a value.
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// funcArgGroup 表示一组工具名及其专用参数集。
type funcArgGroup struct {
	toolNames []string
	argSets   []map[string]any
}

// buildFunctionsArgSets 为函数列表工具生成类型正确的参数集。
// 不同工具需要不同类型的参数值：list_funcs 需要 string queries，而非 int limit。
func buildFunctionsArgSets(limit int) []funcArgGroup {
	return []funcArgGroup{
		{
			// list_funcs / lookup_funcs：queries 是 string/[]string，不能用 int
			toolNames: []string{"list_funcs", "lookup_funcs"},
			argSets: []map[string]any{
				{"queries": ""},
				{"queries": "*"},
				{},
			},
		},
		{
			// func_query：query/queries 是 string
			toolNames: []string{"func_query"},
			argSets: []map[string]any{
				{"query": ""},
				{"query": "*"},
				{"queries": ""},
				{"queries": "*"},
				{},
			},
		},
		{
			// 传统命名工具：使用 int 类型的 limit/count/max_results
			toolNames: []string{"list_functions", "ida_functions", "functions", "get_functions"},
			argSets: []map[string]any{
				{"limit": limit},
				{"count": limit},
				{"max_results": limit},
				{},
			},
		},
	}
}

// allFunctionsCandidates 返回所有函数列表候选工具名（用于 tools/list diff）。
func allFunctionsCandidates() []string {
	var all []string
	for _, g := range buildFunctionsArgSets(0) {
		all = append(all, g.toolNames...)
	}
	return all
}

// functionsFallbackArgSets 为 tools/list 自愈路径生成安全参数（优先 string 类型）。
func functionsFallbackArgSets() []map[string]any {
	return []map[string]any{
		{"queries": ""},
		{"queries": "*"},
		{},
	}
}

// parseFunctionsResult 安全解析 MCP functions 结果。
// 对结果做语义校验：拒绝 server health/status JSON，只接受真正的函数列表。
func parseFunctionsResult(text string) (*IDAMCPFunctionsResult, error) {
	if isParamErrorText(text) || isToolNotFoundText(text) {
		return &IDAMCPFunctionsResult{Error: fmt.Sprintf("ida_functions remote error: %s", text)}, nil
	}

	// 尝试解析为 JSON 数组
	var arr []any
	if err := json.Unmarshal([]byte(text), &arr); err == nil {
		functions := extractFunctionNamesFromArray(arr)
		if len(functions) > 0 {
			return &IDAMCPFunctionsResult{Functions: functions, Total: len(functions)}, nil
		}
		return &IDAMCPFunctionsResult{Error: "ida_functions: JSON array has no recognizable function names"}, nil
	}

	// 尝试解析为 JSON 对象
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err == nil {
		if isServerHealthJSON(obj) {
			return &IDAMCPFunctionsResult{Error: "remote tool returned server status, not function list"}, nil
		}
		if isToolsListJSON(obj) {
			return &IDAMCPFunctionsResult{Error: "remote tool returned tools/list, not function list"}, nil
		}
		// 尝试从已知键名提取函数数组
		for _, key := range []string{"functions", "funcs", "items", "results", "data", "entries", "names", "list"} {
			if val, ok := obj[key]; ok {
				if arr, ok := val.([]interface{}); ok {
					functions := extractFunctionNamesFromArray(arr)
					if len(functions) > 0 {
						return &IDAMCPFunctionsResult{Functions: functions, Total: len(functions)}, nil
					}
				}
			}
		}
		return &IDAMCPFunctionsResult{Error: fmt.Sprintf("ida_functions: unrecognized JSON object, not a function list: %s", truncateForError(text, 200))}, nil
	}

	// 纯文本——尝试按行解析为函数列表
	lines := strings.Split(strings.TrimSpace(text), "\n")
	var functions []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 跳过明显的错误行
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "exception") || strings.Contains(lower, "traceback") {
			continue
		}
		functions = append(functions, line)
	}
	if len(functions) > 0 {
		return &IDAMCPFunctionsResult{Functions: functions, Total: len(functions)}, nil
	}
	return &IDAMCPFunctionsResult{Error: fmt.Sprintf("ida_functions: could not parse as function list: %s", truncateForError(text, 200))}, nil
}

// extractFunctionNamesFromArray 从 JSON 数组元素中提取函数名。
// 支持 zeromcp 返回结构：
//   - 纯字符串数组
//   - 元素含 name/addr/address/ea 等字段
//   - 元素含 data 字段（嵌套函数对象数组，list_funcs/func_query）
//   - 元素含 fn 字段（单个函数对象，lookup_funcs）
func extractFunctionNamesFromArray(arr []any) []string {
	var names []string
	for _, elem := range arr {
		switch v := elem.(type) {
		case string:
			s := strings.TrimSpace(v)
			if s != "" && !isLikelyErrorLine(s) {
				names = append(names, s)
			}
		case map[string]interface{}:
			// 跳过含 error 字段且非 null 的元素
			if errVal, ok := v["error"]; ok && errVal != nil {
				if errStr, ok := errVal.(string); ok && errStr != "" {
					continue
				}
			}

			// zeromcp: data 字段包含嵌套函数数组 (list_funcs, func_query)
			if dataVal, ok := v["data"]; ok {
				if dataArr, ok := dataVal.([]interface{}); ok && len(dataArr) > 0 {
					names = append(names, extractFunctionNamesFromArray(dataArr)...)
					continue
				}
			}

			// zeromcp: fn 字段包含单个函数对象 (lookup_funcs)
			if fnVal, ok := v["fn"]; ok && fnVal != nil {
				if fnMap, ok := fnVal.(map[string]interface{}); ok {
					names = append(names, extractNameAndAddr(fnMap)...)
					continue
				}
			}

			// 直接函数对象：提取 name / address
			names = append(names, extractNameAndAddr(v)...)
		}
	}
	return deduplicateStrings(names)
}

// extractNameAndAddr 从函数对象 map 中提取 name + address 组合。
// 优先输出 "name addr" 格式，其次单独 name 或 addr。
func extractNameAndAddr(v map[string]interface{}) []string {
	var result []string
	var name, addr string

	for _, key := range []string{"name", "function_name", "label", "func_name"} {
		if s, ok := v[key].(string); ok && strings.TrimSpace(s) != "" {
			name = strings.TrimSpace(s)
			break
		}
	}
	for _, key := range []string{"addr", "address", "ea", "start_ea"} {
		if a, ok := v[key]; ok {
			addr = strings.TrimSpace(fmt.Sprintf("%v", a))
			if addr == "0" || addr == "" {
				addr = ""
			}
			break
		}
	}

	if name != "" {
		if addr != "" {
			result = append(result, name+" "+addr)
		} else {
			result = append(result, name)
		}
		return result
	}
	if addr != "" {
		result = append(result, "func@"+addr)
	}
	return result
}

// deduplicateStrings 去重字符串切片，保持插入顺序。
func deduplicateStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// isServerHealthJSON 检测 JSON 对象是否为 server health/status 响应。
// 包含 3 个或以上服务器状态特征字段即判定为 health 响应。
func isServerHealthJSON(obj map[string]any) bool {
	healthFields := []string{
		"status", "uptime_sec", "idb_path", "module", "input_path",
		"auto_analysis_ready", "hexrays_ready", "strings_cache_ready",
		"strings_cache_size", "imagebase",
	}
	count := 0
	for _, field := range healthFields {
		if val, ok := obj[field]; ok {
			count++
			_ = val
		}
	}
	return count >= 3
}

// isToolsListJSON 检测 JSON 对象是否为 tools/list 响应。
func isToolsListJSON(obj map[string]any) bool {
	if tools, ok := obj["tools"]; ok {
		if _, ok := tools.([]interface{}); ok {
			return true
		}
	}
	return false
}

// isLikelyErrorLine 判断文本行是否像错误信息。
func isLikelyErrorLine(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "invalid") ||
		strings.Contains(lower, "exception") ||
		strings.Contains(lower, "traceback") ||
		strings.Contains(lower, "not found")
}

// truncateForError 截断文本用于错误信息。
func truncateForError(text string, maxLen int) string {
	runes := []rune(strings.ReplaceAll(strings.ReplaceAll(text, "\n", " "), "\r", ""))
	if len(runes) <= maxLen {
		return string(runes)
	}
	return string(runes[:maxLen]) + "..."
}

// validateFunctionsResult 是 resultValidator 的实现，用于 Functions 调用。
func validateFunctionsResult(text string) error {
	if isParamErrorText(text) || isToolNotFoundText(text) {
		return fmt.Errorf("error response: %s", truncateForError(text, 100))
	}

	// JSON 数组 → 检查元素是否像函数
	var arr []any
	if err := json.Unmarshal([]byte(text), &arr); err == nil {
		names := extractFunctionNamesFromArray(arr)
		if len(names) > 0 {
			return nil
		}
		return fmt.Errorf("JSON array has no recognizable function names")
	}

	// JSON 对象 → 检查是否为 server health / tools/list
	var obj map[string]any
	if err := json.Unmarshal([]byte(text), &obj); err == nil {
		if isServerHealthJSON(obj) {
			return fmt.Errorf("server health/status response, not function list")
		}
		if isToolsListJSON(obj) {
			return fmt.Errorf("tools/list response, not function list")
		}
		for _, key := range []string{"functions", "funcs", "items", "results", "data", "entries", "names", "list"} {
			if val, ok := obj[key]; ok {
				if arr, ok := val.([]interface{}); ok {
					if len(extractFunctionNamesFromArray(arr)) > 0 {
						return nil
					}
				}
			}
		}
		return fmt.Errorf("JSON object not recognizable as function list")
	}

	// 纯文本 → 看行数
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) >= 1 {
		var valid int
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !isLikelyErrorLine(line) {
				valid++
			}
		}
		if valid > 0 {
			return nil
		}
	}
	return fmt.Errorf("response not recognizable as function list: %s", truncateForError(text, 100))
}

// Decompile 通过 MCP SSE transport 反编译指定函数。
// 同时使用参数名 fallback：address → addr → name → function_name → target。
func (c *RealMCPClient) Decompile(ctx context.Context, target string) (*IDAMCPDecompileResult, error) {
	transport := newSSETransport(c.endpoint, c.timeout, c.hostHeader)
	text, err := transport.callMCPToolTextWithParamFallback(ctx, decompileToolCandidates, map[string]any{"address": target}, decompileParamCandidates)
	if err != nil {
		return &IDAMCPDecompileResult{Error: fmt.Sprintf("ida_decompile: %v", err)}, nil
	}
	if isParamErrorText(text) || isToolNotFoundText(text) {
		return &IDAMCPDecompileResult{Error: fmt.Sprintf("ida_decompile remote error: %s", text)}, nil
	}
	return &IDAMCPDecompileResult{Code: text}, nil
}

// Strings 通过 MCP SSE transport 获取字符串列表。
// 同时使用参数名 fallback：limit → queries → count → max_results。
func (c *RealMCPClient) Strings(ctx context.Context, limit int) (*IDAMCPStringsResult, error) {
	transport := newSSETransport(c.endpoint, c.timeout, c.hostHeader)
	text, err := transport.callMCPToolTextWithParamFallback(ctx, stringsToolCandidates, map[string]any{"limit": limit}, stringsParamCandidates)
	if err != nil {
		return &IDAMCPStringsResult{Error: fmt.Sprintf("ida_strings: %v", err)}, nil
	}
	return parseStringsResult(text)
}

// parseStringsResult 安全解析 MCP strings 结果，区分错误文本和有效字符串列表。
func parseStringsResult(text string) (*IDAMCPStringsResult, error) {
	if isParamErrorText(text) || isToolNotFoundText(text) {
		return &IDAMCPStringsResult{Error: fmt.Sprintf("ida_strings remote error: %s", text)}, nil
	}
	var strs []string
	if err := json.Unmarshal([]byte(text), &strs); err != nil {
		return &IDAMCPStringsResult{Strings: []string{text}, Total: 1}, nil
	}
	return &IDAMCPStringsResult{Strings: strs, Total: len(strs)}, nil
}

// Xrefs 通过 MCP SSE transport 查询交叉引用。
// 同时使用参数名 fallback：address → addr → target。
func (c *RealMCPClient) Xrefs(ctx context.Context, target string) (*IDAMCPXrefsResult, error) {
	transport := newSSETransport(c.endpoint, c.timeout, c.hostHeader)
	text, err := transport.callMCPToolTextWithParamFallback(ctx, xrefsToolCandidates, map[string]any{"address": target}, xrefsParamCandidates)
	if err != nil {
		return &IDAMCPXrefsResult{Error: fmt.Sprintf("ida_xrefs: %v", err)}, nil
	}
	return parseXrefsResult(text)
}

// Disasm 通过 MCP SSE transport 获取指定地址的反汇编指令。
// 候选远端工具：disasm → insn_query。参数候选：address → start → addr。
func (c *RealMCPClient) Disasm(ctx context.Context, address string, end string, count int) (*IDAMCPDisasmResult, error) {
	transport := newSSETransport(c.endpoint, c.timeout, c.hostHeader)
	args := map[string]any{"address": address}
	if end != "" {
		args["end"] = end
	}
	if count > 0 {
		args["count"] = count
	}
	text, err := transport.callMCPToolTextWithParamFallback(ctx, disasmToolCandidates, args, disasmParamCandidates)
	if err != nil {
		return &IDAMCPDisasmResult{Error: fmt.Sprintf("ida_disasm: %v", err)}, nil
	}
	if isParamErrorText(text) || isToolNotFoundText(text) {
		return &IDAMCPDisasmResult{Error: fmt.Sprintf("ida_disasm remote error: %s", text)}, nil
	}
	return &IDAMCPDisasmResult{Instructions: text}, nil
}

// parseXrefsResult 安全解析 MCP xrefs 结果，区分错误文本和有效交叉引用列表。
func parseXrefsResult(text string) (*IDAMCPXrefsResult, error) {
	if isParamErrorText(text) || isToolNotFoundText(text) {
		return &IDAMCPXrefsResult{Error: fmt.Sprintf("ida_xrefs remote error: %s", text)}, nil
	}
	var xrefs []string
	if err := json.Unmarshal([]byte(text), &xrefs); err != nil {
		return &IDAMCPXrefsResult{Xrefs: []string{text}}, nil
	}
	return &IDAMCPXrefsResult{Xrefs: xrefs}, nil
}

// MockMCPClient 用于单元测试的 IDA MCP 客户端，不依赖真实 IDA 服务。
// 所有方法返回预设数据，测试代码可注入自定义返回值。
type MockMCPClient struct {
	StatusFn    func(ctx context.Context) (*IDAMCPStatus, error)
	FunctionsFn func(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error)
	DecompileFn func(ctx context.Context, target string) (*IDAMCPDecompileResult, error)
	StringsFn   func(ctx context.Context, limit int) (*IDAMCPStringsResult, error)
	XrefsFn     func(ctx context.Context, target string) (*IDAMCPXrefsResult, error)
	DisasmFn    func(ctx context.Context, address string, end string, count int) (*IDAMCPDisasmResult, error)
}

func (m *MockMCPClient) Status(ctx context.Context) (*IDAMCPStatus, error) {
	if m.StatusFn != nil {
		return m.StatusFn(ctx)
	}
	return &IDAMCPStatus{Available: true, Endpoint: "mock://ida"}, nil
}

func (m *MockMCPClient) Functions(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error) {
	if m.FunctionsFn != nil {
		return m.FunctionsFn(ctx, limit)
	}
	return &IDAMCPFunctionsResult{Functions: []string{"main", "check_password", "verify"}, Total: 3}, nil
}

func (m *MockMCPClient) Decompile(ctx context.Context, target string) (*IDAMCPDecompileResult, error) {
	if m.DecompileFn != nil {
		return m.DecompileFn(ctx, target)
	}
	return &IDAMCPDecompileResult{Code: "int " + target + "() { return 0; }"}, nil
}

func (m *MockMCPClient) Strings(ctx context.Context, limit int) (*IDAMCPStringsResult, error) {
	if m.StringsFn != nil {
		return m.StringsFn(ctx, limit)
	}
	return &IDAMCPStringsResult{Strings: []string{"password", "flag{", "admin"}, Total: 3}, nil
}

func (m *MockMCPClient) Xrefs(ctx context.Context, target string) (*IDAMCPXrefsResult, error) {
	if m.XrefsFn != nil {
		return m.XrefsFn(ctx, target)
	}
	return &IDAMCPXrefsResult{Xrefs: []string{"call " + target + " from main+0x42", "ref to " + target + " at data+0x10"}}, nil
}

func (m *MockMCPClient) Disasm(ctx context.Context, address string, end string, count int) (*IDAMCPDisasmResult, error) {
	if m.DisasmFn != nil {
		return m.DisasmFn(ctx, address, end, count)
	}
	return &IDAMCPDisasmResult{Instructions: address + ": mov eax, ebx\n" + address + "+4: cmp eax, 0\n" + address + "+8: jne " + address}, nil
}

// DisabledMCPClient IDA MCP 配置异常时使用，所有方法返回配置错误。
// 用于 endpoint 非法但不想阻止服务启动的场景。
type DisabledMCPClient struct {
	configError string
}

// NewDisabledMCPClient 创建 disabled client，configError 描述配置问题。
func NewDisabledMCPClient(configError string) *DisabledMCPClient {
	return &DisabledMCPClient{configError: configError}
}

func (d *DisabledMCPClient) Status(ctx context.Context) (*IDAMCPStatus, error) {
	return &IDAMCPStatus{Available: false, Endpoint: "", Error: d.configError}, nil
}

func (d *DisabledMCPClient) Functions(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error) {
	return &IDAMCPFunctionsResult{Error: d.configError}, nil
}

func (d *DisabledMCPClient) Decompile(ctx context.Context, target string) (*IDAMCPDecompileResult, error) {
	return &IDAMCPDecompileResult{Error: d.configError}, nil
}

func (d *DisabledMCPClient) Strings(ctx context.Context, limit int) (*IDAMCPStringsResult, error) {
	return &IDAMCPStringsResult{Error: d.configError}, nil
}

func (d *DisabledMCPClient) Xrefs(ctx context.Context, target string) (*IDAMCPXrefsResult, error) {
	return &IDAMCPXrefsResult{Error: d.configError}, nil
}

func (d *DisabledMCPClient) Disasm(ctx context.Context, address string, end string, count int) (*IDAMCPDisasmResult, error) {
	return &IDAMCPDisasmResult{Error: d.configError}, nil
}

// EnvIDAEndpoint 读取 IDA MCP 端点环境变量。优先 IDA_MCP_URL，其次 IDA_MCP_ENDPOINT。
// 读不到时返回默认值，不做校验——校验由 NewRealMCPClient 完成。
func EnvIDAEndpoint() string {
	s := strings.TrimSpace(os.Getenv("IDA_MCP_URL"))
	if s == "" {
		s = strings.TrimSpace(os.Getenv("IDA_MCP_ENDPOINT"))
	}
	if s == "" {
		return defaultIDAEndpoint
	}
	return s
}

// EnvIDATimeout 读取 IDA_MCP_TIMEOUT_SECONDS 环境变量。
func EnvIDATimeout() int {
	s := strings.TrimSpace(os.Getenv("IDA_MCP_TIMEOUT_SECONDS"))
	if s == "" {
		return defaultIDATimeoutSec
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return defaultIDATimeoutSec
	}
	return v
}
