package tool

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestParseFunctionsResult_RejectsServerHealth verifies parseFunctionsResult rejects server health/status JSON
func TestParseFunctionsResult_RejectsServerHealth(t *testing.T) {
	healthJSON := `{
		"status": "ok",
		"uptime_sec": 3420.253,
		"idb_path": "D:\\test.exe.i64",
		"module": "test.exe",
		"input_path": "D:\\test.exe",
		"imagebase": "0x140000000",
		"auto_analysis_ready": true,
		"hexrays_ready": true,
		"strings_cache_ready": true,
		"strings_cache_size": 381
	}`

	result, _ := parseFunctionsResult(healthJSON)
	if result.Error == "" {
		t.Fatal("parseFunctionsResult should return error for server health JSON")
	}
	if !strings.Contains(result.Error, "server status") {
		t.Errorf("error should mention 'server status': %q", result.Error)
	}
	if len(result.Functions) != 0 {
		t.Errorf("functions should be empty when rejected: %v", result.Functions)
	}
}

// TestParseFunctionsResult_RejectsToolsList verifies parseFunctionsResult rejects tools/list JSON
func TestParseFunctionsResult_RejectsToolsList(t *testing.T) {
	toolsListJSON := `{"tools": [{"name": "list_funcs"}, {"name": "server_health"}]}`
	result, _ := parseFunctionsResult(toolsListJSON)
	if result.Error == "" {
		t.Fatal("parseFunctionsResult should return error for tools/list JSON")
	}
	if !strings.Contains(result.Error, "tools/list") {
		t.Errorf("error should mention 'tools/list': %q", result.Error)
	}
}

// TestParseFunctionsResult_AcceptsArray verifies parseFunctionsResult accepts string array
func TestParseFunctionsResult_AcceptsArray(t *testing.T) {
	result, _ := parseFunctionsResult(`["main", "check_password", "verify_flag"]`)
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Total != 3 {
		t.Errorf("total: got %d, want 3", result.Total)
	}
	if result.Functions[0] != "main" {
		t.Errorf("first function: got %q, want 'main'", result.Functions[0])
	}
}

// TestParseFunctionsResult_AcceptsObjectWithFunctionsKey verifies parseFunctionsResult accepts object with functions key
func TestParseFunctionsResult_AcceptsObjectWithFunctionsKey(t *testing.T) {
	result, _ := parseFunctionsResult(`{"functions": ["main", "verify"], "total": 2}`)
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Total != 2 {
		t.Errorf("total: got %d, want 2", result.Total)
	}
}

// TestParseFunctionsResult_AcceptsObjectArray verifies parseFunctionsResult accepts function object array
func TestParseFunctionsResult_AcceptsObjectArray(t *testing.T) {
	objArray := `[{"name": "main", "address": "0x401000"}, {"name": "check", "ea": "0x402000"}]`
	result, _ := parseFunctionsResult(objArray)
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Total != 2 { // 2 "name addr" combined entries
		t.Errorf("total: got %d, want 2: %v", result.Total, result.Functions)
	}
}

// TestParseFunctionsResult_RejectsUnrelatedJSON verifies parseFunctionsResult rejects unrelated JSON object
func TestParseFunctionsResult_RejectsUnrelatedJSON(t *testing.T) {
	result, _ := parseFunctionsResult(`{"foo": "bar", "baz": 123}`)
	if result.Error == "" {
		t.Fatal("parseFunctionsResult should return error for unrelated JSON")
	}
}

// TestValidateFunctionsResult_RejectsHealth verifies validateFunctionsResult rejects health JSON
func TestValidateFunctionsResult_RejectsHealth(t *testing.T) {
	err := validateFunctionsResult(`{"status":"ok","uptime_sec":123,"idb_path":"test"}`)
	if err == nil {
		t.Fatal("validateFunctionsResult should reject server health")
	}
	if !strings.Contains(err.Error(), "health") {
		t.Errorf("error should mention 'health': %s", err.Error())
	}
}

