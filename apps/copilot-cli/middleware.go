package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type contextKey string

const requestUserContextKey contextKey = "copilot-cli-user"

const (
	issuerGitHub = "github"
	issuerEntra  = "entra"
)

// AuthenticatedUser is the caller, normalised across the two sign-in paths:
// nav-pilot with a GitHub token, and ki-utvikling (my-copilot) with an
// Entra ID OBO token. Subject is the issuer's stable id (GitHub's numeric
// user id, Entra's oid), never an e-mail or a name. Login is GitHub only.
type AuthenticatedUser struct {
	Issuer  string
	Subject string
	Login   string

	// email is the Nav e-mail from an Entra token; empty for GitHub (see
	// GitHubClient.navEmail). Unexported and never logged.
	email     string
	expiresAt time.Time
}

// authenticator resolves a bearer token to a user through the issuer the
// token's shape names: an Entra token is a JWT, a GitHub token never is.
type authenticator struct {
	github *GitHubClient
	entra  *entraClient
	org    string
	cache  *tokenCache
	// limit caps GitHub token checks on a cache miss below the app's GitHub
	// quota (5,000/h), so a flood of random tokens costs a 429 and not the
	// quota. Global, not per client: behind naisdevice many users share a
	// source address. A flood can make sign-in slow for others, never let
	// anyone in. Texas (the Entra path) is a local sidecar and not limited.
	limit *rate.Limiter
}

func (a *authenticator) resolve(ctx context.Context, token string) (*AuthenticatedUser, error) {
	if user, ok, err := a.cache.get(token); ok {
		return user, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user *AuthenticatedUser
	var err error
	if looksLikeJWT(token) {
		user, err = a.entra.resolveUser(ctx, token)
	} else {
		if !a.limit.Allow() {
			return nil, errRateLimited
		}
		user, err = a.github.resolveUser(ctx, token)
		if err == nil {
			var member bool
			member, err = a.github.isOrgMember(ctx, token, a.org, user.Login)
			if err == nil && !member {
				err = errNotOrgMember
			}
		}
	}
	switch {
	case err == nil:
		a.cache.set(token, user, nil)
	case errors.Is(err, errInvalidToken), errors.Is(err, errNotAUser), errors.Is(err, errNotOrgMember):
		// A refusal is cached briefly too: retrying a bad token must not
		// reach GitHub every time.
		a.cache.set(token, nil, err)
	}
	return user, err
}

var (
	errNotOrgMember = errors.New("not a member of the GitHub organisation")
	errRateLimited  = errors.New("too many sign-in attempts, try again shortly")
)

// looksLikeJWT reports whether token has a JWT's shape: three base64url
// segments, the first a JSON header ("eyJ"). GitHub tokens are prefixed
// opaque strings (ghu_, gho_, …) and never have it.
func looksLikeJWT(token string) bool {
	return strings.HasPrefix(token, "eyJ") && strings.Count(token, ".") == 2
}

// tokenCache remembers a token's outcome for a short while, keyed by a hash
// of the token so the raw token is never kept. A success is never kept past
// the token's own expiry.
type tokenCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	negTTL  time.Duration
	entries map[string]cacheEntry
}

type cacheEntry struct {
	user      *AuthenticatedUser
	err       error
	expiresAt time.Time
}

// ponytail: map with lazy eviction. The limiter bounds GitHub entries to a few
// thousand; Entra entries are only created for tokens Texas answered for.
// Add a size cap if memory ever shows it.
func newTokenCache(ttl time.Duration) *tokenCache {
	return &tokenCache{ttl: ttl, negTTL: time.Minute, entries: make(map[string]cacheEntry)}
}

func (c *tokenCache) get(token string) (*AuthenticatedUser, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := hashToken(token)
	entry, ok := c.entries[key]
	if !ok {
		return nil, false, nil
	}
	if !time.Now().Before(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false, nil
	}
	return entry.user, true, entry.err
}

func (c *tokenCache) set(token string, user *AuthenticatedUser, err error) {
	expires := time.Now().Add(c.negTTL)
	if err == nil {
		expires = time.Now().Add(c.ttl)
		if !user.expiresAt.IsZero() && user.expiresAt.Before(expires) {
			expires = user.expiresAt
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[hashToken(token)] = cacheEntry{user: user, err: err, expiresAt: expires}
}

// hashToken derives a cache key from a token without retaining the token.
// Tokens are high-entropy secrets, not passwords, so a plain SHA-256 is
// enough.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// authMiddleware resolves the caller before the handler runs and puts the
// user in the request context. Fail closed: any error talking to an issuer
// rejects the request. Neither the token nor the identity is logged.
func authMiddleware(a *authenticator, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		user, err := a.resolve(r.Context(), token)
		switch {
		case err == nil:
		case errors.Is(err, errRateLimited):
			w.Header().Set("Retry-After", "5")
			writeError(w, http.StatusTooManyRequests, err.Error())
			return
		case errors.Is(err, errNotOrgMember):
			writeError(w, http.StatusForbidden, fmt.Sprintf("user is not a member of %s", a.org))
			return
		case errors.Is(err, errNotAUser):
			writeError(w, http.StatusForbidden, err.Error())
			return
		case errors.Is(err, errInvalidToken):
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		case errors.Is(err, errIssuerOffline):
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		default:
			slog.Warn("token validation failed upstream", "error", err)
			writeError(w, http.StatusBadGateway, "could not validate token")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), requestUserContextKey, user)))
	}
}

func bearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errors.New("missing Authorization header")
	}
	// The auth scheme is case-insensitive per RFC 9110 §11.1.
	fields := strings.Fields(header)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "Bearer") {
		return "", errors.New("authorization header must use bearer scheme")
	}
	if len(fields) != 2 || len(fields[1]) > 8192 {
		return "", errors.New("empty or malformed bearer token")
	}
	return fields[1], nil
}

func userFromContext(ctx context.Context) (*AuthenticatedUser, bool) {
	user, ok := ctx.Value(requestUserContextKey).(*AuthenticatedUser)
	return user, ok
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, message)
}
