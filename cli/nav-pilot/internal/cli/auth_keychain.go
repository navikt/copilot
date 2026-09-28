package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

// keychainService is the Keychain service name (macOS) / secret collection
// name (Linux libsecret / Windows Credential Manager) under which nav-pilot
// stores the developer's GitHub credentials for copilot-cli access.
const keychainService = "nav-pilot"

// keychainAccount is the Keychain account name for the stored token blob.
const keychainAccount = "github-token"

// storedToken is the JSON blob persisted in the OS credential store. It
// intentionally never touches disk in plaintext — go-keyring delegates to
// the platform-native secret store (macOS Keychain, Windows Credential
// Manager, or libsecret/DBus on Linux).
type storedToken struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	Scope       string    `json:"scope"`
	Login       string    `json:"login"`
	ObtainedAt  time.Time `json:"obtained_at"`
	// ExpiresAt is zero when the token does not expire (GitHub's classic
	// device-flow tokens for GitHub Apps without expiration enabled).
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	// RefreshToken renews an expiring token without a new login (#1118).
	// Empty when the token does not expire.
	RefreshToken     string    `json:"refresh_token,omitempty"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at,omitempty"`
}

// setFrom takes the token fields from GitHub's answer, received at now.
func (t *storedToken) setFrom(r *accessTokenResponse, now time.Time) {
	t.AccessToken, t.TokenType, t.Scope, t.ObtainedAt = r.AccessToken, r.TokenType, r.Scope, now
	t.ExpiresAt, t.RefreshToken, t.RefreshExpiresAt = time.Time{}, r.RefreshToken, time.Time{}
	if r.ExpiresIn > 0 {
		t.ExpiresAt = now.Add(time.Duration(r.ExpiresIn) * time.Second)
	}
	if r.RefreshTokenExpiresIn > 0 {
		t.RefreshExpiresAt = now.Add(time.Duration(r.RefreshTokenExpiresIn) * time.Second)
	}
}

// tokenRefreshMargin is how close to expiry a stored token gets renewed.
const tokenRefreshMargin = 5 * time.Minute

// currentToken is loadToken, with an access token that expires within
// tokenRefreshMargin renewed through its refresh token and saved. When the
// refresh fails the stored token comes back as it was, and callers still
// check expired(). The refresh uses the client ID in effect now, so a token
// from a NAV_PILOT_GITHUB_CLIENT_ID override renews only with the same
// override. If saving fails after a refresh, GitHub has already retired the
// old pair: this run uses the new token, and the next one asks for a login.
//
// Two processes refreshing at once spend the same refresh token, and GitHub
// rotates it, so the slower one is refused. It then reads the keychain again
// and takes the pair the faster one saved.
func currentToken(ctx context.Context) (storedToken, error) {
	t, err := loadToken()
	if err != nil || t.RefreshToken == "" || t.ExpiresAt.IsZero() || time.Until(t.ExpiresAt) > tokenRefreshMargin {
		return t, err
	}
	if !t.RefreshExpiresAt.IsZero() && time.Now().After(t.RefreshExpiresAt) {
		return t, nil
	}
	r, err := refreshAccessToken(ctx, navPilotGitHubClientID(), t.RefreshToken)
	if err != nil {
		debugLog("token refresh failed: %v", err)
		if again, lerr := loadToken(); lerr == nil && again.RefreshToken != t.RefreshToken {
			return again, nil
		}
		return t, nil
	}
	t.setFrom(r, time.Now())
	if err := saveToken(t); err != nil {
		debugLog("saving the refreshed token: %v", err)
	}
	return t, nil
}

// expired reports whether the token is known to have expired. Returns false
// when ExpiresAt is zero (unknown / non-expiring token) — callers should
// still treat a "not expired" result as provisional and let the server-side
// validation (GET /user) be the final word.
func (t storedToken) expired() bool {
	return !t.ExpiresAt.IsZero() && time.Now().After(t.ExpiresAt)
}

// saveToken persists the token to the OS keychain.
func saveToken(t storedToken) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("encoding token: %w", err)
	}
	if err := keyring.Set(keychainService, keychainAccount, string(data)); err != nil {
		return fmt.Errorf("storing token in keychain: %w", err)
	}
	return nil
}

// loadToken reads the token from the OS keychain. Returns keyring.ErrNotFound
// as-is when no token has been stored, which callers should detect with
// errors.Is and treat as "not logged in" rather than an unexpected error.
func loadToken() (storedToken, error) {
	data, err := keyring.Get(keychainService, keychainAccount)
	if err != nil {
		return storedToken{}, err
	}
	var t storedToken
	if err := json.Unmarshal([]byte(data), &t); err != nil {
		return storedToken{}, fmt.Errorf("decoding stored token: %w", err)
	}
	return t, nil
}

// deleteToken removes the token from the OS keychain. Not finding an existing
// entry is not an error — logout is idempotent.
func deleteToken() error {
	if err := keyring.Delete(keychainService, keychainAccount); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("deleting token from keychain: %w", err)
	}
	return nil
}