// TestValidateFunctionsResult_AcceptsArray verifies validateFunctionsResult accepts function array
func TestValidateFunctionsResult_AcceptsArray(t *testing.T) {
	err := validateFunctionsResult(`["func1", "func2"]`)
	if err != nil {
		t.Errorf("validateFunctionsResult should accept function array: %v", err)
	}
}

// TestFunctionsFallback_RejectsHealthThenTriesListFuncs verifies fallback doesn't stop at server_health,
// continuing to try list_funcs which returns a real function list.
func TestFunctionsFallback_RejectsHealthThenTriesListFuncs(t *testing.T) {
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
			// server_health (via tools/list self-healing) → rejected by validator
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"status\":\"ok\",\"uptime_sec\":123,\"idb_path\":\"test\",\"module\":\"test.exe\"}}\n\n"))
		case 2:
			// tools/list returns list_funcs + server_health
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"server_health\"},{\"name\":\"func_list\"}]}}\n\n"))
		case 3:
			// server_health retry → rejected again
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"status\":\"ok\",\"uptime_sec\":123,\"idb_path\":\"test\",\"module\":\"test.exe\"}}\n\n"))
		case 4:
			// func_list + queries=50 → REAL function list
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"check_password\",\"verify\"]}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	text, err := transport.callMCPToolTextWithParamFallbackAndValidator(
		context.Background(),
		[]string{"bad_tool"}, // All original candidates fail
		map[string]any{"limit": 10},
		functionsParamCandidates,
		validateFunctionsResult,
	)
	if err != nil {
		t.Fatalf("fallback should succeed after rejecting health: %v", err)
	}
	if !strings.Contains(text, "main") {
		t.Errorf("result should contain 'main', got: %s", text)
	}
	if strings.Contains(text, "uptime_sec") {
		t.Errorf("result should NOT contain server health fields: %s", text)
	}
	if strings.Contains(text, "status") {
		t.Errorf("result should NOT contain 'status': %s", text)
	}
}

// TestFunctionsFallback_DoesNotTreatStatusAsSuccess verifies fallback doesn't stop at list_funcs
// that returns server health; continues to func_query which returns real functions.
func TestFunctionsFallback_DoesNotTreatStatusAsSuccess(t *testing.T) {
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
		case connCount == 1:
			// list_funcs → server health JSON (bad!)
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"status\":\"ok\",\"uptime_sec\":999,\"idb_path\":\"bad\",\"module\":\"bad.exe\",\"auto_analysis_ready\":true}}\n\n"))
		case connCount == 2:
			// func_query → REAL function list (good!)
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"init\",\"parse_input\",\"check_auth\"]}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	transport := newSSETransport(server.URL+"/sse", 5*time.Second, "")
	_, err := transport.callMCPToolTextWithParamFallbackAndValidator(
		context.Background(),
		[]string{"list_funcs"}, // list_funcs returns health JSON
		map[string]any{"limit": 10},
		functionsParamCandidates,
		validateFunctionsResult,
	)
	// Should fail because validator rejects list_funcs health, no more candidates
	if err == nil {
		t.Error("should fail because list_funcs returned health JSON and no more candidates")
	}
	if !strings.Contains(err.Error(), "health") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention rejection reason, got: %s", err.Error())
	}

	// Second scenario: list_funcs → health, func_query → real functions, should succeed
	var postURL2 string
	var connCount2 int
	server2 := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL2)))
		flusher.Flush()

		if r.Method == http.MethodGet {
			connCount2++
		}
		switch connCount2 {
		case 1:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"status\":\"ok\",\"uptime_sec\":999,\"idb_path\":\"bad\",\"module\":\"bad.exe\",\"auto_analysis_ready\":true}}\n\n"))
		case 2:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"init\",\"parse_input\",\"check_auth\"]}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server2.Close()
	postURL2 = server2.URL + "/msg"

	transport2 := newSSETransport(server2.URL+"/sse", 5*time.Second, "")
	text2, err := transport2.callMCPToolTextWithParamFallbackAndValidator(
		context.Background(),
		[]string{"list_funcs", "func_query"}, // list_funcs → health, func_query → real
		map[string]any{"limit": 10},
		functionsParamCandidates,
		validateFunctionsResult,
	)
	if err != nil {
		t.Fatalf("fallback should succeed on func_query: %v", err)
	}
	if !strings.Contains(text2, "init") {
		t.Errorf("result should contain 'init', got: %s", text2)
	}
	if strings.Contains(text2, "uptime_sec") {
		t.Errorf("result should NOT contain server health fields: %s", text2)
	}
}

