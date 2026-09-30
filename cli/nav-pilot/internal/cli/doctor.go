package cli

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// reportScopeConflicts names the files a plain sync leaves alone.
//
// doctor had no mention of conflicts at all (#651), so the one command whose
// job is to say what is wrong stayed silent about the state that makes sync
// skip files. It does not set hasErrors: a file that differs from what
// nav-pilot installed is often the team's own deliberate edit, and doctor
// exiting non-zero for that would make a normal, chosen state look broken.
func reportScopeConflicts(scope *InstallScope) {
	conflicts := conflictStatePaths(scope)
	if len(conflicts) == 0 {
		return
	}
	fmt.Printf("      %s %d file(s) differ from what nav-pilot installed and are left alone by a plain sync\n",
		yellow("⚠"), len(conflicts))
	for _, p := range conflicts {
		fmt.Printf("          %s %s\n", dim("⊘"), p)
	}
	fmt.Printf("          %s %s takes the source's version of these too, and saves yours as <file>.orig.\n", yellow("Solution:"), bold("nav-pilot sync --apply"))
}

// reportScopeIntegrity checks a scope's files against what nav-pilot
// installed, and returns true only when files are missing.
//
// A modified file is not a fault. It is often a deliberate local edit, so it
// gets a warning that names the file and does not set hasErrors, like
// [reportScopeConflicts]. It used to share the missing-files failure, with
// advice to "restore missing files" when none were missing (#1319).
func reportScopeIntegrity(scope *InstallScope, state *StateFile) bool {
	ok, modified, missing, _, modifiedPaths := countFileIntegrity(scope.RootDir, state)
	if missing == 0 && modified == 0 {
		fmt.Printf("      %s %d files OK\n", green("✓"), ok)
		return false
	}
	if missing > 0 {
		fmt.Printf("      %s %d missing files\n", red("[✗]"), missing)
		paths := missingPaths(scope.RootDir, state)
		for _, p := range paths {
			fmt.Printf("          %s %s\n", dim("-"), p)
		}
		fmt.Printf("          %s Restore them with:\n", red("Solution:"))
		for _, c := range restoreCommands(scope, state, paths) {
			fmt.Printf("            %s\n", bold(c))
		}
		fmt.Printf("          Deleted on purpose? %s stops tracking them.\n", bold("nav-pilot sync "+scopeFlag(scope)+" --apply"))
	}
	if modified > 0 {
		fmt.Printf("      %s %d file(s) changed since nav-pilot installed them\n", yellow("⚠"), modified)
		for _, p := range modifiedPaths {
			fmt.Printf("          %s %s\n", dim("~"), p)
		}
		fmt.Printf("          %s Keep them if the change is yours, or run %s to overwrite them; your copy is saved as <file>.orig.\n",
			yellow("Solution:"), bold("nav-pilot sync --apply"))
	}
	return missing > 0
}

// restoreCommands names the install command that brings back each of paths,
// files a scope tracks but no longer has.
//
// doctor used to say "run nav-pilot sync", but sync reads a missing file as a
// deliberate deletion and stops tracking it (#1334). Installing the artifact
// again is what restores it. A path that names no artifact gets the scope's
// reinstall command instead.
func restoreCommands(scope *InstallScope, state *StateFile, paths []string) []string {
	origin := map[string]string{}
	for _, f := range state.Files {
		origin[f.Path] = f.Source
	}
	var out []string
	for _, p := range paths {
		cmd := "nav-pilot install <name> --type <type> " + scopeFlag(scope)
		if state.SourceRepo != "" {
			cmd = reinstallCommand(scope, state)
		}
		kind, name := artifactOfPath(p)
		if kind == KindHook {
			name, _ = hookOfPath(scope, p)
		}
		if kind != nil && name != "" {
			cmd = fmt.Sprintf("nav-pilot install %s --type %s %s", name, kind.Name, scopeFlag(scope))
			src := cmp.Or(origin[p], state.SourceRepo)
			if src != "" && !sameSourceRepo(src, defaultSourceRepo) {
				cmd += " --source " + src
			}
		}
		if !slices.Contains(out, cmd) {
			out = append(out, cmd)
		}
	}
	return out
}

