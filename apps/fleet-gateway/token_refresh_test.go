package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func newClientEncryptedTestEnv(t *testing.T, rtKeys string) *testEnv {
	t.Helper()
	te := newTestEnv(t)
	te.gw.store.Close()

	cfgCopy := *te.gw.cfg
	cfgCopy.Routes = append([]PilotRoute(nil), te.gw.cfg.Routes...)
	cfgCopy.RTKeys = rtKeys
	cfgCopy.RTStorage = rtStorageClientEncrypted
	cfgCopy.EntraScope = "openid offline_access api://azure-client-id/.default"
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

func (te *testEnv) obtainTokensForResource(resource string) (accessToken, refreshToken string, expiresIn int) {
	te.t.Helper()
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	entraState, stateCookie := te.startLogin(verifier)
	code := te.completeEntraCallback(entraState, stateCookie)

	status, body := te.postToken(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {te.clientID},
		"redirect_uri":  {te.redirectURI},
		"resource":      {resource},
		"code_verifier": {verifier},
	})
	if status != http.StatusOK {
		te.t.Fatalf("expected 200 from /token, got %d (%v)", status, body)
	}
	at, _ := body["access_token"].(string)
	rt, _ := body["refresh_token"].(string)
	exp, _ := body["expires_in"].(float64)
	if at == "" {
		te.t.Fatal("expected non-empty access_token")
	}
	return at, rt, int(exp)
}

func postRevoke(handler http.Handler, token, clientID string) (int, map[string]any) {
	form := url.Values{}
	if token != "" {
		form.Set("token", token)
	}
	if clientID != "" {
		form.Set("client_id", clientID)
	}
	req := httptest.NewRequest(http.MethodPost, "/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestRefreshToken_HappyPathAndMCP(t *testing.T) {
	rtKeys := "k1:" + testKeyBase64(t, false)
	te := newClientEncryptedTestEnv(t, rtKeys)

	var seenTexasAssertions []string
	var texasMu sync.Mutex
	te.texasServer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]string
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		texasMu.Lock()
		seenTexasAssertions = append(seenTexasAssertions, reqBody["user_token"])
		texasMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token": strings.Join([]string{"team", "obo", "token"}, "-"),
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	})

	// Metadata must advertise refresh_token and revocation_endpoint.
	metaReq := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	metaRec := httptest.NewRecorder()
	te.handler.ServeHTTP(metaRec, metaReq)
	if metaRec.Code != http.StatusOK {
		t.Fatalf("expected 200 metadata, got %d", metaRec.Code)
	}
	var meta map[string]any
	if err := json.Unmarshal(metaRec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta["revocation_endpoint"] != te.gw.cfg.BaseURL+"/revoke" {
		t.Fatalf("unexpected revocation_endpoint: %v", meta["revocation_endpoint"])
	}
	grants, _ := meta["grant_types_supported"].([]any)
	if len(grants) != 2 || grants[0] != "authorization_code" || grants[1] != "refresh_token" {
		t.Fatalf("unexpected grant_types_supported: %v", grants)
	}

	at1, rt1, _ := te.obtainTokensForResource(te.resource)
	if rt1 == "" {
		t.Fatal("expected refresh_token in client-encrypted mode")
	}

	// Refresh to get at2 and rotated rt2.
	status, body := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
		"resource":      {te.resource},
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 from refresh, got %d (%v)", status, body)
	}
	at2, _ := body["access_token"].(string)
	rt2, _ := body["refresh_token"].(string)
	if at2 == "" || at2 == at1 || rt2 == "" || rt2 == rt1 {
		t.Fatalf("expected fresh access and rotated refresh tokens")
	}

	// MCP call using at2 must succeed and exchange the refreshed Entra access token via Texas.
	mcpReq := httptest.NewRequest(http.MethodPost, "/mcp/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	mcpReq.Header.Set("Authorization", "Bearer "+at2)
	mcpRec := httptest.NewRecorder()
	te.handler.ServeHTTP(mcpRec, mcpReq)
	if mcpRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on MCP call with refreshed token, got %d (%s)", mcpRec.Code, mcpRec.Body.String())
	}

	texasMu.Lock()
	gotAssertions := slices.Clone(seenTexasAssertions)
	texasMu.Unlock()
	if len(gotAssertions) != 1 || gotAssertions[0] != "entra-refreshed-access-1" {
		t.Fatalf("expected Texas to receive refreshed Entra access token, got %v", gotAssertions)
	}

	// If Entra omits refresh_token on refresh, the new gateway RT retains the prior Entra RT.
	te.mu.Lock()
	te.entraRefreshOmitRT = true
	te.mu.Unlock()

	status3, body3 := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt2},
		"client_id":     {te.clientID},
	})
	if status3 != http.StatusOK {
		t.Fatalf("expected 200 on 2nd refresh (no Entra RT rotation), got %d (%v)", status3, body3)
	}
	rt3, _ := body3["refresh_token"].(string)
	if rt3 == "" {
		t.Fatal("expected gateway refresh_token even when Entra omits refresh_token")
	}
	status4, body4 := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt3},
		"client_id":     {te.clientID},
	})
	if status4 != http.StatusOK {
		t.Fatalf("expected 200 on 3rd refresh reusing prior Entra RT, got %d (%v)", status4, body4)
	}

	te.mu.Lock()
	seenRTs := slices.Clone(te.entraSeenRTs)
	te.mu.Unlock()
	wantRTs := []string{"entra-test-refresh", "entra-rotated-refresh-1", "entra-rotated-refresh-1"}
	if !slices.Equal(seenRTs, wantRTs) {
		t.Fatalf("unexpected Entra RTs redeemed: got %v, want %v", seenRTs, wantRTs)
	}
}

