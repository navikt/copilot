package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/singleflight"
)

// Gateway implements the fleet-gateway OAuth 2.1 authorization server and MCP proxy.
type Gateway struct {
	cfg            *Config
	now            func() time.Time
	pendingConsent *ttlMap[*pendingAuth]
	pendingEntra   *ttlMap[*pendingAuth]
	codes          *ttlMap[*issuedCode]
	sessions       *ttlMap[*gatewaySession]
	cimdCache      *ttlMap[*clientMetadata]
	limiter        *ipRateLimiter
	proxyClient    *http.Client

	wellKnownMu         sync.RWMutex
	wellKnownGroup      singleflight.Group
	cachedAuthzEndpoint string

	TeamTokens TeamTokenSource
}

func newGateway(cfg *Config, now func() time.Time) *Gateway {
	if now == nil {
		now = time.Now
	}
	sessions := newTTLMap[*gatewaySession](cfg.SessionCap, now)
	teamTokens, err := newTeamTokenSource(cfg, sessions, now)
	if err != nil {
		slog.Error("Failed to initialize team token source", "error", err)
	}
	return &Gateway{
		cfg:            cfg,
		now:            now,
		pendingConsent: newTTLMap[*pendingAuth](cfg.PendingCap, now),
		pendingEntra:   newTTLMap[*pendingAuth](cfg.PendingCap, now),
		codes:          newTTLMap[*issuedCode](cfg.CodeCap, now),
		sessions:       sessions,
		cimdCache:      newTTLMap[*clientMetadata](cfg.CIMDCap, now),
		limiter:        newIPRateLimiter(cfg.RateLimitRequests, cfg.RateLimitWindow, cfg.RateLimitCap, now),
		proxyClient:    newHardenedProxyClient(cfg.ProxyHeaderTimeout),
		TeamTokens:     teamTokens,
	}
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
	g.pendingConsent.Sweep()
	g.pendingEntra.Sweep()
	g.codes.Sweep()
	g.sessions.Sweep()
	g.cimdCache.Sweep()
	g.limiter.Sweep()
}

func (g *Gateway) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /ready", readyHandler)
	mux.HandleFunc("GET /{$}", g.rootHandler)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", g.prmHandler)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/{rest...}", g.prmHandler)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", g.asMetadataHandler)
	mux.HandleFunc("GET /authorize", g.rateLimitMiddleware(g.authorizeHandler))
	mux.HandleFunc("POST /consent", g.consentPostHandler)
	mux.HandleFunc("GET /oauth2/callback", g.callbackHandler)
	mux.HandleFunc("POST /token", g.rateLimitMiddleware(g.tokenHandler))
	for _, route := range g.cfg.Routes {
		mux.HandleFunc(route.Route, g.mcpHandler(route))
	}
	return g.loggingMiddleware(mux)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func readyHandler(w http.ResponseWriter, _ *http.Request) {
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
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", g.now().Sub(start).Milliseconds(),
		)
	})
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
		ip := extractClientIP(r.RemoteAddr)
		if !g.limiter.Allow(ip) {
			w.Header().Set("Retry-After", "60")
			if r.URL.Path == "/token" {
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

func extractClientIP(remoteAddr string) string {
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
	mu      sync.Mutex
	buckets map[string]rateWindow
	limit   int
	window  time.Duration
	maxSize int
	now     func() time.Time
}

func newIPRateLimiter(limit int, window time.Duration, maxSize int, now func() time.Time) *ipRateLimiter {
	if now == nil {
		now = time.Now
	}
	if limit <= 0 {
		limit = defaultRateLimitRequests
	}
	if window <= 0 {
		window = defaultRateLimitWindow
	}
	if maxSize <= 0 {
		maxSize = 1
	}
	return &ipRateLimiter{
		buckets: make(map[string]rateWindow),
		limit:   limit,
		window:  window,
		maxSize: maxSize,
		now:     now,
	}
}

func (l *ipRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[ip]
	if !ok || !now.Before(b.resetAt) {
		if !ok && len(l.buckets) >= l.maxSize {
			l.sweepLocked(now)
			if len(l.buckets) >= l.maxSize {
				return false
			}
		}
		l.buckets[ip] = rateWindow{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	l.buckets[ip] = b
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	gw := newGateway(cfg, nil)
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
