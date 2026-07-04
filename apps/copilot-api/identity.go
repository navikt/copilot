package main

import (
	"context"
	"errors"
	"net/http"
)

// ResolvedIdentity is the single source of truth for "who is the caller, as
// a GitHub username?" — computed once per request by the IdentityResolverChain
// and consumed by handlers via GetResolvedIdentity, without any handler
// needing to know which underlying mechanism (SAML, X-On-Behalf-Of, ...)
// produced it.
type ResolvedIdentity struct {
	// GitHubUsername is the verified GitHub login for the authenticated caller.
	GitHubUsername string
	// Source identifies which IdentityResolver produced this identity, for
	// logging/auditing (e.g. "saml", "on-behalf-of").
	Source string
}

// Sentinel errors returned by IdentityResolver implementations. Handlers (or
// the identity middleware) map these to HTTP status codes without needing to
// know which resolver produced them.
var (
	// ErrNoApplicableResolver means no configured resolver's CanResolve
	// matched the request — the caller's auth mechanism isn't recognized.
	ErrNoApplicableResolver = errors.New("no applicable identity resolver for this request")
	// ErrNoGitHubAccount means the caller is authenticated but has no linked
	// GitHub account to resolve (e.g. SAML lookup returned empty).
	ErrNoGitHubAccount = errors.New("no GitHub account linked to caller identity")
	// ErrIdentityHeaderMissing means a trusted-intermediary resolver expected
	// an identity header (e.g. X-On-Behalf-Of) that wasn't present.
	ErrIdentityHeaderMissing = errors.New("required identity header missing")
)

// IdentityResolver resolves the authenticated caller's GitHub username using
// one specific mechanism (SAML lookup, trusted-intermediary header, etc.).
// Each auth mechanism gets its own implementation — adding a new mechanism
// means adding a new IdentityResolver, never modifying existing handlers.
type IdentityResolver interface {
	// CanResolve reports whether this resolver applies to the given
	// authenticated user/request. Must be cheap (no network calls) — it's
	// used purely to pick which resolver's Resolve to call.
	CanResolve(user *User, r *http.Request) bool

	// Resolve returns the caller's verified GitHub username. Only called
	// when CanResolve returned true for the same (user, r) pair.
	Resolve(ctx context.Context, user *User, r *http.Request) (*ResolvedIdentity, error)
}

// IdentityResolverChain tries each configured IdentityResolver in order and
// delegates to the first one whose CanResolve matches. Order matters: more
// specific/trusted resolvers should be registered before general-purpose ones
// (see main.go wiring — the copilot-cli on-behalf-of resolver is registered
// before the SAML resolver).
type IdentityResolverChain struct {
	resolvers []IdentityResolver
}

// NewIdentityResolverChain builds a chain from the given resolvers, tried in
// the order provided.
func NewIdentityResolverChain(resolvers ...IdentityResolver) *IdentityResolverChain {
	return &IdentityResolverChain{resolvers: resolvers}
}

// Resolve finds the first applicable resolver for (user, r) and returns its
// result. Returns ErrNoApplicableResolver if none match.
func (c *IdentityResolverChain) Resolve(ctx context.Context, user *User, r *http.Request) (*ResolvedIdentity, error) {
	for _, resolver := range c.resolvers {
		if resolver.CanResolve(user, r) {
			return resolver.Resolve(ctx, user, r)
		}
	}
	return nil, ErrNoApplicableResolver
}
