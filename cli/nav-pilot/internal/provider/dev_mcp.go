package provider

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
)

// DevMCPEnv names hosts whose MCP servers a developer may use without the
// registry listing them, to test the dev fleet-gateway end to end. The
// registry is public, so dev routes stay out of it. Developer-only: no
// config key, no flag, no help text (#2781).
const DevMCPEnv = "NAV_PILOT_DEV_MCP"

const devMCPSuffix = ".intern.dev.nav.no"

// devMCPHosts is the accepted hosts in NAV_PILOT_DEV_MCP. The value is the
// user's or a repository's .envrc, so only a bare host name ending in
// .intern.dev.nav.no passes: no port, userinfo, path, IP or wildcard. Each
// rejected value gets one line on w, and the warning prints whenever any
// host is accepted.
func devMCPHosts(w io.Writer) []string {
	var hosts []string
	for _, v := range strings.Split(os.Getenv(DevMCPEnv), ",") {
		h := strings.ToLower(strings.TrimSpace(v))
		if h == "" {
			continue
		}
		if !validDevMCPHost(h) {
			fmt.Fprintf(w, "⚠ %s: ignored %q, only host names ending in %s are allowed.\n", DevMCPEnv, v, devMCPSuffix)
			continue
		}
		hosts = append(hosts, h)
	}
	if len(hosts) > 0 {
		fmt.Fprintf(w, "⚠ %s is set: dev MCP servers outside the registry are allowed in this session (%s).\n", DevMCPEnv, strings.Join(hosts, ", "))
	}
	return hosts
}

func validDevMCPHost(h string) bool {
	label, ok := strings.CutSuffix(h, devMCPSuffix)
	if !ok || label == "" {
		return false
	}
	for _, p := range strings.Split(label, ".") {
		if p == "" || p[0] == '-' || p[len(p)-1] == '-' {
			return false
		}
		for _, c := range p {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

// devMCP is [devMCPHosts] once per nav-pilot start, so the warning prints
// on every start but once only. A var so tests can replace it.
var devMCP = sync.OnceValue(func() []string { return devMCPHosts(os.Stderr) })

// devMCPAllowed reports whether s is a remote https server on a host
// NAV_PILOT_DEV_MCP accepted. The host must match exactly: no port.
func devMCPAllowed(s mcpServer, hosts []string) bool {
	if len(hosts) == 0 || s.Type != "remote" || s.hadEnvName {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(s.URL))
	return err == nil && strings.EqualFold(u.Scheme, "https") && u.User == nil && slices.Contains(hosts, strings.ToLower(u.Host))
}
