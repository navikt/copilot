package cli

import (
	"encoding/json"
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
// (wrapped) when no token has been stored, which callers should treat as
// "not logged in" rather than an unexpected error.
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
	if err := keyring.Delete(keychainService, keychainAccount); err != nil && err != keyring.ErrNotFound {
		return fmt.Errorf("deleting token from keychain: %w", err)
	}
	return nil
}
