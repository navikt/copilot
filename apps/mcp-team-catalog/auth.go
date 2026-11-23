package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

const (
	maxBearerLen      = 16 * 1024
	maxTexasRespBytes = 1 << 20
)

var (
	errUnauthorized  = errors.New("invalid or expired bearer token")
	errForbidden     = errors.New("token claims rejected")
	errNotConfigured = errors.New("introspection is not configured")
)

type introspectionClaims struct {
	Active   bool     `json:"active"`
	Idtyp    string   `json:"idtyp,omitempty"`
	Roles    []string `json:"roles,omitempty"`
	NAVident string   `json:"NAVident,omitempty"`
	Azp      string   `json:"azp,omitempty"`
}

func validateHTTPURL(raw string) error {
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

// extractGatewayClientID extracts fleet-gateway's Entra client ID in this cluster
// from AZURE_APP_PRE_AUTHORIZED_APPS, failing closed on unset cluster, malformed JSON,
// or zero/multiple matches.
func extractGatewayClientID(raw, cluster string) (string, error) {
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

func extractBearerToken(authHeader string) (string, bool) {
	if len(authHeader) <= len("Bearer ") || len(authHeader) > maxBearerLen {
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

func (s *server) validateToken(ctx context.Context, token string) (string, error) {
	if s.cfg.IntrospectionEndpoint == "" || s.cfg.GatewayClientID == "" {
		return "", errNotConfigured
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.IntrospectionEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.cfg.HTTPClient.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("texas returned status %d", resp.StatusCode)
	}

	body, err := readLimitedBody(resp.Body, maxTexasRespBytes)
	if err != nil {
		return "", fmt.Errorf("read texas introspection response: %w", err)
	}

	var raw introspectionClaims
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("decode texas response: %w", err)
	}

	if !raw.Active {
		return "", errUnauthorized
	}
	if strings.EqualFold(raw.Idtyp, "app") || slices.Contains(raw.Roles, "access_as_application") {
		return "", errForbidden
	}
	navIdent := strings.TrimSpace(raw.NAVident)
	if navIdent == "" {
		return "", errForbidden
	}
	if subtle.ConstantTimeCompare([]byte(raw.Azp), []byte(s.cfg.GatewayClientID)) != 1 {
		return "", errForbidden
	}

	return navIdent, nil
}

type tokenExchangeError struct {
	statusCode int
	msg        string
}

func (e *tokenExchangeError) Error() string {
	return e.msg
}

func (s *server) exchangeToken(ctx context.Context, userToken string) (string, error) {
	if s.cfg.ExchangeEndpoint == "" || s.cfg.TeamCatalogScope == "" {
		return "", &tokenExchangeError{msg: "downstream token exchange is not configured"}
	}

	payload, err := json.Marshal(struct {
		IdentityProvider string `json:"identity_provider"`
		Target           string `json:"target"`
		UserToken        string `json:"user_token"`
	}{
		IdentityProvider: "entra_id",
		Target:           s.cfg.TeamCatalogScope,
		UserToken:        userToken,
	})
	if err != nil {
		return "", &tokenExchangeError{msg: "failed to encode token exchange request"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.ExchangeEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", &tokenExchangeError{msg: "failed to build token exchange request"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.cfg.HTTPClient.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return "", &tokenExchangeError{msg: "token exchange request failed"}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		code, aadsts := texasError(resp.Body)
		slog.Warn("texas token exchange failed", "status", resp.StatusCode, "error", code, "aadsts", aadsts)
		msg := fmt.Sprintf("token exchange failed with status %d", resp.StatusCode)
		if code != "" {
			msg += ": " + code
		}
		if aadsts != "" {
			msg += " " + aadsts
		}
		return "", &tokenExchangeError{statusCode: resp.StatusCode, msg: msg}
	}

	body, err := readLimitedBody(resp.Body, maxTexasRespBytes)
	if err != nil {
		return "", &tokenExchangeError{msg: "token exchange response exceeded limit"}
	}

	var out struct {
		AccessToken string `json:"access_token"` //nolint:gosec // Texas OBO response field
	}
	if err := json.Unmarshal(body, &out); err != nil || strings.TrimSpace(out.AccessToken) == "" {
		return "", &tokenExchangeError{msg: "invalid token exchange response"}
	}
	return out.AccessToken, nil
}

// texasError returns only the OAuth error code and the AADSTS number from a
// failed exchange. error_description is never returned: Entra can name the user.
func texasError(r io.Reader) (code, aadsts string) {
	b, err := readLimitedBody(r, maxTexasRespBytes)
	if err != nil {
		return "", ""
	}
	var e struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if json.Unmarshal(b, &e) != nil {
		return "", ""
	}
	if safeErrorCode.MatchString(e.Error) {
		code = e.Error
	}
	return code, aadstsCode.FindString(e.Description)
}

var (
	aadstsCode    = regexp.MustCompile(`AADSTS\d{4,6}`)
	safeErrorCode = regexp.MustCompile(`^[a-z_]{1,40}$`)
)

func readLimitedBody(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("response body exceeded %d bytes", limit)
	}
	return b, nil
}
