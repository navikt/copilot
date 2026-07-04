package main

import (
	"context"
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
// set. Must be checked before any general-purpose resolver (e.g. SAML) in
// the chain, since a trusted intermediary's M2M token typically has no email
// claim to resolve via SAML in the first place.
func (o *OnBehalfOfIdentityResolver) CanResolve(user *User, _ *http.Request) bool {
	return user != nil && len(o.trustedClientIDs) > 0 && o.trustedClientIDs[user.AZP]
}

// Resolve trusts the X-On-Behalf-Of header value as the caller's GitHub
// username. Returns ErrIdentityHeaderMissing if the header is absent.
func (o *OnBehalfOfIdentityResolver) Resolve(_ context.Context, _ *User, r *http.Request) (*ResolvedIdentity, error) {
	username := strings.TrimSpace(r.Header.Get("X-On-Behalf-Of"))
	if username == "" {
		return nil, ErrIdentityHeaderMissing
	}
	return &ResolvedIdentity{GitHubUsername: username, Source: "on-behalf-of"}, nil
}
