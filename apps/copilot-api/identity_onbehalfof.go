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

// CanResolve applies only when the caller's M2M token azp is in the trusted
// set AND the request is a read-only GET. The GET constraint is a
// blast-radius limitation: copilot-cli only ever proxies a single GET
// (usage lookup), so a compromised or buggy intermediary must not be able to
// resolve an arbitrary developer's identity for a write route (e.g.
// POST/DELETE /api/v1/copilot/seats). Non-GET requests fall through the chain
// to ErrNoApplicableResolver → 401, which is the intended fail-closed
// behavior.
//
// Must be checked before any general-purpose resolver (e.g. SAML) in the
// chain, since a trusted intermediary's M2M token typically has no email
// claim to resolve via SAML in the first place.
func (o *OnBehalfOfIdentityResolver) CanResolve(user *User, r *http.Request) bool {
	return user != nil && r != nil && r.Method == http.MethodGet &&
		len(o.trustedClientIDs) > 0 && o.trustedClientIDs[user.AZP]
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
		"github_username", username,
		"intermediary_azp", azp,
		"method", r.Method,
		"path", r.URL.Path,
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
