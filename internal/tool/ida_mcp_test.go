package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
)

// invokeIDATool 通过 InvokableTool 接口调用 IDA 工具并解析返回值。
func invokeIDATool[I, O any](t testing.TB, tool einotool.InvokableTool, input I) O {
	t.Helper()
	inputJSON, _ := json.Marshal(input)
	outputJSON, err := tool.InvokableRun(context.Background(), string(inputJSON))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	var out O
	if err := json.Unmarshal([]byte(outputJSON), &out); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, outputJSON)
	}
	return out
}

// endpoint validation

func TestIDAEndpointDefault(t *testing.T) {
	if defaultIDAEndpoint != "http://127.0.0.1:13338/sse" {
		t.Errorf("default endpoint: got %q, want http://127.0.0.1:13338/sse", defaultIDAEndpoint)
	}
}

func TestIDAEndpointAllowLocalhost(t *testing.T) {
	allowed := []string{
		"http://127.0.0.1:13338/sse",
		"http://localhost:13338/sse",
		"http://127.0.0.1:8080/mcp",
		"http://localhost:12345/",
	}
	for _, ep := range allowed {
		validated, err := validateIDAEndpoint(ep)
		if err != nil {
			t.Errorf("endpoint %q should be allowed: %v", ep, err)
		}
		if validated != ep {
			t.Errorf("endpoint %q: got validated=%q", ep, validated)
		}
	}
}

func TestIDAEndpointRejectRemote(t *testing.T) {
	rejected := []string{
		"http://example.com:13338/sse",
		"http://0.0.0.0:13338/sse",
		"https://remote.example.com/mcp",
		"http://[::1]:13338/sse",
		"http://8.8.8.8:13338/sse",
		"http://1.1.1.1:13338/sse",
	}
	for _, ep := range rejected {
		_, err := validateIDAEndpoint(ep)
		if err == nil {
			t.Errorf("endpoint %q should be rejected", ep)
		}
	}
}

func TestIDAEndpointRejectEmptyHost(t *testing.T) {
	_, err := validateIDAEndpoint("http://:13338/sse")
	if err == nil {
		t.Error("endpoint with empty host should be rejected")
	}
}

func TestIDAEndpointRejectInvalidScheme(t *testing.T) {
	_, err := validateIDAEndpoint("tcp://127.0.0.1:13338")
	if err == nil {
		t.Error("non-http scheme should be rejected")
	}
}

func TestIDAEndpointRejectInvalidURL(t *testing.T) {
	_, err := validateIDAEndpoint("not-a-valid-url")
	if err == nil {
		t.Error("invalid URL should be rejected")
	}
}

// RealMCPClient constructor

func TestNewRealMCPClient_RejectRemote(t *testing.T) {
	_, err := NewRealMCPClient("http://example.com:13338/sse", 5)
	if err == nil {
		t.Fatal("remote endpoint should be rejected")
	}
	if !strings.Contains(err.Error(), "must be localhost") {
		t.Errorf("error should mention localhost: %s", err.Error())
	}
}

func TestNewRealMCPClient_AllowLocalhost(t *testing.T) {
	c, err := NewRealMCPClient("http://127.0.0.1:13338/sse", 5)
	if err != nil {
		t.Fatalf("localhost endpoint should be allowed: %v", err)
	}
	if c == nil {
		t.Fatal("client should not be nil")
	}
}

// ida_status

func TestIDAStatus_Available(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		StatusFn: func(ctx context.Context) (*IDAMCPStatus, error) {
			return &IDAMCPStatus{Available: true, Endpoint: "mock://ida"}, nil
		},
	}))

	tool, err := NewIDAStatusTool()
	if err != nil {
		t.Fatalf("NewIDAStatusTool: %v", err)
	}
	out := invokeIDATool[IDAStatusInput, IDAStatusOutput](t, tool, IDAStatusInput{})
	if !out.Available {
		t.Error("status should be available")
	}
	if out.Endpoint == "" {
		t.Error("endpoint should not be empty")
	}
}

