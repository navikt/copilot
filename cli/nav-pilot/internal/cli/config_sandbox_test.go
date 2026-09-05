package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCpltPresetFromConfigGet(t *testing.T) {
	tests := []struct {
		name, out, want string
	}{
		{"value with annotation", "standard\n[cplt] (default — not set in config file)\n", "standard"},
		{"strict", "strict\n[cplt] (set in config file)\n", "strict"},
		{"bare value", "permissive\n", "permissive"},
		{"padded", "  full-trust  \n", "full-trust"},
		{"empty (command failed, nothing on stdout)", "", ""},
		{"only whitespace", "\n\n", ""},
		{"unknown value", "paranoid\n", ""},
		{"error text", "[cplt] unknown config key 'sandbox.preset'\n", ""},
	}
	for _, tc := range tests {
		if got := cpltPresetFromConfigGet(tc.out); got != tc.want {
			t.Errorf("%s: cpltPresetFromConfigGet(%q) = %q, want %q", tc.name, tc.out, got, tc.want)
		}
	}
}

func TestCpltRecommendStrict(t *testing.T) {
	tests := []struct {
		preset string
		want   bool
	}{
		{"strict", false},
		{"standard", true},
		{"permissive", true},
		{"full-trust", true},
		{"", false}, // unknown: skip the recommendation rather than guess
	}
	for _, tc := range tests {
		if got := cpltRecommendStrict(tc.preset); got != tc.want {
			t.Errorf("cpltRecommendStrict(%q) = %v, want %v", tc.preset, got, tc.want)
		}
	}
}

// realCpltCheckBattery is a real `cplt check --json` battery report, captured
// from cplt 2026.09.02-164136-a480712 run in this repository on macOS. Trimmed
// to two of the seven items — one allowed, one blocked — and the home directory
// rewritten from the capturing developer's to /Users/dev. Everything else,
// including the fields doctor reads, is as cplt emitted it.
const realCpltCheckBattery = `{
  "agent": "Copilot",
  "platform": "macos (Seatbelt)",
  "enforcing": true,
  "verified": 4,
  "battery": true,
  "items": [
    {
      "name": "read project dir",
      "category": "filesystem",
      "target": "/Users/dev/go/src/github.com/navikt/copilot",
      "decision": "allowed",
      "expected": "allowed",
      "reason": "covered by the project-dir rule (read+write+execute)."
    },
    {
      "name": "read ~/.ssh/id_ed25519",
      "category": "filesystem",
      "target": "/Users/dev/.ssh/id_ed25519",
      "decision": "blocked",
      "expected": "blocked",
      "reason": "protected credential path, never exposed to the agent (deny-by-default). This is intentional."
    }
  ]
}`

// TestParseCpltCheckReport pins the contract doctor's enforcement check rests
// on: the real report shape decodes, and every way of not getting an answer is
// unknown rather than a verdict in either direction.
func TestParseCpltCheckReport(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantNil      bool
		wantEnforce  bool
		wantVerified int
	}{
		{name: "a real battery report", in: realCpltCheckBattery, wantEnforce: true, wantVerified: 4},
		// A graded battery that came back negative is a verdict, not unknown:
		// it must decode, so doctor renders "NOT enforcing" rather than skip it.
		{name: "a battery that is not enforcing",
			in: `{"agent":"Copilot","platform":"linux (Landlock)","enforcing":false,"verified":0,"battery":true,"items":[]}`},
		// An older cplt has no `check` subcommand: clap writes usage to stderr
		// and stdout is empty. That is unknown, not "not enforcing".
		{name: "no check subcommand", in: "", wantNil: true},
		{name: "cplt missing entirely", in: "", wantNil: true},
		{name: "not JSON at all", in: "error: unrecognized subcommand 'check'\n", wantNil: true},
		{name: "truncated read", in: `{"enforcing":true,"battery":`, wantNil: true},
		// A targeted query is ungraded — `enforcing` is false there for a
		// reason that has nothing to do with the sandbox.
		{name: "a targeted, non-battery query",
			in:      `{"agent":"Copilot","platform":"macos (Seatbelt)","enforcing":false,"verified":0,"battery":false,"items":[]}`,
			wantNil: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCpltCheckReport([]byte(tc.in))
			if tc.wantNil {
				if got != nil {
					t.Fatalf("parseCpltCheckReport(%q) = %+v, want nil (unknown)", tc.in, got)
				}
				return
			}
			if got == nil {
				t.Fatal("parseCpltCheckReport returned nil, want a report")
			}
			if got.Enforcing != tc.wantEnforce {
				t.Errorf("Enforcing = %v, want %v", got.Enforcing, tc.wantEnforce)
			}
			if got.Verified != tc.wantVerified {
				t.Errorf("Verified = %d, want %d", got.Verified, tc.wantVerified)
			}
		})
	}
}

// ─── the allowlist strict implies ────────────────────────────────────────────

