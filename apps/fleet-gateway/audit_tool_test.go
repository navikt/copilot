package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
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

func TestExtractJSONRPCAudit_BadBatchMemberSkipped(t *testing.T) {
	m, tool := extractJSONRPCAudit([]byte(`[42,{"method":"tools/call","params":"x"},{"method":"tools/call","params":{"name":"a"}},"s",{"method":"tools/call","params":{"name":"b"}}]`))
	if m != "batch" || tool != ",a,b" {
		t.Fatalf("got %q %q", m, tool)
	}
}

func TestExtractJSONRPCAudit_BadParamsKeepMethod(t *testing.T) {
	for _, body := range []string{
		`{"method":"tools/call","params":[1]}`,
		`{"method":"tools/call","params":{"name":7}}`,
	} {
		if m, tool := extractJSONRPCAudit([]byte(body)); m != "tools/call" || tool != "" {
			t.Fatalf("%s: got %q %q", body, m, tool)
		}
	}
}

func TestExtractJSONRPCAudit_ExactCaseKeys(t *testing.T) {
	for _, body := range []string{
		`{"method":"tools/call","params":{"name":"evil","Name":"benign"}}`,
		`{"method":"tools/call","params":{"Name":"benign","name":"evil"}}`,
	} {
		if _, tool := extractJSONRPCAudit([]byte(body)); tool != "evil" {
			t.Fatalf("%s: got tool %q, want evil", body, tool)
		}
	}
	if m, _ := extractJSONRPCAudit([]byte(`{"Method":"tools/call"}`)); m != "" {
		t.Fatalf("Method must not match method, got %q", m)
	}
}

func TestExtractJSONRPCAudit_BatchOfEmptyObjectsBoundedMemory(t *testing.T) {
	n := (maxMCPRequestBodyBytes - 2) / 3
	body := []byte("[" + strings.Repeat("{},", n-1) + "{}]")
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	extractJSONRPCAudit(body)
	runtime.ReadMemStats(&after)
	if got := after.TotalAlloc - before.TotalAlloc; got > 16<<20 {
		t.Fatalf("allocated %d MiB for a %d-byte batch, want < 16 MiB", got>>20, len(body))
	}
}
