package main

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

type AuthMiddleware struct {
	store   *TokenStore
	baseURL string
}

func NewAuthMiddleware(store *TokenStore, baseURL string) *AuthMiddleware {
	return &AuthMiddleware{
		store:   store,
		baseURL: baseURL,
	}
}

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.sendUnauthorized(w, false)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			m.sendUnauthorized(w, false) // not a Bearer credential: no error code (RFC 6750 section 3.1)
			return
		}

		token := parts[1]
		tokenData, err := m.store.GetToken(token)
		if err != nil {
			slog.Warn("invalid or expired token", "error", err)
			m.sendUnauthorized(w, true)
			return
		}

		userCtx := &UserContext{
			Login:             tokenData.UserLogin,
			ID:                tokenData.UserID,
			GitHubAccessToken: tokenData.GitHubAccessToken,
		}

		ctx := context.WithValue(r.Context(), userContextKey, userCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) sendUnauthorized(w http.ResponseWriter, tokenPresented bool) {
	resourceMetadataURL := strings.TrimRight(m.baseURL, "/") + "/.well-known/oauth-protected-resource"
	slog.Debug("sending 401 with resource metadata", "resource_metadata_url", resourceMetadataURL)
	challenge := `Bearer resource_metadata="` + resourceMetadataURL + `"`
	if tokenPresented {
		challenge += `, error="invalid_token"` // RFC 6750 section 3.1
	}
	w.Header().Set("WWW-Authenticate", challenge)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","message":"Valid Bearer token required"}`))
}
