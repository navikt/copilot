package main

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userContextKey contextKey = "user"

var authMiddlewareReady atomic.Bool

func init() {
	authMiddlewareReady.Store(false)
}

// User represents an authenticated user from Azure AD token
type User struct {
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	NAVident          string   `json:"navident"`
	PreferredUsername string   `json:"preferred_username"`
	Groups            []string `json:"groups"`
	AZP               string   `json:"azp"` // Authorized party (client ID)
}

// JWKS represents the JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKSCache caches JWKS keys with automatic refresh
type JWKSCache struct {
	jwksURI    string
	keys       map[string]*rsa.PublicKey
	lastUpdate time.Time
	mu         sync.RWMutex
	ttl        time.Duration
}

func newJWKSCache(jwksURI string) *JWKSCache {
	return &JWKSCache{
		jwksURI: jwksURI,
		keys:    make(map[string]*rsa.PublicKey),
		ttl:     1 * time.Hour,
	}
}

func (c *JWKSCache) getKey(kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	key, exists := c.keys[kid]
	needsRefresh := time.Since(c.lastUpdate) > c.ttl
	c.mu.RUnlock()

	if exists && !needsRefresh {
		return key, nil
	}

	// Refresh keys
	if err := c.refresh(); err != nil {
		// If refresh fails but we have a cached key, use it
		if exists {
			return key, nil
		}
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	key, exists = c.keys[kid]
	if !exists {
		return nil, fmt.Errorf("key with kid %s not found after refresh", kid)
	}
	return key, nil
}

func (c *JWKSCache) refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.jwksURI, nil)
	if err != nil {
		return fmt.Errorf("failed to create JWKS request: %w", err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" {
			continue
		}

		pubKey, err := jwkToPublicKey(key)
		if err != nil {
			slog.Warn("Failed to parse JWK", "kid", key.Kid, "error", err)
			continue
		}

		newKeys[key.Kid] = pubKey
	}

	c.keys = newKeys
	c.lastUpdate = time.Now()
	slog.Debug("JWKS cache refreshed", "key_count", len(newKeys))
	return nil
}

func jwkToPublicKey(jwk JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode N: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	var e int
	if len(eBytes) == 3 {
		e = int(eBytes[0])<<16 | int(eBytes[1])<<8 | int(eBytes[2])
	} else if len(eBytes) == 4 {
		e = int(eBytes[0])<<24 | int(eBytes[1])<<16 | int(eBytes[2])<<8 | int(eBytes[3])
	} else {
		e = int(new(big.Int).SetBytes(eBytes).Int64())
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

// TokenValidator validates Azure AD OBO tokens
type TokenValidator struct {
	issuer            string
	audience          string
	jwksCache         *JWKSCache
	preAuthorizedApps map[string]bool
}

func newTokenValidator(config *Config) (*TokenValidator, error) {
	if config.AzureIssuer == "" || config.AzureClientID == "" || config.AzureJWKSURI == "" {
		return nil, errors.New("azure AD configuration incomplete")
	}

	preAuth := make(map[string]bool)
	if config.PreAuthorizedApps != "" {
		var apps []struct {
			ClientID string `json:"clientId"`
		}
		if err := json.Unmarshal([]byte(config.PreAuthorizedApps), &apps); err != nil {
			return nil, fmt.Errorf("failed to parse pre-authorized apps: %w", err)
		}
		for _, app := range apps {
			preAuth[app.ClientID] = true
		}
	}

	return &TokenValidator{
		issuer:            config.AzureIssuer,
		audience:          config.AzureClientID,
		jwksCache:         newJWKSCache(config.AzureJWKSURI),
		preAuthorizedApps: preAuth,
	}, nil
}

func (v *TokenValidator) validate(tokenString string) (*User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid header missing")
		}

		return v.jwksCache.getKey(kid)
	})

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid claims format")
	}

	// Validate issuer
	if iss, ok := claims["iss"].(string); !ok || iss != v.issuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", v.issuer, iss)
	}

	// Validate audience (supports both string and array forms)
	if !validateAudience(claims["aud"], v.audience) {
		return nil, fmt.Errorf("invalid audience: expected %s", v.audience)
	}

	// Validate expiry
	if exp, ok := claims["exp"].(float64); !ok || time.Unix(int64(exp), 0).Before(time.Now()) {
		return nil, errors.New("token expired")
	}

	// Validate authorized party (azp) - CRITICAL security check
	// Fail closed: if no pre-authorized apps are configured, reject all requests
	azp, _ := claims["azp"].(string)
	if len(v.preAuthorizedApps) == 0 {
		return nil, errors.New("no pre-authorized apps configured — rejecting all requests")
	}
	if !v.preAuthorizedApps[azp] {
		return nil, fmt.Errorf("unauthorized client: %s", azp)
	}

	// Extract user information
	email := getStringClaim(claims, "email")
	if email == "" {
		email = getStringClaim(claims, "preferred_username")
	}

	user := &User{
		AZP:               azp,
		PreferredUsername: getStringClaim(claims, "preferred_username"),
		Email:             email,
		Name:              getStringClaim(claims, "name"),
		NAVident:          getStringClaim(claims, "NAVident"),
	}

	// Extract groups
	if groups, ok := claims["groups"].([]interface{}); ok {
		user.Groups = make([]string, 0, len(groups))
		for _, g := range groups {
			if group, ok := g.(string); ok {
				user.Groups = append(user.Groups, group)
			}
		}
	}

	return user, nil
}

