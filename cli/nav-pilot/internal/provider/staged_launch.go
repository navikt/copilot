package provider

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// Tier 2 (payload) launch.
//
// A Tier 2 agentpakke ships pre-built, digest-pinned client configuration
// instead of content nav-pilot materializes. internal/cli stages that payload
// into a private tree (agentpakke.StagePayload) and hands the verified
// directory here; these builders turn it into a cplt invocation.
//
// The invocation shape is transcribed from the reference launcher, grillmester
// at 3573b93cc8b7568516117263562d073cae9ee7fc, scripts/grillmester.py
// build_launch_command (lines 647-689). Two of its cplt flags are launcher
// policy rather than payload contract and are deliberately not adopted:
// --no-audit (line 663) and --project-dir (lines 666-667).
//
// Note on provenance: the reference launcher does not stage — it points cplt at
// the payload in place inside its immutable Homebrew bundle. The
// verify -> copy -> re-verify sequence nav-pilot uses comes from the same
// project's local mode (scripts/grillmester_local.py, _materialize_opencode_config).
// nav-pilot has to stage because its source may be a temp clone with umask
// modes rather than the manifest's.
//
// G2: nothing on this path writes to the user's shared client configuration.
// The staged opencode launch deliberately skips EnsureOpenCodeNavContext and
// EnsureOpenCodeOTelConfig — both write into ~/.config/opencode — and never
// edits the payload either, whose bytes are digest-bound. OTel still travels as
// environment variables, which is not config mutation.

// StagedLaunch is the verified payload tree a Tier 2 launch runs against.
type StagedLaunch struct {
	// Dir is the staged payload directory. It is what cplt is allowed to read
	// and what the client is pointed at.
	Dir string
	// PakkeName is the agentpakke identity, used to plugin-qualify the copilot
	// persona (<pakke>:<agent>).
	PakkeName string
	// Context is the payload context id ("full", "focused", …), for the
	// launch message only.
	Context string
}

// suffix names the agentpakke and context in the "Launching …" line.
func (s StagedLaunch) suffix() string {
	return fmt.Sprintf(" with agentpakke %s (%s)", domain.Bold(s.PakkeName), s.Context)
}

// pakkeDeclaredModel returns the active agentpakke's model declaration for a
// client, or "" when it declares none or declares [agentpakke.InheritModel]
// (F1). "inherit" means no --model flag at all, which is also what the
// reference launcher passes: build_launch_command forwards no model.
func pakkeDeclaredModel(client string) string {
	model := source.ActivePakke().DefaultModel(client)
	if model == agentpakke.InheritModel {
		return ""
	}
	return model
}

// stagedPrimaryAgent returns the persona a staged launch starts, failing loudly
// rather than passing an empty --agent if a future call site sets a pakke that
// does not declare the client (see [SetActivePakke]'s invariant).
func stagedPrimaryAgent(client, pakkeName string) (string, error) {
	agent := PrimaryAgent(client)
	if agent == "" {
		return "", fmt.Errorf("agentpakke %q declares no primary agent for %s — it cannot be launched", pakkeName, client)
	}
	return agent, nil
}

// buildStagedOpenCodeSpec builds the cplt invocation for a staged opencode
// launch. Reference: grillmester.py lines 668-677 — --allow-read <payload>,
// OPENCODE_CONFIG_DIR pointing at the same payload, and --pass-env for it, with
// the client receiving --agent <agent>.
func buildStagedOpenCodeSpec(r domain.ResolvedConfig, s StagedLaunch) (cpltLaunch, error) {
	primary, err := stagedPrimaryAgent("opencode", s.PakkeName)
	if err != nil {
		return cpltLaunch{}, err
	}

	env, _ := telemetry.ApplyOpenCodeOTelEnv(os.Environ(), cliVersion)
	env, _ = telemetry.SetEnvValue(env, "OPENCODE_CONFIG_DIR", s.Dir)

	agent := primary
	if r.Mode == "plan" {
		// opencode's built-in read-only planning agent, as on the legacy path.
		agent = "plan"
	}
	agentArgs := []string{"--agent", agent}
	if r.Model != "" {
		agentArgs = append(agentArgs, "--model", ToOpenCodeModel(r.Model))
	} else if model := pakkeDeclaredModel("opencode"); model != "" {
		agentArgs = append(agentArgs, "--model", model)
	}
	agentArgs = append(agentArgs, r.ExtraArgs...)

	return cpltLaunch{
		agent:         "opencode",
		cpltArgs:      []string{"--allow-read", s.Dir, "--pass-env", "OPENCODE_CONFIG_DIR"},
		agentArgs:     agentArgs,
		env:           env,
		displayName:   "opencode",
		messageSuffix: s.suffix(),
	}, nil
}

