package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// flatAudit maps the AuthZEN-shaped audit record onto flat keys so tests can
// assert single fields; TestAuditMCP_AuthZENShape pins the real shape.
func flatAudit(e map[string]any) map[string]any {
	get := func(path ...string) any {
		var v any = e
		for _, k := range path {
			m, _ := v.(map[string]any)
			v = m[k]
		}
		return v
	}
	decision := "deny"
	if get("decision", "decision") == true {
		decision = "allow"
	}
	return map[string]any{
		"navident":        get("subject", "id"),
		"client_id":       get("subject", "properties", "client_id"),
		"method":          get("action", "name"),
		"tool":            get("action", "properties", "tool"),
		"route":           "/mcp/" + fmt.Sprint(get("resource", "id")),
		"decision":        decision,
		"decision_reason": get("decision", "context", "reason"),
		"header_mismatch": get("context", "header_mismatch"),
		"outcome":         get("context", "outcome"),
		"duration_ms":     get("context", "duration_ms"),
		"raw":             e,
	}
}

// auditPost sends body through the gateway and returns the audit entry and the
// body upstream received.
func auditPost(t *testing.T, body string) (map[string]any, []byte, int) {
	t.Helper()
	return auditPostTools(t, nil, body)
}

// auditPostTools is auditPost on a route whose tool allowlist is tools.
func auditPostTools(t *testing.T, tools map[string]bool, body string) (map[string]any, []byte, int) {
	t.Helper()
	return auditTools(t, http.MethodPost, tools, body)
}

func auditTools(t *testing.T, method string, tools map[string]bool, body string) (map[string]any, []byte, int) {
	t.Helper()
	return auditToolsHeaders(t, method, tools, body, nil)
}