func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key].(string); ok {
		return val
	}
	return ""
}

// extractBearerToken extracts the Bearer token from Authorization header
func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header missing")
	}

	parts := strings.Fields(strings.TrimSpace(authHeader))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("invalid authorization header format")
	}

	return parts[1], nil
}

func validateAudience(rawAudience interface{}, expectedAudience string) bool {
	switch audience := rawAudience.(type) {
	case string:
		return audience == expectedAudience
	case []interface{}:
		for _, item := range audience {
			if value, ok := item.(string); ok && value == expectedAudience {
				return true
			}
		}
	case []string:
		for _, value := range audience {
			if value == expectedAudience {
				return true
			}
		}
	}

	return false
}

// makeAuthMiddleware creates authentication middleware
func makeAuthMiddleware(config *Config) func(http.Handler) http.Handler {
	// In development without Azure config, allow requests through
	if config.Environment == "local" && config.AzureIssuer == "" {
		authMiddlewareReady.Store(true)
		// The mock user's email drives SAML-based username resolution (e.g. the budget
		// endpoint). Make it configurable so local dev can resolve a real GitHub identity.
		devEmail := getEnv("DEV_USER_EMAIL", "dev@nav.no")
		slog.Warn("Running in development mode without Azure AD validation", "devUserEmail", devEmail)
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Mock user for development
				ctx := context.WithValue(r.Context(), userContextKey, &User{
					Email:             devEmail,
					Name:              "Developer User",
					NAVident:          "DEV001",
					PreferredUsername: devEmail,
					Groups:            []string{"dev-group"},
					AZP:               "dev-client",
				})
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
	}

	validator, err := newTokenValidator(config)
	if err != nil {
		authMiddlewareReady.Store(false)
		slog.Error("Failed to create token validator", "error", err)
		// Return middleware that rejects all requests
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				respondError(w, "authentication_unavailable", "Authentication service unavailable", http.StatusServiceUnavailable)
			})
		}
	}
	authMiddlewareReady.Store(true)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := extractBearerToken(r)
			if err != nil {
				respondError(w, "unauthorized", err.Error(), http.StatusUnauthorized)
				return
			}

			user, err := validator.validate(token)
			if err != nil {
				slog.Warn("Token validation failed", "error", err, "path", r.URL.Path)
				respondError(w, "unauthorized", "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			slog.Debug("User authenticated", "navident", user.NAVident)

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// getUserFromContext extracts the authenticated user from request context
func getUserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}
