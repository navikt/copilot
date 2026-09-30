package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// findCplt locates the cplt binary. The `copilot` binary is only accepted when
// it really is cplt — plain GitHub Copilot has no `config` subcommand.
func findCplt() (string, error) {
	cliPath, cliName := providerpkg.FindCopilotCLI()
	if cliPath == "" || cliName != "cplt" {
		return "", fmt.Errorf("cplt (Copilot Sandbox) is not available on your PATH. This command requires cplt")
	}
	return cliPath, nil
}

// cpltConfigSet writes one cplt config key. flags go after the value, such as
// the --force cplt requires before it turns on a key that weakens the sandbox.
func cpltConfigSet(cliPath, key, val string, flags ...string) error {
	args := append([]string{"config", "set", key, val}, flags...)
	out, err := exec.Command(cliPath, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set %s: %v\n%s", key, err, string(out))
	}
	return nil
}

// sandboxToggle is one boolean cplt key the sandbox wizard offers. risk is set
// for the keys cplt itself marks dangerous and refuses to turn on without
// --force; the wizard shows it and asks before passing --force.
type sandboxToggle struct {
	key, label, risk string
}

var sandboxToggles = []sandboxToggle{
	{"sandbox.allow_localhost_any", "Allow any localhost port (dev servers, Gradle daemon, httptest)", ""},
	{"sandbox.allow_jvm_attach", "Allow JVM attach (MockK, Mockito inline, ByteBuddy)", ""},
	{"sandbox.allow_build_credentials", "Allow reading ~/.npmrc, ~/.gradle/gradle.properties, ~/.m2/settings.xml",
		"exposes every registry token in those files, not only the one the project needs"},
	{"sandbox.allow_docker", "Allow Docker (Colima/OrbStack)",
		"container mounts bypass the sandbox"},
	{"sandbox.allow_browser", "Allow browser access",
		"lets the agent launch any application outside the sandbox"},
	{"sandbox.allow_tmp_exec", "Allow executing /tmp binaries",
		"lets the agent run binaries it dropped in /tmp"},
}

// currentSandboxToggles reads each toggle from cplt. A key cplt gives no
// true/false answer for (an older cplt that lacks it) is left out, so the
// wizard neither offers nor writes it.
func currentSandboxToggles(cliPath string) map[string]bool {
	cur := map[string]bool{}
	for _, t := range sandboxToggles {
		switch cpltConfigGet(cliPath, t.key) {
		case "true":
			cur[t.key] = true
		case "false":
			cur[t.key] = false
		}
	}
	return cur
}

// sandboxChanges is what the wizard writes: only keys whose chosen value
// differs from the current one. Keys the user left alone are never touched,
// so a value set earlier, or by hand, survives a run of the wizard.
func sandboxChanges(cur map[string]bool, chosen []string) map[string]bool {
	out := map[string]bool{}
	for key, was := range cur {
		if now := slices.Contains(chosen, key); now != was {
			out[key] = now
		}
	}
	return out
}

// cmdConfigSandbox runs an interactive wizard to configure the cplt sandbox profile.
func cmdConfigSandbox() error {
	cliPath, err := findCplt()
	if err != nil {
		return err
	}

	cur := currentSandboxToggles(cliPath)
	if len(cur) == 0 {
		return fmt.Errorf("could not read the current cplt settings; run %s to see them", bold("cplt config show"))
	}
	var opts []huh.Option[string]
	var choices []string
	for _, t := range sandboxToggles {
		if on, ok := cur[t.key]; ok {
			opts = append(opts, huh.NewOption(t.label, t.key).Selected(on))
			if on {
				choices = append(choices, t.key)
			}
		}
	}

	err = huh.NewMultiSelect[string]().
		Title("Configure cplt sandbox relaxations").
		Description("Current settings are selected. Only what you change is written.").
		Options(opts...).
		Value(&choices).
		WithTheme(navTheme()).
		Run()
	if err != nil {
		return fmt.Errorf("prompt cancelled: %w", err)
	}

	changes := sandboxChanges(cur, choices)
	var risks []string
	for _, t := range sandboxToggles {
		if changes[t.key] && t.risk != "" {
			risks = append(risks, fmt.Sprintf("%s: %s", t.key, t.risk))
		}
	}
	if len(risks) > 0 {
		var ok bool
		if err := huh.NewConfirm().
			Title("These weaken the sandbox. Turn them on?").
			Description(strings.Join(risks, "\n")).
			Value(&ok).
			WithTheme(navTheme()).
			Run(); err != nil {
			return fmt.Errorf("prompt cancelled: %w", err)
		}
		if !ok {
			fmt.Println(dim("cplt sandbox configuration unchanged."))
			return nil
		}
	}

	return applySandboxChanges(cliPath, changes)
}

