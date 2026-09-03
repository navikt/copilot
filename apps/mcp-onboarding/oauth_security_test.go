package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// newChainTestServer wires the real route table (RegisterRoutes) against a
// stubbed GitHub, so the tests below drive the whole authorize -> callback ->
// token chain through the mux rather than calling handlers directly.
func newChainTestServer(t *testing.T) (*http.ServeMux, *OAuthServer) {
	t.Helper()

	github := newGitHubMock(t, map[string]http.HandlerFunc{
		"POST /login/oauth/access_token": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "gho_victim",
				"refresh_token": "ghr_victim",
				"expires_in":    28800,
				"token_type":    "bearer",
			})
		},
		"GET /user": func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(GitHubUser{ID: 42, Login: "victim"})
		},
		"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode([]GitHubOrg{{ID: 1, Login: "navikt"}})
		},
	})

	oauth := NewOAuthServer("http://localhost:8080", newTestGitHubClient(github), NewTokenStore(), "navikt")
	mux := http.NewServeMux()
	oauth.RegisterRoutes(mux)
	return mux, oauth
}

func do(mux *http.ServeMux, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

// registerClient runs Dynamic Client Registration through the mux and returns
// the issued client_id.
func registerClient(t *testing.T, mux *http.ServeMux, redirectURI string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"client_name":   "test client",
		"redirect_uris": []string{redirectURI},
	})
	w := do(mux, httptest.NewRequest("POST", "/register", bytes.NewReader(b)))
	if w.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var reg ClientRegistration
	if err := json.NewDecoder(w.Body).Decode(&reg); err != nil {
		t.Fatalf("register: decode: %v", err)
	}
	return reg.ClientID
}

// runChain drives authorize -> callback -> token and returns the recorders for
// each step. Steps after a non-redirecting authorize are skipped (nil).
func runChain(t *testing.T, mux *http.ServeMux, clientID, redirectURI string) (authorize, callback, token *httptest.ResponseRecorder) {
	t.Helper()

	authorize = do(mux, httptest.NewRequest("GET", "/oauth/authorize?client_id="+url.QueryEscape(clientID)+
		"&redirect_uri="+url.QueryEscape(redirectURI)+"&state=client-state", nil))
	if authorize.Code != http.StatusFound {
		return authorize, nil, nil
	}

	// The redirect goes to GitHub; the internal state is what ties the session
	// to the callback.
	ghURL, err := url.Parse(authorize.Header().Get("Location"))
	if err != nil {
		t.Fatalf("authorize: bad Location: %v", err)
	}
	internalState := ghURL.Query().Get("state")

	callback = do(mux, httptest.NewRequest("GET", "/oauth/callback?code=gh-code&state="+url.QueryEscape(internalState), nil))
	if callback.Code != http.StatusFound {
		return authorize, callback, nil
	}

	cbURL, err := url.Parse(callback.Header().Get("Location"))
	if err != nil {
		t.Fatalf("callback: bad Location: %v", err)
	}
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {cbURL.Query().Get("code")},
		"client_id":    {clientID},
		"redirect_uri": {redirectURI},
	}
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	token = do(mux, req)
	return authorize, callback, token
}

