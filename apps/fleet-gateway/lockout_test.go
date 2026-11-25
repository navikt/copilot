package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

// Generated per run so no key-shaped literal lives in the source.
var testLockoutKey = rand.Text() + rand.Text() // 52 chars; LOCKOUT_KEY needs >= 32

func withLockoutKey(t *testing.T, te *testEnv) *testEnv {
	t.Helper()
	te.gw.store.Close()

	cfgCopy := *te.gw.cfg
	cfgCopy.Routes = append([]routing.Route(nil), te.gw.cfg.Routes...)
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
	assertValidAuditID(t, lines[0]["id"])
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

// Entries queued before a shutdown stay in the Postgres outbox and are logged
// by the first drain after the restart (#2657).
func TestLockout_Postgres_OutboxSurvivesRestart(t *testing.T) {
	pgURL := os.Getenv("FGW_TEST_PG_URL")
	if pgURL == "" {
		t.Skip("FGW_TEST_PG_URL not set")
	}
	ctx := context.Background()
	clk := newFakeClock(time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC))
	kr, err := parseKeyRing("k1:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatalf("parseKeyRing: %v", err)
	}
	schemaURL := newTestPostgresSchemaURL(t, pgURL)
	newGateway := func() (*Gateway, SessionStore) {
		pgStore := openTestPostgresStore(t, schemaURL, clk.Now, nil)
		store := newSealedStore(pgStore, kr)
		cfg := &Config{SessionMaxAge: defaultSessionMaxAge, ConsentTTL: defaultConsentTTL, Store: store, LockoutKey: testLockoutKey}
		return &Gateway{cfg: cfg, now: clk.Now, store: store, rtKeys: kr, lockoutKey: []byte(testLockoutKey)}, pgStore
	}

	before, pgBefore := newGateway()
	if _, err := before.lockOutUser(ctx, "Z123456", "op-before"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}
	pgBefore.Close() // shut down without draining

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	after, pgAfter := newGateway()
	defer pgAfter.Close()
	lockoutAuditOutboxPending.Set(-1)
	after.drainLockoutAudit(ctx)
	lines := parseJSONLogLines(t, buf.Bytes())
	if len(lines) != 1 || lines[0]["action"] != "lock" || lines[0]["operator"] != "op-before" {
		t.Fatalf("drain after restart logged %v, want the lock queued before shutdown", lines)
	}
	if got := testutil.ToFloat64(lockoutAuditOutboxPending); got != 0 {
		t.Fatalf("outbox pending after drain = %v, want 0", got)
	}
}

