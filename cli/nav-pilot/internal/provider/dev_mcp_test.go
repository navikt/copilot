package provider

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestDevMCPHosts(t *testing.T) {
	cases := map[string][]string{
		"":                                  nil,
		"fleet-gateway-b.intern.dev.nav.no": {"fleet-gateway-b.intern.dev.nav.no"},
		" A.Intern.Dev.Nav.No , b.intern.dev.nav.no": {"a.intern.dev.nav.no", "b.intern.dev.nav.no"},
		"fleet-gateway.intern.nav.no":                nil, // prod
		"evil.example.com":                           nil,
		"x.intern.dev.nav.no.evil.com":               nil,
		"intern.dev.nav.no":                          nil,
		".intern.dev.nav.no":                         nil,
		"*.intern.dev.nav.no":                        nil,
		"10.0.0.1":                                   nil,
		"x.intern.dev.nav.no:8443":                   nil,
		"u@x.intern.dev.nav.no":                      nil,
		"x.intern.dev.nav.no/mcp":                    nil,
		"https://x.intern.dev.nav.no":                nil,
	}
	for in, want := range cases {
		t.Setenv(DevMCPEnv, in)
		var w bytes.Buffer
		got := devMCPHosts(&w)
		if !slices.Equal(got, want) {
			t.Errorf("%q: got %v, want %v", in, got, want)
		}
		warned := strings.Contains(w.String(), "NAV_PILOT_DEV_MCP is set: dev MCP servers outside the registry are allowed in this session")
		if warned != (len(want) > 0) {
			t.Errorf("%q: warning %v, output %q", in, warned, w.String())
		}
		if in != "" && len(want) == 0 && !strings.Contains(w.String(), "ignored") {
			t.Errorf("%q: no rejection line: %q", in, w.String())
		}
	}
	// Every start: each call (one per process via devMCP) warns again.
	t.Setenv(DevMCPEnv, "a.intern.dev.nav.no")
	for range 2 {
		var w bytes.Buffer
		devMCPHosts(&w)
		if !strings.Contains(w.String(), "is set") {
			t.Fatal("warning missing on a repeat start")
		}
	}
}

func TestDevMCPAllowed(t *testing.T) {
	hosts := []string{"fleet-gateway-b.intern.dev.nav.no"}
	for u, want := range map[string]bool{
		"https://fleet-gateway-b.intern.dev.nav.no/mcp":      true,
		"HTTPS://Fleet-Gateway-B.intern.dev.nav.no/x":        true,
		"http://fleet-gateway-b.intern.dev.nav.no/mcp":       false,
		"https://fleet-gateway-b.intern.dev.nav.no:8443/mcp": false,
		"https://u@fleet-gateway-b.intern.dev.nav.no/mcp":    false,
		"https://other.intern.dev.nav.no/mcp":                false,
		"https://fleet-gateway.intern.nav.no/mcp":            false,
	} {
		if got := devMCPAllowed(mcpServer{Type: "remote", URL: u}, hosts); got != want {
			t.Errorf("%s: got %v", u, got)
		}
	}
	if devMCPAllowed(mcpServer{Type: "remote", URL: "https://fleet-gateway-b.intern.dev.nav.no/mcp"}, nil) {
		t.Error("unset must allow nothing")
	}
	if devMCPAllowed(mcpServer{Type: "local", Command: []string{"x"}}, hosts) {
		t.Error("local server allowed")
	}
}

func TestUnlistedInHonoursDevMCP(t *testing.T) {
	origPolicy, origReg, origDev := fetchMCPPolicy, fetchMCPRegistry, devMCP
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry, devMCP = origPolicy, origReg, origDev })
	fetchMCPRegistry = func(string) (mcpRegistry, error) {
		return mcpRegistry{Remotes: map[string]bool{"https://ok/mcp": true}, Packages: map[string]bool{}}, nil
	}
	servers := map[string]mcpServer{
		"ok":   {Type: "remote", URL: "https://ok/mcp"},
		"dev":  {Type: "remote", URL: "https://fleet-gateway-b.intern.dev.nav.no/mcp"},
		"prod": {Type: "remote", URL: "https://fleet-gateway-b.intern.nav.no/mcp"},
	}
	names := []string{"dev", "ok", "prod"}
	for _, tt := range []struct {
		hosts []string
		off   []string
	}{
		{nil, []string{"dev", "prod"}}, // unset: unchanged
		{[]string{"fleet-gateway-b.intern.dev.nav.no"}, []string{"prod"}},
	} {
		devMCP = func() []string { return tt.hosts }
		off, err := unlistedIn("https://registry/", servers, names)
		if err != nil || !slices.Equal(off, tt.off) {
			t.Errorf("hosts %v: off = %v, err = %v, want %v", tt.hosts, off, err, tt.off)
		}
	}
}
