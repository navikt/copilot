package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToolMetricsEndpoint(t *testing.T) {
	srv := newTestServer(t, defaultTexasHandler("obo-tok"), func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	h := srv.routes()
	callToolModern(t, h, "user-token", "list_clusters", map[string]any{})
	callToolModern(t, h, "user-token", "Z123456-not-a-tool", map[string]any{})

	rec := httptest.NewRecorder()
	srv.metrics.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	out := rec.Body.String()
	for _, want := range []string{
		`mcp_tool_requests_total{tool="list_clusters"} 1`,
		`mcp_tool_errors_total{tool="list_clusters"} 1`,
		`mcp_tool_requests_total{tool="unknown"} 1`,
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
