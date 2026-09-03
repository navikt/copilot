package main

import (
	"testing"
	"time"
)

// newTestStore creates a store without starting the cleanup goroutine
func newTestStore() *TokenStore {
	return &TokenStore{
		authSessions:  make(map[string]*AuthSession),
		authCodes:     make(map[string]*AuthCode),
		tokens:        make(map[string]*TokenData),
		refreshTokens: make(map[string]*RefreshTokenData),
	}
}

func TestTokenStore_AuthSession(t *testing.T) {
	store := newTestStore()

	session := &AuthSession{
		ClientID:    "test-client",
		ClientState: "state123",
		RedirectURI: "http://localhost:33418",
		CreatedAt:   time.Now(),
	}

	store.SaveAuthSession("state123", session)

	got, err := store.GetAuthSession("state123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.ClientID != "test-client" {
		t.Errorf("expected ClientID 'test-client', got %q", got.ClientID)
	}

	store.DeleteAuthSession("state123")
	_, err = store.GetAuthSession("state123")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTokenStore_AuthSession_NotFound(t *testing.T) {
	store := newTestStore()

	_, err := store.GetAuthSession("nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTokenStore_AuthCode(t *testing.T) {
	store := newTestStore()

	code := &AuthCode{
		ClientID:          "test-client",
		GitHubAccessToken: "gh-token",
		UserLogin:         "testuser",
		UserID:            12345,
		CreatedAt:         time.Now(),
	}

	store.SaveAuthCode("code123", code)

	got, err := store.GetAuthCode("code123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.UserLogin != "testuser" {
		t.Errorf("expected UserLogin 'testuser', got %q", got.UserLogin)
	}

	store.DeleteAuthCode("code123")
	_, err = store.GetAuthCode("code123")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTokenStore_Token(t *testing.T) {
	store := newTestStore()

	token := &TokenData{
		GitHubAccessToken: "gh-access-token",
		UserLogin:         "testuser",
		UserID:            12345,
		ExpiresAt:         time.Now().Add(1 * time.Hour),
	}

	store.SaveToken("token123", token)

	got, err := store.GetToken("token123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.UserLogin != "testuser" {
		t.Errorf("expected UserLogin 'testuser', got %q", got.UserLogin)
	}

	store.DeleteToken("token123")
	_, err = store.GetToken("token123")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTokenStore_Token_Expired(t *testing.T) {
	store := newTestStore()

	token := &TokenData{
		GitHubAccessToken: "gh-access-token",
		UserLogin:         "testuser",
		UserID:            12345,
		ExpiresAt:         time.Now().Add(-1 * time.Hour), // expired
	}

	store.SaveToken("expired-token", token)

	_, err := store.GetToken("expired-token")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound for expired token, got %v", err)
	}
}

func TestTokenStore_RefreshToken(t *testing.T) {
	store := newTestStore()

	refresh := &RefreshTokenData{
		GitHubRefreshToken: "gh-refresh-token",
		UserLogin:          "testuser",
		UserID:             12345,
		CreatedAt:          time.Now(),
	}

	store.SaveRefreshToken("refresh123", refresh)

	got, err := store.GetRefreshToken("refresh123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.GitHubRefreshToken != "gh-refresh-token" {
		t.Errorf("expected GitHubRefreshToken 'gh-refresh-token', got %q", got.GitHubRefreshToken)
	}

	store.DeleteRefreshToken("refresh123")
	_, err = store.GetRefreshToken("refresh123")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
