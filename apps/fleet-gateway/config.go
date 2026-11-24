// Package main implements the fleet-gateway OAuth 2.1 authorization server for MCP routes.
package main

import (
	"context"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/navikt/copilot/fleet-gateway/internal/config"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	identitySourceLegacy        = "legacy"
	identitySourceIntrospection = "introspection"

	defaultPort                             = "8080"
	defaultPendingTTL                       = 10 * time.Minute
	defaultCodeTTL                          = 60 * time.Second
	defaultSessionMaxAge                    = 12 * time.Hour
	defaultSessionAbsoluteMaxAge            = 7 * 24 * time.Hour
	defaultSessionIdleTimeout               = 12 * time.Hour
	defaultRefreshGraceWindow               = 30 * time.Second
	defaultConsentTTL                       = 30 * 24 * time.Hour
	defaultSessionTTL                       = time.Hour
	defaultCIMDTTL                          = 5 * time.Minute
	cimdStaleMax                            = 24 * time.Hour
	defaultOutboundTimeout                  = 10 * time.Second
	defaultUpstreamTimeout                  = 60 * time.Second // MCP tool calls; headers and body each get this long
	defaultMCPMaxInFlight                   = 16
	defaultMCPMaxInFlightPerRoute           = 8
	defaultMCPMaxInFlightPerUser            = 4
	defaultMCPMaxSSE                        = 64 // ~40 KiB each: a 32 KiB read buffer plus the connection
	defaultMCPMaxSSEPerUser                 = 4
	defaultPendingCap                       = 4096
	defaultCodeCap                          = 1024
	defaultSessionCap                       = 4096
	defaultMCPSessionUserCap                = 64
	defaultCIMDCap                          = 64
	defaultDCRClientTTL                     = 30 * 24 * time.Hour
	defaultDCRClientMaxAge                  = 180 * 24 * time.Hour
	defaultDCRClientCap                     = 5000
	defaultDCRRateLimitRequests             = 10
	defaultDCRRateLimitGlobalRequests       = 100
	defaultDCRRateLimitWindow               = time.Hour
	maxDCRBodyBytes                         = 16 * 1024
	defaultRateLimitCap                     = 4096
	defaultRateLimitRequests                = 30
	defaultRateLimitGlobalRequests          = 300
	defaultRateLimitWindow                  = time.Minute
	defaultSweepInterval                    = 30 * time.Second
	defaultSSEMaxDuration                   = 10 * time.Minute
	defaultSSEIdleTimeout                   = 60 * time.Second
	defaultSSEMaxBytes                int64 = 16 << 20 // 16 MiB
	maxFormBodyBytes                        = 20 * 1024
	maxClaimsBytes                          = 4096
	maxCIMDBodyBytes                        = 64 * 1024
	maxUpstreamBodyBytes                    = 1 << 20

	rtStorageNone            = "none"
	rtStorageClientEncrypted = "client-encrypted"
	// rtStorageServer keeps the Entra refresh token inside the session in the store
	// (existing StrategyRefresh behaviour) and issues NO gateway refresh token
	// (cplt re-logs in when the access token expires).
	rtStorageServer = "server"
)

