package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// cpltConfigSet writes one cplt config key.
func cpltConfigSet(cliPath, key, val string) error {
	out, err := exec.Command(cliPath, "config", "set", key, val).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set %s: %v\n%s", key, err, string(out))
	}
	return nil
}

// cmdConfigSandbox runs an interactive wizard to configure the cplt sandbox profile.
func cmdConfigSandbox() error {
	cliPath, err := findCplt()
	if err != nil {
		return err
	}

	var choices []string
	err = huh.NewMultiSelect[string]().
		Title("Configure cplt sandbox relaxations").
		Description("Select which restrictions to lift for agents running under cplt.").
		Options(
			huh.NewOption("Allow Docker (Colima/OrbStack)", "sandbox.allow_docker"),
			huh.NewOption("Allow any localhost port", "sandbox.allow_localhost_any"),
			huh.NewOption("Allow browser access", "sandbox.allow_browser"),
			huh.NewOption("Allow executing /tmp binaries", "sandbox.allow_tmp_exec"),
		).
		Value(&choices).
		WithTheme(navTheme()).
		Run()

	if err != nil {
		return fmt.Errorf("prompt cancelled: %w", err)
	}

	keys := []string{"sandbox.allow_docker", "sandbox.allow_localhost_any", "sandbox.allow_browser", "sandbox.allow_tmp_exec"}
	for _, key := range keys {
		val := "false"
		for _, c := range choices {
			if c == key {
				val = "true"
				break
			}
		}
		if err := cpltConfigSet(cliPath, key, val); err != nil {
			return err
		}
	}

	fmt.Printf("%s Successfully updated cplt sandbox configuration\n", domain.Green("✓"))
	return nil
}

// ─── security posture (sandbox.preset) ───────────────────────────────────────

// cpltRecommendedPreset is the sandbox preset nav-pilot recommends.
//
// What it buys over `standard` is narrower than it used to be. cplt#335 turned
// gh_guard and git_guard on in `standard` too, so naming those as the reason is
// now stale advice. What strict still adds is the network: forced-proxy egress,
// the git guard escalated from warn to block, and `proxy.default_allowlist` —
// which restricts egress to cplt's built-in host list plus whatever
// `proxy.allowed_domains` names, and blocks everything else
// (cplt src/config/types.rs, `Preset::Strict.baseline()`).
//
// That last one is why the recommendation is never made on its own: see
// navAllowedDomains below. Individually set keys still override the preset, so
// it stays safe to recommend to users who have tuned cplt already.
const cpltRecommendedPreset = "strict"

// cpltStrictConsequence is the one-paragraph version of what strict does, used
// wherever nav-pilot recommends it. It leads with the consequence rather than
// the feature list, because the feature list is what went stale.
const cpltStrictConsequence = "Locks egress down: only cplt's built-in host list plus proxy.allowed_domains " +
	"stay reachable, and the git guard blocks rather than warns. nav-pilot seeds the allowlist " +
	"with the Nav hosts it and your agents need, or they go dark. Keys you set yourself still win."

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

// cpltRecommendStrict reports whether nav-pilot should nudge the user towards
// the strict preset. An unknown preset is left alone, and so is a machine where
// strict would not work — see strictPresetSupported.
//
// The platform gate lives here rather than at the call sites because both the
// doctor report and the settings page route through this one predicate. A gate
// added to only one of them is a recommendation nav-pilot still makes.
func cpltRecommendStrict(preset string) bool {
	if preset == "" || preset == cpltRecommendedPreset {
		return false
	}
	ok, _ := strictPresetSupported()
	return ok
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
	// The row is selectable even where the recommendation is withheld, so the
	// refusal is repeated here rather than assumed from the row being hidden.
	if ok, reason := strictPresetSupported(); !ok {
		return fmt.Errorf("nav-pilot will not set sandbox.preset = strict here: %s", reason)
	}

	var ok bool
	if err := huh.NewConfirm().
		Title("Set cplt sandbox.preset = strict?").
		Description(cpltStrictConsequence).
		Value(&ok).
		WithTheme(navTheme()).
		Run(); err != nil {
		return fmt.Errorf("prompt cancelled: %w", err)
	}
	if !ok {
		return nil
	}

	return applyStrictPreset(cliPath)
}

