package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type testEnv struct {
	t                  *testing.T
	gw                 *Gateway
	handler            http.Handler
	clientID           string
	redirectURI        string
	resource           string
	now                time.Time
	mu                 sync.Mutex
	entraToken         string
	entraRefreshToken  string
	entraStatus        int
	entraRefreshStatus int
	entraRefreshCount  int
	entraSeenRTs       []string
	entraRefreshOmitRT bool
	entraRefreshHook   func()
	entraExpiresIn     int
	lastEntraNonce     string
	lastEntraChallenge string
	navIdent           string
	groups             []string
	cimdServer         *httptest.Server
	entraServer        *httptest.Server
	texasServer        *httptest.Server
	upstreamServer     *httptest.Server
}

func makeTestJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payloadBytes, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payload + ".sig"
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	te := &testEnv{
		t:           t,
		redirectURI: "http://127.0.0.1:38249/callback",
		now:         time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC),
		entraToken:  strings.Join([]string{"entra", "test", "access"}, "-"),
		entraStatus: http.StatusOK,
		navIdent:    "Z123456",
		groups:      []string{"group-allowed"},
	}

	te.cimdServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth-client.json":
			writeJSON(w, http.StatusOK, map[string]any{
				"client_id":                  te.cimdServer.URL + "/oauth-client.json",
				"client_name":                "cplt test client",
				"redirect_uris":              []string{te.redirectURI},
				"grant_types":                []string{"authorization_code"},
				"response_types":             []string{"code"},
				"token_endpoint_auth_method": "none",
			})
		case "/other-client.json":
			writeJSON(w, http.StatusOK, map[string]any{
				"client_id":                  te.cimdServer.URL + "/other-client.json",
				"client_name":                "other test client",
				"redirect_uris":              []string{te.redirectURI},
				"grant_types":                []string{"authorization_code", "refresh_token"},
				"response_types":             []string{"code"},
				"token_endpoint_auth_method": "none",
			})
		case "/bad-client-id.json":
			writeJSON(w, http.StatusOK, map[string]any{
				"client_id":                  "https://evil.example/other.json",
				"client_name":                "spoofed",
				"redirect_uris":              []string{te.redirectURI},
				"token_endpoint_auth_method": "none",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(te.cimdServer.Close)

	te.entraServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(w, http.StatusOK, map[string]any{
				"authorization_endpoint": te.entraServer.URL + "/authorize",
				"token_endpoint":         te.entraServer.URL + "/token",
			})
		case "/token":
			_ = r.ParseForm()
			if r.FormValue("grant_type") == "refresh_token" {
				te.mu.Lock()
				te.entraRefreshCount++
				idx := te.entraRefreshCount
				te.entraSeenRTs = append(te.entraSeenRTs, r.FormValue("refresh_token"))
				status := te.entraRefreshStatus
				if status == 0 {
					status = te.entraStatus
				}
				omitRT := te.entraRefreshOmitRT
				expiresIn := te.entraExpiresIn
				hook := te.entraRefreshHook
				te.mu.Unlock()
				if hook != nil {
					hook()
				}
				if expiresIn <= 0 {
					expiresIn = 3600
				}
				if status != http.StatusOK {
					writeJSON(w, status, map[string]any{
						"error":             "invalid_grant",
						"error_description": "AADSTS700082: The refresh token has expired due to inactivity",
					})
					return
				}
				resp := map[string]any{
					"access_token": fmt.Sprintf("entra-refreshed-access-%d", idx),
					"token_type":   "Bearer",
					"expires_in":   expiresIn,
				}
				if !omitRT {
					resp["refresh_token"] = fmt.Sprintf("entra-rotated-refresh-%d", idx)
				}
				writeJSON(w, http.StatusOK, resp)
				return
			}
			te.mu.Lock()
			entraStatus := te.entraStatus
			expectedChallenge := te.lastEntraChallenge
			nonce := te.lastEntraNonce
			navIdent := te.navIdent
			groups := te.groups
			entraToken := te.entraToken
			entraRT := te.entraRefreshToken
			expiresIn := te.entraExpiresIn
			te.mu.Unlock()
			if entraStatus != http.StatusOK {
				writeJSON(w, entraStatus, map[string]any{
					"error":             "invalid_grant",
					"error_description": "AADSTS65001: user has not consented",
				})
				return
			}
			if expectedChallenge != "" {
				verifier := r.FormValue("code_verifier")
				if verifier == "" || pkceS256(verifier) != expectedChallenge {
					writeJSON(w, http.StatusBadRequest, map[string]any{
						"error":             "invalid_grant",
						"error_description": "PKCE code_verifier mismatch",
					})
					return
				}
			}
			claims := map[string]any{
				"sub":      "user-123",
				"NAVident": navIdent,
				"groups":   groups,
			}
			if nonce != "" {
				claims["nonce"] = nonce
			}
			if entraRT == "" {
				entraRT = strings.Join([]string{"entra", "test", "refresh"}, "-")
			}
			if expiresIn <= 0 {
				expiresIn = 3600
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token":  entraToken,
				"refresh_token": entraRT,
				"id_token":      makeTestJWT(claims),
				"token_type":    "Bearer",
				"expires_in":    expiresIn,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(te.entraServer.Close)

	te.texasServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token": strings.Join([]string{"team", "obo", "token"}, "-"),
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	t.Cleanup(te.texasServer.Close)

	te.upstreamServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
	}))
	t.Cleanup(te.upstreamServer.Close)

	te.clientID = te.cimdServer.URL + "/oauth-client.json"
	baseURL := "https://fleet-gateway.intern.dev.nav.no"
	te.resource = baseURL + "/mcp/team-etterlevelse/etterlevelse"

	cfg := &Config{
		Port:                "8080",
		LogLevel:            slog.LevelInfo,
		BaseURL:             baseURL,
		AllowedClientIDs:    map[string]bool{te.clientID: true},
		AllowedRedirectURIs: map[string]bool{te.redirectURI: true},
		Routes: []PilotRoute{
			{
				Route:       "/mcp/team-etterlevelse/etterlevelse",
				Team:        "team-etterlevelse",
				Target:      "dev-gcp:team-etterlevelse:nav-etterlevelse-mcp",
				UpstreamURL: "http://nav-etterlevelse-mcp.team-etterlevelse",
			},
		},
		TeamTokenStrategy:       strategyOBO,
		TexasExchangeEndpoint:   te.texasServer.URL,
		EntraScope:              "openid",
		AzureClientID:           "azure-client-id",
		AzureClientSecret:       "azure-client-secret",
		AzureWellKnownURL:       te.entraServer.URL + "/.well-known/openid-configuration",
		AzureTokenEndpoint:      te.entraServer.URL + "/token",
		CookieSecret:            []byte("0123456789abcdef0123456789abcdef"),
		HTTPClient:              newHardenedHTTPClient(5 * time.Second),
		ProxyHeaderTimeout:      5 * time.Second,
		PendingCap:              1024,
		CodeCap:                 1024,
		SessionCap:              4096,
		CIMDCap:                 64,
		RateLimitCap:            4096,
		RateLimitRequests:       100,
		RateLimitGlobalRequests: 500,
		RateLimitWindow:         time.Minute,
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig failed: %v", err)
	}
	parsedUpstream, err := url.Parse(te.upstreamServer.URL)
	if err != nil {
		t.Fatalf("parse upstream server URL: %v", err)
	}
	cfg.Routes[0].ParsedURL = parsedUpstream

	gw, err := newGateway(cfg, func() time.Time {
		te.mu.Lock()
		defer te.mu.Unlock()
		return te.now
	})
	if err != nil {
		t.Fatalf("newGateway failed: %v", err)
	}
	te.gw = gw
	te.handler = te.gw.routes()
	return te
}

