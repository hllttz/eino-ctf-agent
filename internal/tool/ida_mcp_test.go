package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	if defaultIDAEndpoint != "http://127.0.0.1:13337/sse" {
		t.Errorf("default endpoint: got %q, want http://127.0.0.1:13337/sse", defaultIDAEndpoint)
	}
}

func TestIDAEndpointAllowLocalhost(t *testing.T) {
	allowed := []string{
		"http://127.0.0.1:13337/sse",
		"http://localhost:13337/sse",
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
		"http://192.168.1.1:13337/sse",
		"http://10.0.0.1:13337/sse",
		"http://example.com:13337/sse",
		"http://0.0.0.0:13337/sse",
		"https://remote.example.com/mcp",
		"http://[::1]:13337/sse",
		"http://8.8.8.8:13337/sse",
	}
	for _, ep := range rejected {
		_, err := validateIDAEndpoint(ep)
		if err == nil {
			t.Errorf("endpoint %q should be rejected", ep)
		}
	}
}

func TestIDAEndpointRejectEmptyHost(t *testing.T) {
	_, err := validateIDAEndpoint("http://:13337/sse")
	if err == nil {
		t.Error("endpoint with empty host should be rejected")
	}
}

func TestIDAEndpointRejectInvalidScheme(t *testing.T) {
	_, err := validateIDAEndpoint("tcp://127.0.0.1:13337")
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
	_, err := NewRealMCPClient("http://example.com:13337/sse", 5)
	if err == nil {
		t.Fatal("remote endpoint should be rejected")
	}
	if !strings.Contains(err.Error(), "must be localhost") {
		t.Errorf("error should mention localhost: %s", err.Error())
	}
}

func TestNewRealMCPClient_AllowLocalhost(t *testing.T) {
	c, err := NewRealMCPClient("http://127.0.0.1:13337/sse", 5)
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// helpers

func setTestIDAClient(c IDAMCPClient) IDAMCPClient {
	old := idaClient
	idaClient = c
	return old
}

func restoreIDAClient(old IDAMCPClient) {
	idaClient = old
}