func TestRefreshToken_RotationReplayRejected(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt1, _ := te.obtainTokensForResource(te.resource)

	status, _ := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
	})
	if status != http.StatusOK {
		t.Fatalf("expected first refresh 200, got %d", status)
	}

	te.advance(te.gw.cfg.RefreshGraceWindow)
	replayStatus, replayBody := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
	})
	if replayStatus != http.StatusBadRequest || replayBody["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant on replay, got %d (%v)", replayStatus, replayBody)
	}

	te.mu.Lock()
	count := te.entraRefreshCount
	te.mu.Unlock()
	if count != 1 {
		t.Fatalf("expected Entra refresh to be called once, got %d", count)
	}
}

func TestRefreshToken_TamperWrongKIDAndKeyRotation(t *testing.T) {
	k1 := testKeyBase64(t, false)
	k2 := testKeyBase64(t, true)
	kOther := testKeyBase64(t, false)

	te := newClientEncryptedTestEnv(t, "k1:"+k1)
	_, rt1, _ := te.obtainTokensForResource(te.resource)

	// Tampered ciphertext (flip a byte).
	rawBlob, err := base64.RawURLEncoding.DecodeString(rt1)
	if err != nil {
		t.Fatalf("decode rt1: %v", err)
	}
	tamperedBlob := append([]byte(nil), rawBlob...)
	tamperedBlob[len(tamperedBlob)-1] ^= 0x01
	tamperedRT := base64.RawURLEncoding.EncodeToString(tamperedBlob)

	status, body := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {tamperedRT},
		"client_id":     {te.clientID},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant on tampered token, got %d (%v)", status, body)
	}

	// Wrong kid: token sealed with a key ring that has a different kid.
	otherKR, err := parseKeyRing("k99:" + kOther)
	if err != nil {
		t.Fatalf("parseKeyRing other: %v", err)
	}
	wrongKIDBlob, err := otherKR.Seal([]byte(rtSealAAD), []byte(`{"v":1,"jti":"x","iat":1700000000,"client_id":"`+te.clientID+`","resource":"`+te.resource+`","nav_ident":"Z123456","entra_rt":"rt"}`))
	if err != nil {
		t.Fatalf("seal wrong kid: %v", err)
	}
	status, body = te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {base64.RawURLEncoding.EncodeToString(wrongKIDBlob)},
		"client_id":     {te.clientID},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant on wrong kid, got %d (%v)", status, body)
	}

	// Wrong AAD sealed with valid key.
	wrongAADBlob, err := te.gw.rtKeys.Seal([]byte("wrong-aad"), []byte(`{"v":1,"jti":"x","iat":1700000000,"client_id":"`+te.clientID+`","resource":"`+te.resource+`","nav_ident":"Z123456","entra_rt":"rt"}`))
	if err != nil {
		t.Fatalf("seal wrong aad: %v", err)
	}
	status, body = te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {base64.RawURLEncoding.EncodeToString(wrongAADBlob)},
		"client_id":     {te.clientID},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant on wrong AAD, got %d (%v)", status, body)
	}

	// Key rotation: ring [new, old] opens tokens sealed with old; ring [new] only does not.
	// Mint a second token under k1 before rotating.
	_, rtOldForB, _ := te.obtainTokensForResource(te.resource)

	// Shared unsealed memory store (keys test isolate token ring rotation cleanly).
	shared := newMemoryStore(te.gw.now, map[string]int{kindSession: 100})
	cfgRotated := *te.gw.cfg
	cfgRotated.RTKeys = "k2:" + k2 + ",k1:" + k1
	cfgRotated.Store = shared
	gwRotated, err := newGateway(&cfgRotated, te.gw.now)
	if err != nil {
		t.Fatalf("newGateway rotated: %v", err)
	}
	hRotated := gwRotated.routes()

	reqRot := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
	}.Encode()))
	reqRot.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recRot := httptest.NewRecorder()
	hRotated.ServeHTTP(recRot, reqRot)
	if recRot.Code != http.StatusOK {
		t.Fatalf("expected [new,old] ring to open token sealed with old, got %d (%s)", recRot.Code, recRot.Body.String())
	}
	var rotResp map[string]any
	_ = json.Unmarshal(recRot.Body.Bytes(), &rotResp)
	rtNew, _ := rotResp["refresh_token"].(string)

	// Ring [new] only must reject old token, and accept newly rotated token.
	cfgOnlyNew := *te.gw.cfg
	cfgOnlyNew.RTKeys = "k2:" + k2
	cfgOnlyNew.Store = shared
	gwOnlyNew, err := newGateway(&cfgOnlyNew, te.gw.now)
	if err != nil {
		t.Fatalf("newGateway onlyNew: %v", err)
	}
	hOnlyNew := gwOnlyNew.routes()

	reqOld := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rtOldForB},
		"client_id":     {te.clientID},
	}.Encode()))
	reqOld.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recOld := httptest.NewRecorder()
	hOnlyNew.ServeHTTP(recOld, reqOld)
	var errBody map[string]any
	_ = json.Unmarshal(recOld.Body.Bytes(), &errBody)
	if recOld.Code != http.StatusBadRequest || errBody["error"] != "invalid_grant" {
		t.Fatalf("expected [new] ring to reject old-kid token with 400 invalid_grant, got %d (%v)", recOld.Code, errBody)
	}

	reqNew := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rtNew},
		"client_id":     {te.clientID},
	}.Encode()))
	reqNew.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recNew := httptest.NewRecorder()
	hOnlyNew.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Fatalf("expected [new] ring to accept rotated token, got %d (%s)", recNew.Code, recNew.Body.String())
	}
}