func (te *testEnv) advance(d time.Duration) {
	te.mu.Lock()
	defer te.mu.Unlock()
	te.now = te.now.Add(d)
}

func testPKCE(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func (te *testEnv) authorizeURL(overrides map[string]string, verifier string) string {
	v := url.Values{
		"client_id":             {te.clientID},
		"redirect_uri":          {te.redirectURI},
		"response_type":         {"code"},
		"code_challenge":        {testPKCE(verifier)},
		"code_challenge_method": {"S256"},
		"state":                 {"client-state-123"},
		"resource":              {te.resource},
	}
	for k, val := range overrides {
		if val == "" {
			v.Del(k)
		} else {
			v.Set(k, val)
		}
	}
	return "/authorize?" + v.Encode()
}

var (
	reID   = regexp.MustCompile(`name="id"\s+value="([^"]+)"`)
	reCSRF = regexp.MustCompile(`name="csrf"\s+value="([^"]+)"`)
)

// startLogin runs GET /authorize, which goes straight to Entra; consent is
// asked after the callback.
func (te *testEnv) startLogin(verifier string) (entraState string, stateCookie *http.Cookie) {
	req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
	return te.followToEntra(req)
}

func (te *testEnv) followToEntra(req *http.Request) (entraState string, stateCookie *http.Cookie) {
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	resp := rec.Result()
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		te.t.Fatalf("expected 302 redirect to Entra, got %d", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		te.t.Fatalf("invalid Entra redirect location: %v", err)
	}
	for _, c := range resp.Cookies() {
		if c.Name == entraStateCookieName {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		te.t.Fatal("expected fleet_entra_state cookie")
	}
	te.mu.Lock()
	te.lastEntraNonce = loc.Query().Get("nonce")
	te.lastEntraChallenge = loc.Query().Get("code_challenge")
	te.mu.Unlock()
	return loc.Query().Get("state"), stateCookie
}

func (te *testEnv) callback(entraState string, stateCookie *http.Cookie) *http.Response {
	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=entra-code-123&state="+url.QueryEscape(entraState), nil)
	if stateCookie != nil {
		req.AddCookie(stateCookie)
	}
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	return rec.Result()
}

// parseConsentPage reads the consent form from a 200 response.
func (te *testEnv) parseConsentPage(resp *http.Response) (consentID, csrf string, csrfCookie *http.Cookie, headers http.Header) {
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		te.t.Fatalf("expected 200 consent page, got %d: %s", resp.StatusCode, string(bodyBytes))
	}
	body := string(bodyBytes)
	idMatch := reID.FindStringSubmatch(body)
	csrfMatch := reCSRF.FindStringSubmatch(body)
	if len(idMatch) != 2 || len(csrfMatch) != 2 {
		te.t.Fatalf("missing hidden fields in consent HTML: %s", body)
	}
	for _, c := range resp.Cookies() {
		if c.Name == csrfCookieName {
			csrfCookie = c
		}
	}
	if csrfCookie == nil {
		te.t.Fatal("expected fleet_csrf cookie on consent page")
	}
	return idMatch[1], csrfMatch[1], csrfCookie, resp.Header.Clone()
}

// startAuthorizeAndConsent logs in through Entra and returns the consent page
// shown after the callback. It fails if consent was remembered.
func (te *testEnv) startAuthorizeAndConsent(verifier string) (consentID, csrf string, csrfCookie *http.Cookie, headers http.Header) {
	return te.parseConsentPage(te.callback(te.startLogin(verifier))) //nolint:bodyclose // parseConsentPage closes it
}

func (te *testEnv) postConsent(consentID, csrf string, csrfCookie *http.Cookie, decision string) *http.Response {
	form := url.Values{"id": {consentID}, "csrf": {csrf}, "decision": {decision}}
	req := httptest.NewRequest(http.MethodPost, "/consent", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if csrfCookie != nil {
		req.AddCookie(csrfCookie)
	}
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	return rec.Result()
}

// clientCode checks the redirect back to the client and returns the code.
func (te *testEnv) clientCode(resp *http.Response) string {
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		te.t.Fatalf("expected 302 redirect to client, got %d", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		te.t.Fatalf("invalid client redirect location: %v", err)
	}
	q := loc.Query()
	if q.Get("iss") != te.gw.cfg.BaseURL {
		te.t.Fatalf("expected iss=%q, got %q", te.gw.cfg.BaseURL, q.Get("iss"))
	}
	if q.Get("state") != "client-state-123" {
		te.t.Fatalf("expected state=client-state-123, got %q", q.Get("state"))
	}
	code := q.Get("code")
	if code == "" {
		te.t.Fatalf("expected non-empty gateway code, got error=%q desc=%q", q.Get("error"), q.Get("error_description"))
	}
	return code
}

// completeEntraCallback finishes the login and returns the gateway code,
// approving the consent page if one is shown.
func (te *testEnv) completeEntraCallback(entraState string, stateCookie *http.Cookie) string {
	resp := te.callback(entraState, stateCookie)
	if resp.StatusCode == http.StatusOK {
		id, csrf, cookie, _ := te.parseConsentPage(resp)
		resp = te.postConsent(id, csrf, cookie, "approve")
	}
	return te.clientCode(resp)
}

func (te *testEnv) obtainAuthCode(verifier string) string {
	return te.completeEntraCallback(te.startLogin(verifier))
}

func (te *testEnv) postToken(form url.Values) (int, map[string]any) {
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)
	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestMetadataEndpoints(t *testing.T) {
	te := newTestEnv(t)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantField  string
		wantValue  string
	}{
		{
			name:       "protected resource metadata root",
			path:       "/.well-known/oauth-protected-resource",
			wantStatus: http.StatusOK,
			wantField:  "resource",
			wantValue:  te.resource,
		},
		{
			name:       "protected resource metadata pilot route path",
			path:       "/.well-known/oauth-protected-resource/mcp/team-etterlevelse/etterlevelse",
			wantStatus: http.StatusOK,
			wantField:  "resource",
			wantValue:  te.resource,
		},
		{
			name:       "protected resource metadata unlisted mcp root returns 404",
			path:       "/.well-known/oauth-protected-resource/mcp",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "protected resource metadata unknown path",
			path:       "/.well-known/oauth-protected-resource/unknown",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "authorization server metadata",
			path:       "/.well-known/oauth-authorization-server",
			wantStatus: http.StatusOK,
			wantField:  "issuer",
			wantValue:  te.gw.cfg.BaseURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			te.handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
			if tt.wantField != "" {
				var body map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("unmarshal failed: %v", err)
				}
				if body[tt.wantField] != tt.wantValue {
					t.Fatalf("expected %s=%q, got %v", tt.wantField, tt.wantValue, body[tt.wantField])
				}
			}
		})
	}
}

