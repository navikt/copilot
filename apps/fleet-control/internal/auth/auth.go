// Package auth validates Entra tokens through Texas, and resolves team
// membership through the Nais Console API or a dev-only static list.
package auth

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
	CallerGateway  = "fleet-gateway"
	CallerRegistry = "mcp-registry"
)

// Authz is the M2M guard: a Texas-validated Entra app token whose azp maps
// to a caller allowed on the route.
type Authz struct {
	introspectURL string
	client        *http.Client
	callers       map[string]string // azp -> caller
	homeTenant    string            // Entra tenant id; person tokens from any other tid are refused
}

// NewAuthzFromEnv builds the guard from the Nais environment; a missing or
// invalid setting is an error, so startup is refused.
func NewAuthzFromEnv() (*Authz, error) {
	endpoint := os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT")
	gw, err := ClientIDFromEnv("FLEET_GATEWAY_CLIENT_ID", "fleet-gateway")
	if err != nil {
		return nil, err
	}
	reg, err := ClientIDFromEnv("MCP_REGISTRY_CLIENT_ID", "mcp-registry")
	if err != nil {
		return nil, err
	}
	a, err := newAuthz(endpoint, gw, reg)
	if a != nil {
		// Set by Nais when azure.application is enabled. Empty: every person token is refused.
		a.homeTenant = strings.TrimSpace(os.Getenv("AZURE_APP_TENANT_ID"))
	}
	return a, err
}

func newAuthz(endpoint, gatewayID, registryID string) (*Authz, error) {
	if u, err := url.Parse(endpoint); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("NAIS_TOKEN_INTROSPECTION_ENDPOINT must be an http(s) URL")
	}
	if gatewayID == "" || registryID == "" {
		return nil, errors.New("FLEET_GATEWAY_CLIENT_ID and MCP_REGISTRY_CLIENT_ID must be set")
	}
	if gatewayID == registryID {
		return nil, errors.New("FLEET_GATEWAY_CLIENT_ID and MCP_REGISTRY_CLIENT_ID must differ")
	}
	return &Authz{
		introspectURL: endpoint,
		client:        &http.Client{Timeout: 5 * time.Second},
		callers:       map[string]string{gatewayID: CallerGateway, registryID: CallerRegistry},
	}, nil
}

var (
	// ErrUnauthorized means the token is missing, invalid or not accepted here.
	ErrUnauthorized = errors.New("invalid token")
	errTexas        = errors.New("token validation unavailable")
)

