package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToolMetricsEndpoint(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"kodeverksnavn": []string{"Landkoder"}})
	})
	h := srv.routes()
	callToolModern(t, h, "user-token", "list_kodeverk", map[string]any{})
	callToolModern(t, h, "user-token", "list_kodeverk", map[string]any{"bogus": 1})
	callToolModern(t, h, "user-token", "Z123456-not-a-tool", map[string]any{})

	rec := httptest.NewRecorder()
	srv.metrics.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	// /metrics is on its own port; the MCP port treats it as an MCP request.
	main := httptest.NewRecorder()
	h.ServeHTTP(main, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if main.Code != http.StatusUnauthorized {
		t.Fatalf("GET /metrics on the MCP port: %d, want 401", main.Code)
	}
	out := rec.Body.String()
	for _, want := range []string{
		`mcp_tool_requests_total{tool="list_kodeverk"} 2`,
		`mcp_tool_errors_total{tool="list_kodeverk"} 1`,
		`mcp_tool_requests_total{tool="unknown"} 1`,
		`mcp_tool_duration_seconds_count{tool="search_kodeverk"} 0`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	for _, leak := range []string{"Z123456", "user-token", "obo-tok"} {
		if strings.Contains(out, leak) {
			t.Fatalf("%q leaked into /metrics", leak)
		}
	}
}

func TestToolMetricsCountsFailedExchange(t *testing.T) {
	texas := defaultTexasHandler("obo-tok")
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/exchange" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
		texas(w, r)
	}, func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("kodeverk must not be called after a failed exchange")
	})
	_, resp := callToolModern(t, srv.routes(), "user-token", "list_kodeverk", map[string]any{})
	if res, _ := resp["result"].(map[string]any); res["isError"] != true {
		t.Fatalf("want isError tool result, got %v", resp)
	}
	rec := httptest.NewRecorder()
	srv.metrics.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, want := range []string{
		`mcp_tool_requests_total{tool="list_kodeverk"} 1`,
		`mcp_tool_errors_total{tool="list_kodeverk"} 1`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("missing %s in:\n%s", want, rec.Body.String())
		}
	}
}
