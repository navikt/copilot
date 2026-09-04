package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func newTestOAuthServer() *OAuthServer {
	store := NewTokenStore()
	githubClient := NewGitHubClient("test-client-id", "test-client-secret")
	return NewOAuthServer("http://localhost:8080", githubClient, store, "navikt")
}

func TestHandleRegister_Success(t *testing.T) {
	server := newTestOAuthServer()

	body := map[string]interface{}{
		"client_name":   "VS Code",
		"redirect_uris": []string{"http://127.0.0.1:33418"},
		"grant_types":   []string{"authorization_code"},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp ClientRegistration
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ClientID == "" {
		t.Fatal("expected non-empty client_id")
	}
	if resp.ClientName != "VS Code" {
		t.Fatalf("expected client_name 'VS Code', got %q", resp.ClientName)
	}
	if len(resp.RedirectURIs) != 1 || resp.RedirectURIs[0] != "http://127.0.0.1:33418" {
		t.Fatalf("unexpected redirect_uris: %v", resp.RedirectURIs)
	}
	if resp.TokenEndpointAuthMethod != "none" {
		t.Fatalf("expected token_endpoint_auth_method 'none', got %q", resp.TokenEndpointAuthMethod)
	}

	// The registration is not stored anywhere; it travels inside the client_id.
	info, err := verifyClientID(server.clientIDKey, resp.ClientID)
	if err != nil {
		t.Fatalf("issued client_id does not verify: %v", err)
	}
	if len(info.RedirectURIs) != 1 || info.RedirectURIs[0] != "http://127.0.0.1:33418" {
		t.Fatalf("client_id carries the wrong redirect_uris: %v", info.RedirectURIs)
	}
}

func TestHandleRegister_MissingRedirectURIs(t *testing.T) {
	server := newTestOAuthServer()

	body := map[string]interface{}{
		"client_name": "Test",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleRegister(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "invalid_client_metadata" {
		t.Fatalf("expected error 'invalid_client_metadata', got %q", resp["error"])
	}
}

func TestHandleRegister_InvalidRedirectURI(t *testing.T) {
	server := newTestOAuthServer()

	body := map[string]interface{}{
		"client_name":   "Test",
		"redirect_uris": []string{"http://evil.example.com/callback"},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleRegister(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "invalid_redirect_uri" {
		t.Fatalf("expected error 'invalid_redirect_uri', got %q", resp["error"])
	}
}

func TestHandleRegister_UnsupportedGrantType(t *testing.T) {
	server := newTestOAuthServer()

	body := map[string]interface{}{
		"client_name":   "Test",
		"redirect_uris": []string{"http://127.0.0.1:33418"},
		"grant_types":   []string{"client_credentials"},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleRegister(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRegister_UnsupportedAuthMethod(t *testing.T) {
	server := newTestOAuthServer()

	body := map[string]interface{}{
		"client_name":                "Test",
		"redirect_uris":              []string{"http://127.0.0.1:33418"},
		"token_endpoint_auth_method": "client_secret_basic",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleRegister(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleRegister_Defaults(t *testing.T) {
	server := newTestOAuthServer()

	body := map[string]interface{}{
		"redirect_uris": []string{"http://127.0.0.1:33418/callback"},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp ClientRegistration
	_ = json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.GrantTypes) != 1 || resp.GrantTypes[0] != "authorization_code" {
		t.Fatalf("expected default grant_types [authorization_code], got %v", resp.GrantTypes)
	}
	if len(resp.ResponseTypes) != 1 || resp.ResponseTypes[0] != "code" {
		t.Fatalf("expected default response_types [code], got %v", resp.ResponseTypes)
	}
	if resp.TokenEndpointAuthMethod != "none" {
		t.Fatalf("expected default token_endpoint_auth_method 'none', got %q", resp.TokenEndpointAuthMethod)
	}
}

func TestAuthServerMetadata_IncludesRegistrationEndpoint(t *testing.T) {
	server := newTestOAuthServer()

	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	server.handleAuthServerMetadata(w, req)

	var metadata AuthorizationServerMetadata
	if err := json.NewDecoder(w.Body).Decode(&metadata); err != nil {
		t.Fatalf("failed to decode metadata: %v", err)
	}

	expected := "http://localhost:8080/register"
	if metadata.RegistrationEndpoint != expected {
		t.Fatalf("expected registration_endpoint %q, got %q", expected, metadata.RegistrationEndpoint)
	}
}

func TestHandleAuthorize_MissingClientID(t *testing.T) {
	server := newTestOAuthServer()

	req := httptest.NewRequest("GET", "/oauth/authorize?redirect_uri=http://127.0.0.1:33418&state=abc", nil)
	w := httptest.NewRecorder()

	server.handleAuthorize(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleAuthorize_UnregisteredClientID_Rejected(t *testing.T) {
	server := newTestOAuthServer()

	req := httptest.NewRequest("GET", "/oauth/authorize?client_id=unknown&redirect_uri=http://127.0.0.1:33418&state=abc", nil)
	w := httptest.NewRecorder()

	server.handleAuthorize(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	// invalid_client is what makes an MCP client re-run Dynamic Client
	// Registration after the in-memory store has been emptied by a restart.
	if resp["error"] != "invalid_client" {
		t.Fatalf("expected error 'invalid_client', got %q", resp["error"])
	}
}

func TestHandleAuthorize_RedirectURIMismatch(t *testing.T) {
	server := newTestOAuthServer()

	clientID := mintClientID(server.clientIDKey, clientIDInfo{RedirectURIs: []string{"http://127.0.0.1:33418"}, IssuedAt: time.Now().Unix()})

	req := httptest.NewRequest("GET", "/oauth/authorize?client_id="+url.QueryEscape(clientID)+"&redirect_uri=http://evil.com/callback&state=abc", nil)
	w := httptest.NewRecorder()

	server.handleAuthorize(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleAuthorize_LoopbackDifferentPort(t *testing.T) {
	server := newTestOAuthServer()

	clientID := mintClientID(server.clientIDKey, clientIDInfo{RedirectURIs: []string{"http://127.0.0.1:33418/"}, IssuedAt: time.Now().Unix(), Nonce: generateSecureToken(8)})

	req := httptest.NewRequest("GET", "/oauth/authorize?client_id="+url.QueryEscape(clientID)+"&redirect_uri=http://127.0.0.1:50049/&state=abc", nil)
	w := httptest.NewRecorder()

	server.handleAuthorize(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect (loopback port should be ignored per RFC 8252), got %d: %s", w.Code, w.Body.String())
	}
}

func TestIsValidRedirectURI(t *testing.T) {
	tests := []struct {
		uri  string
		want bool
	}{
		{"http://127.0.0.1:33418", true},
		{"http://127.0.0.1:12345/callback", true},
		{"http://localhost:3000/callback", true},
		{"http://[::1]:33418/callback", true},
		// Loopback only, per RFC 8252 section 7.3: open registration makes any
		// https destination an attacker's for one POST (#633).
		{"https://vscode.dev/redirect", false},
		{"https://example.com/callback", false},
		{"http://evil.example.com/callback", false},
		{"http://0.0.0.0:8080", false},
		{"ftp://127.0.0.1:8080", false},
		{"not-a-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			got := isValidRedirectURI(tt.uri)
			if got != tt.want {
				t.Errorf("isValidRedirectURI(%q) = %v, want %v", tt.uri, got, tt.want)
			}
		})
	}
}

func TestIsRegisteredRedirectURI_LoopbackPortIgnored(t *testing.T) {
	tests := []struct {
		name       string
		registered []string
		uri        string
		want       bool
	}{
		{
			name:       "exact match",
			registered: []string{"http://127.0.0.1:33418/"},
			uri:        "http://127.0.0.1:33418/",
			want:       true,
		},
		{
			name:       "different loopback port",
			registered: []string{"http://127.0.0.1:33418/"},
			uri:        "http://127.0.0.1:50049/",
			want:       true,
		},
		{
			name:       "different loopback port no trailing slash",
			registered: []string{"http://127.0.0.1:33418"},
			uri:        "http://127.0.0.1:50049",
			want:       true,
		},
		{
			name:       "localhost different port",
			registered: []string{"http://localhost:33418/"},
			uri:        "http://localhost:50049/",
			want:       true,
		},
		{
			name:       "different path rejected",
			registered: []string{"http://127.0.0.1:33418/"},
			uri:        "http://127.0.0.1:50049/evil",
			want:       false,
		},
		{
			name:       "https not affected by port rule",
			registered: []string{"https://example.com:443/callback"},
			uri:        "https://example.com:8443/callback",
			want:       false,
		},
		{
			name:       "non-loopback http rejected",
			registered: []string{"http://127.0.0.1:33418/"},
			uri:        "http://evil.com:33418/",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRegisteredRedirectURI(tt.registered, tt.uri)
			if got != tt.want {
				t.Errorf("isRegisteredRedirectURI(%v, %q) = %v, want %v", tt.registered, tt.uri, got, tt.want)
			}
		})
	}
}

// --- CORS on discovery and registration (GHSA-7hwf-488h-59x8) ---

// TestDiscoveryAndRegister_NoWildcardCORS covers the endpoints that still sent
// Access-Control-Allow-Origin: * after /oauth/token stopped. Metadata discovery
// and DCR are done by a native process in every supported client, so no origin
// needs to be granted.
func TestDiscoveryAndRegister_NoWildcardCORS(t *testing.T) {
	mux, _ := newChainTestServer(t)

	body, _ := json.Marshal(map[string]any{
		"client_name":   "test client",
		"redirect_uris": []string{"http://127.0.0.1:33418/callback"},
	})

	cases := []struct {
		method, path string
		body         []byte
	}{
		{"GET", "/.well-known/oauth-authorization-server", nil},
		{"GET", "/.well-known/oauth-protected-resource", nil},
		{"GET", "/.well-known/oauth-protected-resource/mcp", nil},
		{"POST", "/register", body},
		{"OPTIONS", "/register", nil},
	}

	for _, tc := range cases {
		var r *http.Request
		if tc.body != nil {
			r = httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
		} else {
			r = httptest.NewRequest(tc.method, tc.path, nil)
		}
		r.Header.Set("Origin", "https://evil.example")
		w := do(mux, r)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("%s %s: expected no Access-Control-Allow-Origin, got %q", tc.method, tc.path, got)
		}
	}
}

// TestDiscoveryAndRegister_StillWork is the control: dropping CORS must not
// break the endpoints themselves.
func TestDiscoveryAndRegister_StillWork(t *testing.T) {
	mux, _ := newChainTestServer(t)

	w := do(mux, httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("metadata: expected 200, got %d", w.Code)
	}
	var meta AuthorizationServerMetadata
	if err := json.NewDecoder(w.Body).Decode(&meta); err != nil {
		t.Fatalf("metadata: decode: %v", err)
	}
	if meta.RegistrationEndpoint == "" {
		t.Fatalf("metadata: expected a registration_endpoint, got %+v", meta)
	}
	if id := registerClient(t, mux, "http://127.0.0.1:33418/callback"); id == "" {
		t.Fatal("register: expected a client_id")
	}
}
