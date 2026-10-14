package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// --- ExchangeCode tests ---

func TestExchangeCode_Success(t *testing.T) {
	mock := &mockServer{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Error("expected Accept: application/json")
		}
		w.Header().Set("Content-Type", "application/json")
		//nolint:gosec // test data, not real credentials
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "gho_test123",
			"refresh_token": "ghr_refresh456",
			"expires_in":    28800,
			"token_type":    "bearer",
			"scope":         "read:org",
		})
	})}

	client := &GitHubClient{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		HTTPClient:   &http.Client{Transport: &mockTransport{handler: mock.handler}},
		APIBaseURL:   "http://test",
	}

	token, err := client.ExchangeCode("test-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "gho_test123" {
		t.Errorf("expected access_token 'gho_test123', got %q", token.AccessToken)
	}
	if token.RefreshToken != "ghr_refresh456" {
		t.Errorf("expected refresh_token 'ghr_refresh456', got %q", token.RefreshToken)
	}
	if token.TokenType != "bearer" {
		t.Errorf("expected token_type 'bearer', got %q", token.TokenType)
	}
	if token.Scope != "read:org" {
		t.Errorf("expected scope 'read:org', got %q", token.Scope)
	}
}

func TestExchangeCode_ErrorResponse(t *testing.T) {
	mock := &mockServer{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "bad_verification_code",
			"error_description": "The code has expired",
		})
	})}

	client := &GitHubClient{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		HTTPClient:   &http.Client{Transport: &mockTransport{handler: mock.handler}},
	}

	_, err := client.ExchangeCode("expired-code")
	if err == nil {
		t.Fatal("expected error for bad verification code")
	}
}

func TestExchangeCode_InvalidJSON(t *testing.T) {
	mock := &mockServer{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	})}

	client := &GitHubClient{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		HTTPClient:   &http.Client{Transport: &mockTransport{handler: mock.handler}},
	}

	_, err := client.ExchangeCode("some-code")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

// --- RefreshToken tests ---

func TestRefreshToken_Success(t *testing.T) {
	mock := &mockServer{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		//nolint:gosec // test data, not real credentials
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "gho_new_token",
			"refresh_token": "ghr_new_refresh",
			"expires_in":    28800,
			"token_type":    "bearer",
			"scope":         "read:org",
		})
	})}

	client := &GitHubClient{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		HTTPClient:   &http.Client{Transport: &mockTransport{handler: mock.handler}},
	}

	token, err := client.RefreshToken("ghr_old_refresh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "gho_new_token" {
		t.Errorf("expected 'gho_new_token', got %q", token.AccessToken)
	}
	if token.RefreshToken != "ghr_new_refresh" {
		t.Errorf("expected 'ghr_new_refresh', got %q", token.RefreshToken)
	}
}

func TestRefreshToken_Error(t *testing.T) {
	mock := &mockServer{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid_grant",
		})
	})}

	client := &GitHubClient{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		HTTPClient:   &http.Client{Transport: &mockTransport{handler: mock.handler}},
	}

	_, err := client.RefreshToken("bad-refresh-token")
	if err == nil {
		t.Fatal("expected error for invalid_grant")
	}
}

// --- GetUser tests ---

func TestGetUser_Success(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /user": func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("expected Authorization 'Bearer test-token', got %q", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(GitHubUser{
				ID:        42,
				Login:     "octocat",
				Email:     "octocat@github.com",
				Name:      "The Octocat",
				AvatarURL: "https://avatars.githubusercontent.com/u/42",
			})
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	user, err := client.GetUser("test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 42 {
		t.Errorf("expected ID 42, got %d", user.ID)
	}
	if user.Login != "octocat" {
		t.Errorf("expected Login 'octocat', got %q", user.Login)
	}
	if user.Email != "octocat@github.com" {
		t.Errorf("expected Email 'octocat@github.com', got %q", user.Email)
	}
	if user.Name != "The Octocat" {
		t.Errorf("expected Name 'The Octocat', got %q", user.Name)
	}
}

func TestGetUser_Unauthorized(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /user": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	_, err := client.GetUser("bad-token")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

// --- GetUserOrganizations tests ---

func TestGetUserOrganizations_Success(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]GitHubOrg{
				{ID: 1, Login: "navikt"},
				{ID: 2, Login: "github"},
			})
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	orgs, err := client.GetUserOrganizations("test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orgs) != 2 {
		t.Fatalf("expected 2 orgs, got %d", len(orgs))
	}
	if orgs[0].Login != "navikt" {
		t.Errorf("expected first org 'navikt', got %q", orgs[0].Login)
	}
	if orgs[1].Login != "github" {
		t.Errorf("expected second org 'github', got %q", orgs[1].Login)
	}
}