// fakeCplt puts a stand-in `cplt` on PATH that records every `config set` into
// a file and answers `config get` from an optional preset map. It is a real
// binary at a real path, so findCplt, cpltConfigGet and cpltConfigSet all run
// their actual exec paths — the point being to check the wiring, not a helper's
// return value.
//
// Returns the path of the recording log.
func fakeCplt(t *testing.T, get map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "config-set.log")

	var cases strings.Builder
	for k, v := range get {
		fmt.Fprintf(&cases, "    %s) printf '%%s\\n' %q ;;\n", k, v)
	}

	script := fmt.Sprintf(`#!/bin/sh
case "$1 $2" in
  "config set") printf '%%s %%s\n' "$3" "$4" >> %q ;;
  "config get")
    case "$3" in
%s      *) exit 1 ;;
    esac ;;
  *) exit 1 ;;
esac
`, log, cases.String())

	bin := filepath.Join(dir, "cplt")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func configSets(t *testing.T, log string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if k, v, ok := strings.Cut(line, " "); ok {
			out[k] = v
		}
	}
	return out
}

// The one that matters. Turning on strict must leave cplt configured with a
// real allowlist file that really contains the telemetry collector — not merely
// leave a helper returning a list of hosts.
//
// So this asserts the whole chain: cplt was told a path, that path exists, and
// the collector is a line in it.
func TestStrictPresetSeedsAllowlistIntoCpltConfig(t *testing.T) {
	isolatedConfig(t)
	log := fakeCplt(t, nil)

	cliPath, err := findCplt()
	if err != nil {
		t.Fatal(err)
	}
	if err := applyStrictPreset(cliPath); err != nil {
		t.Fatalf("applyStrictPreset: %v", err)
	}

	sets := configSets(t, log)
	if sets["sandbox.preset"] != cpltRecommendedPreset {
		t.Errorf("sandbox.preset = %q, want %q", sets["sandbox.preset"], cpltRecommendedPreset)
	}

	listPath := sets["proxy.allowed_domains"]
	if listPath == "" {
		t.Fatal("strict was set without pointing proxy.allowed_domains anywhere — the lockdown has no Nav hosts")
	}
	data, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatalf("cplt was pointed at %s, which does not exist: %v", listPath, err)
	}
	var got []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			got = append(got, line)
		}
	}
	if !containsStr(got, "collector-internet.nav.cloud.nais.io") {
		t.Errorf("the telemetry collector is not in the file cplt reads: %v", got)
	}
	for _, want := range navAllowedDomains {
		if !containsStr(got, want) {
			t.Errorf("%q missing from the file cplt reads", want)
		}
	}
}

// The lockdown must be armed after the hosts are in place, never before.
func TestStrictPresetSeedsBeforeSettingThePreset(t *testing.T) {
	isolatedConfig(t)
	log := fakeCplt(t, nil)

	cliPath, err := findCplt()
	if err != nil {
		t.Fatal(err)
	}
	if err := applyStrictPreset(cliPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("want two config writes, got %d: %v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "proxy.allowed_domains ") {
		t.Errorf("the preset was set before the allowlist: %v", lines)
	}
}

// An allowlist the user already owns is not nav-pilot's to repoint. The key
// holds exactly one path, so taking it over would revoke every host in theirs —
// under the one preset where an unlisted host is unreachable.
func TestStrictPresetLeavesAUserAllowlistAlone(t *testing.T) {
	isolatedConfig(t)
	log := fakeCplt(t, map[string]string{"proxy.allowed_domains": "/home/me/my-domains.txt"})

	cliPath, err := findCplt()
	if err != nil {
		t.Fatal(err)
	}
	if err := applyStrictPreset(cliPath); err != nil {
		t.Fatal(err)
	}
	if got := configSets(t, log)["proxy.allowed_domains"]; got != "" {
		t.Errorf("nav-pilot repointed the user's allowlist to %q", got)
	}
	// The file is still written, so the user has something to copy from.
	if _, err := os.Stat(navAllowedDomainsPath()); err != nil {
		t.Errorf("no host list written for the user to merge: %v", err)
	}
}

// The list is a security boundary: a host in it is a hole in the lockdown.
// Every entry has to be a bare hostname cplt's exact-or-subdomain matcher can
// read, and the apex Nais domain would open every tenant at once.
func TestNavAllowedDomainsAreBareAndSpecific(t *testing.T) {
	for _, d := range navAllowedDomains {
		if strings.ContainsAny(d, "*/: ") || strings.HasPrefix(d, ".") {
			t.Errorf("%q is not a bare hostname; cplt does not read glob or URL syntax", d)
		}
		if d == "nav.no" || d == "nav.cloud.nais.io" || d == "nais.io" {
			t.Errorf("%q is an apex domain — cplt matches subdomains, so this opens far more than intended", d)
		}
	}
}