// TestRegistryLayer_FunctionsRejectsStatusResult verifies the full registry → handler → tool layer
// rejects server health/status JSON. Direct Eino tool invocation.
func TestRegistryLayer_FunctionsRejectsStatusResult(t *testing.T) {
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
		case connCount >= 1 && connCount <= 7:
			// 7 candidates fail (tool not found) → triggers self-healing
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"Method not found\"}}\n\n"))
		case connCount == 8:
			// tools/list returns server_health + func_list
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"server_health\"},{\"name\":\"func_list\"}]}}\n\n"))
		case connCount == 9:
			// func_list + queries="" → server health JSON (filtered out by irrelevantForFunctions)
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"status\":\"ok\",\"uptime_sec\":999,\"idb_path\":\"bad\",\"module\":\"bad.exe\",\"auto_analysis_ready\":true}}\n\n"))
		case connCount == 10:
			// func_list + queries="*" → REAL function list (self-healed + validator passes)
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"check_password\",\"verify_flag\",\"auth_b64d\"]}\n\n"))
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

	out := invokeIDATool[IDAFunctionsInput, IDAFunctionsOutput](t, tool, IDAFunctionsInput{Limit: 50})

	// strict whitelist: func_list NOT in exact whitelist -> rejected -> all exhausted
	if out.Error == "" {
		t.Fatal("ida_functions should fail when self-healing finds no whitelist tools")
	}
	if !strings.Contains(out.Error, "all candidates exhausted") {
		t.Errorf("error should mention 'all candidates exhausted': %s", out.Error)
	}
	// Error should NOT mention server_health or func_list (both filtered)
	if strings.Contains(out.Error, "server_health") {
		t.Errorf("error should NOT mention 'server_health': %s", out.Error)
	}
	if strings.Contains(out.Error, "func_list") {
		t.Errorf("error should NOT mention 'func_list': %s", out.Error)
	}
}

// TestBuildFunctionsArgSets_NoIntQueries verifies list_funcs/lookup_funcs args
// never contain int values for queries — they must be string or empty.
func TestBuildFunctionsArgSets_NoIntQueries(t *testing.T) {
	for _, group := range buildFunctionsArgSets(200) {
		for _, toolName := range group.toolNames {
			for _, argSet := range group.argSets {
				if val, ok := argSet["queries"]; ok {
					switch v := val.(type) {
					case int:
						t.Errorf("tool %s: queries should not be int, got %d", toolName, v)
					case float64:
						t.Errorf("tool %s: queries should not be float64, got %v", toolName, v)
					}
				}
				if val, ok := argSet["query"]; ok {
					switch v := val.(type) {
					case int:
						t.Errorf("tool %s: query should not be int, got %d", toolName, v)
					case float64:
						t.Errorf("tool %s: query should not be float64, got %v", toolName, v)
					}
				}
				// limit/count/max_results should be int for int-based tools
				for _, intKey := range []string{"limit", "count", "max_results"} {
					if val, ok := argSet[intKey]; ok {
						if _, ok := val.(int); !ok {
							// Only error if tool is in the int-based group
							// list_funcs/lookup_funcs/func_query should not have limit
							if toolName == "list_funcs" || toolName == "lookup_funcs" || toolName == "func_query" {
								t.Errorf("tool %s: should not have int key %s=%v", toolName, intKey, val)
							}
						}
					}
				}
			}
		}
	}
}

