package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// FindCopilotCLI returns the path to cplt or copilot CLI.
// Prefers cplt (unambiguous GitHub Copilot CLI).
// If the "copilot" binary is actually cplt (aliased), it's treated as cplt.
func FindCopilotCLI() (path, name string) {
	if p, err := exec.LookPath("cplt"); err == nil {
		return p, "cplt"
	}
	if p, err := exec.LookPath("copilot"); err == nil {
		if isCplt(p) {
			return p, "cplt"
		}
		return p, "copilot"
	}
	return "", ""
}

// isCplt checks if a binary is actually cplt (Copilot Sandbox) by inspecting
// its version output. Returns true if the binary identifies as cplt/sandbox.
// The spawn is bounded: FindCopilotCLI runs it on every launch path where a
// plain `copilot` binary is on PATH, and a hanging binary must not hang us.
func isCplt(binPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binPath, "--version").CombinedOutput()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	return strings.Contains(s, "cplt") || strings.Contains(s, "copilot-sandbox")
}

// CLIDisplayName returns a user-friendly name for the CLI binary.
func CLIDisplayName(name string) string {
	if name == "cplt" {
		return "Copilot Sandbox (cplt)"
	}
	return name
}

// copilotAgentArgs returns extra CLI flags for a given agent.
// Keep this empty by default so model/effort selection follows agent defaults
// (or explicit user overrides in the CLI), not hardcoded launch arguments.
func copilotAgentArgs(agent string) []string {
	_ = agent
	return nil
}

// BuildCopilotArgs constructs the CLI arguments for launching copilot.
//
// cplt is the sandbox wrapper: its own --agent selects WHICH agent to sandbox
// and otherwise auto-detects from PATH (per `cplt --help`). Because nav-pilot is
// on the copilot launch path here, we pin `cplt --agent copilot` so a different
// agent on PATH (e.g. opencode) is never picked, then forward the copilot
// persona + flags after the "--" separator.
//
// Note: the forwarded --agent is always the active agentpakke's copilot
// persona; resolved.Client selects the launcher and is consumed by
// launchClient before reaching here.
func BuildCopilotArgs(cliName string, resolved domain.ResolvedConfig) []string {
	persona := PrimaryAgent("copilot")
	var args []string
	args = append(args, "--agent", persona)
	args = append(args, copilotAgentArgs(persona)...)
	if resolved.Model != "" {
		args = append(args, "--model", resolved.Model)
	} else if model := pakkeDeclaredModel("copilot"); model != "" {
		// Same fallback the staged Tier 2 copilot path has
		// (buildStagedCopilotSpec), and the same one Tier 1 opencode gets
		// through ToOpenCodeModel. Without it copilot behaved differently by
		// tier, and Tier 1 copilot, the default configuration, was the only
		// launch path an agentpakke could not declare a model for.
		// pakkeDeclaredModel already maps agentpakke.InheritModel to "", so a
		// pakke that declares nothing still emits no --model.
		args = append(args, "--model", model)
	}
	args = append(args, copilotResolvedFlags(resolved)...)
	if cliName == "cplt" {
		cpltArgs := append([]string{"--agent", "copilot", "--"}, args...)
		return append(cpltArgs, resolved.ExtraArgs...)
	}
	return append(args, resolved.ExtraArgs...)
}

// copilotResolvedFlags returns the copilot CLI flags that follow the persona
// and model: the resolved-config tail shared by the legacy launch
// ([BuildCopilotArgs]) and the staged Tier 2 launch
// (buildStagedCopilotSpec). Order and emit conditions are pinned by
// golden_launch_test.go — do not reorder.
func copilotResolvedFlags(resolved domain.ResolvedConfig) []string {
	var args []string
	if resolved.Mode != "" && resolved.Mode != "default" {
		args = append(args, "--mode", resolved.Mode)
	}
	if resolved.ReasoningEffort != "" {
		args = append(args, "--effort", resolved.ReasoningEffort)
	}
	if resolved.ContextTier != "" && resolved.ContextTier != "default" {
		args = append(args, "--context", resolved.ContextTier)
	}
	if resolved.AllowAllTools {
		args = append(args, "--allow-all-tools")
	}
	if !resolved.AskUser {
		args = append(args, "--no-ask-user")
	}
	if resolved.LogLevel != "" {
		args = append(args, "--log-level", resolved.LogLevel)
	}
	return args
}

