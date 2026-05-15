package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// extractMCPText

func TestExtractMCPText_PlainString(t *testing.T) {
	result := json.RawMessage(`"hello world"`)
	text, err := extractMCPText(result)
	if err != nil {
		t.Fatalf("extractMCPText: %v", err)
	}
	if text != "hello world" {
		t.Errorf("got %q, want 'hello world'", text)
	}
}

func TestExtractMCPText_ContentFormat(t *testing.T) {
	result := json.RawMessage(`{"content":[{"type":"text","text":"line1"},{"type":"text","text":"line2"}]}`)
	text, err := extractMCPText(result)
	if err != nil {
		t.Fatalf("extractMCPText: %v", err)
	}
	if text != "line1\nline2" {
		t.Errorf("got %q, want 'line1\\nline2'", text)
	}
}

func TestExtractMCPText_SingleContentItem(t *testing.T) {
	result := json.RawMessage(`{"content":[{"type":"text","text":"function decompiled"}]}`)
	text, err := extractMCPText(result)
	if err != nil {
		t.Fatalf("extractMCPText: %v", err)
	}
	if text != "function decompiled" {
		t.Errorf("got %q", text)
	}
}

func TestExtractMCPText_EmptyContent(t *testing.T) {
	result := json.RawMessage(`{"content":[]}`)
	text, err := extractMCPText(result)
	if err != nil {
		t.Fatalf("extractMCPText: %v", err)
	}
	if !strings.Contains(text, "content") {
		t.Errorf("should contain raw JSON for empty content: %q", text)
	}
}

func TestExtractMCPText_NonTextType(t *testing.T) {
	result := json.RawMessage(`{"content":[{"type":"image","data":"xxx"},{"type":"text","text":"actual text"}]}`)
	text, err := extractMCPText(result)
	if err != nil {
		t.Fatalf("extractMCPText: %v", err)
	}
	if text != "actual text" {
		t.Errorf("got %q, want 'actual text'", text)
	}
}

// tools/list

func TestListMCPTools(t *testing.T) {
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"list_functions\"},{\"name\":\"decompile_function\"},{\"name\":\"list_strings\"},{\"name\":\"get_xrefs_to\"}]}}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	names, err := transport.listMCPTools(context.Background())
	if err != nil {
		t.Fatalf("listMCPTools: %v", err)
	}
	if len(names) != 4 {
		t.Errorf("expected 4 tools, got %d: %v", len(names), names)
	}
}

func TestListMCPTools_RealNames(t *testing.T) {
	// 模拟 zeromcp 返回 ida_ 前缀的工具名
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"ida_functions\"},{\"name\":\"ida_decompile\"},{\"name\":\"ida_strings\"},{\"name\":\"ida_xrefs\"}]}}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	names, err := transport.listMCPTools(context.Background())
	if err != nil {
		t.Fatalf("listMCPTools: %v", err)
	}
	expected := map[string]bool{"ida_functions": true, "ida_decompile": true, "ida_strings": true, "ida_xrefs": true}
	for _, name := range names {
		if !expected[name] {
			t.Errorf("unexpected tool name: %q", name)
		}
	}
}

// MCP content format 集成

