package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Generated per run so no key-shaped literal lives in the source.
var testLockoutKey = rand.Text() + rand.Text() // 52 chars; LOCKOUT_KEY needs >= 32

func withLockoutKey(t *testing.T, te *testEnv) *testEnv {
	t.Helper()
	te.gw.store.Close()

	cfgCopy := *te.gw.cfg
	cfgCopy.Routes = append([]PilotRoute(nil), te.gw.cfg.Routes...)
	cfgCopy.LockoutKey = testLockoutKey
	if err := validateConfig(&cfgCopy); err != nil {
		t.Fatalf("validateConfig failed: %v", err)
	}
	parsedUpstream, err := url.Parse(te.upstreamServer.URL)
	if err != nil {
		t.Fatalf("parse upstream URL: %v", err)
	}
	for i := range cfgCopy.Routes {
		cfgCopy.Routes[i].ParsedURL = parsedUpstream
	}
	gw, err := newGateway(&cfgCopy, te.gw.now)
	if err != nil {
		t.Fatalf("newGateway failed: %v", err)
	}
	t.Cleanup(gw.store.Close)
	te.gw = gw
	te.handler = gw.routes()
	return te
}

func TestLockout_SessionRejectedForTargetUserOnly(t *testing.T) {
	ctx := context.Background()
	te := withLockoutKey(t, newTestEnv(t))

	te.navIdent = "Z111111"
	tokenA := te.obtainAccessTokenForResource(te.resource)

	te.navIdent = "Z222222"
	tokenB := te.obtainAccessTokenForResource(te.resource)

	if rec := mcpPost(te, tokenA, ""); rec.Code != http.StatusOK {
		t.Fatalf("user A before lockout: got %d, want 200", rec.Code)
	}
	if rec := mcpPost(te, tokenB, ""); rec.Code != http.StatusOK {
		t.Fatalf("user B before lockout: got %d, want 200", rec.Code)
	}

	if _, err := te.gw.lockOutUser(ctx, "Z111111", "op1"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}

	if rec := mcpPost(te, tokenA, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("user A after lockout: got %d, want 401", rec.Code)
	}
	// Session row should have been deleted on lookup.
	if _, ok, err := te.gw.store.Get(ctx, kindSession, hashToken(tokenA)); err != nil || ok {
		t.Fatalf("expected locked-out session row to be deleted: ok=%v err=%v", ok, err)
	}

	// Another user's session still works.
	if rec := mcpPost(te, tokenB, ""); rec.Code != http.StatusOK {
		t.Fatalf("user B after A locked out: got %d, want 200", rec.Code)
	}

	// While blocked (before undo), even a session or login in a later second is rejected.
	te.advance(time.Second)
	te.navIdent = "Z111111"
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	codeAfter := te.obtainAuthCode(verifier)
	status, body := te.postToken(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {codeAfter},
		"client_id":     {te.clientID},
		"redirect_uri":  {te.redirectURI},
		"resource":      {te.resource},
		"code_verifier": {verifier},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" || body["error_description"] != "session revoked" {
		t.Fatalf("login while blocked: got %d %v, want 400 invalid_grant (session revoked)", status, body)
	}
	postLockoutHash := hashToken("post-lockout-session")
	if err := te.gw.putSession(ctx, postLockoutHash, &gatewaySession{
		ClientID: te.clientID,
		Resource: te.resource,
		NAVIdent: "Z111111",
		IssuedAt: te.now,
		Expiry:   te.now.Add(time.Hour),
	}, time.Hour); err != nil {
		t.Fatalf("putSession: %v", err)
	}
	if _, ok, err := te.gw.getSession(ctx, postLockoutHash); err != nil || ok {
		t.Fatalf("session issued after lockout second while blocked: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}

func TestLockout_CodeExchangeRejected(t *testing.T) {
	ctx := context.Background()
	te := withLockoutKey(t, newTestEnv(t))

	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	code := te.obtainAuthCode(verifier)

	if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op1"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}

	status, body := te.postToken(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {te.clientID},
		"redirect_uri":  {te.redirectURI},
		"resource":      {te.resource},
		"code_verifier": {verifier},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" || body["error_description"] != "session revoked" {
		t.Fatalf("pre-lockout code exchange: got %d %v, want 400 invalid_grant (session revoked)", status, body)
	}
}

func TestLockout_RefreshTokenRejectedForTargetUserOnly(t *testing.T) {
	ctx := context.Background()
	rtKeys := "k1:" + testKeyBase64(t, false)
	te := withLockoutKey(t, newClientEncryptedTestEnv(t, rtKeys))

	te.navIdent = "Z111111"
	_, rtA, _ := te.obtainTokensForResource(te.resource)

	te.navIdent = "Z222222"
	_, rtB, _ := te.obtainTokensForResource(te.resource)

	if _, err := te.gw.lockOutUser(ctx, "Z111111", "op1"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}

	status, body := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {te.clientID},
		"refresh_token": {rtA},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" || body["error_description"] != "session revoked" {
		t.Fatalf("pre-lockout RT refresh: got %d %v, want 400 invalid_grant (session revoked)", status, body)
	}

	statusB, bodyB := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {te.clientID},
		"refresh_token": {rtB},
	})
	if statusB != http.StatusOK || bodyB["access_token"] == "" {
		t.Fatalf("other user's RT refresh: got %d %v, want 200", statusB, bodyB)
	}
}