// TestBuildFunctionsArgSets_ListFuncsUsesStringQueries verifies list_funcs
// arg sets use string queries, and queries="" succeeds.
func TestBuildFunctionsArgSets_ListFuncsUsesStringQueries(t *testing.T) {
	var foundEmpty, foundStar bool
	for _, group := range buildFunctionsArgSets(200) {
		for _, name := range group.toolNames {
			if name != "list_funcs" {
				continue
			}
			for _, argSet := range group.argSets {
				if v, ok := argSet["queries"]; ok {
					if s, ok := v.(string); ok {
						if s == "" {
							foundEmpty = true
						}
						if s == "*" {
							foundStar = true
						}
					}
				}
			}
		}
	}
	if !foundEmpty {
		t.Error("list_funcs should have arg set with queries=\"\"")
	}
	if !foundStar {
		t.Error("list_funcs should have arg set with queries=\"*\"")
	}
}

// TestExportFuncsNotInCandidates verifies export_funcs is NOT in the
// function list candidates (requires addrs parameter).
func TestExportFuncsNotInCandidates(t *testing.T) {
	for _, group := range buildFunctionsArgSets(0) {
		for _, name := range group.toolNames {
			if name == "export_funcs" {
				t.Fatal("export_funcs should not be in functions candidates (needs addrs)")
			}
		}
	}
	// Also check the global candidate list
	for _, name := range functionsToolCandidates {
		if name == "export_funcs" {
			t.Fatal("functionsToolCandidates should NOT contain export_funcs")
		}
	}
}

// TestFunctions_ListFuncsSuccessWithEmptyQueries verifies the full SSE flow:
// list_funcs + queries="" → returns function list.
func TestFunctions_ListFuncsSuccessWithEmptyQueries(t *testing.T) {
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		// list_funcs + queries="" → real function list
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"check\",\"verify\"]}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	result, err := c.Functions(context.Background(), 200)
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

// TestFunctions_QueriesNotInt verifies the transport never sends int queries
// to list_funcs. We intercept the SSE call and verify the POST body.
func TestFunctions_QueriesNotInt(t *testing.T) {
	var postURL string
	var lastPOSTBody string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()

		if r.Method == http.MethodPost {
			buf := make([]byte, 2048)
			n, _ := r.Body.Read(buf)
			lastPOSTBody = string(buf[:n])
		}

		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"check\",\"verify\"]}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	_, err := c.Functions(context.Background(), 200)
	if err != nil {
		t.Fatalf("Functions: %v", err)
	}

	// Verify last POST body doesn't contain "queries":200 (int)
	if strings.Contains(lastPOSTBody, `"queries":200`) {
		t.Errorf("POST body should NOT contain int queries=200: %s", lastPOSTBody)
	}
	if strings.Contains(lastPOSTBody, `"queries":200.0`) {
		t.Errorf("POST body should NOT contain float queries=200.0: %s", lastPOSTBody)
	}
}

// TestIsFunctionListTool_Accept verifies whitelist tools are accepted.
func TestIsFunctionListTool_Accept(t *testing.T) {
	accepted := []string{
		"list_funcs", "lookup_funcs", "func_query",
		"list_functions", "ida_functions", "functions", "get_functions",
	}
	for _, name := range accepted {
		if !isFunctionListTool(name) {
			t.Errorf("isFunctionListTool(%q) should be true", name)
		}
	}
}

// TestIsFunctionListTool_Reject verifies blacklist tools are rejected.
func TestIsFunctionListTool_Reject(t *testing.T) {
	rejected := []string{
		"server_health", "server_warmup",
		"analyze_component", "diff_before_after", "trace_data_flow",
		"int_convert", "idb_save", "imports", "imports_query",
		"list_globals", "entity_query",
		"decompile", "decompile_func", "decompile_function",
		"disasm", "xrefs_to", "xrefs_from", "find_regex",
		"config", "get_config", "ida_status", "ida_health",
		"health", "ping", "echo", "get_bytes", "get_original_bytes",
	}
	for _, name := range rejected {
		if isFunctionListTool(name) {
			t.Errorf("isFunctionListTool(%q) should be false", name)
		}
	}
}

// TestIsFunctionListTool_RejectNonFunctionUnknown verifies unknown tools without func keyword are rejected.
func TestIsFunctionListTool_RejectNonFunctionUnknown(t *testing.T) {
	unknown := []string{"random_tool", "something_else", "unknown_command"}
	for _, name := range unknown {
		if isFunctionListTool(name) {
			t.Errorf("isFunctionListTool(%q) should be false (no func keyword)", name)
		}
	}
}

// TestFunctionsFallback_RejectsIrrelevantTools verifies the self-healing path
// does NOT try analyze_component, diff_before_after, trace_data_flow.
func TestFunctionsFallback_RejectsIrrelevantTools(t *testing.T) {
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
		case 1, 2, 3, 4, 5, 6, 7:
			// 7 candidates fail
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"Method not found\"}}\n\n"))
		case 8:
			// tools/list returns: analyze_component, diff_before_after, trace_data_flow, list_funcs
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"analyze_component\"},{\"name\":\"diff_before_after\"},{\"name\":\"trace_data_flow\"},{\"name\":\"list_funcs\"}]}}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	result, _ := c.Functions(context.Background(), 200)

	// Should fail because all accepted tools are in candidates, no new ones
	if result.Error == "" {
		t.Error("should fail because all accepted tools are already tried candidates")
	}
	// Error should NOT mention irrelevant tools
	if strings.Contains(result.Error, "analyze_component") {
		t.Errorf("error should NOT mention 'analyze_component': %s", result.Error)
	}
	if strings.Contains(result.Error, "diff_before_after") {
		t.Errorf("error should NOT mention 'diff_before_after': %s", result.Error)
	}
	if strings.Contains(result.Error, "trace_data_flow") {
		t.Errorf("error should NOT mention 'trace_data_flow': %s", result.Error)
	}
}