func TestAuthorizeValidation(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	tests := []struct {
		name             string
		overrides        map[string]string
		allowSpoofClient bool
		wantStatus       int
		wantOAuthError   string
	}{
		{
			name:       "unknown client_id returns 400 without redirect",
			overrides:  map[string]string{"client_id": "https://untrusted.example/client.json"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing client_id returns 400 without redirect",
			overrides:  map[string]string{"client_id": ""},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "redirect mismatch returns 400 without redirect",
			overrides:  map[string]string{"redirect_uri": "http://127.0.0.1:9999/evil"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing redirect_uri returns 400 without redirect",
			overrides:  map[string]string{"redirect_uri": ""},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:             "CIMD document client_id mismatch returns 400",
			allowSpoofClient: true,
			wantStatus:       http.StatusBadRequest,
		},
		{
			name:           "missing resource redirects with invalid_target and iss",
			overrides:      map[string]string{"resource": ""},
			wantStatus:     http.StatusFound,
			wantOAuthError: "invalid_target",
		},
		{
			name:           "foreign resource redirects with invalid_target and iss",
			overrides:      map[string]string{"resource": "https://other.example/mcp"},
			wantStatus:     http.StatusFound,
			wantOAuthError: "invalid_target",
		},
		{
			name:           "non-S256 PKCE redirects with invalid_request",
			overrides:      map[string]string{"code_challenge_method": "plain"},
			wantStatus:     http.StatusFound,
			wantOAuthError: "invalid_request",
		},
		{
			name:           "missing code_challenge redirects with invalid_request",
			overrides:      map[string]string{"code_challenge": ""},
			wantStatus:     http.StatusFound,
			wantOAuthError: "invalid_request",
		},
		{
			name:           "unsupported response_type redirects with unsupported_response_type",
			overrides:      map[string]string{"response_type": "token"},
			wantStatus:     http.StatusFound,
			wantOAuthError: "unsupported_response_type",
		},
		{
			name:           "missing state redirects with invalid_request",
			overrides:      map[string]string{"state": ""},
			wantStatus:     http.StatusFound,
			wantOAuthError: "invalid_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := newTestEnv(t)
			overrides := tt.overrides
			if tt.allowSpoofClient {
				badID := te.cimdServer.URL + "/bad-client-id.json"
				te.gw.cfg.AllowedClientIDs[badID] = true
				overrides = map[string]string{"client_id": badID}
			}

			req := httptest.NewRequest(http.MethodGet, te.authorizeURL(overrides, verifier), nil)
			rec := httptest.NewRecorder()
			te.handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (%s)", tt.wantStatus, rec.Code, rec.Body.String())
			}
			if tt.wantStatus == http.StatusFound {
				loc, err := url.Parse(rec.Header().Get("Location"))
				if err != nil {
					t.Fatalf("invalid redirect URL: %v", err)
				}
				if gotErr := loc.Query().Get("error"); gotErr != tt.wantOAuthError {
					t.Fatalf("expected error=%q, got %q", tt.wantOAuthError, gotErr)
				}
				if gotIss := loc.Query().Get("iss"); gotIss != te.gw.cfg.BaseURL {
					t.Fatalf("expected RFC 9207 iss=%q, got %q", te.gw.cfg.BaseURL, gotIss)
				}
			}
		})
	}
}

