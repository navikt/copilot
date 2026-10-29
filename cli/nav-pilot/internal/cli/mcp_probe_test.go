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

func TestDoctorWarnsWhenNothingListensOnLoopback(t *testing.T) {
	live, dead := loopbackPair(t)
	mcpConsentEnv(t, providerpkg.MCPHostState{Current: providerpkg.MCPHosts{Loopback: []providerpkg.MCPLoopback{live, dead}}}, false)
	var out bytes.Buffer
	reportMCPHosts(&out, "")
	want := "aksel-arcade: nothing listening on 127.0.0.1:" + dead.Port + ". Start the app that serves it."
	if !strings.Contains(out.String(), want) {
		t.Errorf("doctor output lacks %q:\n%s", want, out.String())
	}
	if strings.Contains(out.String(), "live-app: nothing listening") {
		t.Errorf("warned about a server that listens:\n%s", out.String())
	}
}

func TestLaunchWarnsOnceForADeadLoopbackServer(t *testing.T) {
	live, dead := loopbackPair(t)
	mcpConsentEnv(t, providerpkg.MCPHostState{Current: providerpkg.MCPHosts{Loopback: []providerpkg.MCPLoopback{live, dead}}}, false)
	var out bytes.Buffer
	warnDeadMCPLoopbackAtLaunch(&out)
	want := "aksel-arcade: nothing listening on 127.0.0.1:" + dead.Port + ". Start the app that serves it.\n"
	if !strings.HasSuffix(out.String(), want) || strings.Count(out.String(), "\n") != 1 {
		t.Errorf("launch output = %q, want one line ending %q", out.String(), want)
	}

	// Everything answers: the launch says nothing.
	mcpConsentEnv(t, providerpkg.MCPHostState{Current: providerpkg.MCPHosts{Loopback: []providerpkg.MCPLoopback{live}}}, false)
	out.Reset()
	warnDeadMCPLoopbackAtLaunch(&out)
	if out.Len() != 0 {
		t.Errorf("launch printed %q with every loopback server up", out.String())
	}
}
