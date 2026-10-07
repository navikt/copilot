package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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
// --no-audit (line 663) is adopted, in the reference's position: first in the
// cplt vector, before --agent. It was first read as launcher policy; team eSyfo
// corrected that (#437, comment 5437575432): at the cplt baseline grillmester
// v0.3.0 is tested against, cplt's parent-side audit can execute
// repository-controlled Git helpers *outside* the sandbox, so a staged launch
// without it is less isolated than the launcher it is meant to be equivalent
// to. Dropping it again requires evidence that a reviewed cplt baseline fixes
// that behaviour, plus a cplt minimum-version gate that enforces the baseline —
// not the flag looking redundant.
//
// --project-dir (lines 666-667) is passed, as the reference does: the working
// directory, or the user's own nav-pilot --project-dir. It used to be omitted
// on the belief that cplt would inherit the cwd, but cplt widens a cwd inside a
// git repository to the repository root, so a session started from a subfolder
// was scoped to the whole checkout. launchViaCplt adds it for every launch;
// see cpltProjectDir.
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
// EnsureOpenCodeConfig — both write into ~/.config/opencode — and never
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
// client, trimmed, or "" when it declares none or declares
// [agentpakke.InheritModel] (F1). "inherit" means no --model flag at all,
// which is also what the reference launcher passes: build_launch_command
// forwards no model.
//
// Trimmed before the sentinel comparison: the schema only requires
// defaultModel to be a string, so "  inherit  " is a valid declaration that
// would otherwise miss this check, reach ToOpenCodeModel unrecognized, and be
// sent to opencode as the literal (and invalid) "github-copilot/inherit".
func pakkeDeclaredModel(client string) string {
	model := strings.TrimSpace(source.ActivePakke().DefaultModel(client))
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
// wherever --agent is; the reference forwards no model at all. Only `run`
// accepts --variant (anomalyco/opencode#7354, PR #7358).
func openCodeRunArgs(forwarded []string) bool {
	return len(forwarded) > 0 && (forwarded[0] == "run" || len(forwarded) > 1 && forwarded[0] == "--pure" && forwarded[1] == "run")
}

func openCodeClientArgs(bind, forwarded []string, variant string) []string {
	if openCodeRunArgs(forwarded) && variant != "" {
		bind = append(slices.Clone(bind), "--variant", variant)
	}
	switch {
	case len(forwarded) == 0:
		return bind
	case forwarded[0] == "run":
		return append(append([]string{"run"}, bind...), forwarded[1:]...)
	case len(forwarded) > 1 && forwarded[0] == "--pure" && forwarded[1] == "run":
		return append(append([]string{"--pure", "run"}, bind...), forwarded[2:]...)
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
// argument vector itself, and the only user input it forwards into it is the
// directory named by nav-pilot's own --project-dir.
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
// launch. Reference: grillmester.py line 663 — --no-audit — and lines 668-677
// — --allow-read <payload>,
// OPENCODE_CONFIG_DIR pointing at the same payload, and --pass-env for it, with
// the client receiving --agent <agent>.
func buildStagedOpenCodeSpec(r domain.ResolvedConfig, s StagedLaunch) (cpltLaunch, error) {
	// Resolved before the local-model refusal below, not just read off
	// r.Model: "" and "auto"/the legacy alias can still resolve to a payload
	// pakke's own declared default (the same fallback ToOpenCodeModel applies
	// further down), so a pakke that declares a local model has to be caught
	// here too, not only a model the user pinned directly.
	model := r.Model
	if isOpenCodeUnsetModel(model) {
		model = pakkeDeclaredModel("opencode")
	}

	// Same refusal the staged Copilot path makes, for the same reason: a pakke
	// launches from a digest-verified payload built and tested against the model
	// its manifest declares, and nobody reviewed it running on a 4-bit model on a
	// laptop. This path did no local setup at all, so a staged launch with local
	// enabled got no worker binding, no dispatch fragment and no loop guard, and
	// said nothing about it — the developer saw a session that simply never
	// dispatched.
	if local.IsLocal(model) {
		return cpltLaunch{}, fmt.Errorf(
			"%s is a local model, and agentpakke %q launches from a digest-verified payload that nav-pilot does not point at a server on this machine.\n\n  Launch the pakke on its declared model, or run a local session without it: %s",
			model, s.PakkeName, domain.Bold("nav-pilot --client opencode"))
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
	if r.Autonomy == "sandbox" || r.AllowAllTools {
		bind = append(bind, "--auto") // cplt is the boundary, as in OpenCodeArgs
	}
	// Routed through ToOpenCodeModel rather than appended raw: it passes an
	// already-qualified id through unchanged, prefixes a bare one, and maps
	// "", "auto", and the legacy alias to "" so the flag is omitted and
	// opencode picks for itself.
	if resolved := ToOpenCodeModel(model); resolved != "" {
		bind = append(bind, "--model", resolved)
	}
	agentArgs := openCodeClientArgs(bind, r.ExtraArgs, r.ReasoningEffort)
	cpltArgs := []string{"--allow-read", s.Dir, "--pass-env", "OPENCODE_CONFIG_DIR"}
	if openCodeMajor() >= 2 {
		agentArgs, env = openCodeV2Args(agentArgs, env)
		env = withOpenCode2UserConfig(env, s.Dir)
		// Not in cplt's allowlist: without it the user's config is gone.
		if slices.ContainsFunc(env, func(e string) bool { return strings.HasPrefix(e, "OPENCODE_CONFIG=") }) {
			cpltArgs = append(cpltArgs, "--pass-env", "OPENCODE_CONFIG")
		}
	}

	return cpltLaunch{
		agent:         "opencode",
		noAudit:       true,
		cpltArgs:      cpltArgs,
		skillsDir:     materializedSkillsDir(s.Dir),
		agentArgs:     agentArgs,
		env:           env,
		displayName:   "opencode",
		messageSuffix: s.suffix(),
		projectDir:    r.ProjectDir,
	}, nil
}

// buildStagedPiSpec builds the cplt invocation for a staged pi launch.
//
// pi has no --agent, so the payload's primary reaches it as its system prompt:
// --append-system-prompt takes a file, and the roster only decides which file.
// Skills are passed by path rather than discovered, because pi's own auto-load
// paths are project-local and a staged payload is not in the project.
func buildStagedPiSpec(r domain.ResolvedConfig, s StagedLaunch) (cpltLaunch, error) {
	// Resolved before the local-model refusal below, not just read off
	// r.Model: an unset model still falls through to the pakke's own
	// declaration two lines down, and a pakke that declares a local model has
	// to be caught here too, the same class of gap buildStagedOpenCodeSpec had.
	model := r.Model
	if model == "" {
		model = pakkeDeclaredModel("pi")
	}
	if local.IsLocal(model) {
		return cpltLaunch{}, fmt.Errorf(
			"%s is a local model, and agentpakke %q launches from a digest-verified payload that nav-pilot does not point at a server on this machine.\n\n  Launch the pakke on its declared model, or run a local session without it: %s",
			model, s.PakkeName, domain.Bold("nav-pilot --client pi"))
	}
	if err := rejectReservedClientArgs("pi", s.PakkeName, r.ExtraArgs); err != nil {
		return cpltLaunch{}, err
	}
	primary, err := stagedPrimaryAgent("pi", s.Context, s.PakkeName)
	if err != nil {
		return cpltLaunch{}, err
	}

	agentArgs := piSkillArgs(s.Dir, primary)
	agentArgs = append(agentArgs, piModelArg(model)...)
	agentArgs = append(agentArgs, r.ExtraArgs...)

	return cpltLaunch{
		agent:         "pi",
		noAudit:       true,
		cpltArgs:      []string{"--allow-read", s.Dir},
		skillsDir:     materializedSkillsDir(s.Dir),
		agentArgs:     agentArgs,
		displayName:   "pi",
		messageSuffix: s.suffix(),
		projectDir:    r.ProjectDir,
	}, nil
}

// LaunchPiStaged launches pi from a verified Tier 2 payload.
func LaunchPiStaged(r domain.ResolvedConfig, s StagedLaunch) error {
	if _, err := exec.LookPath("pi"); err != nil {
		return fmt.Errorf("pi not found in PATH — install it first, or set a different client with: nav-pilot config set client copilot")
	}
	if err := checkStagedRuntime("pi", pakkeCompatibility("pi")); err != nil {
		return err
	}
	spec, err := buildStagedPiSpec(r, s)
	if err != nil {
		return err
	}
	for _, msg := range PiUnsupportedConfigWarnings(r) {
		fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), msg)
	}
	return launchViaCplt(spec)
}

// buildStagedCopilotSpec builds the cplt invocation for a staged copilot
// launch. Reference: grillmester.py line 663 and lines 668-669 and 679-685 —
// --no-audit and --allow-read <plugin> on the cplt side, and
// --plugin-dir <plugin> before
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
	model, _ = CopilotModelID(model)
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
	// The user's Copilot hooks run in a pakke session too, the action check
	// among them.
	env, checkFlags := withActionCheckServer(r, copilotEnv(r.OtelLogLevel, pakkeAcceptsUserContext("copilot")))

	return cpltLaunch{
		agent:         "copilot",
		noAudit:       true,
		cpltArgs:      append([]string{"--allow-read", s.Dir}, checkFlags...),
		skillsDir:     materializedSkillsDir(s.Dir),
		agentArgs:     agentArgs,
		env:           env,
		displayName:   CLIDisplayName("cplt"),
		messageSuffix: s.suffix(),
		projectDir:    r.ProjectDir,
	}, nil
}

// LaunchOpenCodeStaged launches opencode against a staged Tier 2 payload.
func LaunchOpenCodeStaged(r domain.ResolvedConfig, s StagedLaunch) error {
	if _, err := exec.LookPath("opencode"); err != nil {
		return fmt.Errorf("opencode not found in PATH — install it first: https://opencode.ai")
	}
	if err := CheckOpenCodeMajor(); err != nil {
		telemetryRecorder.RecordLaunchError("opencode", "client_unsupported")
		return err
	}
	if err := checkOpenCode2Launch(r.ExtraArgs); err != nil {
		reason := "client_unsupported"
		if errors.Is(err, errCpltTooOld) {
			reason = "cplt_too_old"
		}
		telemetryRecorder.RecordLaunchError("opencode", reason)
		return err
	}
	// A fresh machine has no .gitignore in the opencode config dir, and under
	// cplt the launch dies before the TUI if OpenCode has to create it itself
	// (#565).
	if err := ensureOpenCodeRuntimeGitignore(); err != nil {
		return fmt.Errorf("preparing opencode's config directory for the sandbox: %w", err)
	}
	if err := checkStagedRuntime("opencode", pakkeCompatibility("opencode")); err != nil {
		return err
	}
	spec, err := buildStagedOpenCodeSpec(r, s)
	if err != nil {
		return err
	}
	spec.env = applyOpenCodePolicy(spec.env)
	warnUntestedOpenCode()
	spec.env = applyOpenCodeMCPPolicy(spec.env, r.ProjectDir)
	spec.env = applyOpenCodeOwnDirs(spec.env, r.ProjectDir)
	spec.env, spec.cpltArgs = applyOpenCodeHooks(r, spec.env, spec.cpltArgs)
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
			s.PakkeName, domain.Bold(domain.PkgForInstall().Pick("brew install navikt/tap/cplt", "sudo apt install cplt")))
	}
	if err := checkStagedRuntime("copilot", pakkeCompatibility("copilot")); err != nil {
		return err
	}
	spec, err := buildStagedCopilotSpec(r, s)
	if err != nil {
		return err
	}
	// Both cplt launch paths must honor copilot_auth_mode, so the staged Tier 2
	// launch applies it exactly as the legacy path (LaunchCopilotResolved) does.
	spec.env, err = applyCopilotAuthMode(spec.env, r.CopilotAuthMode)
	if err != nil {
		return err
	}
	PrintCpltSandboxHint()
	PrintAutonomyNotice(r)
	PrintModelAvailabilityHint(r.Model)
	if note := autopilotNote(r); note != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), note)
	}
	return launchViaCplt(spec)
}

