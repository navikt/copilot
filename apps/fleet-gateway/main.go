package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/navikt/copilot/fleet-gateway/internal/config"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/navikt/copilot/fleet-gateway/internal/audit"
)

// Gateway implements the fleet-gateway OAuth 2.1 authorization server and MCP proxy.
type Gateway struct {
	cfg           *Config
	now           func() time.Time
	store         SessionStore
	rtKeys        *keyRing
	lockoutKey    []byte // HMAC key for lockout records (LOCKOUT_KEY); empty disables lockout checks
	mcpUserKey    []byte // HMAC key for kindMCPSessionUser counters
	cimdCache     *ttlMap[cachedCIMD]
	limiters      map[string]*ipRateLimiter
	rateLimitLog  *rateLimitLogSampler
	proxyClient   *http.Client
	refreshWait   time.Duration // how long a concurrent refresh waits for the winner's grace pair
	inFlight      slots         // buffered MCP calls; bounds memory (see README, Memory and limits)
	userInFlight  *userSlots    // concurrent buffered MCP calls per user on this pod
	sseStreams    slots         // open SSE response streams; they buffer nothing, so get their own cap
	userSSE       *userSlots    // concurrent open SSE response streams per user on this pod
	routeInFlight *userSlots    // concurrent buffered MCP calls per route on this pod, keyed by route path
	routeTable    atomic.Pointer[RouteTable]

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
	var lockoutKey []byte
	if trimmed := strings.TrimSpace(cfg.LockoutKey); trimmed != "" {
		if len(trimmed) < 32 {
			return nil, errors.New("LOCKOUT_KEY must be at least 32 bytes")
		}
		lockoutKey = []byte(trimmed)
	}
	mcpUserKey, err := deriveMCPUserKey(lockoutKey, rtKeys)
	if err != nil {
		return nil, fmt.Errorf("derive mcp user key: %w", err)
	}
	store, ownedStore, err := buildSessionStore(cfg, now, rtKeys)
	if err != nil {
		return nil, fmt.Errorf("initialize session store: %w", err)
	}
	if err = checkLockoutMarker(store, lockoutKey); err != nil {
		if ownedStore {
			store.Close()
		}
		return nil, err
	}
	table := newRouteTable(cfg)
	if cfg.RoutesSource == routesSourceFleetControl {
		// Fail closed (#2797): no Git routes, ever. The file only pins the primary resource.
		table = &RouteTable{PrimaryResource: table.PrimaryResource}
	}
	teamTokens, err := newTeamTokenSource(cfg, store, now)
	if err != nil {
		if ownedStore {
			store.Close()
		}
		return nil, fmt.Errorf("initialize team token source: %w", err)
	}
	limiters := make(map[string]*ipRateLimiter, len(rateLimitedEndpoints))
	for _, ep := range rateLimitedEndpoints {
		limiters[ep] = newIPRateLimiter(cfg.RateLimitRequests, cfg.RateLimitGlobalRequests, cfg.RateLimitWindow, cfg.RateLimitCap, now)
	}
	gw := &Gateway{
		cfg:          cfg,
		now:          now,
		store:        store,
		rtKeys:       rtKeys,
		lockoutKey:   lockoutKey,
		mcpUserKey:   mcpUserKey,
		cimdCache:    newTTLMap[cachedCIMD](cfg.CIMDCap, now),
		limiters:     limiters,
		rateLimitLog: newRateLimitLogSampler(defaultRateLimitLogWindow, defaultRateLimitLogCap, now),
		proxyClient:  newHardenedProxyClient(cfg.ProxyHeaderTimeout),
		refreshWait:  5 * time.Second,
		TeamTokens:   teamTokens,
	}
	gw.routeTable.Store(table)
	return gw, nil
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
	g.drainLockoutAudit(ctx)
	g.cimdCache.Sweep()
	for _, l := range g.limiters {
		l.Sweep()
	}
	if g.rateLimitLog != nil {
		g.rateLimitLog.Sweep()
	}
}

