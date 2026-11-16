// Package main implements the fleet-gateway OAuth 2.1 authorization server for MCP routes.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPort              = "8080"
	defaultPendingTTL        = 10 * time.Minute
	defaultCodeTTL           = 60 * time.Second
	defaultSessionMaxTTL     = 10 * time.Hour
	defaultSessionTTL        = time.Hour
	defaultCIMDTTL           = 5 * time.Minute
	defaultOutboundTimeout   = 10 * time.Second
	defaultPendingCap        = 1024
	defaultCodeCap           = 1024
	defaultSessionCap        = 4096
	defaultCIMDCap           = 64
	defaultRateLimitCap      = 4096
	defaultRateLimitRequests = 30
	defaultRateLimitWindow   = time.Minute
	defaultSweepInterval     = 30 * time.Second
	maxFormBodyBytes         = 16 * 1024
	maxCIMDBodyBytes         = 64 * 1024
	maxUpstreamBodyBytes     = 1 << 20
)

// TeamTokenSource acquires downstream Entra OBO tokens for team MCP servers.
// TODO(step 2): implement Entra refresh and Texas OBO exchange for routed MCP calls.
type TeamTokenSource interface {
	TeamToken(ctx context.Context, sessionID, resource string) (string, error)
}

// Config holds validated runtime configuration for fleet-gateway.
type Config struct {
	Port                   string
	LogLevel               slog.Level
	BaseURL                string
	AllowedClientIDs       map[string]bool
	AllowedRedirectURIs    map[string]bool
	AllowedResources       map[string]bool
	PrimaryResource        string
	EntraScope             string
	AzureClientID          string
	AzureClientSecret      string
	AzureWellKnownURL      string
	AzureTokenEndpoint     string
	AzureAuthorizeEndpoint string
	CookieSecret           []byte
	HTTPClient             *http.Client
	PendingCap             int
	CodeCap                int
	SessionCap             int
	CIMDCap                int
	RateLimitCap           int
	RateLimitRequests      int
	RateLimitWindow        time.Duration
}

func loadConfigFromEnv() (*Config, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BASE_URL")), "/")
	clientIDs := parseCSVSet(os.Getenv("ALLOWED_CLIENT_IDS"))
	redirectURIs := parseCSVSet(os.Getenv("ALLOWED_REDIRECT_URIS"))
	resources := parseCSVSet(os.Getenv("ALLOWED_RESOURCES"))

	primaryResource := ""
	if baseURL != "" && len(resources) == 0 {
		primaryResource = baseURL + "/mcp"
		resources[primaryResource] = true
	} else if len(resources) > 0 {
		if baseURL != "" && resources[baseURL+"/mcp"] {
			primaryResource = baseURL + "/mcp"
		} else {
			for r := range resources {
				if primaryResource == "" || r < primaryResource {
					primaryResource = r
				}
			}
		}
	}

	cookieSecret, err := loadCookieSecret(os.Getenv("COOKIE_SIGNING_SECRET"))
	if err != nil {
		return nil, err
	}

	entraScope := strings.TrimSpace(os.Getenv("ENTRA_SCOPE"))
	if entraScope == "" {
		entraScope = "openid"
	}

	cfg := &Config{
		Port:                   getEnv("PORT", defaultPort),
		LogLevel:               parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		BaseURL:                baseURL,
		AllowedClientIDs:       clientIDs,
		AllowedRedirectURIs:    redirectURIs,
		AllowedResources:       resources,
		PrimaryResource:        primaryResource,
		EntraScope:             entraScope,
		AzureClientID:          strings.TrimSpace(os.Getenv("AZURE_APP_CLIENT_ID")),
		AzureClientSecret:      strings.TrimSpace(os.Getenv("AZURE_APP_CLIENT_SECRET")),
		AzureWellKnownURL:      strings.TrimSpace(os.Getenv("AZURE_APP_WELL_KNOWN_URL")),
		AzureTokenEndpoint:     strings.TrimSpace(os.Getenv("AZURE_OPENID_CONFIG_TOKEN_ENDPOINT")),
		AzureAuthorizeEndpoint: strings.TrimSpace(os.Getenv("AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT")),
		CookieSecret:           cookieSecret,
		HTTPClient:             newHardenedHTTPClient(defaultOutboundTimeout),
		PendingCap:             getEnvInt("PENDING_MAP_CAP", defaultPendingCap),
		CodeCap:                getEnvInt("CODE_MAP_CAP", defaultCodeCap),
		SessionCap:             getEnvInt("SESSION_MAP_CAP", defaultSessionCap),
		CIMDCap:                defaultCIMDCap,
		RateLimitCap:           getEnvInt("RATE_LIMIT_MAP_CAP", defaultRateLimitCap),
		RateLimitRequests:      getEnvInt("RATE_LIMIT_REQUESTS", defaultRateLimitRequests),
		RateLimitWindow:        defaultRateLimitWindow,
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func validateConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("nil config")
	}
	if err := validateHTTPURL(cfg.BaseURL, false); err != nil {
		return fmt.Errorf("invalid BASE_URL: %w", err)
	}
	if len(cfg.AllowedClientIDs) == 0 {
		return errors.New("ALLOWED_CLIENT_IDS must not be empty")
	}
	for clientID := range cfg.AllowedClientIDs {
		if err := validateHTTPURL(clientID, false); err != nil {
			return fmt.Errorf("invalid ALLOWED_CLIENT_IDS entry %q: %w", clientID, err)
		}
	}
	if len(cfg.AllowedRedirectURIs) == 0 {
		return errors.New("ALLOWED_REDIRECT_URIS must not be empty")
	}
	for redirectURI := range cfg.AllowedRedirectURIs {
		if err := validateHTTPURL(redirectURI, true); err != nil {
			return fmt.Errorf("invalid ALLOWED_REDIRECT_URIS entry %q: %w", redirectURI, err)
		}
	}
	if len(cfg.AllowedResources) == 0 {
		return errors.New("ALLOWED_RESOURCES must not be empty")
	}
	for res := range cfg.AllowedResources {
		if err := validateHTTPURL(res, false); err != nil {
			return fmt.Errorf("invalid ALLOWED_RESOURCES entry %q: %w", res, err)
		}
	}
	if cfg.AzureClientID == "" {
		return errors.New("AZURE_APP_CLIENT_ID is required")
	}
	if cfg.AzureClientSecret == "" {
		return errors.New("AZURE_APP_CLIENT_SECRET is required")
	}
	if err := validateHTTPURL(cfg.AzureTokenEndpoint, false); err != nil {
		return fmt.Errorf("invalid AZURE_OPENID_CONFIG_TOKEN_ENDPOINT: %w", err)
	}
	if cfg.AzureAuthorizeEndpoint == "" && cfg.AzureWellKnownURL == "" {
		return errors.New("AZURE_APP_WELL_KNOWN_URL or AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT is required")
	}
	if cfg.AzureAuthorizeEndpoint != "" {
		if err := validateHTTPURL(cfg.AzureAuthorizeEndpoint, false); err != nil {
			return fmt.Errorf("invalid AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT: %w", err)
		}
	}
	if cfg.AzureWellKnownURL != "" {
		if err := validateHTTPURL(cfg.AzureWellKnownURL, false); err != nil {
			return fmt.Errorf("invalid AZURE_APP_WELL_KNOWN_URL: %w", err)
		}
	}
	if len(cfg.CookieSecret) < 32 {
		return errors.New("cookie signing secret must be at least 32 bytes")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = newHardenedHTTPClient(defaultOutboundTimeout)
	}
	return nil
}

