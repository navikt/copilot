package cli

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
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
	resetCpltBuiltinDomains(t)
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
	if err := applyStrictPreset(cliPath, "", ""); err != nil {
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
	for _, want := range navAllowedDomains() {
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
	if err := applyStrictPreset(cliPath, "", ""); err != nil {
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
	if err := applyStrictPreset(cliPath, "", ""); err != nil {
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
	oldCplt(t)
	for _, d := range navAllowedDomains() {
		if strings.ContainsAny(d, "*/: ") || strings.HasPrefix(d, ".") {
			t.Errorf("%q is not a bare hostname; cplt does not read glob or URL syntax", d)
		}
		if d == "nav.no" || d == "nav.cloud.nais.io" || d == "nais.io" {
			t.Errorf("%q is an apex domain — cplt matches subdomains, so this opens far more than intended", d)
		}
	}
}

// The file nav-pilot writes has to be a complete allowlist, never a delta.
//
// cplt blocks everything outside a non-empty `proxy.allowed_domains`
// regardless of `proxy.default_allowlist`, and only unions in its built-in
// per-agent list while that second key is on. Between nav-pilot's two config
// writes, after a failed preset write, or once a user lowers the preset by
// hand, this file IS the allowlist — and a delta would leave github.com and
// every package registry unreachable.
//
// It also has to be complete because the built-in list is per agent, and
// nav-pilot launches three. Only the copilot list carries GitHub and Copilot
// infrastructure: opencode's is opencode.ai and models.dev, pi's is the package
// registries alone.
func TestSeededAllowlistStandsAloneForEveryAgent(t *testing.T) {
	oldCplt(t)
	// Model access and auth, without which no agent reaches a model at all.
	// cplt COPILOT_INFRA_DOMAINS — absent from the opencode and pi lists.
	// opencode's own infrastructure. cplt OPENCODE_DOMAINS — absent from the
	// copilot list.
	// Package registries, so a sandboxed build still resolves.
	for _, host := range []string{
		"githubcopilot.com", "github.com", "api.github.com",
		"opencode.ai", "models.dev",
		"registry.npmjs.org", "repo.maven.apache.org", "pypi.org",
	} {
		if !containsStr(navAllowedDomains(), host) {
			t.Errorf("%q missing — the file is a delta, and on its own it locks the agent out", host)
		}
	}
}

// Copilot CLI verifies MCP servers against a registry at startup, so a
// registry it cannot reach is a session that times out and then hangs — not a
// feature quietly missing. Both of Nav's registry ingresses are in the list
// because the URL comes from Nav's org-level Copilot MCP policy, which points
// at dev today and can change without a nav-pilot release.
func TestSeededAllowlistReachesTheMCPRegistry(t *testing.T) {
	oldCplt(t)
	for _, host := range []string{
		"mcp-registry.nav.no",
		"mcp-registry.ekstern.dev.nav.no",
	} {
		if !containsStr(navAllowedDomains(), host) {
			t.Errorf("%q missing — Copilot CLI hangs at startup when it cannot reach the registry it verifies servers against", host)
		}
	}
}

// The host nav-pilot allows and the URL it calls in `validate` and `install`
// are the same registry, and have to stay the same host.
func TestMCPRegistryURLHostIsAllowed(t *testing.T) {
	oldCplt(t)
	u, err := url.Parse(agentpakke.MCPRegistryURL)
	if err != nil {
		t.Fatalf("MCPRegistryURL is not a URL: %v", err)
	}
	if !containsStr(navAllowedDomains(), u.Hostname()) {
		t.Errorf("nav-pilot calls %s but does not allow %q, so its own registry check fails under strict",
			agentpakke.MCPRegistryURL, u.Hostname())
	}
}

// ─── the platform gate ───────────────────────────────────────────────────────

// stubStrictSupport replaces the platform gate for one test.
func stubStrictSupport(t *testing.T, ok bool, reason string) {
	t.Helper()
	prev := strictPresetSupported
	strictPresetSupported = func() (bool, string) { return ok, reason }
	t.Cleanup(func() { strictPresetSupported = prev })
}

// On a kernel that cannot enforce forced-proxy egress, cplt refuses to launch
// under strict. Recommending it there would stop every session on the machine —
// worse than the problem the recommendation solves — so the nudge is withheld.
func TestStrictNotRecommendedWhereCpltWouldRefuseToLaunch(t *testing.T) {
	stubStrictSupport(t, false, "this kernel cannot enforce forced-proxy egress")
	for _, preset := range []string{"standard", "permissive", "full-trust"} {
		if cpltRecommendStrict(preset) {
			t.Errorf("recommended strict from %q on a kernel where cplt refuses to launch", preset)
		}
	}
}

// The gate must not swallow the recommendation everywhere else.
func TestStrictStillRecommendedWhereItWorks(t *testing.T) {
	stubStrictSupport(t, true, "")
	for _, preset := range []string{"standard", "permissive", "full-trust"} {
		if !cpltRecommendStrict(preset) {
			t.Errorf("did not recommend strict from %q on a supported kernel", preset)
		}
	}
	if cpltRecommendStrict(cpltRecommendedPreset) {
		t.Error("recommended strict to someone already on strict")
	}
}

// The settings row stays selectable, so the action refuses on its own account
// rather than trusting that the row was hidden.
func TestStrictActionRefusesOnAnUnsupportedKernel(t *testing.T) {
	isolatedConfig(t)
	log := fakeCplt(t, nil)
	stubStrictSupport(t, false, "Landlock ABI v2, needs v4")

	err := cmdConfigStrictPreset()
	if err == nil {
		t.Fatal("the action set strict on a kernel where cplt refuses to launch")
	}
	if !strings.Contains(err.Error(), "Landlock ABI v2") {
		t.Errorf("refusal does not say why: %v", err)
	}
	if len(configSets(t, log)) != 0 {
		t.Errorf("cplt config was written anyway: %v", configSets(t, log))
	}
}

// The worst case: strict is already set on a kernel where cplt refuses to
// launch under it. Nothing on that machine starts, and a bare "strict" in the
// settings row — or a green check in doctor — says everything is fine. The
// unsupported state has to surface whatever the current preset is.
func TestPostureRowFlagsStrictAlreadySetOnAnUnsupportedKernel(t *testing.T) {
	stubStrictSupport(t, false, "this kernel cannot enforce forced-proxy egress")
	got := cpltPostureValue(cpltRecommendedPreset)
	if got == cpltRecommendedPreset {
		t.Fatalf("row shows a bare %q on a kernel where cplt will not launch", got)
	}
	if !strings.Contains(got, "will not launch") {
		t.Errorf("row does not say the machine is stuck: %q", got)
	}
}

// The settings page says the option is closed rather than showing a bare
// preset name, which would read as a choice the user declined to make.
func TestPostureRowNamesTheUnsupportedKernel(t *testing.T) {
	stubStrictSupport(t, false, "whatever")
	if got := cpltPostureValue("standard"); !strings.Contains(got, "unavailable") {
		t.Errorf("posture row hides the gate: %q", got)
	}
	stubStrictSupport(t, true, "")
	if got := cpltPostureValue("standard"); !strings.Contains(got, cpltRecommendedPreset) {
		t.Errorf("posture row dropped the recommendation where it works: %q", got)
	}
}

// cplt re-reads this file every few seconds while a session runs, so an
// in-place write is a window in which a reader sees a truncated host list —
// under strict, hosts that briefly stop resolving. The write must be
// temp-file-and-rename, and it must leave no temp file behind.
func TestAllowlistIsWrittenAtomically(t *testing.T) {
	isolatedConfig(t)
	path, err := writeNavAllowedDomains()
	if err != nil {
		t.Fatal(err)
	}
	// Rewriting must be safe and must not accumulate temp files.
	if _, err := writeNavAllowedDomains(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
		if strings.HasPrefix(e.Name(), ".cplt-allowed-domains-") {
			t.Errorf("a temp file was left behind: %s", e.Name())
		}
	}
	if !containsStr(names, filepath.Base(path)) {
		t.Errorf("the allowlist is not there after two writes: %v", names)
	}

	// The file itself is not world-readable. It is not secret, but it is
	// personal config state and there is no reason for it to be looser.
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("allowlist is %o, want no group or other bits", perm)
	}
}

// The two gates a Nais observability query has to pass are configured in two
// different places, and a host missing from either one fails the query.
//
// cplt refuses a host outside `proxy.allowed_domains` before DNS runs
// ("Domain not in allowlist"), and refuses a host that resolved to a private IP
// unless `proxy.allow_private_domains` covers it ("Resolved to a private IP").
// Mimir, Loki and the two Tempo hosts resolve privately over naisdevice, so
// they need both: navOwnDomains below, and the pakke's own propose block, which
// the user is asked to approve at install.
//
// Adding a fifth observability host to one list and not the other is the kind
// of half-change that reads as a network fault at the far end, so this holds
// them equal. Hosts nav-pilot reaches that resolve publicly —
// collector-internet.nav.cloud.nais.io is one — belong in the allowlist only,
// which is why this compares the proposal against the allowlist's observability
// hosts rather than against everything under cloud.nais.io.
func TestObservabilityHostsMatchTheProposal(t *testing.T) {
	observability := func(host string) bool {
		return strings.HasSuffix(host, ".cloud.nais.io") &&
			(strings.HasPrefix(host, "mimir.") ||
				strings.HasPrefix(host, "loki.") ||
				strings.HasPrefix(host, "tempo."))
	}

	var allowed []string
	for _, d := range navOwnDomains {
		if observability(d) {
			allowed = append(allowed, d)
		}
	}

	proposal := agentpakke.Default().CpltProposal()
	if proposal == nil {
		t.Fatal("the default pakke proposes nothing; the observability skill's hosts need a private-domain waiver")
	}
	proposed := proposal.AllowPrivateDomains()

	sort.Strings(allowed)
	if !slices.Equal(allowed, proposed) {
		t.Errorf("allowlist observability hosts %v, proposed waiver %v\n"+
			"(change navOwnDomains, .nav-pilot/agentpakke.json and legacy.go together)", allowed, proposed)
	}

	// Named in full, never as a suffix. `cloud.nais.io` would waive the
	// DNS-rebinding guard for every host under every Nais tenant; each of these
	// is one host the skill actually curls.
	for _, host := range proposed {
		if strings.Count(host, ".") < 4 {
			t.Errorf("%q is shorter than a full host name, so it waives by suffix", host)
		}
	}
}

// An older cplt says "not enforcing" when the only failures are hosts it
// should let through. Every protection held, so doctor must say "too strict",
// not "NOT enforcing". A real leak must still read as one.
func TestCpltCheckTooStrict(t *testing.T) {
	item := func(exp, dec string) string {
		return fmt.Sprintf(`{"expected":%q,"decision":%q}`, exp, dec)
	}
	report := func(enforcing bool, verified int, items ...string) string {
		return fmt.Sprintf(`{"enforcing":%v,"verified":%d,"battery":true,"items":[%s]}`,
			enforcing, verified, strings.Join(items, ","))
	}
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"agent host over-blocked", report(false, 3, item("blocked", "blocked"), item("allowed", "blocked")), true},
		{"ungraded items are ignored", report(false, 1, `{"decision":"blocked"}`, item("blocked", "blocked"), item("allowed", "blocked")), true},
		{"a leak as well", report(false, 2, item("blocked", "allowed"), item("allowed", "blocked")), false},
		{"inconclusive protection", report(false, 2, item("blocked", "inconclusive"), item("allowed", "blocked")), false},
		{"nothing verified", report(false, 0, item("allowed", "blocked")), false},
		{"enforcing", report(true, 3, item("blocked", "blocked")), false},
		{"newer cplt: enforcing, over-blocked", `{"enforcing":true,"verified":3,"over_blocked":1,"battery":true,"items":[]}`, true},
		{"newer cplt: enforcing, nothing over-blocked", `{"enforcing":true,"verified":3,"over_blocked":0,"battery":true,"items":[]}`, false},
		{"real enforcing report", realCpltCheckBattery, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := parseCpltCheckReport([]byte(tc.in))
			if r == nil {
				t.Fatal("report did not parse")
			}
			if got := r.tooStrict(); got != tc.want {
				t.Errorf("tooStrict() = %v, want %v", got, tc.want)
			}
		})
	}
}