func TestLockout_RememberedConsentDeletedAndLegacyHonoured(t *testing.T) {
	ctx := context.Background()

	t.Run("pre-lockout remembered consent is deleted and asks again", func(t *testing.T) {
		te := withLockoutKey(t, newTestEnv(t))
		_ = te.obtainAuthCode(consentVerifier) // approves and remembers consent
		if te.asksConsent() {
			t.Fatal("expected remembered consent before lockout")
		}

		if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op1"); err != nil {
			t.Fatalf("lockOutUser: %v", err)
		}

		if !te.asksConsent() {
			t.Fatal("expected consent prompt after lockout")
		}
		if _, ok, err := te.gw.store.Get(ctx, kindConsent, consentKey(te.navIdent, te.clientID, te.resource)); err != nil || ok {
			t.Fatalf("expected pre-lockout consent row to be deleted: ok=%v err=%v", ok, err)
		}
	})

	t.Run("legacy consent value 1 still honoured when no lockout exists", func(t *testing.T) {
		te := withLockoutKey(t, newTestEnv(t))
		key := consentKey(te.navIdent, te.clientID, te.resource)
		if err := te.gw.store.Put(ctx, kindConsent, key, []byte("1"), te.gw.cfg.ConsentTTL); err != nil {
			t.Fatalf("put legacy consent: %v", err)
		}
		if te.asksConsent() {
			t.Fatal("expected legacy '1' consent to be honoured without lockout")
		}

		if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op1"); err != nil {
			t.Fatalf("lockOutUser: %v", err)
		}
		if !te.asksConsent() {
			t.Fatal("expected legacy '1' consent to be rejected after lockout")
		}
		if _, ok, err := te.gw.store.Get(ctx, kindConsent, key); err != nil || ok {
			t.Fatalf("expected legacy consent row to be deleted after lockout: ok=%v err=%v", ok, err)
		}
	})
}

func TestLockout_MCPSessionUserKeyHashed(t *testing.T) {
	te := newTestEnv(t)
	issued := "up-hashed-1"
	setUpstreamSessionID(te, &issued)

	token := te.obtainAccessTokenForResource(te.resource)
	if rec := mcpPost(te, token, ""); rec.Code != http.StatusOK {
		t.Fatalf("mcpPost: got %d (%s)", rec.Code, rec.Body.String())
	}

	if _, ok, err := te.gw.store.Get(context.Background(), kindMCPSessionUser, te.navIdent); err != nil || ok {
		t.Fatalf("kindMCPSessionUser must not be keyed by plaintext NAVident: ok=%v err=%v", ok, err)
	}
	if got := mcpUserCount(te); got != "1" {
		t.Fatalf("kindMCPSessionUser under hashed key = %q, want 1", got)
	}
}

