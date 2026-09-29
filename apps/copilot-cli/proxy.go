package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// upstream forwards CLI requests to a service in the cluster (copilot-api,
// copilot-survey) with copilot-cli's own M2M token for that service and, for
// a signed-in caller, the verified GitHub login in X-On-Behalf-Of. Each
// downstream decides which of its routes honour the header (see
// OnBehalfOfIdentityResolver in apps/copilot-api and auth.go in
// apps/copilot-survey).
//
// Method, request body and Content-Type go through; status, Content-Type,
// Cache-Control and body come back as they are. No retry: a retry would
// have to buffer the body, and survey bodies must not linger anywhere.
type upstream struct {
	name       string
	httpClient *http.Client
	baseURL    string
	texas      *texasClient
}

func newUpstream(name, baseURL string, texas *texasClient) *upstream {
	return &upstream{
		name: name,
		// No redirects: a 30x comes back as it is, never a second request.
		httpClient: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		baseURL: strings.TrimSuffix(baseURL, "/"),
		texas:   texas,
	}
}

// forward proxies r to path on the upstream. A request with no signed-in
// user (the public survey definitions) goes without a token or header.
func (p *upstream) forward(w http.ResponseWriter, r *http.Request, path string) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, p.baseURL+path, http.MaxBytesReader(w, r.Body, 32<<10))
	if err != nil {
		slog.Error("failed to build upstream request", "upstream", p.name, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	req.Header.Set("User-Agent", r.Header.Get("User-Agent"))
	if user, ok := userFromContext(r.Context()); ok {
		token, tokenErr := p.texas.token(r.Context())
		if tokenErr != nil {
			slog.Error("failed to mint M2M token", "upstream", p.name, "error", tokenErr)
			writeError(w, http.StatusBadGateway, "upstream authentication unavailable")
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-On-Behalf-Of", user.Login)
	}

	resp, err := p.httpClient.Do(req)
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		writeError(w, http.StatusRequestEntityTooLarge, "body is larger than 32 KiB")
		return
	}
	if err != nil {
		slog.Error("upstream request failed", "upstream", p.name, "error_type", fmt.Sprintf("%T", err))
		writeError(w, http.StatusBadGateway, p.name+" unavailable")
		return
	}
	defer resp.Body.Close()

	// A 401 from the service is about copilot-cli's own token, never the
	// caller's: the caller's GitHub sign-in was checked here. Passing it on
	// would tell nav-pilot to sign in again and drop a pending answer.
	if resp.StatusCode == http.StatusUnauthorized && req.Header.Get("Authorization") != "" {
		slog.Error("upstream refused copilot-cli's token", "upstream", p.name)
		writeError(w, http.StatusBadGateway, p.name+" refused the gateway")
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

// usagePath builds the copilot-api path for a given username, mirroring the
// route documented in the PRD (issue #337).
func usagePath(username string) string {
	return fmt.Sprintf("/api/v1/copilot/usage/user/%s", username)
}
