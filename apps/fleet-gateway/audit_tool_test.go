package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// auditPost sends body through the gateway and returns the audit entry and the
// body upstream received.
func auditPost(t *testing.T, body string) (map[string]any, []byte, int) {
	t.Helper()
	var logBuf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(old) })

	te := newTestEnv(t)
	var (
		mu  sync.Mutex
		got []byte
	)
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = b
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	})
	gwToken := te.obtainAccessTokenForResource(te.resource)

	req := httptest.NewRequest(http.MethodPost, "/mcp/etterlevelse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+gwToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)

	var audit map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		var e map[string]any
		if json.Unmarshal([]byte(line), &e) == nil && e["msg"] == "audit mcp" {
			audit = e
		}
	}
	if audit == nil {
		t.Fatalf("no audit entry in logs:\n%s", logBuf.String())
	}
	if strings.Contains(logBuf.String(), "top-secret-arg") {
		t.Fatalf("tool arguments leaked into logs:\n%s", logBuf.String())
	}
	mu.Lock()
	defer mu.Unlock()
	return audit, got, rec.Code
}

func TestAuditMCP_LogsToolName_ForwardsBodyUnchanged(t *testing.T) {
	// Odd whitespace and key order must survive: upstream gets the exact bytes.
	body := "{ \"jsonrpc\":\"2.0\",\"id\":7,\n \"params\":{\"arguments\":{\"q\":\"top-secret-arg\"},\"name\":\"search_docs\"},\"method\":\"tools/call\"}  "
	audit, got, code := auditPost(t, body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if audit["method"] != "tools/call" || audit["tool"] != "search_docs" {
		t.Fatalf("expected method=tools/call tool=search_docs, got %v / %v", audit["method"], audit["tool"])
	}
	if string(got) != body {
		t.Fatalf("upstream body changed:\n got %q\nwant %q", got, body)
	}
}

func TestAuditMCP_ToolName_LogInjection(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"evil\n{\"msg\":\"audit mcp\",\"navident\":\"X999999\"}\r","arguments":{"a":"top-secret-arg"}}}`
	audit, got, _ := auditPost(t, body)
	tool, _ := audit["tool"].(string)
	if strings.ContainsAny(tool, "\n\r") {
		t.Fatalf("tool contains line breaks: %q", tool)
	}
	if !strings.HasPrefix(tool, "evil") || len(tool) > maxAuditMethodLen {
		t.Fatalf("unexpected sanitised tool %q", tool)
	}
	if audit["navident"] != "Z123456" {
		t.Fatalf("navident overwritten: %v", audit["navident"])
	}
	if string(got) != body {
		t.Fatal("upstream body changed")
	}
}

func TestAuditMCP_OversizedBody_NoToolLogged(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"big","arguments":{"x":"` +
		strings.Repeat("a", maxMCPRequestBodyBytes) + `"}}}`
	audit, got, code := auditPost(t, body)
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", code)
	}
	if audit["tool"] != "" || got != nil {
		t.Fatalf("oversized body must not be parsed or forwarded: tool=%v forwarded=%d bytes", audit["tool"], len(got))
	}
}

func TestAuditMCP_Batch(t *testing.T) {
	var parts []string
	for i := 0; i < maxAuditBatchTools+2; i++ {
		parts = append(parts, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"t","arguments":{"a":"top-secret-arg"}}}`)
	}
	parts = append(parts, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	body := "[" + strings.Join(parts, ",") + "]"
	audit, got, _ := auditPost(t, body)
	if audit["method"] != "batch" {
		t.Fatalf("expected method=batch, got %v", audit["method"])
	}
	want := strings.Repeat("t,", maxAuditBatchTools) + "..."
	if audit["tool"] != want {
		t.Fatalf("expected tool=%q, got %v", want, audit["tool"])
	}
	if string(got) != body {
		t.Fatal("upstream body changed")
	}
}

func TestExtractJSONRPCAudit_NonToolCallHasNoTool(t *testing.T) {
	m, tool := extractJSONRPCAudit([]byte(`{"method":"tools/list","params":{"name":"x"}}`))
	if m != "tools/list" || tool != "" {
		t.Fatalf("got %q %q", m, tool)
	}
}
