package main

import (
	"net/http"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// makeRouter wires up the public health endpoints and the authenticated
// /api/v1/* CLI-facing routes. Every /api/v1/* route is wrapped in
// authMiddleware, which validates the caller's GitHub token and enforces
// org membership before the request reaches the copilot-api proxy.
func makeRouter(cfg *Config, gh *GitHubClient, cache *orgMembershipCache, proxy *copilotAPIProxy) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)
	mux.Handle("/metrics", metricsHandler())

	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return authMiddleware(gh, cache, cfg.GitHubOrg, h)
	}

	// requireGET rejects non-GET requests before authMiddleware runs, so a
	// disallowed method gets 405 (not 401) and never triggers token
	// validation or GitHub API calls.
	requireGET := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			h(w, r)
		}
	}

	mux.HandleFunc("/api/v1/usage", requireGET(auth(func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			writeAuthError(w, http.StatusInternalServerError, "missing authenticated user in context")
			return
		}
		proxy.forward(usagePath(user.Login))(w, r)
	})))

	return mux
}
