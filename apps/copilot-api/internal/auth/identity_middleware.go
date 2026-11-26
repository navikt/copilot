package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

const ResolvedIdentityContextKey contextKey = "resolved_identity"

// IdentityMiddleware resolves "who is this caller, as a GitHub username?"
// once per request (via the given chain) and stores the result in the
// request context, so downstream handlers never need to know which auth
// mechanism (SAML, X-On-Behalf-Of, ...) produced it — see
// GetResolvedIdentity and RequireOwnership.
//
// If required is false, resolution failures are logged but the request
// proceeds without a ResolvedIdentity in context (for endpoints that don't
// need per-user ownership checks, e.g. team/org-level aggregates). If
// required is true, resolution failures short-circuit with an appropriate
// error response.
//
// This middleware must run after the authentication middleware
// (Middleware), since it depends on *User already being present in
// the request context.
func IdentityMiddleware(chain *IdentityResolverChain, required bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok || user == nil {
				if required {
					problem.Respond(w, "unauthorized", "Authentication required", http.StatusUnauthorized)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// A user's own token never asserts someone else: refuse the
			// header rather than ignore it.
			if r.Header.Get("X-On-Behalf-Of") != "" && !user.IsAppToken() {
				problem.Respond(w, "unauthorized", "X-On-Behalf-Of is not accepted on a user token", http.StatusUnauthorized)
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

			ctx := context.WithValue(r.Context(), ResolvedIdentityContextKey, identity)
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
		problem.Respond(w, "unauthorized", "Caller identity could not be determined", http.StatusUnauthorized)
	case errors.Is(err, ErrNoGitHubAccount):
		problem.Respond(w, "no_github_account", "No GitHub account linked to your identity", http.StatusForbidden)
	case errors.Is(err, ErrIdentityHeaderMissing):
		problem.Respond(w, "unauthorized", "Missing required identity header", http.StatusUnauthorized)
	case errors.Is(err, ErrInvalidIdentityHeader):
		problem.Respond(w, "unauthorized", "Invalid identity header", http.StatusUnauthorized)
	default:
		slog.Error("Identity resolution failed", "error", err)
		problem.Respond(w, "identity_check_failed", "Failed to verify user identity", http.StatusInternalServerError)
	}
}

// GetResolvedIdentity extracts the ResolvedIdentity placed in context by
// IdentityMiddleware. Returns false if no identity was resolved (e.g. on a
// route where IdentityMiddleware was configured with required=false and
// resolution failed or was skipped).
func GetResolvedIdentity(ctx context.Context) (*ResolvedIdentity, bool) {
	identity, ok := ctx.Value(ResolvedIdentityContextKey).(*ResolvedIdentity)
	return identity, ok && identity != nil
}

// RequireResolvedIdentity wraps a per-user handler so that identity
// resolution is mandatory for this specific route, regardless of how
// IdentityMiddleware was configured globally. This makes it structurally
// impossible to add a new per-user route that forgets to call
// RequireOwnership/GetResolvedIdentity: if resolution fails or was skipped
// upstream, the request is rejected here before the handler ever runs.
func RequireResolvedIdentity(chain *IdentityResolverChain, next http.HandlerFunc) http.HandlerFunc {
	wrapped := IdentityMiddleware(chain, true)(next)
	return wrapped.ServeHTTP
}

// RequireOwnership verifies that the resolved caller identity matches the
// requested username, regardless of which IdentityResolver produced it. This
// is the single ownership-check call site every per-user handler should use
// once IdentityMiddleware is wired in front of it (see Phase 3 cutover) —
// handlers stay entirely unaware of SAML vs. on-behalf-of vs. any future
// mechanism.
//
// Writes an appropriate error response and returns false on failure; callers
// should return immediately when this returns false.
func RequireOwnership(w http.ResponseWriter, r *http.Request, requestedUsername string) bool {
	identity, ok := GetResolvedIdentity(r.Context())
	if !ok {
		problem.Respond(w, "unauthorized", "Caller identity could not be determined", http.StatusUnauthorized)
		return false
	}
	if !strings.EqualFold(identity.GitHubUsername, requestedUsername) {
		slog.Warn("Per-user read denied: identity mismatch",
			"requested_username", logSafe(requestedUsername),
			"resolved_source", identity.Source,
		)
		problem.Respond(w, "forbidden", "You can only view your own usage data", http.StatusForbidden)
		return false
	}
	return true
}

// logSafe strips line breaks from a request-derived value before it is
// logged, so it cannot forge a log line.
func logSafe(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", ""), "\r", "")
}

// RedactPath replaces the path segments that name a person, so a request log
// or a span never says who was active when: everything after /saml/ (the
// caller's Nav e-mail), and the GitHub username in /usage/user/{username}
// and /seats/{username}. Case-insensitive, so /SAML/<e-mail> (a 404, but
// logged first) is caught too.
func RedactPath(p string) string {
	// Slice the lowered string: ToLower can change a rune's byte length.
	low := strings.ToLower(p)
	if i := strings.Index(low, "/saml/"); i >= 0 {
		return low[:i] + "/saml/{identity}"
	}
	for _, m := range []string{"/usage/user/", "/seats/"} {
		if i := strings.Index(low, m); i >= 0 {
			// TrimLeft: %2F before the name decodes to a leading slash.
			_, tail, _ := strings.Cut(strings.TrimLeft(low[i+len(m):], "/"), "/")
			if tail != "" {
				tail = "/" + tail
			}
			return low[:i] + m + "{username}" + tail
		}
	}
	return p
}
