package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode"
)

// newChainTestServer wires the real route table (RegisterRoutes) against a
// stubbed GitHub, so the tests below drive the whole authorize -> callback ->
// token chain through the mux rather than calling handlers directly.
func newChainTestServer(t *testing.T) (*http.ServeMux, *OAuthServer) {
	t.Helper()
	return newChainTestServerWithSecret(t, "test-secret")
}

// newChainTestServerWithSecret is newChainTestServer with control over the
// GitHub client secret, which is what the client_id signing key is derived
// from. Two servers built with the same secret behave like the same app
// restarted; two built with different secrets like a key rotation.
func newChainTestServerWithSecret(t *testing.T, githubClientSecret string) (*http.ServeMux, *OAuthServer) {
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

	githubClient := newTestGitHubClient(github)
	githubClient.ClientSecret = githubClientSecret
	oauth := NewOAuthServer("http://localhost:8080", githubClient, NewTokenStore(), "navikt")
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

// TestTokenEndpoint_NoStore: RFC 6749 §5.1. A token response carries
// credentials, so no intermediary and no browser cache may keep a copy.
func TestTokenEndpoint_NoStore(t *testing.T) {
	mux, _ := newChainTestServer(t)
	redirectURI := "http://127.0.0.1:33418/callback"
	clientID := registerClient(t, mux, redirectURI)

	_, _, token := runChain(t, mux, clientID, redirectURI)
	if token.Code != http.StatusOK {
		t.Fatalf("precondition: token request failed: %d %s", token.Code, token.Body.String())
	}
	if got := token.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store: a token response must not be cached", got)
	}
	if got := token.Header().Get("Pragma"); got != "no-cache" {
		t.Errorf("Pragma = %q, want no-cache", got)
	}
}

// captureHandler keeps every slog.Record so a test can assert on the attribute
// values the handler passed, rather than on a formatted line: the JSON handler
// used in production escapes a newline on the way out, which would hide an
// unsanitised value.
type captureHandler struct{ records *[]slog.Record }

func (h captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h captureHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h captureHandler) WithGroup(string) slog.Handler            { return h }
func (h captureHandler) Handle(_ context.Context, r slog.Record) error {
	*h.records = append(*h.records, r)
	return nil
}

// captureLogs installs a capturing default logger at debug level for the test.
func captureLogs(t *testing.T) *[]slog.Record {
	t.Helper()
	records := &[]slog.Record{}
	prev := slog.Default()
	slog.SetDefault(slog.New(captureHandler{records: records}))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return records
}

// TestAuthorize_AttackerControlledFieldsAreNotLoggedRaw drives the real
// handler with control characters in every field it logs. client_id,
// redirect_uri and User-Agent are all attacker-chosen and all reach a log line
// in /oauth/authorize, so none of them may arrive there raw: a newline forges a
// second entry and an unbounded field pushes real entries out of a buffer.
func TestAuthorize_AttackerControlledFieldsAreNotLoggedRaw(t *testing.T) {
	mux, _ := newChainTestServer(t)
	records := captureLogs(t)

	evil := "evil\nlevel=ERROR msg=\"forged\"\x1b[2K"
	req := httptest.NewRequest("GET", "/oauth/authorize?client_id="+url.QueryEscape(evil)+
		"&redirect_uri="+url.QueryEscape("http://127.0.0.1:1/"+evil)+"&state=abc", nil)
	req.Header.Set("User-Agent", evil)
	do(mux, req)

	if len(*records) == 0 {
		t.Fatal("the handler logged nothing, so this test would pass vacuously")
	}
	sawClientID := false
	for _, rec := range *records {
		rec.Attrs(func(a slog.Attr) bool {
			v, ok := a.Value.Any().(string)
			if !ok {
				return true
			}
			if a.Key == "client_id" {
				sawClientID = true
			}
			for _, r := range v {
				if unicode.IsControl(r) {
					t.Errorf("log record %q attribute %q kept a control character %q: %q",
						rec.Message, a.Key, r, v)
					break
				}
			}
			if len([]rune(v)) > 129 {
				t.Errorf("log record %q attribute %q is unbounded (%d runes)", rec.Message, a.Key, len([]rune(v)))
			}
			return true
		})
	}
	if !sawClientID {
		t.Fatal("no log record carried a client_id, so this test would pass vacuously")
	}
}

