package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func (te *testEnv) obtainAccessTokenForResource(resource string) string {
	te.t.Helper()
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	req := httptest.NewRequest(http.MethodGet, te.authorizeURL(map[string]string{"resource": resource}, verifier), nil)
	entraState, stateCookie := te.followToEntra(req)
	code := te.completeEntraCallback(entraState, stateCookie)
	status, tokBody := te.postToken(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {te.clientID},
		"redirect_uri":  {te.redirectURI},
		"resource":      {resource},
		"code_verifier": {verifier},
	})
	if status != http.StatusOK {
		te.t.Fatalf("expected 200 from /token, got %d (%v)", status, tokBody)
	}
	tok, _ := tokBody["access_token"].(string)
	if tok == "" {
		te.t.Fatal("expected non-empty gateway access_token")
	}
	return tok
}

func TestGatewayAccessToken_HashedStorage_ResourceBinding_AndExpiry(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.Routes = append(te.gw.cfg.Routes, PilotRoute{
		Route:       "/mcp/team-sec/restricted",
		Team:        "team-sec",
		Target:      "dev-gcp:team-sec:sec-mcp",
		UpstreamURL: "http://sec-mcp.team-sec",
	})
	if err := validateConfig(te.gw.cfg); err != nil {
		t.Fatalf("validateConfig failed: %v", err)
	}
	parsedUpstream, err := url.Parse(te.upstreamServer.URL)
	if err != nil {
		t.Fatalf("parse upstream server URL: %v", err)
	}
	for i := range te.gw.cfg.Routes {
		te.gw.cfg.Routes[i].ParsedURL = parsedUpstream
	}
	te.gw.TeamTokens, err = newTeamTokenSource(te.gw.cfg, te.gw.store, func() time.Time {
		te.mu.Lock()
		defer te.mu.Unlock()
		return te.now
	})
	if err != nil {
		t.Fatalf("newTeamTokenSource failed: %v", err)
	}
	te.handler = te.gw.routes()

	token := te.obtainAccessTokenForResource(te.resource)

	// Session map must store only SHA-256(token), never the raw token.
	if _, ok, err := te.gw.getSession(context.Background(), token); err != nil || ok {
		t.Fatal("raw gateway token must not be stored in sessions map")
	}
	if _, ok, err := te.gw.getSession(context.Background(), hashToken(token)); err != nil || !ok {
		t.Fatal("expected SHA-256(token) in sessions map")
	}

	// PRM exposes each listed route and rejects unlisted routes.
	prmReq := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource/mcp/team-sec/restricted", nil)
	prmRec := httptest.NewRecorder()
	te.handler.ServeHTTP(prmRec, prmReq)
	if prmRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on second route PRM, got %d", prmRec.Code)
	}
	var prm map[string]any
	_ = json.Unmarshal(prmRec.Body.Bytes(), &prm)
	if prm["resource"] != te.gw.cfg.BaseURL+"/mcp/team-sec/restricted" {
		t.Fatalf("unexpected PRM resource %v", prm["resource"])
	}

	// Unlisted route returns 404.
	unlistedReq := httptest.NewRequest(http.MethodPost, "/mcp/team-sec/unlisted", strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list"}`))
	unlistedReq.Header.Set("Authorization", "Bearer "+token)
	unlistedRec := httptest.NewRecorder()
	te.handler.ServeHTTP(unlistedRec, unlistedReq)
	if unlistedRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unlisted route, got %d", unlistedRec.Code)
	}

	// Missing Bearer returns 401 with WWW-Authenticate resource_metadata.
	noAuthReq := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{}`))
	noAuthRec := httptest.NewRecorder()
	te.handler.ServeHTTP(noAuthRec, noAuthReq)
	if noAuthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without Bearer, got %d", noAuthRec.Code)
	}
	if wwwAuth := noAuthRec.Header().Get("WWW-Authenticate"); !strings.Contains(wwwAuth, "resource_metadata=") {
		t.Fatalf("expected WWW-Authenticate resource_metadata, got %q", wwwAuth)
	}

	// Token bound to /mcp/team-etterlevelse/etterlevelse cannot call /mcp/team-sec/restricted (403).
	wrongResReq := httptest.NewRequest(http.MethodPost, "/mcp/team-sec/restricted", strings.NewReader(`{}`))
	wrongResReq.Header.Set("Authorization", "Bearer "+token)
	wrongResRec := httptest.NewRecorder()
	te.handler.ServeHTTP(wrongResRec, wrongResReq)
	if wrongResRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for resource mismatch, got %d", wrongResRec.Code)
	}

	// No group gate on the route (required_group removed 2026-11-17): a token
	// bound to the route reaches it; the team MCP authorizes.
	restrictedTok := te.obtainAccessTokenForResource(te.gw.cfg.BaseURL + "/mcp/team-sec/restricted")
	routeReq := httptest.NewRequest(http.MethodPost, "/mcp/team-sec/restricted", strings.NewReader(`{"jsonrpc":"2.0","method":"tools/list"}`))
	routeReq.Header.Set("Authorization", "Bearer "+restrictedTok)
	routeRec := httptest.NewRecorder()
	te.handler.ServeHTTP(routeRec, routeReq)
	if routeRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on the bound route, got %d (%s)", routeRec.Code, routeRec.Body.String())
	}

	// Expiry: advancing past session TTL returns 401.
	te.advance(2 * time.Hour)
	expReq := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{}`))
	expReq.Header.Set("Authorization", "Bearer "+token)
	expRec := httptest.NewRecorder()
	te.handler.ServeHTTP(expRec, expReq)
	if expRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after session expiry, got %d", expRec.Code)
	}
}

