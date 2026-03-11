package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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

	reg, err := server.Store.GetClientRegistration(resp.ClientID)
	if err != nil {
		t.Fatalf("client not stored: %v", err)
	}
	if reg.ClientName != "VS Code" {
		t.Fatalf("stored client_name mismatch: %q", reg.ClientName)
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
		"redirect_uris": []string{"https://vscode.dev/redirect"},
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

func TestHandleRegister_HTTPSRedirectURI(t *testing.T) {
	server := newTestOAuthServer()

	body := map[string]interface{}{
		"redirect_uris": []string{"https://vscode.dev/redirect", "http://127.0.0.1:33418"},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
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

func TestHandleAuthorize_UnregisteredClientID_Allowed(t *testing.T) {
	server := newTestOAuthServer()

	req := httptest.NewRequest("GET", "/oauth/authorize?client_id=unknown&redirect_uri=http://127.0.0.1:33418&state=abc", nil)
	w := httptest.NewRecorder()

	server.handleAuthorize(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAuthorize_RedirectURIMismatch(t *testing.T) {
	server := newTestOAuthServer()

	server.Store.SaveClientRegistration(&ClientRegistration{
		ClientID:     "test-client",
		RedirectURIs: []string{"http://127.0.0.1:33418"},
	})

	req := httptest.NewRequest("GET", "/oauth/authorize?client_id=test-client&redirect_uri=http://evil.com/callback&state=abc", nil)
	w := httptest.NewRecorder()

	server.handleAuthorize(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleAuthorize_LoopbackDifferentPort(t *testing.T) {
	server := newTestOAuthServer()

	server.Store.SaveClientRegistration(&ClientRegistration{
		ClientID:     "cli-client",
		RedirectURIs: []string{"http://127.0.0.1:33418/"},
	})

	req := httptest.NewRequest("GET", "/oauth/authorize?client_id=cli-client&redirect_uri=http://127.0.0.1:50049/&state=abc", nil)
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
		{"https://vscode.dev/redirect", true},
		{"https://example.com/callback", true},
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
