package mcpkit

// MCP revision 2026-07-28 request handling. Copies still live in
// apps/fleet-gateway/echo and mcp-onboarding.
//
// Spec: https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http#request-metadata
// and https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

// ModernVersion is the MCP revision this package implements.
const ModernVersion = "2026-07-28"

// JSON-RPC error codes, _meta keys and settings used on the wire.
const (
	CodeInvalidRequest                    = -32600
	CodeHeaderMismatch                    = -32020
	CodeMissingRequiredClientCapabilities = -32021
	CodeUnsupportedProtocolVersion        = -32022
	CodeMethodNotFound                    = -32601
	CodeInvalidParams                     = -32602
	CodeInternalError                     = -32603
	CodeParseError                        = -32700
	MetaProtocolVersion                   = "io.modelcontextprotocol/protocolVersion"
	MetaClientCapabilities                = "io.modelcontextprotocol/clientCapabilities"
	MetaServerInfo                        = "io.modelcontextprotocol/serverInfo"
	LegacyUntilEnv                        = "MCP_LEGACY_PROTOCOL_UNTIL"
	DiscoverTTLMilliseconds               = 300000
)

// Initialize-era revisions accepted while the legacy window is open.
var legacyVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

// WireError is a JSON-RPC error object. It implements error.
type WireError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *WireError) Error() string { return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message) }

// Envelope holds the request fields the transport rules look at.
type Envelope struct {
	Method string `json:"method"`
	Params struct {
		Name      string          `json:"name"`
		URI       string          `json:"uri"`
		Arguments json.RawMessage `json:"arguments"`
		// ProtocolVersion is the initialize-era handshake field, not _meta.
		ProtocolVersion string                     `json:"protocolVersion"`
		Meta            map[string]json.RawMessage `json:"_meta"`
		// Multi round-trip retry fields, set by the client, never by the model.
		// https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/mrtr
		InputResponses map[string]json.RawMessage `json:"inputResponses"`
		RequestState   string                     `json:"requestState"`
	} `json:"params"`
}

// FormElicitation reports whether the client declared form-mode elicitation.
// An empty elicitation object means form only.
// https://modelcontextprotocol.io/specification/2026-07-28/client/elicitation#capabilities
func (e Envelope) FormElicitation() bool {
	var caps struct {
		Elicitation map[string]json.RawMessage `json:"elicitation"`
	}
	if json.Unmarshal(e.Params.Meta[MetaClientCapabilities], &caps) != nil || caps.Elicitation == nil {
		return false
	}
	_, form := caps.Elicitation["form"]
	_, url := caps.Elicitation["url"]
	return form || !url
}

// HasClientCapabilities reports whether _meta carries clientCapabilities as a
// JSON object; an empty object is valid (no optional capabilities).
func (e Envelope) HasClientCapabilities() bool {
	var m map[string]json.RawMessage
	return json.Unmarshal(e.Params.Meta[MetaClientCapabilities], &m) == nil && m != nil
}

// NegotiateLegacyVersion answers an initialize-era handshake: echo the
// requested revision when supported, else offer the newest one.
// https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle#version-negotiation
func NegotiateLegacyVersion(requested string) string {
	if slices.Contains(legacyVersions, requested) {
		return requested
	}
	return legacyVersions[0]
}

// MetaVersion returns _meta protocolVersion, or "" when absent.
func (e Envelope) MetaVersion() string {
	var v string
	_ = json.Unmarshal(e.Params.Meta[MetaProtocolVersion], &v)
	return v
}

// LegacyAllowed reports whether initialize-era clients are still served.
// Time-boxed: MCP_LEGACY_PROTOCOL_UNTIL=YYYY-MM-DD keeps them working until
// that date (exclusive). Unset or unparsable means 2026-07-28 only.
func LegacyAllowed(now time.Time) bool {
	until, err := time.Parse(time.DateOnly, os.Getenv(LegacyUntilEnv))
	return err == nil && now.Before(until)
}

// SupportedVersions lists the revisions served at now, newest first.
func SupportedVersions(now time.Time) []string {
	if LegacyAllowed(now) {
		return append([]string{ModernVersion}, legacyVersions...)
	}
	return []string{ModernVersion}
}