func TestRefreshToken_AbsoluteCap12h(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	iat := te.gw.now()
	absMax := iat.Add(12 * time.Hour)

	_, rt1, _ := te.obtainTokensForResource(te.resource)

	// Refresh at iat + 11h59m: must succeed, and new session Expiry must be <= iat + 12h.
	te.advance(11*time.Hour + 59*time.Minute)
	status, body := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 at iat+11h59m, got %d (%v)", status, body)
	}
	at2, _ := body["access_token"].(string)
	rt2, _ := body["refresh_token"].(string)
	expIn, _ := body["expires_in"].(float64)
	if expIn > 60 {
		t.Fatalf("expected expires_in capped at <= 60s, got %v", expIn)
	}

	sess, ok, err := te.gw.getSession(context.Background(), hashToken(at2))
	if err != nil || !ok {
		t.Fatalf("getSession failed: ok=%v err=%v", ok, err)
	}
	if sess.Expiry.After(absMax) {
		t.Fatalf("refreshed session Expiry %v exceeded iat+12h (%v)", sess.Expiry, absMax)
	}
	if !sess.IssuedAt.Equal(iat) {
		t.Fatalf("expected original IssuedAt %v preserved, got %v", iat, sess.IssuedAt)
	}

	// Advance past iat + 12h + 1s: refresh with rt2 must fail with invalid_grant.
	te.advance(1*time.Minute + 1*time.Second)
	expStatus, expBody := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt2},
		"client_id":     {te.clientID},
	})
	if expStatus != http.StatusBadRequest || expBody["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant past 12h cap, got %d (%v)", expStatus, expBody)
	}
}

func TestRevoke_RefreshTokenAndAccessToken(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))

	// 1. Revoke refresh token -> refresh rejected; repeated revoke still 200 {}.
	_, rt1, _ := te.obtainTokensForResource(te.resource)
	code, body := postRevoke(te.handler, rt1, te.clientID)
	if code != http.StatusOK || len(body) != 0 {
		t.Fatalf("expected 200 {} on /revoke RT, got %d (%v)", code, body)
	}
	status, tokErr := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
	})
	if status != http.StatusBadRequest || tokErr["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant after /revoke, got %d (%v)", status, tokErr)
	}
	code2, body2 := postRevoke(te.handler, rt1, te.clientID)
	if code2 != http.StatusOK || len(body2) != 0 {
		t.Fatalf("expected 200 {} on idempotent /revoke, got %d (%v)", code2, body2)
	}

	// 2. Revoke access token -> session deleted, MCP call returns 401.
	at2, _, _ := te.obtainTokensForResource(te.resource)
	code3, body3 := postRevoke(te.handler, at2, te.clientID)
	if code3 != http.StatusOK || len(body3) != 0 {
		t.Fatalf("expected 200 {} on /revoke AT, got %d (%v)", code3, body3)
	}
	mcpReq := httptest.NewRequest(http.MethodPost, "/mcp/etterlevelse", strings.NewReader(`{}`))
	mcpReq.Header.Set("Authorization", "Bearer "+at2)
	mcpRec := httptest.NewRecorder()
	te.handler.ServeHTTP(mcpRec, mcpReq)
	if mcpRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after revoking access token, got %d", mcpRec.Code)
	}

	// 3. Unknown token -> indistinguishable 200 {}.
	code4, body4 := postRevoke(te.handler, "non-existent-token", te.clientID)
	if code4 != http.StatusOK || len(body4) != 0 {
		t.Fatalf("expected 200 {} on unknown token revoke, got %d (%v)", code4, body4)
	}
}