func TestLockout_Postgres(t *testing.T) {
	pgURL := os.Getenv("FGW_TEST_PG_URL")
	if pgURL == "" {
		t.Skip("FGW_TEST_PG_URL not set")
	}

	ctx := context.Background()
	clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	pgStore := newTestPostgresStore(t, pgURL, clk.Now, map[string]int{kindSession: 10})
	defer pgStore.Close()

	kr, err := parseKeyRing("k1:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatalf("parseKeyRing: %v", err)
	}
	cfg := &Config{
		SessionMaxAge: defaultSessionMaxAge,
		ConsentTTL:    defaultConsentTTL,
		Store:         newSealedStore(pgStore, kr),
		LockoutKey:    testLockoutKey,
	}
	gw := &Gateway{
		cfg:        cfg,
		now:        clk.Now,
		store:      cfg.Store,
		rtKeys:     kr,
		lockoutKey: []byte(testLockoutKey),
	}

	tokHash := hashToken("pg-session-token")
	sess := &gatewaySession{
		ClientID: "c1",
		Resource: "https://fleet-gateway.intern.dev.nav.no/mcp/ns/app",
		NAVIdent: "Z123456",
		IssuedAt: clk.Now(),
		Expiry:   clk.Now().Add(time.Hour),
	}
	if err := gw.putSession(ctx, tokHash, sess, time.Hour); err != nil {
		t.Fatalf("putSession: %v", err)
	}
	if _, ok, err := gw.getSession(ctx, tokHash); err != nil || !ok {
		t.Fatalf("getSession before lockout: ok=%v err=%v", ok, err)
	}

	if _, err := gw.lockOutUser(ctx, "Z123456", "op-pg"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}
	if _, ok, err := gw.getSession(ctx, tokHash); err != nil || ok {
		t.Fatalf("getSession after lockout: ok=%v err=%v, want ok=false err=nil", ok, err)
	}

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	gw.drainLockoutAudit(ctx)
	lines := parseJSONLogLines(t, buf.Bytes())
	if len(lines) != 1 {
		t.Fatalf("expected 1 audit log line after Postgres drain, got %d (%s)", len(lines), buf.String())
	}
	if lines[0]["msg"] != "audit lockout" || lines[0]["action"] != "lock" || lines[0]["operator"] != "op-pg" {
		t.Fatalf("unexpected Postgres audit line: %v", lines[0])
	}
	buf.Reset()
	gw.drainLockoutAudit(ctx)
	if buf.Len() != 0 {
		t.Fatalf("expected second Postgres drain to log nothing, got %s", buf.String())
	}
}

func TestLockout_Undo(t *testing.T) {
	ctx := context.Background()
	rtKeys := "k1:" + testKeyBase64(t, false)
	te := withLockoutKey(t, newClientEncryptedTestEnv(t, rtKeys))

	issuedBefore := te.now
	tokBefore, rtBefore, _ := te.obtainTokensForResource(te.resource)
	if te.asksConsent() {
		t.Fatal("expected remembered consent before lockout")
	}

	if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op1"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}
	if revoked, err := te.gw.userRevoked(ctx, te.navIdent, issuedBefore); err != nil || !revoked {
		t.Fatalf("userRevoked during lockout: revoked=%v err=%v, want true nil", revoked, err)
	}
	if revoked, err := te.gw.userRevoked(ctx, te.navIdent, issuedBefore.Add(time.Second)); err != nil || !revoked {
		t.Fatalf("userRevoked for later timestamp during lockout: revoked=%v err=%v, want true nil", revoked, err)
	}

	if err := te.gw.undoLockout(ctx, te.navIdent, "op2"); err != nil {
		t.Fatalf("undoLockout: %v", err)
	}
	if revoked, err := te.gw.userRevoked(ctx, te.navIdent, issuedBefore); err != nil || !revoked {
		t.Fatalf("userRevoked for pre-lockout timestamp after undo: revoked=%v err=%v, want true nil", revoked, err)
	}
	if _, ok, err := te.gw.getSession(ctx, hashToken(tokBefore)); err != nil || ok {
		t.Fatalf("pre-lockout getSession after undo: ok=%v err=%v, want false nil", ok, err)
	}
	status, body := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {te.clientID},
		"refresh_token": {rtBefore},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" || body["error_description"] != "session revoked" {
		t.Fatalf("pre-lockout refresh after undo: got %d %v, want 400 invalid_grant (session revoked)", status, body)
	}
	if !te.asksConsent() {
		t.Fatal("expected pre-lockout remembered consent to stay rejected after undo")
	}

	te.advance(time.Second)
	tokAfter := te.obtainAccessTokenForResource(te.resource)
	if rec := mcpPost(te, tokAfter, ""); rec.Code != http.StatusOK {
		t.Fatalf("new session after undo: got %d, want 200", rec.Code)
	}
}