func TestIDAStatus_Unavailable(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		StatusFn: func(ctx context.Context) (*IDAMCPStatus, error) {
			return &IDAMCPStatus{Available: false, Error: "connection refused"}, nil
		},
	}))

	tool, err := NewIDAStatusTool()
	if err != nil {
		t.Fatalf("NewIDAStatusTool: %v", err)
	}
	out := invokeIDATool[IDAStatusInput, IDAStatusOutput](t, tool, IDAStatusInput{})
	if out.Available {
		t.Error("status should be unavailable")
	}
	if out.Error == "" {
		t.Error("should have error message")
	}
}

func TestIDAStatus_NoClient(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(nil))

	tool, err := NewIDAStatusTool()
	if err != nil {
		t.Fatalf("NewIDAStatusTool: %v", err)
	}
	out := invokeIDATool[IDAStatusInput, IDAStatusOutput](t, tool, IDAStatusInput{})
	if out.Available {
		t.Error("should be unavailable when client is nil")
	}
	if !strings.Contains(out.Error, "not configured") {
		t.Errorf("error should mention not configured: %q", out.Error)
	}
}

// Status probe — 不读取完整 SSE body 即返回

func TestRealMCPClientStatus_NoReadBody(t *testing.T) {
	// 用 httptest 模拟 SSE endpoint，持续写入数据。
	// Status 必须在收到响应头后立即关闭连接，不被阻塞。
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		flusher.Flush()
		for i := 0; i < 1000; i++ {
			w.Write([]byte("data: never stop\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	// 绕过 endpoint 校验构造 client，测试 Status 的 body 关闭逻辑。
	c := &RealMCPClient{
		endpoint: server.URL + "/sse",
		timeout:  5 * 1e9, // 5 seconds as nanoseconds
	}

	status, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Available {
		t.Errorf("Status should be available for test server: %s", status.Error)
	}
}

// ida_functions

func TestIDAFunctions_Normal(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		FunctionsFn: func(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error) {
			return &IDAMCPFunctionsResult{
				Functions: []string{"main", "check_flag", "validate"},
				Total:     3,
			}, nil
		},
	}))

	tool, err := NewIDAFunctionsTool()
	if err != nil {
		t.Fatalf("NewIDAFunctionsTool: %v", err)
	}
	out := invokeIDATool[IDAFunctionsInput, IDAFunctionsOutput](t, tool, IDAFunctionsInput{Limit: 10})
	if out.Error != "" {
		t.Errorf("unexpected error: %s", out.Error)
	}
	if out.Total != 3 {
		t.Errorf("total: got %d, want 3", out.Total)
	}
	if !strings.Contains(out.Functions, "main") {
		t.Errorf("functions should contain main: %s", out.Functions)
	}
	if out.Truncated {
		t.Error("should not be truncated")
	}
}

func TestIDAFunctions_Truncated(t *testing.T) {
	manyFuncs := make([]string, 20000)
	for i := range manyFuncs {
		manyFuncs[i] = "function_with_very_long_name_" + strings.Repeat("x", 200)
	}

	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		FunctionsFn: func(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error) {
			return &IDAMCPFunctionsResult{Functions: manyFuncs, Total: len(manyFuncs)}, nil
		},
	}))

	tool, err := NewIDAFunctionsTool()
	if err != nil {
		t.Fatalf("NewIDAFunctionsTool: %v", err)
	}
	out := invokeIDATool[IDAFunctionsInput, IDAFunctionsOutput](t, tool, IDAFunctionsInput{})
	if !out.Truncated {
		t.Error("should be truncated with large output")
	}
	if out.Error != "" {
		t.Errorf("unexpected error: %s", out.Error)
	}
}

