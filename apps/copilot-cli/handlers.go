package main

import (
	"net/http"
	"net/url"
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
func makeRouter(auth *authenticator, api, surveys *upstream) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)
	mux.Handle("/metrics", metricsHandler())

	// Method patterns answer a wrong method with 405 before authMiddleware
	// runs, so it never costs a token check.
	mux.HandleFunc("GET /api/v1/usage", authMiddleware(auth, func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusInternalServerError, "missing authenticated user in context")
			return
		}
		api.forward(w, r, usagePath(user.Login))
	}))

	for _, path := range []string{"/api/v1/surveys/active", "/api/v1/surveys/schema"} {
		mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) { surveys.forward(w, r, path) })
	}
	mux.HandleFunc("POST /api/v1/surveys/{id}/responses", authMiddleware(auth, func(w http.ResponseWriter, r *http.Request) {
		surveys.forward(w, r, "/api/v1/surveys/"+url.PathEscape(r.PathValue("id"))+"/responses")
	}))

	return mux
}