// scopeFlag is the flag that picks a scope on the command line.
func scopeFlag(scope *InstallScope) string {
	if scope.IsUser() {
		return "--user"
	}
	return "--repo"
}

// reportScopeIgnoredButInstalled names files marked ignored in state that are
// nonetheless on disk (#724).
//
// doctor is where this belongs: it was doctor that noticed the symptom in the
// first place, a persona pinned to a model GitHub had withdrawn, and said
// nothing about why sync kept skipping the file. Sync reports it too, but a
// user chasing a stale artifact reaches for doctor.
//
// Like the conflict report it does not set hasErrors. The combination is
// usually an older install's residue rather than something broken now, and the
// file may be exactly what someone wants; what is wrong is that nothing said it
// had stopped being maintained.
func reportScopeIgnoredButInstalled(scope *InstallScope) {
	stale := ignoredButInstalled(scope)
	if len(stale) == 0 {
		return
	}
	fmt.Printf("      %s %d file(s) are recorded as ignored but are installed, so sync skips them\n",
		yellow("⚠"), len(stale))
	for _, p := range stale {
		fmt.Printf("          %s %s\n", dim("⊘"), p)
	}
	fmt.Printf("          %s %s brings one back under sync.\n", yellow("Solution:"), bold("nav-pilot add <type> <name> --force"))
}

// reportGoneSource flags a scope installed from a source that is no longer
// there, with the same command sync refuses with. doctor is where someone
// looks once a command has refused, so the two must not disagree, and a scope
// in this state is stuck until someone reinstalls it.
func reportGoneSource(scope *InstallScope, state *StateFile) bool {
	if !goneSource(state) {
		return false
	}
	fmt.Printf("      %s Installed from %s, which is no longer there\n", red("[✗]"), state.SourceRepo)
	fmt.Printf("          %s Reinstall with %s\n", red("Solution:"), bold(reinstallCommand(scope, state)))
	return true
}

// reportCopilotCLI says whether the copilot binary is on PATH, and returns
// false only when it is missing and the client is copilot: for an opencode
// or pi user it is optional, like their clients are for a copilot user.
func reportCopilotCLI() bool {
	// A copilot that is cplt under another name is not the Copilot CLI.
	if p, _ := exec.LookPath("copilot"); p != "" && !providerpkg.IsCplt(p) {
		fmt.Printf("      %s Binary found: %s\n", green("✓"), p)
		return true
	}
	cfg, _ := readConfig()
	if resolve(cfg, CLIOverrides{}).Client != "copilot" {
		fmt.Printf("      [i] Binary not found on PATH (optional)\n")
		return true
	}
	fmt.Printf("      %s Binary not found on PATH\n", red("[✗]"))
	fmt.Printf("          %s Install the Copilot CLI: %s\n", red("Solution:"), bold(providerpkg.CopilotInstallCommand))
	return false
}

