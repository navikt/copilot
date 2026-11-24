package audit

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/navikt/copilot/fleet-gateway/internal/identity"
)

func capture(t *testing.T, f func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	f()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "time")
	out, _ := json.Marshal(m)
	return string(out)
}

// Golden lines: the shape before the move to this package (README, runbook).
func TestLogMCP_Golden(t *testing.T) {
	got := capture(t, func() {
		LogMCP(MCP{Caller: identity.Caller{Issuer: "fleet-gateway", Subject: "Z1\n", Actor: "c\r", ActorType: "oauth_client", Assurance: "session"}, Method: "tools/call", Tool: "x\r", Route: "team/app", Backend: "b",
			RequestID: "ID", HTTPMethod: "POST", HeaderMismatch: true, ProtocolVersion: "2026-07-28", HasMcpName: true,
			Outcome: "tool_denied", DurationMS: 3, Decision: "deny", DecisionReason: "not_listed"})
	})
	want := `{"action":{"name":"tools/call","properties":{"tool":"x"}},` +
		`"context":{"duration_ms":3,"header_mismatch":true,"http_method":"POST","mcp_headers":{"method":false,"name":true},"outcome":"tool_denied","protocol_version":"2026-07-28","request_id":"ID","scan_incomplete":false},` +
		`"decision":{"context":{"reason":"not_listed"},"decision":false},` +
		`"gen_ai.tool.name":"x","http.method":"POST","jwt.sub":"Z1","level":"INFO","mcp.method.name":"tools/call","msg":"audit mcp",` +
		`"request.id":"ID",` +
		`"resource":{"id":"team/app","properties":{"backend":"b"},"type":"mcp_route"},"route":"team/app",` +
		`"subject":{"id":"Z1","properties":{"actor":{"id":"c","type":"oauth_client"},"assurance":"session","client_id":"c","issuer":"fleet-gateway"},"type":"user"}}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
	// No subject and reason falls back to outcome when stopped early.
	got = capture(t, func() { LogMCP(MCP{Outcome: "unauthorized"}) })
	want = `{"action":{"name":"","properties":{"tool":""}},` +
		`"context":{"duration_ms":0,"header_mismatch":false,"http_method":"","mcp_headers":{"method":false,"name":false},"outcome":"unauthorized","protocol_version":"","request_id":"","scan_incomplete":false},` +
		`"decision":{"context":{"reason":"unauthorized"},"decision":false},"gen_ai.tool.name":"","http.method":"","level":"INFO","mcp.method.name":"","msg":"audit mcp","request.id":"",` +
		`"resource":{"id":"","properties":{"backend":""},"type":"mcp_route"},"route":""}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestLogLockout_Golden(t *testing.T) {
	got := capture(t, func() { LogLockout(LockoutEntry{ID: "i", Action: "lock", UserKey: "k", Operator: "op\n", At: 7}) })
	want := `{"action":"lock","at":7,"id":"i","level":"INFO","msg":"audit lockout","operator":"op","outcome":"ok","user_key":"k"}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
	b, _ := json.Marshal(LockoutEntry{ID: "i", Action: "undo", UserKey: "k", Operator: "o", At: 1})
	if string(b) != `{"id":"i","action":"undo","user_key":"k","operator":"o","at":1}` {
		t.Errorf("outbox entry %s", b)
	}
}

// Transition (owner decision 2026-11-24): the AuthZEN names and the
// agentgateway names are both present until the old names are removed.
func TestLogMCP_BothNameSets(t *testing.T) {
	var m map[string]any
	line := capture(t, func() {
		LogMCP(MCP{Caller: identity.Caller{Subject: "Z9"}, Method: "tools/call", Tool: "t", Route: "r", RequestID: "i", HTTPMethod: "POST", Decision: "allow"})
	})
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatal(err)
	}
	sub := func(k string) map[string]any { v, _ := m[k].(map[string]any); return v }
	pairs := map[string][2]any{
		"jwt.sub":          {m["jwt.sub"], sub("subject")["id"]},
		"mcp.method.name":  {m["mcp.method.name"], sub("action")["name"]},
		"gen_ai.tool.name": {m["gen_ai.tool.name"], sub("action")["properties"].(map[string]any)["tool"]},
		"route":            {m["route"], sub("resource")["id"]},
		"request.id":       {m["request.id"], sub("context")["request_id"]},
		"http.method":      {m["http.method"], sub("context")["http_method"]},
	}
	for k, p := range pairs {
		if p[0] == nil || p[0] != p[1] {
			t.Errorf("%s: new %v, old %v", k, p[0], p[1])
		}
	}
	// No identity, no subject: the new name must not invent one either.
	if anon := capture(t, func() { LogMCP(MCP{Method: "tools/call"}) }); bytes.Contains([]byte(anon), []byte("jwt.sub")) {
		t.Errorf("jwt.sub without a session: %s", anon)
	}
}