func TestRevoke_ClientMismatchAndRequiredClientID(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	otherClient := te.cimdServer.URL + "/other-client.json"
	te.gw.cfg.AllowedClientIDs[otherClient] = true

	at1, rt1, _ := te.obtainTokensForResource(te.resource)

	// Missing client_id -> 400 invalid_request.
	code, body := postRevoke(te.handler, at1, "")
	if code != http.StatusBadRequest || body["error"] != "invalid_request" || body["error_description"] != "client_id is required" {
		t.Fatalf("expected 400 invalid_request client_id is required, got %d (%v)", code, body)
	}

	// Revoke AT with client B -> 200 {}, AT still works.
	code, body = postRevoke(te.handler, at1, otherClient)
	if code != http.StatusOK || len(body) != 0 {
		t.Fatalf("expected 200 {} on /revoke AT mismatch, got %d (%v)", code, body)
	}
	mcpReq := httptest.NewRequest(http.MethodPost, "/mcp/etterlevelse", strings.NewReader(`{}`))
	mcpReq.Header.Set("Authorization", "Bearer "+at1)
	mcpRec := httptest.NewRecorder()
	te.handler.ServeHTTP(mcpRec, mcpReq)
	if mcpRec.Code != http.StatusOK {
		t.Fatalf("expected AT to remain valid after client mismatch revoke, got %d", mcpRec.Code)
	}

	// Revoke RT with client B -> 200 {}, RT still refreshes.
	code, body = postRevoke(te.handler, rt1, otherClient)
	if code != http.StatusOK || len(body) != 0 {
		t.Fatalf("expected 200 {} on /revoke RT mismatch, got %d (%v)", code, body)
	}
	status, refBody := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
	})
	if status != http.StatusOK {
		t.Fatalf("expected RT to remain valid after client mismatch revoke, got %d (%v)", status, refBody)
	}
	rt2, _ := refBody["refresh_token"].(string)

	// Revoke AT and RT with owning client A -> both stop working.
	code, body = postRevoke(te.handler, at1, te.clientID)
	if code != http.StatusOK || len(body) != 0 {
		t.Fatalf("expected 200 {} on /revoke AT with owner, got %d (%v)", code, body)
	}
	mcpReq2 := httptest.NewRequest(http.MethodPost, "/mcp/etterlevelse", strings.NewReader(`{}`))
	mcpReq2.Header.Set("Authorization", "Bearer "+at1)
	mcpRec2 := httptest.NewRecorder()
	te.handler.ServeHTTP(mcpRec2, mcpReq2)
	if mcpRec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after owner revoked AT, got %d", mcpRec2.Code)
	}

	code, body = postRevoke(te.handler, rt2, te.clientID)
	if code != http.StatusOK || len(body) != 0 {
		t.Fatalf("expected 200 {} on /revoke RT with owner, got %d (%v)", code, body)
	}
	status, errBody := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt2},
		"client_id":     {te.clientID},
	})
	if status != http.StatusBadRequest || errBody["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant after owner revoked RT, got %d (%v)", status, errBody)
	}
}

func TestRefreshToken_ClientAndResourceMismatch(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	otherClient := te.cimdServer.URL + "/other-client.json"
	te.gw.cfg.AllowedClientIDs[otherClient] = true

	_, rt1, _ := te.obtainTokensForResource(te.resource)

	// client_id mismatch (even with allowlisted client_id).
	status, body := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {otherClient},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant on client_id mismatch, got %d (%v)", status, body)
	}

	// unallowlisted client_id.
	status, body = te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {"https://untrusted.example/client.json"},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant on unallowlisted client_id, got %d (%v)", status, body)
	}

	// resource mismatch.
	status, body = te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
		"resource":      {"https://fleet-gateway.intern.dev.nav.no/mcp/other"},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant on resource mismatch, got %d (%v)", status, body)
	}

	// Missing or oversized refresh_token.
	status, body = te.postToken(url.Values{
		"grant_type": {"refresh_token"},
		"client_id":  {te.clientID},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant on missing refresh_token, got %d (%v)", status, body)
	}
	status, body = te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {strings.Repeat("a", maxRefreshTokenLen+1)},
		"client_id":     {te.clientID},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant on oversized refresh_token, got %d (%v)", status, body)
	}

	// Negative control: original rt1 was never consumed by rejected checks and still succeeds.
	status, body = te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
	})
	if status != http.StatusOK || body["access_token"] == "" {
		t.Fatalf("expected original rt1 to still succeed after mismatched attempts, got %d (%v)", status, body)
	}
}

