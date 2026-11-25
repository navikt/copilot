package main

import (
	"github.com/navikt/copilot/fleet-gateway/internal/proxy"
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

func TestHardenedProxyClient_CapsResponseHeaders(t *testing.T) {
	tr, ok := proxy.NewHardenedClient(0).Transport.(*http.Transport)
	if !ok || tr.MaxResponseHeaderBytes != 64<<10 {
		t.Fatalf("MaxResponseHeaderBytes = %v, want 64 KiB", tr.MaxResponseHeaderBytes)
	}
}