func TestLockout_UndoNoOpWhenNoRow(t *testing.T) {
	ctx := context.Background()
	te := withLockoutKey(t, newTestEnv(t))

	if err := te.gw.undoLockout(ctx, te.navIdent, "op-noop"); err != nil {
		t.Fatalf("undoLockout with no row: %v", err)
	}
	if _, ok, err := te.gw.store.Get(ctx, kindUserRevoked, te.gw.userKey(te.navIdent)); err != nil || ok {
		t.Fatalf("expected no row written on no-op undo: ok=%v err=%v", ok, err)
	}
}

func TestLockout_LegacyPlainIntRecord(t *testing.T) {
	ctx := context.Background()
	te := withLockoutKey(t, newTestEnv(t))

	lockSec := te.now.Unix()
	key := te.gw.userKey(te.navIdent)
	if err := te.gw.store.Put(ctx, kindUserRevoked, key, []byte("1700000000"), lockoutNoExpiry); err != nil {
		t.Fatalf("put legacy lockout row: %v", err)
	}
	// Even a timestamp far after 1700000000 is blocked because legacy rows parse as blocked=true.
	if revoked, err := te.gw.userRevoked(ctx, te.navIdent, time.Unix(lockSec, 0).UTC()); err != nil || !revoked {
		t.Fatalf("legacy plain-int row should block later timestamp: revoked=%v err=%v", revoked, err)
	}

	if err := te.gw.undoLockout(ctx, te.navIdent, "op-legacy"); err != nil {
		t.Fatalf("undoLockout on legacy row: %v", err)
	}
	if revoked, err := te.gw.userRevoked(ctx, te.navIdent, time.Unix(1700000000, 0).UTC()); err != nil || !revoked {
		t.Fatalf("legacy timestamp <= N should remain revoked after undo: revoked=%v err=%v", revoked, err)
	}
	if revoked, err := te.gw.userRevoked(ctx, te.navIdent, time.Unix(1700000001, 0).UTC()); err != nil || revoked {
		t.Fatalf("timestamp > N should be allowed after undo: revoked=%v err=%v", revoked, err)
	}
}