// TestAuthorize_UnknownClientID_BrowserGetsRecoverablePage: the extension sits
// blocked on its loopback callback and never reads this response, so whatever a
// browser is shown here is the only thing the person gets. It must tell them
// how to recover; machine clients keep the JSON.
func TestAuthorize_UnknownClientID_BrowserGetsRecoverablePage(t *testing.T) {
	mux, _ := newChainTestServer(t)

	req := httptest.NewRequest("GET", "/oauth/authorize?client_id=gone-on-deploy&redirect_uri=http://127.0.0.1:33418/callback&state=abc", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	w := do(mux, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("a browser must get HTML, got Content-Type %q and body %q", ct, w.Body.String())
	}
	body := strings.ToLower(w.Body.String())
	for _, want := range []string{"<html", "sign in", "registration"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not mention %q, so it does not tell the person how to recover: %s", want, w.Body.String())
		}
	}

	// A client that parses the body still gets the machine-readable error.
	jsonReq := httptest.NewRequest("GET", "/oauth/authorize?client_id=gone-on-deploy&redirect_uri=http://127.0.0.1:33418/callback&state=abc", nil)
	jsonReq.Header.Set("Accept", "application/json")
	jw := do(mux, jsonReq)
	var errResp map[string]string
	if err := json.NewDecoder(jw.Body).Decode(&errResp); err != nil {
		t.Fatalf("a JSON client must still get JSON, got %q (%v)", jw.Body.String(), err)
	}
	if errResp["error"] != "invalid_client" {
		t.Fatalf("expected error=invalid_client, got %v", errResp)
	}
	if !strings.Contains(strings.ToLower(errResp["error_description"]), "register") {
		t.Errorf("error_description should name the recovery, got %q", errResp["error_description"])
	}
}

// TestIsRegisteredRedirectURI pins the matcher directly. Without this, a
// version that returns true for any non-empty URI against any non-empty
// registration list is only caught indirectly.
func TestIsRegisteredRedirectURI(t *testing.T) {
	for _, tc := range []struct {
		name       string
		registered []string
		uri        string
		want       bool
	}{
		{"exact loopback match", []string{"http://127.0.0.1:33418/callback"}, "http://127.0.0.1:33418/callback", true},
		{"loopback port is ignored (RFC 8252 7.3)", []string{"http://127.0.0.1:33418/callback"}, "http://127.0.0.1:51234/callback", true},
		// The matcher compares against the registration; the loopback policy is
		// enforced separately, by isValidRedirectURI at /register and /authorize.
		{"exact match, any scheme", []string{"https://vscode.dev/redirect"}, "https://vscode.dev/redirect", true},
		{"attacker host", []string{"http://127.0.0.1:33418/callback"}, "https://attacker.example/cb", false},
		{"loopback path must still match", []string{"http://127.0.0.1:33418/callback"}, "http://127.0.0.1:33418/steal", false},
		{"host must match exactly", []string{"https://vscode.dev/redirect"}, "https://vscode.dev.attacker.example/redirect", false},
		{"empty uri is not registered", []string{"http://127.0.0.1:33418/callback"}, "", false},
		{"no registrations at all", nil, "http://127.0.0.1:33418/callback", false},
	} {
		if got := isRegisteredRedirectURI(tc.registered, tc.uri); got != tc.want {
			t.Errorf("%s: isRegisteredRedirectURI(%v, %q) = %v, want %v", tc.name, tc.registered, tc.uri, got, tc.want)
		}
	}
}