// buildStagedCopilotSpec builds the cplt invocation for a staged copilot
// launch. Reference: grillmester.py lines 668-669 and 679-685 — --allow-read
// <plugin> on the cplt side, and --plugin-dir <plugin> before
// --agent <pakke>:<agent> on the client side.
func buildStagedCopilotSpec(r domain.ResolvedConfig, s StagedLaunch) (cpltLaunch, error) {
	primary, err := stagedPrimaryAgent("copilot", s.PakkeName)
	if err != nil {
		return cpltLaunch{}, err
	}

	agentArgs := []string{"--plugin-dir", s.Dir, "--agent", s.PakkeName + ":" + primary}
	agentArgs = append(agentArgs, copilotResolvedFlags(r)...)
	if r.Model != "" {
		agentArgs = append(agentArgs, "--model", r.Model)
	} else if model := pakkeDeclaredModel("copilot"); model != "" {
		agentArgs = append(agentArgs, "--model", model)
	}
	agentArgs = append(agentArgs, r.ExtraArgs...)

	return cpltLaunch{
		agent:         "copilot",
		cpltArgs:      []string{"--allow-read", s.Dir},
		agentArgs:     agentArgs,
		env:           CopilotEnv(r.OtelLogLevel),
		displayName:   CLIDisplayName("cplt"),
		messageSuffix: s.suffix(),
	}, nil
}

// LaunchOpenCodeStaged launches opencode against a staged Tier 2 payload.
func LaunchOpenCodeStaged(r domain.ResolvedConfig, s StagedLaunch) error {
	if _, err := exec.LookPath("opencode"); err != nil {
		return fmt.Errorf("opencode not found in PATH — install it first: https://opencode.ai")
	}
	spec, err := buildStagedOpenCodeSpec(r, s)
	if err != nil {
		return err
	}
	for _, msg := range OpenCodeUnsupportedConfigWarnings(r) {
		fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), msg)
	}
	return launchViaCplt(spec)
}

// LaunchCopilotStaged launches copilot against a staged Tier 2 payload.
//
// cplt is required here even though the legacy copilot path still accepts a
// plain copilot binary: a payload launch is defined by the sandbox flags
// (--allow-read over the staged tree), and running it unsandboxed is not the
// contract anyone reviewed. There is no fallback and no confirmation prompt.
func LaunchCopilotStaged(r domain.ResolvedConfig, s StagedLaunch) error {
	if _, name := FindCopilotCLI(); name != "cplt" {
		telemetryRecorder.RecordLaunchError("copilot", "client_not_found")
		return fmt.Errorf(
			"launching agentpakke %q requires the cplt sandbox, which is not in PATH.\n"+
				"A Tier 2 agentpakke ships pre-built payloads that nav-pilot only hands to a sandboxed client.\n\n"+
				"  Install it: %s",
			s.PakkeName, domain.Bold("brew install navikt/tap/cplt"))
	}
	spec, err := buildStagedCopilotSpec(r, s)
	if err != nil {
		return err
	}
	PrintCpltSandboxHint()
	PrintModelAvailabilityHint(r.Model)
	return launchViaCplt(spec)
}