// LaunchCopilotResolved launches the Copilot CLI with the resolved launch config.
// If user-scope instructions exist, it sets COPILOT_CUSTOM_INSTRUCTIONS_DIRS
// so cplt picks up ~/.copilot/.github/instructions/*.instructions.md.
//
// When launched via cplt, CopilotAuthMode constrains where cplt may get the
// Copilot token from: env_only aborts the launch unless one is already in the
// environment, gh_only removes the token variables so cplt must use
// `gh auth token`. See applyCopilotAuthMode.
func LaunchCopilotResolved(resolved domain.ResolvedConfig) error {
	// Local inference. The one branch on this path, and it used to be a
	// refusal, on the belief that the Copilot CLI only ever resolves models
	// through GitHub. It does not: `copilot help providers` documents BYOK,
	// where COPILOT_PROVIDER_BASE_URL replaces GitHub's model routing outright.
	// [copilotLocalWorker] is where that is set up, or refused for the reasons
	// that are still true.
	//
	// Nil guard for everyone who has not opted in, and for every hosted session
	// of everyone who has, so no existing launch changes.
	worker, guard, err := copilotLocalWorker(resolved.Model)
	if err != nil {
		return err
	}
	defer guard.Close()
	if guard != nil {
		// The Copilot CLI runs the whole session locally, so this counts prompts
		// rather than delegations. Same instrument, different meaning by client,
		// which the client attribute keeps separable.
		defer func() {
			telemetryRecorder.RecordLocalSession("copilot", worker.Model, guard.Completions(), guard.SawTraffic())
		}()
	}

	cliPath, cliName := FindCopilotCLI()
	if cliPath == "" {
		telemetryRecorder.RecordLaunchError("copilot", "client_not_found")
		return fmt.Errorf("copilot cli not found")
	}
	if cliName == "cplt" {
		PrintCpltSandboxHint()
	}
	env := CopilotEnv(resolved.OtelLogLevel)
	if guard == nil {
		PrintModelAvailabilityHint(resolved.Model)
	} else {
		// Not PrintModelAvailabilityHint: it reads the publisher prefix as
		// nav-pilot's provider-qualified spelling and tells the developer to
		// drop it, which for a Hugging Face model id is the wrong advice.
		env = copilotLocalEnv(env, worker, guard.URL())
		fmt.Fprintf(os.Stderr, "%s Local inference: this whole session runs on %s here on the machine.\n",
			domain.Dim("ℹ"), domain.Bold(worker.Model))
		fmt.Fprintf(os.Stderr, "%s nav-pilot ends a turn after %d identical tool calls in a row.\n\n",
			domain.Dim("ℹ"), local.LoopGuardRepeat())
	}
	args := copilotLaunchArgs(cliName, resolved, IsTerminal(os.Stdin))
	displayName := CLIDisplayName(cliName)
	fmt.Printf("Launching %s with agent %s...\n\n", domain.Bold(displayName), domain.Bold(PrimaryAgent("copilot")))

	// cplt resolves the Copilot token itself; copilot_auth_mode only constrains
	// which source it may use, and can refuse the launch outright.
	if cliName == "cplt" {
		var err error
		env, err = applyCopilotAuthMode(env, resolved.CopilotAuthMode)
		if err != nil {
			return err
		}
	}

	cmd := exec.Command(cliPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "%s Could not launch %s: %v\n", domain.Yellow("⚠"), displayName, err)
		}
		if kind := classifyLaunchError(err); kind != "" {
			telemetryRecorder.RecordLaunchError("copilot", kind)
		}
		return err
	}
	return nil
}

