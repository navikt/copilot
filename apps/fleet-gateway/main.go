package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/navikt/copilot/fleet-gateway/internal/audit"
	"github.com/navikt/copilot/fleet-gateway/internal/config"
	"github.com/navikt/copilot/fleet-gateway/internal/httpx"
	"github.com/navikt/copilot/fleet-gateway/internal/inflight"
	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
	"github.com/navikt/copilot/fleet-gateway/internal/oauth/entra"
	"github.com/navikt/copilot/fleet-gateway/internal/oauth/server"
	"github.com/navikt/copilot/fleet-gateway/internal/policy"
	"github.com/navikt/copilot/fleet-gateway/internal/proxy"
	"github.com/navikt/copilot/fleet-gateway/internal/ratelimit"
	"github.com/navikt/copilot/fleet-gateway/internal/routing"
	"github.com/navikt/copilot/fleet-gateway/internal/ttlmap"
)

// Gateway implements the fleet-gateway OAuth 2.1 authorization server and MCP proxy.
type Gateway struct {
	cfg *Config
	// riskCaller identifies the tools/call caller for the risk check; nil means a
	// logged-in human. Tests set it until agent profiles are identified (#2964).
	riskCaller    func(*http.Request) policy.Caller
	now           func() time.Time
	store         SessionStore
	rtKeys        *keyRing
	lockoutKey    []byte // HMAC key for lockout records (LOCKOUT_KEY); empty disables lockout checks
	mcpUserKey    []byte // HMAC key for kindMCPSessionUser counters
	cimdCache     *ttlmap.Map[server.CachedCIMD]
	limiters      map[string]*ratelimit.Limiter
	rateLimitLog  *ratelimit.LogSampler
	proxyClient   *http.Client
	refreshWait   time.Duration    // how long a concurrent refresh waits for the winner's grace pair
	inFlight      inflight.Slots   // buffered MCP calls; bounds memory (see README, Memory and limits)
	userInFlight  *inflight.PerKey // concurrent buffered MCP calls per user on this pod
	sseStreams    inflight.Slots   // open SSE response streams; they buffer nothing, so get their own cap
	userSSE       *inflight.PerKey // concurrent open SSE response streams per user on this pod
	routeInFlight *inflight.PerKey // concurrent buffered MCP calls per route on this pod, keyed by route path
	routeTable    atomic.Pointer[routing.Table]
	draining      atomic.Bool // set on SIGTERM so /ready drops the pod before Shutdown

	entra *entra.Client

	TeamTokens proxy.TeamTokenSource
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
	table := routing.NewTable(cfg.Routes)
	if cfg.RoutesSource == routesSourceFleetControl {
		// Fail closed (#2797): no Git routes, ever. The file only pins the primary resource.
		table = &routing.Table{PrimaryResource: table.PrimaryResource}
	}
	teamTokens, err := newTeamTokenSource(cfg, store, now)
	if err != nil {
		if ownedStore {
			store.Close()
		}
		return nil, fmt.Errorf("initialize team token source: %w", err)
	}
	limiters := make(map[string]*ratelimit.Limiter, len(metrics.RateLimitedEndpoints))
	for _, ep := range metrics.RateLimitedEndpoints {
		limiters[ep] = ratelimit.New(cfg.RateLimitRequests, cfg.RateLimitGlobalRequests, cfg.RateLimitWindow, cfg.RateLimitCap, now)
	}
	gw := &Gateway{
		cfg:          cfg,
		now:          now,
		store:        store,
		rtKeys:       rtKeys,
		lockoutKey:   lockoutKey,
		mcpUserKey:   mcpUserKey,
		cimdCache:    ttlmap.New[server.CachedCIMD](cfg.CIMDCap, now),
		limiters:     limiters,
		rateLimitLog: ratelimit.NewLogSampler(ratelimit.DefaultLogWindow, ratelimit.DefaultLogCap, now),
		proxyClient:  proxy.NewHardenedClient(cfg.ProxyHeaderTimeout),
		refreshWait:  5 * time.Second,
		TeamTokens:   teamTokens,
		entra: &entra.Client{
			HTTPClient:      cfg.HTTPClient,
			ClientID:        cfg.AzureClientID,
			ClientSecret:    cfg.AzureClientSecret,
			Scope:           cfg.EntraScope,
			RedirectURI:     cfg.BaseURL + "/oauth2/callback",
			TokenEndpoint:   cfg.AzureTokenEndpoint,
			AuthorizeURL:    cfg.AzureAuthorizeEndpoint,
			WellKnownURL:    cfg.AzureWellKnownURL,
			OutboundTimeout: defaultOutboundTimeout,
		},
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
	mux.HandleFunc("GET /authorize", g.rateLimitMiddleware(metrics.EndpointAuthorize, g.limiters[metrics.EndpointAuthorize], g.authorizeHandler))
	mux.HandleFunc("POST /consent", g.rateLimitMiddleware(metrics.EndpointConsent, g.limiters[metrics.EndpointConsent], g.consentPostHandler))
	mux.HandleFunc("GET /static/consent.css", server.ConsentCSS)
	mux.HandleFunc("GET /oauth2/callback", g.callbackHandler)
	mux.HandleFunc("POST /token", g.rateLimitMiddleware(metrics.EndpointToken, g.limiters[metrics.EndpointToken], g.tokenHandler))
	mux.HandleFunc("POST /revoke", g.rateLimitMiddleware(metrics.EndpointRevoke, g.limiters[metrics.EndpointRevoke], g.revokeHandler))
	if g.cfg.DCREnabled {
		mux.HandleFunc("POST /register", g.rateLimitMiddleware(metrics.EndpointRegister, g.limiters[metrics.EndpointRegister], g.registerHandler))
	}
	// Shared by all MCP routes.
	g.inFlight = make(inflight.Slots, g.cfg.MCPMaxInFlight)
	g.userInFlight = inflight.NewPerKey(min(g.cfg.MCPMaxInFlightPerUser, g.cfg.MCPMaxInFlight))
	g.sseStreams = make(inflight.Slots, g.cfg.MCPMaxSSE)
	g.userSSE = inflight.NewPerKey(g.cfg.MCPMaxSSEPerUser)
	g.routeInFlight = inflight.NewPerKey(min(g.cfg.MCPMaxInFlightPerRoute, cap(g.inFlight)))
	// Two method-less, disjoint patterns, as before routing.Table: no /mcp/ subtree,
	// so /mcp, /mcp/ and trailing slashes stay plain 404s and are never redirected.
	mux.HandleFunc("/mcp/{ns}/{name}", g.routeDispatch)
	mux.HandleFunc("/mcp/{name}", g.oldRoutePathHandler)
	return g.loggingMiddleware(mux)
}

// routeDispatch serves /mcp/<namespace>/<name> from the current routing.Table.
func (g *Gateway) routeDispatch(w http.ResponseWriter, r *http.Request) {
	table := g.table() // one load: a swap between lookup and check must not turn 503 into 404
	route, ok := table.RoutesByPath["/mcp/"+r.PathValue("ns")+"/"+r.PathValue("name")]
	if !ok && g.cfg.RoutesSource == routesSourceFleetControl && len(table.Routes) == 0 {
		httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable", "error_description": "Route table not loaded yet. Try again shortly."})
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	afterRouteLookup()
	g.mcpHandler(route).ServeHTTP(w, r)
}

// mcpHandler builds the proxy handler per request, so it reads g.cfg as it is now.
func (g *Gateway) mcpHandler(route routing.Route) http.HandlerFunc {
	h := &proxy.Handler{
		Cfg: proxy.Config{
			BaseURL:              g.cfg.BaseURL,
			GatewayDisabled:      g.cfg.GatewayDisabled,
			RiskEnforcement:      g.cfg.RiskEnforcement,
			HTTPClient:           g.cfg.HTTPClient,
			ProxyHeaderTimeout:   g.cfg.ProxyHeaderTimeout,
			SSEMaxDuration:       g.cfg.SSEMaxDuration,
			SSEIdleTimeout:       g.cfg.SSEIdleTimeout,
			SSEMaxBytes:          g.cfg.SSEMaxBytes,
			MaxResponseBodyBytes: g.cfg.MaxResponseBodyBytes,
			MCPSessionUserCap:    g.cfg.MCPSessionUserCap,
			SessionMaxAge:        g.cfg.SessionMaxAge,
		},
		Now:               g.now,
		Store:             g.store,
		GetSession:        g.getSession,
		MCPSessionUserKey: g.mcpSessionUserKey,
		Client:            g.proxyClient,
		InFlight:          g.inFlight,
		UserInFlight:      g.userInFlight,
		SSEStreams:        g.sseStreams,
		UserSSE:           g.userSSE,
		RouteInFlight:     g.routeInFlight,
		TeamTokens:        g.TeamTokens,
		RiskCaller:        g.riskCaller,
	}
	return h.MCP(route)
}

// oldRoutePathHandler answers the retired /mcp/<name> paths with a 404 that names
// the new /mcp/<namespace>/<name> URL. It does not proxy: there is no alias (#2342 §3.3).
func (g *Gateway) oldRoutePathHandler(w http.ResponseWriter, r *http.Request) {
	body := map[string]string{
		"error":             "not_found",
		"error_description": "MCP routes moved to /mcp/<namespace>/<name>. Update your MCP client config and log in again.",
	}
	// Two namespaces may share a route name: only hint when the match is unique.
	var match *routing.Route
	matches := 0
	routes := g.table().Routes
	for i, route := range routes {
		if strings.HasSuffix(route.Route, "/"+r.PathValue("name")) {
			match = &routes[i]
			matches++
		}
	}
	if matches == 1 {
		body["new_url"] = match.FullRouteID
		body["error_description"] = "This route moved to " + match.FullRouteID + ". Update your MCP client config and log in again."
	}
	httpx.WriteJSON(w, http.StatusNotFound, body)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (g *Gateway) readyHandler(w http.ResponseWriter, r *http.Request) {
	if g.draining.Load() {
		httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "shutting_down"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	// The DB is not a readiness condition (#2893): routing runs from the in-memory
	// snapshot, so pods stay in rotation and the alert on metrics.StoreUp reports the outage.
	if err := g.store.Ping(ctx); err != nil {
		slog.Warn("session store ping failed", "error", err)
		metrics.StoreUp.Set(0)
	} else {
		metrics.StoreUp.Set(1)
	}
	if g.awaitingSnapshot() {
		httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "no_route_snapshot"})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (g *Gateway) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
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
		wrapped := &httpx.StatusWriter{ResponseWriter: w, Status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		if r.URL.Path == "/health" || r.URL.Path == "/ready" {
			return
		}
		metrics.ObserveHTTPRequest(wrapped.Status, wrapped.Reason)
		// Never log query strings, headers, tokens, codes, or verifiers.
		slog.Info("request completed",
			"method", sanitizeLogValue(r.Method),
			"path", sanitizeLogValue(r.URL.Path),
			"status", sanitizeLogValue(strconv.Itoa(wrapped.Status)),
			"duration_ms", g.now().Sub(start).Milliseconds(),
		)
	})
}

func sanitizeLogValue(s string) string { return audit.Sanitize(s) }

const serverReadTimeout = 10 * time.Second

// mcpWriteTimeout is the server's WriteTimeout. Go starts it when the request
// headers are read, so it must cover the slowest non-SSE MCP call end to end:
// the request body (read), one team-token exchange (token), the upstream's
// headers and then its body (upstream each), plus a margin for store lookups.
// SSE clears its own deadline.
func mcpWriteTimeout(read, token, upstream, margin time.Duration) time.Duration {
	return read + token + 2*upstream + margin
}

// maxShutdownDrain stays under terminationGracePeriodSeconds (180, the Nais
// maximum) in .nais/app.yaml, leaving room for the preStop sleep.
const maxShutdownDrain = 170 * time.Second

// shutdownDrain is how long Shutdown waits for in-flight calls: the longest a
// non-SSE call may run (WriteTimeout), capped by the pod's grace period.
func shutdownDrain(writeTimeout time.Duration) time.Duration {
	return min(writeTimeout, maxShutdownDrain)
}

func (g *Gateway) rateLimitMiddleware(endpoint string, limiter *ratelimit.Limiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := ratelimit.ClientIP(r)
		if limiter != nil && !limiter.Allow(ip) {
			metrics.ObserveRateLimited(endpoint)
			if g.rateLimitLog == nil || g.rateLimitLog.Allow(endpoint+"\x00"+ip) {
				slog.Warn("rate limited", "endpoint", endpoint)
			}
			w.Header().Set("Retry-After", "60")
			if r.URL.Path == "/token" || r.URL.Path == "/revoke" || r.URL.Path == "/register" {
				httpx.WriteJSON(w, http.StatusTooManyRequests, map[string]string{
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
	metrics.RoutesLoaded.Set(float64(len(gw.table().Routes)))
	gw.startRouteSync(ctx, fleetControlSyncInterval)
	gw.startShadowSync(ctx, fleetControlSyncInterval)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           gw.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      mcpWriteTimeout(serverReadTimeout, defaultOutboundTimeout, cfg.ProxyHeaderTimeout, 5*time.Second),
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    proxy.MaxRequestHeaderBytes,
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

	metrics.Init(gw.table().Routes)
	metrics.SetRoutesSource(cfg.RoutesSource)
	metrics := metrics.Server(":" + config.Get("METRICS_PORT", "9090"))
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
	gw.draining.Store(true)
	drain := shutdownDrain(server.WriteTimeout)
	slog.Info("Shutting down gracefully...", "drain", drain)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), drain)
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
