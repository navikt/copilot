package routing

import (
	"strings"
	"testing"
)

// TestToolNamesOutsideMCPCharsetRejected guards the CEL rendering: only names
// in the MCP charset reach it, so no escape (such as \xHH for invalid UTF-8,
// which CEL reads as a different code point) can change what the rule allows.
func TestToolNamesOutsideMCPCharsetRejected(t *testing.T) {
	long := strings.Repeat("a", 129)
	for _, bad := range []string{"", "\xff", "caf\u00e9", " x", "x y", `x"]||true||["`, `back\slash`, "a/b", long} {
		r := Route{Route: "/mcp/a/b", Tools: []string{bad}}
		if err := ApplyToolPolicy(&r); err == nil {
			t.Errorf("tool %q accepted", bad)
		}
	}
	r := Route{Route: "/mcp/a/b", Tools: []string{"get_team", "a.b-C9", strings.Repeat("a", 128)}}
	if err := ApplyToolPolicy(&r); err != nil {
		t.Fatal(err)
	}
}

// #2848: the short form <app>.<ns> is dialled as <app>.<ns>.svc.cluster.local,
// so a namespace that is a TLD cannot point the gateway at public DNS.
func TestUpstreamURLShortFormIsClusterLocal(t *testing.T) {
	for _, c := range []struct{ raw, ns, want string }{
		{"http://evil.com/mcp", "com", "http://evil.com.svc.cluster.local/mcp"},
		{"http://evil.net:8080/mcp", "net", "http://evil.net.svc.cluster.local:8080/mcp"},
		// Every upstream in pilot-routes.{dev,prod}.yaml.
		{"http://mcp-onboarding.copilot/mcp/gw", "copilot", "http://mcp-onboarding.copilot.svc.cluster.local/mcp/gw"},
		{"http://mcp-dolly.copilot/", "copilot", "http://mcp-dolly.copilot.svc.cluster.local/"},
		{"http://mcp-kodeverk.copilot/", "copilot", "http://mcp-kodeverk.copilot.svc.cluster.local/"},
		{"http://mcp-team-catalog.copilot/", "copilot", "http://mcp-team-catalog.copilot.svc.cluster.local/"},
		{"http://fleet-echo.copilot/", "copilot", "http://fleet-echo.copilot.svc.cluster.local/"},
		{"http://studio-api.aksel-studio/mcp", "aksel-studio", "http://studio-api.aksel-studio.svc.cluster.local/mcp"},
		{"http://studio-api.aksel-studio.svc.cluster.local/mcp", "aksel-studio", "http://studio-api.aksel-studio.svc.cluster.local/mcp"},
	} {
		u, err := validateUpstreamURL(c.raw, c.ns)
		if err != nil {
			t.Errorf("%s (ns %s): %v", c.raw, c.ns, err)
		} else if u.String() != c.want {
			t.Errorf("%s (ns %s) = %s, want %s", c.raw, c.ns, u, c.want)
		}
	}
	if _, err := validateUpstreamURL("http://xn--nv-fia.copilot/mcp", "copilot"); err == nil {
		t.Error("punycode host accepted")
	}
}
