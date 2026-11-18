package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func cloneTestEnvWithSharedStore(t *testing.T, base *testEnv, shared SessionStore, nowFn func() time.Time) (*Gateway, http.Handler) {
	t.Helper()
	cfgCopy := *base.gw.cfg
	cfgCopy.AllowedClientIDs = map[string]bool{base.clientID: true}
	cfgCopy.AllowedRedirectURIs = map[string]bool{base.redirectURI: true}
	cfgCopy.Routes = append([]PilotRoute(nil), base.gw.cfg.Routes...)
	cfgCopy.Store = shared

	if err := validateConfig(&cfgCopy); err != nil {
		t.Fatalf("validateConfig clone failed: %v", err)
	}
	parsedUpstream, err := url.Parse(base.upstreamServer.URL)
	if err != nil {
		t.Fatalf("parse upstream URL: %v", err)
	}
	for i := range cfgCopy.Routes {
		cfgCopy.Routes[i].ParsedURL = parsedUpstream
	}

	gw, err := newGateway(&cfgCopy, nowFn)
	if err != nil {
		t.Fatalf("newGateway clone failed: %v", err)
	}
	return gw, gw.routes()
}

func TestGatewayReplicas_SharedStore(t *testing.T) {
	te := newTestEnv(t)
	shared := newMemoryStore(te.gw.now, map[string]int{
		kindPendingConsent: 1024,
		kindPendingEntra:   1024,
		kindCode:           1024,
		kindSession:        4096,
	})
	te.gw.cfg.Store = shared
	gwA, handlerA := cloneTestEnvWithSharedStore(t, te, shared, te.gw.now)
	gwB, handlerB := cloneTestEnvWithSharedStore(t, te, shared, te.gw.now)

	// Point testEnv at replica A to use existing login helpers.
	te.gw = gwA
	te.handler = handlerA

	// 1. Login on replica A -> MCP call on replica B succeeds.
	tok := te.obtainAccessTokenForResource(te.resource)

	reqB := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	reqB.Header.Set("Authorization", "Bearer "+tok)
	recB := httptest.NewRecorder()
	handlerB.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("expected 200 on replica B after login on replica A, got %d (%s)", recB.Code, recB.Body.String())
	}

	// 2. Delete session on replica B -> MCP call on replica A returns 401.
	if err := gwB.store.Delete(context.Background(), kindSession, hashToken(tok)); err != nil {
		t.Fatalf("delete session on B failed: %v", err)
	}
	reqA := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	reqA.Header.Set("Authorization", "Bearer "+tok)
	recA := httptest.NewRecorder()
	handlerA.ServeHTTP(recA, reqA)
	if recA.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on replica A after session deleted on B, got %d", recA.Code)
	}

	// 3. Code popped once across A and B: obtained on A, redeemed on B (200), replay on A fails (400).
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	code := te.obtainAuthCode(verifier)

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {te.clientID},
		"redirect_uri":  {te.redirectURI},
		"resource":      {te.resource},
		"code_verifier": {verifier},
	}
	tokenReqB := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	tokenReqB.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenRecB := httptest.NewRecorder()
	handlerB.ServeHTTP(tokenRecB, tokenReqB)
	if tokenRecB.Code != http.StatusOK {
		t.Fatalf("expected 200 redeeming A's code on B, got %d (%s)", tokenRecB.Code, tokenRecB.Body.String())
	}

	statusReplayA, replayBody := te.postToken(form)
	if statusReplayA != http.StatusBadRequest || replayBody["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant replaying code on A, got %d (%v)", statusReplayA, replayBody)
	}
}