// TestOAuthChain_RegisteredClient_IssuesToken is the control: it proves the
// harness can actually reach an access token, so the rejection asserted by
// TestOAuthChain_UnregisteredClientID_Rejected means something.
func TestOAuthChain_RegisteredClient_IssuesToken(t *testing.T) {
	mux, _ := newChainTestServer(t)
	redirectURI := "http://127.0.0.1:33418/callback"
	clientID := registerClient(t, mux, redirectURI)

	authorize, callback, token := runChain(t, mux, clientID, redirectURI)

	if authorize.Code != http.StatusFound {
		t.Fatalf("authorize: expected 302, got %d: %s", authorize.Code, authorize.Body.String())
	}
	if callback.Code != http.StatusFound {
		t.Fatalf("callback: expected 302, got %d: %s", callback.Code, callback.Body.String())
	}
	if got := callback.Header().Get("Location"); !strings.HasPrefix(got, redirectURI+"?code=") {
		t.Fatalf("callback: expected redirect to %s with code, got %q", redirectURI, got)
	}
	if token.Code != http.StatusOK {
		t.Fatalf("token: expected 200, got %d: %s", token.Code, token.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(token.Body).Decode(&resp); err != nil {
		t.Fatalf("token: decode: %v", err)
	}
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Fatalf("token: expected an access_token, got %v", resp)
	}
}

// TestOAuthChain_UnregisteredClientID_Rejected covers GHSA-7hwf-488h-59x8: an
// unregistered client_id must not carry an attacker-chosen redirect_uri
// through the chain.
func TestOAuthChain_UnregisteredClientID_Rejected(t *testing.T) {
	mux, oauth := newChainTestServer(t)
	attackerURI := "https://attacker.example/cb"

	authorize, callback, token := runChain(t, mux, "never-registered", attackerURI)

	if authorize.Code != http.StatusBadRequest {
		t.Fatalf("authorize: expected 400 for unregistered client_id, got %d (Location=%q)",
			authorize.Code, authorize.Header().Get("Location"))
	}
	var errResp map[string]string
	if err := json.NewDecoder(authorize.Body).Decode(&errResp); err != nil {
		t.Fatalf("authorize: expected a JSON error body, got %q (%v)", authorize.Body.String(), err)
	}
	if errResp["error"] != "invalid_client" {
		t.Fatalf("authorize: expected error=invalid_client so the client re-runs DCR, got %v", errResp)
	}
	if callback != nil || token != nil {
		t.Fatal("chain continued past authorize")
	}

	// No session may have been created with the attacker's redirect_uri.
	oauth.Store.mu.RLock()
	sessions := len(oauth.Store.authSessions)
	oauth.Store.mu.RUnlock()
	if sessions != 0 {
		t.Fatalf("expected no auth session for a rejected client_id, got %d", sessions)
	}
}

// TestHandleAuthorize_EmptyRedirectURI_Rejected: an empty redirect_uri must be
// rejected too, not fall through the validation.
func TestHandleAuthorize_EmptyRedirectURI_Rejected(t *testing.T) {
	mux, _ := newChainTestServer(t)
	clientID := registerClient(t, mux, "http://127.0.0.1:33418/callback")

	w := do(mux, httptest.NewRequest("GET", "/oauth/authorize?client_id="+clientID+"&state=abc", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty redirect_uri, got %d (Location=%q)", w.Code, w.Header().Get("Location"))
	}
}

// TestTokenEndpoint_MissingClientID_Rejected: omitting client_id entirely must
// not bypass the client_id check (oauth.go compared only when both sides were
// non-empty).
func TestTokenEndpoint_MissingClientID_Rejected(t *testing.T) {
	mux, oauth := newChainTestServer(t)

	oauth.Store.SaveAuthCode("stolen-code", &AuthCode{
		ClientID:    "legit-client",
		RedirectURI: "http://127.0.0.1:33418/callback",
		UserLogin:   "victim",
		UserID:      42,
		CreatedAt:   time.Now(),
	})

	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {"stolen-code"},
		"redirect_uri": {"http://127.0.0.1:33418/callback"},
		// no client_id at all
	}
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := do(mux, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when client_id is omitted, got %d: %s", w.Code, w.Body.String())
	}
	var errResp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_client" {
		t.Fatalf("expected error=invalid_client, got %v", errResp)
	}
}

// TestTokenEndpoint_NoWildcardCORS: /oauth/token must not hand a wildcard CORS
// grant to arbitrary web pages; that is what let a code be redeemed from the
// victim's browser.
func TestTokenEndpoint_NoWildcardCORS(t *testing.T) {
	mux, _ := newChainTestServer(t)

	form := url.Values{"grant_type": {"authorization_code"}, "code": {"nope"}}
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://attacker.example")
	w := do(mux, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("POST /oauth/token: expected no Access-Control-Allow-Origin, got %q", got)
	}

	pre := httptest.NewRequest("OPTIONS", "/oauth/token", nil)
	pre.Header.Set("Origin", "https://attacker.example")
	wp := do(mux, pre)
	if got := wp.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("OPTIONS /oauth/token: expected no Access-Control-Allow-Origin, got %q", got)
	}
}
