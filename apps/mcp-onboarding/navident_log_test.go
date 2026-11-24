package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// #2835: behind fleet-gateway the server must not log NAVident, and only whoami may return it.
func TestEntraMode_NoNAVidentInLogsOrResponses(t *testing.T) {
	const ident = "Z999999"
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	h := newSimpleMCPHandler()
	user := &UserContext{Login: ident, NAVident: ident}
	for _, tool := range []string{"hello_world", "greet", "echo", "whoami", "team_readiness"} {
		resp := callTool(h, tool, map[string]interface{}{"message": "hei"}, user)
		text := resp.Result.(CallToolResult).Content[0].Text
		if tool == "whoami" {
			if strings.Count(text, ident) != 1 {
				t.Fatalf("whoami should name the ident once: %q", text)
			}
			continue
		}
		if strings.Contains(text, ident) {
			t.Fatalf("%s response contains NAVident: %q", tool, text)
		}
	}
	logs := buf.String()
	if strings.Contains(logs, ident) {
		t.Fatalf("logs contain NAVident:\n%s", logs)
	}
	if !strings.Contains(logs, `"msg":"tool finished","tool":"team_readiness","rpc_id":"1","status":"error"`) {
		t.Fatalf("refused team_readiness not logged as error:\n%s", logs)
	}
	for _, want := range []string{`"tool":"greet"`, `"rpc_id":"1"`, `"status":"success"`} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %s:\n%s", want, logs)
		}
	}
}
