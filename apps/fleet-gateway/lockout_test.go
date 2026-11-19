package main

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestLockout_SessionRejectedForTargetUserOnly(t *testing.T) {
	ctx := context.Background()
	te := newTestEnv(t)

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
	te := newTestEnv(t)

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
	te := newClientEncryptedTestEnv(t, rtKeys)

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
		te := newTestEnv(t)
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
		te := newTestEnv(t)
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

func TestLockout_SurvivesRTKeysRotation(t *testing.T) {
	ctx := context.Background()
	keyA := "k1:" + testKeyBase64(t, false)
	keyB := "k2:" + testKeyBase64(t, false)
	ringBA := keyB + "," + keyA

	te := newClientEncryptedTestEnv(t, keyA)
	token := te.obtainAccessTokenForResource(te.resource)

	if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op1"); err != nil {
		t.Fatalf("lockOutUser with keyA: %v", err)
	}

	// Build a second gateway with rotated key ring "B,A" sharing the underlying inner store.
	innerStore := te.gw.store.(*sealedStore).inner
	krBA, err := parseKeyRing(ringBA)
	if err != nil {
		t.Fatalf("parseKeyRing B,A: %v", err)
	}
	userKeysBA, err := deriveUserKeys(krBA)
	if err != nil {
		t.Fatalf("deriveUserKeys B,A: %v", err)
	}
	gwBA := &Gateway{
		cfg:        te.gw.cfg,
		now:        te.gw.now,
		store:      newSealedStore(innerStore, krBA),
		rtKeys:     krBA,
		userKeys:   userKeysBA,
		TeamTokens: te.gw.TeamTokens,
	}

	if _, ok, err := gwBA.getSession(ctx, hashToken(token)); err != nil || ok {
		t.Fatalf("session after RT_KEYS rotation B,A: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
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
		RTKeys:        "",
	}
	userKeys, err := deriveUserKeys(kr)
	if err != nil {
		t.Fatalf("deriveUserKeys: %v", err)
	}
	gw := &Gateway{
		cfg:      cfg,
		now:      clk.Now,
		store:    cfg.Store,
		rtKeys:   kr,
		userKeys: userKeys,
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
}

func TestLockout_Undo(t *testing.T) {
	ctx := context.Background()
	rtKeys := "k1:" + testKeyBase64(t, false)
	te := newClientEncryptedTestEnv(t, rtKeys)

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
	te := newTestEnv(t)

	if err := te.gw.undoLockout(ctx, te.navIdent, "op-noop"); err != nil {
		t.Fatalf("undoLockout with no row: %v", err)
	}
	if _, ok, err := te.gw.store.Get(ctx, kindUserRevoked, te.gw.userKey(te.navIdent)); err != nil || ok {
		t.Fatalf("expected no row written on no-op undo: ok=%v err=%v", ok, err)
	}
}

func TestLockout_LegacyPlainIntRecord(t *testing.T) {
	ctx := context.Background()
	te := newTestEnv(t)

	lockSec := te.now.Unix()
	key := te.gw.userKey(te.navIdent)
	if err := te.gw.store.Put(ctx, kindUserRevoked, key, []byte("1700000000"), te.gw.lockoutTTL()); err != nil {
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

func TestLockout_MultiKeyMerging(t *testing.T) {
	ctx := context.Background()
	keyA := "k1:" + testKeyBase64(t, false)
	keyB := "k2:" + testKeyBase64(t, false)
	te := newClientEncryptedTestEnv(t, keyB+","+keyA)

	primaryKey := userKeyWith(te.gw.userKeys[0], te.navIdent)
	secondaryKey := userKeyWith(te.gw.userKeys[1], te.navIdent)
	ttl := te.gw.lockoutTTL()

	t.Run("max revoked_before wins with its blocked flag", func(t *testing.T) {
		if err := te.gw.store.Put(ctx, kindUserRevoked, primaryKey, []byte(`{"revoked_before":100,"blocked":false}`), ttl); err != nil {
			t.Fatal(err)
		}
		if err := te.gw.store.Put(ctx, kindUserRevoked, secondaryKey, []byte(`{"revoked_before":200,"blocked":true}`), ttl); err != nil {
			t.Fatal(err)
		}
		rec, ok, err := te.gw.revokedAt(ctx, te.navIdent)
		if err != nil || !ok {
			t.Fatalf("revokedAt: ok=%v err=%v", ok, err)
		}
		if rec.RevokedBefore != 200 || !rec.Blocked {
			t.Fatalf("got %+v, want {RevokedBefore:200 Blocked:true}", rec)
		}
	})

	t.Run("primary wins on tie", func(t *testing.T) {
		if err := te.gw.store.Put(ctx, kindUserRevoked, primaryKey, []byte(`{"revoked_before":200,"blocked":false}`), ttl); err != nil {
			t.Fatal(err)
		}
		if err := te.gw.store.Put(ctx, kindUserRevoked, secondaryKey, []byte(`{"revoked_before":200,"blocked":true}`), ttl); err != nil {
			t.Fatal(err)
		}
		rec, ok, err := te.gw.revokedAt(ctx, te.navIdent)
		if err != nil || !ok {
			t.Fatalf("revokedAt: ok=%v err=%v", ok, err)
		}
		if rec.RevokedBefore != 200 || rec.Blocked {
			t.Fatalf("got %+v, want {RevokedBefore:200 Blocked:false}", rec)
		}
	})
}

func TestLockout_UndoRemovesNonPrimaryRingKey(t *testing.T) {
	ctx := context.Background()
	keyA := "k1:" + testKeyBase64(t, false)
	keyB := "k2:" + testKeyBase64(t, false)
	ringBA := keyB + "," + keyA

	te := newClientEncryptedTestEnv(t, keyA)
	issuedBefore := te.now
	tok := te.obtainAccessTokenForResource(te.resource)

	if _, err := te.gw.lockOutUser(ctx, te.navIdent, "op1"); err != nil {
		t.Fatalf("lockOutUser with keyA: %v", err)
	}

	innerStore := te.gw.store.(*sealedStore).inner
	krBA, err := parseKeyRing(ringBA)
	if err != nil {
		t.Fatalf("parseKeyRing B,A: %v", err)
	}
	userKeysBA, err := deriveUserKeys(krBA)
	if err != nil {
		t.Fatalf("deriveUserKeys B,A: %v", err)
	}
	gwBA := &Gateway{
		cfg:        te.gw.cfg,
		now:        te.gw.now,
		store:      newSealedStore(innerStore, krBA),
		rtKeys:     krBA,
		userKeys:   userKeysBA,
		TeamTokens: te.gw.TeamTokens,
	}

	if err := gwBA.undoLockout(ctx, te.navIdent, "op2"); err != nil {
		t.Fatalf("undoLockout with ring B,A: %v", err)
	}
	// Non-primary keyA row must have been deleted; primary keyB row keeps revoked_before with blocked=false.
	if _, ok, err := gwBA.store.Get(ctx, kindUserRevoked, userKeyWith(userKeysBA[1], te.navIdent)); err != nil || ok {
		t.Fatalf("non-primary keyA row after undo: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
	if _, ok, err := gwBA.store.Get(ctx, kindUserRevoked, gwBA.userKey(te.navIdent)); err != nil || !ok {
		t.Fatalf("primary keyB row after undo: ok=%v err=%v, want ok=true err=nil", ok, err)
	}
	if revoked, err := gwBA.userRevoked(ctx, te.navIdent, issuedBefore); err != nil || !revoked {
		t.Fatalf("ring-B,A userRevoked for pre-lockout timestamp after undo: revoked=%v err=%v, want true nil", revoked, err)
	}
	if _, ok, err := gwBA.getSession(ctx, hashToken(tok)); err != nil || ok {
		t.Fatalf("ring-B,A pre-lockout getSession after undo: ok=%v err=%v, want false nil", ok, err)
	}
	if revoked, err := gwBA.userRevoked(ctx, te.navIdent, issuedBefore.Add(time.Second)); err != nil || revoked {
		t.Fatalf("ring-B,A userRevoked for post-undo timestamp: revoked=%v err=%v, want false nil", revoked, err)
	}
}

func TestRunLockout_CLIArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantUndo  bool
		wantIdent string
		wantErr   bool
	}{
		{name: "none", args: nil, wantErr: true},
		{name: "blank", args: []string{"  "}, wantErr: true},
		{name: "flag only", args: []string{"-x"}, wantErr: true},
		{name: "extra arg", args: []string{"a", "b"}, wantErr: true},
		{name: "undo alone", args: []string{"--undo"}, wantErr: true},
		{name: "undo extra arg", args: []string{"--undo", "a", "b"}, wantErr: true},
		{name: "undo blank", args: []string{"--undo", "  "}, wantErr: true},
		{name: "undo flag", args: []string{"--undo", "-x"}, wantErr: true},
		{name: "valid lock", args: []string{" z123456 "}, wantUndo: false, wantIdent: "Z123456"},
		{name: "valid undo", args: []string{"--undo", " z123456 "}, wantUndo: true, wantIdent: "Z123456"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			undo, ident, err := parseLockoutArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseLockoutArgs(%v): got (%v, %q, nil), want error", tc.args, undo, ident)
				}
				return
			}
			if err != nil || undo != tc.wantUndo || ident != tc.wantIdent {
				t.Fatalf("parseLockoutArgs(%v): got (%v, %q, %v), want (%v, %q, nil)", tc.args, undo, ident, err, tc.wantUndo, tc.wantIdent)
			}
		})
	}

	if code := runLockout(nil); code == 0 {
		t.Fatal("expected non-zero exit code for invalid args")
	}

	setBaseValidEnv(t)
	t.Setenv("OPERATOR", "Z999999")
	if code := runLockout([]string{" z123456 "}); code != 0 {
		t.Fatalf("expected 0 exit code for valid lockout invocation, got %d", code)
	}
	if code := runLockout([]string{"--undo", " z123456 "}); code != 0 {
		t.Fatalf("expected 0 exit code for valid lockout --undo invocation, got %d", code)
	}
}