// openCodeV2Args rewrites a launch's opencode 1 arguments for opencode 2
// (2.0.24 `opencode --help`, `opencode run --help`), and puts what moved into
// OPENCODE_CONFIG_CONTENT:
//
//   - The TUI takes no --agent or --model: they become default_agent and model.
//   - `run` takes --agent, and --model as provider/model#variant; there is no
//     --variant.
//   - There is no --pure.
//   - --log-level is lowercase.
//   - No --standalone: cplt (navikt/cplt#716) gives each session its own
//     service, started by the client inside the sandbox, so it inherits the
//     launch's environment. A standalone client needs a loopback port cplt
//     does not open ("Transport: Was there a typo in the url or port?").
//
// Another subcommand's arguments pass through, --pure and --log-level aside.
func openCodeV2Args(args, env []string) ([]string, []string) {
	// Options end at "--": what follows is message text and passes through as is.
	var rest []string
	if i := slices.Index(args, "--"); i >= 0 {
		args, rest = args[:i], args[i:]
	}
	args = slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--pure" })
	run := len(args) > 0 && args[0] == "run"
	session := run || len(args) == 0 || !openCodeSubcommands[args[0]]
	cfg := map[string]any{}
	var out []string
	model, variant := "", ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		if i+1 < len(args) && session {
			switch {
			case a == "--variant" && run:
				variant = args[i+1]
				i++
				continue
			case a == "--model":
				model = args[i+1]
				i++
				continue
			case a == "--agent" && !run:
				cfg["default_agent"] = args[i+1]
				i++
				continue
			}
		}
		if a == "--log-level" && i+1 < len(args) {
			out = append(out, a, strings.ToLower(args[i+1]))
			i++
			continue
		}
		out = append(out, a)
	}
	if run && variant != "" && model == "" {
		fmt.Fprintf(os.Stderr, "%s --variant %s is not applied: opencode 2 takes a variant only as part of --model (provider/model#variant).\n", domain.Yellow("⚠"), variant)
	}
	if model != "" {
		if run {
			if variant != "" {
				model += "#" + variant
			}
			out = append(out[:1], append([]string{"--model", model}, out[1:]...)...)
		} else {
			cfg["model"] = model
		}
	}
	out = append(out, rest...)
	if len(cfg) > 0 {
		env = withOpenCodeConfigContent(env, cfg)
	}
	return out, env
}