// TestFunctionsFallback_AcceptsFuncTools verifies tools/list returning
// func_query (not in original candidates) enters self-healing and succeeds.
func TestFunctionsFallback_AcceptsFuncTools(t *testing.T) {
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
		case connCount >= 1 && connCount <= 7:
			// 7 candidates fail
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"Method not found\"}}\n\n"))
		case connCount == 8:
			// tools/list returns list_funcs (already in candidates) + func_query (new)
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"list_funcs\"},{\"name\":\"func_list\"}]}}\n\n"))
		case connCount == 9:
			// func_list + queries="" → real function list
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"verify\",\"check\"]}\n\n"))
		default:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[]}\n\n"))
		}
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	result, err := c.Functions(context.Background(), 200)
	if err != nil {
		t.Fatalf("Functions: %v", err)
	}
	// func_list NOT in exact whitelist -> rejected -> all exhausted
	if result.Error == "" {
		t.Fatal("should fail: func_list rejected by strict whitelist")
	}
	if !strings.Contains(result.Error, "all candidates exhausted") {
		t.Errorf("error should mention 'all candidates exhausted': %s", result.Error)
	}
}

// TestRegistryLayer_DoesNotTryIrrelevantTools verifies the full Eino invoke
// path never tries analyze_component, diff_before_after, or trace_data_flow.
func TestRegistryLayer_DoesNotTryIrrelevantTools(t *testing.T) {
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
		case connCount >= 1 && connCount <= 7:
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601,\"message\":\"Method not found\"}}\n\n"))
		case connCount == 8:
			// tools/list returns irrelevant tools + one valid func tool
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[{\"name\":\"analyze_component\"},{\"name\":\"diff_before_after\"},{\"name\":\"trace_data_flow\"},{\"name\":\"server_health\"},{\"name\":\"my_func_list\"}]}}\n\n"))
		case connCount == 9:
			// my_func_list + queries="" → real functions
			w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":[\"main\",\"check_password\",\"verify_flag\",\"auth_b64d\"]}\n\n"))
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

	out := invokeIDATool[IDAFunctionsInput, IDAFunctionsOutput](t, tool, IDAFunctionsInput{Limit: 50})

	// strict whitelist: my_func_list NOT in exact whitelist -> rejected -> all exhausted
	if out.Error == "" {
		t.Fatal("ida_functions should fail: my_func_list rejected by strict whitelist")
	}
	if !strings.Contains(out.Error, "all candidates exhausted") {
		t.Errorf("error should mention 'all candidates exhausted': %s", out.Error)
	}
	// Error should NOT mention irrelevant tools
	for _, bad := range []string{"analyze_component", "diff_before_after", "trace_data_flow", "server_health", "my_func_list"} {
		if strings.Contains(out.Error, bad) {
			t.Errorf("error should NOT contain %q: %s", bad, out.Error)
		}
	}
}

