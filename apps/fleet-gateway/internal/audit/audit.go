// Package audit builds and emits fleet-gateway's audit log lines. The field
// names and nesting are a contract with log queries (README, runbook): change
// them only on purpose.
package audit

import (
	"log/slog"
	"strings"

	"github.com/navikt/copilot/fleet-gateway/internal/identity"
)

// Sanitize strips CR and LF so a value cannot forge a log line.
func Sanitize(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// MCP is one decision on an MCP route, logged as `audit mcp` in the OpenID
// AuthZEN Access Evaluation shape (owner decision 2026-11-18, #2342).
type MCP struct {
	Caller          identity.Caller // zero when the request had no valid session
	Method, Tool    string
	Route, Backend  string // Route without the /mcp/ prefix
	RequestID       string
	HTTPMethod      string
	ScanIncomplete  bool
	HeaderMismatch  bool
	ProtocolVersion string // already capped and quoted by policy.ProtocolVersion
	HasMcpMethod    bool
	HasMcpName      bool
	Outcome         string
	DurationMS      int64
	Decision        string // "allow", "deny" or "" when stopped before the policy
	DecisionReason  string
}

// LogMCP emits the `audit mcp` line.
func LogMCP(m MCP) {
	// decision is true only when the gateway's own checks let the request through;
	// context.outcome tells what happened after that.
	reason := m.DecisionReason
	if m.Decision != "allow" && (m.Decision == "" || reason == "") {
		reason = m.Outcome
	}
	// agentgateway access-log names (owner decision 2026-11-24), flat dotted
	// keys emitted next to the AuthZEN fields. Same values, no new data.
	attrs := []any{
		"mcp.method.name", Sanitize(m.Method),
		"gen_ai.tool.name", Sanitize(m.Tool),
		"route", m.Route,
		"request.id", m.RequestID,
		"http.method", Sanitize(m.HTTPMethod),
	}
	// AuthZEN 1.0 §5.1 makes subject.id REQUIRED, so an unauthenticated
	// request has no subject rather than one with an empty id.
	// properties.client_id predates the actor group and stays for existing
	// queries; actor is who acted for the subject (#2582). No token is logged.
	if c := m.Caller; c.Subject != "" {
		attrs = append(attrs, "jwt.sub", Sanitize(c.Subject))
		attrs = append(attrs, slog.Group("subject",
			"type", "user",
			"id", Sanitize(c.Subject),
			slog.Group("properties",
				"client_id", Sanitize(c.Actor),
				slog.Group("actor", "type", c.ActorType, "id", Sanitize(c.Actor)),
				"issuer", c.Issuer,
				"assurance", c.Assurance,
			),
		))
	}
	slog.Info("audit mcp", append(attrs,
		slog.Group("action",
			"name", Sanitize(m.Method),
			slog.Group("properties", "tool", Sanitize(m.Tool)),
		),
		slog.Group("resource",
			"type", "mcp_route",
			"id", m.Route,
			slog.Group("properties", "backend", m.Backend),
		),
		slog.Group("context",
			"request_id", m.RequestID,
			"http_method", Sanitize(m.HTTPMethod),
			"scan_incomplete", m.ScanIncomplete,
			"header_mismatch", m.HeaderMismatch,
			"protocol_version", Sanitize(m.ProtocolVersion),
			slog.Group("mcp_headers",
				"method", m.HasMcpMethod,
				"name", m.HasMcpName,
			),
			"outcome", m.Outcome,
			"duration_ms", m.DurationMS,
		),
		// AuthZEN 1.0 §5.5: decision boolean plus optional context.
		slog.Group("decision",
			"decision", m.Decision == "allow",
			slog.Group("context", "reason", reason),
		),
	)...)
}

// LockoutEntry is one operator lock/undo action, stored as JSON in the audit
// outbox until it is logged.
type LockoutEntry struct {
	ID       string `json:"id"`
	Action   string `json:"action"`
	UserKey  string `json:"user_key"`
	Operator string `json:"operator"`
	At       int64  `json:"at"`
}

// LogLockout emits the `audit lockout` line for a drained outbox entry.
func LogLockout(e LockoutEntry) {
	slog.Info("audit lockout",
		"outcome", "ok",
		"id", e.ID,
		"action", e.Action,
		"user_key", e.UserKey,
		"operator", Sanitize(e.Operator),
		"at", e.At,
	)
}
