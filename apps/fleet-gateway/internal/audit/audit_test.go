package audit

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
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
		LogMCP(MCP{NAVIdent: "Z1\n", ClientID: "c", Method: "tools/call", Tool: "x\r", Route: "team/app", Backend: "b",
			RequestID: "ID", HTTPMethod: "POST", HeaderMismatch: true, ProtocolVersion: "2026-07-28", HasMcpName: true,
			Outcome: "tool_denied", DurationMS: 3, Decision: "deny", DecisionReason: "not_listed"})
	})
	want := `{"action":{"name":"tools/call","properties":{"tool":"x"}},` +
		`"context":{"duration_ms":3,"header_mismatch":true,"http_method":"POST","mcp_headers":{"method":false,"name":true},"outcome":"tool_denied","protocol_version":"2026-07-28","request_id":"ID","scan_incomplete":false},` +
		`"decision":{"context":{"reason":"not_listed"},"decision":false},"level":"INFO","msg":"audit mcp",` +
		`"resource":{"id":"team/app","properties":{"backend":"b"},"type":"mcp_route"},` +
		`"subject":{"id":"Z1","properties":{"client_id":"c"},"type":"user"}}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
	// No subject and reason falls back to outcome when stopped early.
	got = capture(t, func() { LogMCP(MCP{Outcome: "unauthorized"}) })
	want = `{"action":{"name":"","properties":{"tool":""}},` +
		`"context":{"duration_ms":0,"header_mismatch":false,"http_method":"","mcp_headers":{"method":false,"name":false},"outcome":"unauthorized","protocol_version":"","request_id":"","scan_incomplete":false},` +
		`"decision":{"context":{"reason":"unauthorized"},"decision":false},"level":"INFO","msg":"audit mcp",` +
		`"resource":{"id":"","properties":{"backend":""},"type":"mcp_route"}}`
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
