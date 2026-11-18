package main

// MCP revision 2026-07-28 request handling, shared verbatim with
// apps/fleet-gateway/echo/mcp2026.go (separate Go modules, so copied, not imported).
//
// Spec: https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http#request-metadata
// and https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

const mcpModernVersion = "2026-07-28"

const (
	mcpCodeHeaderMismatch      = -32020
	mcpCodeUnsupportedVersion  = -32022
	mcpCodeMethodNotFound      = -32601
	mcpCodeParseError          = -32700
	mcpMetaProtocolVersion     = "io.modelcontextprotocol/protocolVersion"
	mcpMetaServerInfo          = "io.modelcontextprotocol/serverInfo"
	mcpLegacyUntilEnv          = "MCP_LEGACY_PROTOCOL_UNTIL"
	mcpDiscoverTTLMilliseconds = 300000
)

// Initialize-era revisions accepted while the legacy window is open.
var mcpLegacyVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpEnvelope struct {
	Method string `json:"method"`
	Params struct {
		Name string                     `json:"name"`
		URI  string                     `json:"uri"`
		Meta map[string]json.RawMessage `json:"_meta"`
	} `json:"params"`
}

func (e mcpEnvelope) metaVersion() string {
	var v string
	_ = json.Unmarshal(e.Params.Meta[mcpMetaProtocolVersion], &v)
	return v
}

// mcpLegacyAllowed reports whether initialize-era clients are still served.
// Time-boxed: MCP_LEGACY_PROTOCOL_UNTIL=YYYY-MM-DD keeps them working until
// that date (exclusive). Unset or unparsable means 2026-07-28 only.
func mcpLegacyAllowed(now time.Time) bool {
	until, err := time.Parse(time.DateOnly, os.Getenv(mcpLegacyUntilEnv))
	return err == nil && now.Before(until)
}

func mcpSupportedVersions(now time.Time) []string {
	if mcpLegacyAllowed(now) {
		return append([]string{mcpModernVersion}, mcpLegacyVersions...)
	}
	return []string{mcpModernVersion}
}

// isLegacyMCPRequest reports whether a request uses initialize-era semantics:
// an initialize call, or no 2026-07-28 marker in either header or _meta.
func isLegacyMCPRequest(r *http.Request, env mcpEnvelope) bool {
	if env.Method == "initialize" {
		return true
	}
	if env.metaVersion() != "" {
		return false
	}
	h := r.Header.Get("MCP-Protocol-Version")
	if h == "" {
		return true
	}
	for _, v := range mcpLegacyVersions {
		if h == v {
			return true
		}
	}
	return false
}

func mcpUnsupportedVersion(requested string, now time.Time) *mcpRPCError {
	return &mcpRPCError{
		Code:    mcpCodeUnsupportedVersion,
		Message: "Unsupported protocol version",
		Data:    map[string]any{"supported": mcpSupportedVersions(now), "requested": requested},
	}
}

// validateModernMCPRequest checks the headers a 2026-07-28 server that
// processes the body must check. A non-nil result is answered with HTTP 400.
func validateModernMCPRequest(r *http.Request, env mcpEnvelope, now time.Time) *mcpRPCError {
	mismatch := func(msg string) *mcpRPCError {
		return &mcpRPCError{Code: mcpCodeHeaderMismatch, Message: msg}
	}
	h := r.Header.Get("MCP-Protocol-Version")
	switch {
	case h == "":
		return mismatch("missing MCP-Protocol-Version header")
	case h != mcpModernVersion:
		return mcpUnsupportedVersion(h, now)
	case env.metaVersion() != h:
		return mismatch("MCP-Protocol-Version header does not match _meta " + mcpMetaProtocolVersion)
	case r.Header.Get("Mcp-Method") != env.Method:
		return mismatch("Mcp-Method header does not match method")
	}
	want, named := "", false
	switch env.Method {
	case "tools/call", "prompts/get":
		want, named = env.Params.Name, true
	case "resources/read":
		want, named = env.Params.URI, true
	}
	if named && r.Header.Get("Mcp-Name") != want {
		return mismatch("Mcp-Name header does not match params")
	}
	// No tool here declares x-mcp-header, so there are no Mcp-Param-* headers to check.
	return nil
}

// mcpModernResult adds the fields every 2026-07-28 result carries.
// extra holds per-method fields such as ttlMs and cacheScope.
func mcpModernResult(result any, serverName, serverVersion string, extra map[string]any) any {
	m := map[string]any{}
	if b, err := json.Marshal(result); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	for k, v := range extra {
		m[k] = v
	}
	m["resultType"] = "complete"
	meta, _ := m["_meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
	}
	meta[mcpMetaServerInfo] = map[string]string{"name": serverName, "version": serverVersion}
	m["_meta"] = meta
	return m
}

// mcpDiscoverResult answers server/discover.
func mcpDiscoverResult(capabilities any, now time.Time) map[string]any {
	return map[string]any{
		"supportedVersions": mcpSupportedVersions(now),
		"capabilities":      capabilities,
		"ttlMs":             mcpDiscoverTTLMilliseconds,
		"cacheScope":        "private",
	}
}

func writeMCPError(w http.ResponseWriter, status int, id any, e *mcpRPCError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": e})
}