// applySandboxChanges writes what sandboxChanges returned. Split from the
// wizard so the writes are testable against a cplt on PATH without a terminal.
// The caller has already asked about every risky key, so --force is passed
// for those that are turned on.
func applySandboxChanges(cliPath string, changes map[string]bool) error {
	if len(changes) == 0 {
		fmt.Println(dim("Nothing changed."))
		return nil
	}
	for _, t := range sandboxToggles {
		on, ok := changes[t.key]
		if !ok {
			continue
		}
		var flags []string
		if on && t.risk != "" {
			flags = []string{"--force"}
		}
		if err := cpltConfigSet(cliPath, t.key, strconv.FormatBool(on), flags...); err != nil {
			return err
		}
		fmt.Printf("%s cplt %s = %t\n", domain.Green("✓"), t.key, on)
	}
	return nil
}

// ─── security posture (sandbox.preset) ───────────────────────────────────────

// cpltStrictPreset is the preset the settings page can set for you. It is no
// longer recommended: strict blocks every git push (protect_default_branch_only
// is off in it), so an agent cannot push a feature branch and open a PR, which
// is the workflow nav-pilot is built around. What strict adds over standard is
// the network: forced-proxy egress and `proxy.default_allowlist`, which blocks
// everything but cplt's built-in host list plus `proxy.allowed_domains`
// (cplt src/config/types.rs, `Preset::Strict.baseline()`). Hence
// navAllowedDomains: setting it without the Nav hosts turns them off.
const cpltStrictPreset = "strict"

// cpltStrictConsequence is the one-paragraph version of what strict does,
// leading with what it takes away.
const cpltStrictConsequence = "Blocks all pushes: the agent cannot push a branch or open a PR from one. " +
	"It also locks egress down: only cplt's built-in host list plus proxy.allowed_domains " +
	"stay reachable. nav-pilot seeds the allowlist with the Nav hosts it and your agents need, " +
	"or they go dark. Keys you set yourself still win."

// cpltPresets are the values cplt accepts for sandbox.preset. Anything else is
// treated as unknown rather than guessed at.
var cpltPresets = []string{"strict", "standard", "permissive", "full-trust"}

// cpltPresetFromConfigGet extracts the preset from `cplt config get
// sandbox.preset` output: the value is on the first line, and a following
// annotation line such as "[cplt] (default — not set in config file)" is
// ignored. An empty or unrecognised value yields "" — unknown, never guessed at.
func cpltPresetFromConfigGet(out string) string {
	first, _, _ := strings.Cut(out, "\n")
	val := strings.TrimSpace(first)
	if !containsStr(cpltPresets, val) {
		return ""
	}
	return val
}

// cpltSandboxPreset returns the effective cplt sandbox.preset, or "" when it
// cannot be determined — no cplt, a failed command, or a value this build does
// not know. Callers skip their recommendation on "" rather than guessing.
// A var so tests can stub the process spawn.
var cpltSandboxPreset = func() string {
	cliPath, err := findCplt()
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, cliPath, "config", "get", "sandbox.preset").Output()
	if err != nil {
		return ""
	}
	return cpltPresetFromConfigGet(string(out))
}

// cmdConfigStrictPreset asks for confirmation, seeds the allowlist, and sets
// sandbox.preset = strict. cplt config is personal: nav-pilot never sets it
// silently.
//
// The allowlist is seeded *before* the preset, not after. Strict is a network
// lockdown that takes effect on the next launch, so a machine that gets the
// preset without the hosts is one where nav-pilot's telemetry and every
// Nav-internal endpoint have gone dark — and the user has no reason to connect
// the two. Doing the harmless write first means the only way to end up in that
// state is for the preset to be set by hand.
func cmdConfigStrictPreset() error {
	cliPath, err := findCplt()
	if err != nil {
		return err
	}
	// The row is selectable on every machine, so the refusal is made here.
	if ok, reason := strictPresetSupported(); !ok {
		return fmt.Errorf("nav-pilot will not set sandbox.preset = strict here: %s", reason)
	}

	desc := cpltStrictConsequence
	// The battery takes a few seconds; say so rather than sit silent.
	fmt.Println(dim("Checking the sandbox…"))
	path, host := cpltAgentHostShutOut(cliPath, cpltEnforcement())
	if host != "" {
		desc += fmt.Sprintf("\n\n%s shuts out %s, so this also sets proxy.default_allowlist = true. That opens cplt's built-in hosts, package registries included.",
			path, host)
	}
	var ok bool
	if err := huh.NewConfirm().
		Title("Set cplt sandbox.preset = strict?").
		Description(desc).
		Value(&ok).
		WithTheme(navTheme()).
		Run(); err != nil {
		return fmt.Errorf("prompt cancelled: %w", err)
	}
	if !ok {
		fmt.Println(dim("sandbox.preset unchanged."))
		return nil
	}

	return applyStrictPreset(cliPath, path, host)
}