func TestIDAFunctions_TransportPending(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		FunctionsFn: func(ctx context.Context, limit int) (*IDAMCPFunctionsResult, error) {
			return &IDAMCPFunctionsResult{
				Error: "ida_functions transport not fully implemented; IDA MCP SSE client pending",
			}, nil
		},
	}))

	tool, err := NewIDAFunctionsTool()
	if err != nil {
		t.Fatalf("NewIDAFunctionsTool: %v", err)
	}
	out := invokeIDATool[IDAFunctionsInput, IDAFunctionsOutput](t, tool, IDAFunctionsInput{})
	if out.Error == "" || !strings.Contains(out.Error, "transport not fully implemented") {
		t.Errorf("should return transport pending error: %q", out.Error)
	}
}

// ida_decompile

func TestIDADecompile_Normal(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		DecompileFn: func(ctx context.Context, target string) (*IDAMCPDecompileResult, error) {
			return &IDAMCPDecompileResult{
				Code: "int " + target + "(int a, int b) { return a + b; }",
			}, nil
		},
	}))

	tool, err := NewIDADecompileTool()
	if err != nil {
		t.Fatalf("NewIDADecompileTool: %v", err)
	}
	out := invokeIDATool[IDADecompileInput, IDADecompileOutput](t, tool, IDADecompileInput{Target: "add"})
	if out.Error != "" {
		t.Errorf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Code, "add") {
		t.Errorf("code should contain function name: %s", out.Code)
	}
	if out.Truncated {
		t.Error("should not be truncated")
	}
}

func TestIDADecompile_Truncated(t *testing.T) {
	largeCode := strings.Repeat("// decompiled line\n", 10000)

	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		DecompileFn: func(ctx context.Context, target string) (*IDAMCPDecompileResult, error) {
			return &IDAMCPDecompileResult{Code: largeCode}, nil
		},
	}))

	tool, err := NewIDADecompileTool()
	if err != nil {
		t.Fatalf("NewIDADecompileTool: %v", err)
	}
	out := invokeIDATool[IDADecompileInput, IDADecompileOutput](t, tool, IDADecompileInput{Target: "large_func"})
	if !out.Truncated {
		t.Error("should be truncated with large output")
	}
}

// ida_strings

func TestIDAStrings_Normal(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		StringsFn: func(ctx context.Context, limit int) (*IDAMCPStringsResult, error) {
			return &IDAMCPStringsResult{
				Strings: []string{"password", "flag{test}", "admin"},
				Total:   3,
			}, nil
		},
	}))

	tool, err := NewIDAStringsTool()
	if err != nil {
		t.Fatalf("NewIDAStringsTool: %v", err)
	}
	out := invokeIDATool[IDAStringsInput, IDAStringsOutput](t, tool, IDAStringsInput{})
	if out.Error != "" {
		t.Errorf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Strings, "flag{test}") {
		t.Errorf("strings should contain flag{test}: %s", out.Strings)
	}
}

// ida_xrefs

func TestIDAXrefs_Normal(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		XrefsFn: func(ctx context.Context, target string) (*IDAMCPXrefsResult, error) {
			return &IDAMCPXrefsResult{
				Xrefs: []string{"call " + target + " from main+0x42"},
			}, nil
		},
	}))

	tool, err := NewIDAXrefsTool()
	if err != nil {
		t.Fatalf("NewIDAXrefsTool: %v", err)
	}
	out := invokeIDATool[IDAXrefsInput, IDAXrefsOutput](t, tool, IDAXrefsInput{Target: "check_password"})
	if out.Error != "" {
		t.Errorf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Xrefs, "check_password") {
		t.Errorf("xrefs should contain target: %s", out.Xrefs)
	}
}

// DisabledMCPClient

func TestDisabledMCPClient(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(NewDisabledMCPClient("endpoint is invalid")))

	t.Run("status", func(t *testing.T) {
		tool, _ := NewIDAStatusTool()
		out := invokeIDATool[IDAStatusInput, IDAStatusOutput](t, tool, IDAStatusInput{})
		if out.Available {
			t.Error("should be unavailable with disabled client")
		}
		if !strings.Contains(out.Error, "endpoint is invalid") {
			t.Errorf("error should mention config issue: %q", out.Error)
		}
	})

	t.Run("decompile", func(t *testing.T) {
		tool, _ := NewIDADecompileTool()
		out := invokeIDATool[IDADecompileInput, IDADecompileOutput](t, tool, IDADecompileInput{Target: "test"})
		if out.Error == "" {
			t.Error("should return error with disabled client")
		}
	})
}

