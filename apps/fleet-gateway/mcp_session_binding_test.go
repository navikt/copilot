package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func mcpPost(te *testEnv, token, sessionID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	return rec
}

func mcpBound(te *testEnv, id string) (string, bool) {
	owner, found, err := te.gw.store.Get(context.Background(), kindMCPSession, "/mcp/team-etterlevelse/etterlevelse\n"+id)
	if err != nil {
		te.t.Fatalf("store get: %v", err)
	}
	return string(owner), found
}

func setUpstreamSessionID(te *testEnv, id *string) {
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if *id != "" {
			w.Header().Set("Mcp-Session-Id", *id)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	})
}

func TestMCPSession_ClientSuppliedIDsAreNeverBound(t *testing.T) {
	te := newTestEnv(t)
	none := ""
	setUpstreamSessionID(te, &none) // stateless upstream: 2xx, no session header
	token := te.obtainAccessTokenForResource(te.resource)
	for i := range 20 {
		id := fmt.Sprintf("random-%d", i)
		if rec := mcpPost(te, token, id); rec.Code != http.StatusOK {
			t.Fatalf("unbound client id must pass through, got %d (%s)", rec.Code, rec.Body.String())
		}
		if _, found := mcpBound(te, id); found {
			t.Fatalf("client-supplied id %q created a binding", id)
		}
	}
}

func TestMCPSession_UpstreamIssuedIDBindsToUser(t *testing.T) {
	te := newTestEnv(t)
	issued := "upstream-issued-1"
	setUpstreamSessionID(te, &issued)
	tokenA := te.obtainAccessTokenForResource(te.resource)
	if rec := mcpPost(te, tokenA, ""); rec.Code != http.StatusOK {
		t.Fatalf("initialize: got %d (%s)", rec.Code, rec.Body.String())
	}
	if owner, found := mcpBound(te, issued); !found || owner != te.navIdent {
		t.Fatalf("expected binding to %s, got %q found=%v", te.navIdent, owner, found)
	}

	te.navIdent = "B654321"
	tokenB := te.obtainAccessTokenForResource(te.resource)
	if rec := mcpPost(te, tokenB, issued); rec.Code != http.StatusForbidden {
		t.Fatalf("user B on A's session: expected 403, got %d", rec.Code)
	}
	if rec := mcpPost(te, tokenA, issued); rec.Code != http.StatusOK {
		t.Fatalf("user A on own session: expected 200, got %d", rec.Code)
	}
}

func TestMCPSession_PerUserCap(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPSessionUserCap = 3
	issued := ""
	setUpstreamSessionID(te, &issued)
	token := te.obtainAccessTokenForResource(te.resource)
	for i := range 5 {
		issued = fmt.Sprintf("up-%d", i)
		if rec := mcpPost(te, token, ""); rec.Code != http.StatusOK {
			t.Fatalf("got %d", rec.Code)
		}
		_, found := mcpBound(te, issued)
		if want := i < 3; found != want {
			t.Fatalf("session %d: bound=%v, want %v", i, found, want)
		}
	}
}

func TestMCPSession_DeleteReleasesBindingAndCapSlot(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPSessionUserCap = 1
	issued := "up-0"
	setUpstreamSessionID(te, &issued)
	token := te.obtainAccessTokenForResource(te.resource)
	if rec := mcpPost(te, token, ""); rec.Code != http.StatusOK {
		t.Fatalf("initialize: got %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodDelete, "/mcp/team-etterlevelse/etterlevelse", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Mcp-Session-Id", "up-0")
	rec := httptest.NewRecorder()
	issued = ""
	te.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: got %d", rec.Code)
	}
	if _, found := mcpBound(te, "up-0"); found {
		t.Fatal("DELETE left the binding in place")
	}

	issued = "up-1"
	if rec := mcpPost(te, token, ""); rec.Code != http.StatusOK {
		t.Fatalf("second initialize: got %d", rec.Code)
	}
	if _, found := mcpBound(te, "up-1"); !found {
		t.Fatal("cap slot was not released by DELETE")
	}
}

func mcpDelete(te *testEnv, token, sessionID string) int {
	req := httptest.NewRequest(http.MethodDelete, "/mcp/team-etterlevelse/etterlevelse", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Mcp-Session-Id", sessionID)
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	return rec.Code
}

func mcpUserCount(te *testEnv) string {
	raw, _, err := te.gw.store.Get(context.Background(), kindMCPSessionUser, te.gw.mcpSessionUserKey(te.navIdent))
	if err != nil {
		te.t.Fatalf("store get: %v", err)
	}
	return string(raw)
}

func TestMCPSession_DeleteDoesNotRebindEchoedID(t *testing.T) {
	te := newTestEnv(t)
	issued := "up-0"
	setUpstreamSessionID(te, &issued) // upstream echoes the id on DELETE too
	token := te.obtainAccessTokenForResource(te.resource)
	if rec := mcpPost(te, token, ""); rec.Code != http.StatusOK {
		t.Fatalf("initialize: got %d", rec.Code)
	}
	if code := mcpDelete(te, token, "up-0"); code != http.StatusOK {
		t.Fatalf("delete: got %d", code)
	}
	if _, found := mcpBound(te, "up-0"); found {
		t.Fatal("echoed Mcp-Session-Id on DELETE was bound again")
	}
	if got := mcpUserCount(te); got != "0" {
		t.Fatalf("user count = %q, want 0", got)
	}
}

func TestMCPSession_ConcurrentDeletesFreeOneSlot(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.MCPSessionUserCap = 2
	issued := ""
	setUpstreamSessionID(te, &issued)
	token := te.obtainAccessTokenForResource(te.resource)
	for _, id := range []string{"up-0", "up-1"} {
		issued = id
		if rec := mcpPost(te, token, ""); rec.Code != http.StatusOK {
			t.Fatalf("initialize %s: got %d", id, rec.Code)
		}
	}
	issued = ""
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() { mcpDelete(te, token, "up-0") })
	}
	wg.Wait()
	if got := mcpUserCount(te); got != "1" {
		t.Fatalf("user count = %q after concurrent DELETEs of one session, want 1", got)
	}
}

type unknownKIDBindingStore struct{ SessionStore }

func (s unknownKIDBindingStore) Get(ctx context.Context, kind, key string) ([]byte, bool, error) {
	if kind == kindMCPSession {
		return nil, false, fmt.Errorf("get: %w", errSealUnknownKID)
	}
	return s.SessionStore.Get(ctx, kind, key)
}

func TestMCPSession_UnknownKIDBindingFailsClosed(t *testing.T) {
	te := newTestEnv(t)
	none := ""
	setUpstreamSessionID(te, &none)
	token := te.obtainAccessTokenForResource(te.resource)
	te.gw.store = unknownKIDBindingStore{te.gw.store}
	rec := mcpPost(te, token, "sealed-with-old-key")
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "start a new MCP session") {
		t.Fatalf("got %d %s, want 403 telling the client to start a new session", rec.Code, rec.Body.String())
	}
}
