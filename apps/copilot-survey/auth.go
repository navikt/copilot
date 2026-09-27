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
	"regexp"
	"slices"
	"strings"
	"time"
)

// caller is who a submission comes from. From copilot-cli: a nav-pilot user's
// GitHub login, asserted in X-On-Behalf-Of. From my-copilot: a ki-utvikling
// user's Nav e-mail, from their own OBO token. Never logged.
type caller struct {
	login string
	email string
}

// authenticator validates the Entra token on a submission through the Texas
// sidecar's introspection endpoint (signature, issuer, audience, expiry), then
// takes exactly one of two branches:
//
//   - App token from copilot-cli: idtyp app or the access_as_application
//     role, no NAVident and no e-mail, azp copilot-cli. The user is the
//     well-formed GitHub login in X-On-Behalf-Of.
//   - User token from my-copilot: NAVident and preferred_username set, idtyp
//     not app, azp my-copilot. X-On-Behalf-Of is refused.
//
// Anything else is refused. The middleware wraps the submit route only, so
// the header is honoured on POST /api/v1/surveys/{id}/responses and nowhere
// else.
type authenticator struct {
	httpClient *http.Client
	endpoint   string
	cliID      string
	webID      string
}

func newAuthenticator(endpoint, preAuthorizedApps string) *authenticator {
	return &authenticator{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		endpoint:   endpoint,
		cliID:      clientIDForApp(preAuthorizedApps, "copilot-cli"),
		webID:      clientIDForApp(preAuthorizedApps, "my-copilot"),
	}
}

var (
	errUnauthorized = errors.New("invalid or expired token")
	errForbidden    = errors.New("this caller may not submit answers")
	errBadHeader    = errors.New("X-On-Behalf-Of is missing, malformed, or sent with a user token")
)

var githubLogin = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$`)

func (a *authenticator) resolve(ctx context.Context, token, onBehalfOf string) (*caller, error) {
	if a.endpoint == "" {
		return nil, errUnauthorized
	}
	body, err := json.Marshal(map[string]string{"identity_provider": "azuread", "token": token})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling texas introspection: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("texas introspection returned status %d", resp.StatusCode)
	}
	var c struct {
		Active   bool     `json:"active"`
		Azp      string   `json:"azp"`
		Idtyp    string   `json:"idtyp"`
		Roles    []string `json:"roles"`
		NAVident string   `json:"NAVident"`
		Email    string   `json:"preferred_username"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&c); err != nil {
		return nil, fmt.Errorf("decoding introspection response: %w", err)
	}
	headerSent := onBehalfOf != ""
	onBehalfOf = strings.TrimSpace(onBehalfOf)
	switch {
	case !c.Active:
		return nil, errUnauthorized
	case (c.Idtyp == "app" || slices.Contains(c.Roles, "access_as_application")) && c.NAVident == "" && c.Email == "":
		if a.cliID == "" || c.Azp != a.cliID {
			return nil, errForbidden
		}
		if !githubLogin.MatchString(onBehalfOf) {
			return nil, errBadHeader
		}
		return &caller{login: onBehalfOf}, nil
	case c.NAVident != "" && c.Email != "" && c.Idtyp != "app":
		if headerSent {
			return nil, errBadHeader
		}
		if a.webID == "" || c.Azp != a.webID {
			return nil, errForbidden
		}
		return &caller{email: c.Email}, nil
	}
	return nil, errForbidden
}

type contextKey struct{}

// authMiddleware puts the caller in the request context. Fail closed: any
// error rejects the request. Neither the token nor the caller is logged.
func authMiddleware(a *authenticator, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fields := strings.Fields(r.Header.Get("Authorization"))
		if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") || len(fields[1]) > 8192 {
			writeError(w, http.StatusUnauthorized, "missing or malformed bearer token")
			return
		}
		c, err := a.resolve(r.Context(), fields[1], r.Header.Get("X-On-Behalf-Of"))
		switch {
		case err == nil:
			next(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, c)))
		// 401 for every refusal of the calling app: 403 is kept for "no Nav
		// identity", which is about the person, so clients can tell the two
		// apart.
		case errors.Is(err, errUnauthorized), errors.Is(err, errBadHeader), errors.Is(err, errForbidden):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			slog.Warn("token validation failed upstream", "error", err)
			writeError(w, http.StatusBadGateway, "could not validate token")
		}
	}
}

func callerFromContext(ctx context.Context) (*caller, bool) {
	c, ok := ctx.Value(contextKey{}).(*caller)
	return c, ok
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, message)
}
