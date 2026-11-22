package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postMCP(t *testing.T, body string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	newSimpleMCPHandler().ServeHTTP(rec, req)
	return rec
}

func TestModernOversizedBodyIs413(t *testing.T) {
	big := `{"jsonrpc":"2.0","id":1,"method":"tools/list","pad":"` + strings.Repeat("a", 1<<20) + `"}`
	if rec := postMCP(t, big, map[string]string{"MCP-Protocol-Version": mcpModernVersion}); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("modern: status = %d, want 413", rec.Code)
	}
	// No header: the body may declare 2026-07-28 in _meta, so fail closed.
	if rec := postMCP(t, big, nil); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("no header: status = %d, want 413", rec.Code)
	}
	// An explicit legacy header keeps the old behaviour: no 413.
	if rec := postMCP(t, big, map[string]string{"MCP-Protocol-Version": mcpLegacyVersions[0]}); rec.Code == http.StatusRequestEntityTooLarge {
		t.Fatalf("legacy: got 413, want the old parse error")
	}
}

func TestModernInvalidRequestIs32600(t *testing.T) {
	hdr := map[string]string{"MCP-Protocol-Version": mcpModernVersion, "Mcp-Method": "tools/list"}
	for name, body := range map[string]string{
		"missing method": `{"jsonrpc":"2.0","id":1}`,
		"null id":        `{"jsonrpc":"2.0","id":null,"method":"tools/list"}`,
		"missing id":     `{"jsonrpc":"2.0","method":"tools/list"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := postMCP(t, body, hdr)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "-32600") {
				t.Fatalf("status=%d body=%s, want 400 -32600", rec.Code, rec.Body.String())
			}
		})
	}
	// A notification without id is still accepted.
	nb := `{"jsonrpc":"2.0","method":"notifications/initialized","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + mcpModernVersion + `","io.modelcontextprotocol/clientCapabilities":{}}}}`
	if rec := postMCP(t, nb, map[string]string{"MCP-Protocol-Version": mcpModernVersion, "Mcp-Method": "notifications/initialized"}); rec.Code != http.StatusAccepted {
		t.Fatalf("notification: status = %d, want 202", rec.Code)
	}
}