func TestProxy_OBO_Headers_StreamableHTTP_SSE_Limits_And_Redirects(t *testing.T) {
	te := newTestEnv(t)

	var (
		mu              sync.Mutex
		gotTexasPayload map[string]string
		gotUpstreamAuth string
		gotUpstreamHdrs http.Header
		gotMethods      []string
	)

	te.texasServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		_ = json.NewDecoder(r.Body).Decode(&payload)
		mu.Lock()
		gotTexasPayload = payload
		mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token": strings.Join([]string{"team", "obo", "token"}, "-"),
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	})

	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotUpstreamAuth = r.Header.Get("Authorization")
		gotUpstreamHdrs = r.Header.Clone()
		gotMethods = append(gotMethods, r.Method)
		mu.Unlock()

		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			_, _ = w.Write([]byte("event: message\ndata: {\"one\":1}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			_, _ = w.Write([]byte("event: message\ndata: {\"two\":2}\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Mcp-Session-Id", "upstream-sess-1")
			writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": "ok"})
		}
	})

	gwToken := te.obtainAccessTokenForResource(te.resource)

	// 1. POST with hop-by-hop headers, Cookie, and gateway Authorization.
	req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse?foo=bar", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+gwToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "fleet_csrf=secret-cookie")
	req.Header.Set("Connection", "Keep-Alive, X-Hop-Custom")
	req.Header.Set("Keep-Alive", "timeout=5")
	req.Header.Set("X-Hop-Custom", "should-be-stripped")
	req.Header.Set("Proxy-Authorization", "Basic abc")
	req.Header.Set("Mcp-Session-Id", "client-sess-1")
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST, got %d (%s)", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Mcp-Session-Id") != "upstream-sess-1" {
		t.Fatalf("expected Mcp-Session-Id preserved, got %q", rec.Header().Get("Mcp-Session-Id"))
	}

	mu.Lock()
	if gotTexasPayload["identity_provider"] != "entra_id" ||
		gotTexasPayload["target"] != "dev-gcp:team-etterlevelse:nav-etterlevelse-mcp" ||
		gotTexasPayload["user_token"] != te.entraToken {
		t.Fatalf("unexpected Texas exchange payload: %+v", gotTexasPayload)
	}
	if gotUpstreamAuth != "Bearer "+strings.Join([]string{"team", "obo", "token"}, "-") {
		t.Fatalf("expected team OBO bearer token, got %q", gotUpstreamAuth)
	}
	if strings.Contains(gotUpstreamAuth, gwToken) {
		t.Fatal("gateway token leaked to upstream!")
	}
	for _, stripped := range []string{"Cookie", "Keep-Alive", "X-Hop-Custom", "Proxy-Authorization", "Connection"} {
		if v := gotUpstreamHdrs.Get(stripped); v != "" {
			t.Fatalf("expected header %s stripped, got %q", stripped, v)
		}
	}
	if gotUpstreamHdrs.Get("Mcp-Session-Id") != "client-sess-1" {
		t.Fatalf("expected Mcp-Session-Id forwarded, got %q", gotUpstreamHdrs.Get("Mcp-Session-Id"))
	}
	mu.Unlock()

	// 2. GET SSE streaming passthrough via real HTTP server to test flushing.
	ts := httptest.NewServer(te.handler)
	defer ts.Close()
	sseReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/mcp/team-etterlevelse/etterlevelse", nil)
	sseReq.Header.Set("Authorization", "Bearer "+gwToken)
	sseReq.Header.Set("Accept", "text/event-stream")
	sseResp, err := ts.Client().Do(sseReq) //nolint:gosec // test server URL
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer func() { _ = sseResp.Body.Close() }()
	if sseResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 SSE, got %d", sseResp.StatusCode)
	}
	if sseResp.Header.Get("Connection") != "" {
		t.Fatalf("expected hop-by-hop Connection header stripped from response, got %q", sseResp.Header.Get("Connection"))
	}
	scanner := bufio.NewScanner(sseResp.Body)
	var sseLines []string
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			sseLines = append(sseLines, line)
		}
	}
	if len(sseLines) != 4 {
		t.Fatalf("expected 4 SSE lines streamed, got %v", sseLines)
	}

	// 3. DELETE passthrough.
	delReq := httptest.NewRequest(http.MethodDelete, "/mcp/team-etterlevelse/etterlevelse", nil)
	delReq.Header.Set("Authorization", "Bearer "+gwToken)
	delRec := httptest.NewRecorder()
	te.handler.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on DELETE, got %d", delRec.Code)
	}

	// 4. Unsupported method returns 405.
	putReq := httptest.NewRequest(http.MethodPut, "/mcp/team-etterlevelse/etterlevelse", nil)
	putReq.Header.Set("Authorization", "Bearer "+gwToken)
	putRec := httptest.NewRecorder()
	te.handler.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on PUT, got %d", putRec.Code)
	}

	// 5. Oversized request body returns 413.
	bigBody := bytes.Repeat([]byte("a"), maxMCPRequestBodyBytes+10)
	bigReq := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", bytes.NewReader(bigBody))
	bigReq.Header.Set("Authorization", "Bearer "+gwToken)
	bigRec := httptest.NewRecorder()
	te.handler.ServeHTTP(bigRec, bigReq)
	if bigRec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 on oversized body, got %d", bigRec.Code)
	}

	// 6. Upstream redirect is refused -> 502.
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.example/", http.StatusFound)
	})
	redirReq := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{}`))
	redirReq.Header.Set("Authorization", "Bearer "+gwToken)
	redirRec := httptest.NewRecorder()
	te.handler.ServeHTTP(redirRec, redirReq)
	if redirRec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when upstream redirects, got %d", redirRec.Code)
	}

	// 7. Upstream header timeout -> 502.
	te.gw.proxyClient = newHardenedProxyClient(50 * time.Millisecond)
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	slowReq := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{}`))
	slowReq.Header.Set("Authorization", "Bearer "+gwToken)
	slowRec := httptest.NewRecorder()
	te.handler.ServeHTTP(slowRec, slowReq)
	if slowRec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on upstream timeout, got %d", slowRec.Code)
	}
}

