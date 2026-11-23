package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Callers allowed to read the internal API. Each is matched by its full Entra
// client id, read from config, never by a name.
const (
	callerGateway  = "fleet-gateway"
	callerRegistry = "mcp-registry"
)

// authz is the M2M guard: a Texas-validated Entra app token whose azp maps
// to a caller allowed on the route.
type authz struct {
	introspectURL string
	client        *http.Client
	callers       map[string]string // azp -> caller
}

func newAuthzFromEnv() (*authz, error) {
	endpoint := os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT")
	gw, reg := strings.TrimSpace(os.Getenv("FLEET_GATEWAY_CLIENT_ID")), strings.TrimSpace(os.Getenv("MCP_REGISTRY_CLIENT_ID"))
	return newAuthz(endpoint, gw, reg)
}

func newAuthz(endpoint, gatewayID, registryID string) (*authz, error) {
	if u, err := url.Parse(endpoint); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("NAIS_TOKEN_INTROSPECTION_ENDPOINT must be an http(s) URL")
	}
	if gatewayID == "" || registryID == "" {
		return nil, errors.New("FLEET_GATEWAY_CLIENT_ID and MCP_REGISTRY_CLIENT_ID must be set")
	}
	if gatewayID == registryID {
		return nil, errors.New("FLEET_GATEWAY_CLIENT_ID and MCP_REGISTRY_CLIENT_ID must differ")
	}
	return &authz{
		introspectURL: endpoint,
		client:        &http.Client{Timeout: 5 * time.Second},
		callers:       map[string]string{gatewayID: callerGateway, registryID: callerRegistry},
	}, nil
}

var (
	errUnauthorized = errors.New("invalid token")
	errTexas        = errors.New("token validation unavailable")
)

// require wraps h so only the listed callers reach it: 401 for a missing or
// invalid token, 403 for another caller, 503 when Texas fails.
func (a *authz) require(h http.HandlerFunc, allowed ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := a.caller(r.Context(), r.Header.Get("Authorization"))
		switch {
		case errors.Is(err, errUnauthorized):
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		case err != nil:
			slog.Error("token introspection failed", "error", err)
			http.Error(w, "token validation unavailable", http.StatusServiceUnavailable)
			return
		}
		for _, c := range allowed {
			if c == caller {
				h(w, r)
				return
			}
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	}
}

// caller returns the caller name for a valid app token, "" for a valid app
// token from an unknown client.
func (a *authz) caller(ctx context.Context, header string) (string, error) {
	claims, err := a.introspect(ctx, header)
	if err != nil {
		return "", err
	}
	// Only app tokens: a user token on behalf of fleet-gateway also has its azp.
	if claims.Idtyp != "app" {
		return "", errUnauthorized
	}
	return a.callers[claims.Azp], nil
}

type tokenClaims struct {
	Active   bool   `json:"active"`
	Idtyp    string `json:"idtyp"`
	Azp      string `json:"azp"`
	NAVident string `json:"NAVident"`
	Email    string `json:"preferred_username"`
}

var navIdentRegex = regexp.MustCompile(`^[A-Z][0-9]{6}$`)

// user returns the NAVident and normalised preferred_username of a valid user token that adminClientID obtained
// on behalf of a person (OBO). App tokens and other clients are rejected.
func (a *authz) user(ctx context.Context, header, adminClientID string) (ident, email string, err error) {
	claims, err := a.introspect(ctx, header)
	if err != nil {
		return "", "", err
	}
	if claims.Idtyp == "app" || adminClientID == "" || claims.Azp != adminClientID || !navIdentRegex.MatchString(claims.NAVident) {
		return "", "", errUnauthorized
	}
	// Team membership is keyed on preferred_username; without it there is
	// nothing to look up, so refuse.
	email = normEmail(claims.Email)
	if email == "" {
		return "", "", errUnauthorized
	}
	return claims.NAVident, email, nil
}

// introspect validates the bearer token with Texas and returns its claims
// when active.
func (a *authz) introspect(ctx context.Context, header string) (*tokenClaims, error) {
	tok, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || tok == "" || len(header) > 16*1024 || strings.ContainsAny(tok, " \t\r\n") {
		return nil, errUnauthorized
	}
	body, _ := json.Marshal(map[string]string{"identity_provider": "entra_id", "token": tok})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.introspectURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errTexas, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errTexas, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", errTexas, resp.StatusCode)
	}
	var claims tokenClaims
	const maxBody = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read: %v", errTexas, err)
	}
	if len(raw) > maxBody {
		return nil, fmt.Errorf("%w: response over %d bytes", errTexas, maxBody)
	}
	// Unmarshal, unlike a single Decode, rejects trailing content.
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", errTexas, err)
	}
	if !claims.Active {
		return nil, errUnauthorized
	}
	return &claims, nil
}
