package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

var updateCELGolden = flag.Bool("update-cel-golden", false, "rewrite testdata/agentgateway_cel_golden.txt")

// agentgatewayToolRule renders a route's tool allowlist as the agentgateway
// mcpAuthorization CEL rule that allows the same tools. It is test-only: the
// runtime still enforces allowedTools; this pins the mapping for a later move.
func agentgatewayToolRule(r routing.Route) string {
	if r.AllowedTools == nil {
		return "true" // passAllTools: operator-only, forwards every tool
	}
	quoted := make([]string, len(r.Tools))
	for i, t := range r.Tools {
		// Names are limited to [A-Za-z0-9_.-] by applyToolPolicy, so quoting never escapes.
		quoted[i] = strconv.Quote(t)
	}
	return "mcp.tool.name in [" + strings.Join(quoted, ", ") + "]"
}

// celAllowed reads the rule back and answers whether it allows tool, so the
// test proves the rule permits exactly what allowedTools permits.
func celAllowed(t *testing.T, rule, tool string) bool {
	t.Helper()
	if rule == "true" {
		return true
	}
	body, ok := strings.CutPrefix(rule, "mcp.tool.name in [")
	if !ok || !strings.HasSuffix(body, "]") {
		t.Fatalf("unexpected rule shape: %s", rule)
	}
	rest := strings.TrimSuffix(body, "]")
	for rest != "" {
		lit, err := strconv.QuotedPrefix(rest)
		if err != nil {
			t.Fatalf("rule %s: %v", rule, err)
		}
		name, _ := strconv.Unquote(lit)
		if name == tool {
			return true
		}
		rest = strings.TrimPrefix(rest[len(lit):], ", ")
	}
	return false
}

func TestAgentgatewayCELMatchesAllowlist(t *testing.T) {
	var golden strings.Builder
	for _, file := range []string{"pilot-routes.dev.yaml", "pilot-routes.prod.yaml"} {
		raw, err := os.ReadFile(file) //nolint:gosec // fixed repo file names
		if err != nil {
			t.Fatal(err)
		}
		routes, err := routing.Parse(raw)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		fmt.Fprintf(&golden, "# %s\n", file)
		for _, r := range routes {
			rule := agentgatewayToolRule(r)
			fmt.Fprintf(&golden, "%s: %s\n", r.Route, rule)
			probes := append([]string{"", "delete_all", "not-a-tool", "\"]||true"}, r.Tools...)
			for _, p := range probes {
				want := r.AllowedTools == nil || r.AllowedTools[p]
				if got := celAllowed(t, rule, p); got != want {
					t.Errorf("%s %s: tool %q allowed=%v by CEL, %v by gateway", file, r.Route, p, got, want)
				}
			}
		}
	}

	path := filepath.Join("testdata", "agentgateway_cel_golden.txt")
	if *updateCELGolden {
		if err := os.WriteFile(path, []byte(golden.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path) //nolint:gosec // fixed testdata path
	if err != nil {
		t.Fatal(err)
	}
	if golden.String() != string(want) {
		t.Errorf("CEL rules changed; rerun with -update-cel-golden and review:\n%s", golden.String())
	}
}

// TestToolNamesOutsideMCPCharsetRejected guards the CEL rendering: only names
// in the MCP charset reach it, so no escape (such as \xHH for invalid UTF-8,
// which CEL reads as a different code point) can change what the rule allows.
func TestToolNamesOutsideMCPCharsetRejected(t *testing.T) {
	long := strings.Repeat("a", 129)
	for _, bad := range []string{"", "\xff", "caf\u00e9", " x", "x y", `x"]||true||["`, `back\slash`, "a/b", long} {
		r := routing.Route{Route: "/mcp/a/b", Tools: []string{bad}}
		if err := routing.ApplyToolPolicy(&r); err == nil {
			t.Errorf("tool %q accepted", bad)
		}
	}
	r := routing.Route{Route: "/mcp/a/b", Tools: []string{"get_team", "a.b-C9", strings.Repeat("a", 128)}}
	if err := routing.ApplyToolPolicy(&r); err != nil {
		t.Fatal(err)
	}
}
