package provider

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// Tier 2 (payload) launch.
//
// A Tier 2 agentpakke ships pre-built, digest-pinned client configuration
// instead of content nav-pilot materializes. internal/cli materializes that
// payload once, at install or at first launch, into a content-addressed
// revision under ~/.nav-pilot/pakker/, re-verifies the pinned tree exactly at
// launch (agentpakke.VerifyPayloadExact) and hands the verified directory
// here; these builders turn it into a cplt invocation.
//
// The invocation shape is transcribed from the reference launcher, grillmester
// at 3573b93cc8b7568516117263562d073cae9ee7fc, scripts/grillmester.py
// build_launch_command (lines 647-689).
//
// --no-audit (line 663) is NOT adopted, and no longer emitted. It was on these
// vectors for one reason (#437, comment 5437575432): at the cplt baseline
// grillmester v0.3.0 was tested against, cplt's parent-side audit could execute
// repository-controlled Git helpers *outside* the sandbox. navikt/cplt#211
// removed that, and the staged gate now requires a release containing it —
// minStagedCpltStamp in runtime_gate.go carries the evidence. The condition
// eSyfo set for dropping the flag was exactly this pair: a reviewed cplt
// baseline that fixes the behaviour, plus a runtime gate that enforces the
// baseline. Suppressing the post-session change audit on every staged launch is
// a cost, not a saving, once the escape it bought is gone.
//
// --project-dir (lines 666-667) stays omitted: nav-pilot treats the working
// directory as the project scope. No launch path sets cmd.Dir, so cplt and the
// client inherit the user's cwd, which is what the reference passes explicitly.
// eSyfo accept the omission on exactly that condition.
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
	// Context is the payload context id ("full", "focused", …). It selects
	// the payload whose primaryAgents roster the launch reads, and names the
	// context in the launch message.
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

// pakkeAcceptsUserContext reports whether the agentpakke declares that it
// accepts the user's own client customizations (~/.copilot instructions and
// agents) being mixed into its session.
//
// No manifest field carries that declaration yet — the proposal is
// `acceptsUserContext` on the client entry, see #437, which needs the contract
// owners' agreement before it goes into schemas/agentpakke-v1.json. Absent a
// declaration nothing is mixed in: a third-party pakke's session must not
// silently receive Nav content its author never tested against. When the field
// lands this becomes a one-line read of the manifest, like pakkeDeclaredModel:
//
//	return source.ActivePakke().AcceptsUserContext(client)
func pakkeAcceptsUserContext(client string) bool {
	return false
}

// openCodeSubcommands are opencode's own subcommands, transcribed from the
// reference launcher's OPENCODE_COMMANDS (grillmester.py lines 37-63). A
// forwarded argument vector starting with one of these is not a session entry
// point, so no --agent may be bound to it.
var openCodeSubcommands = map[string]bool{
	"acp": true, "agent": true, "attach": true, "auth": true, "completion": true,
	"db": true, "debug": true, "export": true, "github": true, "import": true,
	"mcp": true, "models": true, "plugin": true, "plug": true, "pr": true,
	"providers": true, "run": true, "serve": true, "session": true, "stats": true,
	"uninstall": true, "upgrade": true, "web": true,
}

// openCodeClientArgs binds the agent selection only to opencode entry points
// that accept it. Transcribed from the reference launcher's
// _opencode_client_arguments (grillmester.py lines 692-704):
//
//	line 698-699  no forwarded arguments      -> <bind>
//	line 700-701  "run" ...                   -> run <bind> ...
//	line 702-703  another opencode subcommand -> forwarded unchanged, no --agent
//	line 704      anything else               -> <bind> ...
//
// bind is "--agent <agent>" plus the resolved --model, which is only meaningful
// wherever --agent is; the reference forwards no model at all.
func openCodeClientArgs(bind, forwarded []string) []string {
	switch {
	case len(forwarded) == 0:
		return bind
	case forwarded[0] == "run":
		return append(append([]string{"run"}, bind...), forwarded[1:]...)
	case openCodeSubcommands[forwarded[0]]:
		return slices.Clone(forwarded)
	default:
		return append(slices.Clone(bind), forwarded...)
	}
}

// containsOption reports whether an argument vector carries an option, in
// either the "--opt value" or "--opt=value" spelling. Transcribed from the
// reference launcher's _contains_option (grillmester.py lines 602-603 at
// 3573b93cc8b7568516117263562d073cae9ee7fc).
func containsOption(args []string, option string) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		return a == option || strings.HasPrefix(a, option+"=")
	})
}

// rejectReservedClientArgs refuses client arguments that a Tier 2 launch owns.
// Transcribed from the reference launcher's _reject_reserved_arguments
// (grillmester.py lines 633-643):
//
//	line 633-636  client --agent
//	line 637-640  client --project-dir
//	line 641-643  copilot --plugin-dir
//
// These select what the session actually runs, and on this path that is fixed
// by the digest-verified payload: a forwarded --plugin-dir would append an
// unverified plugin directory to a verified session. The reference's cplt-side
// checks (lines 613-632) have no counterpart here — nav-pilot builds its cplt
// argument vector itself and forwards nothing of the user's into it.
//
// Refused, not dropped: the user typed it and deserves to be told why it did
// not take effect. Only the staged path is guarded; the legacy path has no
// verified payload to protect.
func rejectReservedClientArgs(client, pakkeName string, args []string) error {
	reserved := []string{"--agent", "--project-dir"}
	if client == "copilot" {
		reserved = append(reserved, "--plugin-dir")
	}
	for _, option := range reserved {
		if containsOption(args, option) {
			return fmt.Errorf("%s is owned by agentpakke %q's verified payload and cannot be passed to %s after --", option, pakkeName, client)
		}
	}
	return nil
}

