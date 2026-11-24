package main

import "testing"

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
		u, err := validateInternalUpstreamURL(c.raw, c.ns)
		if err != nil {
			t.Errorf("%s (ns %s): %v", c.raw, c.ns, err)
		} else if u.String() != c.want {
			t.Errorf("%s (ns %s) = %s, want %s", c.raw, c.ns, u, c.want)
		}
	}
	if _, err := validateInternalUpstreamURL("http://xn--nv-fia.copilot/mcp", "copilot"); err == nil {
		t.Error("punycode host accepted")
	}
}
