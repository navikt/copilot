package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const revokeRedirect = "http://127.0.0.1:33418/callback"

// issueTokens runs the full chain and returns the access and refresh token.
func issueTokens(t *testing.T, mux *http.ServeMux, clientID string) (access, refreshTok string) {
	t.Helper()
	_, _, token := runChain(t, mux, clientID, revokeRedirect)
	if token == nil || token.Code != http.StatusOK {
		t.Fatalf("chain did not reach a token: %v", token)
	}
	return decodeTokens(t, token)
}

func decodeTokens(t *testing.T, w *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	a, _ := resp["access_token"].(string)
	r, _ := resp["refresh_token"].(string)
	if a == "" || r == "" {
		t.Fatalf("missing tokens in %v", resp)
	}
	return a, r
}

func revoke(mux *http.ServeMux, token, clientID string) *httptest.ResponseRecorder {
	form := url.Values{"token": {token}}
	if clientID != "" {
		form.Set("client_id", clientID)
	}
	req := httptest.NewRequest("POST", "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(mux, req)
}

func TestRefresh_InvalidatesOldAccessToken(t *testing.T) {
	mux, oauth := newChainTestServer(t)
	clientID := registerClient(t, mux, revokeRedirect)
	oldAT, oldRT := issueTokens(t, mux, clientID)

	w := refresh(mux, oldRT, clientID)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", w.Code, w.Body.String())
	}
	newAT, newRT := decodeTokens(t, w)
	if _, err := oauth.Store.GetToken(oldAT); err == nil {
		t.Fatal("old access token still valid after refresh")
	}
	if _, err := oauth.Store.GetRefreshToken(oldRT); err == nil {
		t.Fatal("old refresh token still valid after refresh")
	}
	if _, err := oauth.Store.GetToken(newAT); err != nil {
		t.Fatal("new access token not valid")
	}
	// Second refresh retires the first refreshed access token too.
	if w := refresh(mux, newRT, clientID); w.Code != http.StatusOK {
		t.Fatalf("second refresh: %d", w.Code)
	}
	if _, err := oauth.Store.GetToken(newAT); err == nil {
		t.Fatal("access token from first refresh still valid after second refresh")
	}
}

func TestRevoke_BothTokenTypes(t *testing.T) {
	for _, kind := range []string{"access", "refresh"} {
		t.Run(kind, func(t *testing.T) {
			mux, oauth := newChainTestServer(t)
			clientID := registerClient(t, mux, revokeRedirect)
			at, rt := issueTokens(t, mux, clientID)
			tok := at
			if kind == "refresh" {
				tok = rt
			}
			if w := revoke(mux, tok, clientID); w.Code != http.StatusOK {
				t.Fatalf("revoke: %d %s", w.Code, w.Body.String())
			}
			if _, err := oauth.Store.GetToken(at); err == nil {
				t.Fatal("access token survived revocation")
			}
			if _, err := oauth.Store.GetRefreshToken(rt); err == nil {
				t.Fatal("refresh token survived revocation")
			}
			if w := refresh(mux, rt, clientID); w.Code == http.StatusOK {
				t.Fatal("revoked refresh token still refreshes")
			}
		})
	}
}

func TestRevoke_OtherClientRefused(t *testing.T) {
	mux, oauth := newChainTestServer(t)
	victim := registerClient(t, mux, revokeRedirect)
	attacker := registerClient(t, mux, "http://127.0.0.1:44444/callback")
	at, rt := issueTokens(t, mux, victim)

	for _, tok := range []string{at, rt} {
		w := revoke(mux, tok, attacker)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_client") {
			t.Fatalf("cross-client revoke: %d %s", w.Code, w.Body.String())
		}
	}
	if w := revoke(mux, at, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("revoke without client_id: %d", w.Code)
	}
	if _, err := oauth.Store.GetToken(at); err != nil {
		t.Fatal("refused revoke removed the access token")
	}
	if _, err := oauth.Store.GetRefreshToken(rt); err != nil {
		t.Fatal("refused revoke removed the refresh token")
	}
}

func TestRevoke_UnknownTokenReturns200(t *testing.T) {
	mux, _ := newChainTestServer(t)
	clientID := registerClient(t, mux, revokeRedirect)
	if w := revoke(mux, "no-such-token", clientID); w.Code != http.StatusOK {
		t.Fatalf("unknown token: %d %s", w.Code, w.Body.String())
	}
}

// Tokens issued before #2746 carry no ClientID or pairing. They still validate,
// and a refresh token bound to a client can still be revoked by it.
func TestRevoke_LegacyTokens(t *testing.T) {
	mux, oauth := newChainTestServer(t)
	clientID := registerClient(t, mux, revokeRedirect)
	oauth.Store.SaveRefreshToken("legacy-rt", &RefreshTokenData{ClientID: clientID, UserLogin: "u"})
	if w := revoke(mux, "legacy-rt", clientID); w.Code != http.StatusOK {
		t.Fatalf("legacy revoke: %d", w.Code)
	}
	if _, ok := oauth.Store.refreshTokens["legacy-rt"]; ok {
		t.Fatal("legacy refresh token not revoked")
	}
}

func TestMetadata_AdvertisesRevocationEndpoint(t *testing.T) {
	mux, oauth := newChainTestServer(t)
	w := do(mux, httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil))
	if !strings.Contains(w.Body.String(), `"revocation_endpoint":"`+oauth.BaseURL+`/oauth/revoke"`) {
		t.Fatalf("metadata: %s", w.Body.String())
	}
}