func TestStrategyRefresh_Rotation_And_Concurrency(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.TeamTokenStrategy = strategyRefresh
	te.gw.cfg.MCPMaxInFlightPerUser = 8
	te.handler = te.gw.routes()
	var err error
	te.gw.TeamTokens, err = newTeamTokenSource(te.gw.cfg, te.gw.store, func() time.Time {
		te.mu.Lock()
		defer te.mu.Unlock()
		return te.now
	})
	if err != nil {
		t.Fatalf("newTeamTokenSource refresh failed: %v", err)
	}

	var (
		mu             sync.Mutex
		refreshCounter int
		seenRTs        []string
		seenScopes     []string
		upstreamAuths  []string
	)

	te.entraServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(w, http.StatusOK, map[string]any{
				"authorization_endpoint": te.entraServer.URL + "/authorize",
				"token_endpoint":         te.entraServer.URL + "/token",
			})
		case "/token":
			_ = r.ParseForm()
			if r.FormValue("grant_type") == "authorization_code" {
				te.mu.Lock()
				nonce := te.lastEntraNonce
				te.mu.Unlock()
				writeJSON(w, http.StatusOK, map[string]any{
					"access_token":  te.entraToken,
					"refresh_token": "rt-0",
					"id_token":      makeTestJWT(map[string]any{"NAVident": "Z123456", "nonce": nonce}),
					"expires_in":    3600,
				})
				return
			}
			if r.FormValue("grant_type") == "refresh_token" {
				time.Sleep(20 * time.Millisecond) // widen race window for concurrent callers
				mu.Lock()
				refreshCounter++
				idx := refreshCounter
				seenRTs = append(seenRTs, r.FormValue("refresh_token"))
				seenScopes = append(seenScopes, r.FormValue("scope"))
				mu.Unlock()
				writeJSON(w, http.StatusOK, map[string]any{
					"access_token":  fmt.Sprintf("team-rt-access-%d", idx),
					"refresh_token": fmt.Sprintf("rt-%d", idx),
					"expires_in":    3600,
				})
				return
			}
			http.Error(w, "bad grant", http.StatusBadRequest)
		}
	})

	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		upstreamAuths = append(upstreamAuths, r.Header.Get("Authorization"))
		mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	gwToken := te.obtainAccessTokenForResource(te.resource)

	// Concurrent calls to same target coalesce via singleflight.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"method":"tools/list"}`))
			req.Header.Set("Authorization", "Bearer "+gwToken)
			rec := httptest.NewRecorder()
			te.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("concurrent MCP call failed with %d: %s", rec.Code, rec.Body.String())
			}
		}()
	}
	wg.Wait()

	// Sequential second call must send the rotated RT ("rt-1")!
	req2 := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"method":"tools/call"}`))
	req2.Header.Set("Authorization", "Bearer "+gwToken)
	rec2 := httptest.NewRecorder()
	te.handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second MCP call failed: %d", rec2.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	if refreshCounter != 2 {
		t.Fatalf("expected 2 Entra refresh calls (1 coalesced + 1 sequential), got %d", refreshCounter)
	}
	if seenRTs[0] != "rt-0" || seenRTs[1] != "rt-1" {
		t.Fatalf("expected RT rotation rt-0 -> rt-1, got %v", seenRTs)
	}
	wantScope := "api://dev-gcp.team-etterlevelse.nav-etterlevelse-mcp/.default"
	if seenScopes[0] != wantScope {
		t.Fatalf("expected scope %q, got %q", wantScope, seenScopes[0])
	}
	if len(upstreamAuths) != 6 {
		t.Fatalf("expected 6 upstream calls, got %d", len(upstreamAuths))
	}
}

func TestErrorMapping_AADSTS65001_And_NoRawEntraLeak(t *testing.T) {
	tests := []struct {
		name           string
		strategy       string
		upstreamBody   map[string]any
		upstreamStatus int
		wantError      string
		wantMessage    string
		forbiddenLeak  string
	}{
		{
			name:     "Texas AADSTS65001 maps to upstream_access_missing",
			strategy: strategyOBO,
			upstreamBody: map[string]any{
				"error":             "invalid_grant",
				"error_description": "AADSTS65001: The user or administrator has not consented to use the application",
			},
			upstreamStatus: http.StatusBadRequest,
			wantError:      "upstream_access_missing",
			wantMessage:    "fleet-gateway has no access to nav-etterlevelse-mcp; the team must add fleet-gateway to accessPolicy.inbound",
			forbiddenLeak:  "The user or administrator has not consented",
		},
		{
			name:     "Entra refresh AADSTS65001 maps to upstream_access_missing",
			strategy: strategyRefresh,
			upstreamBody: map[string]any{
				"error":             "invalid_grant",
				"error_description": "AADSTS65001: missing consent",
			},
			upstreamStatus: http.StatusBadRequest,
			wantError:      "upstream_access_missing",
			wantMessage:    "fleet-gateway has no access to nav-etterlevelse-mcp; the team must add fleet-gateway to accessPolicy.inbound",
			forbiddenLeak:  "missing consent",
		},
		{
			name:     "Other Texas error maps to generic 502 without leaking raw body",
			strategy: strategyOBO,
			upstreamBody: map[string]any{
				"error":             "server_error",
				"error_description": "SUPER_SECRET_INTERNAL_TRACE_ID_12345",
			},
			upstreamStatus: http.StatusInternalServerError,
			wantError:      "upstream_token_error",
			wantMessage:    "failed to acquire upstream team token",
			forbiddenLeak:  "SUPER_SECRET_INTERNAL_TRACE_ID_12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := newTestEnv(t)
			te.gw.cfg.TeamTokenStrategy = tt.strategy
			var err error
			te.gw.TeamTokens, err = newTeamTokenSource(te.gw.cfg, te.gw.store, te.gw.now)
			if err != nil {
				t.Fatalf("newTeamTokenSource failed: %v", err)
			}

			gwToken := te.obtainAccessTokenForResource(te.resource)

			if tt.strategy == strategyOBO {
				te.texasServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					writeJSON(w, tt.upstreamStatus, tt.upstreamBody)
				})
			} else {
				te.entraServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					writeJSON(w, tt.upstreamStatus, tt.upstreamBody)
				})
			}

			req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"method":"tools/list"}`))
			req.Header.Set("Authorization", "Bearer "+gwToken)
			rec := httptest.NewRecorder()
			te.handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadGateway {
				t.Fatalf("expected 502, got %d (%s)", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), tt.forbiddenLeak) {
				t.Fatalf("response leaked raw upstream error body: %s", rec.Body.String())
			}
			var got map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal 502 JSON: %v", err)
			}
			if got["error"] != tt.wantError || got["message"] != tt.wantMessage {
				t.Fatalf("unexpected 502 JSON: got %+v, want error=%q message=%q", got, tt.wantError, tt.wantMessage)
			}
		})
	}
}