func TestTokenValidation_PKCE_Resource_Reuse_Expiry(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	tests := []struct {
		name       string
		advance    time.Duration
		formMutate func(f url.Values)
		reuseCode  bool
		wantStatus int
		wantError  string
	}{
		{
			name:       "valid token exchange succeeds",
			wantStatus: http.StatusOK,
		},
		{
			name:       "PKCE wrong verifier rejected",
			formMutate: func(f url.Values) { f.Set("code_verifier", "wrong-verifier-that-is-definitely-at-least-43-chars") },
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_grant",
		},
		{
			name:       "PKCE missing verifier rejected",
			formMutate: func(f url.Values) { f.Del("code_verifier") },
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_grant",
		},
		{
			name:       "PKCE too short verifier rejected",
			formMutate: func(f url.Values) { f.Set("code_verifier", "short") },
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_grant",
		},
		{
			name:       "foreign resource at token endpoint rejected",
			formMutate: func(f url.Values) { f.Set("resource", "https://foreign.example/mcp") },
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_target",
		},
		{
			name:       "redirect mismatch at token endpoint rejected",
			formMutate: func(f url.Values) { f.Set("redirect_uri", "http://127.0.0.1:9999/other") },
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_grant",
		},
		{
			name:       "client_id mismatch at token endpoint rejected",
			formMutate: func(f url.Values) { f.Set("client_id", "https://other.example/client.json") },
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_grant",
		},
		{
			name:       "code expiry after 60s rejected",
			advance:    61 * time.Second,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_grant",
		},
		{
			name:       "code reuse rejected",
			reuseCode:  true,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_grant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := newTestEnv(t)
			code := te.obtainAuthCode(verifier)

			form := url.Values{
				"grant_type":    {"authorization_code"},
				"code":          {code},
				"client_id":     {te.clientID},
				"redirect_uri":  {te.redirectURI},
				"resource":      {te.resource},
				"code_verifier": {verifier},
			}
			if tt.formMutate != nil {
				tt.formMutate(form)
			}
			if tt.advance > 0 {
				te.advance(tt.advance)
			}

			status, body := te.postToken(form)
			if tt.reuseCode {
				if status != http.StatusOK {
					t.Fatalf("expected initial token exchange to succeed, got %d", status)
				}
				status, body = te.postToken(form)
			}

			if status != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (%v)", tt.wantStatus, status, body)
			}
			if tt.wantError != "" && body["error"] != tt.wantError {
				t.Fatalf("expected error=%q, got %v", tt.wantError, body["error"])
			}
			if tt.wantStatus == http.StatusOK {
				if at, _ := body["access_token"].(string); at == "" {
					t.Fatal("expected non-empty access_token")
				}
				if body["token_type"] != "Bearer" {
					t.Fatalf("expected Bearer token_type, got %v", body["token_type"])
				}
			}
		})
	}
}

