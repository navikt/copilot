// Package main implements mcp-dolly, an MCP server over Dolly (Nav test data) in dev-gcp.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	defaultPort       = "8080"
	defaultDollyURL   = "https://dolly-backend.ekstern.dev.nav.no"
	maxRequestBytes   = 1 << 20
	httpClientTimeout = 10 * time.Second
	serverName        = "mcp-dolly"
	serverVersion     = "2.0.0"
)

type config struct {
	Port                  string
	LogLevel              slog.Level
	IntrospectionEndpoint string
	ExchangeEndpoint      string
	GatewayClientID       string
	DollyURL              string
	DollyScope            string
	HTTPClient            *http.Client
}

func loadConfigFromEnv() (*config, error) {
	introspectionEndpoint := strings.TrimSpace(os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT"))
	if introspectionEndpoint != "" {
		if err := validateHTTPURL(introspectionEndpoint); err != nil {
			return nil, fmt.Errorf("invalid NAIS_TOKEN_INTROSPECTION_ENDPOINT: %w", err)
		}
	}

	exchangeEndpoint := strings.TrimSpace(os.Getenv("NAIS_TOKEN_EXCHANGE_ENDPOINT"))
	if exchangeEndpoint != "" {
		if err := validateHTTPURL(exchangeEndpoint); err != nil {
			return nil, fmt.Errorf("invalid NAIS_TOKEN_EXCHANGE_ENDPOINT: %w", err)
		}
	}

	dollyURL := getEnv("DOLLY_URL", defaultDollyURL)
	if err := validateHTTPURL(dollyURL); err != nil {
		return nil, fmt.Errorf("invalid DOLLY_URL: %w", err)
	}

	gatewayClientID, err := extractGatewayClientID(os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS"), os.Getenv("NAIS_CLUSTER_NAME"))
	if err != nil {
		return nil, err
	}

	return &config{
		Port:                  getEnv("PORT", defaultPort),
		LogLevel:              parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		IntrospectionEndpoint: introspectionEndpoint,
		ExchangeEndpoint:      exchangeEndpoint,
		GatewayClientID:       gatewayClientID,
		DollyURL:              strings.TrimRight(dollyURL, "/"),
		DollyScope:            strings.TrimSpace(os.Getenv("DOLLY_SCOPE")),
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
	dolly       *dollyClient
	upstreamSem chan struct{}
	quota       *userQuota
	now         func() time.Time
}

func newServer(cfg *config) *server {
	return &server{
		cfg:         cfg,
		dolly:       newDollyClient(cfg.DollyURL, cfg.HTTPClient),
		upstreamSem: make(chan struct{}, maxConcurrentUpstream),
		quota:       newUserQuota(),
		now:         time.Now,
	}
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
	if s.cfg.IntrospectionEndpoint == "" || s.cfg.ExchangeEndpoint == "" || s.cfg.GatewayClientID == "" || s.cfg.DollyScope == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *server) mcpHandler(w http.ResponseWriter, r *http.Request) {
	token, ok := extractBearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "message": "missing or malformed bearer token"})
		return
	}

	navIdent, err := s.validateToken(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, errUnauthorized):
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_token", "message": err.Error()})
		case errors.Is(err, errForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": err.Error()})
		case errors.Is(err, errNotConfigured):
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

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
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
		writeMCPError(w, http.StatusBadRequest, nil, &mcpRPCError{Code: mcpCodeParseError, Message: "Parse error"})
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
		writeMCPError(w, http.StatusBadRequest, rpcID, &mcpRPCError{Code: -32600, Message: "Invalid Request"})
		return
	}

	var env mcpEnvelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		writeMCPError(w, http.StatusBadRequest, rpcID, &mcpRPCError{Code: mcpCodeInvalidParams, Message: "Invalid params"})
		return
	}

	now := s.now()
	if !isLegacyMCPRequest(r, env) {
		s.serveModern(w, r, env, rawID, token, navIdent, now)
		return
	}
	if !mcpLegacyAllowed(now) {
		writeMCPError(w, http.StatusBadRequest, rpcID, mcpUnsupportedVersion(r.Header.Get("MCP-Protocol-Version"), now))
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
			"protocolVersion": mcpNegotiateLegacyVersion(env.Params.ProtocolVersion),
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": serverName, "version": serverVersion},
		}
	case "ping":
		// Initialize-era revisions require an empty result; serveModern still rejects ping.
		result = map[string]any{}
	case "tools/list":
		result = map[string]any{"tools": toolDefinitions()}
	case "tools/call":
		res, rpcErr := s.handleToolCall(r.Context(), token, navIdent, env.Params.Name, env.Params.Arguments)
		if rpcErr != nil {
			writeMCPError(w, http.StatusOK, rpcID, rpcErr)
			return
		}
		result = res
	default:
		writeMCPError(w, http.StatusOK, rpcID, &mcpRPCError{Code: mcpCodeMethodNotFound, Message: "Method not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(rawID),
		"result":  result,
	})
}

func (s *server) serveModern(w http.ResponseWriter, r *http.Request, env mcpEnvelope, id json.RawMessage, token, navIdent string, now time.Time) {
	var rpcID any
	if len(id) > 0 {
		rpcID = id
	}
	if e := validateModernMCPRequest(r, env, now); e != nil {
		writeMCPError(w, http.StatusBadRequest, rpcID, e)
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
		result = mcpDiscoverResult(map[string]any{"tools": map[string]any{}}, now)
	case "tools/list":
		result = map[string]any{"tools": toolDefinitions()}
		extra = map[string]any{"ttlMs": mcpDiscoverTTLMilliseconds, "cacheScope": "private"}
	case "tools/call":
		res, rpcErr := s.handleToolCall(r.Context(), token, navIdent, env.Params.Name, env.Params.Arguments)
		if rpcErr != nil {
			writeMCPError(w, http.StatusOK, rpcID, rpcErr)
			return
		}
		result = res
	default:
		writeMCPError(w, http.StatusNotFound, rpcID, &mcpRPCError{Code: mcpCodeMethodNotFound, Message: "Method not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"id":      rpcID,
		"result":  mcpModernResult(result, serverName, serverVersion, extra),
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

	if cfg.IntrospectionEndpoint == "" || cfg.ExchangeEndpoint == "" || cfg.GatewayClientID == "" || cfg.DollyScope == "" {
		slog.Warn("mcp-dolly auth or downstream config is incomplete",
			"has_introspection_endpoint", cfg.IntrospectionEndpoint != "",
			"has_exchange_endpoint", cfg.ExchangeEndpoint != "",
			"has_gateway_client_id", cfg.GatewayClientID != "",
			"has_dolly_scope", cfg.DollyScope != "",
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("Starting mcp-dolly", "port", cfg.Port)

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
}
