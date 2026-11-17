package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
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

func newTeamTokenSource(cfg *Config, store SessionStore, now func() time.Time) (TeamTokenSource, error) {
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
			store:            store,
			sessionMaxAge:    cfg.SessionMaxAge,
			routesByResource: byRes,
			endpoint:         cfg.TexasExchangeEndpoint,
			client:           cfg.HTTPClient,
			now:              now,
		}, nil
	case strategyRefresh:
		return &StrategyRefresh{
			store:            store,
			sessionMaxAge:    cfg.SessionMaxAge,
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
	store            SessionStore
	sessionMaxAge    time.Duration
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
	sess, ok, err := getSessionFromStore(ctx, s.store, s.now, s.sessionMaxAge, sessionID)
	if err != nil {
		return "", fmt.Errorf("load session: %w", err)
	}
	if !ok || !s.now().Before(sess.Expiry) {
		return "", errors.New("session not found or expired")
	}
	if sess.EntraToken == "" {
		return "", &upstreamTokenError{reason: "session has no entra access token"}
	}

	payload, err := json.Marshal(struct {
		IdentityProvider string `json:"identity_provider"`
		Target           string `json:"target"`
		UserToken        string `json:"user_token"`
	}{
		IdentityProvider: "entra_id",
		Target:           route.Target,
		UserToken:        sess.EntraToken,
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

// StrategyRefresh redeems the session's Entra refresh token directly at Entra.
type StrategyRefresh struct {
	store            SessionStore
	sessionMaxAge    time.Duration
	routesByResource map[string]PilotRoute
	tokenEndpoint    string
	clientID         string
	clientSecret     string
	client           *http.Client
	now              func() time.Time
	sf               singleflight.Group
	refreshMu        [64]sync.Mutex
}

func (s *StrategyRefresh) lockSession(sessionID string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(sessionID))
	return &s.refreshMu[h.Sum32()%uint32(len(s.refreshMu))]
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
	sess, ok, err := getSessionFromStore(ctx, s.store, s.now, s.sessionMaxAge, sessionID)
	if err != nil {
		return "", fmt.Errorf("load session: %w", err)
	}
	if !ok || !s.now().Before(sess.Expiry) {
		return "", errors.New("session not found or expired")
	}

	sfKey := sessionID + "\x00" + route.Target
	ch := s.sf.DoChan(sfKey, func() (any, error) {
		reqCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultOutboundTimeout)
		defer cancel()

		// shortcut: cross-replica concurrent refresh is not locked, Entra tolerates RT reuse in its grace window; add advisory lock if observed
		mu := s.lockSession(sessionID)
		mu.Lock()
		defer mu.Unlock()

		freshSess, ok, err := getSessionFromStore(reqCtx, s.store, s.now, s.sessionMaxAge, sessionID)
		if err != nil {
			return nil, fmt.Errorf("load session: %w", err)
		}
		if !ok || !s.now().Before(freshSess.Expiry) {
			return nil, errors.New("session expired")
		}
		if freshSess.RefreshToken == "" {
			return nil, &upstreamTokenError{reason: "session has no refresh token"}
		}

		form := url.Values{
			"client_id":     {s.clientID},
			"client_secret": {s.clientSecret},
			"grant_type":    {"refresh_token"},
			"refresh_token": {freshSess.RefreshToken},
			"scope":         {entraScopeForTarget(route.Target)},
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
			freshSess.RefreshToken = newRT
			if remTTL := freshSess.Expiry.Sub(s.now()); remTTL > 0 {
				if err := putSessionInStore(reqCtx, s.store, s.now, s.sessionMaxAge, sessionID, freshSess, remTTL); err != nil {
					return nil, fmt.Errorf("persist rotated refresh token: %w", err)
				}
			}
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

func entraScopeForTarget(target string) string {
	cluster, ns, app, err := parseTarget(target)
	if err == nil {
		return "api://" + cluster + "." + ns + "." + app + "/.default"
	}
	return "api://" + strings.ReplaceAll(target, ":", ".") + "/.default"
}
