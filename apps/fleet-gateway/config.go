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
	defaultPort                          = "8080"
	defaultPendingTTL                    = 10 * time.Minute
	defaultCodeTTL                       = 60 * time.Second
	defaultSessionMaxAge                 = 12 * time.Hour
	defaultSessionTTL                    = time.Hour
	defaultCIMDTTL                       = 5 * time.Minute
	defaultOutboundTimeout               = 10 * time.Second
	defaultPendingCap                    = 1024
	defaultCodeCap                       = 1024
	defaultSessionCap                    = 4096
	defaultCIMDCap                       = 64
	defaultRateLimitCap                  = 4096
	defaultRateLimitRequests             = 30
	defaultRateLimitGlobalRequests       = 300
	defaultRateLimitWindow               = time.Minute
	defaultSweepInterval                 = 30 * time.Second
	defaultSSEMaxDuration                = 10 * time.Minute
	defaultSSEIdleTimeout                = 60 * time.Second
	defaultSSEMaxBytes             int64 = 16 << 20 // 16 MiB
	maxFormBodyBytes                     = 20 * 1024
	maxClaimsBytes                       = 4096
	maxCIMDBodyBytes                     = 64 * 1024
	maxUpstreamBodyBytes                 = 1 << 20

	rtStorageNone            = "none"
	rtStorageClientEncrypted = "client-encrypted"
	// rtStorageServer keeps the Entra refresh token inside the session in the store
	// (existing StrategyRefresh behaviour) and issues NO gateway refresh token
	// (cplt re-logs in when the access token expires).
	rtStorageServer = "server"
)

// TeamTokenSource acquires downstream Entra OBO tokens for team MCP servers.
type TeamTokenSource interface {
	TeamToken(ctx context.Context, sessionID, resource string) (string, error)
}

// Config holds validated runtime configuration for fleet-gateway.
type Config struct {
	Port                    string
	LogLevel                slog.Level
	BaseURL                 string
	AllowedClientIDs        map[string]bool
	AllowedRedirectURIs     map[string]bool
	AllowedResources        map[string]bool
	PrimaryResource         string
	Routes                  []PilotRoute
	RoutesByPath            map[string]PilotRoute
	GatewayDisabled         bool
	TeamTokenStrategy       string
	TexasExchangeEndpoint   string
	EntraScope              string
	AzureClientID           string
	AzureClientSecret       string
	AzureWellKnownURL       string
	AzureTokenEndpoint      string
	AzureAuthorizeEndpoint  string
	CookieSecret            []byte
	HTTPClient              *http.Client
	ProxyHeaderTimeout      time.Duration
	SSEMaxDuration          time.Duration
	SSEIdleTimeout          time.Duration
	SSEMaxBytes             int64
	MaxResponseBodyBytes    int64
	PendingCap              int
	CodeCap                 int
	SessionCap              int
	CIMDCap                 int
	RateLimitCap            int
	RateLimitRequests       int
	RateLimitGlobalRequests int
	RateLimitWindow         time.Duration
	SessionStore            string
	DatabaseURL             string
	SessionMaxAge           time.Duration
	RTKeys                  string
	RTStorage               string
	Store                   SessionStore
}

