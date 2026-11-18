// Package main implements fleet-echo, a dev-only MCP test upstream for fleet-gateway.
package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"
)

const (
	defaultPort        = "8080"
	maxBearerLen       = 16 * 1024
	maxRequestBytes    = 1 << 20
	maxTexasRespBytes  = 1 << 20
	texasClientTimeout = 5 * time.Second
)

type config struct {
	Port                  string
	LogLevel              slog.Level
	IntrospectionEndpoint string
	GatewayClientID       string
	HTTPClient            *http.Client
}

func loadConfigFromEnv() (*config, error) {
	endpoint := strings.TrimSpace(os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT"))
	if endpoint != "" {
		if err := validateHTTPURL(endpoint); err != nil {
			return nil, fmt.Errorf("invalid NAIS_TOKEN_INTROSPECTION_ENDPOINT: %w", err)
		}
	}

	gatewayClientID, err := extractGatewayClientID(os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS"))
	if err != nil {
		return nil, err
	}

	return &config{
		Port:                  getEnv("PORT", defaultPort),
		LogLevel:              parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		IntrospectionEndpoint: endpoint,
		GatewayClientID:       gatewayClientID,
		HTTPClient: &http.Client{
			Timeout: texasClientTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("redirects refused")
			},
		},
	}, nil
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("missing host")
	}
	return nil
}

// extractGatewayClientID extracts fleet-gateway's Entra client ID from
// AZURE_APP_PRE_AUTHORIZED_APPS, failing closed on malformed JSON or multiple matches.
func extractGatewayClientID(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	var apps []struct {
		Name     string `json:"name"`
		ClientID string `json:"clientId"`
	}
	if err := json.Unmarshal([]byte(raw), &apps); err != nil {
		return "", fmt.Errorf("parse AZURE_APP_PRE_AUTHORIZED_APPS: %w", err)
	}
	var matches []string
	for _, app := range apps {
		parts := strings.Split(app.Name, ":")
		isGateway := (len(parts) == 3 && parts[1] == "copilot" && parts[2] == "fleet-gateway") ||
			(len(parts) == 1 && parts[0] == "fleet-gateway")
		if isGateway && app.ClientID != "" {
			matches = append(matches, app.ClientID)
		}
	}
	if len(matches) != 1 {
		return "", nil
	}
	return matches[0], nil
}

type introspectionClaims struct {
	Active            bool     `json:"active"`
	Idtyp             string   `json:"idtyp,omitempty"`
	Roles             []string `json:"roles,omitempty"`
	NAVident          string   `json:"NAVident,omitempty"`
	Azp               string   `json:"azp,omitempty"`
	Aud               any      `json:"aud,omitempty"`
	Sub               string   `json:"sub,omitempty"`
	Oid               string   `json:"oid,omitempty"`
	PreferredUsername string   `json:"preferred_username,omitempty"`
	Name              string   `json:"name,omitempty"`
	Scp               string   `json:"scp,omitempty"`
	Groups            []string `json:"groups,omitempty"`
	Tid               string   `json:"tid,omitempty"`
	Ver               string   `json:"ver,omitempty"`
	Iat               int64    `json:"iat,omitempty"`
	Exp               int64    `json:"exp,omitempty"`
}

type echoClaims struct {
	NAVident          string   `json:"NAVident"`
	PreferredUsername string   `json:"preferred_username,omitempty"`
	Name              string   `json:"name,omitempty"`
	Sub               string   `json:"sub,omitempty"`
	Oid               string   `json:"oid,omitempty"`
	Azp               string   `json:"azp"`
	Aud               any      `json:"aud,omitempty"`
	Scp               string   `json:"scp,omitempty"`
	Idtyp             string   `json:"idtyp,omitempty"`
	Groups            []string `json:"groups,omitempty"`
	Tid               string   `json:"tid,omitempty"`
	Ver               string   `json:"ver,omitempty"`
	Iat               int64    `json:"iat,omitempty"`
	Exp               int64    `json:"exp,omitempty"`
}

var (
	errUnauthorized  = errors.New("invalid or expired bearer token")
	errForbidden     = errors.New("token claims rejected")
	errNotConfigured = errors.New("introspection is not configured")
)

type server struct {
	cfg *config
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /ready", readyHandler)
	mux.HandleFunc("/", s.echoHandler)
	return mux
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func readyHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *server) echoHandler(w http.ResponseWriter, r *http.Request) {
	token, ok := extractBearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "message": "missing or malformed bearer token"})
		return
	}

	claims, err := s.validateToken(r.Context(), token)
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
		// MCP 2026-07-28 has no GET stream (SHOULD 405); read claims via the whoami tool.
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

	var rpc struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(trimmed, &rpc) != nil || rpc.Method == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"service": "fleet-echo",
			"claims":  claims,
		})
		return
	}

	var env mcpEnvelope
	_ = json.Unmarshal(trimmed, &env)
	now := time.Now()
	if !isLegacyMCPRequest(r, env) {
		serveModernEcho(w, r, env, json.RawMessage(bytes.TrimSpace(rpc.ID)), claims, now)
		return
	}
	if !mcpLegacyAllowed(now) {
		var id any // null only when the request carried no id
		if v := bytes.TrimSpace(rpc.ID); len(v) > 0 {
			id = json.RawMessage(v)
		}
		writeMCPError(w, http.StatusBadRequest, id, mcpUnsupportedVersion(r.Header.Get("MCP-Protocol-Version"), now))
		return
	}

	isNotification := len(bytes.TrimSpace(rpc.ID)) == 0 || bytes.Equal(bytes.TrimSpace(rpc.ID), []byte("null"))
	if strings.HasPrefix(rpc.Method, "notifications/") && isNotification {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var result any
	switch rpc.Method {
	case "initialize":
		result = map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": "fleet-echo", "version": "1.0.0"},
			"claims":          claims,
		}
	case "tools/list":
		result = map[string]any{
			"tools": []map[string]any{
				{
					"name":        "whoami",
					"description": "Return validated Entra claims from Texas introspection",
					"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
					"annotations": whoamiAnnotations,
				},
			},
		}
	case "tools/call":
		claimsJSON, _ := json.Marshal(claims)
		result = map[string]any{
			"content": []map[string]string{
				{"type": "text", "text": string(claimsJSON)},
			},
			"claims": claims,
		}
	default:
		result = map[string]any{
			"service": "fleet-echo",
			"claims":  claims,
		}
	}

	if isNotification {
		writeJSON(w, http.StatusOK, result)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(bytes.TrimSpace(rpc.ID)),
		"result":  result,
	})
}