// TestTokenEndpoint_CrossClientRedemption_Rejected: a code minted for client A
// must not be redeemable by client B. Only the omitted-client_id case was
// pinned; a mismatched but present client_id is the other half.
func TestTokenEndpoint_CrossClientRedemption_Rejected(t *testing.T) {
	mux, _ := newChainTestServer(t)
	redirectURI := "http://127.0.0.1:33418/callback"
	clientA := registerClient(t, mux, redirectURI)
	clientB := registerClient(t, mux, redirectURI)
	if clientA == clientB {
		t.Fatal("precondition: the two registrations got the same client_id")
	}

	// Mint a code for A, stopping just before the token request.
	authorize := do(mux, httptest.NewRequest("GET", "/oauth/authorize?client_id="+url.QueryEscape(clientA)+
		"&redirect_uri="+url.QueryEscape(redirectURI)+"&state=client-state", nil))
	if authorize.Code != http.StatusFound {
		t.Fatalf("precondition: authorize returned %d: %s", authorize.Code, authorize.Body.String())
	}
	ghURL, err := url.Parse(authorize.Header().Get("Location"))
	if err != nil {
		t.Fatalf("authorize: bad Location: %v", err)
	}
	callback := do(mux, httptest.NewRequest("GET", "/oauth/callback?code=gh-code&state="+
		url.QueryEscape(ghURL.Query().Get("state")), nil))
	if callback.Code != http.StatusFound {
		t.Fatalf("precondition: callback returned %d: %s", callback.Code, callback.Body.String())
	}
	cbURL, err := url.Parse(callback.Header().Get("Location"))
	if err != nil {
		t.Fatalf("callback: bad Location: %v", err)
	}
	code := cbURL.Query().Get("code")
	if code == "" {
		t.Fatal("precondition: no authorization code was minted")
	}

	// Redeem it as B.
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"client_id":    {clientB},
		"redirect_uri": {redirectURI},
	}
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := do(mux, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("client B redeemed client A's code: got %d: %s", w.Code, w.Body.String())
	}
	var errResp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_client" {
		t.Fatalf("expected error=invalid_client, got %v", errResp)
	}
	if errResp["access_token"] != "" {
		t.Fatalf("a token was issued to the wrong client: %v", errResp)
	}
}

// --- refresh_token grant is client-authenticated (GHSA-7hwf-488h-59x8) ---

// issueRefreshToken runs the full chain and returns the refresh_token the
// authorization_code grant handed out.
func issueRefreshToken(t *testing.T, mux *http.ServeMux, clientID, redirectURI string) string {
	t.Helper()
	_, _, token := runChain(t, mux, clientID, redirectURI)
	if token == nil || token.Code != http.StatusOK {
		t.Fatalf("chain did not reach a token: %v", token)
	}
	var resp map[string]any
	if err := json.NewDecoder(token.Body).Decode(&resp); err != nil {
		t.Fatalf("token: decode: %v", err)
	}
	rt, _ := resp["refresh_token"].(string)
	if rt == "" {
		t.Fatalf("token: expected a refresh_token, got %v", resp)
	}
	return rt
}

// refresh posts a refresh_token grant. An empty clientID omits the parameter.
func refresh(mux *http.ServeMux, refreshToken, clientID string) *httptest.ResponseRecorder {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	if clientID != "" {
		form.Set("client_id", clientID)
	}
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(mux, req)
}

// TestRefreshGrant_SameClient_Succeeds is the control for the three rejection
// tests below: the refresh grant still works for the client it was issued to.
func TestRefreshGrant_SameClient_Succeeds(t *testing.T) {
	mux, _ := newChainTestServer(t)
	redirectURI := "http://127.0.0.1:33418/callback"
	clientID := registerClient(t, mux, redirectURI)

	w := refresh(mux, issueRefreshToken(t, mux, clientID, redirectURI), clientID)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("refresh: decode: %v", err)
	}
	if resp["access_token"] == nil || resp["access_token"] == "" {
		t.Fatalf("refresh: expected an access_token, got %v", resp)
	}
	if resp["refresh_token"] == nil || resp["refresh_token"] == "" {
		t.Fatalf("refresh: expected a rotated refresh_token, got %v", resp)
	}
}