func validateHTTPURL(raw string, allowQuery bool) error {
	if raw == "" {
		return errors.New("empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("missing host")
	}
	if u.User != nil {
		return errors.New("userinfo not allowed in URL")
	}
	if u.Fragment != "" {
		return errors.New("fragment not allowed in URL")
	}
	if !allowQuery && u.RawQuery != "" {
		return errors.New("query string not allowed in URL")
	}
	return nil
}

func newHardenedHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("redirects refused")
		},
	}
}

func loadCookieSecret(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("generate cookie secret: %w", err)
		}
		return secret, nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(trimmed); err == nil && len(decoded) >= 32 {
		return decoded, nil
	}
	if len(trimmed) < 32 {
		return nil, errors.New("COOKIE_SIGNING_SECRET must be at least 32 bytes")
	}
	return []byte(trimmed), nil
}

func parseCSVSet(raw string) map[string]bool {
	out := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		item := strings.TrimSpace(part)
		if item != "" {
			out[item] = true
		}
	}
	return out
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

type ttlEntry[V any] struct {
	value     V
	expiresAt time.Time
}

type ttlMap[V any] struct {
	mu      sync.Mutex
	items   map[string]ttlEntry[V]
	maxSize int
	now     func() time.Time
}

func newTTLMap[V any](maxSize int, now func() time.Time) *ttlMap[V] {
	if now == nil {
		now = time.Now
	}
	if maxSize <= 0 {
		maxSize = 1
	}
	return &ttlMap[V]{
		items:   make(map[string]ttlEntry[V]),
		maxSize: maxSize,
		now:     now,
	}
}

func (m *ttlMap[V]) Set(key string, val V, ttl time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	if _, exists := m.items[key]; !exists && len(m.items) >= m.maxSize {
		m.sweepLocked(now)
		if len(m.items) >= m.maxSize {
			return false
		}
	}
	m.items[key] = ttlEntry[V]{value: val, expiresAt: now.Add(ttl)}
	return true
}

func (m *ttlMap[V]) Get(key string) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	if !m.now().Before(entry.expiresAt) {
		delete(m.items, key)
		var zero V
		return zero, false
	}
	return entry.value, true
}

func (m *ttlMap[V]) Pop(key string) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	delete(m.items, key)
	if !m.now().Before(entry.expiresAt) {
		var zero V
		return zero, false
	}
	return entry.value, true
}

func (m *ttlMap[V]) Sweep() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sweepLocked(m.now())
}

func (m *ttlMap[V]) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

func (m *ttlMap[V]) sweepLocked(now time.Time) int {
	removed := 0
	for k, entry := range m.items {
		if !now.Before(entry.expiresAt) {
			delete(m.items, k)
			removed++
		}
	}
	return removed
}
