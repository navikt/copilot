package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/singleflight"
)

// Gateway implements the fleet-gateway OAuth 2.1 authorization server and MCP proxy.
type Gateway struct {
	cfg         *Config
	now         func() time.Time
	store       SessionStore
	rtKeys      *keyRing
	cimdCache   *ttlMap[*clientMetadata]
	limiter     *ipRateLimiter
	proxyClient *http.Client
	refreshWait time.Duration // how long a concurrent refresh waits for the winner's grace pair

	wellKnownMu         sync.RWMutex
	wellKnownGroup      singleflight.Group
	cachedAuthzEndpoint string

	TeamTokens TeamTokenSource
}

func newGateway(cfg *Config, now func() time.Time) (*Gateway, error) {
	if cfg == nil {
		return nil, errors.New("nil config")
	}
	if now == nil {
		now = time.Now
	}
	var rtKeys *keyRing
	if strings.TrimSpace(cfg.RTKeys) != "" {
		kr, err := parseKeyRing(cfg.RTKeys)
		if err != nil {
			return nil, fmt.Errorf("parse RT_KEYS: %w", err)
		}
		rtKeys = kr
	}
	if cfg.RTStorage == rtStorageClientEncrypted && rtKeys == nil {
		return nil, errors.New("RT_KEYS is required when RT_STORAGE=client-encrypted")
	}
	store, ownedStore, err := buildSessionStore(cfg, now, rtKeys)
	if err != nil {
		return nil, fmt.Errorf("initialize session store: %w", err)
	}
	teamTokens, err := newTeamTokenSource(cfg, store, now)
	if err != nil {
		if ownedStore {
			store.Close()
		}
		return nil, fmt.Errorf("initialize team token source: %w", err)
	}
	return &Gateway{
		cfg:         cfg,
		now:         now,
		store:       store,
		rtKeys:      rtKeys,
		cimdCache:   newTTLMap[*clientMetadata](cfg.CIMDCap, now),
		limiter:     newIPRateLimiter(cfg.RateLimitRequests, cfg.RateLimitGlobalRequests, cfg.RateLimitWindow, cfg.RateLimitCap, now),
		proxyClient: newHardenedProxyClient(cfg.ProxyHeaderTimeout),
		refreshWait: 5 * time.Second,
		TeamTokens:  teamTokens,
	}, nil
}

func buildSessionStore(cfg *Config, now func() time.Time, rtKeys *keyRing) (SessionStore, bool, error) {
	if cfg == nil {
		return nil, false, errors.New("nil config")
	}
	if cfg.Store != nil {
		return cfg.Store, false, nil
	}

	caps := map[string]int{
		kindPendingConsent:     cfg.PendingCap,
		kindPendingEntra:       cfg.PendingCap,
		kindPendingEntraLegacy: cfg.PendingCap,
		kindCode:               cfg.CodeCap,
		kindSession:            cfg.SessionCap,
		kindConsent:            cfg.SessionCap,
		kindMCPSession:         cfg.SessionCap,
		kindMCPSessionUser:     cfg.SessionCap,
		kindDCRClient:          cfg.DCRClientCap,
	}

	var store SessionStore
	switch strings.ToLower(strings.TrimSpace(cfg.SessionStore)) {
	case "", "memory":
		store = newMemoryStore(now, caps)
	case "postgres":
		if rtKeys == nil {
			return nil, false, errors.New("RT_KEYS is required when SESSION_STORE=postgres")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pgStore, err := newPostgresStore(ctx, cfg.DatabaseURL, now, caps)
		if err != nil {
			return nil, false, err
		}
		store = pgStore
	default:
		return nil, false, fmt.Errorf("unsupported SESSION_STORE %q", cfg.SessionStore)
	}

	if rtKeys != nil {
		store = newSealedStore(store, rtKeys)
	}
	return store, true, nil
}

func (g *Gateway) startSweeper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				g.sweepAll()
			}
		}
	}()
}

func (g *Gateway) sweepAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := g.store.Sweep(ctx); err != nil {
		slog.Warn("session store sweep failed", "error", err)
	}
	g.cimdCache.Sweep()
	g.limiter.Sweep()
}

func (g *Gateway) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /ready", g.readyHandler)
	mux.HandleFunc("GET /{$}", g.rootHandler)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", g.prmHandler)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/{rest...}", g.prmHandler)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", g.asMetadataHandler)
	mux.HandleFunc("GET /authorize", g.rateLimitMiddleware(g.authorizeHandler))
	mux.HandleFunc("POST /consent", g.consentPostHandler)
	mux.HandleFunc("GET /static/consent.css", consentCSSHandler)
	mux.HandleFunc("GET /oauth2/callback", g.callbackHandler)
	mux.HandleFunc("POST /token", g.rateLimitMiddleware(g.tokenHandler))
	mux.HandleFunc("POST /revoke", g.rateLimitMiddleware(g.revokeHandler))
	if g.cfg.DCREnabled {
		mux.HandleFunc("POST /register", g.rateLimitMiddleware(g.registerHandler))
	}
	for _, route := range g.cfg.Routes {
		mux.HandleFunc(route.Route, g.mcpHandler(route))
	}
	return g.loggingMiddleware(mux)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (g *Gateway) readyHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := g.store.Ping(ctx); err != nil {
		slog.Warn("readiness check failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (g *Gateway) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service":     "fleet-gateway",
		"description": "Nav MCP OAuth 2.1 authorization gateway",
		"endpoints": map[string]string{
			"protected_resource_metadata":   "/.well-known/oauth-protected-resource",
			"authorization_server_metadata": "/.well-known/oauth-authorization-server",
			"authorize":                     "/authorize",
			"token":                         "/token",
			"health":                        "/health",
			"ready":                         "/ready",
		},
	})
}

