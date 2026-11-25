// Package upstream forwards nav-pilot requests to copilot-api and copilot-survey.
package upstream

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
	"strings"
	"time"

	"github.com/navikt/copilot/apps/copilot-cli/internal/httperr"
	"github.com/navikt/copilot/apps/copilot-cli/internal/texas"
)

// Upstream forwards CLI requests to a service in the cluster (copilot-api,
// copilot-survey) with copilot-cli's own M2M token for that service and, for
// a signed-in caller, the verified GitHub login in X-On-Behalf-Of. Each
// downstream decides which of its routes honour the header (see
// OnBehalfOfIdentityResolver in apps/copilot-api and auth.go in
// apps/copilot-survey).
//
// Method, request body and Content-Type go through; status, Content-Type,
// Cache-Control and body come back as they are. No retry: a retry would
// have to buffer the body, and survey bodies must not linger anywhere.
type Upstream struct {
	name       string
	httpClient *http.Client
	baseURL    string
	texas      *texas.Client
}

// New returns an Upstream for baseURL that authenticates with tc.
func New(name, baseURL string, tc *texas.Client) *Upstream {
	return &Upstream{
		name: name,
		// No redirects: a 30x comes back as it is, never a second request.
		httpClient: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		baseURL: strings.TrimSuffix(baseURL, "/"),
		texas:   tc,
	}
}

// Forward proxies r to path on the upstream on behalf of login. An empty
// login (no signed-in user, the public survey definitions) goes without a
// token or header.
func (p *Upstream) Forward(w http.ResponseWriter, r *http.Request, path, login string) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, p.baseURL+path, http.MaxBytesReader(w, r.Body, 32<<10))
	if err != nil {
		slog.Error("failed to build upstream request", "upstream", p.name, "error", err)
		httperr.Write(w, http.StatusInternalServerError, "internal error")
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	req.Header.Set("User-Agent", r.Header.Get("User-Agent"))
	if login != "" {
		token, tokenErr := p.texas.Token(r.Context())
		if tokenErr != nil {
			slog.Error("failed to mint M2M token", "upstream", p.name, "error", tokenErr)
			httperr.Write(w, http.StatusBadGateway, "upstream authentication unavailable")
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-On-Behalf-Of", login)
	}

	resp, err := p.httpClient.Do(req)
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		httperr.Write(w, http.StatusRequestEntityTooLarge, "body is larger than 32 KiB")
		return
	}
	if err != nil {
		slog.Error("upstream request failed", "upstream", p.name, "error_type", fmt.Sprintf("%T", err))
		httperr.Write(w, http.StatusBadGateway, p.name+" unavailable")
		return
	}
	defer resp.Body.Close()

	// A 401 from the service is about copilot-cli's own token, never the
	// caller's: the caller's GitHub sign-in was checked here. Passing it on
	// would tell nav-pilot to sign in again and drop a pending answer.
	if resp.StatusCode == http.StatusUnauthorized && req.Header.Get("Authorization") != "" {
		slog.Error("upstream refused copilot-cli's token", "upstream", p.name)
		httperr.Write(w, http.StatusBadGateway, p.name+" refused the gateway")
		return
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "" {
		w.Header().Set("Cache-Control", cc)
	}
	if v := resp.Header.Get("Vary"); v != "" {
		w.Header().Set("Vary", v)
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		slog.Warn("failed to stream upstream response", "upstream", p.name, "error", err)
		// Best-effort drain so the connection can be reused.
		_, _ = io.Copy(io.Discard, resp.Body)
	}
}

// UsagePath builds the copilot-api path for a given username, mirroring the
// route documented in the PRD (issue #337).
func UsagePath(username string) string {
	return fmt.Sprintf("/api/v1/copilot/usage/user/%s", url.PathEscape(username))
}

// IsOrgMember asks copilot-api whether login is an active member of the org,
// with copilot-cli's M2M token. copilot-api answers with its own GitHub App,
// so the nav-pilot user token needs no permissions. Anything but a 200 with
// a boolean "active" is an error, and the caller refuses the sign-in.
func (p *Upstream) IsOrgMember(ctx context.Context, login string) (bool, error) {
	token, err := p.texas.Token(ctx)
	if err != nil {
		return false, fmt.Errorf("minting M2M token: %w", err)
	}
	body, err := json.Marshal(map[string]string{"login": login})
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/internal/v1/github/org-membership", bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("calling %s membership: %T", p.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%s membership returned status %d", p.name, resp.StatusCode)
	}
	var out struct {
		Active *bool `json:"active"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<10))
	if err := dec.Decode(&out); err != nil || out.Active == nil {
		return false, fmt.Errorf("%s membership: malformed answer", p.name)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return false, fmt.Errorf("%s membership: malformed answer", p.name)
	}
	return *out.Active, nil
}