func (g *Gateway) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /ready", g.readyHandler)
	mux.HandleFunc("GET /{$}", g.rootHandler)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", g.prmHandler)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/{rest...}", g.prmHandler)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", g.asMetadataHandler)
	mux.HandleFunc("GET /authorize", g.rateLimitMiddleware(rateLimitEndpointAuthorize, g.limiters[rateLimitEndpointAuthorize], g.authorizeHandler))
	mux.HandleFunc("POST /consent", g.rateLimitMiddleware(rateLimitEndpointConsent, g.limiters[rateLimitEndpointConsent], g.consentPostHandler))
	mux.HandleFunc("GET /static/consent.css", consentCSSHandler)
	mux.HandleFunc("GET /oauth2/callback", g.callbackHandler)
	mux.HandleFunc("POST /token", g.rateLimitMiddleware(rateLimitEndpointToken, g.limiters[rateLimitEndpointToken], g.tokenHandler))
	mux.HandleFunc("POST /revoke", g.rateLimitMiddleware(rateLimitEndpointRevoke, g.limiters[rateLimitEndpointRevoke], g.revokeHandler))
	if g.cfg.DCREnabled {
		mux.HandleFunc("POST /register", g.rateLimitMiddleware(rateLimitEndpointRegister, g.limiters[rateLimitEndpointRegister], g.registerHandler))
	}
	// Shared by all MCP routes.
	g.inFlight = make(slots, g.cfg.MCPMaxInFlight)
	g.userInFlight = &userSlots{max: min(g.cfg.MCPMaxInFlightPerUser, g.cfg.MCPMaxInFlight)}
	g.sseStreams = make(slots, g.cfg.MCPMaxSSE)
	g.userSSE = &userSlots{max: g.cfg.MCPMaxSSEPerUser}
	g.routeInFlight = &userSlots{max: min(g.cfg.MCPMaxInFlightPerRoute, cap(g.inFlight))}
	// A method-less pattern and no /mcp/ subtree, so /mcp, /mcp/<name> and
	// trailing slashes stay plain 404s and are never redirected.
	mux.HandleFunc("/mcp/{ns}/{name}", g.routeDispatch)
	return g.loggingMiddleware(mux)
}

// routeDispatch serves /mcp/<namespace>/<name> from the current RouteTable.
func (g *Gateway) routeDispatch(w http.ResponseWriter, r *http.Request) {
	table := g.table() // one load: a swap between lookup and check must not turn 503 into 404
	route, ok := table.RoutesByPath["/mcp/"+r.PathValue("ns")+"/"+r.PathValue("name")]
	if !ok && g.cfg.RoutesSource == routesSourceFleetControl && len(table.Routes) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable", "error_description": "Route table not loaded yet. Try again shortly."})
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	afterRouteLookup()
	g.mcpHandler(route).ServeHTTP(w, r)
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
	if g.awaitingSnapshot() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "no_route_snapshot"})
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
		observeHTTPRequest(wrapped.statusCode, wrapped.reason)
		// Never log query strings, headers, tokens, codes, or verifiers.
		slog.Info("request completed",
			"method", sanitizeLogValue(r.Method),
			"path", sanitizeLogValue(r.URL.Path),
			"status", sanitizeLogValue(strconv.Itoa(wrapped.statusCode)),
			"duration_ms", g.now().Sub(start).Milliseconds(),
		)
	})
}

const maxLogErrorLen = 64

// maxLogClientIDLen bounds a client-supplied client_id (CIMD URL or DCR id) in audit lines.
const maxLogClientIDLen = 256

func sanitizeLogValue(s string) string { return audit.Sanitize(s) }

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

const serverReadTimeout = 10 * time.Second

