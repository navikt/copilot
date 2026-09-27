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

// makeRouter wires the health endpoints, the survey definitions (public) and
// the signed-in /api/v1/* routes.
//
// No CORS headers are set, on purpose: browsers never call this service, so
// a cross-origin browser request is refused by the browser itself.
func makeRouter(auth *authenticator, proxy *copilotAPIProxy, surveys *surveyAPI) http.Handler {
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
		proxy.forward(usagePath(user.Login))(w, r)
	}))

	mux.HandleFunc("GET /api/v1/surveys/active", surveys.active)
	mux.HandleFunc("POST /api/v1/surveys/{id}/responses", authMiddleware(auth, surveys.submit))

	return mux
}