// TestParseFunctionsResult_ZeromcpListFuncsDataArray verifies the zeromcp
// list_funcs response format: [{ "data": [{"addr":"...","name":"...","size":"..."}] }]
func TestParseFunctionsResult_ZeromcpListFuncsDataArray(t *testing.T) {
	listFuncsJSON := `[
		{
			"data": [
				{"addr": "0x140001000", "name": "_dynamic_initializer", "size": "0xc"},
				{"addr": "0x14000100c", "name": "main", "size": "0x50"}
			]
		}
	]`
	result, _ := parseFunctionsResult(listFuncsJSON)
	if result.Error != "" {
		t.Fatalf("should parse list_funcs data format: %s", result.Error)
	}
	if result.Total < 2 {
		t.Errorf("expected at least 2 functions, got %d: %v", result.Total, result.Functions)
	}
	if !strings.Contains(result.Functions[0], "0x140001000") {
		t.Errorf("first function should contain addr 0x140001000: %s", result.Functions[0])
	}
	if !strings.Contains(result.Functions[0], "_dynamic_initializer") {
		t.Errorf("first function should contain name: %s", result.Functions[0])
	}
}

// TestParseFunctionsResult_ZeromcpLookupFuncsFnObject verifies the zeromcp
// lookup_funcs response format: [{"query":"*","fn":{"addr":"...","name":"...","size":"..."},"error":null}]
func TestParseFunctionsResult_ZeromcpLookupFuncsFnObject(t *testing.T) {
	lookupFuncsJSON := `[
		{
			"query": "*",
			"fn": {
				"addr": "0x140001000",
				"name": "_dynamic_initializer",
				"size": "0xc"
			},
			"error": null
		},
		{
			"query": "*",
			"fn": {
				"addr": "0x140001050",
				"name": "check_password",
				"size": "0x30"
			},
			"error": null
		}
	]`
	result, _ := parseFunctionsResult(lookupFuncsJSON)
	if result.Error != "" {
		t.Fatalf("should parse lookup_funcs fn format: %s", result.Error)
	}
	if result.Total < 2 {
		t.Errorf("expected at least 2 functions, got %d: %v", result.Total, result.Functions)
	}
}

// TestParseFunctionsResult_ZeromcpFuncQueryDataArray verifies the zeromcp
// func_query response format: [{"data":[{"addr":"...","name":"...","size":"...","has_type":false}]}]
func TestParseFunctionsResult_ZeromcpFuncQueryDataArray(t *testing.T) {
	funcQueryJSON := `[
		{
			"data": [
				{"addr": "0x140001000", "name": "init_routine", "size": "0xc", "has_type": false},
				{"addr": "0x14000100c", "name": "parse_input", "size": "0x80", "has_type": true}
			]
		}
	]`
	result, _ := parseFunctionsResult(funcQueryJSON)
	if result.Error != "" {
		t.Fatalf("should parse func_query data format: %s", result.Error)
	}
	if result.Total < 2 {
		t.Errorf("expected at least 2 functions, got %d: %v", result.Total, result.Functions)
	}
}