// mcpWriteTimeout is the server's WriteTimeout. Go starts it when the request
// headers are read, so it must cover the slowest non-SSE MCP call end to end:
// the request body (read), one team-token exchange (token), the upstream's
// headers and then its body (upstream each), plus a margin for store lookups.
// SSE clears its own deadline.
func mcpWriteTimeout(read, token, upstream, margin time.Duration) time.Duration {
	return read + token + 2*upstream + margin
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
	reason     string // set by markShed; labels deliberate 5xx in metrics
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (g *Gateway) rateLimitMiddleware(endpoint string, limiter *ipRateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r)
		if limiter != nil && !limiter.Allow(ip) {
			observeRateLimited(endpoint)
			if g.rateLimitLog == nil || g.rateLimitLog.Allow(endpoint+"\x00"+ip) {
				slog.Warn("rate limited", "endpoint", endpoint)
			}
			w.Header().Set("Retry-After", "60")
			if r.URL.Path == "/token" || r.URL.Path == "/revoke" || r.URL.Path == "/register" {
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
				l.evictOldestLocked()
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

func (l *ipRateLimiter) evictOldestLocked() {
	var oldestIP string
	var oldestReset time.Time
	first := true
	for ip, b := range l.buckets {
		if first || b.resetAt.Before(oldestReset) {
			oldestIP = ip
			oldestReset = b.resetAt
			first = false
		}
	}
	if !first {
		delete(l.buckets, oldestIP)
	}
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "lockout" {
		os.Exit(runLockout(os.Args[2:]))
	}

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
	routesLoaded.Set(float64(len(gw.table().Routes)))
	gw.startRouteSync(ctx, fleetControlSyncInterval)
	gw.startShadowSync(ctx, fleetControlSyncInterval)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           gw.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      mcpWriteTimeout(serverReadTimeout, defaultOutboundTimeout, cfg.ProxyHeaderTimeout, 5*time.Second),
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    maxRequestHeaderBytes,
	}

	slog.Info("Starting fleet-gateway",
		"port", cfg.Port,
		"base_url", cfg.BaseURL,
		"allowed_clients", len(cfg.AllowedClientIDs),
		"allowed_redirects", len(cfg.AllowedRedirectURIs),
		"allowed_resources", len(gw.table().AllowedResources),
		"pilot_routes", len(gw.table().Routes),
		"team_token_strategy", cfg.TeamTokenStrategy,
		"rt_storage", cfg.RTStorage,
		"identity_source", cfg.IdentitySource,
		"routes_source", cfg.RoutesSource,
	)

	initMetrics(gw.table().Routes)
	metrics := metricsServer(":" + config.Get("METRICS_PORT", "9090"))
	go func() {
		if err := metrics.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

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
	_ = metrics.Shutdown(shutdownCtx)
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	}
}

func parseLockoutArgs(args []string) (undo bool, operator, navIdent string, err error) {
	fs := flag.NewFlagSet("lockout", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&undo, "undo", false, "lift lockout for new logins")
	fs.StringVar(&operator, "operator", "", "operator NAVident")
	if err := fs.Parse(args); err != nil {
		return false, "", "", errors.New("usage: fleet-gateway lockout [--undo] --operator <ident> <NAVident>")
	}
	operator = strings.TrimSpace(operator)
	if operator == "" {
		return false, "", "", errors.New("lockout: --operator is required")
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return false, "", "", errors.New("usage: fleet-gateway lockout [--undo] --operator <ident> <NAVident>")
	}
	navIdent = strings.ToUpper(strings.TrimSpace(rest[0]))
	if navIdent == "" {
		return false, "", "", errors.New("lockout: NAVident must not be empty")
	}
	if strings.HasPrefix(navIdent, "-") {
		return false, "", "", errors.New("usage: fleet-gateway lockout [--undo] --operator <ident> <NAVident>")
	}
	return undo, operator, navIdent, nil
}

func checkLockoutConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("lockout: nil config")
	}
	if strings.ToLower(strings.TrimSpace(cfg.SessionStore)) != "postgres" {
		return errors.New("lockout: SESSION_STORE must be postgres; with the memory store the command cannot reach the running gateway")
	}
	if strings.TrimSpace(cfg.LockoutKey) == "" {
		return errors.New("lockout: LOCKOUT_KEY is not set")
	}
	return nil
}

func runLockout(args []string) int {
	undo, operator, navIdent, err := parseLockoutArgs(args)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}

	cfg, err := loadConfigFromEnv()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lockout: configuration validation failed: %v\n", err)
		return 1
	}
	if err = checkLockoutConfig(cfg); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	gw, err := newGateway(cfg, nil)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lockout: initialize gateway: %v\n", err)
		return 1
	}
	defer gw.store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if undo {
		if err = gw.undoLockout(ctx, navIdent, operator); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "lockout: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(os.Stdout, "lockout undo recorded user_key=%s; the gateway logs the audit line within 30s\n", gw.userKey(navIdent)) //nolint:gosec // userKey is hex-encoded HMAC printed to CLI stdout
		return 0
	}

	if _, err := gw.lockOutUser(ctx, navIdent, operator); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "lockout: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(os.Stdout, "lockout recorded user_key=%s; the gateway logs the audit line within 30s\n", gw.userKey(navIdent)) //nolint:gosec // userKey is hex-encoded HMAC printed to CLI stdout
	return 0
}
