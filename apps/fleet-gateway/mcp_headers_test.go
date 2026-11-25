package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/navikt/copilot/fleet-gateway/internal/policy"

	"github.com/navikt/copilot/fleet-gateway/internal/httpx"
)

func TestProxy_ForwardsMCPRequestMetadataHeaders(t *testing.T) {
	te := newTestEnv(t)
	te.texasServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"access_token": "team-obo", "token_type": "Bearer", "expires_in": 300})
	})
	var (
		mu  sync.Mutex
		got http.Header
	)
	upstreamCalls := 0
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Clone()
		upstreamCalls++
		mu.Unlock()
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": "ok"})
	})
	gwToken := te.obtainAccessTokenForResource(te.resource)

	post := func(hdrs map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search"}}`))
		req.Header.Set("Authorization", "Bearer "+gwToken)
		req.Header.Set("Content-Type", "application/json")
		for k, v := range hdrs {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		return rec
	}

	want := map[string]string{
		"Mcp-Protocol-Version": "2026-07-28",
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             "=?base64?c8O4aw==?=",
		"Mcp-Param-Region":     "=?base64?w6Zyw7g=?=",
		"mcp-param-limit":      "42",
	}
	if rec := post(want); rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	mu.Lock()
	for k, v := range want {
		if got.Get(k) != v {
			t.Errorf("header %s: got %q, want %q", k, got.Get(k), v)
		}
	}
	if got.Get("Authorization") != "Bearer team-obo" {
		t.Errorf("expected gateway OBO token upstream, got %q", got.Get("Authorization"))
	}
	mu.Unlock()

	tooMany := map[string]string{}
	for i := 0; i <= policy.MaxMCPParamHeaders; i++ {
		tooMany["Mcp-Param-P"+strconv.Itoa(i)] = "x"
	}
	for name, hdrs := range map[string]map[string]string{
		"count": tooMany,
		"size":  {"Mcp-Param-Q": strings.Repeat("a", policy.MaxMCPParamHeaderBytes)},
	} {
		if rec := post(hdrs); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", name, rec.Code)
		}
	}
	for name, hdrs := range map[string]http.Header{
		"name size":      {"Mcp-Name": {strings.Repeat("a", policy.MaxMCPNameHeaderBytes+1)}},
		"method size":    {"Mcp-Method": {strings.Repeat("a", policy.MaxMCPNameHeaderBytes+1)}},
		"name twice":     {"Mcp-Name": {"search", "delete"}},
		"method twice":   {"Mcp-Method": {"tools/call", "tools/list"}},
		"obs-text name":  {"Mcp-Name": {"s\xf8k"}},
		"obs-text param": {"Mcp-Param-Q": {"\xe6"}},
	} {
		req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search"}}`))
		req.Header.Set("Authorization", "Bearer "+gwToken)
		for k, v := range hdrs {
			req.Header[k] = v
		}
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", name, rec.Code)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if upstreamCalls != 1 {
		t.Errorf("rejected requests must not reach upstream, got %d calls", upstreamCalls)
	}
}

// #2851: the client query string never reaches the upstream, more than one
// Content-Type is rejected, and headers outside the allowlist are dropped.
func TestProxy_StripsQueryAndRejectsDuplicateContentType(t *testing.T) {
	te := newTestEnv(t)
	te.texasServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"access_token": "team-obo", "token_type": "Bearer", "expires_in": 300})
	})
	var (
		mu       sync.Mutex
		gotQuery string
		gotHdr   http.Header
		calls    int
	)
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotQuery, gotHdr = r.URL.RawQuery, r.Header.Clone()
		calls++
		mu.Unlock()
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": "ok"})
	})
	gwToken := te.obtainAccessTokenForResource(te.resource)

	post := func(target string, ct []string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Header.Set("Authorization", "Bearer "+gwToken)
		req.Header["Content-Type"] = ct
		req.Header.Set("Cookie", "a=b")
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := post("/mcp/team-etterlevelse/etterlevelse?admin=1&sessionId=x", []string{"application/json"}); rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	mu.Lock()
	if gotQuery != "" {
		t.Errorf("client query reached upstream: %q", gotQuery)
	}
	for _, h := range []string{"Cookie", "X-Forwarded-For"} {
		if gotHdr.Get(h) != "" {
			t.Errorf("non-allowlisted header %s reached upstream", h)
		}
	}
	before := calls
	mu.Unlock()

	rec := post("/mcp/team-etterlevelse/etterlevelse", []string{"application/json", "text/plain"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for two Content-Type headers, got %d", rec.Code)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != before {
		t.Errorf("duplicate Content-Type request reached upstream")
	}
}
