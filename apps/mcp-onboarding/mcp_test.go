package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newSimpleMCPHandler() *MCPHandler {
	githubClient := NewGitHubClient("test-client-id", "test-client-secret")
	return newTestMCPHandler(githubClient)
}

func TestMCPHandler_Initialize(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "testuser", ID: 123}

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
	}

	resp := h.processRequest(req, user)
	if resp.Error != nil {
		t.Fatalf("expected no error, got: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(InitializeResult)
	if !ok {
		t.Fatal("expected InitializeResult type")
	}
	if result.ServerInfo.Name != "mcp-onboarding" {
		t.Errorf("expected server name 'mcp-onboarding', got %q", result.ServerInfo.Name)
	}
	if result.ServerInfo.Version != version {
		t.Errorf("expected server version %q, got %q", version, result.ServerInfo.Version)
	}
	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version '2024-11-05', got %q", result.ProtocolVersion)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability to be set")
	}
}

func TestMCPHandler_ListTools(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "testuser", ID: 123}

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/list",
	}

	resp := h.processRequest(req, user)
	if resp.Error != nil {
		t.Fatalf("expected no error, got: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(ListToolsResult)
	if !ok {
		t.Fatal("expected ListToolsResult type")
	}
	if len(result.Tools) == 0 {
		t.Error("expected at least one tool")
	}

	// Verify core tools exist
	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}
	requiredTools := []string{"hello_world", "greet", "whoami", "echo", "get_time"}
	for _, name := range requiredTools {
		if !toolNames[name] {
			t.Errorf("expected tool %q to be listed", name)
		}
	}
}

func TestMCPHandler_Ping(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "testuser", ID: 123}

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "ping",
	}

	resp := h.processRequest(req, user)
	if resp.Error != nil {
		t.Fatalf("expected no error, got: %s", resp.Error.Message)
	}
}

func TestMCPHandler_MethodNotFound(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "testuser", ID: 123}

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "nonexistent/method",
	}

	resp := h.processRequest(req, user)
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestMCPHandler_CallTool_HelloWorld(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "octocat", ID: 42}

	resp := callTool(h, "hello_world", nil, user)
	if resp.Error != nil {
		t.Fatalf("expected no error, got: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(CallToolResult)
	if !ok {
		t.Fatal("expected CallToolResult type")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content item")
	}
	if result.IsError {
		t.Error("expected IsError to be false")
	}
}

func TestMCPHandler_CallTool_Greet(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "testuser", ID: 123}

	resp := callTool(h, "greet", map[string]interface{}{"name": "World"}, user)
	if resp.Error != nil {
		t.Fatalf("expected no error, got: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(CallToolResult)
	if !ok {
		t.Fatal("expected CallToolResult type")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestMCPHandler_CallTool_Echo(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "testuser", ID: 123}

	resp := callTool(h, "echo", map[string]interface{}{"message": "test message"}, user)
	if resp.Error != nil {
		t.Fatalf("expected no error, got: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(CallToolResult)
	if !ok {
		t.Fatal("expected CallToolResult type")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestMCPHandler_CallTool_GetTime(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "testuser", ID: 123}

	formats := []string{"iso", "unix", "human", ""}
	for _, format := range formats {
		args := map[string]interface{}{}
		if format != "" {
			args["format"] = format
		}

		resp := callTool(h, "get_time", args, user)
		if resp.Error != nil {
			t.Fatalf("expected no error for format %q, got: %s", format, resp.Error.Message)
		}
	}
}

func TestMCPHandler_CallTool_UnknownTool(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "testuser", ID: 123}

	resp := callTool(h, "nonexistent_tool", nil, user)
	if resp.Error != nil {
		// Tool not found might return error in result content, not in JSONRPC error
		return
	}
	result, ok := resp.Result.(CallToolResult)
	if ok && result.IsError {
		// Expected - unknown tool returns error in content
		return
	}
	t.Error("expected error for unknown tool")
}

func TestMCPHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	h := newSimpleMCPHandler()

	req := httptest.NewRequest("DELETE", "/mcp", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestMCPHandler_HandleJSONRPC_ParseError(t *testing.T) {
	h := newSimpleMCPHandler()

	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()
	h.handleJSONRPC(w, req)

	var resp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected parse error code -32700, got %d", resp.Error.Code)
	}
}

func TestMCPHandler_CallTool_Whoami(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "octocat", ID: 42, GitHubAccessToken: "test-token"}

	resp := callTool(h, "whoami", nil, user)
	if resp.Error != nil {
		t.Fatalf("expected no error, got: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(CallToolResult)
	if !ok {
		t.Fatal("expected CallToolResult type")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
}

func TestLoggingMiddleware_PreservesFlusher(t *testing.T) {
	var sawFlusher bool
	handler := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, sawFlusher = w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !sawFlusher {
		t.Fatal("loggingMiddleware stripped http.Flusher from ResponseWriter")
	}
}

func TestHandleRoot_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	handleRoot(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST / status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
