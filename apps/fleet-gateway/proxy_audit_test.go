package main

import (
	"net/http"
	"testing"
)

func TestMCPSession_RejectedUpstreamReplyLeavesNoBinding(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MaxResponseBodyBytes = 8
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "too-big")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	})
	token := te.obtainAccessTokenForResource(te.resource)
	if rec := mcpPost(te, token, ""); rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
	if _, found := mcpBound(te, "too-big"); found {
		t.Fatal("a rejected upstream reply created a binding")
	}
}
