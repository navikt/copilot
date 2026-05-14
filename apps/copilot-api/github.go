package main

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GitHubClient wraps GitHub API operations
type GitHubClient struct {
	httpClient     *http.Client
	org            string
	appID          string
	privateKey     *rsa.PrivateKey
	installationID string
	token          string
	tokenExpiry    time.Time
}

func newGitHubClient(config *Config) (*GitHubClient, error) {
	if config.GitHubAppID == "" || config.GitHubAppPrivateKey == "" || config.GitHubInstallationID == "" {
		return nil, errors.New("GitHub App configuration incomplete")
	}

	// Parse private key
	block, _ := pem.Decode([]byte(config.GitHubAppPrivateKey))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		parsedKey, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse private key: %w (PKCS1: %v)", err2, err)
		}
		var ok bool
		privateKey, ok = parsedKey.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
	}

	return &GitHubClient{
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		org:            config.GitHubOrg,
		appID:          config.GitHubAppID,
		privateKey:     privateKey,
		installationID: config.GitHubInstallationID,
	}, nil
}

// generateJWT creates a GitHub App JWT
func (g *GitHubClient) generateJWT() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		Issuer:    g.appID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(g.privateKey)
}

// getInstallationToken gets an installation access token
func (g *GitHubClient) getInstallationToken(ctx context.Context) (string, error) {
	// Return cached token if still valid
	if g.token != "" && time.Now().Before(g.tokenExpiry) {
		return g.token, nil
	}

	jwtToken, err := g.generateJWT()
	if err != nil {
		return "", fmt.Errorf("generate JWT: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%s/access_tokens", g.installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	// Cache token with 5min buffer before expiry
	g.token = result.Token
	g.tokenExpiry = result.ExpiresAt.Add(-5 * time.Minute)

	return result.Token, nil
}

// CopilotBilling represents GitHub Copilot billing data
type CopilotBilling struct {
	SeatBreakdown struct {
		Total               int `json:"total"`
		AddedThisCycle      int `json:"added_this_cycle"`
		PendingInvitation   int `json:"pending_invitation"`
		PendingCancellation int `json:"pending_cancellation"`
		ActiveThisCycle     int `json:"active_this_cycle"`
		InactiveThisCycle   int `json:"inactive_this_cycle"`
	} `json:"seat_breakdown"`
	SeatManagementSetting string `json:"seat_management_setting,omitempty"`
}

func (g *GitHubClient) getCopilotBilling(ctx context.Context) (*CopilotBilling, error) {
	token, err := g.getInstallationToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get installation token: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/orgs/%s/copilot/billing", g.org)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		slog.Error("GitHub API error", "status", resp.StatusCode, "body", buf.String())
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var billing CopilotBilling
	if err := json.NewDecoder(resp.Body).Decode(&billing); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &billing, nil
}

// collectGitHubMetrics fetches GitHub billing data and updates metrics
func collectGitHubMetrics(client *GitHubClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	billing, err := client.getCopilotBilling(ctx)
	if err != nil {
		slog.Error("Failed to collect GitHub metrics", "error", err)
		return
	}

	metricsCollector.mu.Lock()
	defer metricsCollector.mu.Unlock()

	metricsCollector.githubSeatsTotal = int64(billing.SeatBreakdown.Total)
	metricsCollector.githubSeatsActive = int64(billing.SeatBreakdown.ActiveThisCycle)
	metricsCollector.githubSeatsInactive = int64(billing.SeatBreakdown.InactiveThisCycle)
	metricsCollector.githubSeatsPending = int64(billing.SeatBreakdown.PendingInvitation)
	metricsCollector.githubSeatsCancelling = int64(billing.SeatBreakdown.PendingCancellation)
	metricsCollector.lastCollectionTimestamp = time.Now().Unix()

	slog.Info("GitHub metrics collected",
		"total", billing.SeatBreakdown.Total,
		"active", billing.SeatBreakdown.ActiveThisCycle,
		"inactive", billing.SeatBreakdown.InactiveThisCycle,
	)
}
