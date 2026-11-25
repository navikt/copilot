// Package teamtoken acquires downstream team tokens for MCP routes, by Texas
// OBO exchange or by redeeming the session's Entra refresh token.
package teamtoken

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

	"github.com/navikt/copilot/fleet-gateway/internal/oauth"
	"github.com/navikt/copilot/fleet-gateway/internal/routing"
	"github.com/navikt/copilot/fleet-gateway/internal/session"
)

// Same limit as maxUpstreamBodyBytes in main.
const maxBodyBytes = 1 << 20

// AccessMissingError means Entra refused the token because the team has not
// let fleet-gateway in (AADSTS65001).
type AccessMissingError struct {
	App string
}

func (e *AccessMissingError) Error() string {
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

// OBO exchanges the session's Entra access token via Nais Texas OBO.
type OBO struct {
	Store         session.Store
	SessionMaxAge time.Duration
	Endpoint      string
	Client        *http.Client
	Now           func() time.Time
}

// TeamToken exchanges the session's Entra access token for a downstream team token via Texas.
func (s *OBO) TeamToken(ctx context.Context, sessionID string, route routing.Route) (string, error) {
	if s.Endpoint == "" {
		return "", &upstreamTokenError{reason: "texas exchange endpoint not configured"}
	}
	sess, ok, err := oauth.GetSession(ctx, s.Store, s.Now, s.SessionMaxAge, sessionID)
	if err != nil {
		return "", fmt.Errorf("load session: %w", err)
	}
	if !ok || !s.Now().Before(sess.Expiry) {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", &upstreamTokenError{reason: "build texas request"}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "fleet-gateway/1.0")

	resp, err := s.Client.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return "", &upstreamTokenError{reason: "texas exchange request failed"}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
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

// Refresh redeems the session's Entra refresh token directly at Entra.
type Refresh struct {
	Store         session.Store
	SessionMaxAge time.Duration
	TokenEndpoint string
	ClientID      string
	ClientSecret  string //nolint:gosec // never serialized; sent only in the token request form
	Client        *http.Client
	Now           func() time.Time
	Timeout       time.Duration // per refresh call, detached from the request
	sf            singleflight.Group
	refreshMu     [64]sync.Mutex
}

func (s *Refresh) lockSession(sessionID string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(sessionID))
	return &s.refreshMu[h.Sum32()%uint32(len(s.refreshMu))]
}

// TeamToken acquires a downstream team token by refreshing against Entra with per-session serialization.
func (s *Refresh) TeamToken(ctx context.Context, sessionID string, route routing.Route) (string, error) {
	if s.TokenEndpoint == "" {
		return "", &upstreamTokenError{reason: "entra token endpoint not configured"}
	}
	sess, ok, err := oauth.GetSession(ctx, s.Store, s.Now, s.SessionMaxAge, sessionID)
	if err != nil {
		return "", fmt.Errorf("load session: %w", err)
	}
	if !ok || !s.Now().Before(sess.Expiry) {
		return "", errors.New("session not found or expired")
	}

	sfKey := sessionID + "\x00" + route.Target
	ch := s.sf.DoChan(sfKey, func() (any, error) {
		reqCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.Timeout)
		defer cancel()

		// shortcut: cross-replica concurrent refresh is not locked, Entra tolerates RT reuse in its grace window; add advisory lock if observed
		mu := s.lockSession(sessionID)
		mu.Lock()
		defer mu.Unlock()

		freshSess, ok, err := oauth.GetSession(reqCtx, s.Store, s.Now, s.SessionMaxAge, sessionID)
		if err != nil {
			return nil, fmt.Errorf("load session: %w", err)
		}
		if !ok || !s.Now().Before(freshSess.Expiry) {
			return nil, errors.New("session expired")
		}
		if freshSess.RefreshToken == "" {
			return nil, &upstreamTokenError{reason: "session has no refresh token"}
		}

		form := url.Values{
			"client_id":     {s.ClientID},
			"client_secret": {s.ClientSecret},
			"grant_type":    {"refresh_token"},
			"refresh_token": {freshSess.RefreshToken},
			"scope":         {entraScopeForTarget(route.Target)},
		}

		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, s.TokenEndpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, &upstreamTokenError{reason: "build entra refresh request"}
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "fleet-gateway/1.0")

		resp, err := s.Client.Do(req) //nolint:gosec // endpoint validated at startup
		if err != nil {
			return nil, &upstreamTokenError{reason: "entra refresh request failed"}
		}
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
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
			if remTTL := freshSess.Expiry.Sub(s.Now()); remTTL > 0 {
				if err := oauth.PutSession(reqCtx, s.Store, s.Now, s.SessionMaxAge, sessionID, freshSess, remTTL); err != nil {
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
			return &AccessMissingError{App: app}
		}
	}
	if bytes.Contains(body, []byte("AADSTS65001")) {
		return &AccessMissingError{App: app}
	}
	// Log-safe diagnostics only: the OAuth error code and the AADSTS number.
	code, aadsts := SafeUpstreamErrorDetails(body)
	return &upstreamTokenError{reason: fmt.Sprintf("upstream token endpoint returned status %d error=%q %s", status, code, aadsts)}
}

// SafeUpstreamErrorDetails returns the OAuth error code and AADSTS number from
// an error body, and nothing else, so the result is safe to log.
func SafeUpstreamErrorDetails(body []byte) (string, string) {
	var errPayload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &errPayload) != nil {
		return "", ""
	}
	code := errPayload.Error
	if !safeErrorCode.MatchString(code) {
		code = ""
	}
	aadsts := aadstsCode.FindString(errPayload.ErrorDescription)
	return code, aadsts
}

var (
	aadstsCode    = regexp.MustCompile(`AADSTS\d{4,6}`)
	safeErrorCode = regexp.MustCompile(`^[a-z_]{1,40}$`)
)

func entraScopeForTarget(target string) string {
	cluster, ns, app, err := routing.ParseTarget(target)
	if err == nil {
		return "api://" + cluster + "." + ns + "." + app + "/.default"
	}
	return "api://" + strings.ReplaceAll(target, ":", ".") + "/.default"
}
