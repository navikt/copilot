package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/navikt/copilot/mcp-onboarding/internal/github"
	"github.com/navikt/copilot/mcp-onboarding/internal/store"
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
	oauth.Store.SaveRefreshToken("legacy-rt", &store.RefreshTokenData{ClientID: clientID, UserLogin: "u"})
	if w := revoke(mux, "legacy-rt", clientID); w.Code != http.StatusOK {
		t.Fatalf("legacy revoke: %d", w.Code)
	}
	if _, _, refreshTokens := oauth.Store.Sizes(); refreshTokens != 0 {
		t.Fatal("legacy refresh token not revoked")
	}
}

func TestMetadata_AdvertisesRevocationEndpoint(t *testing.T) {
	mux, oauth := newChainTestServer(t)
	w := do(mux, httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil))
	if !strings.Contains(w.Body.String(), `"revocation_endpoint":"`+oauth.BaseURL+`/oauth/revoke"`) || !strings.Contains(w.Body.String(), `"revocation_endpoint_auth_methods_supported":["none"]`) {
		t.Fatalf("metadata: %s", w.Body.String())
	}
}

// newServerWithRefreshHook is newChainTestServer, but onRefresh runs inside
// GitHub's refresh call: after the handler read the refresh token, before it saves.
func newServerWithRefreshHook(t *testing.T, onRefresh func()) (*http.ServeMux, *OAuthServer) {
	t.Helper()
	github := newGitHubMock(t, map[string]http.HandlerFunc{
		"POST /login/oauth/access_token": func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			if r.PostFormValue("grant_type") == "refresh_token" {
				onRefresh()
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "gho_victim", "refresh_token": "ghr_victim",
				"expires_in": 28800, "token_type": "bearer",
			})
		},
		"GET /user": func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(github.User{ID: 42, Login: "victim"})
		},
		"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode([]github.Org{{ID: 1, Login: "navikt"}})
		},
	})
	gh := newTestGitHubClient(github)
	gh.ClientSecret = "test-secret"
	oauth := NewOAuthServer("http://localhost:8080", gh, store.NewTokenStore(), "navikt")
	mux := http.NewServeMux()
	oauth.RegisterRoutes(mux)
	return mux, oauth
}

func TestRefresh_RevokeDuringRefreshWins(t *testing.T) {
	var hook func()
	mux, oauth := newServerWithRefreshHook(t, func() { hook() })
	clientID := registerClient(t, mux, revokeRedirect)
	at, rt := issueTokens(t, mux, clientID)
	hook = func() {
		if w := revoke(mux, rt, clientID); w.Code != http.StatusOK {
			t.Errorf("revoke: %d", w.Code)
		}
	}
	w := refresh(mux, rt, clientID)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_grant") {
		t.Fatalf("refresh after concurrent revoke: status %d", w.Code) // body would hold tokens
	}
	_, tokens, refreshTokens := oauth.Store.Sizes()
	if tokens != 0 || refreshTokens != 0 {
		_, err := oauth.Store.GetToken(at)
		t.Fatalf("store not empty after revoke: %d access, %d refresh (old access %v)",
			tokens, refreshTokens, err == nil)
	}
}

func TestRefresh_ConcurrentDoubleRefreshOneWins(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)
	// Both refreshes must have read the token before either saves.
	mux, _ := newServerWithRefreshHook(t, func() { wg.Done(); wg.Wait() })
	clientID := registerClient(t, mux, revokeRedirect)
	_, rt := issueTokens(t, mux, clientID)

	codes := make(chan int, 2)
	for range 2 {
		go func() { codes <- refresh(mux, rt, clientID).Code }()
	}
	ok := 0
	for range 2 {
		if <-codes == http.StatusOK {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("expected exactly one successful refresh, got %d", ok)
	}
}

func TestRevoke_PurgedAccessTokenRevokesRefresh(t *testing.T) {
	mux, oauth := newChainTestServer(t)
	clientID := registerClient(t, mux, revokeRedirect)
	at, rt := issueTokens(t, mux, clientID)
	oauth.Store.DeleteToken(at)
	if w := revoke(mux, at, clientID); w.Code != http.StatusOK {
		t.Fatalf("revoke: %d", w.Code)
	}
	if _, err := oauth.Store.GetRefreshToken(rt); err == nil {
		t.Fatal("refresh token survived revocation of its purged access token")
	}
}
