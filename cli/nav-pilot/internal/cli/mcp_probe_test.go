package cli

import (
	"bytes"
	"net"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// loopbackPair is one server with a listener and one on a port just closed.
func loopbackPair(t *testing.T) (live, dead providerpkg.MCPLoopback) {
	t.Helper()
	prevClient := providerpkg.MCPClient
	t.Cleanup(func() { providerpkg.MCPClient = prevClient })
	d := t.TempDir()
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME"} {
		t.Setenv(k, d)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, deadPort, _ := net.SplitHostPort(closed.Addr().String())
	_ = closed.Close()
	_, livePort, _ := net.SplitHostPort(ln.Addr().String())
	return providerpkg.MCPLoopback{Server: "io.github.navikt/live-app", Host: "127.0.0.1", Port: livePort},
		providerpkg.MCPLoopback{Server: "no.nav.aksel/aksel-arcade", Host: "127.0.0.1", Port: deadPort}
}

// Doctor's probe runs under mcp_hosts = off and without cplt: the sandbox
// guards are about consent, not about whether the app is up.
func TestDoctorWarnsWhenNothingListensOnLoopback(t *testing.T) {
	live, dead := loopbackPair(t)
	mcpConsentEnv(t, providerpkg.MCPHostState{Current: providerpkg.MCPHosts{Loopback: []providerpkg.MCPLoopback{live, dead}}}, false)
	cpltInstalled = func() bool { return false }
	writeTestFile(t, configPath(), "version = 1\nmcp_hosts = \"off\"\n")
	var out bytes.Buffer
	probeMCPLoopback(&out, "      ", providerpkg.MCPClientCopilot)
	want := "      " + yellow("⚠") + " aksel-arcade: nothing listening on 127.0.0.1:" + dead.Port + ". Start the app that serves it.\n"
	if out.String() != want {
		t.Errorf("doctor output = %q, want %q", out.String(), want)
	}
}

func TestLaunchWarnsOnceForADeadLoopbackServer(t *testing.T) {
	live, dead := loopbackPair(t)
	mcpConsentEnv(t, providerpkg.MCPHostState{Current: providerpkg.MCPHosts{Loopback: []providerpkg.MCPLoopback{live, dead}}}, false)
	for _, client := range []string{providerpkg.MCPClientCopilot, providerpkg.MCPClientOpenCode} {
		var out bytes.Buffer
		probeMCPLoopback(&out, "", client)
		want := "aksel-arcade: nothing listening on 127.0.0.1:" + dead.Port + ". Start the app that serves it.\n"
		if !strings.HasSuffix(out.String(), want) || strings.Count(out.String(), "\n") != 1 {
			t.Errorf("%s launch output = %q, want one line ending %q", client, out.String(), want)
		}
	}

	// pi reads no MCP config nav-pilot knows: nothing probed, nothing said.
	var out bytes.Buffer
	probeMCPLoopback(&out, "", "pi")
	if out.Len() != 0 {
		t.Errorf("pi launch printed %q", out.String())
	}

	// Everything answers: the launch says nothing.
	mcpConsentEnv(t, providerpkg.MCPHostState{Current: providerpkg.MCPHosts{Loopback: []providerpkg.MCPLoopback{live}}}, false)
	out.Reset()
	probeMCPLoopback(&out, "", providerpkg.MCPClientCopilot)
	if out.Len() != 0 {
		t.Errorf("launch printed %q with every loopback server up", out.String())
	}
}
