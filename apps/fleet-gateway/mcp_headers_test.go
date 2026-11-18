package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestProxy_ForwardsMCPRequestMetadataHeaders(t *testing.T) {
	te := newTestEnv(t)
	te.texasServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "team-obo", "token_type": "Bearer", "expires_in": 300})
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
		writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": "ok"})
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
	for i := 0; i <= maxMCPParamHeaders; i++ {
		tooMany["Mcp-Param-P"+strconv.Itoa(i)] = "x"
	}
	for name, hdrs := range map[string]map[string]string{
		"count": tooMany,
		"size":  {"Mcp-Param-Q": strings.Repeat("a", maxMCPParamHeaderBytes)},
	} {
		if rec := post(hdrs); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", name, rec.Code)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if upstreamCalls != 1 {
		t.Errorf("rejected requests must not reach upstream, got %d calls", upstreamCalls)
	}
}