func TestHardening_OIDCNonce_PKCE_Singleflight_Prompt_And_Claims(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	t.Run("prompt allowlist and claims JSON/size validation", func(t *testing.T) {
		te := newTestEnv(t)

		// Valid prompt and valid JSON object claims pass on to Entra (302).
		okReq := httptest.NewRequest(http.MethodGet, te.authorizeURL(map[string]string{
			"prompt": "select_account",
			"claims": `{"id_token":{"acrs":{"essential":true}}}`,
		}, verifier), nil)
		okRec := httptest.NewRecorder()
		te.handler.ServeHTTP(okRec, okReq)
		if okRec.Code != http.StatusFound {
			t.Fatalf("expected 302 for valid prompt and claims, got %d", okRec.Code)
		}

		badCases := []struct {
			name      string
			overrides map[string]string
		}{
			{name: "invalid prompt", overrides: map[string]string{"prompt": "admin"}},
			{name: "invalid JSON claims", overrides: map[string]string{"claims": "{not-json"}},
			{name: "non-object JSON claims", overrides: map[string]string{"claims": `["array"]`}},
			{name: "oversized claims", overrides: map[string]string{"claims": fmt.Sprintf(`{"k":"%s"}`, strings.Repeat("x", maxClaimsBytes))}},
		}
		for _, bc := range badCases {
			t.Run(bc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, te.authorizeURL(bc.overrides, verifier), nil)
				rec := httptest.NewRecorder()
				te.handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusFound {
					t.Fatalf("expected 302 redirect error, got %d", rec.Code)
				}
				loc, _ := url.Parse(rec.Header().Get("Location"))
				if loc.Query().Get("error") != "invalid_request" {
					t.Fatalf("expected invalid_request, got %q", loc.Query().Get("error"))
				}
			})
		}
	})

	t.Run("OIDC nonce validation rejects mismatch and missing id_token", func(t *testing.T) {
		for _, mode := range []string{"wrong_nonce", "missing_nonce", "missing_id_token"} {
			t.Run(mode, func(t *testing.T) {
				te := newTestEnv(t)
				te.entraServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/.well-known/openid-configuration":
						writeJSON(w, http.StatusOK, map[string]any{
							"authorization_endpoint": te.entraServer.URL + "/authorize",
							"token_endpoint":         te.entraServer.URL + "/token",
						})
					case "/token":
						resp := map[string]any{
							"access_token": te.entraToken,
							"token_type":   "Bearer",
							"expires_in":   3600,
						}
						switch mode {
						case "wrong_nonce":
							resp["id_token"] = makeTestJWT(map[string]any{"nonce": "wrong-nonce"})
						case "missing_nonce":
							resp["id_token"] = makeTestJWT(map[string]any{"sub": "u1"})
						}
						writeJSON(w, http.StatusOK, resp)
					}
				})

				entraState, stateCookie := te.startLogin(verifier)
				req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=entra-code-123&state="+url.QueryEscape(entraState), nil)
				req.AddCookie(stateCookie)
				rec := httptest.NewRecorder()
				te.handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusFound {
					t.Fatalf("expected 302 error redirect, got %d", rec.Code)
				}
				loc, _ := url.Parse(rec.Header().Get("Location"))
				if loc.Query().Get("error") != "server_error" {
					t.Fatalf("expected server_error for %s, got %q", mode, loc.Query().Get("error"))
				}
			})
		}
	})

	t.Run("Entra well-known metadata fetched once via singleflight", func(t *testing.T) {
		te := newTestEnv(t)
		var wellKnownHits atomic.Int32
		te.entraServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/.well-known/openid-configuration" {
				wellKnownHits.Add(1)
				time.Sleep(30 * time.Millisecond)
				writeJSON(w, http.StatusOK, map[string]any{
					"authorization_endpoint": te.entraServer.URL + "/authorize",
					"token_endpoint":         te.entraServer.URL + "/token",
				})
				return
			}
			http.NotFound(w, r)
		})

		const n = 10
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ep, err := te.gw.entraAuthorizeEndpoint(t.Context())
				if err != nil || ep != te.entraServer.URL+"/authorize" {
					t.Errorf("unexpected endpoint %q err=%v", ep, err)
				}
			}()
		}
		wg.Wait()

		if hits := wellKnownHits.Load(); hits != 1 {
			t.Fatalf("expected 1 singleflight fetch of well-known metadata, got %d", hits)
		}
	})
}