// IsLegacyRequest reports whether a request uses initialize-era semantics:
// no 2026-07-28 marker in either header or _meta. The method plays no part,
// so an initialize that declares 2026-07-28 still gets its headers checked.
func IsLegacyRequest(r *http.Request, env Envelope) bool {
	if env.MetaVersion() != "" {
		return false
	}
	h := r.Header.Get("MCP-Protocol-Version")
	if h == "" {
		return true
	}
	for _, v := range legacyVersions {
		if h == v {
			return true
		}
	}
	return false
}

// UnsupportedVersion is the -32022 error naming the supported revisions.
func UnsupportedVersion(requested string, now time.Time) *WireError {
	return &WireError{
		Code:    CodeUnsupportedProtocolVersion,
		Message: "Unsupported protocol version",
		Data:    map[string]any{"supported": SupportedVersions(now), "requested": requested},
	}
}

// ValidateModernRequest checks the headers a 2026-07-28 server that
// processes the body must check. A non-nil result is answered with HTTP 400.
func ValidateModernRequest(r *http.Request, env Envelope, now time.Time) *WireError {
	mismatch := func(msg string) *WireError {
		return &WireError{Code: CodeHeaderMismatch, Message: msg}
	}
	// Required _meta fields absent from params: InvalidParams, not HeaderMismatch.
	// https://modelcontextprotocol.io/specification/2026-07-28/schema#requestmetaobject
	invalid := func(msg string) *WireError {
		return &WireError{Code: CodeInvalidParams, Message: msg}
	}
	h := r.Header.Get("MCP-Protocol-Version")
	switch {
	case h == "":
		return mismatch("missing MCP-Protocol-Version header")
	case h != ModernVersion:
		return UnsupportedVersion(h, now)
	case env.MetaVersion() == "":
		return invalid("missing _meta " + MetaProtocolVersion)
	case !env.HasClientCapabilities():
		return invalid("missing _meta " + MetaClientCapabilities + " (use {} for none)")
	case env.MetaVersion() != h:
		return mismatch("MCP-Protocol-Version header does not match _meta " + MetaProtocolVersion)
	}
	gotMethod, ok := DecodeHeaderValue(r.Header.Get("Mcp-Method"))
	if !ok {
		return mismatch("Mcp-Method header has a malformed base64 value")
	}
	if gotMethod != env.Method {
		return mismatch("Mcp-Method header does not match method")
	}
	want, named := "", false
	switch env.Method {
	case "tools/call", "prompts/get":
		want, named = env.Params.Name, true
	case "resources/read":
		want, named = env.Params.URI, true
	}
	if named {
		got, ok := DecodeHeaderValue(r.Header.Get("Mcp-Name"))
		if !ok {
			return mismatch("Mcp-Name header has a malformed base64 value")
		}
		if got != want {
			return mismatch("Mcp-Name header does not match params")
		}
	}
	// No tool here declares x-mcp-header, so there are no Mcp-Param-* headers to check.
	return nil
}

// DecodeHeaderValue decodes the transport's =?base64?...?= sentinel form;
// any other value is returned as is. ok is false for a malformed encoding.
// https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http#value-encoding
func DecodeHeaderValue(v string) (string, bool) {
	inner, found := strings.CutPrefix(v, "=?base64?")
	if !found {
		return v, true
	}
	inner, found = strings.CutSuffix(inner, "?=")
	if !found {
		return "", false
	}
	b, err := base64.StdEncoding.DecodeString(inner)
	return string(b), err == nil
}

// ModernResult adds the fields every 2026-07-28 result carries.
// extra holds per-method fields such as ttlMs and cacheScope.
func ModernResult(result any, serverName, serverVersion string, extra map[string]any) any {
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
	if _, set := m["resultType"]; !set { // a tool may answer input_required
		m["resultType"] = "complete"
	}
	meta, _ := m["_meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
	}
	meta[MetaServerInfo] = map[string]string{"name": serverName, "version": serverVersion}
	m["_meta"] = meta
	return m
}

// DiscoverResult answers server/discover.
func DiscoverResult(capabilities any, now time.Time) map[string]any {
	return map[string]any{
		"supportedVersions": SupportedVersions(now),
		"capabilities":      capabilities,
		"ttlMs":             DiscoverTTLMilliseconds,
		"cacheScope":        "private",
	}
}

// WriteError writes a JSON-RPC error response with the given HTTP status.
func WriteError(w http.ResponseWriter, status int, id any, e *WireError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "error": e})
}