// applyStrictPreset is everything cmdConfigStrictPreset does once the user has
// said yes. Split out so the seed-then-set order — the part that matters — is
// testable against a real cplt on PATH, without a terminal.
//
// host is the agent host the user's allowlist file (path) shuts out, as the
// battery found before the prompt, or "". Repaired here, after the yes, never
// before: cplt config is the user's.
func applyStrictPreset(cliPath, path, host string) error {
	if host != "" {
		if err := repairCpltAgentHosts(cliPath, path, host); err != nil {
			return err
		}
	}
	path, adopted, err := seedCpltAllowlist(cliPath)
	if err != nil {
		return err
	}
	if adopted {
		fmt.Printf("%s cplt proxy.allowed_domains = %s (%d hosts, %d of them Nav's)\n",
			domain.Green("✓"), path, len(navAllowedDomains()), len(navOwnDomains))
	} else {
		fmt.Printf("%s You already have proxy.allowed_domains set, so nav-pilot left it alone.\n",
			domain.Yellow("⚠"))
		fmt.Printf("  Add the hosts in %s to your own file, or strict will block them.\n", path)
	}

	if err := cpltConfigSet(cliPath, "sandbox.preset", cpltStrictPreset); err != nil {
		return err
	}
	fmt.Printf("%s cplt sandbox.preset = %s\n", domain.Green("✓"), cpltStrictPreset)
	// A deliberate choice, not nav-pilot's old advice: never offer to leave it.
	providerpkg.FirstTime("leave-strict-offer")
	return nil
}

// Sandbox enforcement, verified rather than inferred from configuration.
//
// doctor used to check cplt's *setup* only: a version, a preset, and a grep of
// `cplt config show`. That answers "is it configured", never "is it enforcing",
// and the failure mode is on record — an earlier grep here tested a config key
// cplt has never had, so the check reported a problem that could not exist and
// pointed users at a `cplt config set` cplt rejects (#406).
//
// `cplt check` (navikt/cplt#145) runs probes inside the real resolved sandbox —
// the same policy an agent would get — and reports, per probe, whether cplt
// allows or blocks it, why, and the exact fix. `--json` makes it machine-
// readable and the battery exits non-zero when it cannot confirm enforcement.