// withOpenCode2UserConfig keeps the user's own opencode config in a Tier 2
// launch on opencode 2. opencode 1 reads OPENCODE_CONFIG_DIR beside the user's
// config dir; opencode 2 reads it in place of that dir
// (packages/cli/src/server-process.ts at v2.0.24), so the payload would be
// the only global config. Put back:
//
//   - the user's opencode.jsonc, else opencode.json, as OPENCODE_CONFIG,
//     unless the user set that themselves;
//   - the user's skills directories, as "skills" in OPENCODE_CONFIG_CONTENT.
//
// opencode 2 ranks OPENCODE_CONFIG above the config dir, which would let the
// user's file override the payload's opencode.json; opencode 1 ranks the
// payload higher. So the payload's permissions and agents go into
// OPENCODE_CONFIG_CONTENT as well, which ranks above both: opencode 2 lays a
// later source's agent fields over an earlier one's, so a user agent with a
// payload agent's name would otherwise replace its prompt and model. Agents
// come from the payload's opencode.json, with relative {file:} paths made
// absolute (the content resolves them from the project), and from its
// agent/mode markdown files. Nothing else: other keys may name paths relative
// to the payload.
//
// Not put back: the user's agents, commands, modes, tools and plugins
// directories under the config dir. opencode 2 reads those only from a config
// directory, and it takes one global directory, the payload.
func withOpenCode2UserConfig(env []string, payload string) []string {
	dir := openCodeConfigDir()
	if !slices.ContainsFunc(env, func(e string) bool { return strings.HasPrefix(e, "OPENCODE_CONFIG=") }) {
		for _, n := range []string{"opencode.jsonc", "opencode.json"} {
			if f := filepath.Join(dir, n); fileExists(f) {
				env, _ = telemetry.SetEnvValue(env, "OPENCODE_CONFIG", f)
				break
			}
		}
	}
	add := map[string]any{}
	var skills []any
	for _, n := range []string{"skill", "skills"} {
		if d := filepath.Join(dir, n); dirHasEntries(d) {
			skills = append(skills, d)
		}
	}
	if len(skills) > 0 {
		add["skills"] = skills
	}
	for _, n := range []string{"opencode.json", "opencode.jsonc"} {
		b, err := os.ReadFile(filepath.Join(payload, n))
		if err != nil {
			continue
		}
		b = openCodeFileRefs.ReplaceAllFunc(b, func(m []byte) []byte {
			p := string(m[len("{file:") : len(m)-1])
			if filepath.IsAbs(p) || strings.HasPrefix(p, "~/") {
				return m
			}
			return []byte("{file:" + filepath.ToSlash(filepath.Join(payload, p)) + "}")
		})
		var cfg map[string]any
		if json.Unmarshal(stripJSONC(b), &cfg) == nil {
			for _, k := range []string{"permission", "permissions", "agent", "agents"} {
				if v, ok := cfg[k]; ok {
					add[k] = mergeJSON(add[k], v)
				}
			}
		}
	}
	// As opencode loads them: markdown agents after the config dir's files.
	if md := openCodeMarkdownAgents(payload); len(md) > 0 {
		add["agent"] = mergeJSON(add["agent"], md)
	}
	if len(add) == 0 {
		return env
	}
	return withOpenCodeConfigContent(env, add)
}