// applyStrictPreset is everything cmdConfigStrictPreset does once the user has
// said yes. Split out so the seed-then-set order — the part that matters — is
// testable against a real cplt on PATH, without a terminal.
func applyStrictPreset(cliPath string) error {
	path, adopted, err := seedCpltAllowlist(cliPath)
	if err != nil {
		return err
	}
	if adopted {
		fmt.Printf("%s cplt proxy.allowed_domains = %s (%d Nav hosts)\n",
			domain.Green("✓"), path, len(navAllowedDomains))
	} else {
		fmt.Printf("%s You already have proxy.allowed_domains set, so nav-pilot left it alone.\n",
			domain.Yellow("⚠"))
		fmt.Printf("  Add the hosts in %s to your own file, or strict will block them.\n", path)
	}

	if err := cpltConfigSet(cliPath, "sandbox.preset", cpltRecommendedPreset); err != nil {
		return err
	}
	fmt.Printf("%s cplt sandbox.preset = %s\n", domain.Green("✓"), cpltRecommendedPreset)
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
// The recommendation therefore comes with the hosts. cplt merges
// `proxy.allowed_domains` into the built-in list rather than replacing it, so
// this file only has to carry the delta.

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
// package registries and nothing else. An opencode session on nav-pilot's
// default `github-copilot/auto` model could not reach a model host at all.
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
var navAllowedDomains = append(append([]string{}, cpltBuiltinDomains...), navOwnDomains...)

// cpltBuiltinDomains mirrors cplt's own built-in allowlists for the three
// agents nav-pilot launches: COPILOT_INFRA_DOMAINS, OPENCODE_DOMAINS and
// PACKAGE_REGISTRY_DOMAINS in cplt src/agent.rs.
//
// Copied rather than referenced because there is nothing to reference — cplt
// exposes the list to `cplt --observe-domains` and to its own proxy, not to a
// config command. A copy can go stale, so the cost of it being wrong is worth
// stating: a host cplt adds later and nav-pilot does not is one an agent cannot
// reach under strict, which is a visible failure with a one-line fix here. The
// reverse — a host cplt drops — leaves an entry that was already reachable.
// Neither silently weakens anything, because everything here is already in
// cplt's own default-on list.
var cpltBuiltinDomains = []string{
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
}

// navAllowedDomainsPath is where nav-pilot keeps the file cplt reads.
//
// `proxy.allowed_domains` takes a *path*, not a list, and cplt re-reads that
// file every few seconds. It lives beside the nav-pilot config rather than
// inside cplt's own so nav-pilot can rewrite it without touching a file the
// user owns.
//
// It is written on exactly one path today — the posture action — so a user who
// adopts strict keeps whatever list shipped that day. Refreshing it when it is
// already the configured allowlist is the obvious next step and is not in this
// change.
func navAllowedDomainsPath() string {
	return filepath.Join(filepath.Dir(configPath()), "cplt-allowed-domains.txt")
}

// writeNavAllowedDomains renders navAllowedDomains to disk and returns the path.
// The file is nav-pilot's to rewrite; the comment header says so, because a
// stray hostname in it is a hole in the user's allowlist.
func writeNavAllowedDomains() (string, error) {
	path := navAllowedDomainsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	var b strings.Builder
	b.WriteString("# Written by nav-pilot. Edits are overwritten on the next run.\n")
	b.WriteString("# The complete set of hosts nav-pilot and the agents and skills it installs\n")
	b.WriteString("# need. Read by cplt via proxy.allowed_domains.\n")
	b.WriteString("#\n")
	b.WriteString("# Deleting this file does not fail loudly: cplt keeps serving its built-in\n")
	b.WriteString("# list and the Nav hosts below simply stop being reachable.\n")
	for _, d := range navAllowedDomains {
		b.WriteString(d + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
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