// cmdDoctor runs system health checks and outputs actionable diagnostics.
func cmdDoctor() error {
	fmt.Printf("%s\n\n", bold("nav-pilot doctor"))
	hasErrors := false

	// The slow waits (cplt processes, GitHub) are independent of each other,
	// so they start now and run while the earlier sections print. Output stays
	// in the same order: each result is read where it used to be computed.
	cpltPath, _ := exec.LookPath("cplt")
	var enforcement func() *cpltCheckReport
	var latest func() latestCplt
	var preset func() string
	var trust func() trustRead
	var mcpRefresh func() error
	ghProbe := async(func() ghProbeResult {
		st, detail := ghAuthProbe()
		return ghProbeResult{st, detail}
	})
	if cpltPath != "" {
		enforcement = async(cpltEnforcement)
		latest = async(func() latestCplt {
			v, err := latestCpltVersion()
			return latestCplt{v, err}
		})
		preset = async(cpltSandboxPreset)
		trust = async(func() trustRead {
			t, ok := readCpltTrust(cpltPath, "")
			return trustRead{t, ok}
		})
		go cpltBuiltinDomains()
		if mcpHostsMode() != "off" {
			cfg, _ := readConfig()
			providerpkg.MCPClient = resolve(cfg, CLIOverrides{}).Client
			mcpRefresh = async(refreshMCPRegistry)
		}
	}

	// 1. Configuration
	// The file every other command reads, NAV_PILOT_CONFIG included.
	cfgPath := configPath()
	fmt.Printf("[i] Configuration (%s)\n", cfgPath)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("    • File not found (using default values)\n")
			fmt.Printf("      %s To create configuration, run: %s\n", yellow("Solution:"), bold("nav-pilot config init"))
			fmt.Println()
		} else {
			hasErrors = true
			fmt.Printf("    %s Error reading config: %v\n\n", red("[✗]"), err)
		}
	} else {
		var cfg Config
		_, parseErr := toml.Decode(string(data), &cfg)
		if parseErr != nil {
			hasErrors = true
			fmt.Printf("    %s TOML parse error: %v\n", red("[✗]"), parseErr)
			fmt.Printf("      Until it parses, nav-pilot will not launch, and the built-in hooks ignore it\n")
			fmt.Printf("      and run with their defaults (redaction and loop guard on).\n")
			fmt.Printf("      %s Fix syntax in %s or run %s\n\n", red("Solution:"), cfgPath, bold("nav-pilot config validate"))
		} else {
			fmt.Printf("    %s Valid syntax and known keys\n", green("✓"))
			printAutonomyNudge(os.Stdout, &cfg, "    ")
			fmt.Println()
		}
	}

	// 2. Context Installation
	fmt.Printf("[i] Context Installation\n")
	userScope, userErr := ScopeUser()
	var userState *StateFile
	if userErr != nil {
		hasErrors = true
		fmt.Printf("    • User scope (~/.copilot): %s Failed to determine user home: %v\n", red("[✗]"), userErr)
	} else {
		userState, _ = readScopedState(userScope)
		if userState != nil {
			fmt.Printf("    • User scope (~/.copilot): %s\n", bold(userState.Collection))
			hasErrors = reportScopeIntegrity(userScope, userState) || hasErrors
			hasErrors = reportGoneSource(userScope, userState) || hasErrors
			reportScopeConflicts(userScope)
			reportScopeIgnoredButInstalled(userScope)
		} else {
			fmt.Printf("    • User scope (~/.copilot): Not installed\n")
		}
	}

	repoDir, err := os.Getwd()
	var repoState *StateFile
	if err != nil {
		hasErrors = true
		fmt.Printf("    • Repo scope (.github): %s Failed to determine current directory: %v\n", red("[✗]"), err)
	} else {
		repoScope := ScopeRepo(repoDir)
		repoState, _ = readScopedState(repoScope)
		if repoState != nil {
			fmt.Printf("    • Repo scope (.github): %s\n", bold(repoState.Collection))
			hasErrors = reportScopeIntegrity(repoScope, repoState) || hasErrors
			hasErrors = reportGoneSource(repoScope, repoState) || hasErrors
			reportScopeConflicts(repoScope)
			reportScopeIgnoredButInstalled(repoScope)
		} else {
			fmt.Printf("    • Repo scope (.github): Not installed\n")
		}
	}
	if userState == nil && repoState == nil {
		fmt.Printf("      %s Install with %s\n", yellow("Solution:"), bold(installCommandFor(nil, nil)))
	}
	fmt.Println()

	// 2b. Hooks
	//
	// nav-pilot reports GITHUB_COPILOT_PROMPT_MODE_REPO_HOOKS rather than
	// setting it. Setting it would mean writing an undocumented, changelog-only
	// variable into the user's shell to loosen a posture the CLI made
	// secure-by-default on purpose — and an interactive user in a trusted
	// checkout does not need it at all. Prompt mode (-p) in an untrusted folder
	// does, and that is the one case worth naming out loud, because there the
	// gate silently does not load and everything else looks fine.
	fmt.Printf("[i] Hooks\n")
	reportHooks(repoDir, userScope)
	fmt.Println()

	// 3. Client Agents
	fmt.Printf("[i] Client Agents\n")

	// copilot. cplt is only the sandbox: it starts the copilot it finds on
	// PATH, so without one no copilot launch works, sandboxed or not.
	fmt.Printf("    • copilot\n")
	if !reportCopilotCLI() {
		hasErrors = true
	}

	// cplt
	fmt.Printf("    • cplt (sandbox)\n")
	if cpltPath == "" {
		hasErrors = true
		fmt.Printf("      %s Binary not found on PATH\n", red("[✗]"))
		mgr := domain.PkgForInstall()
		fmt.Printf("          %s Install cplt via %s: %s\n", red("Solution:"), mgr.Label,
			bold(mgr.Pick("brew install navikt/tap/cplt", "sudo apt install cplt")))
	} else {
		versionOut, err := runBounded(cpltPath, "--version")
		version := strings.TrimSpace(string(versionOut))
		if err != nil {
			version = "unknown"
		}
		fmt.Printf("      %s Binary found: %s (%s)\n", green("✓"), cpltPath, version)

		l := latest()
		reportCpltVersionWith(cpltPath, version, l.version, l.err)

		// Security posture. No recommendation: standard is the default and
		// lets the agent push a feature branch; strict blocks all pushes.
		// What is reported is a preset cplt cannot honour here.
		preset := preset()
		supported, unsupportedReason := strictPresetSupported()
		switch {
		case preset == "":
			// cplt could not tell us; nothing to say.
		case !supported && preset == cpltStrictPreset:
			// A green check would hide it: strict is set on a machine where
			// cplt refuses to launch under it.
			hasErrors = true
			fmt.Printf("      %s Sandbox preset is %q, which cplt cannot honour here\n", red("[✗]"), preset)
			fmt.Printf("          %s.\n", unsupportedReason)
			fmt.Printf("          %s Run %s, or upgrade the kernel.\n",
				red("Solution:"), bold("cplt config set sandbox.preset standard"))
		default:
			fmt.Printf("      %s Sandbox preset is %s\n", green("✓"), preset)
		}

		// A user allowlist the battery shows blocking the agent's own host.
		// Copilot cannot even log in.
		if path, host := cpltAgentHostShutOut(cpltPath, enforcement()); host != "" {
			hasErrors = true
			fmt.Printf("      %s proxy.allowed_domains (%s) shuts out Copilot's own hosts: cplt blocks %s\n",
				yellow("⚠"), path, host)
			fmt.Printf("          %s Run %s, or %s and pick %s.\n", yellow("Solution:"),
				bold("cplt config set proxy.default_allowlist true"), bold("nav-pilot config"), bold("cplt strict preset"))
		}

		// The waiver the pakke asks for, and whether this scope granted it.
		// Placed with the allowlist rather than with the pakke sections
		// because it is the same question from the other side: the allowlist
		// decides whether a host may be reached at all, this decides whether
		// a host that resolves privately may be reached once DNS has answered.
		reportSandboxWaiver(os.Stdout, agentpakke.Default())
		reportMCPHostsWith(os.Stdout, cpltPath, mcpRefresh)
		reportMCPTools(os.Stdout)

		// The persona is pinned by nav-pilot itself, not by user configuration:
		// BuildCopilotArgs unconditionally emits `cplt --agent copilot --
		// --agent nav-pilot` on every launch. There is nothing to check and
		// nothing for the user to set.
		//
		// This previously grepped `cplt config show` for "nav-pilot". cplt has
		// no such key, so the check always failed and pointed users at
		// `cplt config set copilot.agent_name nav-pilot` — a key that has never
		// existed in cplt, which rejects it with "unknown config key" (#406).
		fmt.Printf("      %s Agent pinned to %s at launch\n", green("✓"), providerpkg.PrimaryAgent("copilot"))
	}

	// opencode
	fmt.Printf("    • opencode\n")
	ocPath, _ := exec.LookPath("opencode")
	if ocPath == "" {
		fmt.Printf("      [i] Binary not found on PATH (optional)\n")
	} else {
		fmt.Printf("      %s Binary found: %s\n", green("✓"), ocPath)
		switch v, tested, err := providerpkg.OpenCodeVersionStatus(); {
		case err != nil:
			fmt.Printf("      %s Could not read the opencode version: %v\n", yellow("⚠"), err)
		case tested:
			fmt.Printf("      %s Version %s is inside the tested range (%s)\n", green("✓"), v, providerpkg.OpenCodeTestedRange)
		default:
			fmt.Printf("      %s Version %s is outside the tested range (%s). Hooks, the dispatch gate and the session policy may not apply as described.\n", yellow("⚠"), v, providerpkg.OpenCodeTestedRange)
		}
		switch listed, unlisted, err := providerpkg.OpenCodeMCPReport(""); {
		case err != nil:
			fmt.Printf("      %s MCP servers not checked against Nav's MCP registry: %v\n", yellow("⚠"), err)
		case len(unlisted) > 0:
			fmt.Printf("      %s MCP servers turned off at launch (not in Nav's MCP registry): %s\n", yellow("⚠"), strings.Join(unlisted, ", "))
			fmt.Printf("          See %s\n", providerpkg.MCPRegistryHelpURL)
		case len(listed) > 0:
			fmt.Printf("      %s MCP servers in Nav's MCP registry: %s\n", green("✓"), strings.Join(listed, ", "))
		}
		// Check opencode context
		configDir, err := os.UserConfigDir()
		if err != nil {
			configDir = filepath.Join(os.Getenv("HOME"), ".config")
		}
		ocDir := filepath.Join(configDir, "opencode")
		ocScope := &InstallScope{Name: "opencode", RootDir: ocDir, StateFile: ".nav-pilot-state.json"}
		ocState, _ := readScopedState(ocScope)
		if ocState != nil {
			ok, _, missing, _, _ := countFileIntegrity(ocDir, ocState)
			if missing > 0 {
				hasErrors = true
				fmt.Printf("      %s Context is missing %d files\n", red("[✗]"), missing)
				fmt.Printf("          %s Run %s to fix.\n", red("Solution:"), bold("nav-pilot sync"))
			} else {
				fmt.Printf("      %s Context securely materialized (%d files OK)\n", green("✓"), ok)
			}
		} else {
			fmt.Printf("      [i] Context not initialized yet\n")
		}
	}

	// pi
	fmt.Printf("    • pi\n")
	piPath, _ := exec.LookPath("pi")
	if piPath == "" {
		fmt.Printf("      [i] Binary not found on PATH (optional)\n")
	} else {
		fmt.Printf("      %s Binary found: %s\n", green("✓"), piPath)
	}

	// Push and PRs from inside the sandbox need a gh login the client can read.
	fmt.Printf("    • GitHub sign-in (push and PRs)\n")
	ghCfg, _ := readConfig()
	gh := ghProbe()
	reportGHAuth(os.Stdout, "      ", resolve(ghCfg, CLIOverrides{}).Client, gh.st, gh.detail)
	fmt.Println()

	// 3b. Model pins
	//
	// Asks the client which models this account can launch, rather than
	// trusting the generated picker: availability is per account and per plan,
	// and the picker is generated from a global catalogue (#717). Warn-only,
	// and an unanswerable question says so.
	fmt.Printf("[i] Model pins\n")
	reportModelPins()
	fmt.Println()

	// 3c. Local model. Config only; alpha local doctor runs the probes.
	fmt.Printf("[i] Local model\n")
	localCfg, _ := readConfig()
	reportLocalModel(os.Stdout, resolve(localCfg, CLIOverrides{}), local.Installed())
	fmt.Println()

	// 4. Project Security
	fmt.Printf("[i] Project Security (.cplt.toml)\n")
	if cpltPath != "" {
		// Its own deadline: an earlier slow check must not be able to starve
		// this one into an empty read, which would print a false "trusted".
		// cplt's own trust verdict (navikt/cplt#644); an older cplt without
		// `trust show --json` gets the config show read.
		if tr := trust(); tr.ok {
			if reportCpltTrust(tr.t) {
				hasErrors = true
			}
		} else {
			cfgOut, cfgErr := runBoundedCombined(cpltPath, "config", "show")
			if reportCpltProjectConfig(string(cfgOut), cfgErr) {
				hasErrors = true
			}
		}

		// Enforcement, probed rather than inferred. Everything above this line
		// reads configuration; `cplt check` runs the probes inside the sandbox
		// the agent would actually get. A repo can be perfectly configured and
		// still not be enforcing.
		switch report := enforcement(); {
		case report == nil:
			fmt.Printf("    %s Could not verify sandbox enforcement\n", dim("-"))
			fmt.Printf("        %s Run %s by hand — this run could not read a verdict, which an older cplt (no such subcommand), a timeout or an interrupted probe all produce.\n", dim("Solution:"), bold("cplt check"))
		case report.tooStrict():
			// Every protection held; the sandbox blocked something it should
			// allow. Too strict, not a leak.
			hasErrors = true
			fmt.Printf("    %s Sandbox is enforcing but too strict (%d protections verified)\n", yellow("⚠"), report.Verified)
			fmt.Printf("        %s Run %s — it names what is blocked and the fix.\n", yellow("Solution:"), bold("cplt check"))
		case report.unverified():
			// Every probe that ran held; some could not run at all.
			fmt.Printf("    %s Could not verify every protection (%d verified, the rest inconclusive)\n", dim("-"), report.Verified)
			fmt.Printf("        %s Run %s — it names each probe that could not run.\n", dim("Solution:"), bold("cplt check"))
		case report.Enforcing:
			fmt.Printf("    %s Sandbox is enforcing (%d protections verified)\n", green("✓"), report.Verified)
		default:
			hasErrors = true
			fmt.Printf("    %s Sandbox is NOT enforcing\n", red("[✗]"))
			fmt.Printf("        %s Run %s — it names each failing probe and its fix.\n", red("Solution:"), bold("cplt check"))
		}
	} else {
		fmt.Printf("    • Skipped (cplt not installed)\n")
	}
	fmt.Println()

	// 4b. naisdevice
	//
	// Only for a machine running a nais agentpakke: its preToolUse gate is the
	// thing that needs naisdevice, and a user without one has no tenant rules
	// to be told about. Nothing here runs at launch — doctor is a command the
	// user asks for, and `nais device status` is a process spawn.
	if naisPakkeInstalled(userState, repoState) {
		fmt.Printf("[i] naisdevice (tenant gate)\n")
		naisPath, _ := exec.LookPath("nais")
		reportNaisdevice(os.Stdout, naisPath, naisStatusFilePath())
		fmt.Println()
	}

	// 5. Dependencies
	fmt.Printf("[i] Dependencies\n")
	checkDep := func(name string) {
		p, _ := exec.LookPath(name)
		if p == "" {
			hasErrors = true
			fmt.Printf("    %s %s: Not found on PATH\n", red("[✗]"), name)
			fmt.Printf("        %s Install %s to use nav-pilot fully.\n", red("Solution:"), name)
		} else {
			fmt.Printf("    %s %s: OK\n", green("✓"), name)
		}
	}
	checkDep("git")
	if reportRtkLeftovers() {
		hasErrors = true
	}
	fmt.Println()

	if hasErrors {
		fmt.Printf("%s Health check complete with warnings. See solutions above.\n", yellow("⚠"))
	} else {
		fmt.Printf("%s All systems healthy! 🚀\n", green("✓"))
	}

	return nil
}

