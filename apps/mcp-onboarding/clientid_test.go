package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
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

// TestSignedClientID_Expired is the TTL the deleted cleanup loop used to
// enforce. /register is unauthenticated and accepts any https redirect_uri, so
// a client_id that never expires is an unrevocable registration bound to an
// attacker's callback.
func TestSignedClientID_Expired(t *testing.T) {
	mux, server := newChainTestServer(t)

	fresh := mintClientID(server.clientIDKey, clientIDInfo{
		RedirectURIs: []string{testRedirectURI},
		IssuedAt:     time.Now().Unix(),
		Nonce:        generateSecureToken(8),
	})
	if w := authorize(mux, fresh, testRedirectURI); w.Code != http.StatusFound {
		t.Fatalf("control: a fresh client_id should authorise, got %d: %s", w.Code, w.Body.String())
	}

	cases := map[string]int64{
		"older than the window": time.Now().Add(-clientIDTTL - time.Hour).Unix(),
		"absent":                0,
		"negative":              -1,
		"in the future":         time.Now().Add(48 * time.Hour).Unix(),
	}
	for name, issuedAt := range cases {
		t.Run(name, func(t *testing.T) {
			id := mintClientID(server.clientIDKey, clientIDInfo{
				RedirectURIs: []string{testRedirectURI},
				IssuedAt:     issuedAt,
				Nonce:        generateSecureToken(8),
			})
			w := authorize(mux, id, testRedirectURI)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
			// invalid_client is the recoverable answer: the client re-registers
			// rather than retrying a dead client_id forever.
			assertInvalidClient(t, w)
		})
	}

	// Just inside the window still authorises, so the check is a window and not
	// a blanket rejection.
	recent := mintClientID(server.clientIDKey, clientIDInfo{
		RedirectURIs: []string{testRedirectURI},
		IssuedAt:     time.Now().Add(-clientIDTTL + time.Hour).Unix(),
		Nonce:        generateSecureToken(8),
	})
	if w := authorize(mux, recent, testRedirectURI); w.Code != http.StatusFound {
		t.Fatalf("a client_id just inside the window should authorise, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandleRegister_RedirectURITooLong: the 10-URI and 64 KB caps bound the
// request, not one URI, so /register used to answer 201 with a client_id that
// /oauth/authorize would then always reject.
func TestHandleRegister_RedirectURITooLong(t *testing.T) {
	mux, _ := newChainTestServer(t)

	// Control: an ordinary registration on the same server still succeeds.
	if id := registerClient(t, mux, testRedirectURI); id == "" {
		t.Fatal("control: expected a client_id")
	}

	long := "https://example.com/" + strings.Repeat("a", 6000)
	b, _ := json.Marshal(map[string]any{"redirect_uris": []string{long}})
	w := do(mux, httptest.NewRequest("POST", "/register", bytes.NewReader(b)))
	if w.Code != http.StatusBadRequest {
		var resp map[string]any
		_ = json.NewDecoder(w.Body).Decode(&resp)
		id, _ := resp["client_id"].(string)
		t.Fatalf("expected 400, got %d with a %d-character client_id", w.Code, len(id))
	}
}

// TestSignedClientID_NonCanonicalTag: a 32-byte tag is 43 base64url characters
// carrying 258 bits, and Go's decoder accepts the two slack bits set. Without
// canonical encoding the same registration has four spellings, and client_id is
// not an identifier anything can safely key on.
func TestSignedClientID_NonCanonicalTag(t *testing.T) {
	mux, _ := newChainTestServer(t)
	genuine := registerClient(t, mux, testRedirectURI)
	if w := authorize(mux, genuine, testRedirectURI); w.Code != http.StatusFound {
		t.Fatalf("control: genuine client_id should authorise, got %d: %s", w.Code, w.Body.String())
	}

	payload, tag, _ := strings.Cut(genuine, ".")
	want, err := base64.RawURLEncoding.DecodeString(tag)
	if err != nil {
		t.Fatal(err)
	}

	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	variants := 0
	for i := 0; i < len(alphabet); i++ {
		alt := tag[:len(tag)-1] + string(alphabet[i])
		if alt == tag {
			continue
		}
		got, err := base64.RawURLEncoding.DecodeString(alt)
		if err != nil || !bytes.Equal(got, want) {
			continue
		}
		variants++
		w := authorize(mux, payload+"."+alt, testRedirectURI)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("non-canonical tag %q: expected 400, got %d: %s", alt, w.Code, w.Body.String())
		}
	}
	// If this ever hits zero the test has stopped testing anything.
	if variants == 0 {
		t.Fatal("found no non-canonical spellings of the tag; the test proves nothing")
	}
}

// TestVerifyClientID_ChecksTagFirst pins two properties the comment on
// verifyClientID claims and that no behavioural test can see, because every
// path fails closed: the tag is compared before the payload is parsed, and the
// comparison is constant time. Both are defence in depth, so pinning the source
// is enough — the reviewer's point was only that they were asserted and
// unguarded.
func TestVerifyClientID_ChecksTagFirst(t *testing.T) {
	src, err := os.ReadFile("clientid.go")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(src), "func verifyClientID(")
	if start < 0 {
		t.Fatal("verifyClientID not found in clientid.go")
	}
	body := string(src)[start:]
	if end := strings.Index(body, "\nfunc "); end > 0 {
		body = body[:end]
	}

	compare := strings.Index(body, "hmac.Equal")
	if compare < 0 {
		t.Fatal("verifyClientID does not compare the tag with hmac.Equal; a plain string comparison is not constant time")
	}
	for _, after := range []string{"json.Unmarshal", "DecodeString(encodedPayload)", "info.RedirectURIs", "info.IssuedAt"} {
		at := strings.Index(body, after)
		if at < 0 {
			t.Fatalf("verifyClientID no longer contains %q", after)
		}
		if at < compare {
			t.Fatalf("%s runs before the hmac.Equal tag check; an unauthenticated payload must not reach it", after)
		}
	}
}

// TestSignedClientID_EmptyNonce: the nonce is what keeps two registrations of
// the same redirect_uris distinct, so an id minted without one has an identity
// that is not its own. Enforced at verification rather than trusted from the
// mint side.
func TestSignedClientID_EmptyNonce(t *testing.T) {
	mux, server := newChainTestServer(t)
	uri := "http://127.0.0.1:33418/callback"

	// Control: the same mint with a nonce authorises on this server.
	withNonce := mintClientID(server.clientIDKey, clientIDInfo{
		RedirectURIs: []string{uri}, IssuedAt: time.Now().Unix(), Nonce: generateSecureToken(8),
	})
	req := httptest.NewRequest("GET", "/oauth/authorize?client_id="+url.QueryEscape(withNonce)+"&redirect_uri="+url.QueryEscape(uri)+"&state=abc", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("control: a nonced client_id should authorise, got %d: %s", w.Code, w.Body.String())
	}

	without := mintClientID(server.clientIDKey, clientIDInfo{
		RedirectURIs: []string{uri}, IssuedAt: time.Now().Unix(),
	})
	req = httptest.NewRequest("GET", "/oauth/authorize?client_id="+url.QueryEscape(without)+"&redirect_uri="+url.QueryEscape(uri)+"&state=abc", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("a client_id with an empty nonce was accepted: got %d", w.Code)
	}
}
