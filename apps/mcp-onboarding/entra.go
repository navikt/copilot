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
	"os"
	"slices"
	"strings"
	"time"
)

// EntraAuth authenticates requests that fleet-gateway forwards with an OBO
// token for this app. It runs on its own path so the GitHub OAuth path on
// /mcp is unchanged. Checks follow the fleet-gateway team guide and
// apps/fleet-gateway/echo: Texas introspection (aud), no app tokens, NAVident
// present, azp is fleet-gateway.
type EntraAuth struct {
	introspectionEndpoint string
	gatewayClientID       string
	client                *http.Client
}

var (
	errEntraUnauthorized = errors.New("token not active")
	errEntraForbidden    = errors.New("token not accepted")
)

// NewEntraAuthFromEnv returns nil when Entra is not configured, so the /mcp/gw
// route is never registered there.
func NewEntraAuthFromEnv() (*EntraAuth, error) {
	endpoint := strings.TrimSpace(os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT"))
	clientID, err := gatewayClientID(os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS"), os.Getenv("NAIS_CLUSTER_NAME"))
	if err != nil {
		return nil, err
	}
	if endpoint == "" || clientID == "" {
		return nil, nil
	}
	return &EntraAuth{
		introspectionEndpoint: endpoint,
		gatewayClientID:       clientID,
		client: &http.Client{
			Timeout:       5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects refused") },
		},
	}, nil
}

// gatewayClientID picks fleet-gateway's client ID in this cluster from
// AZURE_APP_PRE_AUTHORIZED_APPS. An unset cluster, or zero or several matches,
// yield "", which leaves Entra mode off.
func gatewayClientID(raw, cluster string) (string, error) {
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
	for _, a := range apps {
		if a.Name == cluster+":copilot:fleet-gateway" && a.ClientID != "" {
			matches = append(matches, a.ClientID)
		}
	}
	if len(matches) != 1 {
		return "", nil
	}
	return matches[0], nil
}

func (e *EntraAuth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "bearer") || strings.TrimSpace(token) == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="mcp-onboarding"`) // RFC 6750 section 3.1: no error code without a token
			writeEntraError(w, http.StatusUnauthorized)
			return
		}
		user, err := e.validate(r.Context(), strings.TrimSpace(token))
		switch {
		case errors.Is(err, errEntraUnauthorized):
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			writeEntraError(w, http.StatusUnauthorized)
			return
		case errors.Is(err, errEntraForbidden):
			writeEntraError(w, http.StatusForbidden)
			return
		case err != nil:
			slog.Warn("texas introspection failed", "error", err)
			writeEntraError(w, http.StatusBadGateway)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func (e *EntraAuth) validate(ctx context.Context, token string) (*UserContext, error) {
	payload, _ := json.Marshal(map[string]string{"identity_provider": "entra_id", "token": token})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.introspectionEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req) //nolint:gosec // endpoint injected by Nais
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("texas returned status %d", resp.StatusCode)
	}
	var c struct {
		Active   bool     `json:"active"`
		Idtyp    string   `json:"idtyp"`
		Roles    []string `json:"roles"`
		NAVident string   `json:"NAVident"`
		Azp      string   `json:"azp"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&c); err != nil {
		return nil, fmt.Errorf("decode texas response: %w", err)
	}
	if !c.Active {
		return nil, errEntraUnauthorized
	}
	if strings.EqualFold(c.Idtyp, "app") || slices.Contains(c.Roles, "access_as_application") {
		return nil, errEntraForbidden
	}
	navIdent := strings.TrimSpace(c.NAVident)
	if navIdent == "" {
		return nil, errEntraForbidden
	}
	if subtle.ConstantTimeCompare([]byte(c.Azp), []byte(e.gatewayClientID)) != 1 {
		return nil, errEntraForbidden
	}
	// Login is the NAVident, never an email. It is personal data, logged at INFO deliberately as an audit trail (owner decision 2026-11-17).
	return &UserContext{Login: navIdent, NAVident: navIdent}, nil
}

func writeEntraError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, strings.ToLower(strings.ReplaceAll(http.StatusText(status), " ", "_")))
}

// entraTools is the allowlist of tools that work without the user's GitHub
// token. Anything not listed is refused in Entra mode, so a new tool is
// closed by default.
var entraTools = map[string]bool{
	"hello_world":            true,
	"greet":                  true,
	"whoami":                 true,
	"echo":                   true,
	"get_time":               true,
	"search_customizations":  true,
	"list_agents":            true,
	"list_instructions":      true,
	"list_prompts":           true,
	"list_skills":            true,
	"get_installation_guide": true,
}

// githubLoginRequired points at this deployment's own direct endpoint (BASE_URL), so prod never names the dev host.
func githubLoginRequired(baseURL string) string {
	return "Dette verktøyet krever GitHub-innlogging. Bruk det direkte endepunktet " + strings.TrimRight(baseURL, "/") + "/mcp."
}