func (g *Gateway) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := g.now()
		wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		if r.URL.Path == "/health" || r.URL.Path == "/ready" {
			return
		}
		// Never log query strings, headers, tokens, codes, or verifiers.
		slog.Info("request completed",
			"method", sanitizeLogValue(r.Method),
			"path", sanitizeLogValue(r.URL.Path),
			"status", wrapped.statusCode,
			"duration_ms", g.now().Sub(start).Milliseconds(),
		)
	})
}

const maxLogErrorLen = 64

func sanitizeLogValue(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func truncateLogValue(s string, maxChars int) string {
	s = sanitizeLogValue(s)
	if maxChars <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == maxChars {
			return s[:i]
		}
		count++
	}
	return s
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (g *Gateway) rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r)
		if !g.limiter.Allow(ip) {
			w.Header().Set("Retry-After", "60")
			if r.URL.Path == "/token" || r.URL.Path == "/revoke" {
				writeJSON(w, http.StatusTooManyRequests, map[string]string{
					"error":             "temporarily_unavailable",
					"error_description": "rate limit exceeded",
				})
				return
			}
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// extractClientIP resolves the client IP for rate limiting.
// Only the Nais ingress is trusted: we take the LAST valid IP address in
// X-Forwarded-For (appended by the trusted ingress) so earlier client-supplied
// values cannot spoof the rate-limit key, and fall back to r.RemoteAddr when
// X-Forwarded-For is absent or unparseable.
func extractClientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	var lastToken string
	for _, hdr := range r.Header.Values("X-Forwarded-For") {
		for _, part := range strings.Split(hdr, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				lastToken = trimmed
			}
		}
	}
	if lastToken != "" {
		if ip := parseIPToken(lastToken); ip != "" {
			return ip
		}
	}
	return parseRemoteAddrIP(r.RemoteAddr)
}

func parseIPToken(token string) string {
	if host, _, err := net.SplitHostPort(token); err == nil && host != "" {
		token = host
	}
	token = strings.TrimSuffix(strings.TrimPrefix(token, "["), "]")
	if ip := net.ParseIP(token); ip != nil {
		return ip.String()
	}
	return ""
}

func parseRemoteAddrIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return host
	}
	if remoteAddr != "" {
		return remoteAddr
	}
	return "unknown"
}

type rateWindow struct {
	count   int
	resetAt time.Time
}

type ipRateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]rateWindow
	limit       int
	globalLimit int
	global      rateWindow
	window      time.Duration
	maxSize     int
	now         func() time.Time
}

func newIPRateLimiter(limit, globalLimit int, window time.Duration, maxSize int, now func() time.Time) *ipRateLimiter {
	if now == nil {
		now = time.Now
	}
	if limit <= 0 {
		limit = defaultRateLimitRequests
	}
	if globalLimit <= 0 {
		globalLimit = defaultRateLimitGlobalRequests
	}
	if window <= 0 {
		window = defaultRateLimitWindow
	}
	if maxSize <= 0 {
		maxSize = 1
	}
	return &ipRateLimiter{
		buckets:     make(map[string]rateWindow),
		limit:       limit,
		globalLimit: globalLimit,
		window:      window,
		maxSize:     maxSize,
		now:         now,
	}
}

func (l *ipRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if !now.Before(l.global.resetAt) {
		l.global = rateWindow{count: 0, resetAt: now.Add(l.window)}
	}
	if l.global.count >= l.globalLimit {
		return false
	}

	b, ok := l.buckets[ip]
	if !ok || !now.Before(b.resetAt) {
		if !ok && len(l.buckets) >= l.maxSize {
			l.sweepLocked(now)
			if len(l.buckets) >= l.maxSize {
				return false
			}
		}
		l.buckets[ip] = rateWindow{count: 1, resetAt: now.Add(l.window)}
		l.global.count++
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	l.buckets[ip] = b
	l.global.count++
	return true
}

func (l *ipRateLimiter) Sweep() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sweepLocked(l.now())
}

func (l *ipRateLimiter) sweepLocked(now time.Time) int {
	removed := 0
	for ip, b := range l.buckets {
		if !now.Before(b.resetAt) {
			delete(l.buckets, ip)
			removed++
		}
	}
	return removed
}

func main() {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		slog.Error("Configuration validation failed", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	gw, err := newGateway(cfg, nil)
	if err != nil {
		slog.Error("Failed to initialize gateway", "error", err)
		os.Exit(1)
	}
	defer gw.store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	gw.startSweeper(ctx, defaultSweepInterval)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           gw.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("Starting fleet-gateway",
		"port", cfg.Port,
		"base_url", cfg.BaseURL,
		"allowed_clients", len(cfg.AllowedClientIDs),
		"allowed_redirects", len(cfg.AllowedRedirectURIs),
		"allowed_resources", len(cfg.AllowedResources),
		"pilot_routes", len(cfg.Routes),
		"team_token_strategy", cfg.TeamTokenStrategy,
		"rt_storage", cfg.RTStorage,
	)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	}
}
