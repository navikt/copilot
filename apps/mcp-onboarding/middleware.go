package main

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

type AuthMiddleware struct {
	store *TokenStore
}

func NewAuthMiddleware(store *TokenStore) *AuthMiddleware {
	return &AuthMiddleware{store: store}
}

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			m.sendUnauthorized(w, r)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			m.sendUnauthorized(w, r)
			return
		}

		token := parts[1]
		tokenData, err := m.store.GetToken(token)
		if err != nil {
			slog.Warn("invalid or expired token", "error", err)
			m.sendUnauthorized(w, r)
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

func (m *AuthMiddleware) sendUnauthorized(w http.ResponseWriter, r *http.Request) {
	resourceMetadataURL := getBaseURL(r) + "/.well-known/oauth-protected-resource"
	slog.Debug("sending 401 with resource metadata", "resource_metadata_url", resourceMetadataURL)
	w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+resourceMetadataURL+`"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","message":"Valid Bearer token required"}`))
}

func getBaseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		if fwdProto := r.Header.Get("X-Forwarded-Proto"); fwdProto != "" {
			scheme = fwdProto
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host
}
