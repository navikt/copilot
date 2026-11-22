// Package main implements mcp-kodeverk, a read-only MCP server over Nav's Felles kodeverk API.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/navikt/copilot/lib/mcpkit"
)

const (
	defaultPort        = "8080"
	defaultMetricsPort = "9090"
	defaultKodeverkURL = "http://kodeverk-api.team-rocket"
	maxRequestBytes    = 1 << 20
	httpClientTimeout  = 10 * time.Second
	serverName         = "mcp-kodeverk"
	serverVersion      = "1.0.0"
)

type config struct {
	Port                  string
	MetricsPort           string
	LogLevel              slog.Level
	IntrospectionEndpoint string
	ExchangeEndpoint      string
	GatewayClientID       string
	KodeverkURL           string
	KodeverkScope         string
	HTTPClient            *http.Client
}

func loadConfigFromEnv() (*config, error) {
	introspectionEndpoint := strings.TrimSpace(os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT"))
	if introspectionEndpoint != "" {
		if err := mcpkit.ValidateHTTPURL(introspectionEndpoint); err != nil {
			return nil, fmt.Errorf("invalid NAIS_TOKEN_INTROSPECTION_ENDPOINT: %w", err)
		}
	}

	exchangeEndpoint := strings.TrimSpace(os.Getenv("NAIS_TOKEN_EXCHANGE_ENDPOINT"))
	if exchangeEndpoint != "" {
		if err := mcpkit.ValidateHTTPURL(exchangeEndpoint); err != nil {
			return nil, fmt.Errorf("invalid NAIS_TOKEN_EXCHANGE_ENDPOINT: %w", err)
		}
	}

	kodeverkURL := getEnv("KODEVERK_URL", defaultKodeverkURL)
	if err := mcpkit.ValidateHTTPURL(kodeverkURL); err != nil {
		return nil, fmt.Errorf("invalid KODEVERK_URL: %w", err)
	}

	gatewayClientID, err := mcpkit.GatewayClientID(os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS"), os.Getenv("NAIS_CLUSTER_NAME"))
	if err != nil {
		return nil, err
	}

	return &config{
		Port:                  getEnv("PORT", defaultPort),
		MetricsPort:           getEnv("METRICS_PORT", defaultMetricsPort),
		LogLevel:              parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		IntrospectionEndpoint: introspectionEndpoint,
		ExchangeEndpoint:      exchangeEndpoint,
		GatewayClientID:       gatewayClientID,
		KodeverkURL:           strings.TrimRight(kodeverkURL, "/"),
		KodeverkScope:         strings.TrimSpace(os.Getenv("KODEVERK_SCOPE")),
		HTTPClient: &http.Client{
			Timeout: httpClientTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("redirects refused")
			},
		},
	}, nil
}

type server struct {
	cfg         *config
	kodeverk    *kodeverkClient
	upstreamSem chan struct{}
	metrics     *mcpkit.ToolMetrics
}

func newServer(cfg *config) *server {
	return &server{
		cfg:         cfg,
		kodeverk:    newKodeverkClient(cfg.KodeverkURL, cfg.HTTPClient),
		upstreamSem: make(chan struct{}, maxConcurrentUpstream),
		metrics:     mcpkit.NewToolMetrics(toolNames()),
	}
}

func toolNames() []string {
	var names []string
	for _, d := range baseToolDefinitions() {
		names = append(names, d["name"].(string))
	}
	return names
}

// callTool runs a tool and records its per-tool metrics (#2564).
func (s *server) callTool(ctx context.Context, token, name string, args json.RawMessage) (any, *mcpWireError) {
	start := time.Now()
	res, rpcErr := s.handleToolCall(ctx, token, name, args)
	m, _ := res.(map[string]any)
	isErr, _ := m["isError"].(bool)
	s.metrics.Observe(name, rpcErr != nil || isErr, time.Since(start))
	return res, rpcErr
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /ready", s.readyHandler)
	mux.HandleFunc("/", s.mcpHandler)
	return mux
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (s *server) readyHandler(w http.ResponseWriter, _ *http.Request) {
	if s.cfg.IntrospectionEndpoint == "" || s.cfg.ExchangeEndpoint == "" || s.cfg.GatewayClientID == "" || s.cfg.KodeverkScope == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *server) mcpHandler(w http.ResponseWriter, r *http.Request) {
	token, ok := mcpkit.BearerToken(r.Header.Get("Authorization"))
	if !ok {
		mcpkit.SetChallenge(w, serverName, false)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "message": "missing or malformed bearer token"})
		return
	}

	_, err := s.texas().Introspect(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, mcpkit.ErrUnauthorized):
			mcpkit.SetChallenge(w, serverName, true)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_token", "message": err.Error()})
		case errors.Is(err, mcpkit.ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": err.Error()})
		case errors.Is(err, mcpkit.ErrNotConfigured):
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "unavailable", "message": "authentication is not configured"})
		default:
			slog.Warn("texas introspection failed", "method", sanitizeLogValue(r.Method), "path", sanitizeLogValue(r.URL.Path), "error", sanitizeLogValue(err.Error()))
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "introspection_failed", "message": "failed to validate token"})
		}
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	body, tooLarge, err := mcpkit.ReadRequestBody(w, r, maxRequestBytes)
	if err != nil {
		if tooLarge {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "payload_too_large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	trimmed := bytes.TrimSpace(body)
	var rpc struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
	}
	if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(trimmed, &rpc) != nil {
		mcpkit.WriteError(w, http.StatusBadRequest, nil, &mcpkit.WireError{Code: mcpkit.CodeParseError, Message: "Parse error"})
		return
	}

	rawID := bytes.TrimSpace(rpc.ID)
	isNullID := bytes.Equal(rawID, []byte("null"))
	var rpcID any
	if len(rawID) > 0 && !isNullID {
		rpcID = json.RawMessage(rawID)
	}
	isNotification := strings.HasPrefix(rpc.Method, "notifications/")
	if rpc.Method == "" || isNullID || (!isNotification && len(rawID) == 0) {
		mcpkit.WriteError(w, http.StatusBadRequest, rpcID, &mcpkit.WireError{Code: mcpkit.CodeInvalidRequest, Message: "Invalid Request"})
		return
	}

	var env mcpkit.Envelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		mcpkit.WriteError(w, http.StatusBadRequest, rpcID, &mcpkit.WireError{Code: mcpCodeInvalidParams, Message: "Invalid params"})
		return
	}

	now := time.Now()
	if !mcpkit.IsLegacyRequest(r, env) {
		s.serveModern(w, r, env, rawID, token, now)
		return
	}
	if !mcpkit.LegacyAllowed(now) {
		mcpkit.WriteError(w, http.StatusBadRequest, rpcID, mcpkit.UnsupportedVersion(r.Header.Get("MCP-Protocol-Version"), now))
		return
	}

	if isNotification && len(rawID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var result any
	switch rpc.Method {
	case "initialize":
		result = map[string]any{
			"protocolVersion": mcpkit.NegotiateLegacyVersion(env.Params.ProtocolVersion),
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": serverName, "version": serverVersion},
		}
	case "ping":
		// Initialize-era revisions require an empty result; serveModern still rejects ping.
		result = map[string]any{}
	case "tools/list":
		result = map[string]any{"tools": toolDefinitions()}
	case "tools/call":
		res, rpcErr := s.callTool(r.Context(), token, env.Params.Name, env.Params.Arguments)
		if rpcErr != nil {
			mcpkit.WriteError(w, http.StatusOK, rpcID, rpcErr)
			return
		}
		result = res
	default:
		mcpkit.WriteError(w, http.StatusOK, rpcID, &mcpkit.WireError{Code: mcpCodeMethodNotFound, Message: "Method not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(rawID),
		"result":  result,
	})
}

func (s *server) serveModern(w http.ResponseWriter, r *http.Request, env mcpkit.Envelope, id json.RawMessage, token string, now time.Time) {
	var rpcID any
	if len(id) > 0 {
		rpcID = id
	}
	if e := mcpkit.ValidateModernRequest(r, env, now); e != nil {
		mcpkit.WriteError(w, http.StatusBadRequest, rpcID, e)
		return
	}
	// A notification has no id; with one it is a request and falls through to -32601.
	if strings.HasPrefix(env.Method, "notifications/") && len(id) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result any
	var extra map[string]any
	switch env.Method {
	case "server/discover":
		result = mcpkit.DiscoverResult(map[string]any{"tools": map[string]any{}}, now)
	case "tools/list":
		result = map[string]any{"tools": toolDefinitions()}
		extra = map[string]any{"ttlMs": mcpkit.DiscoverTTLMilliseconds, "cacheScope": "private"}
	case "tools/call":
		res, rpcErr := s.callTool(r.Context(), token, env.Params.Name, env.Params.Arguments)
		if rpcErr != nil {
			mcpkit.WriteError(w, http.StatusOK, rpcID, rpcErr)
			return
		}
		result = res
	default:
		mcpkit.WriteError(w, http.StatusNotFound, rpcID, &mcpkit.WireError{Code: mcpCodeMethodNotFound, Message: "Method not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"id":      rpcID,
		"result":  mcpkit.ModernResult(result, serverName, serverVersion, extra),
	})
}

func sanitizeLogValue(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
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

func main() {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})))

	if cfg.IntrospectionEndpoint == "" || cfg.ExchangeEndpoint == "" || cfg.GatewayClientID == "" || cfg.KodeverkScope == "" {
		slog.Warn("mcp-kodeverk auth or downstream config is incomplete",
			"has_introspection_endpoint", cfg.IntrospectionEndpoint != "",
			"has_exchange_endpoint", cfg.ExchangeEndpoint != "",
			"has_gateway_client_id", cfg.GatewayClientID != "",
			"has_kodeverk_scope", cfg.KodeverkScope != "",
		)
	}

	srv := newServer(cfg)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// /metrics has its own port so the MCP port serves only MCP.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("GET /metrics", srv.metrics)
	metricsServer := &http.Server{
		Addr:              ":" + cfg.MetricsPort,
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("metrics server failed", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("Starting mcp-kodeverk", "port", cfg.Port)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown failed", "error", err)
	}
	_ = metricsServer.Shutdown(shutdownCtx)
}
