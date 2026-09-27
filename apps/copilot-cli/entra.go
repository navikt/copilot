package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"
)

// entraClient validates Entra ID user tokens through the Texas sidecar's
// introspection endpoint (NAIS_TOKEN_INTROSPECTION_ENDPOINT). Texas checks
// the signature, issuer, expiry and that the audience is this app; this
// client adds what Texas does not know about:
//
//   - the token is a user's (an OBO token), never an app-only token: an
//     app-only token names no person, and accepting one would let any
//     pre-authorized app submit as itself (idtyp=app, no NAVident);
//   - the calling app (azp) is one NAIS pre-authorized in
//     accessPolicy.inbound, read from AZURE_APP_PRE_AUTHORIZED_APPS.
//
// See https://doc.nais.io/auth/entra-id/how-to/secure/.
type entraClient struct {
	httpClient *http.Client
	endpoint   string
	allowedAzp []string
}

func newEntraClient(endpoint string, preAuthorizedApps string) *entraClient {
	return &entraClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		endpoint:   endpoint,
		allowedAzp: parsePreAuthorizedApps(preAuthorizedApps),
	}
}

// parsePreAuthorizedApps reads the client IDs out of NAIS's
// AZURE_APP_PRE_AUTHORIZED_APPS ([{"name":"…","clientId":"…"}]). Malformed
// input yields none, which turns the Entra path off.
func parsePreAuthorizedApps(raw string) []string {
	var apps []struct {
		ClientID string `json:"clientId"`
	}
	if raw == "" || json.Unmarshal([]byte(raw), &apps) != nil {
		return nil
	}
	var ids []string
	for _, a := range apps {
		if a.ClientID != "" {
			ids = append(ids, a.ClientID)
		}
	}
	return ids
}

func (c *entraClient) configured() bool { return c.endpoint != "" && len(c.allowedAzp) > 0 }

func (c *entraClient) resolveUser(ctx context.Context, token string) (*AuthenticatedUser, error) {
	if !c.configured() {
		return nil, errIssuerOffline
	}
	body, err := json.Marshal(map[string]string{"identity_provider": "azuread", "token": token})
	if err != nil {
		return nil, fmt.Errorf("encoding introspection request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building introspection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling texas introspection: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("texas introspection returned status %d", resp.StatusCode)
	}

	var claims struct {
		Active   bool   `json:"active"`
		Azp      string `json:"azp"`
		Oid      string `json:"oid"`
		Idtyp    string `json:"idtyp"`
		NAVident string `json:"NAVident"`
		Exp      int64  `json:"exp"`
		// preferred_username is the Nav e-mail (UPN) on a user token.
		Email string `json:"preferred_username"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&claims); err != nil {
		return nil, fmt.Errorf("decoding introspection response: %w", err)
	}
	switch {
	case !claims.Active || claims.Exp <= 0:
		return nil, errInvalidToken
	case claims.Idtyp == "app" || claims.NAVident == "" || claims.Oid == "":
		return nil, errNotAUser
	case !slices.Contains(c.allowedAzp, claims.Azp):
		return nil, errInvalidToken
	}
	return &AuthenticatedUser{Issuer: issuerEntra, Subject: claims.Oid, email: claims.Email, expiresAt: time.Unix(claims.Exp, 0)}, nil
}

var errNotAUser = errors.New("token does not identify a user")