func TestAuditMCP_LogsFields_BoundsMethod_And_NoSecretsOrArgs(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	defer slog.SetDefault(oldLogger)

	te := newTestEnv(t)
	gwToken := te.obtainAccessTokenForResource(te.resource)

	longMethod := strings.Repeat("m", 120)
	secretArg := "super-secret-tool-argument-value"
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":{"name":"my_tool","arguments":{"secret":%q}}}`, longMethod, secretArg)
	req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+gwToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var foundAudit map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) == nil && entry["msg"] == "audit mcp" {
			foundAudit = flatAudit(entry)
		}
	}
	if foundAudit == nil {
		t.Fatalf("expected 'audit mcp' log entry, got logs:\n%s", logBuf.String())
	}
	if foundAudit["navident"] != "Z123456" {
		t.Fatalf("expected navident=Z123456, got %v", foundAudit["navident"])
	}
	if foundAudit["client_id"] != te.clientID {
		t.Fatalf("expected client_id=%q, got %v", te.clientID, foundAudit["client_id"])
	}
	if foundAudit["route"] != "/mcp/team-etterlevelse/etterlevelse" {
		t.Fatalf("expected route=/mcp/team-etterlevelse/etterlevelse, got %v", foundAudit["route"])
	}
	if foundAudit["outcome"] != "ok" {
		t.Fatalf("expected outcome=ok, got %v", foundAudit["outcome"])
	}
	if _, ok := foundAudit["duration_ms"]; !ok {
		t.Fatal("expected duration_ms in audit log")
	}
	loggedMethod, _ := foundAudit["method"].(string)
	if len(loggedMethod) != maxAuditMethodLen+len(`"..."`) || !strings.HasSuffix(loggedMethod, `"...`) {
		t.Fatalf("expected method quoted and cut at %d with a ... marker, got %q", maxAuditMethodLen, loggedMethod)
	}

	allLogs := logBuf.String()
	for _, secret := range []string{
		secretArg,
		gwToken,
		te.entraToken,
		strings.Join([]string{"entra", "test", "refresh"}, "-"),
		strings.Join([]string{"team", "obo", "token"}, "-"),
	} {
		if strings.Contains(allLogs, secret) {
			t.Fatalf("audit/server logs leaked %q: %s", secret, allLogs)
		}
	}
}

func TestProxy_StrictHeaderAllowlists(t *testing.T) {
	te := newTestEnv(t)

	var (
		mu              sync.Mutex
		gotUpstreamHdrs http.Header
		gotUpstreamHost string
	)

	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotUpstreamHdrs = r.Header.Clone()
		gotUpstreamHost = r.Host
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Mcp-Session-Id", "up-sess-1")
		w.Header().Set("Mcp-Protocol-Version", "2025-03-26")
		w.Header().Set("Last-Event-ID", "ev-42")
		w.Header().Set("Set-Cookie", "evil=1; Path=/")
		w.Header().Set("Location", "https://evil.example/")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		w.Header().Set("Content-Security-Policy", "default-src *")
		w.Header().Set("X-Upstream-Internal", "secret")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	})

	gwToken := te.obtainAccessTokenForResource(te.resource)

	req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Host = "spoofed-host.example"
	req.Header.Set("Authorization", "Bearer "+gwToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Session-Id", "cli-sess-1")
	req.Header.Set("Mcp-Protocol-Version", "2025-03-26")
	req.Header.Set("Last-Event-ID", "ev-1")
	req.Header.Set("Cookie", "session=secret")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "evil.example")
	req.Header.Set("Forwarded", "for=1.2.3.4")
	req.Header.Set("X-Arbitrary-Client-Header", "nope")

	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if gotUpstreamHost == "spoofed-host.example" {
		t.Fatalf("upstream Host header was spoofed: %q", gotUpstreamHost)
	}
	for _, want := range []struct{ key, val string }{
		{"Content-Type", "application/json"},
		{"Accept", "application/json, text/event-stream"},
		{"Mcp-Session-Id", "cli-sess-1"},
		{"Mcp-Protocol-Version", "2025-03-26"},
		{"Last-Event-Id", "ev-1"},
	} {
		if got := gotUpstreamHdrs.Get(want.key); got != want.val {
			t.Fatalf("expected allowlisted request header %s=%q, got %q", want.key, want.val, got)
		}
	}
	for _, forbidden := range []string{
		"Cookie",
		"X-Forwarded-For",
		"X-Forwarded-Proto",
		"X-Forwarded-Host",
		"Forwarded",
		"X-Arbitrary-Client-Header",
	} {
		if got := gotUpstreamHdrs.Get(forbidden); got != "" {
			t.Fatalf("forbidden request header %s leaked to upstream: %q", forbidden, got)
		}
	}

	respHdrs := rec.Result().Header
	for _, want := range []struct{ key, val string }{
		{"Content-Type", "application/json"},
		{"Cache-Control", "no-cache"},
		{"Mcp-Session-Id", "up-sess-1"},
		{"Mcp-Protocol-Version", "2025-03-26"},
		{"Last-Event-Id", "ev-42"},
	} {
		if got := respHdrs.Get(want.key); got != want.val {
			t.Fatalf("expected allowlisted response header %s=%q, got %q", want.key, want.val, got)
		}
	}
	for _, forbidden := range []string{
		"Set-Cookie",
		"Location",
		"Access-Control-Allow-Origin",
		"Strict-Transport-Security",
		"Content-Security-Policy",
		"X-Upstream-Internal",
	} {
		if got := respHdrs.Get(forbidden); got != "" {
			t.Fatalf("forbidden response header %s leaked to client: %q", forbidden, got)
		}
	}
}