// copilotLocalWorker prepares a Copilot CLI session to run against the local
// server, and is where the old refusal moved to for the cases it is still true
// of.
//
// The mechanism is BYOK, the client's own documented one (`copilot help
// providers`): COPILOT_PROVIDER_BASE_URL replaces GitHub's model routing for
// the whole session, and GitHub authentication is not required while it is set.
// There is no subcommand and no config file behind it — `providers` is a help
// topic, not a command — so the environment is not the shortcut here, it is the
// interface.
//
// That one provider is also the reason this gates on the *session* model and
// not on [local.Enabled], which is where it differs from the opencode path.
// opencode selects a backend per agent, so a cloud main agent can hand tasks to
// a local worker and both run at once; the Copilot CLI points every request in
// the session at one endpoint. Setting these variables for a developer who
// asked for a hosted model would not add a local worker to their session, it
// would move their session onto the local model without saying so.
//
// The returned guard is nil whenever there is nothing local about this launch,
// and [local.Guard.Close] is nil-safe, so the caller defers it either way.
func copilotLocalWorker(sessionModel string) (local.Model, *local.Guard, error) {
	if local.DisabledLocalModel(sessionModel) {
		return local.Model{}, nil, fmt.Errorf(
			"%s runs on this machine, but local inference is off for this install, so there is nothing to point the client at.\n\n  Turn it on:\n\n    %s",
			domain.Bold(sessionModel), domain.Bold("nav-pilot alpha local init"))
	}
	if !local.IsLocal(sessionModel) {
		return local.Model{}, nil, nil
	}
	// Same proof the opencode path takes, and for the same reason: the guard
	// forwards to a fixed 127.0.0.1 port, so a server that died and left that
	// port to whatever bound it next would have the session proxied to a
	// stranger with nothing on screen to say so.
	worker, err := localWorker()
	if err != nil {
		return local.Model{}, nil, err
	}
	if worker.Model != sessionModel {
		// Reachable the moment the manifest names more than one model: `alpha
		// local start` records what it loaded, and the session model is a
		// separate config value the developer can change afterwards. The
		// server answers on the model it has loaded whatever the request
		// names, so without this the session would run on a model nobody
		// chose and nothing would say so.
		return local.Model{}, nil, fmt.Errorf(
			"the local server on this machine is serving %s, and this session is configured for %s.\n\n  Use what is running:\n\n    %s\n\n  Or load the other model:\n\n    %s\n    %s\n    %s",
			domain.Bold(worker.Model), domain.Bold(sessionModel),
			domain.Bold("nav-pilot config set model "+worker.Model),
			domain.Bold("nav-pilot config set local_model "+sessionModel),
			domain.Bold("nav-pilot alpha local stop"),
			domain.Bold("nav-pilot alpha local start"))
	}
	guard, err := local.StartGuard(local.ServerURL())
	if err != nil {
		return local.Model{}, nil, err
	}
	return worker, guard, nil
}

