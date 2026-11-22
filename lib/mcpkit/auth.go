// Package mcpkit holds the code Nav's Go MCP servers share: Texas
// introspection and OBO exchange, the MCP 2026-07-28 wire rules, body limits
// and per-tool metrics. Standard library only. The HTTP handler itself stays
// in each server, since each owns its tools and its JSON-RPC edge cases.
package mcpkit

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const (
	// MaxBearerLen is the longest Authorization header accepted.
	MaxBearerLen      = 16 * 1024
	maxTexasRespBytes = 1 << 20
)

// Introspection outcomes. Map them to 401, 403 and 503; any other error from
// Introspect means Texas failed and is a 502.
var (
	ErrUnauthorized  = errors.New("invalid or expired bearer token")
	ErrForbidden     = errors.New("token claims rejected")
	ErrNotConfigured = errors.New("introspection is not configured")
)

// Texas talks to the Nais token sidecar. Empty endpoints fail closed.
type Texas struct {
	IntrospectionEndpoint string
	ExchangeEndpoint      string
	// GatewayClientID is the only azp accepted, see GatewayClientID.
	GatewayClientID string
	HTTPClient      *http.Client
}

type introspectionClaims struct {
	Active   bool     `json:"active"`
	Idtyp    string   `json:"idtyp,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	NAVident string   `json:"NAVident,omitempty"`
	Azp      string   `json:"azp,omitempty"`
}

// ValidateHTTPURL checks that raw is an absolute http or https URL with a host.
func ValidateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("missing host")
	}
	return nil
}

// GatewayClientID extracts fleet-gateway's Entra client ID in this cluster
// from AZURE_APP_PRE_AUTHORIZED_APPS, failing closed on unset cluster, malformed JSON,
// or zero/multiple matches.
func GatewayClientID(raw, cluster string) (string, error) {
	cluster = strings.TrimSpace(cluster)
	if strings.TrimSpace(raw) == "" || cluster == "" {
		return "", nil
	}
	var apps []struct {
		Name     string `json:"name"`
		ClientID string `json:"clientId"`
	}
	if err := json.Unmarshal([]byte(raw), &apps); err != nil {
		return "", fmt.Errorf("parse AZURE_APP_PRE_AUTHORIZED_APPS: %w", err)
	}
	var matches []string
	for _, app := range apps {
		if app.Name == cluster+":copilot:fleet-gateway" && app.ClientID != "" {
			matches = append(matches, app.ClientID)
		}
	}
	if len(matches) != 1 {
		return "", nil
	}
	return matches[0], nil
}

// BearerToken returns the token from an Authorization header, rejecting
// other schemes, oversized headers and tokens with whitespace.
func BearerToken(authHeader string) (string, bool) {
	if len(authHeader) <= len("Bearer ") || len(authHeader) > MaxBearerLen {
		return "", false
	}
	if !strings.EqualFold(authHeader[:len("Bearer ")], "Bearer ") {
		return "", false
	}
	tok := strings.TrimSpace(authHeader[len("Bearer "):])
	if tok == "" || strings.ContainsAny(tok, " \t\r\n") {
		return "", false
	}
	return tok, true
}

// Introspect validates a user token with Texas and returns its NAVident.
// App tokens, tokens without NAVident and tokens whose azp is not the
// gateway are ErrForbidden.
func (t *Texas) Introspect(ctx context.Context, token string) (string, error) {
	if t.IntrospectionEndpoint == "" || t.GatewayClientID == "" {
		return "", ErrNotConfigured
	}

	payload, err := json.Marshal(struct {
		IdentityProvider string `json:"identity_provider"`
		Token            string `json:"token"`
	}{
		IdentityProvider: "entra_id",
		Token:            token,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.IntrospectionEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := t.HTTPClient.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("texas returned status %d", resp.StatusCode)
	}

	body, err := ReadLimited(resp.Body, maxTexasRespBytes)
	if err != nil {
		return "", fmt.Errorf("read texas introspection response: %w", err)
	}

	var raw introspectionClaims
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("decode texas response: %w", err)
	}

	if !raw.Active {
		return "", ErrUnauthorized
	}
	if strings.EqualFold(raw.Idtyp, "app") || slices.Contains(raw.Roles, "access_as_application") {
		return "", ErrForbidden
	}
	navIdent := strings.TrimSpace(raw.NAVident)
	if navIdent == "" {
		return "", ErrForbidden
	}
	if subtle.ConstantTimeCompare([]byte(raw.Azp), []byte(t.GatewayClientID)) != 1 {
		return "", ErrForbidden
	}

	return navIdent, nil
}

// ExchangeError is an OBO failure. Its message never carries a token or a
// response body, so it is safe to show in a tool result.
type ExchangeError struct {
	StatusCode int // Texas' status, 0 when no response was read
	Msg        string
}

func (e *ExchangeError) Error() string {
	return e.Msg
}

// Exchange trades userToken for an on-behalf-of token for target.
// Every error is an *ExchangeError.
func (t *Texas) Exchange(ctx context.Context, userToken, target string) (string, error) {
	if t.ExchangeEndpoint == "" || target == "" {
		return "", &ExchangeError{Msg: "downstream token exchange is not configured"}
	}

	payload, err := json.Marshal(struct {
		IdentityProvider string `json:"identity_provider"`
		Target           string `json:"target"`
		UserToken        string `json:"user_token"`
	}{
		IdentityProvider: "entra_id",
		Target:           target,
		UserToken:        userToken,
	})
	if err != nil {
		return "", &ExchangeError{Msg: "failed to encode token exchange request"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.ExchangeEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", &ExchangeError{Msg: "failed to build token exchange request"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := t.HTTPClient.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return "", &ExchangeError{Msg: "token exchange request failed"}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", &ExchangeError{
			StatusCode: resp.StatusCode,
			Msg:        fmt.Sprintf("token exchange failed with status %d", resp.StatusCode),
		}
	}

	body, err := ReadLimited(resp.Body, maxTexasRespBytes)
	if err != nil {
		return "", &ExchangeError{Msg: "token exchange response exceeded limit"}
	}

	var out struct {
		AccessToken string `json:"access_token"` //nolint:gosec // Texas OBO response field
	}
	if err := json.Unmarshal(body, &out); err != nil || strings.TrimSpace(out.AccessToken) == "" {
		return "", &ExchangeError{Msg: "invalid token exchange response"}
	}
	return out.AccessToken, nil
}

// ReadLimited reads r, failing once it passes limit bytes; it never
// buffers more than limit+1.
func ReadLimited(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("response body exceeded %d bytes", limit)
	}
	return b, nil
}

// ReadRequestBody reads at most limit bytes of r's body. tooLarge reports a
// body over limit (answer 413); the read stops there instead of buffering it.
func ReadRequestBody(w http.ResponseWriter, r *http.Request, limit int64) (body []byte, tooLarge bool, err error) {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	body, err = io.ReadAll(r.Body)
	var maxErr *http.MaxBytesError
	return body, errors.As(err, &maxErr), err
}

// SetChallenge sets the RFC 6750 WWW-Authenticate header for a 401. A missing
// or non-Bearer token gets only the realm; a presented token that failed
// validation gets error="invalid_token" (section 3.1).
func SetChallenge(w http.ResponseWriter, realm string, tokenPresented bool) {
	if tokenPresented {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		return
	}
	w.Header().Set("WWW-Authenticate", `Bearer realm="`+realm+`"`)
}