// reportCpltVersion is doctor's cplt version line: behind the latest release,
// current, or could not tell — checked whether or not nav-pilot itself is
// current. Warn-only: nav-pilot never upgrades cplt, and a slow or offline
// GitHub must not fail the health check. An unreadable installed version
// reports unknown, never "up to date".
//
// A cplt without `config hosts` leaves nav-pilot on its frozen copy of cplt's
// host list. That is said once, and the upgrade is offered only when there may
// be one: a current cplt without the subcommand has nothing newer to go to.
func reportCpltVersion(cpltPath, version string) {
	latest, lerr := latestCpltVersion()
	reportCpltVersionWith(cpltPath, version, latest, lerr)
}

func reportCpltVersionWith(cpltPath, version, latest string, lerr error) {
	installed := parseCpltVersion(version)
	upgrade := bold(domain.PkgOwner(cpltPath).Pick("brew upgrade navikt/tap/cplt", "sudo apt upgrade cplt"))
	_, hostsFromCplt := cpltBuiltinDomains()
	const fallback = "`cplt config hosts` gave no usable answer, so nav-pilot uses its own, possibly stale, copy of cplt's host list"
	switch classifyCpltSkew(installed, latest, lerr) {
	case cpltVersionBehind:
		fmt.Printf("      %s cplt %s is out of date (latest: %s)\n", yellow("⚠"), installed, latest)
		if !hostsFromCplt {
			fmt.Printf("          %s %s.\n", dim("Note:"), fallback)
		}
		fmt.Printf("          %s Run %s\n", yellow("Solution:"), upgrade)
	case cpltVersionCurrent:
		fmt.Printf("      %s cplt is up to date\n", green("✓"))
		// Current, yet without the subcommand: nothing newer to upgrade to.
		if !hostsFromCplt {
			fmt.Printf("          %s %s.\n", dim("Note:"), fallback)
		}
	default:
		fmt.Printf("      %s Could not check for a newer cplt release (%s)\n", dim("-"), cpltSkewUnknownReason(installed, latest, lerr))
		if !hostsFromCplt {
			fmt.Printf("      %s cplt may be too old: %s\n", yellow("⚠"), fallback)
			fmt.Printf("          %s Run %s\n", yellow("Solution:"), upgrade)
		}
	}
}