func runTwoInstanceRefreshTests(t *testing.T, shared SessionStore, rtKeys string) {
	t.Helper()
	te := newClientEncryptedTestEnv(t, rtKeys)

	gwA, handlerA := cloneTestEnvWithSharedStore(t, te, shared, te.gw.now)
	_, handlerB := cloneTestEnvWithSharedStore(t, te, shared, te.gw.now)

	te.gw = gwA
	te.handler = handlerA

	// 1. Token minted on A refreshes on B, then MCP call on A succeeds with B's access token.
	_, rt1, _ := te.obtainTokensForResource(te.resource)

	formB := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
		"resource":      {te.resource},
	}
	reqB := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(formB.Encode()))
	reqB.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recB := httptest.NewRecorder()
	handlerB.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("expected 200 refreshing A's token on B, got %d (%s)", recB.Code, recB.Body.String())
	}
	var respB map[string]any
	_ = json.Unmarshal(recB.Body.Bytes(), &respB)
	atB, _ := respB["access_token"].(string)
	rtB, _ := respB["refresh_token"].(string)
	if atB == "" || rtB == "" {
		t.Fatalf("expected access_token and refresh_token from B, got %v", respB)
	}

	mcpReq := httptest.NewRequest(http.MethodPost, "/mcp/etterlevelse", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	mcpReq.Header.Set("Authorization", "Bearer "+atB)
	mcpRec := httptest.NewRecorder()
	handlerA.ServeHTTP(mcpRec, mcpReq)
	if mcpRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on A with token refreshed on B, got %d (%s)", mcpRec.Code, mcpRec.Body.String())
	}

	// 2. Concurrent double-refresh across A and B with rtB: both get the same
	// pair, Entra is called once.
	te.mu.Lock()
	countBefore := te.entraRefreshCount
	te.mu.Unlock()

	var startWG, doneWG sync.WaitGroup
	results := make([]map[string]any, 2)
	startWG.Add(1)
	for i, h := range []http.Handler{handlerA, handlerB} {
		doneWG.Add(1)
		go func(i int, target http.Handler) {
			defer doneWG.Done()
			startWG.Wait()
			f := url.Values{
				"grant_type":    {"refresh_token"},
				"refresh_token": {rtB},
				"client_id":     {te.clientID},
			}
			req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(f.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			target.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("concurrent refresh %d: want 200, got %d: %s", i, rec.Code, rec.Body.String())
				return
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &results[i])
		}(i, h)
	}
	startWG.Done()
	doneWG.Wait()
	if t.Failed() {
		return
	}
	if results[0]["access_token"] != results[1]["access_token"] || results[0]["refresh_token"] != results[1]["refresh_token"] {
		t.Fatalf("concurrent refreshes must get the same pair: %v vs %v", results[0], results[1])
	}
	te.mu.Lock()
	delta := te.entraRefreshCount - countBefore
	te.mu.Unlock()
	if delta != 1 {
		t.Fatalf("expected fake Entra to count exactly 1 redemption during double-refresh, got %d", delta)
	}
}

func TestRefreshToken_TwoGatewaysSharedStore_Memory(t *testing.T) {
	rtKeys := "k1:" + testKeyBase64(t, false)
	kr, err := parseKeyRing(rtKeys)
	if err != nil {
		t.Fatalf("parseKeyRing: %v", err)
	}
	clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	shared := newSealedStore(newMemoryStore(clk.Now, map[string]int{
		kindPendingConsent: 1024,
		kindPendingEntra:   1024,
		kindCode:           1024,
		kindSession:        4096,
	}), kr)
	defer shared.Close()

	runTwoInstanceRefreshTests(t, shared, rtKeys)
}

func TestRefreshToken_TwoGatewaysSharedStore_Postgres(t *testing.T) {
	pgURL := os.Getenv("FGW_TEST_PG_URL")
	if pgURL == "" {
		t.Skip("FGW_TEST_PG_URL not set")
	}
	rtKeys := "k1:" + testKeyBase64(t, false)
	kr, err := parseKeyRing(rtKeys)
	if err != nil {
		t.Fatalf("parseKeyRing: %v", err)
	}
	clk := newFakeClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	pgStore := newTestPostgresStore(t, pgURL, clk.Now, map[string]int{
		kindPendingConsent: 1024,
		kindPendingEntra:   1024,
		kindCode:           1024,
		kindSession:        4096,
	})
	shared := newSealedStore(pgStore, kr)
	defer shared.Close()

	runTwoInstanceRefreshTests(t, shared, rtKeys)
}