func TestLockout_ConfigValidation(t *testing.T) {
	t.Run("unset LOCKOUT_KEY allowed", func(t *testing.T) {
		setBaseValidEnv(t)
		t.Setenv("LOCKOUT_KEY", "")
		cfg, err := loadConfigFromEnv()
		if err != nil {
			t.Fatalf("loadConfigFromEnv: %v", err)
		}
		if cfg.LockoutKey != "" {
			t.Fatalf("got LockoutKey=%q, want empty", cfg.LockoutKey)
		}
	})

	t.Run("short LOCKOUT_KEY rejected", func(t *testing.T) {
		setBaseValidEnv(t)
		t.Setenv("LOCKOUT_KEY", "too-short")
		if _, err := loadConfigFromEnv(); err == nil || !strings.Contains(err.Error(), "LOCKOUT_KEY") {
			t.Fatalf("expected LOCKOUT_KEY length error, got %v", err)
		}
	})

	t.Run("valid LOCKOUT_KEY trimmed", func(t *testing.T) {
		setBaseValidEnv(t)
		t.Setenv("LOCKOUT_KEY", "  "+testLockoutKey+"  ")
		cfg, err := loadConfigFromEnv()
		if err != nil {
			t.Fatalf("loadConfigFromEnv: %v", err)
		}
		if cfg.LockoutKey != testLockoutKey {
			t.Fatalf("got LockoutKey=%q, want %q", cfg.LockoutKey, testLockoutKey)
		}
	})
}

func TestRunLockout_CLIArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantUndo     bool
		wantOperator string
		wantIdent    string
		wantErr      bool
	}{
		{name: "missing operator none", args: nil, wantErr: true},
		{name: "missing operator with ident", args: []string{"z123456"}, wantErr: true},
		{name: "missing operator with undo", args: []string{"--undo", "z123456"}, wantErr: true},
		{name: "empty operator", args: []string{"--operator", "", "z123456"}, wantErr: true},
		{name: "whitespace operator", args: []string{"--operator", "   ", "z123456"}, wantErr: true},
		{name: "no ident", args: []string{"--operator", "op1"}, wantErr: true},
		{name: "blank ident", args: []string{"--operator", "op1", "  "}, wantErr: true},
		{name: "two idents", args: []string{"--operator", "op1", "a", "b"}, wantErr: true},
		{name: "dash x flag", args: []string{"-x"}, wantErr: true},
		{name: "dash x with operator", args: []string{"--operator", "op1", "-x"}, wantErr: true},
		{name: "dash x after double dash", args: []string{"--operator", "op1", "--", "-x"}, wantErr: true},
		{name: "valid lock", args: []string{"--operator", " op1 ", " z123456 "}, wantUndo: false, wantOperator: "op1", wantIdent: "Z123456"},
		{name: "valid undo", args: []string{"--undo", "--operator", "op1", " z123456 "}, wantUndo: true, wantOperator: "op1", wantIdent: "Z123456"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			undo, operator, ident, err := parseLockoutArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseLockoutArgs(%v): got (%v, %q, %q, nil), want error", tc.args, undo, operator, ident)
				}
				return
			}
			if err != nil || undo != tc.wantUndo || operator != tc.wantOperator || ident != tc.wantIdent {
				t.Fatalf("parseLockoutArgs(%v): got (%v, %q, %q, %v), want (%v, %q, %q, nil)",
					tc.args, undo, operator, ident, err, tc.wantUndo, tc.wantOperator, tc.wantIdent)
			}
		})
	}

	if code := runLockout(nil); code == 0 {
		t.Fatal("expected non-zero exit code for invalid args")
	}

	setBaseValidEnv(t)
	t.Setenv("LOCKOUT_KEY", testLockoutKey)
	if code := runLockout([]string{"--operator", "op1", "z123456"}); code == 0 {
		t.Fatal("expected non-zero exit code with memory session store")
	}
}

func TestCheckLockoutConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name:    "memory store rejected",
			cfg:     &Config{SessionStore: "memory", LockoutKey: testLockoutKey},
			wantErr: "SESSION_STORE must be postgres",
		},
		{
			name:    "empty store rejected",
			cfg:     &Config{SessionStore: "", LockoutKey: testLockoutKey},
			wantErr: "SESSION_STORE must be postgres",
		},
		{
			name:    "empty LockoutKey rejected",
			cfg:     &Config{SessionStore: "postgres", LockoutKey: ""},
			wantErr: "LOCKOUT_KEY is not set",
		},
		{
			name: "postgres and key accepted",
			cfg:  &Config{SessionStore: "postgres", LockoutKey: testLockoutKey},
		},
		{
			name: "trimmed uppercase postgres and key accepted",
			cfg:  &Config{SessionStore: " POSTGRES ", LockoutKey: testLockoutKey},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkLockoutConfig(tc.cfg)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("checkLockoutConfig(%+v) = %v, want error containing %q", tc.cfg, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("checkLockoutConfig(%+v) unexpected error: %v", tc.cfg, err)
			}
		})
	}
}