func TestGatewayReplicas_RefreshRotationAcrossReplicas(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.TeamTokenStrategy = strategyRefresh

	var (
		mu        sync.Mutex
		seenRTs   []string
		rtCounter int
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
					"refresh_token": "rt-init",
					"id_token":      makeTestJWT(map[string]any{"NAVident": "Z123456", "nonce": nonce}),
					"expires_in":    3600,
				})
				return
			}
			if r.FormValue("grant_type") == "refresh_token" {
				mu.Lock()
				rtCounter++
				idx := rtCounter
				seenRTs = append(seenRTs, r.FormValue("refresh_token"))
				mu.Unlock()
				writeJSON(w, http.StatusOK, map[string]any{
					"access_token":  fmt.Sprintf("team-at-%d", idx),
					"refresh_token": fmt.Sprintf("rt-next-%d", idx),
					"expires_in":    3600,
				})
				return
			}
		}
	})

	shared := newMemoryStore(te.gw.now, map[string]int{
		kindPendingConsent: 1024,
		kindPendingEntra:   1024,
		kindCode:           1024,
		kindSession:        4096,
	})
	gwA, handlerA := cloneTestEnvWithSharedStore(t, te, shared, te.gw.now)
	_, handlerB := cloneTestEnvWithSharedStore(t, te, shared, te.gw.now)

	te.gw = gwA
	te.handler = handlerA
	tok := te.obtainAccessTokenForResource(te.resource)

	// Call on B rotates rt-init -> rt-next-1 and writes back to shared store.
	reqB := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"method":"tools/list"}`))
	reqB.Header.Set("Authorization", "Bearer "+tok)
	recB := httptest.NewRecorder()
	handlerB.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("expected 200 on B, got %d (%s)", recB.Code, recB.Body.String())
	}

	// Next call on A must see rotated rt-next-1.
	reqA := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"method":"tools/call"}`))
	reqA.Header.Set("Authorization", "Bearer "+tok)
	recA := httptest.NewRecorder()
	handlerA.ServeHTTP(recA, reqA)
	if recA.Code != http.StatusOK {
		t.Fatalf("expected 200 on A, got %d (%s)", recA.Code, recA.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seenRTs) != 2 || seenRTs[0] != "rt-init" || seenRTs[1] != "rt-next-1" {
		t.Fatalf("expected RT rotation across replicas [rt-init rt-next-1], got %v", seenRTs)
	}
}

func TestSessionAbsoluteCap(t *testing.T) {
	te := newTestEnv(t)
	if te.gw.cfg.SessionMaxAge != 12*time.Hour {
		t.Fatalf("expected default SessionMaxAge 12h, got %v", te.gw.cfg.SessionMaxAge)
	}

	// Entra returns a 24h expires_in; gateway must cap at SessionMaxAge (12h).
	te.entraServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(w, http.StatusOK, map[string]any{
				"authorization_endpoint": te.entraServer.URL + "/authorize",
				"token_endpoint":         te.entraServer.URL + "/token",
			})
		case "/token":
			te.mu.Lock()
			nonce := te.lastEntraNonce
			te.mu.Unlock()
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token":  te.entraToken,
				"refresh_token": "rt-cap",
				"id_token":      makeTestJWT(map[string]any{"NAVident": "Z123456", "nonce": nonce}),
				"expires_in":    86400, // 24 hours
			})
		}
	})

	issuedAt := te.gw.now()
	tok := te.obtainAccessTokenForResource(te.resource)

	sess, ok, err := te.gw.getSession(context.Background(), hashToken(tok))
	if err != nil || !ok {
		t.Fatalf("getSession failed: ok=%v err=%v", ok, err)
	}
	wantMaxExpiry := issuedAt.Add(12 * time.Hour)
	if sess.Expiry.After(wantMaxExpiry) {
		t.Fatalf("session expiry %v exceeded issuedAt+SessionMaxAge (%v)", sess.Expiry, wantMaxExpiry)
	}
	if !sess.IssuedAt.Equal(issuedAt) {
		t.Fatalf("expected IssuedAt=%v, got %v", issuedAt, sess.IssuedAt)
	}

	// Even if putSession is called directly with a longer Expiry and TTL, absolute cap holds.
	extended := *sess
	extended.Expiry = issuedAt.Add(48 * time.Hour)
	if putErr := te.gw.putSession(context.Background(), hashToken(tok), &extended, 48*time.Hour); putErr != nil {
		t.Fatalf("putSession failed: %v", putErr)
	}
	reloaded, ok, err := te.gw.getSession(context.Background(), hashToken(tok))
	if err != nil || !ok {
		t.Fatalf("getSession after put failed: ok=%v err=%v", ok, err)
	}
	if reloaded.Expiry.After(wantMaxExpiry) {
		t.Fatalf("putSession allowed expiry %v past issuedAt+SessionMaxAge (%v)", reloaded.Expiry, wantMaxExpiry)
	}

	// Advance fake clock to issuedAt + 12h - 1s: still valid.
	te.advance(12*time.Hour - time.Second)
	okReq := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{}`))
	okReq.Header.Set("Authorization", "Bearer "+tok)
	okRec := httptest.NewRecorder()
	te.handler.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("expected 200 just before absolute cap, got %d", okRec.Code)
	}

	// Advance to issuedAt + 12h + 1s: rejected with 401.
	te.advance(2 * time.Second)
	expReq := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{}`))
	expReq.Header.Set("Authorization", "Bearer "+tok)
	expRec := httptest.NewRecorder()
	te.handler.ServeHTTP(expRec, expReq)
	if expRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 past issuedAt+SessionMaxAge, got %d", expRec.Code)
	}
}