var openCodeFileRefs = regexp.MustCompile(`\{file:[^}]+\}`)

// openCodeMarkdownAgents reads a config dir's agent and mode files as opencode
// 2 names them (packages/core/src/config/plugin/agent.ts at v2.0.24): the
// path under the folder without .md, the body as the prompt. A file whose
// frontmatter is not the flat shape nav-pilot writes is left out, and keeps
// its rank below the user's config.
func openCodeMarkdownAgents(dir string) map[string]any {
	out := map[string]any{}
	for _, sub := range []string{"agent", "agents", "mode", "modes"} {
		root := filepath.Join(dir, sub)
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") || (strings.HasPrefix(sub, "mode") && filepath.Dir(p) != root) {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			fm, body, ok := source.SplitFrontmatter(b)
			if !ok {
				return nil
			}
			agent, ok := parseFlatYAML(strings.Split(string(fm), "\n"), 0)
			if !ok {
				return nil
			}
			if _, set := agent["mode"]; !set && strings.HasPrefix(sub, "mode") {
				agent["mode"] = "primary"
			}
			agent["prompt"] = strings.TrimSpace(string(body))
			rel, _ := filepath.Rel(root, p)
			out[strings.TrimSuffix(filepath.ToSlash(rel), ".md")] = agent
			return nil
		})
	}
	return out
}

