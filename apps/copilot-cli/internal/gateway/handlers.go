package gateway

import (
	"net/http"
	"net/url"

	"github.com/navikt/copilot/apps/copilot-cli/internal/auth"
	"github.com/navikt/copilot/apps/copilot-cli/internal/httperr"
	"github.com/navikt/copilot/apps/copilot-cli/internal/upstream"
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

// makeRouter wires the health endpoints and the /api/v1/* routes nav-pilot
// calls. Their shapes never change: shipped binaries depend on them. Usage
// goes to copilot-api, surveys to copilot-survey.
//
// No CORS headers are set, on purpose: browsers never call this service, so
// a cross-origin browser request is refused by the browser itself.
func makeRouter(a *auth.Authenticator, api, surveys *upstream.Upstream) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)
	mux.Handle("/metrics", metricsHandler())

	// Method patterns answer a wrong method with 405 before auth.Middleware
	// runs, so it never costs a token check.
	mux.HandleFunc("GET /api/v1/usage", auth.Middleware(a, func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFromContext(r.Context())
		if !ok {
			httperr.Write(w, http.StatusInternalServerError, "missing authenticated user in context")
			return
		}
		api.Forward(w, r, upstream.UsagePath(user.Login), user.Login)
	}))

	mux.HandleFunc("POST /api/v1/auth/revoke", auth.RevokeHandler(a))

	// client=nav-pilot, with nav-pilot's User-Agent, lets copilot-survey
	// leave out surveys that need a newer nav-pilot (min_cli_version).
	mux.HandleFunc("GET /api/v1/surveys/active", func(w http.ResponseWriter, r *http.Request) {
		surveys.Forward(w, r, "/api/v1/surveys/active?client=nav-pilot", "")
	})
	mux.HandleFunc("GET /api/v1/surveys/schema", func(w http.ResponseWriter, r *http.Request) {
		surveys.Forward(w, r, "/api/v1/surveys/schema", "")
	})
	mux.HandleFunc("POST /api/v1/surveys/{id}/responses", auth.Middleware(a, func(w http.ResponseWriter, r *http.Request) {
		surveys.Forward(w, r, "/api/v1/surveys/"+url.PathEscape(r.PathValue("id"))+"/responses", login(r))
	}))
	// Whether the signed-in user has answered, also on the web (same hash).
	mux.HandleFunc("GET /api/v1/surveys/{id}/status", auth.Middleware(a, func(w http.ResponseWriter, r *http.Request) {
		surveys.Forward(w, r, "/api/v1/surveys/"+url.PathEscape(r.PathValue("id"))+"/status", login(r))
	}))

	return httpMetrics(mux)
}

// login is the signed-in caller's login, or "" for none, which Forward sends
// without a token.
func login(r *http.Request) string {
	if user, ok := auth.UserFromContext(r.Context()); ok {
		return user.Login
	}
	return ""
}
