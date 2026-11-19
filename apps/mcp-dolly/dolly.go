package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Per-request worst-case memory: body 1 MiB + ReadAll growth + json.Compact copy => ~4 MiB.
// With maxConcurrentUpstream = 2 this stays far below GOMEMLIMIT 200 MiB.
const (
	maxUpstreamBodyBytes  = 1 << 20 // 1 MiB
	maxConcurrentUpstream = 2
	consumerID            = "mcp-dolly"
)

type upstreamStatusError struct {
	status int
}

func (e *upstreamStatusError) Error() string {
	switch e.status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Sprintf("dolly-backend denied access (status %d)", e.status)
	case http.StatusNotFound:
		return "not found in Dolly (status 404)"
	}
	return fmt.Sprintf("dolly-backend returned status %d", e.status)
}

type dollyClient struct {
	baseURL    string
	httpClient *http.Client
}

func newDollyClient(baseURL string, httpClient *http.Client) *dollyClient {
	return &dollyClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

// get is the only upstream call. It is GET-only by construction: this server never writes to Dolly.
func (c *dollyClient) get(ctx context.Context, token, path string, query url.Values) ([]byte, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("build dolly request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Nav-Call-Id", randomCallID())
	req.Header.Set("Nav-Consumer-Id", consumerID)

	resp, err := c.httpClient.Do(req) //nolint:gosec // baseURL validated at startup, path built from validated integers
	if err != nil {
		return nil, fmt.Errorf("dolly-backend request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &upstreamStatusError{status: resp.StatusCode}
	}
	body, err := readLimitedBody(resp.Body, maxUpstreamBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("dolly-backend response exceeded %d bytes; use a smaller pageSize", maxUpstreamBodyBytes)
	}
	var out bytes.Buffer
	if err := json.Compact(&out, body); err != nil {
		return nil, fmt.Errorf("invalid JSON from dolly-backend")
	}
	return out.Bytes(), nil
}

// currentBrukerID returns the Dolly user id for the token's owner.
func (c *dollyClient) currentBrukerID(ctx context.Context, token string) (string, error) {
	body, err := c.get(ctx, token, "/api/v1/bruker/current", nil)
	if err != nil {
		return "", err
	}
	var b struct {
		BrukerID string `json:"brukerId"`
	}
	if err := json.Unmarshal(body, &b); err != nil || b.BrukerID == "" {
		return "", fmt.Errorf("dolly-backend returned no brukerId")
	}
	return b.BrukerID, nil
}

func randomCallID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