func TestConsent_CSRF_SecurityHeaders_And_NoRememberedConsent(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	tests := []struct {
		name       string
		mutate     func(id, csrf *string, cookie **http.Cookie)
		decision   string
		wantStatus int
		wantErr    string
	}{
		{
			name:       "wrong form CSRF fails with 403",
			mutate:     func(_, csrf *string, _ **http.Cookie) { *csrf = "wrong-csrf" },
			decision:   "approve",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "missing CSRF cookie fails with 403",
			mutate:     func(_, _ *string, cookie **http.Cookie) { *cookie = nil },
			decision:   "approve",
			wantStatus: http.StatusForbidden,
		},
		{
			name: "mismatched CSRF cookie fails with 403",
			mutate: func(_, _ *string, cookie **http.Cookie) {
				*cookie = &http.Cookie{Name: csrfCookieName, Value: "different-cookie-value"}
			},
			decision:   "approve",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "unknown consent id fails with 403",
			mutate:     func(id, _ *string, _ **http.Cookie) { *id = "nonexistent-id" },
			decision:   "approve",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "deny decision redirects with access_denied and iss",
			decision:   "deny",
			wantStatus: http.StatusFound,
			wantErr:    "access_denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := newTestEnv(t)
			id, csrf, cookie, consentHeaders := te.startAuthorizeAndConsent(verifier)

			if got := consentHeaders.Get("X-Frame-Options"); got != "DENY" {
				t.Fatalf("expected X-Frame-Options DENY, got %q", got)
			}
			if got := consentHeaders.Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") {
				t.Fatalf("expected frame-ancestors 'none' in CSP, got %q", got)
			}
			if cookie.Name != "__Host-fleet_csrf" || cookie.Path != "/" || cookie.Domain != "" ||
				!cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
				t.Fatalf("unexpected CSRF cookie attributes: %+v", cookie)
			}

			if tt.mutate != nil {
				tt.mutate(&id, &csrf, &cookie)
			}

			form := url.Values{"id": {id}, "csrf": {csrf}, "decision": {tt.decision}}
			req := httptest.NewRequest(http.MethodPost, "/consent", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if cookie != nil {
				req.AddCookie(cookie)
			}
			rec := httptest.NewRecorder()
			te.handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
			if tt.wantErr != "" {
				loc, _ := url.Parse(rec.Header().Get("Location"))
				if loc.Query().Get("error") != tt.wantErr {
					t.Fatalf("expected error=%q, got %q", tt.wantErr, loc.Query().Get("error"))
				}
				if loc.Query().Get("iss") != te.gw.cfg.BaseURL {
					t.Fatalf("expected iss=%q, got %q", te.gw.cfg.BaseURL, loc.Query().Get("iss"))
				}
			}
		})
	}

	t.Run("consent is remembered for the same user, client and route", func(t *testing.T) {
		te := newTestEnv(t)
		_ = te.obtainAuthCode(verifier)

		resp := te.callback(te.startLogin(verifier)) //nolint:bodyclose // clientCode closes it
		_ = te.clientCode(resp)                      // 302 straight to the client, no consent page
	})
}