func parseJSONLogLines(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("unmarshal log line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func TestLockout_DrainAudit(t *testing.T) {
	ctx := context.Background()
	te := withLockoutKey(t, newTestEnv(t))

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op-lock\n"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}
	if err := te.gw.undoLockout(ctx, te.navIdent, "op-undo"); err != nil {
		t.Fatalf("undoLockout: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected lockOutUser/undoLockout not to log directly, got %s", buf.String())
	}

	te.gw.drainLockoutAudit(ctx)
	lines := parseJSONLogLines(t, buf.Bytes())
	if len(lines) != 2 {
		t.Fatalf("expected 2 drained audit lines, got %d (%s)", len(lines), buf.String())
	}

	wantKey := te.gw.userKey(te.navIdent)
	for i, want := range []struct {
		action   string
		operator string
	}{
		{action: "lock", operator: "op-lock"},
		{action: "undo", operator: "op-undo"},
	} {
		got := lines[i]
		if got["msg"] != "audit lockout" || got["outcome"] != "ok" || got["action"] != want.action ||
			got["user_key"] != wantKey || got["operator"] != want.operator || got["at"] == nil {
			t.Fatalf("line %d mismatch: got %v, want action=%s operator=%s user_key=%s", i, got, want.action, want.operator, wantKey)
		}
	}

	buf.Reset()
	te.gw.drainLockoutAudit(ctx)
	if buf.Len() != 0 {
		t.Fatalf("expected second drain to log nothing, got %s", buf.String())
	}
}

func TestLockout_SweepAllDrainsAudit(t *testing.T) {
	ctx := context.Background()
	te := withLockoutKey(t, newTestEnv(t))

	if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op1"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	te.gw.sweepAll()

	got := buf.String()
	if !strings.Contains(got, "audit lockout") || !strings.Contains(got, `"action":"lock"`) {
		t.Fatalf("expected sweepAll to drain lockout audit log, got %s", got)
	}
}

func TestLockout_NoExpiryAfter400Days(t *testing.T) {
	ctx := context.Background()
	te := withLockoutKey(t, newTestEnv(t))

	issuedAt := te.now
	if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op1"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}

	te.advance(400 * 24 * time.Hour)
	te.gw.sweepAll()

	if _, ok, err := te.gw.store.Get(ctx, kindUserRevoked, te.gw.userKey(te.navIdent)); err != nil || !ok {
		t.Fatalf("lockout record missing after 400 days: ok=%v err=%v", ok, err)
	}
	if _, ok, err := te.gw.store.Get(ctx, kindUserRevokedMarker, userRevokedMarkerKey); err != nil || !ok {
		t.Fatalf("lockout marker missing after 400 days: ok=%v err=%v", ok, err)
	}
	if revoked, err := te.gw.userRevoked(ctx, te.navIdent, issuedAt); err != nil || !revoked {
		t.Fatalf("userRevoked after 400 days: revoked=%v err=%v, want true nil", revoked, err)
	}
}

type failingGetStore struct {
	SessionStore
	err error
}

func (s *failingGetStore) Get(ctx context.Context, kind, key string) ([]byte, bool, error) {
	if kind == kindUserRevokedMarker {
		return nil, false, s.err
	}
	return s.SessionStore.Get(ctx, kind, key)
}

func TestLockout_NewGatewayMarkerCheck(t *testing.T) {
	ctx := context.Background()
	te := newTestEnv(t)
	shared := newMemoryStore(te.gw.now, nil)

	baseCfg := *te.gw.cfg
	baseCfg.Routes = append([]PilotRoute(nil), te.gw.cfg.Routes...)
	baseCfg.Store = shared
	baseCfg.LockoutKey = ""

	// Succeeds when LockoutKey is empty and no marker exists.
	gwNoKey, err := newGateway(&baseCfg, te.gw.now)
	if err != nil {
		t.Fatalf("newGateway without marker: %v", err)
	}
	if revoked, revErr := gwNoKey.userRevoked(ctx, te.navIdent, te.now); revErr != nil || revoked {
		t.Fatalf("userRevoked without LockoutKey: revoked=%v err=%v, want false nil", revoked, revErr)
	}

	// Lock out a user using a gateway with LockoutKey set against the shared store.
	withKeyCfg := baseCfg
	withKeyCfg.LockoutKey = testLockoutKey
	gwWithKey, err := newGateway(&withKeyCfg, te.gw.now)
	if err != nil {
		t.Fatalf("newGateway with LockoutKey: %v", err)
	}
	if _, err := gwWithKey.lockOutUser(ctx, te.navIdent, "op1"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}

	// Now newGateway with empty LockoutKey must fail because the marker exists.
	const wantMsg = "LOCKOUT_KEY is missing but lockout records exist; set the LOCKOUT_KEY secret"
	if _, err := newGateway(&baseCfg, te.gw.now); err == nil || err.Error() != wantMsg {
		t.Fatalf("newGateway with marker and empty LockoutKey: got %v, want %q", err, wantMsg)
	}

	// Store errors on marker Get also fail startup.
	errCfg := baseCfg
	errCfg.Store = &failingGetStore{SessionStore: newMemoryStore(te.gw.now, nil), err: errors.New("boom")}
	if _, err := newGateway(&errCfg, te.gw.now); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("newGateway with failing marker Get: got %v, want error containing boom", err)
	}
}

