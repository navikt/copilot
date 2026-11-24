package provider

import (
	"slices"
	"testing"
)

func withFleetGatewayDefaults(t *testing.T, version string) {
	t.Helper()
	prevFlags, prevProbe, prevProposal := cpltDefaultFlags, probeCpltVersion, cpltProposalFlags
	t.Cleanup(func() { cpltDefaultFlags, probeCpltVersion, cpltProposalFlags = prevFlags, prevProbe, prevProposal })
	cpltDefaultFlags = fleetGatewayFlags
	probeCpltVersion = func() (string, error) { return version, nil }
	cpltProposalFlags = func() []string { return nil }
}

func TestFleetGatewayHostsOnEveryLaunch(t *testing.T) {
	withFleetGatewayDefaults(t, "cplt 2026.11.23-195628-9ef3d81\n")
	got := cpltArgv(cpltLaunch{agent: "copilot", agentArgs: []string{"-p", "hei"}})
	want := []string{"--agent", "copilot",
		"--allow-private-domain", "fleet-gateway.intern.nav.no",
		"--allow-private-domain", "fleet-gateway.intern.dev.nav.no",
		"--", "-p", "hei"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestFleetGatewayKeepsUserFlagsAndDedupes(t *testing.T) {
	withFleetGatewayDefaults(t, "cplt 0.0.0\n") // dev build: current source
	user := []string{"--allow-private-domain=fleet-gateway.intern.nav.no", "--allow-read", "fleet-gateway.intern.dev.nav.no"}
	got := cpltArgv(cpltLaunch{agent: "copilot", sessionFlags: user})
	// The user's own flags stay, in order and last. Only the host they already
	// grant is dropped; the same name as another flag's value is not a grant.
	want := append([]string{"--agent", "copilot", "--allow-private-domain", "fleet-gateway.intern.dev.nav.no"}, append(user, "--")...)
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestFleetGatewaySkippedOnOldOrUnknownCplt(t *testing.T) {
	for _, v := range []string{"cplt 2026.04.30-125006-5b19b0b\n", "cplt 0.1.0\n", "cplt 2026.04.11-48aedfc\n", "", "garbled\x00output", "cplt dev\n", "cplt 2026.11.23\n", "cplt 0.0.0-rc1\n"} {
		withFleetGatewayDefaults(t, v)
		if got := cpltArgv(cpltLaunch{agent: "copilot"}); slices.Contains(got, "--allow-private-domain") {
			t.Errorf("%q: got %q", v, got)
		}
	}
	withFleetGatewayDefaults(t, "cplt 2026.04.30-130904-61cec3b\n")
	if got := cpltArgv(cpltLaunch{agent: "copilot"}); !slices.Contains(got, "fleet-gateway.intern.nav.no") {
		t.Errorf("floor release: got %q", got)
	}
}

func TestFleetGatewayAddsDevMCPHosts(t *testing.T) {
	withFleetGatewayDefaults(t, "cplt 0.0.0\n")
	old := devMCP
	t.Cleanup(func() { devMCP = old })
	devMCP = func() []string {
		return []string{"fleet-gateway-b.intern.dev.nav.no", "fleet-gateway.intern.dev.nav.no"}
	}
	got := cpltArgv(cpltLaunch{agent: "copilot"})
	want := []string{"--agent", "copilot",
		"--allow-private-domain", "fleet-gateway.intern.nav.no",
		"--allow-private-domain", "fleet-gateway.intern.dev.nav.no",
		"--allow-private-domain", "fleet-gateway-b.intern.dev.nav.no",
		"--"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}
