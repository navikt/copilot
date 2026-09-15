package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// naisStatusWithToken is the shape `nais device status --output json` answers
// with, bearer token and all. Every test that renders output feeds this through
// the real parser, so a leak anywhere on the path shows up as a failing test
// rather than as a token in someone's terminal.
const naisStatusToken = "eyJhbGciOiJub3RhcmVhbHRva2VuIn0.SECRET-SESSION-KEY"

func naisStatusWithToken(tenant string, connected bool) []byte {
	state := "Disconnected"
	if connected {
		state = "Connected"
	}
	doc := map[string]any{
		"connectionState": state,
		"Tenants": []map[string]any{
			{"name": "NAV", "active": false, "session": map[string]any{"key": naisStatusToken, "expiry": "2026-01-01T00:00:00Z"}},
			{"name": tenant, "active": true, "session": map[string]any{"key": naisStatusToken, "expiry": "2026-01-01T00:00:00Z"}},
		},
	}
	out, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return out
}

// stubNaisStatus makes the agent lookup answer from a fixed payload, parsed by
// the real parser.
func stubNaisStatus(t *testing.T, payload []byte, lookupErr error) {
	t.Helper()
	old := naisDeviceStatus
	naisDeviceStatus = func(string) (bool, string, error) {
		if lookupErr != nil {
			return false, "", lookupErr
		}
		return parseNaisDeviceStatus(payload)
	}
	t.Cleanup(func() { naisDeviceStatus = old })
}

func writeStatusFile(t *testing.T, s naisAgentStatus) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent-status.json")
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func renderNaisdevice(t *testing.T, naisPath, statusPath string) string {
	t.Helper()
	var buf bytes.Buffer
	reportNaisdevice(&buf, naisPath, statusPath)
	out := buf.String()
	// The one assertion every state shares: nothing on this path may print the
	// session key that lives in the agent's JSON.
	if strings.Contains(out, naisStatusToken) {
		t.Fatalf("naisdevice section leaked session material:\n%s", out)
	}
	return out
}

func wantIn(t *testing.T, out string, phrases ...string) {
	t.Helper()
	for _, p := range phrases {
		if !strings.Contains(out, p) {
			t.Errorf("output missing %q:\n%s", p, out)
		}
	}
}

// ─── the reported states ─────────────────────────────────────────────────────

func TestReportNaisdevice_NoBinary(t *testing.T) {
	out := renderNaisdevice(t, "", filepath.Join(t.TempDir(), "agent-status.json"))
	wantIn(t, out, "on PATH, so nothing here can ask which tenant is active", "matches on that tenant", "doc.nais.io")
	// The CLI is not the agent: the file the gate reads is still worth a verdict.
	wantIn(t, out, "No status file yet")
	if strings.Contains(out, "did not answer within") {
		t.Errorf("a missing CLI is not a wedged agent:\n%s", out)
	}
}

func TestReportNaisdevice_NotConnected(t *testing.T) {
	stubNaisStatus(t, naisStatusWithToken("dev-nais.io", false), nil)
	out := renderNaisdevice(t, "/usr/local/bin/nais", filepath.Join(t.TempDir(), "agent-status.json"))
	wantIn(t, out, "installed but not connected", "Connect naisdevice")
	if strings.Contains(out, "the gate calls this tenant") {
		t.Errorf("a disconnected agent must not be reported as a live tenant:\n%s", out)
	}
}

func TestReportNaisdevice_ConnectedNamesBothTenantForms(t *testing.T) {
	stubNaisStatus(t, naisStatusWithToken("dev-nais.io", true), nil)
	out := renderNaisdevice(t, "/usr/local/bin/nais", filepath.Join(t.TempDir(), "agent-status.json"))
	wantIn(t, out, "Connected to", "dev-nais.io", "the gate calls this tenant", "dev-nais")
}

func TestReportNaisdevice_LookupTimeoutIsNotAVerdict(t *testing.T) {
	stubNaisStatus(t, nil, errNaisDeviceUnreachable)
	out := renderNaisdevice(t, "/usr/local/bin/nais", filepath.Join(t.TempDir(), "agent-status.json"))
	wantIn(t, out, "did not answer within", "Check that naisdevice is running")
	if strings.Contains(out, "not connected") {
		t.Errorf("a timeout must not be reported as disconnected:\n%s", out)
	}
}