func TestLockout_DrainAuditSetsPendingGauge(t *testing.T) {
	ctx := context.Background()
	te := withLockoutKey(t, newTestEnv(t))
	if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}
	if n := te.gw.lockouts().DrainAudit(context.Background()); n != 0 {
		t.Fatalf("DrainAudit left %d entries, want 0", n)
	}
	if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}
	// A cancelled context makes the read fail: the gauge keeps its last value.
	lockoutAuditOutboxPending.Set(7)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	te.gw.drainLockoutAudit(cancelled)
	if got := testutil.ToFloat64(lockoutAuditOutboxPending); got != 7 {
		t.Fatalf("gauge after failed drain = %v, want unchanged 7", got)
	}
	te.gw.drainLockoutAudit(ctx)
	if got := testutil.ToFloat64(lockoutAuditOutboxPending); got != 0 {
		t.Fatalf("gauge after drain = %v, want 0", got)
	}
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
	var seenIDs []string
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
		seenIDs = append(seenIDs, assertValidAuditID(t, got["id"]))
	}
	if seenIDs[0] == seenIDs[1] {
		t.Fatalf("expected distinct audit IDs, got duplicate %q", seenIDs[0])
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
	baseCfg.Routes = append([]routing.Route(nil), te.gw.cfg.Routes...)
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
	if _, err = gwWithKey.lockOutUser(ctx, te.navIdent, "op1"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}

	// Now newGateway with empty LockoutKey must fail because the marker exists.
	const wantMsg = "LOCKOUT_KEY is missing but lockout records exist; set the LOCKOUT_KEY secret"
	if _, err = newGateway(&baseCfg, te.gw.now); err == nil || err.Error() != wantMsg {
		t.Fatalf("newGateway with marker and empty LockoutKey: got %v, want %q", err, wantMsg)
	}

	// Marker holds the key fingerprint, same key starts, different key refuses startup.
	gotMarker, ok, err := shared.Get(ctx, kindUserRevokedMarker, userRevokedMarkerKey)
	if err != nil || !ok || string(gotMarker) != lockoutKeyFingerprint([]byte(testLockoutKey)) {
		t.Fatalf("marker after lockOutUser: got=%q ok=%v err=%v", gotMarker, ok, err)
	}
	gwSameKey, err := newGateway(&withKeyCfg, te.gw.now)
	if err != nil {
		t.Fatalf("newGateway with matching LockoutKey: %v", err)
	}
	_ = gwSameKey

	otherKey := rand.Text() + rand.Text()
	wrongKeyCfg := withKeyCfg
	wrongKeyCfg.LockoutKey = otherKey
	if _, err = newGateway(&wrongKeyCfg, te.gw.now); err == nil ||
		!strings.Contains(err.Error(), "LOCKOUT_KEY does not match the key the existing lockout records were written with") ||
		strings.Contains(err.Error(), testLockoutKey) ||
		strings.Contains(err.Error(), otherKey) ||
		strings.Contains(err.Error(), lockoutKeyFingerprint([]byte(testLockoutKey))) ||
		strings.Contains(err.Error(), lockoutKeyFingerprint([]byte(otherKey))) {
		t.Fatalf("newGateway with mismatched LockoutKey: got %v", err)
	}

	// Legacy "1" marker is accepted, warns once without leaking key/fingerprint, and is rewritten.
	if err = shared.Put(ctx, kindUserRevokedMarker, userRevokedMarkerKey, []byte("1"), lockoutNoExpiry); err != nil {
		t.Fatalf("put legacy marker: %v", err)
	}
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	if _, err = newGateway(&withKeyCfg, te.gw.now); err != nil {
		t.Fatalf("newGateway with legacy '1' marker: %v", err)
	}
	warnLines := parseJSONLogLines(t, buf.Bytes())
	if len(warnLines) != 1 || warnLines[0]["level"] != "WARN" {
		t.Fatalf("expected 1 WARN line on legacy marker upgrade, got %s", buf.String())
	}
	if strings.Contains(buf.String(), testLockoutKey) || strings.Contains(buf.String(), lockoutKeyFingerprint([]byte(testLockoutKey))) {
		t.Fatalf("legacy marker warning leaked key or fingerprint: %s", buf.String())
	}
	rewritten, ok, err := shared.Get(ctx, kindUserRevokedMarker, userRevokedMarkerKey)
	if err != nil || !ok || string(rewritten) != lockoutKeyFingerprint([]byte(testLockoutKey)) {
		t.Fatalf("rewritten marker: got=%q ok=%v err=%v", rewritten, ok, err)
	}
	buf.Reset()
	if _, err = newGateway(&withKeyCfg, te.gw.now); err != nil {
		t.Fatalf("second newGateway after legacy marker upgrade: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no warning after legacy marker already rewritten, got %s", buf.String())
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

func assertValidAuditID(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	if !ok || len(s) != 32 {
		t.Fatalf("expected 32-char hex audit id, got %#v", v)
	}
	if _, err := hex.DecodeString(s); err != nil {
		t.Fatalf("expected valid hex audit id, got %q: %v", s, err)
	}
	return s
}

type auditHookStore struct {
	SessionStore
	onAuditGet func()
	onAuditCAS func()
	putErr     error
	deleteErr  error
}

func (s *auditHookStore) Get(ctx context.Context, kind, key string) ([]byte, bool, error) {
	val, ok, err := s.SessionStore.Get(ctx, kind, key)
	if err == nil && ok && kind == kindAuditOutbox && s.onAuditGet != nil {
		hook := s.onAuditGet
		s.onAuditGet = nil
		hook()
	}
	return val, ok, err
}

func (s *auditHookStore) Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error {
	if kind == kindAuditOutbox && s.putErr != nil {
		return s.putErr
	}
	return s.SessionStore.Put(ctx, kind, key, val, ttl)
}

func (s *auditHookStore) Delete(ctx context.Context, kind, key string) error {
	if kind == kindAuditOutbox && s.deleteErr != nil {
		return s.deleteErr
	}
	return s.SessionStore.Delete(ctx, kind, key)
}

func (s *auditHookStore) CompareAndSwap(ctx context.Context, kind, key string, oldVal, newVal []byte, ttl time.Duration) (bool, error) {
	if kind == kindAuditOutbox {
		if s.onAuditCAS != nil {
			hook := s.onAuditCAS
			s.onAuditCAS = nil
			hook()
		}
		if newVal == nil && s.deleteErr != nil {
			return false, s.deleteErr
		}
		if newVal != nil && s.putErr != nil {
			return false, s.putErr
		}
	}
	return s.SessionStore.CompareAndSwap(ctx, kind, key, oldVal, newVal, ttl)
}

func runLockoutFingerprintAndAuditContract(t *testing.T, newStore func(t *testing.T, now func() time.Time) SessionStore) {
	t.Helper()
	ctx := context.Background()

	t.Run("fingerprint match, mismatch, and legacy rewrite", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
		store := newStore(t, clk.Now)
		gw := &Gateway{
			now:        clk.Now,
			store:      store,
			lockoutKey: []byte(testLockoutKey),
		}

		if _, err := gw.lockOutUser(ctx, "Z111111", "op1"); err != nil {
			t.Fatalf("lockOutUser: %v", err)
		}
		if err := checkLockoutMarker(store, []byte(testLockoutKey)); err != nil {
			t.Fatalf("checkLockoutMarker same key: %v", err)
		}

		otherKey := rand.Text() + rand.Text()
		if err := checkLockoutMarker(store, []byte(otherKey)); err == nil ||
			!strings.Contains(err.Error(), "LOCKOUT_KEY does not match the key the existing lockout records were written with") ||
			strings.Contains(err.Error(), testLockoutKey) ||
			strings.Contains(err.Error(), otherKey) ||
			strings.Contains(err.Error(), lockoutKeyFingerprint([]byte(testLockoutKey))) ||
			strings.Contains(err.Error(), lockoutKeyFingerprint([]byte(otherKey))) {
			t.Fatalf("checkLockoutMarker mismatched key: got %v", err)
		}

		if err := store.Put(ctx, kindUserRevokedMarker, userRevokedMarkerKey, []byte("1"), lockoutNoExpiry); err != nil {
			t.Fatalf("put legacy marker: %v", err)
		}
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		defer slog.SetDefault(prev)

		if err := checkLockoutMarker(store, []byte(testLockoutKey)); err != nil {
			t.Fatalf("checkLockoutMarker legacy '1' marker: %v", err)
		}
		warnLines := parseJSONLogLines(t, buf.Bytes())
		if len(warnLines) != 1 || warnLines[0]["level"] != "WARN" {
			t.Fatalf("expected 1 WARN line on legacy marker upgrade, got %s", buf.String())
		}
		if strings.Contains(buf.String(), testLockoutKey) || strings.Contains(buf.String(), lockoutKeyFingerprint([]byte(testLockoutKey))) {
			t.Fatalf("legacy marker warning leaked key or fingerprint: %s", buf.String())
		}
		gotMarker, ok, err := store.Get(ctx, kindUserRevokedMarker, userRevokedMarkerKey)
		if err != nil || !ok || string(gotMarker) != lockoutKeyFingerprint([]byte(testLockoutKey)) {
			t.Fatalf("rewritten marker: got=%q ok=%v err=%v", gotMarker, ok, err)
		}
		buf.Reset()
		if err := checkLockoutMarker(store, []byte(testLockoutKey)); err != nil {
			t.Fatalf("second checkLockoutMarker after upgrade: %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("expected no warning on subsequent check, got %s", buf.String())
		}
	})

	t.Run("drain keeps entry appended between read and delete", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
		base := newStore(t, clk.Now)
		hooked := &auditHookStore{SessionStore: base}
		gw := &Gateway{
			now:        clk.Now,
			store:      hooked,
			lockoutKey: []byte(testLockoutKey),
		}

		if _, err := gw.lockOutUser(ctx, "Z111111", "op-first"); err != nil {
			t.Fatalf("lockOutUser: %v", err)
		}
		hooked.onAuditGet = func() {
			if err := gw.undoLockout(ctx, "Z111111", "op-between"); err != nil {
				t.Errorf("undoLockout during drain: %v", err)
			}
		}

		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		defer slog.SetDefault(prev)

		gw.drainLockoutAudit(ctx)
		lines1 := parseJSONLogLines(t, buf.Bytes())
		if len(lines1) != 1 || lines1[0]["action"] != "lock" || lines1[0]["operator"] != "op-first" {
			t.Fatalf("first drain expected only op-first, got %s", buf.String())
		}
		id1 := assertValidAuditID(t, lines1[0]["id"])

		buf.Reset()
		gw.drainLockoutAudit(ctx)
		lines2 := parseJSONLogLines(t, buf.Bytes())
		if len(lines2) != 1 || lines2[0]["action"] != "undo" || lines2[0]["operator"] != "op-between" {
			t.Fatalf("second drain expected op-between, got %s", buf.String())
		}
		id2 := assertValidAuditID(t, lines2[0]["id"])
		if id1 == id2 {
			t.Fatalf("expected distinct audit IDs, got %q", id1)
		}

		buf.Reset()
		gw.drainLockoutAudit(ctx)
		if buf.Len() != 0 {
			t.Fatalf("expected third drain to log nothing, got %s", buf.String())
		}
		if _, ok, err := base.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey); err != nil || ok {
			t.Fatalf("expected empty outbox after drains: ok=%v err=%v", ok, err)
		}
	})

	t.Run("drain logs and removes legacy entry without id", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
		store := newStore(t, clk.Now)
		gw := &Gateway{
			now:        clk.Now,
			store:      store,
			lockoutKey: []byte(testLockoutKey),
		}

		legacyJSON := `[{"action":"lock","user_key":"uk-legacy","operator":"op-legacy","at":1700000000}]`
		if err := store.Put(ctx, kindAuditOutbox, lockoutAuditOutboxKey, []byte(legacyJSON), lockoutNoExpiry); err != nil {
			t.Fatalf("put legacy audit outbox: %v", err)
		}

		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		defer slog.SetDefault(prev)

		gw.drainLockoutAudit(ctx)
		lines := parseJSONLogLines(t, buf.Bytes())
		if len(lines) != 1 || lines[0]["msg"] != "audit lockout" || lines[0]["id"] != "" || lines[0]["operator"] != "op-legacy" {
			t.Fatalf("legacy audit drain lines: got %s", buf.String())
		}
		if _, ok, err := store.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey); err != nil || ok {
			t.Fatalf("expected legacy outbox deleted: ok=%v err=%v", ok, err)
		}
		buf.Reset()
		gw.drainLockoutAudit(ctx)
		if buf.Len() != 0 {
			t.Fatalf("expected second drain to log nothing, got %s", buf.String())
		}
	})

	t.Run("failing Delete keeps entries for next drain", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
		base := newStore(t, clk.Now)
		hooked := &auditHookStore{SessionStore: base}
		gw := &Gateway{
			now:        clk.Now,
			store:      hooked,
			lockoutKey: []byte(testLockoutKey),
		}

		if _, err := gw.lockOutUser(ctx, "Z222222", "op-del-fail"); err != nil {
			t.Fatalf("lockOutUser: %v", err)
		}
		hooked.deleteErr = errors.New("delete boom")

		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		defer slog.SetDefault(prev)

		gw.drainLockoutAudit(ctx)
		lines1 := parseJSONLogLines(t, buf.Bytes())
		if len(lines1) != 2 || lines1[0]["msg"] != "audit lockout" || lines1[1]["level"] != "WARN" {
			t.Fatalf("expected audit line + warn on Delete failure, got %s", buf.String())
		}
		id1 := assertValidAuditID(t, lines1[0]["id"])
		if _, ok, err := base.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey); err != nil || !ok {
			t.Fatalf("expected outbox preserved on Delete failure: ok=%v err=%v", ok, err)
		}

		hooked.deleteErr = nil
		buf.Reset()
		gw.drainLockoutAudit(ctx)
		lines2 := parseJSONLogLines(t, buf.Bytes())
		if len(lines2) != 1 || lines2[0]["msg"] != "audit lockout" || lines2[0]["operator"] != "op-del-fail" {
			t.Fatalf("expected retry drain to log op-del-fail, got %s", buf.String())
		}
		if gotID := assertValidAuditID(t, lines2[0]["id"]); gotID != id1 {
			t.Fatalf("expected same audit id %q on retry, got %q", id1, gotID)
		}
		if _, ok, err := base.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey); err != nil || ok {
			t.Fatalf("expected outbox deleted after retry: ok=%v err=%v", ok, err)
		}
	})

	t.Run("failing Put keeps entries for next drain", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
		base := newStore(t, clk.Now)
		hooked := &auditHookStore{SessionStore: base}
		gw := &Gateway{
			now:        clk.Now,
			store:      hooked,
			lockoutKey: []byte(testLockoutKey),
		}

		if _, err := gw.lockOutUser(ctx, "Z333333", "op-put-fail-1"); err != nil {
			t.Fatalf("lockOutUser: %v", err)
		}
		hooked.onAuditGet = func() {
			if err := gw.undoLockout(ctx, "Z333333", "op-put-fail-2"); err != nil {
				t.Errorf("undoLockout during drain: %v", err)
			}
			hooked.putErr = errors.New("put boom")
		}

		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		defer slog.SetDefault(prev)

		gw.drainLockoutAudit(ctx)
		lines1 := parseJSONLogLines(t, buf.Bytes())
		if len(lines1) != 2 || lines1[0]["msg"] != "audit lockout" || lines1[1]["level"] != "WARN" {
			t.Fatalf("expected audit line + warn on Put failure, got %s", buf.String())
		}

		hooked.putErr = nil
		buf.Reset()
		gw.drainLockoutAudit(ctx)
		lines2 := parseJSONLogLines(t, buf.Bytes())
		if len(lines2) != 2 || lines2[0]["operator"] != "op-put-fail-1" || lines2[1]["operator"] != "op-put-fail-2" {
			t.Fatalf("expected both entries preserved for next drain, got %s", buf.String())
		}
		if _, ok, err := base.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey); err != nil || ok {
			t.Fatalf("expected outbox deleted after retry: ok=%v err=%v", ok, err)
		}
	})

	t.Run("drain keeps entry appended between re-read and CAS write", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
		base := newStore(t, clk.Now)
		hooked := &auditHookStore{SessionStore: base}
		gw := &Gateway{
			now:        clk.Now,
			store:      hooked,
			lockoutKey: []byte(testLockoutKey),
		}

		if _, err := gw.lockOutUser(ctx, "Z111111", "op-first"); err != nil {
			t.Fatalf("lockOutUser: %v", err)
		}
		// Conflict on the first CAS (forces drain to re-read), then conflict again
		// between drain's re-read and its CAS write.
		hooked.onAuditCAS = func() {
			if err := gw.undoLockout(ctx, "Z111111", "op-between-1"); err != nil {
				t.Errorf("undoLockout before first CAS: %v", err)
			}
			hooked.onAuditCAS = func() {
				if _, err := gw.lockOutUser(ctx, "Z222222", "op-between-2"); err != nil {
					t.Errorf("lockOutUser between re-read and CAS: %v", err)
				}
			}
		}

		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		defer slog.SetDefault(prev)

		gw.drainLockoutAudit(ctx)
		lines1 := parseJSONLogLines(t, buf.Bytes())
		if len(lines1) != 1 || lines1[0]["operator"] != "op-first" {
			t.Fatalf("first drain expected only op-first (no re-log on CAS retry), got %s", buf.String())
		}

		buf.Reset()
		gw.drainLockoutAudit(ctx)
		lines2 := parseJSONLogLines(t, buf.Bytes())
		if len(lines2) != 2 || lines2[0]["operator"] != "op-between-1" || lines2[1]["operator"] != "op-between-2" {
			t.Fatalf("second drain expected both interleaved entries, got %s", buf.String())
		}
		if _, ok, err := base.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey); err != nil || ok {
			t.Fatalf("expected empty outbox after second drain: ok=%v err=%v", ok, err)
		}
	})

	t.Run("two concurrent appends are both kept", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
		store := newStore(t, clk.Now)
		gw := &Gateway{
			now:        clk.Now,
			store:      store,
			lockoutKey: []byte(testLockoutKey),
		}

		start := make(chan struct{})
		errs := make(chan error, 2)
		go func() {
			<-start
			_, err := gw.lockOutUser(ctx, "Z444444", "op-conc-1")
			errs <- err
		}()
		go func() {
			<-start
			errs <- gw.undoLockout(ctx, "Z555555", "op-conc-2")
		}()
		close(start)
		for range 2 {
			if err := <-errs; err != nil {
				t.Fatalf("concurrent append failed: %v", err)
			}
		}

		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		defer slog.SetDefault(prev)

		gw.drainLockoutAudit(ctx)
		lines := parseJSONLogLines(t, buf.Bytes())
		if len(lines) != 2 {
			t.Fatalf("expected 2 logged entries, got %s", buf.String())
		}
		ops := map[string]bool{}
		for _, l := range lines {
			op, _ := l["operator"].(string)
			ops[op] = true
			assertValidAuditID(t, l["id"])
		}
		if !ops["op-conc-1"] || !ops["op-conc-2"] {
			t.Fatalf("expected both op-conc-1 and op-conc-2, got %v", ops)
		}
		if _, ok, err := store.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey); err != nil || ok {
			t.Fatalf("expected outbox empty after drain: ok=%v err=%v", ok, err)
		}
	})

	t.Run("legacy sealed outbox fixture can be appended and drained", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
		store := newStore(t, clk.Now)
		sealed, ok := store.(*sealedStore)
		if !ok {
			t.Fatal("expected *sealedStore")
		}
		gw := &Gateway{
			now:        clk.Now,
			store:      store,
			lockoutKey: []byte(testLockoutKey),
		}

		legacyEntries := []lockoutAuditEntry{{
			ID:       "0123456789abcdef0123456789abcdef",
			Action:   "lock",
			UserKey:  userKeyWith([]byte(testLockoutKey), "Z999999"),
			Operator: "op-legacy-sealed",
			At:       clk.Now().Unix(),
		}}
		plaintext, err := json.Marshal(legacyEntries)
		if err != nil {
			t.Fatal(err)
		}
		blob, err := sealed.KeyRing().Seal(sealAAD(kindAuditOutbox, lockoutAuditOutboxKey), plaintext)
		if err != nil {
			t.Fatal(err)
		}
		if err := sealed.Inner().Put(ctx, kindAuditOutbox, lockoutAuditOutboxKey, blob, lockoutNoExpiry); err != nil {
			t.Fatal(err)
		}

		if _, err := gw.lockOutUser(ctx, "Z888888", "op-new-after-legacy"); err != nil {
			t.Fatalf("lockOutUser on legacy sealed outbox: %v", err)
		}
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		defer slog.SetDefault(prev)

		gw.drainLockoutAudit(ctx)
		lines := parseJSONLogLines(t, buf.Bytes())
		if len(lines) != 2 || lines[0]["operator"] != "op-legacy-sealed" || lines[1]["operator"] != "op-new-after-legacy" {
			t.Fatalf("expected both legacy sealed and appended entries drained, got %s", buf.String())
		}
		if _, ok, err := store.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey); err != nil || ok {
			t.Fatalf("expected outbox deleted after drain: ok=%v err=%v", ok, err)
		}

		// Also verify draining directly from a legacy sealed row without an append.
		if err := sealed.Inner().Put(ctx, kindAuditOutbox, lockoutAuditOutboxKey, blob, lockoutNoExpiry); err != nil {
			t.Fatal(err)
		}
		buf.Reset()
		gw.drainLockoutAudit(ctx)
		lines2 := parseJSONLogLines(t, buf.Bytes())
		if len(lines2) != 1 || lines2[0]["operator"] != "op-legacy-sealed" {
			t.Fatalf("expected direct drain of legacy sealed outbox, got %s", buf.String())
		}
		if _, ok, err := store.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey); err != nil || ok {
			t.Fatalf("expected outbox deleted after direct legacy drain: ok=%v err=%v", ok, err)
		}
	})
}