// The incident: a hand-written allowlist with Nav hosts only, on a cplt that
// blocks the agent's hosts under it. The battery's agent-host probe says so on
// every cplt version; doctor and the posture step read that probe, and only
// warn when an allowlist is set.
func TestAgentHostShutOut(t *testing.T) {
	report := func(items ...string) *cpltCheckReport {
		r := parseCpltCheckReport([]byte(`{"enforcing":false,"verified":3,"battery":true,"items":[` + strings.Join(items, ",") + `]}`))
		if r == nil {
			t.Fatal("report did not parse")
		}
		return r
	}
	// Reasons verbatim from `cplt check --json` (cplt 2026.09.28 and 2026.09.29).
	const byAllowlist = "a fail-closed domain allowlist is active and this host is not in it (agent defaults + your allowed_domains)."
	const byBlocklist = "the host matches your blocklist file (--blocked-domains) or a blocklist subscription."
	reach := func(dec string, reason ...string) string {
		why := byAllowlist
		if len(reason) > 0 {
			why = reason[0]
		}
		return fmt.Sprintf(`{"name":"reach githubcopilot.com","category":"network","target":"githubcopilot.com:443","expected":"allowed","decision":%q,"reason":%q}`, dec, why)
	}
	const ssrf = `{"name":"reach metadata IP (SSRF)","category":"network","target":"169.254.169.254:443","expected":"blocked","decision":"blocked"}`
	const home = `{"name":"write $HOME (root)","category":"filesystem","target":"/Users/dev","expected":"allowed","decision":"blocked"}`
	const allowlist = "~/.config/cplt/allowed-domains.txt"

	tests := []struct {
		name     string
		report   *cpltCheckReport
		allowed  string
		wantHost string
	}{
		{"the incident", report(reach("blocked"), ssrf), allowlist, "githubcopilot.com"},
		{"agent host allowed", report(reach("allowed"), ssrf), allowlist, ""},
		{"blocklisted, not the allowlist's doing", report(reach("blocked", byBlocklist), ssrf), allowlist, ""},
		{"inconclusive is not blocked", report(reach("inconclusive")), allowlist, ""},
		{"another over-block is not the agent host", report(home, ssrf), allowlist, ""},
		{"blocked without an allowlist", report(reach("blocked")), "", ""},
		{"no report", nil, allowlist, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isolatedConfig(t)
			fakeCplt(t, map[string]string{"proxy.allowed_domains": tc.allowed})
			cliPath, err := findCplt()
			if err != nil {
				t.Fatal(err)
			}
			path, host := cpltAgentHostShutOut(cliPath, tc.report)
			if host != tc.wantHost {
				t.Errorf("host = %q, want %q", host, tc.wantHost)
			}
			if tc.wantHost != "" && path != tc.allowed {
				t.Errorf("path = %q, want %q", path, tc.allowed)
			}
		})
	}
}