func TestLockout_SurvivesRTKeyReplacement(t *testing.T) {
	ctx := context.Background()
	clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	inner := newMemoryStore(clk.Now, nil)

	krA, err := parseKeyRing("k1:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatalf("parseKeyRing A: %v", err)
	}
	krB, err := parseKeyRing("k2:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatalf("parseKeyRing B: %v", err)
	}

	storeA := newSealedStore(inner, krA)
	storeB := newSealedStore(inner, krB)

	gwA := &Gateway{
		now:        clk.Now,
		store:      storeA,
		lockoutKey: []byte(testLockoutKey),
	}
	if _, lockErr := gwA.lockOutUser(ctx, "Z123456", "op1"); lockErr != nil {
		t.Fatalf("lockOutUser: %v", lockErr)
	}

	gwB := &Gateway{
		now:        clk.Now,
		store:      storeB,
		lockoutKey: []byte(testLockoutKey),
	}
	if revoked, revErr := gwB.userRevoked(ctx, "Z123456", clk.Now()); revErr != nil || !revoked {
		t.Fatalf("userRevoked across RT_KEYS replacement: revoked=%v err=%v, want true nil", revoked, revErr)
	}

	const wantMsg = "LOCKOUT_KEY is missing but lockout records exist; set the LOCKOUT_KEY secret"
	if err := checkLockoutMarker(storeB, nil); err == nil || err.Error() != wantMsg {
		t.Fatalf("checkLockoutMarker(storeB, nil): got %v, want %q", err, wantMsg)
	}

	// PutEvict and Pop also pass unsealed lockout kinds directly through.
	if _, err := storeA.PutEvict(ctx, kindUserRevoked, "evict-key", []byte(`{"revoked_before":1,"blocked":true}`), lockoutNoExpiry); err != nil {
		t.Fatalf("PutEvict unsealed kind: %v", err)
	}
	if got, ok, err := storeB.Pop(ctx, kindUserRevoked, "evict-key"); err != nil || !ok || !bytes.Equal(got, []byte(`{"revoked_before":1,"blocked":true}`)) {
		t.Fatalf("Pop unsealed kind across key rings: got=%q ok=%v err=%v", got, ok, err)
	}
}