func TestReportNaisdevice_InSandboxExplainsTheDeniedSocket(t *testing.T) {
	t.Setenv(cpltSandboxEnvVar, "1")
	stubNaisStatus(t, nil, errNaisDeviceUnreachable)
	out := renderNaisdevice(t, "/usr/local/bin/nais", filepath.Join(t.TempDir(), "agent-status.json"))
	wantIn(t, out, "denies its socket", "says nothing about naisdevice")
}

// ─── the status file, which is what the gate actually reads ──────────────────

func TestReportNaisStatusFile_AbsentIsNormal(t *testing.T) {
	stubNaisStatus(t, naisStatusWithToken("NAV", true), nil)
	out := renderNaisdevice(t, "/usr/local/bin/nais", filepath.Join(t.TempDir(), "agent-status.json"))
	wantIn(t, out, "No status file yet", "nais/device#564", "Nothing on this machine is misconfigured")
	if strings.Contains(out, "Solution") {
		t.Errorf("an unreleased file is not the user's problem to solve:\n%s", out)
	}
}

func TestReportNaisStatusFile_FreshNamesTheGrant(t *testing.T) {
	stubNaisStatus(t, naisStatusWithToken("NAV", true), nil)
	path := writeStatusFile(t, naisAgentStatus{
		ConnectionState: "Connected", Tenant: "NAV",
		UpdatedAt: time.Now(), HeartbeatSeconds: 60,
	})
	out := renderNaisdevice(t, "/usr/local/bin/nais", path)
	wantIn(t, out, "Status file is current", "cplt config set allow.read", path)
}

func TestReportNaisStatusFile_FreshInsideSandboxIsEnforcing(t *testing.T) {
	t.Setenv(cpltSandboxEnvVar, "1")
	stubNaisStatus(t, nil, errNaisDeviceUnreachable)
	path := writeStatusFile(t, naisAgentStatus{
		ConnectionState: "Connected", Tenant: "ssb.no",
		UpdatedAt: time.Now(), HeartbeatSeconds: 60,
	})
	out := renderNaisdevice(t, "/usr/local/bin/nais", path)
	wantIn(t, out, "tenant rules are enforced")
	if strings.Contains(out, "cplt config set allow.read") {
		t.Errorf("the grant is already in place here; do not ask for it again:\n%s", out)
	}
}

// The state the section exists for, seen from inside: naisdevice is connected,
// the file is there, and the gate still sees nothing because the read was never
// granted.
func TestReportNaisStatusFile_UnreadableNamesTheGrant(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0000 file regardless")
	}
	stubNaisStatus(t, naisStatusWithToken("NAV", true), nil)
	path := writeStatusFile(t, naisAgentStatus{
		ConnectionState: "Connected", Tenant: "NAV",
		UpdatedAt: time.Now(), HeartbeatSeconds: 60,
	})
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	out := renderNaisdevice(t, "/usr/local/bin/nais", path)
	wantIn(t, out, "not allowed to read it", "does not", "cplt config set allow.read")
	if strings.Contains(out, "Status file is current") {
		t.Errorf("a file that could not be read is not a current one:\n%s", out)
	}
}

func TestReportNaisStatusFile_NoConfigDir(t *testing.T) {
	stubNaisStatus(t, naisStatusWithToken("NAV", true), nil)
	out := renderNaisdevice(t, "/usr/local/bin/nais", "")
	wantIn(t, out, "Could not work out where naisdevice keeps its status file")
}

func TestReportNaisStatusFile_StaleIsNotCurrent(t *testing.T) {
	stubNaisStatus(t, naisStatusWithToken("NAV", true), nil)
	path := writeStatusFile(t, naisAgentStatus{
		ConnectionState: "Connected", Tenant: "NAV",
		UpdatedAt: time.Now().Add(-4 * time.Hour), HeartbeatSeconds: 60,
	})
	out := renderNaisdevice(t, "/usr/local/bin/nais", path)
	wantIn(t, out, "has not been updated since", "would enforce against whatever this file still says")
	if strings.Contains(out, "Status file is current") {
		t.Errorf("a stale file must not be reported as current:\n%s", out)
	}
}

func TestReportNaisStatusFile_WarningIsPassedOn(t *testing.T) {
	stubNaisStatus(t, naisStatusWithToken("NAV", true), nil)
	path := writeStatusFile(t, naisAgentStatus{
		ConnectionState: "Connected", Tenant: "NAV", Warning: "kernel module out of date",
		UpdatedAt: time.Now(), HeartbeatSeconds: 60,
	})
	wantIn(t, renderNaisdevice(t, "/usr/local/bin/nais", path), "kernel module out of date")
}