func TestCallback_StateCookieBinding(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	tests := []struct {
		name       string
		mutate     func(te *testEnv, state *string, cookie **http.Cookie)
		wantStatus int
	}{
		{
			name:       "missing state cookie fails with 403",
			mutate:     func(_ *testEnv, _ *string, cookie **http.Cookie) { *cookie = nil },
			wantStatus: http.StatusForbidden,
		},
		{
			name: "forged state cookie signature fails with 403",
			mutate: func(_ *testEnv, state *string, cookie **http.Cookie) {
				*cookie = &http.Cookie{Name: entraStateCookieName, Value: *state + ".forged-signature"}
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "cookie signed for different state fails with 403",
			mutate: func(te *testEnv, _ *string, cookie **http.Cookie) {
				*cookie = &http.Cookie{Name: entraStateCookieName, Value: te.gw.signEntraState("other-state")}
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := newTestEnv(t)
			entraState, stateCookie := te.startLogin(verifier)

			if stateCookie.Name != "__Host-fleet_entra_state" || stateCookie.Path != "/" || stateCookie.Domain != "" ||
				!stateCookie.HttpOnly || !stateCookie.Secure || stateCookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("unexpected state cookie attributes: %+v", stateCookie)
			}

			tt.mutate(te, &entraState, &stateCookie)

			req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=entra-code&state="+url.QueryEscape(entraState), nil)
			if stateCookie != nil {
				req.AddCookie(stateCookie)
			}
			rec := httptest.NewRecorder()
			te.handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestMapCapsAndRateLimit(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "pendingConsent map cap returns 503",
			run: func(t *testing.T) {
				te := newTestEnv(t)
				te.gw.store = newMemoryStore(te.gw.now, map[string]int{kindPendingConsent: 1, kindPendingEntra: 1024, kindCode: 1024, kindSession: 4096})

				_, _, _, _ = te.startAuthorizeAndConsent(verifier)

				resp := te.callback(te.startLogin(verifier))
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusServiceUnavailable {
					t.Fatalf("expected 503 at pending cap, got %d", resp.StatusCode)
				}
			},
		},
		{
			name: "pendingEntra map cap returns 503",
			run: func(t *testing.T) {
				te := newTestEnv(t)
				te.gw.store = newMemoryStore(te.gw.now, map[string]int{kindPendingConsent: 1024, kindPendingEntra: 1, kindCode: 1024, kindSession: 4096})

				_, _ = te.startLogin(verifier)

				req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
				rec := httptest.NewRecorder()
				te.handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusServiceUnavailable {
					t.Fatalf("expected 503 at pendingEntra cap, got %d", rec.Code)
				}
			},
		},
		{
			name: "codes map cap returns 503",
			run: func(t *testing.T) {
				te := newTestEnv(t)
				te.gw.store = newMemoryStore(te.gw.now, map[string]int{kindPendingConsent: 1024, kindPendingEntra: 1024, kindCode: 1, kindSession: 4096})

				_ = te.obtainAuthCode(verifier)

				st2, stCookie2 := te.startLogin(verifier)
				req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=entra-code&state="+url.QueryEscape(st2), nil)
				req.AddCookie(stCookie2)
				rec := httptest.NewRecorder()
				te.handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusServiceUnavailable {
					t.Fatalf("expected 503 at codes cap, got %d", rec.Code)
				}
			},
		},
		{
			name: "sessions map cap returns 503",
			run: func(t *testing.T) {
				te := newTestEnv(t)
				te.gw.store = newMemoryStore(te.gw.now, map[string]int{kindPendingConsent: 1024, kindPendingEntra: 1024, kindCode: 1024, kindSession: 1})

				c1 := te.obtainAuthCode(verifier)
				status1, _ := te.postToken(url.Values{
					"grant_type":    {"authorization_code"},
					"code":          {c1},
					"client_id":     {te.clientID},
					"redirect_uri":  {te.redirectURI},
					"resource":      {te.resource},
					"code_verifier": {verifier},
				})
				if status1 != http.StatusOK {
					t.Fatalf("expected first token to succeed, got %d", status1)
				}

				c2 := te.obtainAuthCode(verifier)
				status2, body2 := te.postToken(url.Values{
					"grant_type":    {"authorization_code"},
					"code":          {c2},
					"client_id":     {te.clientID},
					"redirect_uri":  {te.redirectURI},
					"resource":      {te.resource},
					"code_verifier": {verifier},
				})
				if status2 != http.StatusServiceUnavailable {
					t.Fatalf("expected 503 at session cap, got %d (%v)", status2, body2)
				}
			},
		},
		{
			name: "per-IP rate limit enforced on authorize and token",
			run: func(t *testing.T) {
				te := newTestEnv(t)
				for _, ep := range rateLimitedEndpoints {
					te.gw.limiters[ep] = newIPRateLimiter(2, 100, time.Minute, 10, func() time.Time { return te.now })
				}
				te.handler = te.gw.routes()

				for i := 0; i < 2; i++ {
					req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
					req.RemoteAddr = "198.51.100.10:1234"
					rec := httptest.NewRecorder()
					te.handler.ServeHTTP(rec, req)
					if rec.Code != http.StatusFound {
						t.Fatalf("request %d expected 302, got %d", i, rec.Code)
					}
				}
				req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
				req.RemoteAddr = "198.51.100.10:1234"
				rec := httptest.NewRecorder()
				te.handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusTooManyRequests {
					t.Fatalf("expected 429 on /authorize, got %d", rec.Code)
				}

				// Different IP is still allowed on /token up to limit, then blocked.
				for i := 0; i < 2; i++ {
					tReq := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader("grant_type=authorization_code"))
					tReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					tReq.RemoteAddr = "198.51.100.20:5678"
					tRec := httptest.NewRecorder()
					te.handler.ServeHTTP(tRec, tReq)
					if tRec.Code == http.StatusTooManyRequests {
						t.Fatalf("unexpected 429 on /token attempt %d", i)
					}
				}
				tReq := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader("grant_type=authorization_code"))
				tReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				tReq.RemoteAddr = "198.51.100.20:5678"
				tRec := httptest.NewRecorder()
				te.handler.ServeHTTP(tRec, tReq)
				if tRec.Code != http.StatusTooManyRequests {
					t.Fatalf("expected 429 on /token, got %d", tRec.Code)
				}
			},
		},
		{
			name: "X-Forwarded-For uses last trusted ingress IP and enforces global ceiling",
			run: func(t *testing.T) {
				te := newTestEnv(t)
				te.gw.limiters[rateLimitEndpointAuthorize] = newIPRateLimiter(2, 4, time.Minute, 10, func() time.Time { return te.now })
				te.handler = te.gw.routes()

				// Spoofing earlier XFF entries cannot bypass per-IP limit keyed on the last entry.
				for i, firstHop := range []string{"1.1.1.1", "2.2.2.2"} {
					req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
					req.RemoteAddr = "10.0.0.1:443"
					req.Header.Set("X-Forwarded-For", firstHop+", 203.0.113.10")
					rec := httptest.NewRecorder()
					te.handler.ServeHTTP(rec, req)
					if rec.Code != http.StatusFound {
						t.Fatalf("request %d expected 302, got %d", i, rec.Code)
					}
				}
				blockedReq := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
				blockedReq.RemoteAddr = "10.0.0.1:443"
				blockedReq.Header.Set("X-Forwarded-For", "3.3.3.3, 203.0.113.10")
				blockedRec := httptest.NewRecorder()
				te.handler.ServeHTTP(blockedRec, blockedReq)
				if blockedRec.Code != http.StatusTooManyRequests {
					t.Fatalf("expected 429 when spoofing earlier XFF hop, got %d", blockedRec.Code)
				}

				// Distinct client IP behind same Nais ingress (10.0.0.1) is allowed until global ceiling (4).
				for i, ip := range []string{"203.0.113.11", "203.0.113.12"} {
					req := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
					req.RemoteAddr = "10.0.0.1:443"
					req.Header.Set("X-Forwarded-For", ip)
					rec := httptest.NewRecorder()
					te.handler.ServeHTTP(rec, req)
					if rec.Code != http.StatusFound {
						t.Fatalf("distinct IP %d expected 302 (blocked per-IP must not exhaust global), got %d", i, rec.Code)
					}
				}

				// Global ceiling reached: a brand-new IP is also rejected with 429.
				globalReq := httptest.NewRequest(http.MethodGet, te.authorizeURL(nil, verifier), nil)
				globalReq.RemoteAddr = "10.0.0.1:443"
				globalReq.Header.Set("X-Forwarded-For", "203.0.113.99")
				globalRec := httptest.NewRecorder()
				te.handler.ServeHTTP(globalRec, globalReq)
				if globalRec.Code != http.StatusTooManyRequests {
					t.Fatalf("expected 429 at global ceiling, got %d", globalRec.Code)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestLogsNeverContainSecrets(t *testing.T) {
	var logBuf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(oldLogger)

	te := newTestEnv(t)
	verifier := "secret-pkce-verifier-value-that-must-never-appear-in-logs"
	code := te.obtainAuthCode(verifier)

	status, body := te.postToken(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {te.clientID},
		"redirect_uri":  {te.redirectURI},
		"resource":      {te.resource},
		"code_verifier": {verifier},
	})
	if status != http.StatusOK {
		t.Fatalf("expected 200 from /token, got %d", status)
	}

	accessToken, _ := body["access_token"].(string)
	logs := logBuf.String()
	for _, secret := range []string{
		verifier,
		code,
		accessToken,
		"entra-code-123",
		strings.Join([]string{"entra", "test", "access"}, "-"),
		strings.Join([]string{"entra", "test", "refresh"}, "-"),
		"azure-client-secret",
	} {
		if strings.Contains(logs, secret) {
			t.Fatalf("log output leaked secret %q: %s", secret, logs)
		}
	}
}

func TestNewGateway_FailsOnInvalidTeamTokenStrategy(t *testing.T) {
	te := newTestEnv(t)
	badCfg := *te.gw.cfg
	badCfg.TeamTokenStrategy = "unsupported-strategy"
	if _, err := newGateway(&badCfg, nil); err == nil {
		t.Fatal("expected newGateway to fail when newTeamTokenSource returns an error")
	}
}

func TestCallback_RequiresNAVident(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	t.Run("missing NAVident in id_token and access_token is rejected", func(t *testing.T) {
		te := newTestEnv(t)
		te.mu.Lock()
		te.navIdent = ""
		te.mu.Unlock()

		entraState, stateCookie := te.startLogin(verifier)
		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=entra-code-123&state="+url.QueryEscape(entraState), nil)
		req.AddCookie(stateCookie)
		rec := httptest.NewRecorder()
		te.handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Fatalf("expected 302 redirect, got %d", rec.Code)
		}
		loc, err := url.Parse(rec.Header().Get("Location"))
		if err != nil {
			t.Fatalf("invalid redirect location: %v", err)
		}
		if loc.Query().Get("error") != "access_denied" {
			t.Fatalf("expected access_denied when NAVident missing, got %q (%s)", loc.Query().Get("error"), loc.Query().Get("error_description"))
		}
	})

	t.Run("NAVident in access_token claims satisfies requirement", func(t *testing.T) {
		te := newTestEnv(t)
		te.mu.Lock()
		te.navIdent = ""
		te.entraToken = makeTestJWT(map[string]any{"NAVident": "A987654"})
		te.mu.Unlock()

		code := te.obtainAuthCode(verifier)
		if code == "" {
			t.Fatal("expected auth code when NAVident is present in access_token")
		}
	})
}

func TestCallback_RequiresAccessTokenForOBO(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	t.Run("obo rejects empty access_token even when id_token is present", func(t *testing.T) {
		te := newTestEnv(t)
		te.mu.Lock()
		te.entraToken = ""
		te.mu.Unlock()

		entraState, stateCookie := te.startLogin(verifier)
		resp := te.callback(entraState, stateCookie)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			t.Fatalf("expected 302 redirect, got %d", resp.StatusCode)
		}
		loc, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			t.Fatalf("invalid redirect location: %v", err)
		}
		if loc.Query().Get("error") != "server_error" {
			t.Fatalf("expected server_error when access_token empty in obo mode, got %q (%s)", loc.Query().Get("error"), loc.Query().Get("error_description"))
		}
	})

	t.Run("refresh strategy accepts empty access_token when id_token is present", func(t *testing.T) {
		te := newTestEnv(t)
		te.gw.cfg.TeamTokenStrategy = strategyRefresh
		te.mu.Lock()
		te.entraToken = ""
		te.mu.Unlock()

		entraState, stateCookie := te.startLogin(verifier)
		resp := te.callback(entraState, stateCookie)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected consent page (200) in refresh mode, got %d", resp.StatusCode)
		}
	})
}

