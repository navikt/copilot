package mcpkit

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// The names below match github.com/modelcontextprotocol/go-sdk/auth so a
// server can later swap mcpkit.RequireBearerToken for auth.RequireBearerToken
// and keep Texas.Verifier. Deliberate differences are marked; see
// https://github.com/modelcontextprotocol/go-sdk/issues/1543.

// ErrInvalidToken is go-sdk's name for a token that failed validation (401).
var ErrInvalidToken = ErrUnauthorized

// TokenInfo is what a TokenVerifier learned about a token.
type TokenInfo struct {
	Scopes     []string
	Expiration time.Time
	UserID     string
	Extra      map[string]any
}

// TokenVerifier validates a bearer token.
type TokenVerifier func(ctx context.Context, token string, req *http.Request) (*TokenInfo, error)

// RequireBearerTokenOptions configures RequireBearerToken.
// Difference from go-sdk: Realm replaces ResourceMetadataURL (the gateway
// serves resource metadata), and there is no AllowMissingExpiration: exp is
// always required.
type RequireBearerTokenOptions struct {
	Realm string
}

type tokenInfoKey struct{}

// TokenInfoFromContext returns the TokenInfo RequireBearerToken stored, or nil.
func TokenInfoFromContext(ctx context.Context) *TokenInfo {
	ti, _ := ctx.Value(tokenInfoKey{}).(*TokenInfo)
	return ti
}

// Verifier returns a TokenVerifier backed by Introspect. UserID is the
// NAVident, Expiration the token's exp, and Extra["token"] the raw token for
// Exchange.
func (t *Texas) Verifier() TokenVerifier {
	return func(ctx context.Context, token string, _ *http.Request) (*TokenInfo, error) {
		c, err := t.introspect(ctx, token)
		if err != nil {
			return nil, err
		}
		ti := &TokenInfo{UserID: c.NAVident, Extra: map[string]any{"token": token}}
		if c.Exp > 0 {
			ti.Expiration = time.Unix(c.Exp, 0)
		}
		return ti, nil
	}
}

// RequireBearerToken rejects requests without a valid bearer token and stores
// the TokenInfo in the request context. Differences from go-sdk, all kept on
// purpose (go-sdk#1543):
//   - BearerToken's 16 KiB cap and whitespace check run before the verifier.
//   - ErrForbidden is 403, ErrNotConfigured 503, any other verifier error
//     502. go-sdk turns every error but ErrInvalidToken into 500.
//   - A missing or past Expiration is the same 401 as ErrInvalidToken.
//   - The challenge carries a realm, sanitised by SetChallenge.
func RequireBearerToken(verifier TokenVerifier, opts *RequireBearerTokenOptions) func(http.Handler) http.Handler {
	realm := ""
	if opts != nil {
		realm = opts.Realm
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := BearerToken(r.Header.Get("Authorization"))
			if !ok {
				SetChallenge(w, realm, false)
				writeAuthJSON(w, http.StatusUnauthorized, "unauthorized", "missing or malformed bearer token")
				return
			}
			ti, err := verifier(r.Context(), token, r)
			// A missing exp is the zero time, so it fails the same check as a past one.
			if err == nil && (ti == nil || !ti.Expiration.After(time.Now())) {
				err = ErrInvalidToken
			}
			switch {
			case err == nil:
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), tokenInfoKey{}, ti)))
			case errors.Is(err, ErrInvalidToken):
				SetChallenge(w, realm, true)
				writeAuthJSON(w, http.StatusUnauthorized, "invalid_token", ErrInvalidToken.Error())
			case errors.Is(err, ErrForbidden):
				writeAuthJSON(w, http.StatusForbidden, "forbidden", ErrForbidden.Error())
			case errors.Is(err, ErrNotConfigured):
				writeAuthJSON(w, http.StatusServiceUnavailable, "unavailable", "authentication is not configured")
			default:
				slog.Warn("token verification failed", "path", strings.NewReplacer("\n", "", "\r", "").Replace(r.URL.Path), "error", strings.NewReplacer("\n", "", "\r", "").Replace(err.Error()))
				writeAuthJSON(w, http.StatusBadGateway, "introspection_failed", "failed to validate token")
			}
		})
	}
}

func writeAuthJSON(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": msg})
}