func TestRefreshToken_PlaintextEntraRTNeverStored(t *testing.T) {
	rtKeys := "k1:" + testKeyBase64(t, false)
	kr, err := parseKeyRing(rtKeys)
	if err != nil {
		t.Fatalf("parseKeyRing: %v", err)
	}

	te := newClientEncryptedTestEnv(t, rtKeys)
	const secretEntraRT = "super-secret-entra-rt-marker-999"
	te.mu.Lock()
	te.entraRefreshToken = secretEntraRT
	te.mu.Unlock()

	rawMem := newMemoryStore(te.gw.now, map[string]int{
		kindPendingConsent: 1024,
		kindPendingEntra:   1024,
		kindCode:           1024,
		kindSession:        4096,
	})
	sealed := newSealedStore(rawMem, kr)
	gw, handler := cloneTestEnvWithSharedStore(t, te, sealed, te.gw.now)
	te.gw = gw
	te.handler = handler

	assertNoPlaintextInMemoryStore := func(stage string, needles ...string) {
		t.Helper()
		rawMem.mu.Lock()
		defer rawMem.mu.Unlock()
		for kind, bucket := range rawMem.items {
			for k, item := range bucket {
				for _, needle := range needles {
					if bytes.Contains(item.value, []byte(needle)) {
						t.Fatalf("%s: raw memoryStore (%s/%s) leaked plaintext %q", stage, kind, k, needle)
					}
				}
			}
		}
	}

	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	code := te.obtainAuthCode(verifier)
	assertNoPlaintextInMemoryStore("after callback (code stored)", secretEntraRT)

	status, body := te.postToken(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {te.clientID},
		"redirect_uri":  {te.redirectURI},
		"resource":      {te.resource},
		"code_verifier": {verifier},
	})
	if status != http.StatusOK {
		t.Fatalf("token exchange failed: %d (%v)", status, body)
	}
	at1, _ := body["access_token"].(string)
	rt1, _ := body["refresh_token"].(string)

	assertNoPlaintextInMemoryStore("after authorization_code", secretEntraRT)
	sess1, ok, err := gw.getSession(context.Background(), hashToken(at1))
	if err != nil || !ok {
		t.Fatalf("getSession at1 failed: ok=%v err=%v", ok, err)
	}
	if sess1.RefreshToken != "" {
		t.Fatalf("expected empty RefreshToken in stored session, got %q", sess1.RefreshToken)
	}

	status2, body2 := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
	})
	if status2 != http.StatusOK {
		t.Fatalf("refresh failed: %d (%v)", status2, body2)
	}
	at2, _ := body2["access_token"].(string)

	assertNoPlaintextInMemoryStore("after refresh_token", secretEntraRT, "entra-rotated-refresh-1")
	sess2, ok, err := gw.getSession(context.Background(), hashToken(at2))
	if err != nil || !ok {
		t.Fatalf("getSession at2 failed: ok=%v err=%v", ok, err)
	}
	if sess2.RefreshToken != "" {
		t.Fatalf("expected empty RefreshToken in refreshed session, got %q", sess2.RefreshToken)
	}
}

func TestRefreshToken_UpstreamInvalidGrantAndStoreErrors503(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	at1, rt1, _ := te.obtainTokensForResource(te.resource)

	// Upstream Entra invalid_grant => our invalid_grant without leaking upstream details.
	te.mu.Lock()
	te.entraRefreshStatus = http.StatusBadRequest
	te.mu.Unlock()

	status, body := te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt1},
		"client_id":     {te.clientID},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("expected 400 invalid_grant on upstream error, got %d (%v)", status, body)
	}
	if desc, _ := body["error_description"].(string); strings.Contains(desc, "AADSTS") {
		t.Fatalf("error_description must remain generic, got %q", desc)
	}

	// Reset upstream to healthy for store failure tests.
	te.mu.Lock()
	te.entraRefreshStatus = http.StatusOK
	te.mu.Unlock()

	_, rt2, _ := te.obtainTokensForResource(te.resource)
	origStore := te.gw.store

	// RevokeOnce store error => 503 temporarily_unavailable.
	te.gw.store = &failingStore{
		SessionStore:   origStore,
		failRevokeOnce: errors.New("db unavailable"),
	}
	status, body = te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt2},
		"client_id":     {te.clientID},
	})
	if status != http.StatusServiceUnavailable || body["error"] != "temporarily_unavailable" {
		t.Fatalf("expected 503 temporarily_unavailable on RevokeOnce failure, got %d (%v)", status, body)
	}

	// Put session store error during refresh => 503 temporarily_unavailable.
	te.gw.store = &failingStore{
		SessionStore: origStore,
		failPut:      errors.New("db unavailable"),
	}
	status, body = te.postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt2},
		"client_id":     {te.clientID},
	})
	if status != http.StatusServiceUnavailable || body["error"] != "temporarily_unavailable" {
		t.Fatalf("expected 503 temporarily_unavailable on Put failure, got %d (%v)", status, body)
	}

	// /revoke Delete error => 503 temporarily_unavailable.
	te.gw.store = &failingStore{
		SessionStore: origStore,
		failDelete:   errors.New("db unavailable"),
	}
	revStatus, revBody := postRevoke(te.handler, at1, te.clientID)
	if revStatus != http.StatusServiceUnavailable || revBody["error"] != "temporarily_unavailable" {
		t.Fatalf("expected 503 temporarily_unavailable on /revoke Delete error, got %d (%v)", revStatus, revBody)
	}

	// /revoke RevokeOnce error => 503 temporarily_unavailable.
	te.gw.store = &failingStore{
		SessionStore:   origStore,
		failRevokeOnce: errors.New("db unavailable"),
	}
	revStatus, revBody = postRevoke(te.handler, rt2, te.clientID)
	if revStatus != http.StatusServiceUnavailable || revBody["error"] != "temporarily_unavailable" {
		t.Fatalf("expected 503 temporarily_unavailable on /revoke RevokeOnce error, got %d (%v)", revStatus, revBody)
	}
}

