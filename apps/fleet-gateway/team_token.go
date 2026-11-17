package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	strategyOBO     = "obo"
	strategyRefresh = "refresh"
)

type upstreamAccessMissingError struct {
	App string
}

func (e *upstreamAccessMissingError) Error() string {
	return fmt.Sprintf("fleet-gateway has no access to %s; the team must add fleet-gateway to accessPolicy.inbound", e.App)
}

type upstreamTokenError struct {
	reason string
}

func (e *upstreamTokenError) Error() string {
	if e.reason != "" {
		return e.reason
	}
	return "upstream token acquisition failed"
}

func newTeamTokenSource(cfg *Config, sessions *ttlMap[*gatewaySession], now func() time.Time) (TeamTokenSource, error) {
	if now == nil {
		now = time.Now
	}
	byRes := make(map[string]PilotRoute, len(cfg.Routes))
	for _, r := range cfg.Routes {
		byRes[cfg.BaseURL+r.Route] = r
	}

	switch cfg.TeamTokenStrategy {
	case "", strategyOBO:
		return &StrategyOBO{
			sessions:         sessions,
			routesByResource: byRes,
			endpoint:         cfg.TexasExchangeEndpoint,
			client:           cfg.HTTPClient,
			now:              now,
		}, nil
	case strategyRefresh:
		return &StrategyRefresh{
			sessions:         sessions,
			routesByResource: byRes,
			tokenEndpoint:    cfg.AzureTokenEndpoint,
			clientID:         cfg.AzureClientID,
			clientSecret:     cfg.AzureClientSecret,
			client:           cfg.HTTPClient,
			now:              now,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported TEAM_TOKEN_STRATEGY %q", cfg.TeamTokenStrategy)
	}
}

// StrategyOBO exchanges the session's Entra access token via Nais Texas OBO.
type StrategyOBO struct {
	sessions         *ttlMap[*gatewaySession]
	routesByResource map[string]PilotRoute
	endpoint         string
	client           *http.Client
	now              func() time.Time
}

// TeamToken exchanges the session's Entra access token for a downstream team token via Texas.
func (s *StrategyOBO) TeamToken(ctx context.Context, sessionID, resource string) (string, error) {
	if s.endpoint == "" {
		return "", &upstreamTokenError{reason: "texas exchange endpoint not configured"}
	}
	route, ok := s.routesByResource[resource]
	if !ok {
		return "", fmt.Errorf("unknown route resource %q", resource)
	}
	sess, ok := s.sessions.Get(sessionID)
	if !ok || !s.now().Before(sess.expiry) {
		return "", errors.New("session not found or expired")
	}
	if sess.entraToken == "" {
		return "", &upstreamTokenError{reason: "session has no entra access token"}
	}

	payload, err := json.Marshal(struct {
		IdentityProvider string `json:"identity_provider"`
		Target           string `json:"target"`
		UserToken        string `json:"user_token"`
	}{
		IdentityProvider: "entra_id",
		Target:           route.Target,
		UserToken:        sess.entraToken,
	})
	if err != nil {
		return "", &upstreamTokenError{reason: "encode texas request"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", &upstreamTokenError{reason: "build texas request"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fleet-gateway/1.0")

	resp, err := s.client.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return "", &upstreamTokenError{reason: "texas exchange request failed"}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamBodyBytes))
	if err != nil {
		return "", &upstreamTokenError{reason: "read texas response"}
	}

	if resp.StatusCode != http.StatusOK {
		return "", mapUpstreamTokenFailure(body, route.App, resp.StatusCode)
	}

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return "", &upstreamTokenError{reason: "invalid texas token response"}
	}
	accessToken, _ := out["access_token"].(string)
	if accessToken == "" {
		return "", &upstreamTokenError{reason: "invalid texas token response"}
	}
	return accessToken, nil
}

// StrategyRefresh redeems the session's in-memory Entra refresh token directly at Entra.
type StrategyRefresh struct {
	sessions         *ttlMap[*gatewaySession]
	routesByResource map[string]PilotRoute
	tokenEndpoint    string
	clientID         string
	clientSecret     string
	client           *http.Client
	now              func() time.Time
	sf               singleflight.Group
}

// TeamToken acquires a downstream team token by refreshing against Entra with per-session serialization.
func (s *StrategyRefresh) TeamToken(ctx context.Context, sessionID, resource string) (string, error) {
	if s.tokenEndpoint == "" {
		return "", &upstreamTokenError{reason: "entra token endpoint not configured"}
	}
	route, ok := s.routesByResource[resource]
	if !ok {
		return "", fmt.Errorf("unknown route resource %q", resource)
	}
	sess, ok := s.sessions.Get(sessionID)
	if !ok || !s.now().Before(sess.expiry) {
		return "", errors.New("session not found or expired")
	}

	sfKey := sessionID + "\x00" + route.Target
	ch := s.sf.DoChan(sfKey, func() (any, error) {
		reqCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultOutboundTimeout)
		defer cancel()

		sess.refreshMu.Lock()
		defer sess.refreshMu.Unlock()

		if !s.now().Before(sess.expiry) {
			return nil, errors.New("session expired")
		}
		if sess.refreshToken == "" {
			return nil, &upstreamTokenError{reason: "session has no refresh token"}
		}

		form := url.Values{
			"client_id":     {s.clientID},
			"client_secret": {s.clientSecret},
			"grant_type":    {"refresh_token"},
			"refresh_token": {sess.refreshToken},
			"scope":         {"api://" + route.Target + "/.default"},
		}

		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, s.tokenEndpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, &upstreamTokenError{reason: "build entra refresh request"}
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "fleet-gateway/1.0")

		resp, err := s.client.Do(req) //nolint:gosec // endpoint validated at startup
		if err != nil {
			return nil, &upstreamTokenError{reason: "entra refresh request failed"}
		}
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamBodyBytes))
		if err != nil {
			return nil, &upstreamTokenError{reason: "read entra refresh response"}
		}

		if resp.StatusCode != http.StatusOK {
			return nil, mapUpstreamTokenFailure(body, route.App, resp.StatusCode)
		}

		var out map[string]any
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, &upstreamTokenError{reason: "invalid entra refresh response"}
		}
		accessToken, _ := out["access_token"].(string)
		if accessToken == "" {
			return nil, &upstreamTokenError{reason: "invalid entra refresh response"}
		}
		if newRT, _ := out["refresh_token"].(string); newRT != "" {
			sess.refreshToken = newRT
		}
		return accessToken, nil
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return "", res.Err
		}
		tok, _ := res.Val.(string)
		return tok, nil
	}
}

func mapUpstreamTokenFailure(body []byte, app string, status int) error {
	var errPayload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &errPayload) == nil {
		if strings.Contains(errPayload.ErrorDescription, "AADSTS65001") ||
			strings.Contains(errPayload.Error, "AADSTS65001") {
			return &upstreamAccessMissingError{App: app}
		}
	}
	if bytes.Contains(body, []byte("AADSTS65001")) {
		return &upstreamAccessMissingError{App: app}
	}
	// Log-safe diagnostics only: the OAuth error code and the AADSTS number.
	code := errPayload.Error
	if !safeErrorCode.MatchString(code) {
		code = ""
	}
	aadsts := aadstsCode.FindString(errPayload.ErrorDescription)
	return &upstreamTokenError{reason: fmt.Sprintf("upstream token endpoint returned status %d error=%q %s", status, code, aadsts)}
}

var (
	aadstsCode    = regexp.MustCompile(`AADSTS\d{4,6}`)
	safeErrorCode = regexp.MustCompile(`^[a-z_]{1,40}$`)
)