// TeamTokenSource acquires downstream Entra OBO tokens for team MCP servers.
type TeamTokenSource interface {
	// route is the entry the request was dispatched with, so the token's
	// audience and the upstream come from one snapshot (#2641).
	TeamToken(ctx context.Context, sessionID string, route PilotRoute) (string, error)
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
	TexasIntrospectEndpoint string // validates the Entra access token at login for NAVident (#2594)
	IdentitySource          string // legacy (default) or introspection; where login takes NAVident from (#2594)
	// RoutesSource is git (default, PILOT_ROUTES_FILE only) or fleet-control (#2641).
	RoutesSource string
	// RoutesShadowSource fleet-control compares snapshots with the Git table without serving them (#2760).
	RoutesShadowSource      string
	FleetControlURL         string
	FleetControlAudience    string
	TexasTokenEndpoint      string // M2M token for fleet-control
	EntraScope              string
	AzureClientID           string
	AzureClientSecret       string
	AzureWellKnownURL       string
	AzureTokenEndpoint      string
	AzureIssuer             string // expected iss of the Entra id_token
	AzureAuthorizeEndpoint  string
	CookieSecret            []byte
	CookieVerifyKeys        [][]byte // extra keys accepted on verify only (older RT_KEYS entries)
	HTTPClient              *http.Client
	ProxyHeaderTimeout      time.Duration
	MCPMaxInFlight          int // concurrent buffered /mcp/* requests per pod
	MCPMaxInFlightPerRoute  int // concurrent buffered requests per route, under MCPMaxInFlight
	MCPMaxInFlightPerUser   int // concurrent buffered /mcp/* requests per user per pod
	MCPMaxSSE               int // concurrent SSE response streams per pod
	MCPMaxSSEPerUser        int // concurrent SSE response streams per user per pod
	SSEMaxDuration          time.Duration
	SSEIdleTimeout          time.Duration
	SSEMaxBytes             int64
	MaxResponseBodyBytes    int64
	PendingCap              int
	CodeCap                 int
	SessionCap              int
	MCPSessionUserCap       int
	CIMDCap                 int
	DCREnabled              bool
	DCRClientTTL            time.Duration
	DCRClientMaxAge         time.Duration
	DCRClientCap            int
	DCRRateLimitRequests    int
	DCRRateLimitGlobal      int
	RateLimitCap            int
	RateLimitRequests       int
	RateLimitGlobalRequests int
	RateLimitWindow         time.Duration
	SessionStore            string
	DatabaseURL             string
	SessionMaxAge           time.Duration // longest life of one access token session
	// SessionAbsoluteMaxAge is the hard cap from the original login; refresh
	// never moves it. SessionIdleTimeout is how long a refresh token lives
	// without a refresh (sliding). Internal defaults 7 d / 12 h; an external
	// host sets shorter values (#2363).
	SessionAbsoluteMaxAge time.Duration
	SessionIdleTimeout    time.Duration
	RefreshGraceWindow    time.Duration
	ConsentTTL            time.Duration
	RTKeys                string
	RTStorage             string
	LockoutKey            string
	Store                 SessionStore
}