func loadConfigFromEnv() (*Config, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BASE_URL")), "/")
	clientIDs := parseCSVSet(os.Getenv("ALLOWED_CLIENT_IDS"))
	redirectURIs := parseCSVSet(os.Getenv("ALLOWED_REDIRECT_URIS"))

	routes, err := loadPilotRoutes(os.Getenv("PILOT_ROUTES_FILE"))
	if err != nil {
		return nil, err
	}

	resources := make(map[string]bool, len(routes))
	routesByPath := make(map[string]PilotRoute, len(routes))
	primaryResource := ""
	for i := range routes {
		fullURL := baseURL + routes[i].Route
		routes[i].FullRouteID = fullURL
		resources[fullURL] = true
		routesByPath[routes[i].Route] = routes[i]
		if primaryResource == "" {
			primaryResource = fullURL
		}
	}

	cookieSecret, err := loadCookieSecret(os.Getenv("COOKIE_SIGNING_SECRET"))
	if err != nil {
		return nil, err
	}

	strategy := strings.ToLower(strings.TrimSpace(os.Getenv("TEAM_TOKEN_STRATEGY")))
	if strategy == "" {
		strategy = strategyOBO
	}

	rtKeys := strings.TrimSpace(os.Getenv("RT_KEYS"))
	rtStorage := strings.ToLower(strings.TrimSpace(os.Getenv("RT_STORAGE")))
	if rtStorage == rtStorageNone {
		return nil, fmt.Errorf("invalid RT_STORAGE %q: expected client-encrypted or server", rtStorage)
	}

	gatewayDisabled, err := getEnvBool("GATEWAY_DISABLED")
	if err != nil {
		return nil, err
	}

	// The user's Entra access token must be issued to fleet-gateway itself:
	// Texas OBO requires the assertion's aud to be the exchanging app, and
	// strategy refresh or client-encrypted refresh tokens need offline_access.
	entraScope := strings.TrimSpace(os.Getenv("ENTRA_SCOPE"))
	if entraScope == "" {
		entraScope = defaultEntraScope(strategy, rtStorage, rtKeys, strings.TrimSpace(os.Getenv("AZURE_APP_CLIENT_ID")))
	}

	cfg := &Config{
		Port:                    getEnv("PORT", defaultPort),
		LogLevel:                parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		BaseURL:                 baseURL,
		AllowedClientIDs:        clientIDs,
		AllowedRedirectURIs:     redirectURIs,
		AllowedResources:        resources,
		PrimaryResource:         primaryResource,
		Routes:                  routes,
		RoutesByPath:            routesByPath,
		GatewayDisabled:         gatewayDisabled,
		TeamTokenStrategy:       strategy,
		TexasExchangeEndpoint:   strings.TrimSpace(os.Getenv("NAIS_TOKEN_EXCHANGE_ENDPOINT")),
		EntraScope:              entraScope,
		AzureClientID:           strings.TrimSpace(os.Getenv("AZURE_APP_CLIENT_ID")),
		AzureClientSecret:       strings.TrimSpace(os.Getenv("AZURE_APP_CLIENT_SECRET")),
		AzureWellKnownURL:       strings.TrimSpace(os.Getenv("AZURE_APP_WELL_KNOWN_URL")),
		AzureTokenEndpoint:      strings.TrimSpace(os.Getenv("AZURE_OPENID_CONFIG_TOKEN_ENDPOINT")),
		AzureAuthorizeEndpoint:  strings.TrimSpace(os.Getenv("AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT")),
		CookieSecret:            cookieSecret,
		HTTPClient:              newHardenedHTTPClient(defaultOutboundTimeout),
		ProxyHeaderTimeout:      defaultOutboundTimeout,
		SSEMaxDuration:          getEnvDuration("SSE_MAX_DURATION", defaultSSEMaxDuration),
		SSEIdleTimeout:          getEnvDuration("SSE_IDLE_TIMEOUT", defaultSSEIdleTimeout),
		SSEMaxBytes:             getEnvInt64("SSE_MAX_BYTES", defaultSSEMaxBytes),
		MaxResponseBodyBytes:    getEnvInt64("MAX_RESPONSE_BODY_BYTES", maxMCPResponseBodyBytes),
		PendingCap:              getEnvInt("PENDING_MAP_CAP", defaultPendingCap),
		CodeCap:                 getEnvInt("CODE_MAP_CAP", defaultCodeCap),
		SessionCap:              getEnvInt("SESSION_MAP_CAP", defaultSessionCap),
		CIMDCap:                 defaultCIMDCap,
		RateLimitCap:            getEnvInt("RATE_LIMIT_MAP_CAP", defaultRateLimitCap),
		RateLimitRequests:       getEnvInt("RATE_LIMIT_REQUESTS", defaultRateLimitRequests),
		RateLimitGlobalRequests: getEnvInt("RATE_LIMIT_GLOBAL_REQUESTS", defaultRateLimitGlobalRequests),
		RateLimitWindow:         defaultRateLimitWindow,
		SessionStore:            strings.ToLower(getEnv("SESSION_STORE", "memory")),
		DatabaseURL:             strings.TrimSpace(os.Getenv("DATABASE_URL")),
		SessionMaxAge:           getEnvDuration("SESSION_MAX_AGE", defaultSessionMaxAge),
		RTKeys:                  rtKeys,
		RTStorage:               rtStorage,
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
	validatedRoutes, err := validatePilotRoutes(cfg.Routes)
	if err != nil {
		return err
	}
	cfg.Routes = validatedRoutes
	cfg.RoutesByPath = make(map[string]PilotRoute, len(validatedRoutes))
	cfg.AllowedResources = make(map[string]bool, len(validatedRoutes))
	for i := range cfg.Routes {
		fullURL := cfg.BaseURL + cfg.Routes[i].Route
		cfg.Routes[i].FullRouteID = fullURL
		cfg.RoutesByPath[cfg.Routes[i].Route] = cfg.Routes[i]
		cfg.AllowedResources[fullURL] = true
	}
	if cfg.PrimaryResource == "" || !cfg.AllowedResources[cfg.PrimaryResource] {
		cfg.PrimaryResource = cfg.Routes[0].FullRouteID
	}

	switch strings.ToLower(strings.TrimSpace(cfg.TeamTokenStrategy)) {
	case "", strategyOBO:
		cfg.TeamTokenStrategy = strategyOBO
		if err := validateHTTPURL(cfg.TexasExchangeEndpoint, false); err != nil {
			return fmt.Errorf("invalid NAIS_TOKEN_EXCHANGE_ENDPOINT: %w", err)
		}
	case strategyRefresh:
		cfg.TeamTokenStrategy = strategyRefresh
		if cfg.TexasExchangeEndpoint != "" {
			if err := validateHTTPURL(cfg.TexasExchangeEndpoint, false); err != nil {
				return fmt.Errorf("invalid NAIS_TOKEN_EXCHANGE_ENDPOINT: %w", err)
			}
		}
	default:
		return fmt.Errorf("invalid TEAM_TOKEN_STRATEGY %q: expected obo or refresh", cfg.TeamTokenStrategy)
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
	if cfg.ProxyHeaderTimeout <= 0 {
		cfg.ProxyHeaderTimeout = defaultOutboundTimeout
	}
	if cfg.SSEMaxDuration <= 0 {
		cfg.SSEMaxDuration = defaultSSEMaxDuration
	}
	if cfg.SSEIdleTimeout <= 0 {
		cfg.SSEIdleTimeout = defaultSSEIdleTimeout
	}
	if cfg.SSEMaxBytes <= 0 {
		cfg.SSEMaxBytes = defaultSSEMaxBytes
	}
	if cfg.MaxResponseBodyBytes <= 0 {
		cfg.MaxResponseBodyBytes = maxMCPResponseBodyBytes
	}
	if cfg.RateLimitGlobalRequests <= 0 {
		cfg.RateLimitGlobalRequests = defaultRateLimitGlobalRequests
	}
	switch strings.ToLower(strings.TrimSpace(cfg.SessionStore)) {
	case "", "memory":
		cfg.SessionStore = "memory"
	case "postgres":
		cfg.SessionStore = "postgres"
		// Empty DATABASE_URL means the libpq PG* environment (Nais Postgres).
		if cfg.DatabaseURL != "" || os.Getenv("PGHOST") == "" {
			if err := validatePostgresURL(cfg.DatabaseURL); err != nil {
				return fmt.Errorf("invalid DATABASE_URL: %w", err)
			}
		}
		if strings.TrimSpace(cfg.RTKeys) == "" {
			return errors.New("RT_KEYS is required when SESSION_STORE=postgres")
		}
	default:
		return fmt.Errorf("invalid SESSION_STORE %q: expected memory or postgres", cfg.SessionStore)
	}
	if strings.TrimSpace(cfg.RTKeys) != "" {
		if _, err := parseKeyRing(cfg.RTKeys); err != nil {
			return fmt.Errorf("invalid RT_KEYS: %w", err)
		}
	}
	// rtStorageServer ("server") keeps the Entra refresh token inside the session
	// in the store (existing StrategyRefresh behaviour) and issues NO gateway
	// refresh token (cplt re-logs in when the access token expires).
	switch strings.ToLower(strings.TrimSpace(cfg.RTStorage)) {
	case "":
		if strings.TrimSpace(cfg.RTKeys) != "" {
			cfg.RTStorage = rtStorageClientEncrypted
		} else {
			cfg.RTStorage = rtStorageNone
		}
	case rtStorageNone:
		if strings.TrimSpace(cfg.RTKeys) != "" {
			return fmt.Errorf("invalid RT_STORAGE %q: expected client-encrypted or server", cfg.RTStorage)
		}
		cfg.RTStorage = rtStorageNone
	case rtStorageClientEncrypted:
		cfg.RTStorage = rtStorageClientEncrypted
		if strings.TrimSpace(cfg.RTKeys) == "" {
			return errors.New("RT_KEYS is required when RT_STORAGE=client-encrypted")
		}
	case rtStorageServer:
		cfg.RTStorage = rtStorageServer
	default:
		return fmt.Errorf("invalid RT_STORAGE %q: expected client-encrypted or server", cfg.RTStorage)
	}
	if cfg.RTStorage == rtStorageClientEncrypted && cfg.TeamTokenStrategy == strategyRefresh {
		return errors.New("strategy refresh needs RT_STORAGE=server")
	}
	if cfg.SessionMaxAge <= 0 {
		cfg.SessionMaxAge = defaultSessionMaxAge
	}
	if cfg.PendingCap <= 0 {
		cfg.PendingCap = defaultPendingCap
	}
	if cfg.CodeCap <= 0 {
		cfg.CodeCap = defaultCodeCap
	}
	if cfg.SessionCap <= 0 {
		cfg.SessionCap = defaultSessionCap
	}
	return nil
}

func validatePostgresURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return errors.New("empty URL")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		// url.Error embeds the raw URL, i.e. the DSN with its password.
		return errors.New("unparseable URL")
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" && u.Query().Get("host") == "" {
		return errors.New("missing host")
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

func getEnvBool(key string) (bool, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return b, nil
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

func getEnvInt64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
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

// defaultEntraScope asks Entra for an access token issued to fleet-gateway
// itself, plus offline_access when strategy is refresh or client-encrypted
// refresh tokens are enabled (RT_KEYS non-empty and RTStorage != server).
func defaultEntraScope(strategy, rtStorage, rtKeys, clientID string) string {
	needsOffline := strategy == strategyRefresh ||
		(strings.TrimSpace(rtKeys) != "" && strings.ToLower(strings.TrimSpace(rtStorage)) != rtStorageServer)
	if needsOffline {
		return "openid offline_access api://" + clientID + "/.default"
	}
	return "openid api://" + clientID + "/.default"
}