func (s *server) validateToken(ctx context.Context, token string) (*echoClaims, error) {
	if s.cfg.IntrospectionEndpoint == "" || s.cfg.GatewayClientID == "" {
		return nil, errNotConfigured
	}

	payload, err := json.Marshal(struct {
		IdentityProvider string `json:"identity_provider"`
		Token            string `json:"token"`
	}{
		IdentityProvider: "entra_id",
		Token:            token,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.IntrospectionEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.cfg.HTTPClient.Do(req) //nolint:gosec // endpoint validated at startup
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("texas returned status %d", resp.StatusCode)
	}

	var raw introspectionClaims
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxTexasRespBytes)).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode texas response: %w", err)
	}

	if !raw.Active {
		return nil, errUnauthorized
	}
	if strings.EqualFold(raw.Idtyp, "app") || slices.Contains(raw.Roles, "access_as_application") {
		return nil, errForbidden
	}
	navIdent := strings.TrimSpace(raw.NAVident)
	if navIdent == "" {
		return nil, errForbidden
	}
	if subtle.ConstantTimeCompare([]byte(raw.Azp), []byte(s.cfg.GatewayClientID)) != 1 {
		return nil, errForbidden
	}

	return &echoClaims{
		NAVident:          navIdent,
		PreferredUsername: raw.PreferredUsername,
		Name:              raw.Name,
		Sub:               raw.Sub,
		Oid:               raw.Oid,
		Azp:               raw.Azp,
		Aud:               raw.Aud,
		Scp:               raw.Scp,
		Idtyp:             raw.Idtyp,
		Groups:            raw.Groups,
		Tid:               raw.Tid,
		Ver:               raw.Ver,
		Iat:               raw.Iat,
		Exp:               raw.Exp,
	}, nil
}

func extractBearerToken(authHeader string) (string, bool) {
	if len(authHeader) <= len("Bearer ") || len(authHeader) > maxBearerLen {
		return "", false
	}
	if !strings.EqualFold(authHeader[:len("Bearer ")], "Bearer ") {
		return "", false
	}
	tok := strings.TrimSpace(authHeader[len("Bearer "):])
	if tok == "" || strings.ContainsAny(tok, " \t\r\n") {
		return "", false
	}
	return tok, true
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

	if cfg.IntrospectionEndpoint == "" || cfg.GatewayClientID == "" {
		slog.Warn("fleet-echo auth is not fully configured",
			"has_introspection_endpoint", cfg.IntrospectionEndpoint != "",
			"has_gateway_client_id", cfg.GatewayClientID != "",
		)
	}

	srv := &server{cfg: cfg}
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("Starting fleet-echo", "port", cfg.Port)

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

// whoamiAnnotations are client hints, never a security boundary.
var whoamiAnnotations = map[string]bool{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}

// serveModernEcho answers a 2026-07-28 request: stateless, no initialize.
func serveModernEcho(w http.ResponseWriter, r *http.Request, env mcpEnvelope, id json.RawMessage, claims *echoClaims, now time.Time) {
	var rpcID any
	if len(id) > 0 {
		rpcID = id
	}
	if e := validateModernMCPRequest(r, env, now); e != nil {
		writeMCPError(w, http.StatusBadRequest, rpcID, e)
		return
	}
	if strings.HasPrefix(env.Method, "notifications/") {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result any
	var extra map[string]any
	switch env.Method {
	case "server/discover":
		result = mcpDiscoverResult(map[string]any{"tools": map[string]any{}}, now)
	case "tools/list":
		result = map[string]any{"tools": []map[string]any{{
			"name":        "whoami",
			"description": "Return validated Entra claims from Texas introspection",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
			"annotations": whoamiAnnotations,
		}}}
		extra = map[string]any{"ttlMs": mcpDiscoverTTLMilliseconds, "cacheScope": "private"}
	case "tools/call":
		if env.Params.Name != "whoami" {
			writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": rpcID, "error": mcpRPCError{Code: -32602, Message: "unknown tool"}})
			return
		}
		claimsJSON, _ := json.Marshal(claims)
		result = map[string]any{"content": []map[string]string{{"type": "text", "text": string(claimsJSON)}}, "claims": claims}
	default:
		writeMCPError(w, http.StatusNotFound, rpcID, &mcpRPCError{Code: mcpCodeMethodNotFound, Message: "Method not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": rpcID, "result": mcpModernResult(result, "fleet-echo", "1.0.0", extra)})
}
