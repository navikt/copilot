package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const testRedirectURI = "http://127.0.0.1:33418/callback"

// authorize runs one /oauth/authorize request through the mux.
func authorize(mux *http.ServeMux, clientID, redirectURI string) *httptest.ResponseRecorder {
	return do(mux, httptest.NewRequest("GET", "/oauth/authorize?client_id="+url.QueryEscape(clientID)+
		"&redirect_uri="+url.QueryEscape(redirectURI)+"&state=client-state", nil))
}

// TestSignedClientID_SurvivesRestart is the point of the whole change: a
// client_id issued by one process still authorises against a later process
// with an empty store, because the registration travels inside the client_id
// rather than in a map that a deploy wipes.
func TestSignedClientID_SurvivesRestart(t *testing.T) {
	before, _ := newChainTestServer(t)
	clientID := registerClient(t, before, testRedirectURI)

	// Same app, restarted: new mux, new store, same signing key.
	after, _ := newChainTestServer(t)

	auth, callback, token := runChain(t, after, clientID, testRedirectURI)
	if auth.Code != http.StatusFound {
		t.Fatalf("authorize after restart: expected 302, got %d: %s", auth.Code, auth.Body.String())
	}
	if callback.Code != http.StatusFound {
		t.Fatalf("callback after restart: expected 302, got %d: %s", callback.Code, callback.Body.String())
	}
	if token.Code != http.StatusOK {
		t.Fatalf("token after restart: expected 200, got %d: %s", token.Code, token.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(token.Body).Decode(&resp); err != nil {
		t.Fatalf("token: decode: %v", err)
	}
	if resp["access_token"] == "" || resp["access_token"] == nil {
		t.Fatalf("token: no access_token in %v", resp)
	}
}

// TestSignedClientID_ForgedTag pairs the rejection with a control on the same
// server, so the test cannot pass just because everything is rejected.
func TestSignedClientID_ForgedTag(t *testing.T) {
	issuer, _ := newChainTestServer(t)
	clientID := registerClient(t, issuer, testRedirectURI)

	verifier, _ := newChainTestServer(t)

	if w := authorize(verifier, clientID, testRedirectURI); w.Code != http.StatusFound {
		t.Fatalf("control: genuine client_id should authorise, got %d: %s", w.Code, w.Body.String())
	}

	payload, tag, _ := strings.Cut(clientID, ".")
	forged := payload + "." + flipLastChar(tag)
	w := authorize(verifier, forged, testRedirectURI)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("forged tag: expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertInvalidClient(t, w)
}

// TestSignedClientID_TamperedPayload swaps the signed redirect_uri for an
// attacker's own and keeps the original tag. This is the GHSA-7hwf-488h-59x8
// shape: if it went through, the authorization code would be delivered to the
// attacker's URL.
func TestSignedClientID_TamperedPayload(t *testing.T) {
	mux, _ := newChainTestServer(t)
	clientID := registerClient(t, mux, testRedirectURI)

	if w := authorize(mux, clientID, testRedirectURI); w.Code != http.StatusFound {
		t.Fatalf("control: genuine client_id should authorise, got %d: %s", w.Code, w.Body.String())
	}

	_, tag, _ := strings.Cut(clientID, ".")
	evil := "https://evil.example.com/callback"
	// Written out as the raw wire format an attacker would craft, rather than
	// through the server's own type.
	tampered, err := json.Marshal(map[string]any{"u": []string{evil}, "t": 1})
	if err != nil {
		t.Fatal(err)
	}
	swapped := base64.RawURLEncoding.EncodeToString(tampered) + "." + tag

	w := authorize(mux, swapped, evil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("tampered payload: expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Fatalf("tampered payload: server redirected to %q", loc)
	}
	assertInvalidClient(t, w)
}

// TestSignedClientID_DifferentKey covers key rotation: a client_id signed under
// the old key stops verifying, and the client gets the recoverable
// invalid_client answer rather than a hang.
func TestSignedClientID_DifferentKey(t *testing.T) {
	issuer, _ := newChainTestServerWithSecret(t, "secret-before-rotation")
	clientID := registerClient(t, issuer, testRedirectURI)

	same, _ := newChainTestServerWithSecret(t, "secret-before-rotation")
	if w := authorize(same, clientID, testRedirectURI); w.Code != http.StatusFound {
		t.Fatalf("control: same key should authorise, got %d: %s", w.Code, w.Body.String())
	}

	rotated, _ := newChainTestServerWithSecret(t, "secret-after-rotation")
	w := authorize(rotated, clientID, testRedirectURI)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("rotated key: expected 400, got %d: %s", w.Code, w.Body.String())
	}
	assertInvalidClient(t, w)
}

// TestSignedClientID_Malformed feeds junk to the verifier. Nothing may panic:
// the tag is checked before any of this reaches a decoder.
func TestSignedClientID_Malformed(t *testing.T) {
	mux, _ := newChainTestServer(t)
	genuine := registerClient(t, mux, testRedirectURI)
	payload, tag, _ := strings.Cut(genuine, ".")

	cases := map[string]string{
		"no separator":       strings.ReplaceAll(genuine, ".", ""),
		"truncated":          genuine[:len(genuine)/2],
		"payload only":       payload + ".",
		"tag only":           "." + tag,
		"empty parts":        ".",
		"not base64":         "!!!.???",
		"random opaque id":   generateSecureToken(16),
		"two separators":     genuine + "." + genuine,
		"huge":               strings.Repeat("a", 5000),
		"tag over wrong pay": base64.RawURLEncoding.EncodeToString([]byte("not json")) + "." + tag,
	}

	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			w := authorize(mux, id, testRedirectURI)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}

	// And the control still works after all of that.
	if w := authorize(mux, genuine, testRedirectURI); w.Code != http.StatusFound {
		t.Fatalf("control: genuine client_id should authorise, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandleRegister_PublicClientResponse checks the RFC 7591 shape for a
// public client: no client_secret, so no client_secret_expires_at either.
func TestHandleRegister_PublicClientResponse(t *testing.T) {
	mux, _ := newChainTestServer(t)
	b, _ := json.Marshal(map[string]any{
		"client_name":   "VS Code",
		"redirect_uris": []string{testRedirectURI},
	})
	w := do(mux, httptest.NewRequest("POST", "/register", bytes.NewReader(b)))
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["client_secret"]; ok {
		t.Fatal("a public client must not be issued a client_secret")
	}
	if _, ok := resp["client_secret_expires_at"]; ok {
		t.Fatal("client_secret_expires_at is only defined when a client_secret is issued")
	}
	if resp["token_endpoint_auth_method"] != "none" {
		t.Fatalf("expected token_endpoint_auth_method 'none', got %v", resp["token_endpoint_auth_method"])
	}
	if _, ok := resp["client_id_issued_at"].(float64); !ok {
		t.Fatalf("expected a numeric client_id_issued_at, got %v", resp["client_id_issued_at"])
	}

	// The client_id grew from 22 characters to a signed blob; nothing may
	// depend on it staying short, but it must stay a sane size.
	clientID, _ := resp["client_id"].(string)
	if len(clientID) < 60 || len(clientID) > 512 {
		t.Fatalf("unexpected client_id length %d: %q", len(clientID), clientID)
	}
}

func TestHandleRegister_TooManyRedirectURIs(t *testing.T) {
	mux, _ := newChainTestServer(t)
	uris := make([]string, 11)
	for i := range uris {
		uris[i] = testRedirectURI
	}
	b, _ := json.Marshal(map[string]any{"redirect_uris": uris})
	w := do(mux, httptest.NewRequest("POST", "/register", bytes.NewReader(b)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func assertInvalidClient(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	// invalid_client is what tells the client to re-run registration instead of
	// retrying the same client_id forever.
	if resp["error"] != "invalid_client" {
		t.Fatalf("expected error 'invalid_client', got %q", resp["error"])
	}
}

func flipLastChar(s string) string {
	if s == "" {
		return "A"
	}
	last := s[len(s)-1]
	if last == 'A' {
		return s[:len(s)-1] + "B"
	}
	return s[:len(s)-1] + "A"
}

// TestSignedClientID_EmptyPayloadIsRejected pins that a correctly signed but
// empty registration is still refused; without redirect_uris there is nothing
// to match against.
func TestSignedClientID_EmptyPayloadIsRejected(t *testing.T) {
	key := deriveClientIDKey("a-secret")
	id := mintClientID(key, clientIDInfo{})
	if _, err := verifyClientID(key, id); err == nil {
		t.Fatal("expected a client_id with no redirect_uris to be rejected")
	}
}

// TestDeriveClientIDKey_Deterministic is what makes the restart property work:
// the same secret must produce the same key, a different secret must not.
func TestDeriveClientIDKey_Deterministic(t *testing.T) {
	a := deriveClientIDKey("shared-secret")
	b := deriveClientIDKey("shared-secret")
	if !bytes.Equal(a, b) {
		t.Fatal("same secret produced different signing keys")
	}
	if bytes.Equal(a, deriveClientIDKey("other-secret")) {
		t.Fatal("different secrets produced the same signing key")
	}
	if bytes.Equal(deriveClientIDKey(""), deriveClientIDKey("")) {
		t.Fatal("an unset secret must produce a random per-process key")
	}
}