// parseFlatYAML reads nested maps of scalars, the frontmatter shape
// BuildAgentFrontmatter and OpenCodeToolPermission write. ok is false on
// anything else, a list for one.
// ponytail: no YAML library in go.mod; a richer payload frontmatter needs one.
func parseFlatYAML(lines []string, indent int) (map[string]any, bool) {
	out := map[string]any{}
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if len(l)-len(strings.TrimLeft(l, " ")) != indent {
			return nil, false
		}
		k, v, found := strings.Cut(t, ":")
		if !found || strings.HasPrefix(t, "- ") {
			return nil, false
		}
		k = yamlScalar(strings.TrimSpace(k)).(string)
		if v = strings.TrimSpace(v); v != "" {
			out[k] = yamlScalar(v)
			continue
		}
		j := i + 1
		for j < len(lines) && (strings.TrimSpace(lines[j]) == "" || len(lines[j])-len(strings.TrimLeft(lines[j], " ")) > indent) {
			j++
		}
		if j == i+1 {
			return nil, false
		}
		first := strings.TrimLeft(lines[i+1], " ")
		child, ok := parseFlatYAML(lines[i+1:j], len(lines[i+1])-len(first))
		if !ok {
			return nil, false
		}
		out[k] = child
		i = j - 1
	}
	return out, true
}

func yamlScalar(v string) any {
	if s, err := strconv.Unquote(v); err == nil && strings.HasPrefix(v, `"`) {
		return s
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	switch v {
	case "true":
		return true
	case "false":
		return false
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return v
}