// cpltCheckReport is the part of a `cplt check --json` battery report doctor
// reads. cplt's Report carries per-probe items with reasons and fixes too;
// doctor prints the verdict and leaves the detail to `cplt check` itself.
type cpltCheckReport struct {
	// Enforcing is true only when every graded expectation held AND at least
	// one protection was actually verified — cplt's own definition.
	Enforcing bool `json:"enforcing"`
	// Verified counts the expected-blocked probes that really were blocked.
	Verified int `json:"verified"`
	// Battery marks the full enforcement battery. A targeted query
	// (`cplt check path …`) is not graded and must never be read as a verdict.
	Battery bool `json:"battery"`
	// OverBlocked counts probes that should get through but were blocked.
	// From navikt/cplt#604 on, cplt reports enforcing with this above zero.
	OverBlocked int `json:"over_blocked"`
	// Items are the graded probes.
	Items []struct {
		Category string `json:"category"`
		Target   string `json:"target"`
		Expected string `json:"expected"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	} `json:"items"`
}

// agentHostBlocked returns the host of the battery's agent-host probe when
// cplt's allowlist blocked it, else "". That probe is the battery's only
// network item expected to get through: `reach <the agent's first default
// host>`. The reason must name the allowlist: a host on a blocklist is blocked
// too, and blaming allowed_domains for that sends the user to the wrong fix.
func (r *cpltCheckReport) agentHostBlocked() string {
	if r == nil {
		return ""
	}
	for _, it := range r.Items {
		if it.Category == "network" && it.Expected == "allowed" && it.Decision == "blocked" &&
			strings.Contains(it.Reason, "allowlist") {
			host, _, _ := strings.Cut(it.Target, ":")
			return host
		}
	}
	return ""
}

// unverified reports a verdict that failed only because probes could not run:
// some protections were verified, and every failing probe is inconclusive.
// That is "could not verify", not "NOT enforcing".
func (r *cpltCheckReport) unverified() bool {
	if r.Enforcing || r.Verified == 0 {
		return false
	}
	failed := false
	for _, it := range r.Items {
		if it.Expected == "" || it.Expected == it.Decision {
			continue
		}
		if it.Decision != "inconclusive" {
			return false
		}
		failed = true
	}
	return failed
}

// tooStrict reports a verdict that failed only because the sandbox blocked
// something it should allow: every protection held, but a probe that should
// get through did not. That is an over-tight config, not a leak, and doctor
// must not call it "NOT enforcing", nor plain enforcing. Newer cplt says
// enforcing with over_blocked above zero; an older cplt says not enforcing and
// this reads the same thing out of its items.
func (r *cpltCheckReport) tooStrict() bool {
	if r.Enforcing {
		return r.OverBlocked > 0
	}
	if r.Verified == 0 {
		return false
	}
	failed := false
	for _, it := range r.Items {
		if it.Expected == "" || it.Expected == it.Decision {
			continue
		}
		if it.Expected != "allowed" || it.Decision != "blocked" {
			return false
		}
		failed = true
	}
	return failed
}

// parseCpltCheckReport decodes a battery report, or returns nil for "unknown".
//
// Unknown covers every way this can fail to be an answer: a cplt old enough to
// have no `check` subcommand (clap writes a usage error to stderr and leaves
// stdout empty), no cplt at all, a killed process, or a report that is not the
// graded battery. Unknown is never rendered as enforcing and never as failing —
// a security check that goes green, or red, on missing data is the same bug in
// two directions.
func parseCpltCheckReport(out []byte) *cpltCheckReport {
	var r cpltCheckReport
	if err := json.Unmarshal(out, &r); err != nil || !r.Battery {
		return nil
	}
	return &r
}

// cpltCheckTimeout bounds the enforcement battery. It is longer than
// cpltCommandTimeout because `cplt check` builds the resolved sandbox and spawns
// probes inside it rather than reading a config file — measured at well under a
// second warm, but a cold run after a cplt upgrade does more work.
const cpltCheckTimeout = 15 * time.Second

// cpltEnforcement runs `cplt check --json` in the current directory and returns
// the battery verdict, or nil when it could not be established.
//
// The non-zero exit cplt uses for "not enforcing" is the answer, not a failure:
// the report is on stdout either way, so the error is deliberately ignored and
// the decision left to the decode.
//
// A var so tests can exercise doctor without spawning a sandbox.
var cpltEnforcement = func() *cpltCheckReport {
	cliPath, err := findCplt()
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), cpltCheckTimeout)
	defer cancel()
	out, _ := exec.CommandContext(ctx, cliPath, "check", "--json").Output()
	return parseCpltCheckReport(out)
}

// ─── the allowlist strict implies ────────────────────────────────────────────

// The strict preset is a full network lockdown, not just a set of guards.
// Beyond forced-proxy egress it turns on `proxy.default_allowlist`, which makes
// cplt's built-in per-agent host list the *only* reachable set of hosts
// (cplt src/config/types.rs, `Preset::Strict.baseline()`). That list covers
// GitHub Copilot's own infrastructure and the public package registries. It
// covers nothing of Nav's — so recommending strict on its own would silently
// cut nav-pilot's telemetry export and every Nav-internal host the agents and
// skills nav-pilot installs are built around.
//
// The recommendation therefore comes with the hosts, written to a file
// `proxy.allowed_domains` points at. What goes in that file is the complete
// list rather than the Nav delta, for reasons set out at navAllowedDomains
// below.

