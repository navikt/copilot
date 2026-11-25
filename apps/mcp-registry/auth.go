package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	// ErrUnauthorized indicates missing, malformed, or rejected authentication credentials.
	ErrUnauthorized = errors.New("unauthorized: valid bearer token required")
)

// IntrospectionResult models Texas token introspection response.
type IntrospectionResult struct {
	Active   bool     `json:"active"`
	Azp      string   `json:"azp,omitempty"`
	IDType   string   `json:"idtyp,omitempty"`
	NAVident string   `json:"NAVident,omitempty"`
	Groups   []string `json:"groups,omitempty"`
}

// Principal is an authenticated human caller.
type Principal struct {
	// Actor is the caller's NAVident, never the email address.
	Actor  string
	Azp    string
	Groups []string
}

// IntrospectFunc defines a function for introspecting an authentication token.
type IntrospectFunc func(ctx context.Context, token string) (*IntrospectionResult, error)

// NewTexasIntrospector returns an IntrospectFunc that calls NAIS Texas introspection endpoint.
func NewTexasIntrospector(endpoint string) IntrospectFunc {
	client := &http.Client{Timeout: 5 * time.Second}
	return func(ctx context.Context, token string) (*IntrospectionResult, error) {
		if endpoint == "" {
			return nil, errors.New("introspection endpoint is not configured")
		}

		payload, err := json.Marshal(map[string]string{
			"identity_provider": "entra_id",
			"token":             token,
		})
		if err != nil {
			return nil, fmt.Errorf("marshaling introspection request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("creating introspection request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		// Introspection endpoint is configured via NAIS Texas sidecar env variable
		resp, err := client.Do(req) //nolint:gosec // Texas endpoint is trusted cluster-local sidecar
		if err != nil {
			return nil, fmt.Errorf("calling texas introspection: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("texas introspection returned status %d", resp.StatusCode)
		}

		var res IntrospectionResult
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&res); err != nil {
			return nil, fmt.Errorf("decoding introspection response: %w", err)
		}

		return &res, nil
	}
}

// Authenticate extracts the Bearer token from the request, verifies it with introspect,
// validates required claims (idtyp != 'app', non-empty NAVident), and returns the
// uppercased NAVident as actor with the token's azp and Entra groups.
func Authenticate(r *http.Request, introspect IntrospectFunc) (*Principal, error) {
	if introspect == nil {
		return nil, errors.New("introspector not configured")
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, ErrUnauthorized
	}

	// Cap Authorization header length at 8 KiB
	if len(authHeader) > 8*1024 {
		return nil, ErrUnauthorized
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, ErrUnauthorized
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return nil, ErrUnauthorized
	}

	res, err := introspect(r.Context(), token)
	if err != nil {
		return nil, ErrUnauthorized
	}

	if !res.Active {
		return nil, ErrUnauthorized
	}

	// Reject app tokens (idtyp must not be 'app')
	if strings.EqualFold(res.IDType, "app") {
		return nil, ErrUnauthorized
	}

	// Require non-empty NAVident
	if strings.TrimSpace(res.NAVident) == "" {
		return nil, ErrUnauthorized
	}

	return &Principal{Actor: strings.ToUpper(strings.TrimSpace(res.NAVident)), Azp: res.Azp, Groups: res.Groups}, nil
}