type failingStore struct {
	SessionStore
	failGet        error
	failPut        error
	failPop        error
	failDelete     error
	failRevokeOnce error
	failRevoke     error
	failPing       error
}

func (f *failingStore) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	if f.failRevoke != nil {
		return f.failRevoke
	}
	return f.SessionStore.Revoke(ctx, jti, expiresAt)
}

func (f *failingStore) Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error {
	if f.failPut != nil {
		return f.failPut
	}
	return f.SessionStore.Put(ctx, kind, key, val, ttl)
}

func (f *failingStore) Get(ctx context.Context, kind, key string) ([]byte, bool, error) {
	if f.failGet != nil {
		return nil, false, f.failGet
	}
	return f.SessionStore.Get(ctx, kind, key)
}

func (f *failingStore) Pop(ctx context.Context, kind, key string) ([]byte, bool, error) {
	if f.failPop != nil {
		return nil, false, f.failPop
	}
	return f.SessionStore.Pop(ctx, kind, key)
}

func (f *failingStore) Delete(ctx context.Context, kind, key string) error {
	if f.failDelete != nil {
		return f.failDelete
	}
	return f.SessionStore.Delete(ctx, kind, key)
}

func (f *failingStore) RevokeOnce(ctx context.Context, jti string, expiresAt time.Time) (bool, error) {
	if f.failRevokeOnce != nil {
		return false, f.failRevokeOnce
	}
	return f.SessionStore.RevokeOnce(ctx, jti, expiresAt)
}

func (f *failingStore) Ping(ctx context.Context) error {
	if f.failPing != nil {
		return f.failPing
	}
	return f.SessionStore.Ping(ctx)
}

func TestStoreError_Returns503FailClosed(t *testing.T) {
	te := newTestEnv(t)
	tok := te.obtainAccessTokenForResource(te.resource)

	te.gw.store = &failingStore{
		SessionStore: te.gw.store,
		failGet:      errors.New("db connection refused"),
		failPut:      errors.New("db connection refused"),
		failPop:      errors.New("db connection refused"),
	}

	// Proxy session lookup error must return 503 JSON {"error":"temporarily_unavailable"}, NOT 401.
	req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on store Get error, got %d (%s)", rec.Code, rec.Body.String())
	}
	var errBody map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("expected JSON body on 503, got %q: %v", rec.Body.String(), err)
	}
	if errBody["error"] != "temporarily_unavailable" {
		t.Fatalf("expected error=temporarily_unavailable, got %v", errBody)
	}

	// /authorize store Put error must return 503.
	authReq := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"), nil)
	authRec := httptest.NewRecorder()
	te.handler.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on /authorize store error, got %d", authRec.Code)
	}

	// /token store Pop error must return 503.
	tokForm := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"some-code"},
		"client_id":     {te.clientID},
		"redirect_uri":  {te.redirectURI},
		"code_verifier": {"abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"},
	}
	status, tokErr := te.postToken(tokForm)
	if status != http.StatusServiceUnavailable || tokErr["error"] != "temporarily_unavailable" {
		t.Fatalf("expected 503 temporarily_unavailable on /token store error, got %d (%v)", status, tokErr)
	}
}

func TestReady_StorePingFailureReturns503(t *testing.T) {
	te := newTestEnv(t)
	te.gw.store = &failingStore{
		SessionStore: te.gw.store,
		failPing:     errors.New("db connection refused"),
	}

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on /ready store Ping error, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON body on 503, got %q: %v", rec.Body.String(), err)
	}
	if body["status"] != "not_ready" {
		t.Fatalf("expected status=not_ready, got %v", body)
	}
}

