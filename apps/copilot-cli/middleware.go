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

const issuerGitHub = "github"

// AuthenticatedUser is the caller: a navikt member signed in to nav-pilot
// with a GitHub App user token. Subject is GitHub's numeric user id, never an
// e-mail or a name.
type AuthenticatedUser struct {
	Issuer  string
	Subject string
	Login   string

	expiresAt time.Time
}

// authenticator resolves a GitHub bearer token to a navikt member.
type authenticator struct {
	github *GitHubClient
	org    string
	cache  *tokenCache
	// limit is the global ceiling on GitHub token checks on a cache miss,
	// below the app's GitHub quota (5,000/h), so a flood of random tokens
	// costs a 429 and not the quota. Not per source address: behind
	// naisdevice many users share one. A flood can make sign-in slow for
	// others, never let anyone in.
	limit *rate.Limiter

	// perToken gives each token its own small bucket in front of limit, so
	// one client retrying or firing requests in parallel with the same token
	// cannot drain the global bucket for everyone. A token gets an entry only
	// once limit admitted it, so random tokens cannot grow the map faster
	// than the global rate.
	mu       sync.Mutex
	perToken map[string]*tokenBucket
}

type tokenBucket struct {
	lim  *rate.Limiter
	seen time.Time
}

const (
	perTokenEvery = 10 * time.Second
	perTokenBurst = 3
	// perTokenIdle is well past a full refill (burst × every), so dropping
	// an idle bucket loses nothing.
	perTokenIdle = time.Minute
)

// allow spends one GitHub check for the token hashed to key: first from its
// own bucket, then from the global one.
func (a *authenticator) allow(key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	b, ok := a.perToken[key]
	if ok {
		b.seen = now
		return b.lim.AllowN(now, 1) && a.limit.AllowN(now, 1)
	}
	if !a.limit.AllowN(now, 1) {
		return false
	}
	if a.perToken == nil {
		a.perToken = make(map[string]*tokenBucket)
	}
	for k, old := range a.perToken {
		if now.Sub(old.seen) > perTokenIdle {
			delete(a.perToken, k)
		}
	}
	b = &tokenBucket{lim: rate.NewLimiter(rate.Every(perTokenEvery), perTokenBurst), seen: now}
	b.lim.AllowN(now, 1)
	a.perToken[key] = b
	return true
}

func (a *authenticator) resolve(ctx context.Context, token string) (*AuthenticatedUser, error) {
	if user, ok, err := a.cache.get(token); ok {
		return user, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if !a.allow(hashToken(token)) {
		return nil, errRateLimited
	}
	user, err := a.github.resolveUser(ctx, token)
	if err == nil {
		var member bool
		member, err = a.github.isOrgMember(ctx, token, a.org, user.Login)
		if err == nil && !member {
			err = errNotOrgMember
		}
	}
	switch {
	case err == nil:
		a.cache.set(token, user, nil)
	case errors.Is(err, errInvalidToken) || errors.Is(err, errNotOrgMember):
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

// ponytail: map with lazy eviction. The limiter bounds entries to a few
// thousand. Add a size cap if memory ever shows it.
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
	key := hashToken(token)
	// A token GitHub called invalid never turns valid again, so a success
	// from a check that started earlier must not replace the refusal (say,
	// one that raced a revoke).
	if old, ok := c.entries[key]; ok && err == nil && errors.Is(old.err, errInvalidToken) && time.Now().Before(old.expiresAt) {
		return
	}
	c.entries[key] = cacheEntry{user: user, err: err, expiresAt: expires}
}

// revoked refuses token from the cache for as long as a success could have
// been cached, so a revoked token stops working here at once.
func (c *tokenCache) revoked(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[hashToken(token)] = cacheEntry{err: errInvalidToken, expiresAt: time.Now().Add(c.ttl)}
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

// revokeHandler revokes the caller's own token (nav-pilot auth logout). It
// takes no body: the token revoked is the bearer token presented, so a caller
// can only revoke a token it holds. No org check, so someone who has left the
// organisation can still sign out.
func revokeHandler(a *authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		if !a.allow(hashToken(token)) {
			w.Header().Set("Retry-After", "5")
			writeError(w, http.StatusTooManyRequests, errRateLimited.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		err = a.github.revoke(ctx, token)
		switch {
		case err == nil:
			a.cache.revoked(token)
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, errInvalidToken):
			a.cache.revoked(token)
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, errIssuerOffline):
			writeError(w, http.StatusServiceUnavailable, err.Error())
		default:
			slog.Warn("token revoke failed upstream", "error", err)
			writeError(w, http.StatusBadGateway, "could not revoke token")
		}
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
		return "", errors.New("the Authorization header must use the Bearer scheme")
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