func TestConfig_RTStorageModesAndMetadata(t *testing.T) {
	validKey := "k1:" + testKeyBase64(t, false)

	tests := []struct {
		name        string
		override    map[string]string
		wantStorage string
		wantError   string
	}{
		{
			name:        "unset RT_STORAGE and no RT_KEYS resolves to none",
			override:    map[string]string{},
			wantStorage: rtStorageNone,
		},
		{
			name:        "unset RT_STORAGE with RT_KEYS resolves to client-encrypted",
			override:    map[string]string{"RT_KEYS": validKey},
			wantStorage: rtStorageClientEncrypted,
		},
		{
			name:      "explicit client-encrypted requires RT_KEYS",
			override:  map[string]string{"RT_STORAGE": "client-encrypted"},
			wantError: "RT_KEYS is required",
		},
		{
			name:      "invalid RT_STORAGE fails closed",
			override:  map[string]string{"RT_STORAGE": "cookie"},
			wantError: "invalid RT_STORAGE",
		},
		{
			name:      "literal none in env RT_STORAGE fails closed",
			override:  map[string]string{"RT_STORAGE": "none"},
			wantError: "invalid RT_STORAGE",
		},
		{
			name: "strategy refresh with explicit client-encrypted fails",
			override: map[string]string{
				"TEAM_TOKEN_STRATEGY": "refresh",
				"RT_STORAGE":          "client-encrypted",
				"RT_KEYS":             validKey,
			},
			wantError: "strategy refresh needs RT_STORAGE=server",
		},
		{
			name: "strategy refresh with unset RT_STORAGE and RT_KEYS resolves client-encrypted and fails",
			override: map[string]string{
				"TEAM_TOKEN_STRATEGY": "refresh",
				"RT_KEYS":             validKey,
			},
			wantError: "strategy refresh needs RT_STORAGE=server",
		},
		{
			name: "strategy refresh with RT_STORAGE=server and RT_KEYS succeeds",
			override: map[string]string{
				"TEAM_TOKEN_STRATEGY": "refresh",
				"RT_STORAGE":          "server",
				"RT_KEYS":             validKey,
			},
			wantStorage: rtStorageServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBaseValidEnv(t)
			for k, v := range tt.override {
				t.Setenv(k, v)
			}
			cfg, err := loadConfigFromEnv()
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if cfg.RTStorage != tt.wantStorage {
				t.Fatalf("expected RTStorage=%q, got %q", tt.wantStorage, cfg.RTStorage)
			}
		})
	}

	// Verify RT_STORAGE=server and RT_STORAGE=none behavior on /token and metadata.
	for _, mode := range []string{rtStorageServer, rtStorageNone} {
		t.Run("no client refresh token in mode "+mode, func(t *testing.T) {
			te := newTestEnv(t)
			if mode == rtStorageServer {
				te.gw.store.Close()
				cfgCopy := *te.gw.cfg
				cfgCopy.RTKeys = validKey
				cfgCopy.RTStorage = rtStorageServer
				cfgCopy.TeamTokenStrategy = strategyRefresh
				if err := validateConfig(&cfgCopy); err != nil {
					t.Fatalf("validateConfig: %v", err)
				}
				gw, err := newGateway(&cfgCopy, te.gw.now)
				if err != nil {
					t.Fatalf("newGateway: %v", err)
				}
				t.Cleanup(gw.store.Close)
				te.gw = gw
				te.handler = gw.routes()
			} else if te.gw.cfg.RTStorage != rtStorageNone {
				t.Fatalf("expected default testEnv RTStorage=none, got %q", te.gw.cfg.RTStorage)
			}

			at, rt, _ := te.obtainTokensForResource(te.resource)
			if rt != "" {
				t.Fatalf("expected no gateway refresh_token in mode %s, got %q", mode, rt)
			}
			if mode == rtStorageServer {
				sess, ok, err := te.gw.getSession(context.Background(), hashToken(at))
				if err != nil || !ok || sess.RefreshToken == "" {
					t.Fatalf("expected Entra RT preserved in server mode session: ok=%v err=%v", ok, err)
				}
			}

			metaReq := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
			metaRec := httptest.NewRecorder()
			te.handler.ServeHTTP(metaRec, metaReq)
			var meta map[string]any
			_ = json.Unmarshal(metaRec.Body.Bytes(), &meta)
			if _, hasRevoke := meta["revocation_endpoint"]; hasRevoke {
				t.Fatalf("expected no revocation_endpoint in mode %s, got %v", mode, meta)
			}
			grants, _ := meta["grant_types_supported"].([]any)
			if len(grants) != 1 || grants[0] != "authorization_code" {
				t.Fatalf("expected only authorization_code in metadata for mode %s, got %v", mode, grants)
			}

			status, body := te.postToken(url.Values{
				"grant_type":    {"refresh_token"},
				"refresh_token": {fmt.Sprintf("dummy-%s", mode)},
				"client_id":     {te.clientID},
			})
			if status != http.StatusBadRequest || body["error"] != "unsupported_grant_type" {
				t.Fatalf("expected 400 unsupported_grant_type in mode %s, got %d (%v)", mode, status, body)
			}
		})
	}
}

