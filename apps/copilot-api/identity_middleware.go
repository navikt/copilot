package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

const resolvedIdentityContextKey contextKey = "resolved_identity"

// IdentityMiddleware resolves "who is this caller, as a GitHub username?"
// once per request (via the given chain) and stores the result in the
// request context, so downstream handlers never need to know which auth
// mechanism (SAML, X-On-Behalf-Of, ...) produced it — see
// GetResolvedIdentity and requireOwnership.
//
// If required is false, resolution failures are logged but the request
// proceeds without a ResolvedIdentity in context (for endpoints that don't
// need per-user ownership checks, e.g. team/org-level aggregates). If
// required is true, resolution failures short-circuit with an appropriate
// error response.
//
// This middleware must run after the authentication middleware
// (makeAuthMiddleware), since it depends on *User already being present in
// the request context.
func IdentityMiddleware(chain *IdentityResolverChain, required bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := getUserFromContext(r.Context())
			if !ok || user == nil {
				if required {
					respondError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			identity, err := chain.Resolve(r.Context(), user, r)
			if err != nil {
				if required {
					writeIdentityResolutionError(w, err)
					return
				}
				slog.Debug("Identity resolution skipped (not required for this route)", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), resolvedIdentityContextKey, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeIdentityResolutionError maps IdentityResolver sentinel errors to HTTP
// responses. Falls back to a generic 500 for unexpected errors (e.g. a SAML
// lookup that failed due to a GitHub API outage) rather than leaking
// internal error details to the client.
func writeIdentityResolutionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoApplicableResolver):
		respondError(w, "unauthorized", "Caller identity could not be determined", http.StatusUnauthorized)
	case errors.Is(err, ErrNoGitHubAccount):
		respondError(w, "no_github_account", "No GitHub account linked to your identity", http.StatusForbidden)
	case errors.Is(err, ErrIdentityHeaderMissing):
		respondError(w, "unauthorized", "Missing required identity header", http.StatusUnauthorized)
	default:
		slog.Error("Identity resolution failed", "error", err)
		respondError(w, "identity_check_failed", "Failed to verify user identity", http.StatusInternalServerError)
	}
}

// GetResolvedIdentity extracts the ResolvedIdentity placed in context by
// IdentityMiddleware. Returns false if no identity was resolved (e.g. on a
// route where IdentityMiddleware was configured with required=false and
// resolution failed or was skipped).
func GetResolvedIdentity(ctx context.Context) (*ResolvedIdentity, bool) {
	identity, ok := ctx.Value(resolvedIdentityContextKey).(*ResolvedIdentity)
	return identity, ok && identity != nil
}

// requireOwnership verifies that the resolved caller identity matches the
// requested username, regardless of which IdentityResolver produced it. This
// is the single ownership-check call site every per-user handler should use
// once IdentityMiddleware is wired in front of it (see Phase 3 cutover) —
// handlers stay entirely unaware of SAML vs. on-behalf-of vs. any future
// mechanism.
//
// Writes an appropriate error response and returns false on failure; callers
// should return immediately when this returns false.
func requireOwnership(w http.ResponseWriter, r *http.Request, requestedUsername string) bool {
	identity, ok := GetResolvedIdentity(r.Context())
	if !ok {
		respondError(w, "unauthorized", "Caller identity could not be determined", http.StatusUnauthorized)
		return false
	}
	if !strings.EqualFold(identity.GitHubUsername, requestedUsername) {
		slog.Warn("Per-user read denied: identity mismatch",
			"requested_username", requestedUsername,
			"resolved_source", identity.Source,
		)
		respondError(w, "forbidden", "You can only view your own usage data", http.StatusForbidden)
		return false
	}
	return true
}