func auditToolsHeaders(t *testing.T, method string, tools map[string]bool, body string, hdrs http.Header) (map[string]any, []byte, int) {
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
	te.gw.cfg.Routes[0].allowedTools = tools
	te.gw.routeTable.Store(newRouteTable(te.gw.cfg))
	te.handler = te.gw.routes()
	gwToken := te.obtainAccessTokenForResource(te.resource)

	req := httptest.NewRequest(method, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+gwToken)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdrs {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)

	var audit map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		var e map[string]any
		if json.Unmarshal([]byte(line), &e) == nil && e["msg"] == "audit mcp" {
			audit = flatAudit(e)
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
	if !strings.HasPrefix(tool, `"evil\n{`) {
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
	want := `[` + strings.Repeat(`"t",`, maxAuditBatchTools) + `"..."]`
	if audit["tool"] != want {
		t.Fatalf("expected tool=%q, got %v", want, audit["tool"])
	}
	if string(got) != body {
		t.Fatal("upstream body changed")
	}
}

func TestAuditMCP_MarksUnusualMethodAndTool(t *testing.T) {
	for _, tc := range []struct{ body, method, tool string }{
		{`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whoami "}}`, "tools/call", `"whoami "`},
		{`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whoami\u0000"}}`, "tools/call", `"whoami\x00"`},
		{`{"jsonrpc":"2.0","id":1,"method":"tools/\u200bcall"}`, `"tools/\u200bcall"`, ""},
		{`{"jsonrpc":"2.0","id":1,"method":"tools/call\ufeff"}`, `"tools/call\ufeff"`, ""},
	} {
		audit, _, _ := auditPostTools(t, map[string]bool{"whoami": true}, tc.body)
		if audit["method"] != tc.method || audit["tool"] != tc.tool {
			t.Errorf("%s: got method %v tool %v, want %s / %s", tc.body, audit["method"], audit["tool"], tc.method, tc.tool)
		}
	}
}

func TestAuditMCP_HeaderMismatch(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search"}}`
	for _, tc := range []struct {
		hdrs http.Header
		want bool
	}{
		{http.Header{}, false},
		{http.Header{"Mcp-Method": {"tools/call"}, "Mcp-Name": {"search"}}, false},
		{http.Header{"Mcp-Name": {"=?base64?c2VhcmNo?="}}, false},
		{http.Header{"Mcp-Method": {"tools/list"}}, true},
		{http.Header{"Mcp-Name": {"delete"}}, true},
	} {
		audit, _, code := auditToolsHeaders(t, http.MethodPost, nil, body, tc.hdrs)
		if code != http.StatusOK || audit["header_mismatch"] != tc.want {
			t.Errorf("%v: got code %d header_mismatch %v, want %v", tc.hdrs, code, audit["header_mismatch"], tc.want)
		}
	}
}

func TestAuditMCP_AuthZENShape(t *testing.T) {
	audit, _, _ := auditToolsHeaders(t, http.MethodPost, map[string]bool{"search": true},
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete","arguments":{"a":"top-secret-arg"}}}`,
		http.Header{"Mcp-Name": {"search"}, "Mcp-Protocol-Version": {"2026-07-28"}})
	raw := audit["raw"].(map[string]any)
	ctx := raw["context"].(map[string]any)
	if id, _ := ctx["request_id"].(string); len(id) != 26 {
		t.Errorf("request_id %q, want 26 base32 characters", id)
	}
	ctx["request_id"], ctx["duration_ms"] = "ID", 0
	delete(raw, "time")
	got, _ := json.Marshal(raw)
	want := `{"action":{"name":"tools/call","properties":{"tool":"delete"}},` +
		`"context":{"duration_ms":0,"header_mismatch":true,"http_method":"POST","mcp_headers":{"method":false,"name":true},"outcome":"tool_denied","protocol_version":"2026-07-28","request_id":"ID","scan_incomplete":false},` +
		`"decision":{"context":{"reason":"not_listed"},"decision":false},"level":"INFO","msg":"audit mcp",` +
		`"resource":{"id":"team-etterlevelse/etterlevelse","properties":{"backend":"dev-gcp:team-etterlevelse:nav-etterlevelse-mcp"},"type":"mcp_route"},` +
		`"subject":{"id":"Z123456","properties":{"client_id":"` + fmt.Sprint(audit["client_id"]) + `"},"type":"user"}}`
	if audit["client_id"] == "" || string(got) != want {
		t.Errorf("audit record\n got %s\nwant %s", got, want)
	}
}

func TestAuditMCP_UnauthenticatedHasNoSubject(t *testing.T) {
	audit, _, code := auditToolsHeaders(t, http.MethodPost, nil, `{"method":"ping"}`,
		http.Header{"Authorization": {"Bearer not-a-token"}})
	raw := audit["raw"].(map[string]any)
	if code != http.StatusUnauthorized {
		t.Fatalf("code %d, want 401", code)
	}
	if _, ok := raw["subject"]; ok {
		t.Errorf("unauthenticated record has subject %v; AuthZEN requires subject.id", raw["subject"])
	}
}

func TestAuditMCP_ScanIncompleteFromScan(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{`{"method":"tools/call","params":{"name":"...incomplete"}}`, false},
		{`{"method":"ping"}x`, true},
	} {
		audit, _, _ := auditPost(t, tc.body)
		ctx := audit["raw"].(map[string]any)["context"].(map[string]any)
		if ctx["scan_incomplete"] != tc.want {
			t.Errorf("%s: scan_incomplete %v, want %v", tc.body, ctx["scan_incomplete"], tc.want)
		}
	}
}

func TestToolPolicy_HeaderMismatchDeniedOnAllowlist(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search"}}`
	tools := map[string]bool{"search": true}
	_, got, code := auditToolsHeaders(t, http.MethodPost, tools, body, http.Header{"Mcp-Name": {"delete"}})
	if code != http.StatusBadRequest || got != nil {
		t.Fatalf("mismatch on allowlist: code %d, forwarded %q; want 400 and nothing forwarded", code, got)
	}
	audit, got, code := auditToolsHeaders(t, http.MethodPost, tools, body, http.Header{"Mcp-Name": {"search"}})
	if code != http.StatusOK || got == nil || audit["header_mismatch"] != false {
		t.Fatalf("matching headers: code %d", code)
	}
	for name, b := range map[string]string{
		"batch":      "[" + body + "]",
		"tools/list": `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
	} {
		if _, got, code := auditToolsHeaders(t, http.MethodPost, tools, b, http.Header{"Mcp-Name": {"delete"}}); code != http.StatusBadRequest || got != nil {
			t.Fatalf("%s with Mcp-Name: code %d, forwarded %q; want 400 and nothing forwarded", name, code, got)
		}
	}
	if _, _, code := auditToolsHeaders(t, http.MethodPost, nil, body, http.Header{"Mcp-Name": {"delete"}}); code != http.StatusOK {
		t.Fatalf("passAllTools route must keep forwarding on mismatch, got %d", code)
	}
}