// Require wraps h so only the listed callers reach it: 401 for a missing or
// invalid token, 403 for another caller, 503 when Texas fails.
func (a *Authz) Require(h http.HandlerFunc, allowed ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, err := a.caller(r.Context(), r.Header.Get("Authorization"))
		switch {
		case errors.Is(err, ErrUnauthorized):
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

// KnownClient reports whether azp is already mapped to an M2M caller.
func (a *Authz) KnownClient(azp string) bool {
	_, ok := a.callers[azp]
	return ok
}

// caller returns the caller name for a valid app token, "" for a valid app
// token from an unknown client.
func (a *Authz) caller(ctx context.Context, header string) (string, error) {
	claims, err := a.Introspect(ctx, header)
	if err != nil {
		return "", err
	}
	// Only app tokens: a user token on behalf of fleet-gateway also has its azp.
	if claims.Idtyp != "app" {
		return "", ErrUnauthorized
	}
	return a.callers[claims.Azp], nil
}

// TokenClaims are the Texas introspection claims fleet-control reads.
type TokenClaims struct {
	Active   bool   `json:"active"`
	Idtyp    string `json:"idtyp"`
	Azp      string `json:"azp"`
	NAVident string `json:"NAVident"`
	Email    string `json:"preferred_username"`
	Tid      string `json:"tid"`
	// Oid is the Entra object id of a person. The two-person rule in
	// v1alpha1 compares it; /admin/v1 does not read it.
	Oid string `json:"oid"`
	// acct is an optional Entra claim: 0 member, 1 guest. Nais does not
	// document it, so it may be absent.
	Acct *json.Number `json:"acct"`
}

// guest reports whether claims may belong to a guest account; anything
// missing or unexpected counts as a guest. acct decides when present.
// Otherwise the token must come from the home tenant and preferred_username
// must not be a B2B guest UPN (#EXT#).
func (c *TokenClaims) guest(homeTenant string) bool {
	if c.Acct != nil {
		return c.Acct.String() != "0"
	}
	return homeTenant == "" || c.Tid != homeTenant || strings.Contains(strings.ToUpper(c.Email), "#EXT#")
}

var navIdentRegex = regexp.MustCompile(`^[A-Z][0-9]{6}$`)

// User returns the NAVident and normalised preferred_username of a valid User token that adminClientID obtained
// on behalf of a person (OBO). App tokens and other clients are rejected.
func (a *Authz) User(ctx context.Context, header, adminClientID string) (ident, email string, err error) {
	claims, err := a.Introspect(ctx, header)
	if err != nil {
		return "", "", err
	}
	return a.checkUser(claims, adminClientID)
}

func (a *Authz) checkUser(claims *TokenClaims, adminClientID string) (ident, email string, err error) {
	// Person routes: idtyp absent or "user", and no guest accounts.
	if (claims.Idtyp != "" && claims.Idtyp != "user") || claims.guest(a.homeTenant) || adminClientID == "" || claims.Azp != adminClientID || !navIdentRegex.MatchString(claims.NAVident) {
		return "", "", ErrUnauthorized
	}
	// Team membership is keyed on preferred_username; without it there is
	// nothing to look up, so refuse.
	email = normEmail(claims.Email)
	if email == "" {
		return "", "", ErrUnauthorized
	}
	return claims.NAVident, email, nil
}

// Orderer is user plus the Entra oid, for /apis: a person token without a
// lowercase-UUID oid is refused like any invalid token. When delegated is set, it also passes a
// person's OBO token obtained by another app. azp is then that app's client
// id, and the handler must bind it to a published agent profile. Texas has
// checked aud, so the app is one preauthorized in accessPolicy.inbound.
func (a *Authz) Orderer(ctx context.Context, header, adminClientID string, delegated bool) (ident, email, oid, azp string, err error) {
	claims, err := a.Introspect(ctx, header)
	if err != nil {
		return "", "", "", "", err
	}
	if !uuidRe.MatchString(claims.Oid) {
		return "", "", "", "", ErrUnauthorized
	}
	client := adminClientID
	if delegated && adminClientID != "" && claims.Azp != adminClientID && uuidRe.MatchString(claims.Azp) {
		client = claims.Azp
	}
	ident, email, err = a.checkUser(claims, client)
	return ident, email, claims.Oid, claims.Azp, err
}

// Introspect validates the bearer token with Texas and returns its claims
// when active.
func (a *Authz) Introspect(ctx context.Context, header string) (*TokenClaims, error) {
	tok, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || tok == "" || len(header) > 16*1024 || strings.ContainsAny(tok, " \t\r\n") {
		return nil, ErrUnauthorized
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
	var claims TokenClaims
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
		return nil, ErrUnauthorized
	}
	return &claims, nil
}

// ClientIDFromEnv resolves app's Entra client id from AZURE_APP_PRE_AUTHORIZED_APPS,
// which Nais fills from accessPolicy.inbound, matched on
// "<NAIS_CLUSTER_NAME>:copilot:<app>". Not listed there: the explicit envVar.
// Unparseable JSON refuses startup rather than falling back.
func ClientIDFromEnv(envVar, app string) (string, error) {
	if raw := strings.TrimSpace(os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS")); raw != "" {
		var apps []struct {
			Name     string `json:"name"`
			ClientID string `json:"clientId"`
		}
		if err := json.Unmarshal([]byte(raw), &apps); err != nil {
			return "", fmt.Errorf("AZURE_APP_PRE_AUTHORIZED_APPS: %w", err)
		}
		want := os.Getenv("NAIS_CLUSTER_NAME") + ":copilot:" + app
		for _, a := range apps {
			if a.Name == want && strings.TrimSpace(a.ClientID) != "" {
				return strings.TrimSpace(a.ClientID), nil
			}
		}
	}
	return strings.TrimSpace(os.Getenv(envVar)), nil
}
