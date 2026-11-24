package provider

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry, devMCP, devMCPEnabled = origPolicy, origReg, origDev, nil })
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
		if want := tt.hosts; !slices.Equal(devMCPEnabled, want) {
			t.Errorf("enabled hosts = %v, want %v", devMCPEnabled, want)
		}
	}
}

// A .envrc sets the override and P=https://a.intern.dev.nav.no/; the raw
// URL is {env:P}http://evil.example/mcp. The expansion looks like the dev
// host, but cplt strips P, so the client would reach evil.example.
func TestDevMCPRejectsEnvPlaceholder(t *testing.T) {
	raw := []byte(`{"type":"remote","url":"{env:P}http://evil.example/mcp"}`)
	servers := map[string]mcpServer{}
	for name, s := range openCodeMCPServersIn([][]byte{[]byte(`{"mcp":{"x":` + string(raw) + `}}`)}, func(k string) string {
		if k == "P" {
			return "https://a.intern.dev.nav.no/"
		}
		return ""
	}) {
		servers[name] = s
	}
	s, ok := servers["x"]
	if !ok || !strings.HasPrefix(s.URL, "https://a.intern.dev.nav.no/") {
		t.Fatalf("setup: expansion not as expected: %+v", s)
	}
	if devMCPAllowed(s, []string{"a.intern.dev.nav.no"}) {
		t.Fatal("{env:} URL allowed by NAV_PILOT_DEV_MCP")
	}
}

func TestOpenCodeMCPReportGroupsDevMCP(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	proj := t.TempDir()
	os.Mkdir(filepath.Join(proj, ".git"), 0o700)
	os.WriteFile(filepath.Join(proj, "opencode.json"), []byte(`{"mcp": {"ok": {"type": "remote", "url": "https://ok/mcp"}, "dev": {"type": "remote", "url": "https://a.intern.dev.nav.no/mcp"}, "bad": {"type": "remote", "url": "https://bad/mcp"}}}`), 0o600)
	origPolicy, origReg, origDev := fetchMCPPolicy, fetchMCPRegistry, devMCP
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry, devMCP, devMCPEnabled = origPolicy, origReg, origDev, nil })
	fetchMCPPolicy = func() (string, error) { return "https://registry/", nil }
	fetchMCPRegistry = func(string) (mcpRegistry, error) {
		return mcpRegistry{Remotes: map[string]bool{"https://ok/mcp": true}, Packages: map[string]bool{}}, nil
	}
	devMCP = func() []string { return []string{"a.intern.dev.nav.no"} }
	listed, dev, unlisted, err := OpenCodeMCPReport(proj)
	if err != nil || !slices.Equal(listed, []string{"ok"}) || !slices.Equal(dev, []string{"dev"}) || !slices.Equal(unlisted, []string{"bad"}) {
		t.Fatalf("report = %v %v %v %v", listed, dev, unlisted, err)
	}
}

// An entry Go and the client may read differently (#2846) must not pass
// NAV_PILOT_DEV_MCP or reach cplt's --allow-private-domain, even on an
// accepted dev host.
func TestDevMCPRejectsAmbiguousEntry(t *testing.T) {
	withFleetGatewayDefaults(t, "cplt 0.0.0\n")
	origReg, origDev := fetchMCPRegistry, devMCP
	t.Cleanup(func() { fetchMCPRegistry, devMCP, devMCPEnabled = origReg, origDev, nil })
	fetchMCPRegistry = func(string) (mcpRegistry, error) {
		return mcpRegistry{Remotes: map[string]bool{}, Packages: map[string]bool{}}, nil
	}
	t.Setenv(DevMCPEnv, "a.intern.dev.nav.no")
	devMCP = func() []string { return devMCPHosts(io.Discard) }
	const host = "a.intern.dev.nav.no"
	for name, raw := range map[string]string{
		"url and URL":   `{"type":"remote","url":"https://a.intern.dev.nav.no/mcp","URL":"https://a.intern.dev.nav.no/mcp"}`,
		"duplicate url": `{"type":"remote","url":"https://evil.example/mcp","url":"https://a.intern.dev.nav.no/mcp"}`,
		"cased type":    `{"Type":"remote","url":"https://a.intern.dev.nav.no/mcp"}`,
	} {
		devMCPEnabled = nil
		s, ok := openCodeMCPServersIn([][]byte{[]byte(`{"mcp":{"x":` + raw + `}}`)}, nil)["x"]
		if !ok {
			t.Fatalf("%s: entry dropped, want it kept and turned off", name)
		}
		if devMCPAllowed(s, []string{host}) {
			t.Errorf("%s: allowed by NAV_PILOT_DEV_MCP", name)
		}
		off, err := unlistedIn("https://registry/", map[string]mcpServer{"x": s}, []string{"x"})
		if err != nil || !slices.Equal(off, []string{"x"}) {
			t.Errorf("%s: off = %v, err = %v, want [x]", name, off, err)
		}
		if slices.Contains(fleetGatewayFlags(nil), host) {
			t.Errorf("%s: --allow-private-domain %s added", name, host)
		}
	}
	// Control: the same host in a clean entry does pass, so the checks above can fail.
	s := openCodeMCPServersIn([][]byte{[]byte(`{"mcp":{"x":{"type":"remote","url":"https://a.intern.dev.nav.no/mcp"}}}`)}, nil)["x"]
	if off, _ := unlistedIn("https://registry/", map[string]mcpServer{"x": s}, []string{"x"}); len(off) != 0 || !slices.Contains(fleetGatewayFlags(nil), host) {
		t.Errorf("control: off = %v, flags = %v", off, fleetGatewayFlags(nil))
	}
}
