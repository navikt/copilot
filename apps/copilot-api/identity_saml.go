package main

import (
	"context"
	"fmt"
	"net/http"
)

// samlUsernameLookup abstracts just the one GitHub operation SAML identity
// resolution needs. This is deliberately narrower than the full GitHubAPI
// interface (interface segregation) — SAMLIdentityResolver has no business
// depending on billing/seat-management methods it never calls.
type samlUsernameLookup interface {
	getUsernameBySamlIdentity(ctx context.Context, identity string) (string, error)
}

// SAMLIdentityResolver resolves an Azure AD-authenticated caller's GitHub
// username via GitHub's SAML/SCIM identity mapping. This is the mechanism
// used by my-copilot (the web UI) and any other Azure AD-authenticated
// caller that isn't a trusted intermediary.
type SAMLIdentityResolver struct {
	githubClient samlUsernameLookup
}

// NewSAMLIdentityResolver builds a resolver backed by the given GitHub
// client. githubClient must not be nil — callers should only register this
// resolver in the chain when a GitHub client is actually available.
func NewSAMLIdentityResolver(githubClient samlUsernameLookup) *SAMLIdentityResolver {
	return &SAMLIdentityResolver{githubClient: githubClient}
}

// CanResolve applies whenever the caller has an Azure AD email to resolve
// via SAML — i.e. any request that isn't handled by a more specific resolver
// (such as OnBehalfOfIdentityResolver) earlier in the chain.
func (s *SAMLIdentityResolver) CanResolve(user *User, _ *http.Request) bool {
	return user != nil && user.Email != ""
}

// Resolve looks up the caller's GitHub username via SAML/SCIM.
func (s *SAMLIdentityResolver) Resolve(ctx context.Context, user *User, _ *http.Request) (*ResolvedIdentity, error) {
	username, err := s.githubClient.getUsernameBySamlIdentity(ctx, user.Email)
	if err != nil {
		return nil, fmt.Errorf("SAML identity resolution failed: %w", err)
	}
	if username == "" {
		return nil, ErrNoGitHubAccount
	}
	return &ResolvedIdentity{GitHubUsername: username, Source: "saml"}, nil
}