func refreshForm(te *testEnv, rt string) url.Values {
	return url.Values{"grant_type": {"refresh_token"}, "refresh_token": {rt}, "client_id": {te.clientID}}
}

func TestRefreshToken_InFlightRetryGets503AndRecovers(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	te.gw.refreshWait = 0 // nothing to wait for: the first attempt failed upstream
	_, rt, _ := te.obtainTokensForResource(te.resource)

	te.mu.Lock()
	te.entraRefreshStatus = http.StatusServiceUnavailable
	te.mu.Unlock()
	if status, body := te.postToken(refreshForm(te, rt)); status != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on transient upstream error, got %d (%v)", status, body)
	}

	te.mu.Lock()
	te.entraRefreshStatus = http.StatusOK
	te.mu.Unlock()
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(refreshForm(te, rt).Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "temporarily_unavailable") {
		t.Fatalf("retry inside pending TTL: want 503 temporarily_unavailable, got %d %s", rec.Code, rec.Body.String())
	}
	if ra, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil || ra < 1 || ra > int(te.gw.cfg.RefreshGraceWindow.Seconds()/2) {
		t.Fatalf("bad Retry-After %q", rec.Header().Get("Retry-After"))
	}

	te.advance(refreshPendingTTL + time.Second)
	if status, body := te.postToken(refreshForm(te, rt)); status != http.StatusOK {
		t.Fatalf("retry after pending TTL: want 200, got %d (%v)", status, body)
	}
}

func TestRefreshToken_ConcurrentUseDuringInFlightGetsSamePair(t *testing.T) {
	te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
	_, rt, _ := te.obtainTokensForResource(te.resource)

	entered, release := make(chan struct{}), make(chan struct{})
	te.mu.Lock()
	te.entraRefreshHook = func() { close(entered); <-release }
	te.mu.Unlock()
	type res struct {
		status int
		body   map[string]any
	}
	first, second := make(chan res, 1), make(chan res, 1)
	go func() { s, b := te.postToken(refreshForm(te, rt)); first <- res{s, b} }()
	<-entered
	go func() { s, b := te.postToken(refreshForm(te, rt)); second <- res{s, b} }()
	time.Sleep(100 * time.Millisecond) // let the second request reach the pending claim
	close(release)
	a, b := <-first, <-second
	if a.status != http.StatusOK || b.status != http.StatusOK {
		t.Fatalf("both uses: want 200, got %d (%v) and %d (%v)", a.status, a.body, b.status, b.body)
	}
	if a.body["access_token"] != b.body["access_token"] || a.body["refresh_token"] != b.body["refresh_token"] {
		t.Fatalf("both uses must get the same pair")
	}
	te.mu.Lock()
	n := te.entraRefreshCount
	te.mu.Unlock()
	if n != 1 {
		t.Fatalf("Entra must be called once, count=%d", n)
	}
}

func TestRefreshToken_RevokedDuringInFlightIssuesNothing(t *testing.T) {
	for _, tc := range []string{"revoke_rt", "family"} {
		t.Run(tc, func(t *testing.T) {
			te := newClientEncryptedTestEnv(t, "k1:"+testKeyBase64(t, false))
			_, rt, _ := te.obtainTokensForResource(te.resource)
			payload, err := te.gw.openRefreshToken(rt)
			if err != nil {
				t.Fatal(err)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			te.mu.Lock()
			te.entraRefreshHook = func() { close(entered); <-release }
			te.mu.Unlock()
			type res struct {
				status int
				body   map[string]any
			}
			done := make(chan res, 1)
			go func() { s, b := te.postToken(refreshForm(te, rt)); done <- res{s, b} }()
			<-entered
			if tc == "revoke_rt" {
				if s, _ := postRevoke(te.handler, rt, te.clientID); s != http.StatusOK {
					t.Fatalf("revoke: %d", s)
				}
			} else if err := te.gw.store.Revoke(context.Background(), "fam:"+payload.FID, time.Unix(payload.EXP, 0)); err != nil {
				t.Fatal(err)
			}
			close(release)
			r := <-done
			if r.status != http.StatusBadRequest || r.body["error"] != "invalid_grant" || r.body["access_token"] != nil || r.body["refresh_token"] != nil {
				t.Fatalf("want invalid_grant and no tokens, got %d (%v)", r.status, r.body)
			}
		})
	}
}