// Registry

func TestRegistryHasIDATools(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{}))

	r := NewRegistry()

	statusTool, err := NewIDAStatusTool()
	if err != nil {
		t.Fatalf("NewIDAStatusTool: %v", err)
	}
	r.Register("ida_status", statusTool)

	functionsTool, err := NewIDAFunctionsTool()
	if err != nil {
		t.Fatalf("NewIDAFunctionsTool: %v", err)
	}
	r.Register("ida_functions", functionsTool)

	decompileTool, err := NewIDADecompileTool()
	if err != nil {
		t.Fatalf("NewIDADecompileTool: %v", err)
	}
	r.Register("ida_decompile", decompileTool)

	stringsTool, err := NewIDAStringsTool()
	if err != nil {
		t.Fatalf("NewIDAStringsTool: %v", err)
	}
	r.Register("ida_strings", stringsTool)

	xrefsTool, err := NewIDAXrefsTool()
	if err != nil {
		t.Fatalf("NewIDAXrefsTool: %v", err)
	}
	r.Register("ida_xrefs", xrefsTool)

	expected := []string{"ida_status", "ida_functions", "ida_decompile", "ida_strings", "ida_xrefs"}
	for _, name := range expected {
		tool, err := r.Get(name)
		if err != nil {
			t.Errorf("tool %q not in registry: %v", name, err)
			continue
		}
		if tool == nil {
			t.Errorf("tool %q is nil", name)
		}
	}

	if len(r.All()) != 5 {
		t.Errorf("All() count: got %d, want 5", len(r.All()))
	}
}

// SSE transport

// mockMCPServer 模拟 MCP SSE 服务，返回 endpoint event 和指定的 JSON-RPC 响应。
type mockMCPServer struct {
	*httptest.Server
	postURL   string
	responses map[int]string // id → JSON response data
}

func newMockMCPServer(t *testing.T) *mockMCPServer {
	t.Helper()
	m := &mockMCPServer{
		responses: make(map[int]string),
	}
	m.Server = newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/sse") {
			m.handleSSE(w, r)
		} else if r.Method == http.MethodPost {
			m.handlePost(w, r)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	m.postURL = m.Server.URL + "/messages"
	return m
}

func (m *mockMCPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// 发送 endpoint event
	w.Write([]byte("event: endpoint\ndata: " + m.postURL + "\n\n"))
	flusher.Flush()

	// 逐个发送已注册的响应
	for id, data := range m.responses {
		w.Write([]byte(fmt.Sprintf("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":%s}\n\n", id, data)))
		flusher.Flush()
	}
}

func (m *mockMCPServer) handlePost(w http.ResponseWriter, r *http.Request) {
	// 接受 POST 但不返回 body，结果通过 SSE 返回
	w.WriteHeader(http.StatusAccepted)
}

// addResponse 向 mock server 添加一个预设响应。
func (m *mockMCPServer) addResponse(id int, resultJSON string) {
	m.responses[id] = resultJSON
}

func TestSSEParseEndpoints(t *testing.T) {
	server := newMockMCPServer(t)
	defer server.Close()
	server.addResponse(1, `"hello"`)

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	result, err := transport.callMCPToolText(context.Background(), "test_tool", map[string]any{})
	if err != nil {
		t.Fatalf("callMCPToolText: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestSSEMultiLineData(t *testing.T) {
	// 用 httptest 模拟发送多行 data 事件
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		// 多行 data
		w.Write([]byte("event: message\ndata: line1\ndata: line2\ndata: line3\n\n"))
		flusher.Flush()
	}))
	defer server.Close()

	transport := newSSETransport(server.URL+"/sse", 2*time.Second, "")
	req, _ := http.NewRequest("GET", server.URL+"/sse", nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE request: %v", err)
	}
	defer resp.Body.Close()

	events := transport.parseSSE(context.Background(), resp.Body)
	ev := <-events
	if ev.Event != "message" {
		t.Errorf("expected message event, got %q", ev.Event)
	}
	if !strings.Contains(ev.Data, "line1") || !strings.Contains(ev.Data, "line3") {
		t.Errorf("multi-line data: %q", ev.Data)
	}
}

func TestJSONRPCIDMatching(t *testing.T) {
	// 每次 callMCPTool 创建独立 SSE 连接，reqID 固定为 1。
	// 验证同一连接内多个响应能按 id=1 正确匹配。
	server := newMockMCPServer(t)
	defer server.Close()
	server.addResponse(1, `"expected_result"`)

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	r1, err := transport.callMCPToolText(context.Background(), "tool_a", map[string]any{})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if r1 != "expected_result" {
		t.Errorf("got %q, want 'expected_result'", r1)
	}

	// 第二次调用使用独立连接，同样返回 id=1 的结果
	r2, err := transport.callMCPToolText(context.Background(), "tool_b", map[string]any{})
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}
	if r2 != "expected_result" {
		t.Errorf("call 2: got %q, want 'expected_result'", r2)
	}
}

