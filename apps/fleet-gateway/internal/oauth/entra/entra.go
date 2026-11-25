// Package entra talks to Entra ID for the gateway's login: the authorize
// endpoint, code and refresh-token redemption, the signed state cookie value
// and id_token claim checks.
package entra

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/navikt/copilot/fleet-gateway/internal/config"
)

// Same limits as maxCIMDBodyBytes and maxUpstreamBodyBytes in main.
const (
	maxWellKnownBytes = 64 * 1024
	maxTokenBodyBytes = 1 << 20
)

// Client calls Entra ID's endpoints. Endpoint URLs are validated at startup.
type Client struct {
	HTTPClient      *http.Client
	ClientID        string
	ClientSecret    string //nolint:gosec // never serialized; sent only in the token request form
	Scope           string
	RedirectURI     string
	TokenEndpoint   string
	AuthorizeURL    string // static authorize endpoint; when empty it is read from WellKnownURL
	WellKnownURL    string
	OutboundTimeout time.Duration

	mu          sync.RWMutex
	group       singleflight.Group
	cachedAuthz string
}

// AuthorizeEndpoint returns the authorize endpoint, fetching the well-known
// metadata once.
func (c *Client) AuthorizeEndpoint(ctx context.Context) (string, error) {
	if c.AuthorizeURL != "" {
		return c.AuthorizeURL, nil
	}

	c.mu.RLock()
	cached := c.cachedAuthz
	c.mu.RUnlock()
	if cached != "" {
		return cached, nil
	}

	ch := c.group.DoChan("authz", func() (any, error) {
		c.mu.RLock()
		if c.cachedAuthz != "" {
			ep := c.cachedAuthz
			c.mu.RUnlock()
			return ep, nil
		}
		c.mu.RUnlock()

		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.OutboundTimeout)
		defer cancel()

		req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, c.WellKnownURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "fleet-gateway/1.0")

		resp, err := c.HTTPClient.Do(req) //nolint:gosec // URL validated at startup
		if err != nil {
			return "", err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("well-known returned status %d", resp.StatusCode)
		}

		var metadata struct {
			AuthorizationEndpoint string `json:"authorization_endpoint"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxWellKnownBytes)).Decode(&metadata); err != nil {
			return "", fmt.Errorf("decode well-known: %w", err)
		}
		if err := config.ValidateHTTPURL(metadata.AuthorizationEndpoint, false); err != nil {
			return "", fmt.Errorf("invalid authorization_endpoint in well-known: %w", err)
		}

		c.mu.Lock()
		c.cachedAuthz = metadata.AuthorizationEndpoint
		c.mu.Unlock()
		return metadata.AuthorizationEndpoint, nil
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return "", res.Err
		}
		ep, _ := res.Val.(string)
		return ep, nil
	}
}

// RedeemCode exchanges an Entra authorization code.
func (c *Client) RedeemCode(ctx context.Context, code, codeVerifier string) (map[string]any, int, error) {
	form := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.RedirectURI},
		"scope":         {c.Scope},
	}
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}

	body, status, err := c.postToken(ctx, form)
	if err != nil {
		return nil, status, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, status, err
	}
	return out, status, nil
}

// RedeemRefreshToken exchanges an Entra refresh token. It also returns the raw body.
func (c *Client) RedeemRefreshToken(ctx context.Context, entraRT string) (map[string]any, int, []byte, error) {
	form := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {entraRT},
		"scope":         {c.Scope},
	}
	body, status, err := c.postToken(ctx, form)
	if err != nil {
		return nil, status, body, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, status, body, err
	}
	return out, status, body, nil
}

func (c *Client) postToken(ctx context.Context, form url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fleet-gateway/1.0")

	resp, err := c.HTTPClient.Do(req) //nolint:gosec // URL validated at startup
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenBodyBytes))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// SignState returns state with an HMAC of it under key.
func SignState(key []byte, state string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("entra-state:"))
	mac.Write([]byte(state))
	return state + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyState checks that cookieVal is queryState signed by one of keys.
func VerifyState(keys [][]byte, cookieVal, queryState string) bool {
	if queryState == "" || cookieVal == "" {
		return false
	}
	parts := strings.Split(cookieVal, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	valid := false
	for _, key := range keys {
		if subtle.ConstantTimeCompare([]byte(cookieVal), []byte(SignState(key, parts[0]))) == 1 {
			valid = true
		}
	}
	if !valid {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[0]), []byte(queryState)) == 1
}

// ParseJWTClaims decodes the payload of a JWT without checking its signature.
func ParseJWTClaims(rawJWT string) (map[string]any, error) {
	parts := strings.Split(rawJWT, ".")
	if len(parts) != 3 || parts[1] == "" {
		return nil, errors.New("malformed JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil || claims == nil {
		return nil, errors.New("invalid JWT claims JSON")
	}
	return claims, nil
}

// idTokenSkew is the clock skew tolerated on exp, nbf and iat.
const idTokenSkew = 2 * time.Minute

// ValidateIDTokenClaims checks iss, aud/azp and exp/nbf/iat of the Entra
// id_token (OIDC Core 3.1.3.7, steps 2-5 and 9-10). The signature is not
// checked: the token comes straight from the configured Entra token endpoint
// over TLS in the back-channel code redemption, which step 6 allows to stand
// in for signature validation. Every claim is still treated as hostile, and a
// missing claim or empty configuration fails closed.
func ValidateIDTokenClaims(claims map[string]any, issuer, clientID string, now time.Time) error {
	if issuer == "" || clientID == "" {
		return errors.New("id_token validation not configured")
	}
	if iss, _ := claims["iss"].(string); iss != issuer {
		return errors.New("id_token iss mismatch")
	}
	var auds []string
	switch v := claims["aud"].(type) {
	case string:
		auds = []string{v}
	case []any:
		for _, a := range v {
			s, ok := a.(string)
			if !ok {
				return errors.New("id_token aud malformed")
			}
			auds = append(auds, s)
		}
	}
	if !slices.Contains(auds, clientID) {
		return errors.New("id_token aud mismatch")
	}
	if azp, ok := claims["azp"]; (len(auds) > 1 || ok) && azp != clientID {
		return errors.New("id_token azp mismatch")
	}
	numeric := func(name string) (time.Time, bool) {
		f, ok := claims[name].(float64)
		// Upper bound (year 9999) keeps int64 conversion and time math from overflowing.
		if !ok || f <= 0 || f > 253402300799 {
			return time.Time{}, false
		}
		return time.Unix(int64(f), 0), true
	}
	exp, ok := numeric("exp")
	if !ok || !now.Before(exp.Add(idTokenSkew)) {
		return errors.New("id_token expired or exp missing")
	}
	iat, ok := numeric("iat")
	if !ok || iat.After(now.Add(idTokenSkew)) {
		return errors.New("id_token iat missing or in the future")
	}
	if _, present := claims["nbf"]; present {
		nbf, ok := numeric("nbf")
		if !ok || nbf.After(now.Add(idTokenSkew)) {
			return errors.New("id_token not yet valid")
		}
	}
	return nil
}
