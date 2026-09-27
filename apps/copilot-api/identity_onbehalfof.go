package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// OnBehalfOfIdentityResolver trusts the X-On-Behalf-Of header as the
// caller's verified GitHub username, for requests authenticated as one of a
// configured set of trusted intermediary services (e.g. copilot-cli). Those
// services have already validated the developer's own GitHub token and org
// membership before forwarding the request — see apps/copilot-cli.
//
// This resolver does not itself verify the requested path username matches
// the header; that comparison happens in the ownership-check step shared by
// all resolvers (see requireOwnership), since it's identical regardless of
// which resolver produced the ResolvedIdentity.
type OnBehalfOfIdentityResolver struct {
	// trustedClientIDs is the set of Entra ID client IDs (azp claim values)
	// permitted to use X-On-Behalf-Of. Every entry is a service that owns
	// its own upstream authentication (e.g. copilot-cli validates GitHub
	// device-flow tokens + navikt org membership itself).
	trustedClientIDs map[string]bool
}

// NewOnBehalfOfIdentityResolver builds a resolver that trusts X-On-Behalf-Of
// only from the given set of client IDs. Passing an empty/nil set is valid —
// CanResolve will simply never match, effectively disabling the resolver
// (used when no trusted intermediary is configured yet).
func NewOnBehalfOfIdentityResolver(trustedClientIDs map[string]bool) *OnBehalfOfIdentityResolver {
	return &OnBehalfOfIdentityResolver{trustedClientIDs: trustedClientIDs}
}

// onBehalfOfRoutes are the only routes X-On-Behalf-Of is honoured on: the
// per-user usage reads copilot-cli forwards for nav-pilot. Keyed on the
// ServeMux pattern, so it matches only in the per-route
// requireResolvedIdentity. The global IdentityMiddleware pass sees the outer
// mux's pattern, "/api/v1/", so it never resolves (and never writes the
// audit line).
var onBehalfOfRoutes = map[string]bool{
	"GET /api/v1/copilot/usage/user/{username}":               true,
	"GET /api/v1/copilot/usage/user/{username}/weekly":        true,
	"GET /api/v1/copilot/usage/user/{username}/daily-credits": true,
}

// CanResolve applies only when all of these hold:
//
//   - the token is an app token (User.isAppToken), not a user's OBO token;
//   - its azp is in the trusted set (copilot-cli);
//   - the request is a GET on a route in onBehalfOfRoutes.
//
// GET-only and the route list bound the blast radius: a compromised or buggy
// intermediary cannot resolve an arbitrary developer's identity for a write
// route (e.g. POST/DELETE /api/v1/copilot/seats) or a read it never forwards.
// Anything else falls through the chain to ErrNoApplicableResolver → 401,
// the intended fail-closed behaviour.
//
// Must be checked before any general-purpose resolver (e.g. SAML) in the
// chain, since a trusted intermediary's M2M token typically has no email
// claim to resolve via SAML in the first place.
func (o *OnBehalfOfIdentityResolver) CanResolve(user *User, r *http.Request) bool {
	return user != nil && r != nil && r.Method == http.MethodGet && onBehalfOfRoutes[r.Pattern] &&
		user.isAppToken() && len(o.trustedClientIDs) > 0 && o.trustedClientIDs[user.AZP]
}

// Resolve trusts the X-On-Behalf-Of header value as the caller's GitHub
// username. Returns ErrIdentityHeaderMissing if the header is absent, or
// ErrInvalidIdentityHeader if it isn't a well-formed GitHub username. This
// validation is defense-in-depth: even a compromised or buggy trusted
// intermediary can't inject malformed identifiers (e.g. control characters,
// path separators) into downstream systems like the budget/BigQuery clients.
//
// On success an audit line is emitted: the intermediary's M2M token carries
// no NAVident, so this is the only record attributing the resolved action to
// both the developer (GitHub username) and the intermediary (azp).
func (o *OnBehalfOfIdentityResolver) Resolve(ctx context.Context, user *User, r *http.Request) (*ResolvedIdentity, error) {
	username := strings.TrimSpace(r.Header.Get("X-On-Behalf-Of"))
	if username == "" {
		return nil, ErrIdentityHeaderMissing
	}
	if !isValidGitHubUsername(username) {
		return nil, ErrInvalidIdentityHeader
	}

	var azp string
	if user != nil {
		azp = user.AZP
	}
	slog.InfoContext(ctx, "resolved identity via trusted intermediary (X-On-Behalf-Of)",
		"github_username", logSafe(username),
		"intermediary_azp", azp,
		"method", logSafe(r.Method),
		"path", logSafe(r.URL.Path),
	)

	return &ResolvedIdentity{GitHubUsername: username, Source: "on-behalf-of"}, nil
}

// trustedClientIDForApp extracts the Entra ID client ID of a pre-authorized
// inbound app by its NAIS application name from the raw
// AZURE_APP_PRE_AUTHORIZED_APPS JSON that NAIS injects (auto-populated from
// accessPolicy.inbound.rules). Entries look like:
//
//	[{"name":"dev-gcp:copilot:copilot-cli","clientId":"<uuid>"}]
//
// The name is <cluster>:<namespace>:<app>; matching is on the final
// :-separated segment (the app name) so the caller needn't know the cluster
// or namespace. Fails closed on anything ambiguous: empty input yields
// ("", nil); malformed JSON yields ("", error); more than one entry matching
// appName yields ("", nil) with a warning logged — granting X-On-Behalf-Of
// trust to the wrong client must never happen by accident.
func trustedClientIDForApp(preAuthorizedApps, appName string) (string, error) {
	if strings.TrimSpace(preAuthorizedApps) == "" {
		return "", nil
	}

	var apps []struct {
		Name     string `json:"name"`
		ClientID string `json:"clientId"`
	}
	if err := json.Unmarshal([]byte(preAuthorizedApps), &apps); err != nil {
		return "", fmt.Errorf("parsing AZURE_APP_PRE_AUTHORIZED_APPS: %w", err)
	}

	var matches []string
	for _, app := range apps {
		segments := strings.Split(app.Name, ":")
		if segments[len(segments)-1] == appName && app.ClientID != "" {
			matches = append(matches, app.ClientID)
		}
	}

	switch len(matches) {
	case 0:
		return "", nil
	case 1:
		return matches[0], nil
	default:
		slog.Warn("multiple pre-authorized apps match name — refusing X-On-Behalf-Of trust (ambiguous)",
			"app", appName, "match_count", len(matches))
		return "", nil
	}
}