func TestJSONRPCErrorResponse(t *testing.T) {
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-1,\"message\":\"tool not found\"}}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	_, err := transport.callMCPToolText(context.Background(), "nonexistent", map[string]any{})
	if err == nil {
		t.Fatal("expected error response")
	}
	if !strings.Contains(err.Error(), "tool not found") {
		t.Errorf("error should contain tool not found: %s", err.Error())
	}
}

func TestSSEPostEndpointRejectRemote(t *testing.T) {
	// endpoint event 指向公网 URL 必须拒绝
	_, err := resolveAndValidatePostEndpoint("http://8.8.8.8:8080/messages", "http://127.0.0.1:13338/sse")
	if err == nil {
		t.Fatal("public IP endpoint should be rejected")
	}
	if !strings.Contains(err.Error(), "private IP") {
		t.Errorf("error should mention private IP: %s", err.Error())
	}
}

func TestSSEPostEndpointAllowRelative(t *testing.T) {
	// 相对路径应被解析为相对于 sse endpoint 的地址
	_, err := resolveAndValidatePostEndpoint("/messages?sessionId=abc", "http://127.0.0.1:13338/sse")
	if err != nil {
		t.Errorf("relative post endpoint should be allowed: %v", err)
	}
}

func TestSSETimeout(t *testing.T) {
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		time.Sleep(10 * time.Second) // 不发 message event，模拟超时
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 500*time.Millisecond, "")
	_, err := transport.callMCPToolText(context.Background(), "test", map[string]any{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// TestIDAFunctionsThroughSSE 通过 mock SSE server 调用 ida_functions。
func TestIDAFunctionsThroughSSE(t *testing.T) {
	server := newMockMCPServer(t)
	defer server.Close()
	server.addResponse(1, `["main","check_password","verify_flag"]`)

	// 构造指向 mock server 的 RealMCPClient
	c := &RealMCPClient{
		endpoint: server.URL + "/sse",
		timeout:  5 * time.Second,
	}
	defer restoreIDAClient(setTestIDAClient(c))

	result, err := c.Functions(context.Background(), 10)
	if err != nil {
		t.Fatalf("Functions: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Functions error: %s", result.Error)
	}
	if len(result.Functions) != 3 {
		t.Errorf("expected 3 functions, got %d", len(result.Functions))
	}
	if result.Functions[0] != "main" {
		t.Errorf("first function: got %q, want 'main'", result.Functions[0])
	}
}

// TestIDADecompileThroughSSE 通过 mock SSE server 调用 ida_decompile。
func TestIDADecompileThroughSSE(t *testing.T) {
	server := newMockMCPServer(t)
	defer server.Close()
	server.addResponse(1, `"int __cdecl main(int argc, const char **argv) {\n  if ( argc == 2 )\n    return check_password(argv[1]);\n  return 0;\n}"`)

	c := &RealMCPClient{
		endpoint: server.URL + "/sse",
		timeout:  5 * time.Second,
	}
	defer restoreIDAClient(setTestIDAClient(c))

	result, err := c.Decompile(context.Background(), "main")
	if err != nil {
		t.Fatalf("Decompile: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Decompile error: %s", result.Error)
	}
	if !strings.Contains(result.Code, "check_password") {
		t.Errorf("decompiled code should contain check_password: %s", result.Code)
	}
}

// TestRealMCPClient_StatusStillWorks 验证 Status 仍使用简单 HTTP probe。
func TestRealMCPClient_StatusStillWorks(t *testing.T) {
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &RealMCPClient{
		endpoint: server.URL + "/sse",
		timeout:  5 * time.Second,
	}
	status, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Available {
		t.Error("Status should be available")
	}
}

// TestExistingMockMCPClientStillWorks 验证 MockMCPClient 测试不回退。
func TestExistingMockMCPClientStillWorks(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{}))

	tool, _ := NewIDAStatusTool()
	out := invokeIDATool[IDAStatusInput, IDAStatusOutput](t, tool, IDAStatusInput{})
	if !out.Available {
		t.Error("mock status should be available")
	}

	funcTool, _ := NewIDAFunctionsTool()
	funcOut := invokeIDATool[IDAFunctionsInput, IDAFunctionsOutput](t, funcTool, IDAFunctionsInput{Limit: 5})
	if funcOut.Error != "" {
		t.Errorf("mock functions should not error: %s", funcOut.Error)
	}
	if !strings.Contains(funcOut.Functions, "main") {
		t.Errorf("mock functions should contain main: %s", funcOut.Functions)
	}
}

