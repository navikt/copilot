package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// texasClient exchanges copilot-cli's own workload identity for an M2M
// access token via the Texas sidecar's client_credentials endpoint
// (NAIS_TOKEN_ENDPOINT), scoped to the copilot-api audience.
//
// See: https://doc.nais.io/auth/explanations/#texas
type texasClient struct {
	httpClient *http.Client
	endpoint   string
	audience   string

	mu          sync.Mutex
	cachedToken string
	// expiresAt is when the cached token should be considered stale and
	// re-fetched, with a safety margin subtracted from the real expiry.
	expiresAt time.Time
}

func newTexasClient(endpoint, audience string) *texasClient {
	return &texasClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		endpoint:   endpoint,
		audience:   audience,
	}
}

type texasTokenRequest struct {
	IdentityProvider string `json:"identity_provider"`
	Target           string `json:"target"`
}

type texasTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// token returns a valid M2M access token, refreshing it via Texas when the
// cached one is missing or close to expiry.
func (c *texasClient) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.cachedToken != "" && time.Now().Before(c.expiresAt) {
		token := c.cachedToken
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	if c.endpoint == "" {
		return "", fmt.Errorf("NAIS_TOKEN_ENDPOINT is not configured — cannot mint M2M token")
	}

	body, err := json.Marshal(texasTokenRequest{
		IdentityProvider: "azuread",
		Target:           c.audience,
	})
	if err != nil {
		return "", fmt.Errorf("encoding texas token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("building texas token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling texas sidecar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("texas token exchange returned status %d", resp.StatusCode)
	}

	var result texasTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding texas token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("texas token response missing access_token")
	}

	c.mu.Lock()
	c.cachedToken = result.AccessToken
	// Refresh 30s before actual expiry to avoid using a token that expires
	// mid-flight to copilot-api.
	margin := 30 * time.Second
	ttl := time.Duration(result.ExpiresIn) * time.Second
	if ttl <= margin {
		ttl = margin
	}
	c.expiresAt = time.Now().Add(ttl - margin)
	c.mu.Unlock()

	return result.AccessToken, nil
}
