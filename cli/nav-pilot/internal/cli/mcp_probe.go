package cli

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// loopbackProbeTimeout bounds each loopback connect; a listener on this machine
// answers in well under a millisecond.
const loopbackProbeTimeout = 300 * time.Millisecond

// deadMCPLoopback is the loopback servers nothing listens for, in order. A
// local app serves these (#1448); stopped, the client only says "SSE connect
// failed". The probes run in parallel, so the wait is one timeout at most.
func deadMCPLoopback(loopback []providerpkg.MCPLoopback) []providerpkg.MCPLoopback {
	dead := make([]bool, len(loopback))
	var wg sync.WaitGroup
	for i, l := range loopback {
		wg.Go(func() {
			c, err := net.DialTimeout("tcp", net.JoinHostPort(l.Host, l.Port), loopbackProbeTimeout)
			if err != nil {
				dead[i] = true
				return
			}
			_ = c.Close()
		})
	}
	wg.Wait()
	var out []providerpkg.MCPLoopback
	for i, l := range loopback {
		if dead[i] {
			out = append(out, l)
		}
	}
	return out
}

// warnDeadMCPLoopback prints one warning per loopback server nothing
// listens for, nothing when all answer. indent is doctor's, "" at launch.
func warnDeadMCPLoopback(w io.Writer, indent string, loopback []providerpkg.MCPLoopback) {
	for _, l := range deadMCPLoopback(loopback) {
		short := l.Server[strings.LastIndex(l.Server, "/")+1:]
		fmt.Fprintf(w, "%s%s %s: nothing listening on %s. Start the app that serves it.\n",
			indent, yellow("⚠"), safe(short, 64), net.JoinHostPort(l.Host, l.Port))
	}
}

// probeMCPLoopback reads the configured loopback servers for client and
// warns about the dead ones. Silent when every one answers, none is
// configured, or the client reads no MCP config nav-pilot knows (pi). It runs
// whatever mcp_hosts says and with or without cplt: a stopped app is not a
// sandbox question. Never stops a launch.
func probeMCPLoopback(w io.Writer, indent, client string) {
	if client != providerpkg.MCPClientCopilot && client != providerpkg.MCPClientOpenCode {
		return
	}
	providerpkg.MCPClient = client
	if st, err := readMCPHostState(); err == nil {
		warnDeadMCPLoopback(w, indent, st.Current.Loopback)
	}
}