// helpers

func setTestIDAClient(c IDAMCPClient) IDAMCPClient {
	old := idaClient
	idaClient = c
	return old
}

func restoreIDAClient(old IDAMCPClient) {
	idaClient = old
}

// Host header override tests

// TestHostHeader_StatusProbe verifies Status probe request carries the overridden Host header.
func TestHostHeader_StatusProbe(t *testing.T) {
	var capturedHost string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHost = r.Host
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &RealMCPClient{
		endpoint:   server.URL + "/sse",
		timeout:    5 * time.Second,
		hostHeader: "127.0.0.1:13338",
	}

	status, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Available {
		t.Error("Status should be available")
	}
	if capturedHost != "127.0.0.1:13338" {
		t.Errorf("Host header: got %q, want '127.0.0.1:13338'", capturedHost)
	}
}

// TestHostHeader_StatusNoOverride verifies Status probe does NOT override Host when hostHeader is empty.
func TestHostHeader_StatusNoOverride(t *testing.T) {
	var capturedHost string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHost = r.Host
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &RealMCPClient{
		endpoint:   server.URL + "/sse",
		timeout:    5 * time.Second,
		hostHeader: "",
	}

	status, err := c.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Available {
		t.Error("Status should be available")
	}
	if capturedHost == "127.0.0.1:13338" {
		t.Errorf("Host header should NOT be overridden when hostHeader is empty, got %q", capturedHost)
	}
}

// TestIsPrivateIP tests the isPrivateIP helper function.
func TestIsPrivateIP(t *testing.T) {
	privateIPs := []string{
		"10.0.0.1",
		"10.255.255.255",
		"172.16.0.1",
		"172.26.192.1",
		"172.31.255.255",
		"192.168.0.1",
		"192.168.255.255",
	}
	for _, ipStr := range privateIPs {
		if !isPrivateIP(net.ParseIP(ipStr)) {
			t.Errorf("%q should be recognized as private IP", ipStr)
		}
	}

	publicIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"172.15.0.1",
		"172.32.0.1",
		"192.167.255.255",
		"192.169.0.1",
		"11.0.0.1",
	}
	for _, ipStr := range publicIPs {
		if isPrivateIP(net.ParseIP(ipStr)) {
			t.Errorf("%q should NOT be recognized as private IP", ipStr)
		}
	}
}
