package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
)

const samlNameIDPath = "/internal/v1/saml/name-id"

// samlNameIDRequests counts the handler's outcomes on the name-id route by
// status: calls that passed the token check and the method match. 401 and
// 405 are answered before the handler and are not counted. Volume and status
// carry no personal data and are the one tripwire on a route that otherwise
// leaves no record: a burst of 200/404 means copilot-survey is enumerating.
var samlNameIDRequests = map[int]*atomic.Int64{
	http.StatusOK: {}, http.StatusBadRequest: {}, http.StatusForbidden: {},
	http.StatusNotFound: {}, http.StatusServiceUnavailable: {},
}

// samlNameIDMetrics is the counter in Prometheus text format.
func samlNameIDMetrics() string {
	var b strings.Builder
	b.WriteString("\n# HELP copilot_api_saml_name_id_requests_total Authenticated calls to POST /internal/v1/saml/name-id by handler status\n")
	b.WriteString("# TYPE copilot_api_saml_name_id_requests_total counter\n")
	for _, status := range []int{200, 400, 403, 404, 503} {
		fmt.Fprintf(&b, "copilot_api_saml_name_id_requests_total{status=\"%d\"} %d\n", status, samlNameIDRequests[status].Load())
	}
	return b.String()
}

// registerInternalRoutes mounts the name-id route on the root mux, behind
// the token check but outside /api/v1/. Off (403) unless copilot-survey is
// a pre-authorized app.
func registerInternalRoutes(mux *http.ServeMux, auth func(http.Handler) http.Handler, surveyClientID string, lookup func(context.Context, string) (string, error)) {
	if surveyClientID != "" {
		slog.Info("copilot-survey trusted for POST /internal/v1/saml/name-id", "client_id", surveyClientID)
	}
	mux.Handle("POST "+samlNameIDPath, auth(samlNameIDHandler(surveyClientID, lookup)))
}

// samlNameIDHandler serves POST /internal/v1/saml/name-id. It takes a GitHub
// login in the body and returns the Nav e-mail, the nameId of the login's
// SAML SSO identity in navikt. The only caller is copilot-survey, which needs
// the e-mail for its dedup hash when an answer comes from nav-pilot. Doing
// the lookup here keeps the GitHub App key in this pod.
//
// The route is outside the /api/v1/ chain on purpose. There is no identity
// resolver and so no audit line, no request log and no trace, and the login
// travels in the body, so the path names no one. Only an app token whose azp
// is copilot-survey's client id gets through. Every other valid token gets
// 403, including every user token; a missing or invalid one gets 401.
func samlNameIDHandler(surveyClientID string, lookup func(context.Context, string) (string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limited := http.MaxBytesReader(w, r.Body, 1<<10)
		rec := &statusCounter{ResponseWriter: w}
		defer func() {
			if c := samlNameIDRequests[rec.status]; c != nil {
				c.Add(1)
			}
		}()
		w = rec
		user, ok := getUserFromContext(r.Context())
		if !ok || surveyClientID == "" || !user.isAppToken() || user.AZP != surveyClientID {
			respondError(w, "forbidden", "Only copilot-survey may call this route", http.StatusForbidden)
			return
		}
		if lookup == nil {
			respondError(w, "service_unavailable", "GitHub is not configured for this environment", http.StatusServiceUnavailable)
			return
		}
		var body struct {
			Login string `json:"login"`
		}
		dec := json.NewDecoder(limited)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil || !isValidGitHubUsername(body.Login) || !errors.Is(dec.Decode(&struct{}{}), io.EOF) {
			respondError(w, "invalid_parameter", `The body must be {"login": "<GitHub login>"}`, http.StatusBadRequest)
			return
		}
		nameID, err := lookup(r.Context(), body.Login)
		switch {
		case errors.Is(err, errNoSAMLIdentity):
			respondError(w, "no_saml_identity", "No SAML identity is linked to this GitHub account", http.StatusNotFound)
		case err != nil:
			// The lookup's errors carry status codes, never the login.
			slog.Error("SAML name-id lookup failed", "error", err)
			respondError(w, "github_error", "Could not look up the SAML identity", http.StatusServiceUnavailable)
		default:
			noCacheControl(w)
			respondJSON(w, map[string]string{"name_id": nameID}, http.StatusOK)
		}
	}
}

type statusCounter struct {
	http.ResponseWriter
	status int
}

func (s *statusCounter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