// navAllowedDomains is the COMPLETE set of hosts nav-pilot writes to the file
// cplt reads. Complete, not a delta — that distinction is the whole design.
//
// `proxy.allowed_domains` is fail-closed on its own account, independently of
// `proxy.default_allowlist`: cplt blocks any host outside a non-empty allowlist
// (cplt src/proxy.rs, the `BlockedAllowlist` arm), and the built-in per-agent
// list is only unioned in while `default_allowlist` is on (cplt
// src/proxy_domains.rs, `DomainList::current` — "no sticky half, the file is
// the sole source"). So a delta file is a trap with three doors into it: the
// window between nav-pilot's two config writes, a failed preset write, and a
// user who later lowers the preset by hand or with `--preset`. Behind any of
// them the file IS the allowlist, and a delta file would leave github.com and
// every package registry unreachable, with nothing on screen naming nav-pilot.
//
// Writing the built-ins back out costs nothing — cplt unions and dedupes — and
// makes the file correct standing alone. Which it also has to be for a second
// reason: the built-in list is per agent (cplt src/agent.rs,
// `Agent::default_allowed_domains`). nav-pilot launches copilot, opencode and
// pi through cplt, and only the copilot list carries GitHub and Copilot
// infrastructure. opencode gets `opencode.ai` and `models.dev`; pi gets the
// package registries and nothing else. cplt is what connects opencode to the
// GitHub Copilot provider in the first place, so a hosted session — with an
// explicit Copilot model, or with none at all, letting opencode resolve its
// own default from that same provider — needs the Copilot entries Nav adds
// below, or it cannot reach a model host. The two exceptions: a custom
// provider-qualified id (e.g. an `anthropic/...` model) routes elsewhere and
// needs its own allowlist/egress handling, and a local `mlx/<id>` session (see
// ToOpenCodeModel) talks to a loopback server instead, which needs
// `sandbox.allow_localhost_any`, not these entries.
//
// Every Nav entry below is something nav-pilot or an artifact it installs
// actually fetches, with the call site named. Hosts that appear in the
// artifacts as citation links, human-facing UI links, or sample config for the
// developer's own application are NOT here — nothing fetches them, and each one
// would widen the lockdown for nothing.
//
// cplt matches each entry exact-or-subdomain and does not read glob syntax, so
// these are bare hostnames with no leading `*.` — and they are specific hosts
// rather than `nav.cloud.nais.io`, which would open every Nais tenant at once.
func navAllowedDomains() []string {
	builtin, _ := cpltBuiltinDomains()
	return append(slices.Clone(builtin), navOwnDomains...)
}

// cpltHosts is `cplt config hosts --agent <name> --json` (navikt/cplt#608):
// the agent's effective hosts, built-in plus any cplt detects on this machine.
type cpltHosts struct {
	Version          int      `json:"version"`
	AgentHosts       []string `json:"agent_hosts"`
	DefaultAllowlist []string `json:"default_allowlist"`
}

// cpltHostsFor asks the installed cplt for one agent's hosts. An error means a
// cplt too old for the subcommand (clap exits non-zero), no cplt, or an answer
// in a shape this build does not know. A var so tests can stub the spawn.
var cpltHostsFor = func(agent string) (*cpltHosts, error) {
	cliPath, err := findCplt()
	if err != nil {
		return nil, err
	}
	out, err := runBounded(cliPath, "config", "hosts", "--agent", agent, "--json")
	if err != nil {
		return nil, err
	}
	var h cpltHosts
	if err := json.Unmarshal(out, &h); err != nil {
		return nil, err
	}
	// Both lists are consumed: an empty one would drop that agent's hosts.
	if h.Version != 1 || len(h.DefaultAllowlist) == 0 || len(h.AgentHosts) == 0 {
		return nil, fmt.Errorf("cplt config hosts: unexpected answer (version %d)", h.Version)
	}
	return &h, nil
}

// cpltBuiltinDomains is cplt's own list for the agents nav-pilot launches:
// copilot's default allowlist (its infrastructure plus the package registries,
// which is all pi gets) and opencode's agent hosts. fromCplt is false when the
// installed cplt could not say, and the list is cpltHostsFallback.
//
// Asked once per process: doctor and config read it several times, each read
// is two cplt spawns, and every count they print must match the file written.
// Callers must not modify the slice.
var cpltBuiltinDomains = memoCpltBuiltinDomains()

// memoCpltBuiltinDomains is separate so tests can start from a fresh answer.
func memoCpltBuiltinDomains() func() ([]string, bool) {
	return sync.OnceValues(askCpltBuiltinDomains)
}

func askCpltBuiltinDomains() (hosts []string, fromCplt bool) {
	copilot, err := cpltHostsFor("copilot")
	if err != nil {
		return cpltHostsFallback, false
	}
	opencode, err := cpltHostsFor("opencode")
	if err != nil {
		return cpltHostsFallback, false
	}
	return dedupeHosts(append(slices.Clone(copilot.DefaultAllowlist), opencode.AgentHosts...)), true
}

