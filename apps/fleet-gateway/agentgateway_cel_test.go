package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var updateCELGolden = flag.Bool("update-cel-golden", false, "rewrite testdata/agentgateway_cel_golden.txt")

// agentgatewayToolRule renders a route's tool allowlist as the agentgateway
// mcpAuthorization CEL rule that allows the same tools. It is test-only: the
// runtime still enforces allowedTools; this pins the mapping for a later move.
func agentgatewayToolRule(r PilotRoute) string {
	if r.allowedTools == nil {
		return "true" // passAllTools: operator-only, forwards every tool
	}
	quoted := make([]string, len(r.Tools))
	for i, t := range r.Tools {
		// Go and CEL share these escapes (\" \\ \n \t \xHH \uXXXX \UXXXXXXXX).
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
		routes, err := parseAndValidatePilotRoutes(raw)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		fmt.Fprintf(&golden, "# %s\n", file)
		for _, r := range routes {
			rule := agentgatewayToolRule(r)
			fmt.Fprintf(&golden, "%s: %s\n", r.Route, rule)
			probes := append([]string{"", "delete_all", "not-a-tool", "\"]||true"}, r.Tools...)
			for _, p := range probes {
				want := r.allowedTools == nil || r.allowedTools[p]
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

// TestAgentgatewayCELEscapesToolNames guards against a tool name breaking out
// of the CEL list literal and widening the rule.
func TestAgentgatewayCELEscapesToolNames(t *testing.T) {
	r := PilotRoute{Route: "/mcp/a/b", Tools: []string{`x"]||true||["`, `back\slash`}}
	if err := applyToolPolicy(&r); err != nil {
		t.Fatal(err)
	}
	rule := agentgatewayToolRule(r)
	if strings.Contains(rule, `x"]`) {
		t.Fatalf("unescaped tool name in rule: %s", rule)
	}
	for _, tool := range r.Tools {
		if !celAllowed(t, rule, tool) {
			t.Errorf("rule %s does not allow %q", rule, tool)
		}
	}
	if celAllowed(t, rule, "delete_all") {
		t.Errorf("rule %s allows delete_all", rule)
	}
}