// stagedPrimaryAgent returns the persona a staged launch starts: the first
// agent the launched *context's* payload declares, not the client entry's. It
// fails loudly rather than passing an empty --agent if a future call site sets
// a pakke that does not declare the client or the context (see
// [SetActivePakke]'s invariant).
func stagedPrimaryAgent(client, context, pakkeName string) (string, error) {
	agent := PrimaryAgentFor(client, context)
	if agent == "" {
		return "", fmt.Errorf("agentpakke %q declares no primary agent for %s context %q — it cannot be launched", pakkeName, client, context)
	}
	return agent, nil
}

// buildStagedOpenCodeSpec builds the cplt invocation for a staged opencode
// launch. Reference: grillmester.py lines 668-677 — --allow-read <payload>,
// OPENCODE_CONFIG_DIR pointing at the same payload, and --pass-env for it, with
// the client receiving --agent <agent>.
func buildStagedOpenCodeSpec(r domain.ResolvedConfig, s StagedLaunch) (cpltLaunch, error) {
	// Same refusal the staged Copilot path makes, for the same reason: a pakke
	// launches from a digest-verified payload built and tested against the model
	// its manifest declares, and nobody reviewed it running on a 4-bit model on a
	// laptop. This path did no local setup at all, so a staged launch with local
	// enabled got no worker binding, no dispatch fragment and no loop guard, and
	// said nothing about it — the developer saw a session that simply never
	// dispatched.
	if local.IsLocal(r.Model) {
		return cpltLaunch{}, fmt.Errorf(
			"%s is a local model, and agentpakke %q launches from a digest-verified payload that nav-pilot does not point at a server on this machine.\n\n  Launch the pakke on its declared model, or run a local session without it: %s",
			r.Model, s.PakkeName, domain.Bold("nav-pilot --client opencode"))
	}
	if err := rejectReservedClientArgs("opencode", s.PakkeName, r.ExtraArgs); err != nil {
		return cpltLaunch{}, err
	}
	primary, err := stagedPrimaryAgent("opencode", s.Context, s.PakkeName)
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
	bind := []string{"--agent", agent}
	if r.Model != "" {
		bind = append(bind, "--model", ToOpenCodeModel(r.Model))
	} else if model := pakkeDeclaredModel("opencode"); model != "" {
		bind = append(bind, "--model", model)
	}
	agentArgs := openCodeClientArgs(bind, r.ExtraArgs)

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
// launch. Reference: grillmester.py lines 668-669 and 679-685 —
// --allow-read <plugin> on the cplt side, and --plugin-dir <plugin> before
// --agent <pakke>:<agent> on the client side.
func buildStagedCopilotSpec(r domain.ResolvedConfig, s StagedLaunch) (cpltLaunch, error) {
	if err := rejectReservedClientArgs("copilot", s.PakkeName, r.ExtraArgs); err != nil {
		return cpltLaunch{}, err
	}
	primary, err := stagedPrimaryAgent("copilot", s.Context, s.PakkeName)
	if err != nil {
		return cpltLaunch{}, err
	}

	model := r.Model
	if model == "" {
		model = pakkeDeclaredModel("copilot")
	}
	// The refusal the legacy path no longer needs, kept where it is still true.
	// A local session is BYOK: COPILOT_PROVIDER_BASE_URL replaces the model
	// routing for the whole session, and GitHub authentication stops being
	// required with it. A Tier 2 launch is defined by a digest-verified payload
	// built and tested against the model its manifest declares, and nobody
	// reviewed it running on a 4-bit model on a laptop — so this is refused
	// rather than redirected. LaunchCopilotStaged reached the old refusal at no
	// point, which is why a staged launch on a local model id went to GitHub.
	if local.IsLocal(model) {
		return cpltLaunch{}, fmt.Errorf(
			"%s is a local model, and agentpakke %q launches from a digest-verified payload that nav-pilot does not point at a server on this machine.\n\n  Launch the pakke on its declared model, or run a local session without it: %s",
			model, s.PakkeName, domain.Bold("nav-pilot --client copilot"))
	}
	agentArgs := []string{"--plugin-dir", s.Dir, "--agent", s.PakkeName + ":" + primary}
	agentArgs = append(agentArgs, copilotResolvedFlags(r)...)
	if model != "" {
		agentArgs = append(agentArgs, "--model", model)
	}
	agentArgs = append(agentArgs, r.ExtraArgs...)

	return cpltLaunch{
		agent:         "copilot",
		cpltArgs:      []string{"--allow-read", s.Dir},
		agentArgs:     agentArgs,
		env:           copilotEnv(r.OtelLogLevel, pakkeAcceptsUserContext("copilot")),
		displayName:   CLIDisplayName("cplt"),
		messageSuffix: s.suffix(),
	}, nil
}

// LaunchOpenCodeStaged launches opencode against a staged Tier 2 payload.
func LaunchOpenCodeStaged(r domain.ResolvedConfig, s StagedLaunch) error {
	if _, err := exec.LookPath("opencode"); err != nil {
		return fmt.Errorf("opencode not found in PATH — install it first: https://opencode.ai")
	}
	if err := checkStagedRuntime("opencode", pakkeCompatibility("opencode")); err != nil {
		return err
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
	if err := checkStagedRuntime("copilot", pakkeCompatibility("copilot")); err != nil {
		return err
	}
	spec, err := buildStagedCopilotSpec(r, s)
	if err != nil {
		return err
	}
	PrintCpltSandboxHint()
	PrintModelAvailabilityHint(r.Model)
	return launchViaCplt(spec)
}