func TestGetUserOrganizations_Forbidden(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	_, err := client.GetUserOrganizations("test-token")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

// --- CheckOrgMembership tests ---

func TestCheckOrgMembership_IsMember(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]GitHubOrg{
				{ID: 1, Login: "navikt"},
				{ID: 2, Login: "other-org"},
			})
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	isMember, org := client.CheckOrgMembership("test-token", []string{"navikt"})
	if !isMember {
		t.Error("expected user to be a member of navikt")
	}
	if org != "navikt" {
		t.Errorf("expected matched org 'navikt', got %q", org)
	}
}

func TestCheckOrgMembership_CaseInsensitive(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]GitHubOrg{
				{ID: 1, Login: "NavIKT"},
			})
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	isMember, org := client.CheckOrgMembership("test-token", []string{"navikt"})
	if !isMember {
		t.Error("expected case-insensitive org match")
	}
	if org != "NavIKT" {
		t.Errorf("expected matched org login 'NavIKT', got %q", org)
	}
}

func TestCheckOrgMembership_NotMember(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]GitHubOrg{
				{ID: 1, Login: "other-org"},
			})
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	isMember, org := client.CheckOrgMembership("test-token", []string{"navikt"})
	if isMember {
		t.Error("expected user NOT to be a member of navikt")
	}
	if org != "" {
		t.Errorf("expected empty org string, got %q", org)
	}
}

func TestCheckOrgMembership_MultipleAllowedOrgs(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]GitHubOrg{
				{ID: 1, Login: "some-other-org"},
				{ID: 2, Login: "navikt"},
			})
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	isMember, org := client.CheckOrgMembership("test-token", []string{"github", "navikt"})
	if !isMember {
		t.Error("expected user to be a member of one of the allowed orgs")
	}
	if org != "navikt" {
		t.Errorf("expected matched org 'navikt', got %q", org)
	}
}

func TestCheckOrgMembership_APIError(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /user/orgs": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	isMember, org := client.CheckOrgMembership("test-token", []string{"navikt"})
	if isMember {
		t.Error("expected false when API returns error")
	}
	if org != "" {
		t.Errorf("expected empty org on error, got %q", org)
	}
}

// --- ListDirectory tests ---

func TestListDirectory_Success(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/app/contents/.github": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]RepoContent{
				{Name: "copilot-instructions.md", Type: "file", Path: ".github/copilot-instructions.md"},
				{Name: "workflows", Type: "dir", Path: ".github/workflows"},
			})
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	contents, err := client.ListDirectory("test-token", "navikt", "app", ".github")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(contents))
	}
	if contents[0].Name != "copilot-instructions.md" {
		t.Errorf("expected first entry 'copilot-instructions.md', got %q", contents[0].Name)
	}
}

func TestListDirectory_NotFound(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/app/contents/.github/missing": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	contents, err := client.ListDirectory("test-token", "navikt", "app", ".github/missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contents != nil {
		t.Errorf("expected nil for 404, got %v", contents)
	}
}

func TestListDirectory_ServerError(t *testing.T) {
	server := newGitHubMock(t, map[string]http.HandlerFunc{
		"GET /repos/navikt/app/contents/dir": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"Internal Server Error"}`))
		},
	})
	defer server.Close()

	client := newTestGitHubClient(server)
	_, err := client.ListDirectory("test-token", "navikt", "app", "dir")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// --- NewGitHubClient tests ---

func TestNewGitHubClient(t *testing.T) {
	client := NewGitHubClient("my-id", "my-secret")
	if client.ClientID != "my-id" {
		t.Errorf("expected ClientID 'my-id', got %q", client.ClientID)
	}
	if client.ClientSecret != "my-secret" {
		t.Errorf("expected ClientSecret 'my-secret', got %q", client.ClientSecret)
	}
	if client.APIBaseURL != "https://api.github.com" {
		t.Errorf("expected APIBaseURL 'https://api.github.com', got %q", client.APIBaseURL)
	}
	if client.HTTPClient == nil {
		t.Error("expected non-nil HTTPClient")
	}
}