// copilotLocalEnv points the Copilot CLI at the loop guard in front of the
// local server. Pure, so the whole delta a local launch adds to the developer's
// environment is one readable list rather than something to reconstruct from a
// launch.
//
// The base URL is the guard's and never the server's, for the reason the
// opencode provider block states: every completion has to pass through the
// thing that can stop a runaway loop. The guard reads chat-completions
// requests, which is what the client's default wire API sends — pinned here
// rather than left to the default, because an exported
// COPILOT_PROVIDER_WIRE_API=responses would send a shape the guard cannot read
// and would silently forward.
//
// The token limits come from the manifest, under the keys the generator already
// publishes them as. They are named MLX_OPENCODE_* and read by both clients:
// they describe the context the model was measured at, which does not change
// with the client asking. Without them the client falls back to defaults for a
// model id its catalogue has never heard of.
//
// SetEnvValue rather than SetEnvIfAbsent: these decide where the prompt goes,
// and an exported COPILOT_PROVIDER_BASE_URL left over from something else must
// not quietly win.
func copilotLocalEnv(env []string, m local.Model, guardURL string) []string {
	for _, kv := range [][2]string{
		{"COPILOT_PROVIDER_BASE_URL", guardURL + "/v1"},
		{"COPILOT_PROVIDER_TYPE", "openai"},
		{"COPILOT_PROVIDER_WIRE_API", "completions"},
		// Optional for a local provider, per `copilot help providers`. Sent
		// anyway so the value in the logs is nav-pilot's name and not a
		// developer's real key picked up from the environment.
		{"COPILOT_PROVIDER_API_KEY", "nav-pilot"},
		{"COPILOT_MODEL", m.Model},
		{"COPILOT_PROVIDER_MAX_PROMPT_TOKENS", strconv.Itoa(localParamInt(m, "MLX_OPENCODE_CONTEXT", 32768))},
		{"COPILOT_PROVIDER_MAX_OUTPUT_TOKENS", strconv.Itoa(localParamInt(m, "MLX_OPENCODE_OUTPUT", 8192))},
	} {
		env, _ = telemetrypkg.SetEnvValue(env, kv[0], kv[1])
	}
	return env
}

// copilotLaunchArgs is the vector LaunchCopilot passes to the binary it
// resolved: [BuildCopilotArgs], plus cplt's --yes when no terminal can answer
// the confirmation cplt asks before it starts anything. This path builds its
// own vector rather than going through cpltArgv, so it needs the same treatment
// separately or it keeps dying on the prompt that the opencode and pi paths no
// longer die on.
//
// The plain copilot CLI never gets it: --yes is cplt's flag and copilot has no
// confirmation to skip.
func copilotLaunchArgs(cliName string, resolved domain.ResolvedConfig, tty bool) []string {
	args := BuildCopilotArgs(cliName, resolved)
	if cliName != "cplt" {
		return args
	}
	return withCpltConfirmation(args, tty)
}

// ghTokenVars are the environment variables cplt recognises as carrying a
// GitHub token, in the precedence order it applies.
var ghTokenVars = []string{"GH_TOKEN", "GITHUB_TOKEN", "COPILOT_GITHUB_TOKEN"}

// applyCopilotAuthMode enforces copilot_auth_mode on the environment handed to
// cplt.
//
// nav-pilot does not extract or inject a token. With cplt's gh guard on — which
// since cplt#335 is every preset except permissive and full-trust, `standard`
// included, so it is the case by default rather than only under the `strict`
// nav-pilot recommends — cplt uses an inherited
// GH_TOKEN/GITHUB_TOKEN/COPILOT_GITHUB_TOKEN when one is present and
// otherwise runs `gh auth token --hostname github.com` in the unsandboxed
// parent, handing the result to the agent over a one-time 0600 file rather than
// the environment. With the gh guard off, cplt does none of that and Copilot
// authenticates on its own. Either way this function only decides which sources
// reach cplt, and refuses to launch when the mode asks for one that is absent.
//
//   - auto:     no constraint (default).
//   - env_only: a token must already be in the environment; the launch aborts
//     if not. Enforced here, so it holds whatever cplt is configured to do.
//   - gh_only:  the token variables are removed from the child environment, so
//     no env token reaches the sandbox. What cplt does then is its own call.
//
// An empty authMode is treated as "auto": every real launch resolves the mode
// through resolve() (default "auto"), but a directly constructed ResolvedConfig
// must not hard-fail the launch.
func applyCopilotAuthMode(env []string, authMode string) ([]string, error) {
	switch authMode {
	case "", "auto":
		return env, nil

	case "env_only":
		if !hasEnvToken(env) {
			return env, fmt.Errorf(
				"copilot_auth_mode=env_only: none of %s is set; launch aborted",
				strings.Join(ghTokenVars, "/"))
		}
		return env, nil

	case "gh_only":
		return stripEnvTokens(env), nil

	default:
		return env, fmt.Errorf("unknown copilot_auth_mode %q (allowed: %s)",
			authMode, strings.Join(domain.ValidCopilotAuthModes, ", "))
	}
}