// dedupeHosts drops repeats, keeping first-seen order.
func dedupeHosts(hosts []string) []string {
	seen := map[string]bool{}
	out := hosts[:0]
	for _, h := range hosts {
		if !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	return out
}

// cpltHostsFallback is the FALLBACK for a cplt older than `cplt config hosts`.
// Frozen on purpose: it is what nav-pilot wrote before it could ask cplt, so a
// user on an old cplt gets exactly the file they got before. Do not update it;
// upgrading cplt is the fix, and doctor says so.
//
// A stale copy fails visibly, never open: a host cplt added later is one an
// agent cannot reach under strict, and a host cplt dropped was already
// reachable.
var cpltHostsFallback = []string{
	// Copilot: auth, model access and telemetry.
	"githubcopilot.com",
	"api.github.com",
	"github.com",
	"copilot-proxy.githubusercontent.com",
	"actions.githubusercontent.com",
	"default.exp2.cds.s9ch.io",

	// opencode's own infrastructure.
	"opencode.ai",
	"models.dev",

	// Package registries, shared by every agent.
	"registry.npmjs.org",
	"registry.yarnpkg.com",
	"repo.maven.apache.org",
	"plugins.gradle.org",
	"crates.io",
	"static.crates.io",
	"pypi.org",
	"files.pythonhosted.org",
}

// navOwnDomains are the hosts nav-pilot adds on top: nothing in cplt's
// built-in lists reaches any of them.
var navOwnDomains = []string{
	// nav-pilot's own OTel metrics export, on every command.
	// internal/telemetry/telemetry.go, defaultTelemetryEndpoint — also injected
	// into copilot and opencode sessions as OTEL_EXPORTER_OTLP_ENDPOINT.
	"collector-internet.nav.cloud.nais.io",

	// Nav's MCP registry, in production. Copilot CLI resolves an MCP registry
	// at startup and verifies every MCP server against it, so an unreachable
	// registry is not a missing extra: it is a startup that times out and then
	// hangs. nav-pilot itself reads the same host in `validate` and `install`
	// (internal/agentpakke, MCPRegistryURL), which checks a pakke's mcpServers
	// against what the registry publishes.
	// apps/mcp-registry/.nais/prod-gcp.yaml is the ingress.
	"mcp-registry.nav.no",

	// Nav's MCP registry, in dev — and the one Copilot CLI actually resolves
	// today. The registry URL comes from Nav's org-level Copilot MCP policy
	// over api.github.com, not from anything on the machine, so it cannot be
	// pointed at production from here. A debug log on a Nav machine reads
	// `Registry https://mcp-registry.ekstern.dev.nav.no/: servers will be
	// verified against this registry`, followed by a connection to that host.
	// Remove this entry when the org policy names the production registry.
	// apps/mcp-registry/.nais/dev-gcp.yaml is the ingress.
	"mcp-registry.ekstern.dev.nav.no",

	// The Aksel design-system MCP server the aksel agent is built around.
	// agents/aksel.agent.md declares it as a streamable-http MCP endpoint, and
	// skills/aksel-builder/SKILL.md says in as many words that the URL has to
	// be allowlisted for the agent to work.
	"aksel-mcp.nav.no",

	// The documented fallback when that MCP is unavailable: the agent fetches
	// the llm.md index and follows the .md links off it.
	// skills/aksel-builder/SKILL.md, skills/aksel-spacing/SKILL.md.
	"aksel.nav.no",

	// The observability skill hands the agent literal curl commands against
	// Prometheus, Loki and Tempo. Mimir and Loki are single global endpoints;
	// Tempo is per cluster, and every concrete example in the skill uses one of
	// these two. The skill's `dev-fss`/`prod-fss` clusters appear only as a
	// Loki label value, never as a Tempo hostname, so they are left out until
	// something shows Tempo is reachable there.
	// skills/observability-debugging/SKILL.md.
	// grafana.nav.cloud.nais.io is deliberately absent: the skill presents it
	// as a browser link for the human, never as something the agent fetches.
	//
	// These four are the only entries in this list that need a second thing as
	// well. They resolve to private addresses over naisdevice (10.43.0.60 for
	// Mimir and Loki, 10.6.8.200 and 10.7.8.200 for Tempo), and cplt's
	// DNS-rebinding guard refuses a private-resolving host after DNS regardless
	// of this allowlist: "Resolved to a private IP, blocked by cplt". Lifting
	// that is proxy.allow_private_domains, which the pakke asks the user to
	// approve rather than nav-pilot writing it: .nav-pilot/agentpakke.json,
	// policies.propose.cplt. Held equal to this list by
	// TestObservabilityHostsMatchTheProposal.
	//
	// collector-internet.nav.cloud.nais.io above resolves publicly, so it needs
	// this list and nothing more.
	"mimir.nav.cloud.nais.io",
	"loki.nav.cloud.nais.io",
	"tempo.dev-gcp.nav.cloud.nais.io",
	"tempo.prod-gcp.nav.cloud.nais.io",

	// The Entra ID OIDC discovery document for the nav.no tenant, curl'd
	// directly by skills/nav-auth/SKILL.md.
	"login.microsoftonline.com",

	// Nav's Maven mirror. skills/spring-boot-scaffold/SKILL.md writes it into
	// the generated build.gradle.kts repositories block, so an agent that
	// scaffolds a service and then builds it resolves against this host.
	// repo.maven.apache.org being built in does not help.
	"github-package-registry-mirror.gc.nav.no",

	// scripts/install.sh — the documented install path for both nav-pilot and
	// cplt — is `curl https://raw.githubusercontent.com/...  | bash`, and the
	// release assets it then fetches redirect to the object hosts. None of
	// these is covered by cplt's `github.com`: the matcher is
	// exact-or-subdomain and githubusercontent.com is a different apex, of
	// which cplt lists only the copilot-proxy and actions subdomains.
	"raw.githubusercontent.com",
	"objects.githubusercontent.com",
	"release-assets.githubusercontent.com",

	// The Gradle wrapper (gradle-wrapper.properties) downloads the distribution
	// from services.gradle.org, which redirects to github.com and then to
	// release-assets.githubusercontent.com above; github.com is in cplt's list.
	"services.gradle.org",
}

// navAllowedDomainsPath is where nav-pilot keeps the file cplt reads.
//
// `proxy.allowed_domains` takes a *path*, not a list, and cplt re-reads that
// file every few seconds. It lives beside the nav-pilot config rather than
// inside cplt's own so nav-pilot can rewrite it without touching a file the
// user owns.
//
// It is written in full when the user adopts strict. After that a launch only
// appends the Nav hosts it lacks and keeps the MCP section in step
// (syncMCPAllowlist), so a list from an older release catches up.
func navAllowedDomainsPath() string {
	return filepath.Join(filepath.Dir(configPath()), "cplt-allowed-domains.txt")
}

// writeNavAllowedDomains renders navAllowedDomains to disk and returns the path.
// The file is nav-pilot's to rewrite; the comment header says so, because a
// stray hostname in it is a hole in the user's allowlist.
//
// Written by temp-file-and-rename rather than in place. cplt re-reads this file
// every few seconds while a session runs, and a reader that catches an in-place
// write half-done sees a truncated host list — which under strict is not a
// cosmetic glitch but a set of hosts that briefly stop resolving. rename(2) is
// atomic within a directory, so a reader sees either the old file or the new
// one. The temp file is created in the same directory for that reason: a
// rename across filesystems is not atomic and may not be a rename at all.
//
// Same directory permissions as the rest of nav-pilot's config (config_setup.go):
// 0700, because ~/.nav-pilot is personal state.
func writeNavAllowedDomains() (string, error) {
	path := navAllowedDomainsPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	var b strings.Builder
	b.WriteString(navAllowlistHeader + " Edits are overwritten on the next run.\n")
	b.WriteString("# The complete set of hosts nav-pilot and the agents and skills it installs\n")
	b.WriteString("# need. Read by cplt via proxy.allowed_domains.\n")
	b.WriteString("#\n")
	b.WriteString("# Deleting this file does not fail loudly: cplt keeps serving its built-in\n")
	b.WriteString("# list and the Nav hosts below simply stop being reachable.\n")
	own := navAllowedDomains()
	for _, d := range own {
		b.WriteString(d + "\n")
	}
	// The hosts the user approved for their MCP servers, taken from Nav's MCP
	// registry (provider/mcp_hosts.go). A section of its own, below everything
	// above, so it adds and never replaces.
	var mcp []string
	for _, d := range mcpAllowlistHosts() {
		if !slices.Contains(own, d) {
			mcp = append(mcp, d)
		}
	}
	if len(mcp) > 0 {
		b.WriteString(mcpAllowlistMarker + "\n")
		for _, d := range mcp {
			b.WriteString(d + "\n")
		}
	}
	tmp, err := os.CreateTemp(dir, ".cplt-allowed-domains-*")
	if err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	defer os.Remove(tmp.Name()) // no-op once the rename succeeds
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	// CreateTemp makes the file 0600. cplt reads it as the same user, so that
	// is all it needs.
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
}

// navAllowlistHeader opens every allowlist file nav-pilot writes: a file at
// navAllowedDomainsPath without it is not nav-pilot's to change.
const navAllowlistHeader = "# Written by nav-pilot."

// mcpAllowlistMarker opens the MCP section of nav-pilot's allowlist file.
const mcpAllowlistMarker = "# MCP servers you configured: hosts from Nav's MCP registry, approved by you."

// mcpAllowlistHosts is a var so tests need no consent record.
var mcpAllowlistHosts = providerpkg.MCPAllowlistHosts

// syncMCPAllowlist rewrites the MCP section of nav-pilot's allowlist file when
// it no longer matches what is approved. Above it, the bytes are kept as read,
// so no cplt is asked for its hosts at launch, except that a Nav host added to
// navOwnDomains since the file was written is appended there: a static list,
// no spawn and no network. Only a file nav-pilot already wrote, by its path
// and its header: without an allowlist the proxy lets public hosts through
// anyway, a launch must not start writing files the user never asked for, and
// a file the user wrote is theirs.
func syncMCPAllowlist() { syncAllowlist(true) }

// syncAllowlist is syncMCPAllowlist; with mcp false it only appends missing
// Nav hosts and leaves the MCP section byte for byte, for when the consent
// record cannot be read.
func syncAllowlist(mcp bool) {
	path := navAllowedDomainsPath()
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), navAllowlistHeader) {
		return
	}
	prefix, section, hasSection := strings.Cut(string(data), mcpAllowlistMarker+"\n")
	var own []string
	for _, line := range strings.Split(prefix, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			own = append(own, line)
		}
	}
	var missing []string
	for _, d := range navOwnDomains {
		if !slices.Contains(own, d) {
			missing = append(missing, d)
		}
	}
	own = append(own, missing...)
	// Without mcp the section is kept as read, and the record not consulted.
	tail := ""
	if hasSection {
		tail = mcpAllowlistMarker + "\n" + section
	}
	if mcp {
		want := slices.DeleteFunc(slices.Clone(mcpAllowlistHosts()), func(h string) bool { return slices.Contains(own, h) })
		if !slices.Equal(strings.Fields(section), want) {
			tail = ""
			if len(want) > 0 {
				tail = mcpAllowlistMarker + "\n" + strings.Join(want, "\n") + "\n"
			}
		}
	}
	out := prefix
	if len(missing) > 0 {
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += strings.Join(missing, "\n") + "\n"
	}
	out += tail
	if out == string(data) {
		return
	}
	if err := writeFileAtomic(path, []byte(out), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not update %s with the Nav and MCP hosts: %v\n", domain.Yellow("⚠"), path, err)
	}
}