func TestProxy_SSEBounds_And_NonSSEResponseLimit(t *testing.T) {
	t.Run("SSE stops on max stream duration", func(t *testing.T) {
		te := newTestEnv(t)
		te.gw.cfg.SSEMaxDuration = 80 * time.Millisecond
		te.gw.cfg.SSEIdleTimeout = time.Second
		te.handler = te.gw.routes()

		done := make(chan struct{})
		te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer close(done)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			ticker := time.NewTicker(15 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-r.Context().Done():
					return
				case <-ticker.C:
					if _, err := w.Write([]byte("data: tick\n\n")); err != nil {
						return
					}
					if flusher != nil {
						flusher.Flush()
					}
				}
			}
		})

		gwToken := te.obtainAccessTokenForResource(te.resource)
		ts := httptest.NewServer(te.handler)
		defer ts.Close()

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/mcp/team-etterlevelse/etterlevelse", nil)
		req.Header.Set("Authorization", "Bearer "+gwToken)
		resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
		if err != nil {
			t.Fatalf("SSE request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "data: tick") {
			t.Fatalf("expected at least one SSE event before max duration, got %q", string(body))
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("upstream handler did not stop after max SSE duration")
		}
	})

	t.Run("SSE stops on idle timeout between events", func(t *testing.T) {
		te := newTestEnv(t)
		te.gw.cfg.SSEMaxDuration = time.Second
		te.gw.cfg.SSEIdleTimeout = 60 * time.Millisecond
		te.handler = te.gw.routes()

		done := make(chan struct{})
		te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer close(done)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			_, _ = w.Write([]byte("data: first\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			<-r.Context().Done()
		})

		gwToken := te.obtainAccessTokenForResource(te.resource)
		ts := httptest.NewServer(te.handler)
		defer ts.Close()

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/mcp/team-etterlevelse/etterlevelse", nil)
		req.Header.Set("Authorization", "Bearer "+gwToken)
		resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
		if err != nil {
			t.Fatalf("SSE request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "data: first") {
			t.Fatalf("expected initial event, got %q", string(body))
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("upstream handler did not stop after SSE idle timeout")
		}
	})

	t.Run("SSE stops on max total bytes", func(t *testing.T) {
		te := newTestEnv(t)
		te.gw.cfg.SSEMaxBytes = 64
		te.gw.cfg.SSEMaxDuration = time.Second
		te.gw.cfg.SSEIdleTimeout = time.Second
		te.handler = te.gw.routes()

		te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			for i := 0; i < 20; i++ {
				if r.Context().Err() != nil {
					return
				}
				if _, err := w.Write([]byte("data: 0123456789abcdef\n\n")); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		})

		gwToken := te.obtainAccessTokenForResource(te.resource)
		ts := httptest.NewServer(te.handler)
		defer ts.Close()

		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/mcp/team-etterlevelse/etterlevelse", nil)
		req.Header.Set("Authorization", "Bearer "+gwToken)
		resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
		if err != nil {
			t.Fatalf("SSE request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		if int64(len(body)) > 64 {
			t.Fatalf("expected SSE body capped at 64 bytes, got %d", len(body))
		}
	})

	t.Run("SSE stops when client disconnects", func(t *testing.T) {
		te := newTestEnv(t)
		te.gw.cfg.SSEMaxDuration = 10 * time.Second
		te.gw.cfg.SSEIdleTimeout = 10 * time.Second
		te.handler = te.gw.routes()

		upstreamDone := make(chan struct{})
		te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer close(upstreamDone)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			_, _ = w.Write([]byte("data: hello\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
			<-r.Context().Done()
		})

		gwToken := te.obtainAccessTokenForResource(te.resource)
		ts := httptest.NewServer(te.handler)
		defer ts.Close()

		ctx, cancel := context.WithCancel(t.Context())
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/mcp/team-etterlevelse/etterlevelse", nil)
		req.Header.Set("Authorization", "Bearer "+gwToken)
		resp, err := ts.Client().Do(req) //nolint:gosec // test server URL
		if err != nil {
			cancel()
			t.Fatalf("SSE request failed: %v", err)
		}
		buf := make([]byte, 32)
		_, _ = resp.Body.Read(buf)
		cancel()
		_ = resp.Body.Close()

		select {
		case <-upstreamDone:
		case <-time.After(time.Second):
			t.Fatal("upstream handler did not stop on client disconnect")
		}
	})

	t.Run("non-SSE response exceeding size limit returns 502", func(t *testing.T) {
		te := newTestEnv(t)
		te.gw.cfg.MaxResponseBodyBytes = 32
		te.handler = te.gw.routes()

		te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", "should-not-appear-on-502")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(strings.Repeat("x", 100)))
		})

		gwToken := te.obtainAccessTokenForResource(te.resource)
		req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"method":"tools/list"}`))
		req.Header.Set("Authorization", "Bearer "+gwToken)
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502 for oversized non-SSE response, got %d (%s)", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Mcp-Session-Id") != "" {
			t.Fatalf("expected upstream headers not copied on 502 error, got %q", rec.Header().Get("Mcp-Session-Id"))
		}
	})
}

func TestEntraScopeForTarget(t *testing.T) {
	if got := entraScopeForTarget("dev-gcp:copilot:fleet-echo"); got != "api://dev-gcp.copilot.fleet-echo/.default" {
		t.Fatalf("entraScopeForTarget = %q, want %q", got, "api://dev-gcp.copilot.fleet-echo/.default")
	}
}

func TestProxy_KillSwitch(t *testing.T) {
	setBaseValidEnv(t)
	t.Setenv("GATEWAY_DISABLED", "not-a-bool")
	if _, err := loadConfigFromEnv(); err == nil {
		t.Fatal("expected invalid GATEWAY_DISABLED to return config error")
	}

	tests := []struct {
		name            string
		gatewayDisabled string
		targetPath      string
		wantGlobal      bool
	}{
		{
			name:       "route enabled:false returns 503",
			targetPath: "/mcp/team-sec/disabled",
			wantGlobal: false,
		},
		{
			name:            "GatewayDisabled returns 503",
			gatewayDisabled: "true",
			targetPath:      "/mcp/team-etterlevelse/etterlevelse",
			wantGlobal:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			oldLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
			defer slog.SetDefault(oldLogger)

			te := newTestEnv(t)
			var upstreamCalls atomic.Int32
			te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				upstreamCalls.Add(1)
				writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": "ok"})
			})

			routesYAML := `version: 2