func TestHostCookies_ClearedWithPathRoot(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	te := newTestEnv(t)

	entraState, stateCookie := te.startLogin(verifier)
	if stateCookie.Path != "/" || stateCookie.Domain != "" || !stateCookie.Secure || !stateCookie.HttpOnly {
		t.Fatalf("unexpected state cookie: %+v", stateCookie)
	}

	cbResp := te.callback(entraState, stateCookie) //nolint:bodyclose // parseConsentPage closes it
	var clearedState *http.Cookie
	for _, c := range cbResp.Cookies() {
		if c.Name == "__Host-fleet_entra_state" {
			clearedState = c
		}
	}
	if clearedState == nil || clearedState.Path != "/" || clearedState.MaxAge != -1 || !clearedState.Secure || !clearedState.HttpOnly {
		t.Fatalf("unexpected cleared state cookie: %+v", clearedState)
	}

	id, csrf, csrfCookie, _ := te.parseConsentPage(cbResp)
	consentResp := te.postConsent(id, csrf, csrfCookie, "approve")
	_ = consentResp.Body.Close()
	var clearedCSRF *http.Cookie
	for _, c := range consentResp.Cookies() {
		if c.Name == "__Host-fleet_csrf" {
			clearedCSRF = c
		}
	}
	if clearedCSRF == nil || clearedCSRF.Path != "/" || clearedCSRF.MaxAge != -1 || !clearedCSRF.Secure || !clearedCSRF.HttpOnly {
		t.Fatalf("unexpected cleared CSRF cookie: %+v", clearedCSRF)
	}
}