func loadConfigFromEnv() (*Config, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BASE_URL")), "/")
	clientIDs := config.CSVSet(os.Getenv("ALLOWED_CLIENT_IDS"))
	redirectURIs := config.CSVSet(os.Getenv("ALLOWED_REDIRECT_URIS"))

	routes, err := fileRouteSource{path: os.Getenv("PILOT_ROUTES_FILE")}.Routes()
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

	cookieKeys, err := loadCookieSecret(os.Getenv("COOKIE_SIGNING_SECRET"), os.Getenv("RT_KEYS"), os.Getenv("NAIS_CLUSTER_NAME") != "")
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

	dcrEnabled, err := config.Bool("DCR_ENABLED")
	if err != nil {
		return nil, err
	}
	gatewayDisabled, err := config.Bool("GATEWAY_DISABLED")
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
		Port:                    config.Get("PORT", defaultPort),
		LogLevel:                config.LogLevel(config.Get("LOG_LEVEL", "INFO")),
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
		TexasIntrospectEndpoint: strings.TrimSpace(os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT")),
		IdentitySource:          strings.TrimSpace(os.Getenv("IDENTITY_SOURCE")),
		RoutesSource:            strings.TrimSpace(os.Getenv("ROUTES_SOURCE")),
		RoutesShadowSource:      strings.TrimSpace(os.Getenv("ROUTES_SHADOW_SOURCE")),
		FleetControlURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("FLEET_CONTROL_URL")), "/"),
		FleetControlAudience:    strings.TrimSpace(os.Getenv("FLEET_CONTROL_AUDIENCE")),
		TexasTokenEndpoint:      strings.TrimSpace(os.Getenv("NAIS_TOKEN_ENDPOINT")),
		EntraScope:              entraScope,
		AzureClientID:           strings.TrimSpace(os.Getenv("AZURE_APP_CLIENT_ID")),
		AzureClientSecret:       strings.TrimSpace(os.Getenv("AZURE_APP_CLIENT_SECRET")),
		AzureWellKnownURL:       strings.TrimSpace(os.Getenv("AZURE_APP_WELL_KNOWN_URL")),
		AzureTokenEndpoint:      strings.TrimSpace(os.Getenv("AZURE_OPENID_CONFIG_TOKEN_ENDPOINT")),
		AzureIssuer:             strings.TrimSpace(os.Getenv("AZURE_OPENID_CONFIG_ISSUER")),
		AzureAuthorizeEndpoint:  strings.TrimSpace(os.Getenv("AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT")),
		CookieSecret:            cookieKeys[0],
		CookieVerifyKeys:        cookieKeys[1:],
		HTTPClient:              config.NewHardenedHTTPClient(defaultOutboundTimeout),
		ProxyHeaderTimeout:      config.Duration("UPSTREAM_TIMEOUT", defaultUpstreamTimeout),
		MCPMaxInFlight:          config.Int("MCP_MAX_IN_FLIGHT", defaultMCPMaxInFlight),
		MCPMaxInFlightPerRoute:  config.Int("MCP_MAX_IN_FLIGHT_PER_ROUTE", defaultMCPMaxInFlightPerRoute),
		MCPMaxInFlightPerUser:   config.Int("MCP_MAX_IN_FLIGHT_PER_USER", defaultMCPMaxInFlightPerUser),
		MCPMaxSSE:               config.Int("MCP_MAX_SSE", defaultMCPMaxSSE),
		MCPMaxSSEPerUser:        config.Int("MCP_MAX_SSE_PER_USER", defaultMCPMaxSSEPerUser),
		SSEMaxDuration:          config.Duration("SSE_MAX_DURATION", defaultSSEMaxDuration),
		SSEIdleTimeout:          config.Duration("SSE_IDLE_TIMEOUT", defaultSSEIdleTimeout),
		SSEMaxBytes:             config.Int64("SSE_MAX_BYTES", defaultSSEMaxBytes),
		MaxResponseBodyBytes:    config.Int64("MAX_RESPONSE_BODY_BYTES", maxMCPResponseBodyBytes),
		PendingCap:              config.Int("PENDING_MAP_CAP", defaultPendingCap),
		CodeCap:                 config.Int("CODE_MAP_CAP", defaultCodeCap),
		SessionCap:              config.Int("SESSION_MAP_CAP", defaultSessionCap),
		MCPSessionUserCap:       config.Int("MCP_SESSION_USER_CAP", defaultMCPSessionUserCap),
		CIMDCap:                 defaultCIMDCap,
		DCREnabled:              dcrEnabled,
		DCRClientTTL:            config.Duration("DCR_CLIENT_TTL", defaultDCRClientTTL),
		DCRClientMaxAge:         config.Duration("DCR_CLIENT_MAX_AGE", defaultDCRClientMaxAge),
		DCRClientCap:            config.Int("DCR_CLIENT_CAP", defaultDCRClientCap),
		DCRRateLimitRequests:    config.Int("DCR_RATE_LIMIT_REQUESTS", defaultDCRRateLimitRequests),
		DCRRateLimitGlobal:      config.Int("DCR_RATE_LIMIT_GLOBAL_REQUESTS", defaultDCRRateLimitGlobalRequests),
		RateLimitCap:            config.Int("RATE_LIMIT_MAP_CAP", defaultRateLimitCap),
		RateLimitRequests:       config.Int("RATE_LIMIT_REQUESTS", defaultRateLimitRequests),
		RateLimitGlobalRequests: config.Int("RATE_LIMIT_GLOBAL_REQUESTS", defaultRateLimitGlobalRequests),
		RateLimitWindow:         defaultRateLimitWindow,
		SessionStore:            strings.ToLower(config.Get("SESSION_STORE", "memory")),
		DatabaseURL:             strings.TrimSpace(os.Getenv("DATABASE_URL")),
		SessionMaxAge:           config.Duration("SESSION_MAX_AGE", defaultSessionMaxAge),
		SessionAbsoluteMaxAge:   config.Duration("SESSION_ABSOLUTE_MAX_AGE", defaultSessionAbsoluteMaxAge),
		SessionIdleTimeout:      config.Duration("SESSION_IDLE_TIMEOUT", defaultSessionIdleTimeout),
		RefreshGraceWindow:      config.Duration("REFRESH_GRACE_WINDOW", defaultRefreshGraceWindow),
		ConsentTTL:              config.Duration("CONSENT_TTL", defaultConsentTTL),
		RTKeys:                  rtKeys,
		RTStorage:               rtStorage,
		LockoutKey:              strings.TrimSpace(os.Getenv("LOCKOUT_KEY")),
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
	if err := config.ValidateHTTPURL(cfg.BaseURL, false); err != nil {
		return fmt.Errorf("invalid BASE_URL: %w", err)
	}
	if len(cfg.AllowedClientIDs) == 0 {
		return errors.New("ALLOWED_CLIENT_IDS must not be empty")
	}
	for clientID := range cfg.AllowedClientIDs {
		if err := config.ValidateHTTPURL(clientID, false); err != nil {
			return fmt.Errorf("invalid ALLOWED_CLIENT_IDS entry %q: %w", clientID, err)
		}
	}
	if len(cfg.AllowedRedirectURIs) == 0 {
		return errors.New("ALLOWED_REDIRECT_URIS must not be empty")
	}
	for redirectURI := range cfg.AllowedRedirectURIs {
		if err := config.ValidateHTTPURL(redirectURI, true); err != nil {
			return fmt.Errorf("invalid ALLOWED_REDIRECT_URIS entry %q: %w", redirectURI, err)
		}
	}
	validatedRoutes, err := validatePilotRoutes(cfg.Routes)
	if err != nil {
		return err
	}
	cfg.Routes = validatedRoutes
	switch cfg.RoutesSource {
	case "", routesSourceGit:
		cfg.RoutesSource = routesSourceGit
	case routesSourceFleetControl:
	default:
		return fmt.Errorf("unsupported ROUTES_SOURCE %q (git or fleet-control)", cfg.RoutesSource)
	}
	switch cfg.RoutesShadowSource {
	case "":
	case routesSourceFleetControl:
		if cfg.RoutesSource != routesSourceGit {
			return errors.New("ROUTES_SHADOW_SOURCE=fleet-control requires ROUTES_SOURCE=git")
		}
	default:
		return fmt.Errorf("unsupported ROUTES_SHADOW_SOURCE %q (fleet-control or empty)", cfg.RoutesShadowSource)
	}
	if cfg.RoutesSource == routesSourceFleetControl || cfg.RoutesShadowSource == routesSourceFleetControl {
		if err := config.ValidateHTTPURL(cfg.FleetControlURL, false); err != nil {
			return fmt.Errorf("invalid FLEET_CONTROL_URL: %w", err)
		}
		if err := config.ValidateHTTPURL(cfg.TexasTokenEndpoint, false); err != nil {
			return fmt.Errorf("invalid NAIS_TOKEN_ENDPOINT: %w", err)
		}
		if cfg.FleetControlAudience == "" {
			return errors.New("FLEET_CONTROL_AUDIENCE is required when ROUTES_SOURCE or ROUTES_SHADOW_SOURCE is fleet-control")
		}
	}
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
		if err := config.ValidateHTTPURL(cfg.TexasExchangeEndpoint, false); err != nil {
			return fmt.Errorf("invalid NAIS_TOKEN_EXCHANGE_ENDPOINT: %w", err)
		}
	case strategyRefresh:
		cfg.TeamTokenStrategy = strategyRefresh
		if cfg.TexasExchangeEndpoint != "" {
			if err := config.ValidateHTTPURL(cfg.TexasExchangeEndpoint, false); err != nil {
				return fmt.Errorf("invalid NAIS_TOKEN_EXCHANGE_ENDPOINT: %w", err)
			}
		}
	default:
		return fmt.Errorf("invalid TEAM_TOKEN_STRATEGY %q: expected obo or refresh", cfg.TeamTokenStrategy)
	}

	switch cfg.IdentitySource {
	case "", identitySourceLegacy:
		cfg.IdentitySource = identitySourceLegacy
	case identitySourceIntrospection:
		if err := config.ValidateHTTPURL(cfg.TexasIntrospectEndpoint, false); err != nil {
			return fmt.Errorf("invalid NAIS_TOKEN_INTROSPECTION_ENDPOINT: %w", err)
		}
	default:
		return fmt.Errorf("invalid IDENTITY_SOURCE %q: expected legacy or introspection", cfg.IdentitySource)
	}

	if cfg.AzureClientID == "" {
		return errors.New("AZURE_APP_CLIENT_ID is required")
	}
	if cfg.AzureClientSecret == "" {
		return errors.New("AZURE_APP_CLIENT_SECRET is required")
	}
	if err := config.ValidateHTTPURL(cfg.AzureTokenEndpoint, false); err != nil {
		return fmt.Errorf("invalid AZURE_OPENID_CONFIG_TOKEN_ENDPOINT: %w", err)
	}
	if err := config.ValidateHTTPURL(cfg.AzureIssuer, false); err != nil {
		return fmt.Errorf("invalid AZURE_OPENID_CONFIG_ISSUER: %w", err)
	}
	if cfg.AzureAuthorizeEndpoint == "" && cfg.AzureWellKnownURL == "" {
		return errors.New("AZURE_APP_WELL_KNOWN_URL or AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT is required")
	}
	if cfg.AzureAuthorizeEndpoint != "" {
		if err := config.ValidateHTTPURL(cfg.AzureAuthorizeEndpoint, false); err != nil {
			return fmt.Errorf("invalid AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT: %w", err)
		}
	}
	if cfg.AzureWellKnownURL != "" {
		if err := config.ValidateHTTPURL(cfg.AzureWellKnownURL, false); err != nil {
			return fmt.Errorf("invalid AZURE_APP_WELL_KNOWN_URL: %w", err)
		}
	}
	if len(cfg.CookieSecret) < 32 {
		return errors.New("cookie signing secret must be at least 32 bytes")
	}
	cfg.LockoutKey = strings.TrimSpace(cfg.LockoutKey)
	if cfg.LockoutKey != "" && len(cfg.LockoutKey) < 32 {
		return errors.New("LOCKOUT_KEY must be at least 32 bytes")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = config.NewHardenedHTTPClient(defaultOutboundTimeout)
	}
	if cfg.ProxyHeaderTimeout <= 0 {
		cfg.ProxyHeaderTimeout = defaultUpstreamTimeout
	}
	if cfg.MCPMaxInFlight <= 0 {
		cfg.MCPMaxInFlight = defaultMCPMaxInFlight
	}
	if cfg.MCPMaxInFlightPerRoute <= 0 {
		cfg.MCPMaxInFlightPerRoute = defaultMCPMaxInFlightPerRoute
	}
	if cfg.MCPMaxInFlightPerUser <= 0 {
		cfg.MCPMaxInFlightPerUser = defaultMCPMaxInFlightPerUser
	}
	if cfg.MCPMaxSSE <= 0 {
		cfg.MCPMaxSSE = defaultMCPMaxSSE
	}
	if cfg.MCPMaxSSEPerUser <= 0 {
		cfg.MCPMaxSSEPerUser = defaultMCPMaxSSEPerUser
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
	if cfg.RateLimitCap <= 0 {
		cfg.RateLimitCap = defaultRateLimitCap
	}
	if cfg.RateLimitGlobalRequests <= 0 {
		cfg.RateLimitGlobalRequests = defaultRateLimitGlobalRequests
	}
	// Below that, a full map evicts live buckets and resets throttled IPs inside the window (ipRateLimiter.Allow).
	if cfg.RateLimitCap < cfg.RateLimitGlobalRequests {
		cfg.RateLimitCap = cfg.RateLimitGlobalRequests
	}
	switch strings.ToLower(strings.TrimSpace(cfg.SessionStore)) {
	case "", "memory":
		cfg.SessionStore = "memory"
	case "postgres":
		cfg.SessionStore = "postgres"
		// Empty DATABASE_URL means the libpq PG* environment (Nais Postgres).
		if cfg.DatabaseURL != "" || os.Getenv("PGHOST") == "" {
			if err := config.ValidatePostgresURL(cfg.DatabaseURL); err != nil {
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
	if cfg.SessionAbsoluteMaxAge <= 0 {
		cfg.SessionAbsoluteMaxAge = defaultSessionAbsoluteMaxAge
	}
	if cfg.SessionIdleTimeout <= 0 {
		cfg.SessionIdleTimeout = defaultSessionIdleTimeout
	}
	if cfg.SessionAbsoluteMaxAge < cfg.SessionMaxAge {
		return errors.New("SESSION_ABSOLUTE_MAX_AGE must be at least SESSION_MAX_AGE")
	}
	if cfg.RefreshGraceWindow <= 0 {
		cfg.RefreshGraceWindow = defaultRefreshGraceWindow
	}
	if cfg.ConsentTTL <= 0 {
		cfg.ConsentTTL = defaultConsentTTL
	}
	if cfg.DCRClientTTL <= 0 {
		cfg.DCRClientTTL = defaultDCRClientTTL
	}
	if cfg.DCRClientMaxAge <= 0 {
		cfg.DCRClientMaxAge = defaultDCRClientMaxAge
	}
	if cfg.DCRClientMaxAge < cfg.DCRClientTTL {
		cfg.DCRClientMaxAge = cfg.DCRClientTTL
	}
	if cfg.DCRRateLimitGlobal <= 0 {
		cfg.DCRRateLimitGlobal = defaultDCRRateLimitGlobalRequests
	}
	if cfg.DCRClientCap <= 0 {
		cfg.DCRClientCap = defaultDCRClientCap
	}
	if cfg.DCRRateLimitRequests <= 0 {
		cfg.DCRRateLimitRequests = defaultDCRRateLimitRequests
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
	if cfg.MCPSessionUserCap <= 0 {
		cfg.MCPSessionUserCap = defaultMCPSessionUserCap
	}
	return nil
}

// stateCookieKeyInfo is the HKDF label that separates the state-cookie key from
// the RT_KEYS AES key; the AES key itself is never used for HMAC.
const stateCookieKeyInfo = "fleet-gateway entra-state cookie hmac v1"

// loadCookieSecret returns the key for the Entra state cookie. Every replica must
// share it, or a callback that lands on another pod (or after a deploy) fails.
// Order: COOKIE_SIGNING_SECRET, else an HKDF subkey of the primary RT_KEYS key,
// else a random key, which is only allowed outside Nais (local runs and tests).
// The first key signs. With RT_KEYS, every ring entry's subkey follows, so a
// cookie signed by a pod on the previous primary still verifies mid-rotation.
func loadCookieSecret(raw, rtKeys string, onNais bool) ([][]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" && strings.TrimSpace(rtKeys) != "" {
		kr, err := parseKeyRing(rtKeys)
		if err != nil {
			return nil, fmt.Errorf("invalid RT_KEYS: %w", err)
		}
		ringKeys := kr.Keys()
		keys := make([][]byte, 0, len(ringKeys))
		for _, rk := range ringKeys {
			k, err := hkdf.Key(sha256.New, rk, nil, stateCookieKeyInfo, 32)
			if err != nil {
				return nil, err
			}
			keys = append(keys, k)
		}
		return keys, nil
	}
	if trimmed == "" {
		if onNais {
			return nil, errors.New("COOKIE_SIGNING_SECRET or RT_KEYS is required on Nais: a per-pod random key breaks login across replicas")
		}
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("generate cookie secret: %w", err)
		}
		return [][]byte{secret}, nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(trimmed); err == nil && len(decoded) >= 32 {
		return [][]byte{decoded}, nil
	}
	if len(trimmed) < 32 {
		return nil, errors.New("COOKIE_SIGNING_SECRET must be at least 32 bytes")
	}
	return [][]byte{[]byte(trimmed)}, nil
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
