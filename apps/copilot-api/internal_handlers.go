package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// samlNameIDHandler serves POST /internal/v1/saml/name-id: a GitHub login in
// the body, the Nav e-mail (the nameId of the login's SAML SSO identity in
// navikt) out. It exists so the GitHub App key stays in this pod: the only
// caller is copilot-survey, which needs the e-mail for its dedup hash when an
// answer comes from nav-pilot.
//
// Outside the /api/v1/ chain on purpose: no identity resolver, so no audit
// line; no request log or trace; the login travels in the body, so the path
// names no one. Only an app token whose azp is copilot-survey's client id
// gets through; everyone else gets 403, including every user token.
func samlNameIDHandler(surveyClientID string, lookup func(context.Context, string) (string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil || !isValidGitHubUsername(body.Login) {
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