routes:
  - route: /mcp/team-etterlevelse/etterlevelse
    team: team-etterlevelse
    target: dev-gcp:team-etterlevelse:nav-etterlevelse-mcp
    upstream_url: http://nav-etterlevelse-mcp.team-etterlevelse
    passAllTools: true
  - route: /mcp/team-sec/disabled
    team: team-sec
    target: dev-gcp:team-sec:sec-mcp
    upstream_url: http://sec-mcp.team-sec
    passAllTools: true
    enabled: false
`
			parsedRoutes, err := parseAndValidatePilotRoutes([]byte(routesYAML))
			if err != nil {
				t.Fatalf("parseAndValidatePilotRoutes failed: %v", err)
			}
			te.gw.cfg.Routes = parsedRoutes
			if err = validateConfig(te.gw.cfg); err != nil {
				t.Fatalf("validateConfig failed: %v", err)
			}
			parsedUpstream, err := url.Parse(te.upstreamServer.URL)
			if err != nil {
				t.Fatalf("parse upstream server URL: %v", err)
			}
			for i := range te.gw.cfg.Routes {
				te.gw.cfg.Routes[i].ParsedURL = parsedUpstream
			}
			te.gw.TeamTokens, err = newTeamTokenSource(te.gw.cfg, te.gw.store, func() time.Time {
				te.mu.Lock()
				defer te.mu.Unlock()
				return te.now
			})
			if err != nil {
				t.Fatalf("newTeamTokenSource failed: %v", err)
			}

			if tt.gatewayDisabled != "" {
				setBaseValidEnv(t)
				t.Setenv("GATEWAY_DISABLED", tt.gatewayDisabled)
				loadedCfg, err := loadConfigFromEnv()
				if err != nil {
					t.Fatalf("loadConfigFromEnv failed: %v", err)
				}
				te.gw.cfg.GatewayDisabled = loadedCfg.GatewayDisabled
			}
			te.handler = te.gw.routes()

			// Send without Authorization to verify kill switch runs after method check and before auth.
			req := httptest.NewRequest(http.MethodPost, tt.targetPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
			rec := httptest.NewRecorder()
			te.handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d (%s)", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Retry-After"); got != "300" {
				t.Fatalf("expected Retry-After=300, got %q", got)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode 503 JSON: %v", err)
			}
			if body["error"] != "temporarily_unavailable" || body["message"] != "this route is temporarily disabled by the gateway operator" {
				t.Fatalf("unexpected 503 body: %+v", body)
			}
			if upstreamCalls.Load() != 0 {
				t.Fatalf("expected 0 upstream calls, got %d", upstreamCalls.Load())
			}

			var foundWarn, foundAudit map[string]any
			for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
				var entry map[string]any
				if json.Unmarshal([]byte(line), &entry) != nil {
					continue
				}
				switch entry["msg"] {
				case "route disabled by kill switch":
					foundWarn = entry
				case "audit mcp":
					foundAudit = flatAudit(entry)
				}
			}
			if foundWarn == nil {
				t.Fatalf("expected kill switch warning log, got:\n%s", logBuf.String())
			}
			if foundWarn["route"] != tt.targetPath || foundWarn["global"] != tt.wantGlobal {
				t.Fatalf("unexpected warning fields: %+v", foundWarn)
			}
			if foundAudit == nil || foundAudit["outcome"] != "disabled" {
				t.Fatalf("expected audit mcp outcome=disabled, got %+v", foundAudit)
			}
		})
	}
}

func TestProxy_MCPSessionIDBoundToUser(t *testing.T) {
	te := newTestEnv(t)

	var upstreamCalls atomic.Int32
	te.upstreamServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		if r.Header.Get("Mcp-Session-Id") == "" {
			w.Header().Set("Mcp-Session-Id", "mcp-sess-user-a")
		}
		writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": "ok"})
	})

	// User A obtains an Mcp-Session-Id from upstream.
	te.mu.Lock()
	te.navIdent = "A111111"
	te.mu.Unlock()
	tokA := te.obtainAccessTokenForResource(te.resource)

	reqA1 := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	reqA1.Header.Set("Authorization", "Bearer "+tokA)
	recA1 := httptest.NewRecorder()
	te.handler.ServeHTTP(recA1, reqA1)
	if recA1.Code != http.StatusOK {
		t.Fatalf("expected 200 for user A initialize, got %d (%s)", recA1.Code, recA1.Body.String())
	}
	mcpSessID := recA1.Header().Get("Mcp-Session-Id")
	if mcpSessID != "mcp-sess-user-a" {
		t.Fatalf("expected Mcp-Session-Id=mcp-sess-user-a, got %q", mcpSessID)
	}

	// User B with a valid token for the same route sending user A's Mcp-Session-Id gets 403.
	te.mu.Lock()
	te.navIdent = "B222222"
	te.mu.Unlock()
	tokB := te.obtainAccessTokenForResource(te.resource)

	callsBeforeB := upstreamCalls.Load()
	reqB := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	reqB.Header.Set("Authorization", "Bearer "+tokB)
	reqB.Header.Set("Mcp-Session-Id", mcpSessID)
	recB := httptest.NewRecorder()
	te.handler.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for user B using user A's Mcp-Session-Id, got %d (%s)", recB.Code, recB.Body.String())
	}
	var errBody map[string]string
	if err := json.Unmarshal(recB.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decode 403 body: %v", err)
	}
	if errBody["error"] != "forbidden" {
		t.Fatalf("expected error=forbidden, got %+v", errBody)
	}
	if upstreamCalls.Load() != callsBeforeB {
		t.Fatalf("expected user B request to be rejected before upstream, got %d extra calls", upstreamCalls.Load()-callsBeforeB)
	}

	// A second Mcp-Session-Id header cannot smuggle A's id past the check.
	reqDup := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	reqDup.Header.Set("Authorization", "Bearer "+tokB)
	reqDup.Header.Add("Mcp-Session-Id", "fresh-id-b")
	reqDup.Header.Add("Mcp-Session-Id", mcpSessID)
	recDup := httptest.NewRecorder()
	te.handler.ServeHTTP(recDup, reqDup)
	if recDup.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate Mcp-Session-Id headers, got %d", recDup.Code)
	}

	// User A still gets through with that Mcp-Session-Id.
	reqA2 := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`))
	reqA2.Header.Set("Authorization", "Bearer "+tokA)
	reqA2.Header.Set("Mcp-Session-Id", mcpSessID)
	recA2 := httptest.NewRecorder()
	te.handler.ServeHTTP(recA2, reqA2)
	if recA2.Code != http.StatusOK {
		t.Fatalf("expected 200 for user A reusing own Mcp-Session-Id, got %d (%s)", recA2.Code, recA2.Body.String())
	}
	if upstreamCalls.Load() != callsBeforeB+1 {
		t.Fatalf("expected user A request to reach upstream, got %d calls", upstreamCalls.Load()-callsBeforeB)
	}
}