func TestLockout_FingerprintAndAudit_Memory(t *testing.T) {
	runLockoutFingerprintAndAuditContract(t, func(t *testing.T, now func() time.Time) SessionStore {
		t.Helper()
		kr, err := parseKeyRing("k1:" + testKeyBase64(t, false))
		if err != nil {
			t.Fatalf("parseKeyRing: %v", err)
		}
		s := newSealedStore(newMemoryStore(now, map[string]int{kindSession: 10}), kr)
		t.Cleanup(s.Close)
		return s
	})
}

func TestLockout_FingerprintAndAudit_Postgres(t *testing.T) {
	pgURL := os.Getenv("FGW_TEST_PG_URL")
	if pgURL == "" {
		t.Skip("FGW_TEST_PG_URL not set")
	}
	runLockoutFingerprintAndAuditContract(t, func(t *testing.T, now func() time.Time) SessionStore {
		t.Helper()
		kr, err := parseKeyRing("k1:" + testKeyBase64(t, false))
		if err != nil {
			t.Fatalf("parseKeyRing: %v", err)
		}
		pgStore := newTestPostgresStore(t, pgURL, now, map[string]int{kindSession: 10})
		s := newSealedStore(pgStore, kr)
		t.Cleanup(s.Close)
		return s
	})
}