// envNamesCaseInsensitive mirrors the OS rule for environment variable names:
// Windows treats them case-insensitively, every other platform exactly. Kept as
// a var so tests can exercise both matchers. Without it a lower-case gh_token
// would slip past gh_only's stripping on Windows, and fail env_only that the OS
// would have satisfied.
var envNamesCaseInsensitive = runtime.GOOS == "windows"

// isGHTokenEntry reports whether an "NAME=value" entry names one of
// ghTokenVars, and returns its raw value.
func isGHTokenEntry(entry string) (value string, ok bool) {
	name, value, found := strings.Cut(entry, "=")
	if !found {
		return "", false
	}
	if envNamesCaseInsensitive {
		return value, slices.ContainsFunc(ghTokenVars, func(k string) bool {
			return strings.EqualFold(name, k)
		})
	}
	return value, slices.Contains(ghTokenVars, name)
}

// hasEnvToken reports whether env carries a non-blank GitHub token variable.
// Blank values do not count: cplt trims before deciding a token is present, so
// an empty GH_TOKEN would send it to `gh auth token` anyway.
func hasEnvToken(env []string) bool {
	for _, e := range env {
		if value, ok := isGHTokenEntry(e); ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// stripEnvTokens removes every GitHub token variable from env, blank ones
// included, so nothing is left for cplt to inherit.
func stripEnvTokens(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if _, ok := isGHTokenEntry(e); ok {
			continue
		}
		out = append(out, e)
	}
	return out
}

// cpltSandboxHintShown tracks whether the cplt sandbox hint has been shown this session.
var cpltSandboxHintShown bool

// IsTerminal reports whether f is a terminal. Used to suppress informational
// hints in non-interactive contexts, to decide whether anything can answer
// cplt's launch confirmation (see withCpltConfirmation), and by `alpha local
// init` to decide whether anything can answer its download confirmation.
//
// It asks the kernel rather than reading the file mode, because
// os.ModeCharDevice — what isInteractive in internal/cli checks — is also set
// for /dev/null, and /dev/null is exactly what stdin is on a dispatched,
// non-interactive run. The cheap check answers "a human is there" for the one
// case that most needs the answer to be no.
func IsTerminal(f *os.File) bool {
	_, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	return err == nil
}

// PrintCpltSandboxHint prints a one-time tip about cplt sandbox configuration
// for users who may not know how to configure cplt outside of nav-pilot.
// Suppressed by NAV_PILOT_CPLT_HINT=0 or in non-interactive mode.
func PrintCpltSandboxHint() {
	if cpltSandboxHintShown || !IsTerminal(os.Stdin) {
		return
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("NAV_PILOT_CPLT_HINT")), "0") {
		return
	}
	cpltSandboxHintShown = true
	fmt.Printf("%s Launching via cplt (Copilot Sandbox). Sandbox settings are managed by cplt, not nav-pilot.\n", domain.Dim("ℹ"))
	fmt.Printf("  View current settings: %s\n", domain.Bold("cplt config list"))
	fmt.Printf("  Change a setting:      %s\n", domain.Bold("cplt config set <key> <value>"))
	fmt.Printf("  Suppress this hint:    set %s in your shell\n\n", domain.Bold("NAV_PILOT_CPLT_HINT=0"))
}

