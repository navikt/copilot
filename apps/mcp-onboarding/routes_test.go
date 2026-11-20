package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPRoutesAnswer405ForRemovedVerbs(t *testing.T) {
	t.Setenv(mcpLegacyUntilEnv, "")
	mux := http.NewServeMux()
	id := func(h http.Handler) http.Handler { return h }
	registerMCPRoutes(mux, newSimpleMCPHandler(), id, id)
	mux.HandleFunc("/", http.NotFound)
	for _, tc := range []struct{ verb, path string }{
		{"GET", "/mcp"}, {"DELETE", "/mcp"}, {"GET", "/mcp/gw"}, {"DELETE", "/mcp/gw"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(tc.verb, tc.path, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", tc.verb, tc.path, rec.Code)
		}
		if got := rec.Header().Get("Allow"); got != http.MethodPost {
			t.Errorf("%s %s Allow = %q, want POST", tc.verb, tc.path, got)
		}
	}
}