func TestLockout_AuditOutboxSurvivesRTKeyReplacement(t *testing.T) {
	ctx := context.Background()
	clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	inner := newMemoryStore(clk.Now, nil)
	krA, err := parseKeyRing("k1:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatal(err)
	}
	krB, err := parseKeyRing("k2:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatal(err)
	}
	gwA := &Gateway{now: clk.Now, store: newSealedStore(inner, krA), lockoutKey: []byte(testLockoutKey)}
	if _, err = gwA.lockOutUser(ctx, "Z123456", "op1"); err != nil {
		t.Fatalf("lockOutUser: %v", err)
	}
	gwB := &Gateway{now: clk.Now, store: newSealedStore(inner, krB), lockoutKey: []byte(testLockoutKey)}
	// A second lockout after the RT_KEYS swap must still append to the outbox written under the old keys.
	if _, err = gwB.lockOutUser(ctx, "Z654321", "op2"); err != nil {
		t.Fatalf("lockOutUser after RT_KEYS replacement: %v", err)
	}
	raw, ok, err := gwB.store.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey)
	if err != nil || !ok {
		t.Fatalf("outbox get: ok=%v err=%v", ok, err)
	}
	var entries []lockoutAuditEntry
	if err = json.Unmarshal(raw, &entries); err != nil || len(entries) != 2 {
		t.Fatalf("outbox entries = %d (err %v), want 2", len(entries), err)
	}
}
