package main

import (
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
		name:       name,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		texas:      texas,
	}
}

// forward proxies r to path on the upstream. A request with no signed-in
// user (the public survey definitions) goes without a token or header.
func (p *upstream) forward(w http.ResponseWriter, r *http.Request, path string) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, p.baseURL+path, http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		slog.Error("failed to build upstream request", "upstream", p.name, "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
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
	if err != nil {
		slog.Error("upstream request failed", "upstream", p.name, "error_type", fmt.Sprintf("%T", err))
		writeError(w, http.StatusBadGateway, p.name+" unavailable")
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	if cc := resp.Header.Get("Cache-Control"); cc != "" {
		w.Header().Set("Cache-Control", cc)
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