// TestValidateFunctionsResult_AcceptsZeromcpNestedData verifies validateFunctionsResult
// accepts zeromcp nested data format (list_funcs with data field).
func TestValidateFunctionsResult_AcceptsZeromcpNestedData(t *testing.T) {
	text := `[{"data":[{"addr":"0x140001000","name":"main","size":"0x50"}]}]`
	err := validateFunctionsResult(text)
	if err != nil {
		t.Errorf("validator should accept zeromcp data format: %v", err)
	}
}

// TestValidateFunctionsResult_AcceptsZeromcpFnObject verifies validateFunctionsResult
// accepts zeromcp fn object format (lookup_funcs with fn field).
func TestValidateFunctionsResult_AcceptsZeromcpFnObject(t *testing.T) {
	text := `[{"query":"*","fn":{"addr":"0x140001000","name":"main","size":"0x50"},"error":null}]`
	err := validateFunctionsResult(text)
	if err != nil {
		t.Errorf("validator should accept zeromcp fn format: %v", err)
	}
}

// TestValidateFunctionsResult_RejectsLookupFuncsWithError verifies validateFunctionsResult
// rejects lookup_funcs response with non-null error field.
func TestValidateFunctionsResult_RejectsLookupFuncsWithError(t *testing.T) {
	text := `[{"query":"*","fn":null,"error":"some error occurred"}]`
	err := validateFunctionsResult(text)
	if err == nil {
		t.Error("validator should reject lookup_funcs response with error field")
	}
}

// TestFunctionListFilterRejectsNonListTools verifies isExactFunctionListTool
// rejects func_profile, export_funcs, define_func, analyze_function.
func TestFunctionListFilterRejectsNonListTools(t *testing.T) {
	rejected := []string{
		"func_profile", "export_funcs", "define_func", "analyze_function",
		"analyze_component", "diff_before_after", "trace_data_flow",
		"server_health", "server_warmup", "decompile", "disasm", "xrefs_to",
	}
	for _, name := range rejected {
		if isExactFunctionListTool(strings.ToLower(name)) {
			t.Errorf("isExactFunctionListTool(%q) should be false", name)
		}
	}
	// These should still be accepted
	accepted := []string{
		"list_funcs", "lookup_funcs", "func_query",
		"list_functions", "ida_functions", "functions", "get_functions",
	}
	for _, name := range accepted {
		if !isExactFunctionListTool(strings.ToLower(name)) {
			t.Errorf("isExactFunctionListTool(%q) should be true", name)
		}
	}
}

// TestIDADisasm_MockSuccess verifies ida_disasm tool with mock Disasm succeeds.
func TestIDADisasm_MockSuccess(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		DisasmFn: func(ctx context.Context, address string, end string, count int) (*IDAMCPDisasmResult, error) {
			return &IDAMCPDisasmResult{
				Instructions: address + ": mov eax, 0\n" + address + "+4: cmp eax, ebx\n" + address + "+8: jne 0x401050",
			}, nil
		},
	}))

	tool, err := NewIDADisasmTool()
	if err != nil {
		t.Fatalf("NewIDADisasmTool: %v", err)
	}
	out := invokeIDATool[IDADisasmInput, IDADisasmOutput](t, tool, IDADisasmInput{Address: "0x401000", Count: 10})
	if out.Error != "" {
		t.Fatalf("ida_disasm should succeed: %s", out.Error)
	}
	if !strings.Contains(out.Instructions, "mov eax") {
		t.Errorf("instructions should contain 'mov eax': %s", out.Instructions)
	}
	if !strings.Contains(out.Instructions, "jne") {
		t.Errorf("instructions should contain 'jne': %s", out.Instructions)
	}
}

// TestIDADisasm_Truncated verifies ida_disasm handles large output truncation.
func TestIDADisasm_Truncated(t *testing.T) {
	largeAsm := strings.Repeat("0x401000: nop\n", 20000)

	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{
		DisasmFn: func(ctx context.Context, address string, end string, count int) (*IDAMCPDisasmResult, error) {
			return &IDAMCPDisasmResult{Instructions: largeAsm}, nil
		},
	}))

	tool, err := NewIDADisasmTool()
	if err != nil {
		t.Fatalf("NewIDADisasmTool: %v", err)
	}
	out := invokeIDATool[IDADisasmInput, IDADisasmOutput](t, tool, IDADisasmInput{Address: "0x401000"})
	if !out.Truncated {
		t.Error("should be truncated with large output")
	}
	if out.Error != "" {
		t.Errorf("unexpected error: %s", out.Error)
	}
}