// cpltConfigGet reads one cplt config key. Empty on any failure — callers treat
// that as "not set", which is the safe reading: it makes them seed rather than
// assume a user allowlist exists.
func cpltConfigGet(cliPath, key string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, cliPath, "config", "get", key).Output()
	if err != nil {
		return ""
	}
	first, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(first)
}

// seedCpltAllowlist writes the domains file and points cplt at it, so that
// turning on strict does not take nav-pilot's own network with it.
//
// It will not take over an allowlist the user already has. `allowed_domains`
// holds exactly one path, so repointing it at nav-pilot's file would silently
// revoke every host in theirs — under a preset whose whole point is that
// unlisted hosts are unreachable. In that case the file is still written and
// the path returned, and the caller tells the user to include it.
func seedCpltAllowlist(cliPath string) (path string, adopted bool, err error) {
	mcpConfigScope()
	path, err = writeNavAllowedDomains()
	if err != nil {
		return "", false, err
	}
	if existing := cpltConfigGet(cliPath, "proxy.allowed_domains"); existing != "" && existing != path {
		return path, false, nil
	}
	if err := cpltConfigSet(cliPath, "proxy.allowed_domains", path); err != nil {
		return path, false, err
	}
	return path, true, nil
}

// cpltAgentHostShutOut returns the user's allowlist file and the agent host
// cplt blocks, or "" and "" when there is nothing to warn about.
//
// cplt blocks every host outside a non-empty proxy.allowed_domains, and before
// navikt/cplt#605 it added the agent's own hosts only while
// proxy.default_allowlist was on. A hand-written file without github.com then
// locks Copilot out of its own /login. Rather than re-derive that from the file
// and the cplt version, this reads the battery's own probe of the agent's
// first host: blocked there is the fact, on every cplt version.
func cpltAgentHostShutOut(cliPath string, r *cpltCheckReport) (path, host string) {
	host = r.agentHostBlocked()
	if host == "" {
		return "", ""
	}
	// Without an allowlist the block has another cause, and
	// default_allowlist would not lift it.
	if path = cpltConfigGet(cliPath, "proxy.allowed_domains"); path == "" {
		return "", ""
	}
	return path, host
}

// repairCpltAgentHosts turns on proxy.default_allowlist, which adds cplt's
// built-in list: the agent's hosts and the package registries too. The user's
// file is not touched; rewriting it would mean editing something the user
// owns. Called only after the user has said yes.
func repairCpltAgentHosts(cliPath, path, host string) error {
	if err := cpltConfigSet(cliPath, "proxy.default_allowlist", "true"); err != nil {
		return err
	}
	fmt.Printf("%s cplt proxy.default_allowlist = true: %s shut out %s. This also lets the package registries through.\n",
		domain.Green("✓"), path, host)
	return nil
}