// reportCpltProjectConfig is doctor's .cplt.toml line, from the output of
// `cplt config show` run in the current directory. Reports whether it found a
// problem.
func reportCpltProjectConfig(cfgOut string, cfgErr error) bool {
	switch {
	case strings.Contains(cfgOut, "pending"):
		fmt.Printf("    %s Pending permissions detected!\n", red("[✗]"))
		fmt.Printf("        %s Run %s in this directory to approve new sandbox rules.\n", red("Solution:"), bold("cplt trust"))
		return true
	case cfgErr != nil:
		// Unreadable config is unknown, not trusted: never print a green
		// tick for rules we failed to look at.
		fmt.Printf("    %s Could not read cplt config (%v)\n", yellow("⚠"), cfgErr)
		fmt.Printf("        %s Run %s in this directory to check for pending sandbox rules.\n", yellow("Solution:"), bold("cplt trust"))
		return true
	}
	// cplt prints this header only for a .cplt.toml committed at the repo
	// root, wherever in the repo it is run. A file check in the working
	// directory would miss it from a subdirectory.
	if strings.Contains(cfgOut, "Repo Config (.cplt.toml)") {
		fmt.Printf("    %s .cplt.toml rules are trusted\n", green("✓"))
		return false
	}
	return reportNoRepoConfig()
}

// reportNoRepoConfig is doctor's line for a repository, or a directory, with
// no committed .cplt.toml.
func reportNoRepoConfig() bool {
	switch {
	case source.FindGitRoot(".") == "":
		fmt.Printf("    • Not in a git repository\n")
	default:
		fmt.Printf("    • No committed .cplt.toml in this repository\n")
		// cplt init only prints a preview; --write is the user's call.
		fmt.Printf("        %s Run %s in the repository root to preview the rules cplt suggests for its tooling.\n", dim("Tip:"), bold("cplt init"))
	}
	return false
}

type (
	latestCplt struct {
		version string
		err     error
	}
	trustRead struct {
		t  cpltTrust
		ok bool
	}
	ghProbeResult struct {
		st     ghAuth
		detail string
	}
)

// async starts f now and returns a getter that waits for its result.
func async[T any](f func() T) func() T {
	ch := make(chan T, 1)
	go func() { ch <- f() }()
	return sync.OnceValue(func() T { return <-ch })
}
