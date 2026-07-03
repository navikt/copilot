package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// copilotAPIProxy forwards authenticated CLI requests to copilot-api,
// exchanging copilot-cli's workload identity for an M2M token and
// identifying the calling developer via X-On-Behalf-Of. copilot-api trusts
// this header only when the M2M token's azp matches copilot-cli's client ID
// (see apps/copilot-api auth.go resolveRequestUser).
type copilotAPIProxy struct {
	httpClient *http.Client
	baseURL    string
	texas      *texasClient
}

func newCopilotAPIProxy(baseURL string, texas *texasClient) *copilotAPIProxy {
	return &copilotAPIProxy{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		texas:      texas,
	}
}

// forward proxies the incoming request to the given copilot-api path
// (e.g. "/api/v1/copilot/usage/user/{username}") on behalf of the
// authenticated user found in the request context.
func (p *copilotAPIProxy) forward(upstreamPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			writeAuthError(w, http.StatusUnauthorized, "missing authenticated user")
			return
		}

		token, err := p.texas.token(r.Context())
		if err != nil {
			slog.Error("failed to mint M2M token for copilot-api", "error", err)
			writeAuthError(w, http.StatusBadGateway, "upstream authentication unavailable")
			return
		}

		upstreamURL := p.baseURL + upstreamPath
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL, nil)
		if err != nil {
			slog.Error("failed to build copilot-api request", "error", err)
			writeAuthError(w, http.StatusInternalServerError, "internal error")
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-On-Behalf-Of", user.Login)

		resp, err := p.httpClient.Do(req)
		if err != nil {
			slog.Error("copilot-api request failed", "error", err)
			writeAuthError(w, http.StatusBadGateway, "copilot-api unavailable")
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			slog.Warn("failed to stream copilot-api response", "error", err)
		}
	}
}

// usagePath builds the copilot-api path for a given username, mirroring the
// route documented in the PRD (issue #337).
func usagePath(username string) string {
	return fmt.Sprintf("/api/v1/copilot/usage/user/%s", username)
}