// Only probes that could not run, beside verified protections: "could not
// verify", not "NOT enforcing". A real leak or over-block is not that.
func TestCpltCheckUnverified(t *testing.T) {
	item := func(exp, dec string) string {
		return fmt.Sprintf(`{"expected":%q,"decision":%q}`, exp, dec)
	}
	report := func(enforcing bool, verified int, items ...string) string {
		return fmt.Sprintf(`{"enforcing":%v,"verified":%d,"battery":true,"items":[%s]}`,
			enforcing, verified, strings.Join(items, ","))
	}
	for _, tc := range []struct {
		name string
		in   string
		want bool
	}{
		{"staging probe inconclusive", report(false, 5, item("blocked", "blocked"), item("blocked", "inconclusive")), true},
		{"inconclusive and a leak", report(false, 4, item("blocked", "allowed"), item("blocked", "inconclusive")), false},
		{"inconclusive and an over-block", report(false, 4, item("allowed", "blocked"), item("blocked", "inconclusive")), false},
		{"nothing verified", report(false, 0, item("blocked", "inconclusive")), false},
		{"enforcing", report(true, 3, item("blocked", "blocked")), false},
		{"ungraded inconclusive only", report(false, 3, `{"decision":"inconclusive"}`), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseCpltCheckReport([]byte(tc.in)).unverified(); got != tc.want {
				t.Errorf("unverified() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Saying yes to the posture step on the incident's config must repair it, not
// leave it as it was. And it must not write anything when there is nothing
// to repair. Goes through applyStrictPreset, the code behind the yes.
func TestPostureRepairsAnAllowlistWithoutAgentHosts(t *testing.T) {
	for _, tc := range []struct {
		host, want string
	}{
		{"githubcopilot.com", "true"},
		{"", ""},
	} {
		isolatedConfig(t)
		log := fakeCplt(t, map[string]string{"proxy.allowed_domains": "~/.config/cplt/allowed-domains.txt"})
		cliPath, err := findCplt()
		if err != nil {
			t.Fatal(err)
		}
		if err := applyStrictPreset(cliPath, "~/.config/cplt/allowed-domains.txt", tc.host); err != nil {
			t.Fatal(err)
		}
		if got := configSets(t, log)["proxy.default_allowlist"]; got != tc.want {
			t.Errorf("host %q: default_allowlist set to %q, want %q", tc.host, got, tc.want)
		}
	}
}

// oldCplt stands in a cplt without `config hosts`, so the list is the fallback
// whatever cplt the machine running the tests has.
func oldCplt(t *testing.T) {
	t.Helper()
	orig := cpltHostsFor
	t.Cleanup(func() { cpltHostsFor = orig })
	cpltHostsFor = func(string) (*cpltHosts, error) { return nil, fmt.Errorf("exit status 2") }
}

// fakeCpltHosts puts a cplt on PATH that answers `config hosts` the way cplt
// does from navikt/cplt#608 on, with the JSON verbatim from a real run.
func fakeCpltHosts(t *testing.T) {
	t.Helper()
	resetCpltBuiltinDomains(t)
	dir := t.TempDir()
	script := `#!/bin/sh
[ "$1 $2 $3 $5" = "config hosts --agent --json" ] || exit 2
case "$4" in
  copilot) echo '{"agent_hosts":["githubcopilot.com","api.github.com","github.com","copilot-proxy.githubusercontent.com","actions.githubusercontent.com","default.exp2.cds.s9ch.io"],"default_allowlist":["githubcopilot.com","api.github.com","github.com","copilot-proxy.githubusercontent.com","actions.githubusercontent.com","default.exp2.cds.s9ch.io","registry.npmjs.org","registry.yarnpkg.com","repo.maven.apache.org","plugins.gradle.org","plugins-artifacts.gradle.org","crates.io","static.crates.io","pypi.org","files.pythonhosted.org","packages.confluent.io","jitpack.io"],"version":1}' ;;
  opencode) echo '{"agent_hosts":["opencode.ai","models.dev","githubcopilot.com","api.github.com","github.com","copilot-proxy.githubusercontent.com","actions.githubusercontent.com","default.exp2.cds.s9ch.io"],"default_allowlist":["opencode.ai","models.dev","registry.npmjs.org"],"version":1}' ;;
  *) exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "cplt"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// nav-pilot asks cplt for its hosts rather than keeping a copy that goes
// stale (navikt/cplt#608): copilot's default allowlist plus opencode's agent
// hosts, once each, and Nav's own on top.
func TestCpltBuiltinDomainsComeFromCplt(t *testing.T) {
	isolatedConfig(t)
	fakeCpltHosts(t)
	got, fromCplt := cpltBuiltinDomains()
	if !fromCplt {
		t.Fatal("fromCplt = false against a cplt that answers config hosts")
	}
	// jitpack.io is in cplt's list and not in the fallback; opencode.ai
	// comes only from opencode's agent hosts.
	for _, want := range []string{"jitpack.io", "plugins-artifacts.gradle.org", "opencode.ai", "models.dev", "githubcopilot.com"} {
		if !containsStr(got, want) {
			t.Errorf("%q missing from %v", want, got)
		}
	}
	all := navAllowedDomains()
	seen := map[string]bool{}
	for _, d := range all {
		if seen[d] {
			t.Errorf("%q listed twice", d)
		}
		seen[d] = true
	}
	if len(all) != len(got)+len(navOwnDomains) {
		t.Errorf("navAllowedDomains has %d hosts, want %d builtin + %d Nav", len(all), len(got), len(navOwnDomains))
	}
}

// A cplt older than `config hosts` exits non-zero. Existing users on it get
// the list nav-pilot wrote before, unchanged.
func TestCpltBuiltinDomainsFallBackOnAnOldCplt(t *testing.T) {
	isolatedConfig(t)
	fakeCplt(t, nil) // exits 1 on config hosts, like clap on an unknown subcommand
	got, fromCplt := cpltBuiltinDomains()
	if fromCplt {
		t.Error("fromCplt = true against a cplt without config hosts")
	}
	if !slices.Equal(got, cpltHostsFallback) {
		t.Errorf("got %v, want the fallback", got)
	}
}

// A future schema is not read as version 1.
func TestCpltHostsRejectsAnUnknownVersion(t *testing.T) {
	isolatedConfig(t)
	dir := t.TempDir()
	script := "#!/bin/sh\necho '{\"agent_hosts\":[\"a.example\"],\"default_allowlist\":[\"a.example\"],\"version\":2}'\n"
	if err := os.WriteFile(filepath.Join(dir, "cplt"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, fromCplt := cpltBuiltinDomains(); fromCplt {
		t.Error("read a version 2 answer as version 1")
	}
}

// doctor says when cplt is behind whether or not nav-pilot is, and names a
// cplt without `config hosts` once, offering the upgrade only when there may
// be one.
func TestDoctorReportsCpltVersion(t *testing.T) {
	const installed = "cplt 2026.09.28-185454-a924b3d"
	for _, tc := range []struct {
		name, latest string
		newCplt      bool
		want, reject []string
	}{
		{"behind, new cplt", "2026.09.29-105335-ce50857", true,
			[]string{"out of date (latest: 2026.09.29-105335-ce50857)", "brew upgrade navikt/tap/cplt"}, []string{"config hosts"}},
		{"behind, old cplt", "2026.09.29-105335-ce50857", false,
			[]string{"out of date", "Note:", "config hosts", "brew upgrade navikt/tap/cplt"}, []string{"may be too old"}},
		{"current, cplt without config hosts", "2026.09.28-185454-a924b3d", false,
			[]string{"cplt is up to date", "Note:", "config hosts"}, []string{"Solution"}},
		{"unknown, old cplt", "", false,
			[]string{"Could not check", "may be too old", "config hosts", "brew upgrade navikt/tap/cplt"}, []string{"up to date"}},
		{"current, new cplt", "2026.09.28-185454-a924b3d", true,
			[]string{"cplt is up to date"}, []string{"config hosts", "Solution"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolatedConfig(t)
			if tc.newCplt {
				fakeCpltHosts(t)
			} else {
				fakeCplt(t, nil)
			}
			orig := latestCpltVersion
			t.Cleanup(func() { latestCpltVersion = orig })
			latestCpltVersion = func() (string, error) { return tc.latest, nil }
			out := captureStdout(func() { reportCpltVersion("/usr/local/bin/cplt", installed) })
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in:\n%s", w, out)
				}
			}
			for _, r := range tc.reject {
				if strings.Contains(out, r) {
					t.Errorf("unexpected %q in:\n%s", r, out)
				}
			}
			if n := strings.Count(out, "config hosts"); n > 1 {
				t.Errorf("fallback named %d times, want once:\n%s", n, out)
			}
		})
	}
}

// The posture step runs the battery before its prompt; it says so first.
func TestPostureSaysItIsCheckingTheSandbox(t *testing.T) {
	src := readSourceFile(t, "config_sandbox.go")
	check := strings.Index(src, `fmt.Println(dim("Checking the sandbox…"))`)
	run := strings.Index(src, "cpltAgentHostShutOut(cliPath, cpltEnforcement())")
	if check < 0 || run < 0 || check > run {
		t.Error("cmdConfigStrictPreset must print \"Checking the sandbox…\" before it runs cplt check")
	}
}

// resetCpltBuiltinDomains forgets the memoised cplt answer, before and after
// the test, so each test asks the cplt on its own PATH.
func resetCpltBuiltinDomains(t *testing.T) {
	t.Helper()
	cpltBuiltinDomains = memoCpltBuiltinDomains()
	t.Cleanup(func() { cpltBuiltinDomains = memoCpltBuiltinDomains() })
}

// doctor and config read the host list several times; cplt is asked once.
func TestCpltBuiltinDomainsAsksCpltOnce(t *testing.T) {
	isolatedConfig(t)
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := fmt.Sprintf("#!/bin/sh\necho x >> %q\necho '{\"agent_hosts\":[\"a.example\"],\"default_allowlist\":[\"a.example\"],\"version\":1}'\n", log)
	if err := os.WriteFile(filepath.Join(dir, "cplt"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for range 4 {
		navAllowedDomains()
	}
	calls, _ := os.ReadFile(log)
	if n := strings.Count(string(calls), "x"); n != 2 {
		t.Errorf("cplt spawned %d times for 4 reads, want 2 (copilot + opencode, once)", n)
	}
}