func TestLogInjection_SanitizedAndEntraErrorTruncated(t *testing.T) {
	if got := sanitizeLogValue("a\nb\rc\r\nd"); got != "abcd" {
		t.Fatalf("sanitizeLogValue=%q, want %q", got, "abcd")
	}
	long := strings.Repeat("x", 80) + "\n\r"
	if got := truncateLogValue(long, maxLogErrorLen); len(got) != maxLogErrorLen || strings.ContainsAny(got, "\r\n") {
		t.Fatalf("truncateLogValue len=%d val=%q", len(got), got)
	}

	var logBuf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	defer slog.SetDefault(oldLogger)

	te := newTestEnv(t)
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"

	// 1. loggingMiddleware strips CR/LF from method and path.
	pathReq := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	pathReq.Method = "GET\nINJECT"
	pathReq.URL.Path = "/evil\nmsg=fake\r"
	te.handler.ServeHTTP(httptest.NewRecorder(), pathReq)

	// 2. callbackHandler sanitizes and truncates Entra error param to 64 chars.
	entraState, stateCookie := te.startLogin(verifier)
	injectedErr := "access\n_denied\r_" + strings.Repeat("a", 80)
	cbReq := httptest.NewRequest(http.MethodGet, "/oauth2/callback?error="+url.QueryEscape(injectedErr)+"&state="+url.QueryEscape(entraState), nil)
	cbReq.AddCookie(stateCookie)
	cbRec := httptest.NewRecorder()
	te.handler.ServeHTTP(cbRec, cbReq)
	if cbRec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect on entra error, got %d", cbRec.Code)
	}

	// 3. authorizeHandler sanitizes client_id on CIMD fetch failure.
	badClientID := te.cimdServer.URL + "/missing\nfake\r"
	te.gw.cfg.AllowedClientIDs[badClientID] = true
	authReq := httptest.NewRequest(http.MethodGet, te.authorizeURL(map[string]string{"client_id": badClientID}, verifier), nil)
	te.handler.ServeHTTP(httptest.NewRecorder(), authReq)

	var sawRequestCompleted, sawEntraError, sawCIMDFailed bool
	for _, line := range strings.Split(strings.TrimSpace(logBuf.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("expected valid single-line JSON log, got %q: %v", line, err)
		}
		switch entry["msg"] {
		case "request completed":
			if entry["path"] == "/evilmsg=fake" && entry["method"] == "GETINJECT" {
				sawRequestCompleted = true
			}
		case "audit login":
			if entry["outcome"] == "entra_error" {
				sawEntraError = true
				loggedErr, _ := entry["error"].(string)
				if strings.ContainsAny(loggedErr, "\r\n") {
					t.Fatalf("entra_error log contained CR/LF: %q", loggedErr)
				}
				if len(loggedErr) != maxLogErrorLen {
					t.Fatalf("expected logged error truncated to %d chars, got %d (%q)", maxLogErrorLen, len(loggedErr), loggedErr)
				}
			}
		case "CIMD fetch or validation failed":
			sawCIMDFailed = true
			cid, _ := entry["client_id"].(string)
			if strings.ContainsAny(cid, "\r\n") {
				t.Fatalf("client_id log contained CR/LF: %q", cid)
			}
		}
	}
	if !sawRequestCompleted || !sawEntraError || !sawCIMDFailed {
		t.Fatalf("missing expected log entries (req=%v entraErr=%v cimd=%v):\n%s",
			sawRequestCompleted, sawEntraError, sawCIMDFailed, logBuf.String())
	}
}