func TestConfig_SessionStoreValidationAndSealing(t *testing.T) {
	validKey := "k1:" + testKeyBase64(t, false)

	tests := []struct {
		name      string
		override  map[string]string
		wantError string
	}{
		{
			name:      "default memory store without RT_KEYS succeeds",
			override:  map[string]string{},
			wantError: "",
		},
		{
			name:      "invalid SESSION_STORE fails closed",
			override:  map[string]string{"SESSION_STORE": "redis"},
			wantError: "invalid SESSION_STORE",
		},
		{
			name: "postgres requires DATABASE_URL",
			override: map[string]string{
				"SESSION_STORE": "postgres",
				"RT_KEYS":       validKey,
			},
			wantError: "invalid DATABASE_URL",
		},
		{
			name: "postgres rejects non-postgres URL scheme",
			override: map[string]string{
				"SESSION_STORE": "postgres",
				"DATABASE_URL":  "http://localhost:5432/db",
				"RT_KEYS":       validKey,
			},
			wantError: "unsupported scheme",
		},
		{
			name: "postgres requires RT_KEYS",
			override: map[string]string{
				"SESSION_STORE": "postgres",
				"DATABASE_URL":  "postgres://localhost:5432/fgw",
			},
			wantError: "RT_KEYS is required",
		},
		{
			name: "postgres with postgresql scheme and RT_KEYS succeeds",
			override: map[string]string{
				"SESSION_STORE":   "postgres",
				"DATABASE_URL":    "postgresql://localhost:5432/fgw",
				"RT_KEYS":         validKey,
				"SESSION_MAX_AGE": "6h",
			},
			wantError: "",
		},
		{
			name: "invalid RT_KEYS fails even for memory store",
			override: map[string]string{
				"SESSION_STORE": "memory",
				"RT_KEYS":       "bad-format",
			},
			wantError: "invalid RT_KEYS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBaseValidEnv(t)
			for k, v := range tt.override {
				t.Setenv(k, v)
			}
			cfg, err := loadConfigFromEnv()
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if tt.override["SESSION_MAX_AGE"] == "6h" && cfg.SessionMaxAge != 6*time.Hour {
					t.Fatalf("expected SessionMaxAge 6h, got %v", cfg.SessionMaxAge)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}

	// Verify memory store wrapping behavior with RT_KEYS.
	te := newTestEnv(t)
	if _, isSealed := te.gw.store.(*sealedStore); isSealed {
		t.Fatal("memory store without RT_KEYS must not be wrapped in sealedStore")
	}
	sealedCfg := *te.gw.cfg
	sealedCfg.RTKeys = validKey
	sealedGW, err := newGateway(&sealedCfg, te.gw.now)
	if err != nil {
		t.Fatalf("newGateway with RT_KEYS failed: %v", err)
	}
	defer sealedGW.store.Close()
	if _, isSealed := sealedGW.store.(*sealedStore); !isSealed {
		t.Fatal("memory store with RT_KEYS set must be wrapped in sealedStore")
	}
}

func TestSealedSessionUnknownKID_Returns401(t *testing.T) {
	te := newTestEnv(t)
	inner := newMemoryStore(te.gw.now, map[string]int{
		kindPendingConsent: 1024,
		kindPendingEntra:   1024,
		kindCode:           1024,
		kindSession:        4096,
	})

	kr1, err := parseKeyRing("k1:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatalf("parseKeyRing k1 failed: %v", err)
	}
	kr2, err := parseKeyRing("k2:" + testKeyBase64(t, true))
	if err != nil {
		t.Fatalf("parseKeyRing k2 failed: %v", err)
	}

	gw1, handler1 := cloneTestEnvWithSharedStore(t, te, newSealedStore(inner, kr1), te.gw.now)
	te.gw = gw1
	te.handler = handler1
	tok := te.obtainAccessTokenForResource(te.resource)

	_, handler2 := cloneTestEnvWithSharedStore(t, te, newSealedStore(inner, kr2), te.gw.now)
	req := httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	handler2.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when session kid is absent from key ring, got %d (%s)", rec.Code, rec.Body.String())
	}
	var errBody map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("expected JSON body on 401, got %q: %v", rec.Body.String(), err)
	}
	if errBody["error"] != "invalid_token" {
		t.Fatalf("expected error=invalid_token, got %v", errBody)
	}
	if _, ok, err := inner.Get(context.Background(), kindSession, hashToken(tok)); err != nil || !ok {
		t.Fatalf("expected row with unknown kid to be kept for key rollout, got ok=%v err=%v", ok, err)
	}
}
