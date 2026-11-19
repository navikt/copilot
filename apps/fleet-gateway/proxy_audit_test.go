package main

import (
	"encoding/json"
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

func TestRPCMethodTool_MethodCaseIsAmbiguous(t *testing.T) {
	_, _, amb := rpcMethodTool(json.RawMessage(`"Tools/Call"`), json.RawMessage(`{"name":"x"}`))
	if !amb {
		t.Fatal("Tools/Call must be flagged ambiguous")
	}
}

func TestAcceptsEventStream_QZero(t *testing.T) {
	for accept, want := range map[string]bool{
		"text/event-stream;q=0":                   false,
		"application/json, text/event-stream;q=0": false,
		"*/*;q=0.0, application/json":             false,
		"text/event-stream;q=0.5":                 true,
		"application/json, text/event-stream":     true,
	} {
		if got := acceptsEventStream(http.Header{"Accept": {accept}}); got != want {
			t.Errorf("%q: got %v, want %v", accept, got, want)
		}
	}
}

func TestHardenedProxyClient_CapsResponseHeaders(t *testing.T) {
	tr, ok := newHardenedProxyClient(0).Transport.(*http.Transport)
	if !ok || tr.MaxResponseHeaderBytes != 64<<10 {
		t.Fatalf("MaxResponseHeaderBytes = %v, want 64 KiB", tr.MaxResponseHeaderBytes)
	}
}