func TestSSETransport_MCPContentFormat(t *testing.T) {
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		resp := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"part A"},{"type":"text","text":"part B"}]}}`
		w.Write([]byte(fmt.Sprintf("event: message\ndata: %s\n\n", resp)))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	text, err := transport.callMCPToolText(context.Background(), "test_tool", map[string]any{})
	if err != nil {
		t.Fatalf("callMCPToolText: %v", err)
	}
	if text != "part A\npart B" {
		t.Errorf("expected 'part A\\npart B', got %q", text)
	}
}

// IDA tools through SSE

func TestIDAXrefsThroughSSE(t *testing.T) {
	server := newMockMCPServer(t)
	defer server.Close()
	server.addResponse(1, `["call check_password from main+0x42","ref flag_data at data+0x10"]`)

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	defer restoreIDAClient(setTestIDAClient(c))

	result, err := c.Xrefs(context.Background(), "check_password")
	if err != nil {
		t.Fatalf("Xrefs: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Xrefs error: %s", result.Error)
	}
	if !strings.Contains(result.Xrefs[0], "check_password") {
		t.Errorf("xrefs should contain target: %q", result.Xrefs)
	}
}

func TestIDAStringsThroughSSE(t *testing.T) {
	server := newMockMCPServer(t)
	defer server.Close()
	server.addResponse(1, `["flag{test}","password123","admin"]`)

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	defer restoreIDAClient(setTestIDAClient(c))

	result, err := c.Strings(context.Background(), 10)
	if err != nil {
		t.Fatalf("Strings: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Strings error: %s", result.Error)
	}
	if len(result.Strings) != 3 {
		t.Errorf("expected 3 strings, got %d", len(result.Strings))
	}
}

// 相对 endpoint 解析

func TestResolveRelativeEndpoint_SlashPath(t *testing.T) {
	resolved, err := resolveAndValidatePostEndpoint("/sse?session=abc", "http://127.0.0.1:13338/sse")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != "http://127.0.0.1:13338/sse?session=abc" {
		t.Errorf("got %q", resolved)
	}
}

func TestResolveRelativeEndpoint_NoSlashPath(t *testing.T) {
	resolved, err := resolveAndValidatePostEndpoint("messages?session=abc", "http://127.0.0.1:13338/sse")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != "http://127.0.0.1:13338/messages?session=abc" {
		t.Errorf("got %q", resolved)
	}
}

func TestResolveFullURLEndpoint(t *testing.T) {
	resolved, err := resolveAndValidatePostEndpoint("http://127.0.0.1:13338/sse?session=abc", "http://127.0.0.1:13338/sse")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved != "http://127.0.0.1:13338/sse?session=abc" {
		t.Errorf("got %q", resolved)
	}
}

func TestResolveFullURLRejectRemote(t *testing.T) {
	_, err := resolveAndValidatePostEndpoint("http://evil.com/sse", "http://127.0.0.1:13338/sse")
	if err == nil {
		t.Fatal("remote endpoint should be rejected")
	}
}

func TestResolveFullURLAllowPrivateIP(t *testing.T) {
	_, err := resolveAndValidatePostEndpoint("http://192.168.1.2/sse", "http://127.0.0.1:13338/sse")
	if err != nil {
		t.Fatalf("private IP endpoint should be allowed for WSL scenarios: %v", err)
	}
}

// 相对 endpoint 集成测试

func TestIDAFunctions_RelativeEndpoint(t *testing.T) {
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"verify\",\"check\"]}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = "/sse?session=test123"

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	defer restoreIDAClient(setTestIDAClient(c))

	result, err := c.Functions(context.Background(), 10)
	if err != nil {
		t.Fatalf("Functions: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Functions error: %s", result.Error)
	}
	if len(result.Functions) != 3 {
		t.Errorf("expected 3 functions, got %d: %v", len(result.Functions), result.Functions)
	}
}

// isToolNotFound / isToolNotFoundText

func TestIsToolNotFound(t *testing.T) {
	tests := []struct {
		errMsg   string
		expected bool
	}{
		{"Method 'list_functions' not found", true},
		{"mcp rpc error -32601: Method not found", true},
		{"method not_found", true},
		// Tool not found 变体（不同 MCP 实现）
		{"Tool not found", true},
		{"tool not_found", true},
		{"Unknown method: list_functions", true},
		{"Unknown tool: ida_functions", true},
		{"tool 'list_funcs' not registered", true},
		// 不应匹配
		{"sse connect: connection refused", false},
		{"sse stream closed before message", false},
		{"timeout", false},
	}
	for _, tt := range tests {
		got := isToolNotFound(errors.New(tt.errMsg))
		if got != tt.expected {
			t.Errorf("isToolNotFound(%q) = %v, want %v", tt.errMsg, got, tt.expected)
		}
	}
}

func TestIsToolNotFoundText(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"Method 'list_functions' not found", true},
		{"Tool 'ida_functions' not found", true},
		{"tool not registered", true},
		{"Unknown tool foobar", true},
		{"decompiled code here", false},
		{"main, verify, check", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isToolNotFoundText(tt.text); got != tt.expected {
			t.Errorf("isToolNotFoundText(%q) = %v, want %v", tt.text, got, tt.expected)
		}
	}
}

// 候选名 fallback

func TestFunctionsFallback_AllCandidatesFail(t *testing.T) {
	// 单候选失败即可验证错误信息格式
	server := newMockMCPServerError(t, "Method 'bad1' not found")
	defer server.Close()

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	_, err := transport.callMCPToolTextWithFallback(context.Background(), []string{"bad1"}, map[string]any{})
	if err == nil {
		t.Fatal("expected error when all candidates fail")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should indicate tool name not found: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "[bad1]") {
		t.Errorf("error should list tried candidates: %s", err.Error())
	}
}

func newMockMCPServerError(t *testing.T, errMsg string) *httptest.Server {
	t.Helper()
	var postURL string
	s := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"%s\"}}\n\n", errMsg)))
		flusher.Flush()
	}))
	postURL = s.URL + "/msg"
	return s
}

func TestDecompileFallback_Success(t *testing.T) {
	server := newMockMCPServer(t)
	defer server.Close()
	server.addResponse(1, `"decompiled code here"`)

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	defer restoreIDAClient(setTestIDAClient(c))

	result, err := c.Decompile(context.Background(), "main")
	if err != nil {
		t.Fatalf("Decompile: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Decompile error: %s", result.Error)
	}
	if !strings.Contains(result.Code, "decompiled") {
		t.Errorf("expected decompiled code: %q", result.Code)
	}
}

// Method not found 不包装成 "unavailable"

func TestMethodNotFound_NotReportedAsUnavailable(t *testing.T) {
	server := newMockMCPServerError(t, "Method not found")
	defer server.Close()

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	_, err := transport.callMCPToolTextWithFallback(context.Background(), []string{"wrong_tool"}, map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(strings.ToLower(err.Error()), "unavailable") {
		t.Errorf("error should NOT say 'unavailable': %s", err.Error())
	}
}

// 所有候选失败时，错误信息包含 tools/list 返回的 available tools

func TestFallbackError_IncludesAvailableTools(t *testing.T) {
	// 自愈 fallback：候选失败 → tools/list → 自动重试成功 → 不应报错
	var postURL string
	var connCount int
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		if r.Method == http.MethodGet {
			connCount++
		}
		if connCount == 1 {
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"Method 'bad1' not found\"}}\n\n"))
		} else if connCount == 2 {
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"ida_functions\"},{\"name\":\"list_funcs\"}]}}\n\n"))
		} else {
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"verify\"]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	text, err := transport.callMCPToolTextWithFallback(context.Background(), []string{"bad1"}, map[string]any{})
	if err != nil {
		t.Fatalf("self-healing should succeed: %v", err)
	}
	if !strings.Contains(text, "main") {
		t.Errorf("self-healed result should contain 'main': %q", text)
	}
}

// isResultMethodNotFound (已合并到上方 TestIsToolNotFoundText)

// fallback: Method not found 在 MCP result content 中而非 JSON-RPC error

func TestFallback_MethodNotFoundInResultContent(t *testing.T) {
	// 模拟 zeromcp：Method not found 作为 tools/call result content 返回
	var postURL string
	var connCount int
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		if r.Method == http.MethodGet {
			connCount++
		}
		if connCount == 1 {
			// 第 1 个候选：result content 包含 Method not found
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Method 'list_functions' not found\"}],\"isError\":true}}\n\n"))
		} else {
			// 第 2 个候选：成功返回
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"verify\"]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	text, err := transport.callMCPToolTextWithFallback(
		context.Background(),
		[]string{"list_functions", "ida_functions"},
		map[string]any{"limit": 10},
	)
	if err != nil {
		t.Fatalf("fallback should succeed on second candidate: %v", err)
	}
	if !strings.Contains(text, "main") {
		t.Errorf("got %q, want 'main' in result", text)
	}
}

// 自愈 fallback：候选全部失败，tools/list 返回新工具名，自动重试成功

func TestSelfHealingFallback(t *testing.T) {
	var postURL string
	var connCount int
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		if r.Method == http.MethodGet {
			connCount++
		}
		if connCount <= 2 {
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"Method not found\"}}\n\n"))
		} else if connCount == 3 {
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"list_funcs\"},{\"name\":\"decompile\"}]}}\n\n"))
		} else {
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"verify\",\"check\"]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	text, err := transport.callMCPToolTextWithFallback(
		context.Background(),
		[]string{"bad1", "bad2"},
		map[string]any{"limit": 10},
	)
	if err != nil {
		t.Fatalf("self-healing fallback should succeed: %v", err)
	}
	if !strings.Contains(text, "main") {
		t.Errorf("got %q, want 'main' in self-healed result", text)
	}
}

func TestDiffTools(t *testing.T) {
	tools := []string{"list_funcs", "decompile", "ida_functions"}
	candidates := []string{"ida_functions", "get_functions"}
	diff := diffTools(tools, candidates)
	if len(diff) != 2 {
		t.Errorf("expected 2 new tools, got %d: %v", len(diff), diff)
	}
	if diff[0] != "list_funcs" || diff[1] != "decompile" {
		t.Errorf("unexpected diff: %v", diff)
	}
}

// isParamError

func TestIsParamErrorText(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"Invalid params: missing required parameters: ['queries']", true},
		{"invalid params", true},
		{"missing required parameter", true},
		{"unexpected parameter", true},
		{"unknown parameter 'addr'", true},
		{"bad parameter", true},
		{"decompiled code here", false},
		{"main, verify, check", false},
		{"Method not found", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isParamErrorText(tt.text); got != tt.expected {
			t.Errorf("isParamErrorText(%q) = %v, want %v", tt.text, got, tt.expected)
		}
	}
}

// buildAltArgSets

func TestBuildAltArgSets(t *testing.T) {
	// functions: limit → queries → count
	sets := buildAltArgSets(map[string]any{"limit": 200}, functionsParamCandidates)
	if len(sets) != 4 { // limit, queries, count, max_results
		t.Fatalf("expected 4 sets, got %d", len(sets))
	}
	// First: original key
	if _, ok := sets[0]["limit"]; !ok {
		t.Errorf("first set should have 'limit': %v", sets[0])
	}
	// Second: first alternative
	if _, ok := sets[1]["queries"]; !ok {
		t.Errorf("second set should have 'queries': %v", sets[1])
	}
	// Verify all values are 200
	for i, s := range sets {
		for _, v := range s {
			if v != 200 {
				t.Errorf("set %d value should be 200, got %v", i, v)
			}
		}
	}
}

func TestBuildAltArgSets_Decompile(t *testing.T) {
	sets := buildAltArgSets(map[string]any{"address": "main"}, decompileParamCandidates)
	if len(sets) != 5 { // address, addr, name, function_name, target
		t.Fatalf("expected 5 sets, got %d", len(sets))
	}
	if _, ok := sets[0]["address"]; !ok {
		t.Errorf("first set should have 'address': %v", sets[0])
	}
	if _, ok := sets[1]["addr"]; !ok {
		t.Errorf("second set should have 'addr': %v", sets[1])
	}
}

// 参数错误不被当作有效数据

func TestParseFunctionsResult_RejectsParamError(t *testing.T) {
	result, _ := parseFunctionsResult("Invalid params: missing required parameters: ['queries']")
	if result.Error == "" {
		t.Fatal("should return error for param error text")
	}
	if !strings.Contains(result.Error, "remote error") {
		t.Errorf("error should indicate remote error: %q", result.Error)
	}
	if len(result.Functions) != 0 {
		t.Errorf("functions should be empty when error detected: %v", result.Functions)
	}
}

func TestParseStringsResult_RejectsParamError(t *testing.T) {
	result, _ := parseStringsResult("Invalid params: unknown parameter 'limit'")
	if result.Error == "" {
		t.Fatal("should return error for param error text")
	}
	if result.Total != 0 {
		t.Errorf("total should be 0 when error detected: %d", result.Total)
	}
}

func TestParseXrefsResult_RejectsParamError(t *testing.T) {
	result, _ := parseXrefsResult("bad parameter: missing required parameter: address")
	if result.Error == "" {
		t.Fatal("should return error for param error text")
	}
}

func TestParseFunctionsResult_RejectsMethodNotFound(t *testing.T) {
	result, _ := parseFunctionsResult("Method 'list_functions' not found")
	if result.Error == "" {
		t.Fatal("should return error when result contains method not found")
	}
}

// 参数 fallback 集成测试

func TestParamFallback_FunctionsSuccess(t *testing.T) {
	// 模拟：工具名 ida_functions 存在，但参数 "limit" 无效，参数 "queries" 有效
	var postURL string
	var connCount int
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		if r.Method == http.MethodGet {
			connCount++
		}
		// GET 1: ida_functions + limit → param error
		// GET 2: ida_functions + queries → success
		switch connCount {
		case 1:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Invalid params: missing required parameters: ['queries']\"}],\"isError\":true}}\n\n"))
		case 2:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"check_password\",\"flag_check\"]}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	text, err := transport.callMCPToolTextWithParamFallback(
		context.Background(),
		[]string{"ida_functions"},
		map[string]any{"limit": 10},
		functionsParamCandidates,
	)
	if err != nil {
		t.Fatalf("param fallback should succeed: %v", err)
	}
	if !strings.Contains(text, "main") {
		t.Errorf("result should contain 'main': %q", text)
	}
	if !strings.Contains(text, "check_password") {
		t.Errorf("result should contain 'check_password': %q", text)
	}
}

func TestParamFallback_DecompileSuccess(t *testing.T) {
	// 模拟：工具 decompile_func 存在，参数 "address" 无效，参数 "function_name" 有效
	var postURL string
	var connCount int
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		if r.Method == http.MethodGet {
			connCount++
		}
		switch connCount {
		case 1:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"Invalid params: unexpected parameter 'address'\"}\n\n"))
		case 2:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"int main() { return 0; }\"}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"\"}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	text, err := transport.callMCPToolTextWithParamFallback(
		context.Background(),
		[]string{"decompile_func"},
		map[string]any{"address": "main"},
		decompileParamCandidates,
	)
	if err != nil {
		t.Fatalf("param fallback should succeed: %v", err)
	}
	if !strings.Contains(text, "main()") {
		t.Errorf("result should contain decompiled code: %q", text)
	}
}

func TestParamFallback_AllFail_ErrorIncludesInfo(t *testing.T) {
	// 模拟：只有 ida_functions 工具存在，但所有参数名都失败
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		// 总是返回参数错误
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Invalid params: missing required parameters: ['queries']\"}],\"isError\":true}}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	_, err := transport.callMCPToolTextWithParamFallback(
		context.Background(),
		[]string{"ida_functions"},
		map[string]any{"limit": 10},
		functionsParamCandidates,
	)
	if err == nil {
		t.Fatal("expected error when all fallbacks fail")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "ida_functions") {
		t.Errorf("error should mention attempted tool: %s", errMsg)
	}
	if !strings.Contains(strings.ToLower(errMsg), "tried") {
		t.Errorf("error should mention tried candidates: %s", errMsg)
	}
}

// tools/list 返回 list_funcs，保守候选 ida_functions 不存在，fallback 应成功

func TestToolsListHasListFuncs_IDAFunctionsFallbackSucceeds(t *testing.T) {
	// 候选工具名 "ida_functions" → Method not found
	// tools/list 返回 ["list_funcs"]
	// 自愈 fallback 用 list_funcs + queries 参数成功
	var postURL string
	var connCount int
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		if r.Method == http.MethodGet {
			connCount++
		}
		switch {
		case connCount <= 1:
			// 候选 tool name 失败（Method not found）
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"Method not found\"}}\n\n"))
		case connCount == 2:
			// tools/list 返回 list_funcs
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"list_funcs\"}]}}\n\n"))
		case connCount == 3:
			// list_funcs + limit → param error
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Invalid params: missing required parameters: ['queries']\"}],\"isError\":true}}\n\n"))
		case connCount == 4:
			// list_funcs + queries → success
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"check_password\",\"verify\"]}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	text, err := transport.callMCPToolTextWithParamFallback(
		context.Background(),
		[]string{"ida_functions"},
		map[string]any{"limit": 10},
		functionsParamCandidates,
	)
	if err != nil {
		t.Fatalf("tools/list fallback should succeed: %v", err)
	}
	if !strings.Contains(text, "main") {
		t.Errorf("result should contain 'main': %q", text)
	}
}

// 验证 ReAct 暴露给模型的工具名保持稳定

func TestEinoToolNames_Stable(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{}))

	expectedNames := []string{
		"ida_status",
		"ida_functions",
		"ida_decompile",
		"ida_strings",
		"ida_xrefs",
	}

	ctx := context.Background()
	for _, name := range expectedNames {
		switch name {
		case "ida_status":
			tool, _ := NewIDAStatusTool()
			info, _ := tool.Info(ctx)
			if info != nil && info.Name != name {
				t.Errorf("tool name mismatch: got %q, want %q", info.Name, name)
			}
		case "ida_functions":
			tool, _ := NewIDAFunctionsTool()
			info, _ := tool.Info(ctx)
			if info != nil && info.Name != name {
				t.Errorf("tool name mismatch: got %q, want %q", info.Name, name)
			}
		case "ida_decompile":
			tool, _ := NewIDADecompileTool()
			info, _ := tool.Info(ctx)
			if info != nil && info.Name != name {
				t.Errorf("tool name mismatch: got %q, want %q", info.Name, name)
			}
		case "ida_strings":
			tool, _ := NewIDAStringsTool()
			info, _ := tool.Info(ctx)
			if info != nil && info.Name != name {
				t.Errorf("tool name mismatch: got %q, want %q", info.Name, name)
			}
		case "ida_xrefs":
			tool, _ := NewIDAXrefsTool()
			info, _ := tool.Info(ctx)
			if info != nil && info.Name != name {
				t.Errorf("tool name mismatch: got %q, want %q", info.Name, name)
			}
		}
	}
}

// 工具注册层集成测试：模拟真实 ReAct Agent 调用 ida_functions 走完整 fallback 路径。
// 此测试用 mock MCP SSE server 模拟远端，但通过真实的 Eino InvokableTool 调用。

func TestRegistryLayer_FunctionsFallbackToRemoteName(t *testing.T) {
	// Mock MCP server：list_funcs 存在（参数 queries），其他候选名均不存在。
	// 调用链：Eino tool ida_functions → idaClient.Functions → list_funcs 候选
	// → list_funcs + limit 参数错误 → list_funcs + queries 成功。
	var postURL string
	var connCount int
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		if r.Method == http.MethodGet {
			connCount++
		}
		// 流程：
		// conn 1: list_funcs + limit → param error → 触发参数 fallback
		// conn 2: list_funcs + queries → SUCCESS → return
		switch connCount {
		case 1:
			// list_funcs 存在但参数名不对（需要 queries 而非 limit）
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Invalid params: missing required parameters: ['queries']\"}],\"isError\":true}}\n\n"))
		case 2:
			// 参数 fallback：list_funcs + queries → 返回函数列表
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"check_password\",\"verify_flag\",\"auth_b64d\"]}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	// 构造 RealMCPClient 指向 mock server
	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	defer restoreIDAClient(setTestIDAClient(c))

	// 获取注册给 ReAct Agent 的 ida_functions 工具
	tool, err := NewIDAFunctionsTool()
	if err != nil {
		t.Fatalf("NewIDAFunctionsTool: %v", err)
	}

	// 验证对模型暴露的工具名是 ida_functions（稳定名）
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("tool.Info: %v", err)
	}
	if info.Name != "ida_functions" {
		t.Errorf("tool name exposed to model: got %q, want 'ida_functions'", info.Name)
	}

	// 通过 Eino InvokableTool 接口调用（模拟 ReAct Agent 调用路径）
	out := invokeIDATool[IDAFunctionsInput, IDAFunctionsOutput](t, tool, IDAFunctionsInput{Limit: 50})

	// 断言：不应返回错误
	if out.Error != "" {
		t.Fatalf("registry-layer call should succeed via fallback; got error: %s", out.Error)
	}

	// 断言：结果应包含函数列表
	if !strings.Contains(out.Functions, "main") {
		t.Errorf("result should contain 'main': %s", out.Functions)
	}
	if !strings.Contains(out.Functions, "check_password") {
		t.Errorf("result should contain 'check_password': %s", out.Functions)
	}

	// 断言：结果不应包含 "MCP tool name not found"
	if strings.Contains(out.Functions, "not found") {
		t.Errorf("result should NOT contain 'not found': %s", out.Functions)
	}
	if strings.Contains(out.Functions, "MCP tool name") {
		t.Errorf("result should NOT contain 'MCP tool name': %s", out.Functions)
	}

	// 断言：结果不应包含远端原始错误文本
	if strings.Contains(out.Functions, "Invalid params") {
		t.Errorf("result should NOT contain 'Invalid params': %s", out.Functions)
	}
}

// 同样的注册层测试覆盖 ida_decompile
func TestRegistryLayer_DecompileFallbackToRemoteName(t *testing.T) {
	var postURL string
	var connCount int
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		if r.Method == http.MethodGet {
			connCount++
		}
		switch connCount {
		case 1:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"Tool not found\"}}\n\n"))
		case 2:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"int main(int argc, char **argv) { return check_flag(argv[1]); }\"}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"\"}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	defer restoreIDAClient(setTestIDAClient(c))

	tool, err := NewIDADecompileTool()
	if err != nil {
		t.Fatalf("NewIDADecompileTool: %v", err)
	}

	out := invokeIDATool[IDADecompileInput, IDADecompileOutput](t, tool, IDADecompileInput{Target: "main"})
	if out.Error != "" {
		t.Fatalf("registry-layer decompile should succeed via fallback; got error: %s", out.Error)
	}
	if !strings.Contains(out.Code, "check_flag") {
		t.Errorf("result should contain 'check_flag': %s", out.Code)
	}
}

// TestRegistryLayer_FunctionsFallbackWithEmptyParams 模拟 zeromcp 场景：
// list_funcs 不接受任何参数（limit/queries/count/max_results 均返回 param error），
// 空参数兜底最终成功。验证对模型暴露的工具名保持 ida_functions。
func TestRegistryLayer_FunctionsFallbackWithEmptyParams(t *testing.T) {
	var postURL string
	var connCount int
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		if r.Method == http.MethodGet {
			connCount++
		}
		// zeromcp: list_funcs 不接受任何带 key 的参数，只有空参数能成功
		switch {
		case connCount <= 4:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"Invalid params: unexpected parameter\"}],\"isError\":true}}\n\n"))
		case connCount == 5:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"check_password\",\"verify_flag\",\"auth_b64d\",\"init\"]}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	defer restoreIDAClient(setTestIDAClient(c))

	tool, err := NewIDAFunctionsTool()
	if err != nil {
		t.Fatalf("NewIDAFunctionsTool: %v", err)
	}

	// 验证对模型暴露的工具名是 ida_functions
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("tool.Info: %v", err)
	}
	if info.Name != "ida_functions" {
		t.Errorf("tool name exposed to model: got %q, want 'ida_functions'", info.Name)
	}

	out := invokeIDATool[IDAFunctionsInput, IDAFunctionsOutput](t, tool, IDAFunctionsInput{Limit: 200})

	// 不应返回错误
	if out.Error != "" {
		t.Fatalf("empty-params fallback should succeed; got error: %s", out.Error)
	}

	// 结果应包含函数列表
	if !strings.Contains(out.Functions, "main") {
		t.Errorf("result should contain 'main': %s", out.Functions)
	}
	if !strings.Contains(out.Functions, "check_password") {
		t.Errorf("result should contain 'check_password': %s", out.Functions)
	}

	// 断言结果不包含错误提示
	forbidden := []string{
		"MCP tool name not found",
		"ida_functions 不存在",
		"请改用 list_funcs",
		"Invalid Host",
		"Invalid params",
	}
	for _, phrase := range forbidden {
		if strings.Contains(out.Functions, phrase) {
			t.Errorf("result should NOT contain %q: %s", phrase, out.Functions)
		}
	}
}

// Host header override tests

// TestHostHeader_SSEGetRequest verifies GET SSE request carries the overridden Host header.
func TestHostHeader_SSEGetRequest(t *testing.T) {
	var capturedHost string
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			capturedHost = r.Host
		}
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"ok\"}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "127.0.0.1:13338")
	_, err := transport.callMCPToolText(context.Background(), "test_tool", map[string]any{})
	if err != nil {
		t.Fatalf("callMCPToolText: %v", err)
	}
	if capturedHost != "127.0.0.1:13338" {
		t.Errorf("GET Host header: got %q, want '127.0.0.1:13338'", capturedHost)
	}
}

// TestHostHeader_SSEPostRequest verifies POST tools/call request carries the overridden Host header.
func TestHostHeader_SSEPostRequest(t *testing.T) {
	var capturedPostHost string
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			capturedPostHost = r.Host
		}
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"ok\"}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "127.0.0.1:13338")
	_, err := transport.callMCPToolText(context.Background(), "test_tool", map[string]any{})
	if err != nil {
		t.Fatalf("callMCPToolText: %v", err)
	}
	if capturedPostHost != "127.0.0.1:13338" {
		t.Errorf("POST Host header: got %q, want '127.0.0.1:13338'", capturedPostHost)
	}
}

// TestHostHeader_ListMCPTools verifies tools/list request carries the overridden Host header.
func TestHostHeader_ListMCPTools(t *testing.T) {
	var capturedGetHost, capturedPostHost string
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			capturedGetHost = r.Host
		} else if r.Method == http.MethodPost {
			capturedPostHost = r.Host
		}
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"list_funcs\"}]}}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "127.0.0.1:13338")
	_, err := transport.listMCPTools(context.Background())
	if err != nil {
		t.Fatalf("listMCPTools: %v", err)
	}
	if capturedGetHost != "127.0.0.1:13338" {
		t.Errorf("GET Host header: got %q, want '127.0.0.1:13338'", capturedGetHost)
	}
	if capturedPostHost != "127.0.0.1:13338" {
		t.Errorf("POST Host header: got %q, want '127.0.0.1:13338'", capturedPostHost)
	}
}

// TestHostHeader_NoOverride verifies Host header is not modified when hostHeader is empty.
func TestHostHeader_NoOverride(t *testing.T) {
	var capturedHost string
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			capturedHost = r.Host
		}
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"ok\"}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	_, err := transport.callMCPToolText(context.Background(), "test_tool", map[string]any{})
	if err != nil {
		t.Fatalf("callMCPToolText: %v", err)
	}
	if capturedHost == "127.0.0.1:13338" {
		t.Errorf("Host header should NOT be overridden when hostHeader is empty, got %q", capturedHost)
	}
}

// TestHostHeader_FallbackPersists verifies Host header persists through fallback retries.
func TestHostHeader_FallbackPersists(t *testing.T) {
	var capturedHosts []string
	var postURL string
	var connCount int
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			capturedHosts = append(capturedHosts, r.Host)
			connCount++
		}
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		switch connCount {
		case 1, 2:
			// Both bad1 and bad2 → Method not found
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"Method not found\"}}\n\n"))
		case 3:
			// tools/list → returns real tool names
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"list_funcs\"}]}}\n\n"))
		case 4:
			// retry with list_funcs → success
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"verify\"]}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "127.0.0.1:13338")
	text, err := transport.callMCPToolTextWithFallback(
		context.Background(),
		[]string{"bad1", "bad2"},
		map[string]any{"limit": 10},
	)
	if err != nil {
		t.Fatalf("fallback should succeed: %v", err)
	}
	if !strings.Contains(text, "main") {
		t.Errorf("result should contain 'main': %q", text)
	}
	if len(capturedHosts) < 3 {
		t.Fatalf("expected at least 3 GET requests, got %d", len(capturedHosts))
	}
	for i, host := range capturedHosts {
		if host != "127.0.0.1:13338" {
			t.Errorf("GET request %d Host header: got %q, want '127.0.0.1:13338'", i+1, host)
		}
	}
}