// PrintModelAvailabilityHint shows a note when a specific model is configured.
// Warns on provider-qualified format (e.g. github-copilot/claude-sonnet-4.5)
// and reminds users about org-level availability restrictions.
func PrintModelAvailabilityHint(model string) {
	if !IsTerminal(os.Stdin) {
		return
	}
	if model == "" || model == "auto" {
		return
	}
	if strings.Contains(model, "/") {
		shortID := strings.SplitN(model, "/", 2)[1]
		if shortID == "" {
			shortID = model
		}
		fmt.Printf("%s Model %s is in provider-qualified format. nav-pilot translates it, but the canonical form is preferred: %s\n\n",
			domain.Yellow("⚠"), domain.Bold(model), domain.Bold("nav-pilot config set model "+shortID))
		return
	}
	fmt.Printf("%s Model: %s — if unavailable in your org, run: %s\n\n",
		domain.Dim("ℹ"), domain.Bold(model), domain.Bold("nav-pilot config set model auto"))
}

// CopilotEnv returns the environment for launching cplt, injecting
// COPILOT_CUSTOM_INSTRUCTIONS_DIRS if user-scope customizations exist
// (instructions and/or agents), and OTEL_LOG_LEVEL if otelLogLevel is set.
func CopilotEnv(otelLogLevel string) []string {
	return copilotEnv(otelLogLevel, true)
}

// copilotEnv is CopilotEnv with the user-instructions injection made optional.
// injectUserInstructions is false for a staged Tier 2 launch, which must not
// mix the user's own ~/.copilot content into a third-party pakke's session; an
// exported COPILOT_CUSTOM_INSTRUCTIONS_DIRS is still inherited untouched from
// os.Environ(), as the reference launcher does.
func copilotEnv(otelLogLevel string, injectUserInstructions bool) []string {
	copilotDir := userCopilotDir()
	if !injectUserInstructions {
		copilotDir = ""
	}
	env := os.Environ()
	key := "COPILOT_CUSTOM_INSTRUCTIONS_DIRS"
	if copilotDir != "" {
		existing := telemetrypkg.LookupEnvValue(env, key)
		if existing != "" {
			alreadyPresent := false
			for _, p := range strings.Split(existing, ",") {
				if strings.TrimSpace(p) == copilotDir {
					alreadyPresent = true
					break
				}
			}
			if !alreadyPresent {
				copilotDir = existing + "," + copilotDir
			} else {
				copilotDir = existing
			}
		}

		env, _ = telemetrypkg.SetEnvValue(env, key, copilotDir)
	}

	env, _ = telemetrypkg.ApplyCopilotOTelEnv(env, cliVersion)

	if strings.TrimSpace(otelLogLevel) != "" {
		env, _ = telemetrypkg.SetEnvIfAbsent(env, "OTEL_LOG_LEVEL", strings.TrimSpace(otelLogLevel))
	}

	return env
}

// userCopilotDir returns ~/.copilot if it contains user-scope customizations
// (instructions or agents), or "" otherwise.
func userCopilotDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	base := filepath.Join(home, ".copilot")

	instructions, _ := filepath.Glob(filepath.Join(base, ".github", "instructions", "*.instructions.md"))
	if len(instructions) > 0 {
		return base
	}

	agents, _ := filepath.Glob(filepath.Join(base, "agents", "*.agent.md"))
	if len(agents) > 0 {
		return base
	}

	return ""
}

// printCopilotDiagnostics prints the cplt system configuration for nav-pilot status.
func printCopilotDiagnostics() {
	cliPath, cliName := FindCopilotCLI()
	if cliPath == "" || cliName != "cplt" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var sb strings.Builder
	versionOut, err := exec.CommandContext(ctx, cliPath, "--version").Output()
	version := strings.TrimSpace(string(versionOut))
	if err != nil {
		version = "unknown"
	}
	sb.WriteString(fmt.Sprintf("\n%s cplt found at %s (%s)\n", domain.Green("✓"), cliPath, version))

	sb.WriteString("  agent pinned    : copilot → nav-pilot\n")
	sb.WriteString("  cplt config     : run 'cplt config show' to view sandbox settings\n")

	fmt.Print(sb.String())
}