// TestRefreshGrant_OtherClient_Rejected is the substantive one: a refresh token
// stolen from client A must not be redeemable by client B. Without the binding
// the refresh grant asks for no client at all, so anyone holding the token can
// mint access tokens for 30 days (GHSA-7hwf-488h-59x8).
func TestRefreshGrant_OtherClient_Rejected(t *testing.T) {
	mux, _ := newChainTestServer(t)
	victimRedirect := "http://127.0.0.1:33418/callback"
	victimClient := registerClient(t, mux, victimRedirect)
	attackerClient := registerClient(t, mux, "http://127.0.0.1:44444/callback")

	rt := issueRefreshToken(t, mux, victimClient, victimRedirect)

	w := refresh(mux, rt, attackerClient)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("refresh as other client: expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var errResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("refresh: decode: %v", err)
	}
	if errResp["error"] != "invalid_client" {
		t.Fatalf("refresh: expected invalid_client, got %v", errResp)
	}

	// A rejected attempt must not burn the victim's token.
	if w := refresh(mux, rt, victimClient); w.Code != http.StatusOK {
		t.Fatalf("victim refresh after rejected attempt: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRefreshGrant_MissingClientID_Rejected(t *testing.T) {
	mux, _ := newChainTestServer(t)
	redirectURI := "http://127.0.0.1:33418/callback"
	clientID := registerClient(t, mux, redirectURI)

	w := refresh(mux, issueRefreshToken(t, mux, clientID, redirectURI), "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("refresh without client_id: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRefreshGrant_UnboundToken_Rejected pins the transition decision: a
// refresh token carrying no client_id is rejected rather than grandfathered.
// Nothing is grandfathered in practice — the store is in-memory, so a restart
// drops every refresh token along with every client registration — and an
// unbound token accepted on trust is exactly the hole being closed.
func TestRefreshGrant_UnboundToken_Rejected(t *testing.T) {
	mux, oauth := newChainTestServer(t)
	clientID := registerClient(t, mux, "http://127.0.0.1:33418/callback")

	oauth.Store.SaveRefreshToken("legacy-unbound", &RefreshTokenData{
		GitHubRefreshToken: "ghr_victim",
		UserLogin:          "victim",
		UserID:             42,
		CreatedAt:          time.Now(),
	})

	for _, cid := range []string{"", clientID} {
		w := refresh(mux, "legacy-unbound", cid)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("unbound refresh token with client_id=%q: expected 400, got %d: %s", cid, w.Code, w.Body.String())
		}
	}
}

// registerStatus runs Dynamic Client Registration and returns the recorder,
// for the cases where the registration is expected to fail.
func registerStatus(mux *http.ServeMux, redirectURIs ...string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(map[string]any{"redirect_uris": redirectURIs})
	return do(mux, httptest.NewRequest("POST", "/register", bytes.NewReader(b)))
}

// TestRegister_LoopbackOnlyPolicy is the fix for #633. /register is
// unauthenticated (RFC 7591, which is how MCP clients expect to onboard), so
// "registered" proves only that the server minted the id — an attacker buys one
// for a single POST and then runs the GHSA-7hwf-488h-59x8 phishing flow with a
// redirect_uri it controls. Restricting public clients to the loopback
// interface (RFC 8252 section 7.3) takes the destination out of the attacker's
// reach instead: a code delivered to 127.0.0.1 lands on the victim's own
// machine.
//
// The accepted cases sit beside the rejected ones deliberately: a policy that
// refuses everything would pass the rejection half on its own.
func TestRegister_LoopbackOnlyPolicy(t *testing.T) {
	mux, _ := newChainTestServer(t)

	for _, uri := range []string{
		"http://127.0.0.1:33418",
		"http://127.0.0.1:51234/callback",
		"http://127.0.0.1/callback",
		"http://[::1]:33418/callback",
		"http://localhost:3000/callback",
	} {
		if w := registerStatus(mux, uri); w.Code != http.StatusCreated {
			t.Errorf("accepted-case %q: expected 201, got %d: %s", uri, w.Code, w.Body.String())
		}
	}

	for _, uri := range []string{
		"https://attacker.example/cb",
		"https://vscode.dev/redirect",
		"http://evil.example.com/callback",
		"http://127.0.0.1.attacker.example/cb",
		"http://0.0.0.0:8080",
		"ftp://127.0.0.1:8080",
		"not-a-url",
	} {
		w := registerStatus(mux, uri)
		if w.Code != http.StatusBadRequest {
			t.Errorf("rejected-case %q: expected 400, got %d: %s", uri, w.Code, w.Body.String())
			continue
		}
		var resp map[string]string
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp["error"] != "invalid_redirect_uri" {
			t.Errorf("rejected-case %q: expected error invalid_redirect_uri, got %q", uri, resp["error"])
		}
	}

	// One bad URI poisons the whole registration; a loopback URI alongside it
	// must not launder it in.
	if w := registerStatus(mux, "http://127.0.0.1:33418", "https://attacker.example/cb"); w.Code != http.StatusBadRequest {
		t.Errorf("mixed registration: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuthorize_PreExistingHTTPSClientID_Rejected is the defence in depth.
// Registrations are not stored — a client_id is a signed blob with a 30-day TTL
// — so ones minted before this policy stay verifiable until they expire. The
// policy therefore has to hold at /oauth/authorize too, and it has to fail on
// the recoverable path: invalid_client tells the editor to re-register, where a
// bare error would leave it waiting on a loopback callback that never arrives.
func TestAuthorize_PreExistingHTTPSClientID_Rejected(t *testing.T) {
	mux, server := newChainTestServer(t)

	legacy := mintClientID(server.clientIDKey, clientIDInfo{
		RedirectURIs: []string{"https://attacker.example/cb"},
		IssuedAt:     time.Now().Unix(),
		Nonce:        generateSecureToken(8),
	})
	w := do(mux, httptest.NewRequest("GET", "/oauth/authorize?client_id="+url.QueryEscape(legacy)+
		"&redirect_uri="+url.QueryEscape("https://attacker.example/cb")+"&state=abc", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a signed client_id with an https redirect_uri, got %d: %s", w.Code, w.Body.String())
	}
	assertInvalidClient(t, w)

	// Control: the identical construction with a loopback redirect_uri still
	// authorises, so the rejection is the policy and not the minting path.
	fresh := mintClientID(server.clientIDKey, clientIDInfo{
		RedirectURIs: []string{testRedirectURI},
		IssuedAt:     time.Now().Unix(),
		Nonce:        generateSecureToken(8),
	})
	if w := do(mux, httptest.NewRequest("GET", "/oauth/authorize?client_id="+url.QueryEscape(fresh)+
		"&redirect_uri="+url.QueryEscape(testRedirectURI)+"&state=abc", nil)); w.Code != http.StatusFound {
		t.Fatalf("control: a loopback client_id should authorise, got %d: %s", w.Code, w.Body.String())
	}
}

// TestOAuthChain_IPv6LoopbackClient_IssuesToken: the whole chain still works
// end to end for a loopback client, on the IPv6 form the policy newly names.
func TestOAuthChain_IPv6LoopbackClient_IssuesToken(t *testing.T) {
	mux, _ := newChainTestServer(t)
	redirectURI := "http://[::1]:33418/callback"
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
}