func TestNaisAgentStatusStale(t *testing.T) {
	now := time.Now()
	for _, tt := range []struct {
		name string
		s    naisAgentStatus
		want bool
	}{
		{"one heartbeat late", naisAgentStatus{UpdatedAt: now.Add(-70 * time.Second), HeartbeatSeconds: 60}, false},
		{"four heartbeats late", naisAgentStatus{UpdatedAt: now.Add(-5 * time.Minute), HeartbeatSeconds: 60}, true},
		{"no heartbeat declared falls back", naisAgentStatus{UpdatedAt: now.Add(-5 * time.Minute)}, true},
		{"zero time is stale", naisAgentStatus{}, true},
	} {
		if got := tt.s.stale(now); got != tt.want {
			t.Errorf("%s: stale = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// ─── the token, on every path that touches the agent ─────────────────────────

func TestParseNaisDeviceStatus_KeepsOnlyTheActiveTenant(t *testing.T) {
	raw := naisStatusWithToken("dev-nais.io", true)
	connected, tenant, err := parseNaisDeviceStatus(raw)
	if err != nil || !connected || tenant != "dev-nais.io" {
		t.Fatalf("parse = (%v, %q, %v), want (true, \"dev-nais.io\", nil)", connected, tenant, err)
	}
	if strings.Contains(tenant, naisStatusToken) {
		t.Fatal("tenant name carries session material")
	}
}

// A failing `nais` must not hand its own output back: exec.ExitError carries
// stderr, and this is a command whose output holds a bearer token.
func TestNaisDeviceStatus_FailureDropsCommandOutput(t *testing.T) {
	path := fakeNais(t, "printf '%s' \""+naisStatusToken+"\"; printf '%s' \""+naisStatusToken+"\" >&2; exit 1")
	_, _, err := naisDeviceStatus(path)
	if err == nil {
		t.Fatal("want an error from a failing nais")
	}
	if strings.Contains(err.Error(), naisStatusToken) {
		t.Fatalf("error carries session material: %v", err)
	}
}

func TestParseNaisDeviceStatus_GarbageDoesNotEchoInput(t *testing.T) {
	_, _, err := parseNaisDeviceStatus([]byte("not json " + naisStatusToken))
	if err == nil {
		t.Fatal("want an error for unparseable output")
	}
	if strings.Contains(err.Error(), naisStatusToken) {
		t.Fatalf("parse error echoes the input it choked on: %v", err)
	}
}

// ─── the bound on a wedged agent ─────────────────────────────────────────────

func TestNaisDeviceStatus_BoundedByItsOwnDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a process for naisDeviceTimeout")
	}
	path := fakeNais(t, "sleep 30")
	start := time.Now()
	if _, _, err := naisDeviceStatus(path); err == nil {
		t.Fatal("want an error from a wedged agent")
	}
	if elapsed := time.Since(start); elapsed > naisDeviceTimeout+2*time.Second {
		t.Fatalf("doctor waited %s on a wedged agent, bound is %s", elapsed, naisDeviceTimeout)
	}
}

// fakeNais writes an executable stub standing in for the nais binary.
func fakeNais(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nais")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// ─── tenant names, and when the section shows at all ─────────────────────────

func TestGateTenant(t *testing.T) {
	for in, want := range map[string]string{
		"NAV":                   "nav",
		"dev-nais.io":           "dev-nais",
		"ssb.no":                "ssb",
		"arbeidstilsynet.no":    "atil",
		"landbruksdirektoratet": "ldir",
		"  Ssb.No  ":            "ssb",
	} {
		if got := gateTenant(in); got != want {
			t.Errorf("gateTenant(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNaisPakkeInstalled(t *testing.T) {
	nais := &StateFile{SourceRepo: "nais/pilot"}
	other := &StateFile{SourceRepo: "navikt/copilot"}
	if naisPakkeInstalled(nil, other) {
		t.Error("a non-nais pakke must not pull in the naisdevice section")
	}
	if !naisPakkeInstalled(other, nais) {
		t.Error("a nais pakke in any scope must show the section")
	}
	if naisPakkeInstalled(nil, nil) {
		t.Error("nothing installed, nothing to report")
	}
}