// Two namespaces sharing a route name: the 404 must not point at an arbitrary team's route.
func TestOldRoutePathAmbiguousNameOmitsNewURL(t *testing.T) {
	te := newTestEnv(t)
	dup := te.gw.cfg.Routes[0]
	dup.Route = "/mcp/other-ns/" + dup.Route[strings.LastIndex(dup.Route, "/")+1:]
	dup.FullRouteID = te.gw.cfg.BaseURL + dup.Route
	te.gw.cfg.Routes = append(te.gw.cfg.Routes, dup)

	name := dup.Route[strings.LastIndex(dup.Route, "/")+1:]
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mcp/"+name, nil))
	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusNotFound || body["new_url"] != "" {
		t.Fatalf("want 404 without new_url, got %d %v", rec.Code, body)
	}
}

// The retired /mcp/<name> path is not an alias: 404, with the new URL in the body (#2342 §3.3).
func TestOldRoutePathReturns404WithNewURL(t *testing.T) {
	te := newTestEnv(t)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		req := httptest.NewRequest(method, "/mcp/etterlevelse", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: want 404, got %d", method, rec.Code)
		}
		var body map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if want := te.gw.cfg.BaseURL + "/mcp/team-etterlevelse/etterlevelse"; body["new_url"] != want {
			t.Fatalf("%s: new_url = %q, want %q", method, body["new_url"], want)
		}
	}
}
