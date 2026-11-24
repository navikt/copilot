package main

import "testing"

// #2848: a namespace that is a TLD must not turn <app>.<ns> into public DNS.
func TestUpstreamURLPublicSuffixNamespace(t *testing.T) {
	for _, c := range []struct{ raw, ns string }{
		{"http://evil.com/mcp", "com"},
		{"http://evil.net/mcp", "net"},
		{"http://evil.dev/mcp", "dev"},
		{"http://xn--nv-fia.copilot/mcp", "copilot"},
	} {
		if _, err := validateInternalUpstreamURL(c.raw, c.ns); err == nil {
			t.Errorf("%s (ns %s) accepted", c.raw, c.ns)
		}
	}
	// Every upstream in pilot-routes.{dev,prod}.yaml, plus the long form.
	for _, c := range []struct{ raw, ns string }{
		{"http://mcp-onboarding.copilot/mcp/gw", "copilot"},
		{"http://mcp-dolly.copilot/", "copilot"},
		{"http://mcp-kodeverk.copilot/", "copilot"},
		{"http://mcp-team-catalog.copilot/", "copilot"},
		{"http://fleet-echo.copilot/", "copilot"},
		{"http://studio-api.aksel-studio/mcp", "aksel-studio"},
		{"http://evil.com.svc.cluster.local/mcp", "com"},
	} {
		if _, err := validateInternalUpstreamURL(c.raw, c.ns); err != nil {
			t.Errorf("%s (ns %s): %v", c.raw, c.ns, err)
		}
	}
}