// TestIDADisasm_NoClient verifies ida_disasm returns error when no client configured.
func TestIDADisasm_NoClient(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(nil))

	tool, err := NewIDADisasmTool()
	if err != nil {
		t.Fatalf("NewIDADisasmTool: %v", err)
	}
	out := invokeIDATool[IDADisasmInput, IDADisasmOutput](t, tool, IDADisasmInput{Address: "0x401000"})
	if out.Error == "" {
		t.Error("should return error when client is nil")
	}
	if !strings.Contains(out.Error, "not configured") {
		t.Errorf("error should mention 'not configured': %q", out.Error)
	}
}

// TestIDADisasmThroughSSE verifies ida_disasm through mock SSE server.
func TestIDADisasmThroughSSE(t *testing.T) {
	var postURL string
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		w.Write([]byte(fmt.Sprintf("event: endpoint\ndata: %s\n\n", postURL)))
		flusher.Flush()
		w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"0x401000: mov eax, ebx\\n0x401004: cmp eax, 0\\n0x401008: jne 0x401020\"}\n\n"))
		flusher.Flush()
	}))
	defer server.Close()
	postURL = server.URL + "/msg"

	c := &RealMCPClient{endpoint: server.URL + "/sse", timeout: 5 * time.Second}
	result, err := c.Disasm(context.Background(), "0x401000", "", 10)
	if err != nil {
		t.Fatalf("Disasm: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("Disasm error: %s", result.Error)
	}
	if !strings.Contains(result.Instructions, "mov eax") {
		t.Errorf("instructions should contain 'mov eax': %s", result.Instructions)
	}
}

// TestRegistryHasIDADisasm verifies ida_disasm is registerable in registry.
func TestRegistryHasIDADisasm(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{}))

	r := NewRegistry()
	tool, err := NewIDADisasmTool()
	if err != nil {
		t.Fatalf("NewIDADisasmTool: %v", err)
	}
	r.Register("ida_disasm", tool)
	got, err := r.Get("ida_disasm")
	if err != nil {
		t.Fatalf("Get ida_disasm: %v", err)
	}
	if got == nil {
		t.Fatal("ida_disasm tool is nil")
	}
}

// TestIDADisasmToolNameStable verifies ida_disasm tool name is stable.
func TestIDADisasmToolNameStable(t *testing.T) {
	defer restoreIDAClient(setTestIDAClient(&MockMCPClient{}))

	tool, err := NewIDADisasmTool()
	if err != nil {
		t.Fatalf("NewIDADisasmTool: %v", err)
	}
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("tool.Info: %v", err)
	}
	if info.Name != "ida_disasm" {
		t.Errorf("tool name: got %q, want 'ida_disasm'", info.Name)
	}
}

// TestDisasmCandidates_PreferDisasm verifies disasm candidates prefer "disasm" first.
func TestDisasmCandidates_PreferDisasm(t *testing.T) {
	if disasmToolCandidates[0] != "disasm" {
		t.Errorf("first disasm candidate should be 'disasm', got %q", disasmToolCandidates[0])
	}
	if disasmToolCandidates[1] != "insn_query" {
		t.Errorf("second disasm candidate should be 'insn_query', got %q", disasmToolCandidates[1])
	}
}

// TestDisasmParamCandidates verifies disasm param candidates.
func TestDisasmParamCandidates(t *testing.T) {
	expected := []string{"address", "start", "addr"}
	for i, exp := range expected {
		if disasmParamCandidates[i] != exp {
			t.Errorf("disasmParamCandidates[%d]: got %q, want %q", i, disasmParamCandidates[i], exp)
		}
	}
}
