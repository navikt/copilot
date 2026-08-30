package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// ConfigPathOverride can be set in tests to redirect the opencode config.
var ConfigPathOverride string

// NavContextDirOverride can be set in tests to redirect Nav context materialization.
var NavContextDirOverride string

// openCodeConfigPath returns the path to opencode's global config.
// Honors ConfigPathOverride (test seam).
// Falls back to os.TempDir() when the home directory cannot be resolved so the
// returned path is always absolute.
func openCodeConfigPath() string {
	if ConfigPathOverride != "" {
		return ConfigPathOverride
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "nav-pilot", ".config", "opencode", "opencode.json")
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// openCodeNavContextDir returns the directory for Nav context materialization.
// Always uses the user-global opencode config dir (~/.config/opencode/) so Nav
// context is available across all repos regardless of whether the developer
// is inside a git repo or has run `nav-pilot export opencode` manually before.
// Honors NavContextDirOverride (test seam).
// Falls back to os.TempDir() when the home directory cannot be resolved so the
// returned path is always absolute.
func openCodeNavContextDir() string {
	if NavContextDirOverride != "" {
		return NavContextDirOverride
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "nav-pilot", ".config", "opencode")
	}
	return filepath.Join(home, ".config", "opencode")
}

// EnsureOpenCodeNavContext resolves the Nav artifact source and materializes
// AGENTS.md, skills, agents, and commands into opencode's user config directory.
// Uses SyncOpenCodeArtifacts for conflict detection and state tracking.
// Returns a short summary string (e.g. "AGENTS.md + 3 skill(s)") suitable for
// the launch message, or an empty string if nothing was produced.
// Non-fatal: callers should warn and continue on error.
func EnsureOpenCodeNavContext() (string, error) {
	outputDir := openCodeNavContextDir()
	prevState, _ := artifacts.ReadOpenCodeState(outputDir)

	sRepo := ""
	if prevState != nil && prevState.SourceRepo != "" {
		sRepo = prevState.SourceRepo
	}

	src, err := source.ResolveSource("", sRepo, cliVersion)
	if err != nil {
		return "", fmt.Errorf("resolving source: %w", err)
	}
	defer src.Cleanup()

	if prevState != nil {
		assessment := assessStaleness(prevState.Version)
		recordFreshness("opencode", artifacts.OpenCodeScopeName, assessment)
	}

	skills, commands, agents, instrCount, conflicts, err := artifacts.SyncOpenCodeArtifacts(src.Dir, outputDir, src.Version, src.SHA, src.Repo)
	if err != nil {
		return "", err
	}

	for _, c := range conflicts {
		fmt.Fprintf(os.Stderr, "%s Nav context file modified locally, not overwriting: %s\n", domain.Yellow("⚠"), c)
	}

	summary := artifacts.ExportSummary(skills, commands, agents, instrCount)
	if summary == "nothing to export" {
		return "", nil
	}
	return summary, nil
}

// OpenCodeArgs builds the CLI arguments for launching opencode non-interactively.
// Maps resolved config fields to opencode flags; omits unset/default fields.
func OpenCodeArgs(resolved domain.ResolvedConfig) []string {
	var args []string
	// The model nav-pilot sets for the session. The flag outranks opencode's own
	// config and its recent-model list, and on `opencode run` it outranks an
	// agent's frontmatter too, because there it is the request model. In the TUI,
	// which is what nav-pilot launches, an agent that declares its own `model:`
	// uses that instead (verified against opencode 1.18.25). So the order is
	// agent specialisation, then nav-pilot's session model, then whatever the
	// client would have picked on its own.
	args = append(args, "--model", ToOpenCodeModel(resolved.Model))
	if resolved.Mode == "plan" {
		// opencode's built-in read-only planning agent. Nav context still loads
		// via AGENTS.md regardless of the active agent.
		args = append(args, "--agent", "plan")
	} else {
		// Launch the materialized Nav primary agent so the session starts with
		// Nav's persona and context (parity with the copilot client persona).
		args = append(args, "--agent", PrimaryAgent("opencode"))
	}
	if resolved.ReasoningEffort != "" {
		args = append(args, "--variant", resolved.ReasoningEffort)
	}
	if resolved.AllowAllTools {
		args = append(args, "--dangerously-skip-permissions")
	}
	if lvl := openCodeLogLevel(resolved.LogLevel); lvl != "" {
		args = append(args, "--log-level", lvl)
	}
	return args
}

// openCodeAgentArgs is what a legacy opencode launch passes the client:
// [OpenCodeArgs], with the user's pass-through arguments forwarded through the
// same rules the staged path uses (openCodeClientArgs), so `run` keeps its
// place as the first argument opencode sees.
//
// Until this existed the pass-through arguments were parsed, resolved, and then
// dropped on the floor: `nav-pilot -- run "…"` started the TUI with the request
// discarded, which is a whole non-interactive dispatch thrown away in silence.
// With none of them openCodeClientArgs returns the bind untouched, so every
// launch that has ever worked is byte-identical (golden_launch_test.go).
func openCodeAgentArgs(resolved domain.ResolvedConfig) []string {
	return openCodeClientArgs(OpenCodeArgs(resolved), resolved.ExtraArgs)
}

// OpenCodeUnsupportedConfigWarnings returns informational warning strings for
// config fields that are explicitly set to a non-default value but have no
// opencode equivalent.
func OpenCodeUnsupportedConfigWarnings(r domain.ResolvedConfig) []string {
	var w []string
	if r.Mode == "autopilot" {
		w = append(w, `mode "autopilot" has no opencode equivalent — running with opencode defaults (use allow_all_tools = true to skip confirmations)`)
	}
	if r.ContextTier != "" {
		w = append(w, fmt.Sprintf("context_tier %q has no opencode equivalent — ignored", r.ContextTier))
	}
	if !r.AskUser {
		w = append(w, "ask_user = false has no opencode equivalent — ignored")
	}
	return w
}

// openCodeLogLevel translates a nav-pilot log level to opencode's accepted set
// (DEBUG, INFO, WARN, ERROR).
func openCodeLogLevel(level string) string {
	switch level {
	case "debug", "all":
		return "DEBUG"
	case "info":
		return "INFO"
	case "warning":
		return "WARN"
	case "error":
		return "ERROR"
	default:
		return ""
	}
}

// EnsureOpenCodeOTelConfig reads ~/.config/opencode/opencode.json (or creates it),
// sets experimental.openTelemetry=true without clobbering other keys, and writes back.
func EnsureOpenCodeOTelConfig() error {
	path := openCodeConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating opencode config dir: %w", err)
	}

	var cfg map[string]any

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading opencode config: %w", err)
		}
		cfg = map[string]any{
			"$schema":    "https://opencode.ai/config.json",
			"autoupdate": "notify",
			"share":      "disabled",
			"logLevel":   "INFO",
		}
	} else {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("opencode config is not valid JSON (%s): %w", path, err)
		}
		// A file holding the literal `null` parses without error and leaves cfg
		// nil, and assigning into a nil map panics. Erroring for the same reason
		// unparseable content does: the file is the developer's, and replacing it
		// with a fresh object loses whatever they meant by it.
		if cfg == nil {
			return fmt.Errorf("opencode config is not a JSON object (%s): remove or fix the file", path)
		}
	}

	experimental, _ := cfg["experimental"].(map[string]any)
	if experimental == nil {
		experimental = make(map[string]any)
	}
	if v, ok := experimental["openTelemetry"]; ok && v == true {
		return nil
	}
	experimental["openTelemetry"] = true
	cfg["experimental"] = experimental

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling opencode config: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("writing opencode config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("setting opencode config permissions: %w", err)
	}
	return nil
}

// LocalProviderID is the opencode provider id the local server is registered
// under. Shared with the generator in navikt/mlx-workspace, whose opencode-init
// task writes the same block for its benchmark workspaces.
const LocalProviderID = "mlx"

// EnsureOpenCodeLocalProvider registers the local server as an opencode
// provider, so `--model mlx/<id>` reaches this machine instead of GitHub.
//
// It is a config write rather than an environment variable because opencode
// picks its backend from the provider block and has no base-URL variable to
// override. It merges, like EnsureOpenCodeOTelConfig above and for the same
// reason: the file is the developer's, not nav-pilot's.
//
// The three limits are the manifest's, not this package's. They were measured:
// a context declared lower than the model's real one keeps each auto-compaction
// small, and the chunk timeout is the gap opencode tolerates between streamed
// tokens — at 96k tokens we measured a single token taking three and a half
// minutes, and the default timeout drops the connection mid-generation and
// returns an empty response the client does not report as a failure.
func EnsureOpenCodeLocalProvider(m local.Model) error {
	return mutateOpenCodeConfig(func(cfg map[string]any) bool {
		providers, _ := cfg["provider"].(map[string]any)
		if providers == nil {
			providers = make(map[string]any)
		}
		providers[LocalProviderID] = map[string]any{
			"npm":  "@ai-sdk/openai-compatible",
			"name": "Local (nav-pilot)",
			"options": map[string]any{
				// The loop guard, not the server: every completion has to pass
				// through the thing that can stop a runaway loop.
				"baseURL":      local.GuardURL() + "/v1",
				"apiKey":       "nav-pilot",
				"chunkTimeout": chunkTimeoutMS(m),
				"timeout":      false,
			},
			"models": map[string]any{
				m.Model: map[string]any{
					"limit": map[string]any{
						"context": localParamInt(m, "MLX_OPENCODE_CONTEXT", 32768),
						"output":  localParamInt(m, "MLX_OPENCODE_OUTPUT", 8192),
					},
				},
			},
		}
		cfg["provider"] = providers
		bindLocalWorker(cfg, m)
		return true
	})
}

// bindLocalWorker pins the worker subagent to the local model, and is the
// difference between the alpha saving AI credits and spending more of
// them. An opencode agent with no model of its own runs on the session's
// model: with a cloud main agent, every task dispatched to `lokal-arbeider`
// would have gone to the cloud, at cloud prices, while the dispatch policy
// beside it told the main agent those tasks were free.
//
// The materialized agent file cannot carry it — transformAgent reduces every
// agent's frontmatter to description and mode — and it should not: the model
// id is the developer's resolved manifest entry, not a property of the agent
// text, and the same file is synced to machines with no local model at all.
//
// `agent.<name>.model` is opencode's own per-agent selection (config.json
// $defs.AgentConfig.model, "provider/model"), verified on opencode 1.18.23:
// `opencode debug agent lokal-arbeider` resolves this block to
// {"providerID": "mlx", "modelID": "<id>"}.
//
// Merging rather than replacing, for the same reason every other writer here
// merges: a developer who set their own tools or permission on this agent
// keeps them, and only the model is nav-pilot's.
func bindLocalWorker(cfg map[string]any, m local.Model) bool {
	agents, _ := cfg["agent"].(map[string]any)
	if agents == nil {
		agents = make(map[string]any)
	}
	worker, _ := agents[local.WorkerAgent].(map[string]any)
	if worker == nil {
		worker = make(map[string]any)
	}
	want := LocalProviderID + "/" + m.Model
	if worker["model"] == want {
		return false
	}
	worker["model"] = want
	agents[local.WorkerAgent] = worker
	cfg["agent"] = agents
	return true
}

// unbindLocalWorker takes the binding back out, leaving anything else the
// developer put on that agent — and the "agent" key itself when nav-pilot is
// what emptied it.
func unbindLocalWorker(cfg map[string]any) bool {
	agents, _ := cfg["agent"].(map[string]any)
	worker, found := agents[local.WorkerAgent].(map[string]any)
	if !found {
		return false
	}
	if _, hadModel := worker["model"]; !hadModel {
		return false
	}
	delete(worker, "model")
	if len(worker) == 0 {
		delete(agents, local.WorkerAgent)
	}
	if len(agents) == 0 {
		delete(cfg, "agent")
	} else {
		cfg["agent"] = agents
	}
	return true
}

// RemoveOpenCodeLocalProvider takes the local provider block back out of
// opencode's config, which is what `alpha local off` owes the developer.
//
// Turning dispatch off in nav-pilot's own config only stops nav-pilot from
// choosing the model. The block [EnsureOpenCodeLocalProvider] wrote stays in a
// file opencode reads on its own, so a developer running opencode directly
// could still pick the model and reach the guard's port — which after `off` is
// whatever happens to be listening there. `start` writes the block back.
//
// A config with no local block, and a config that is not there at all, are both
// nothing to do rather than errors: off must work on a machine where opencode
// was never configured.
func RemoveOpenCodeLocalProvider() error {
	return mutateOpenCodeConfig(func(cfg map[string]any) bool {
		// The worker's binding goes with it: a subagent pinned to a provider
		// that is no longer registered is a session that fails on dispatch
		// rather than one that falls back.
		unbound := unbindLocalWorker(cfg)
		providers, _ := cfg["provider"].(map[string]any)
		if _, found := providers[LocalProviderID]; !found {
			return unbound
		}
		delete(providers, LocalProviderID)
		// An empty "provider": {} left behind is nav-pilot's litter in someone
		// else's file, so it goes too — but only when nav-pilot emptied it.
		if len(providers) == 0 {
			delete(cfg, "provider")
		} else {
			cfg["provider"] = providers
		}
		return true
	})
}

// mutateOpenCodeConfig reads opencode's config, hands the parsed object to
// mutate, and writes it back only when mutate reports it changed something.
//
// Every writer here needs the same merge, and for the same reason
// [EnsureOpenCodeLocalProvider] states: the file is the developer's and
// nav-pilot owns a few keys in it, not the file. A config that is not there is
// one to create rather than an error, because `start` has to work on a machine
// where opencode was never configured; a config that is there and unparseable
// is an error, because guessing at it would overwrite whatever the developer
// actually wrote.
func mutateOpenCodeConfig(mutate func(cfg map[string]any) bool) error {
	path := openCodeConfigPath()
	cfg := map[string]any{"$schema": "https://opencode.ai/config.json"}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("opencode config is not valid JSON (%s): %w", path, err)
		}
	case !os.IsNotExist(err):
		return fmt.Errorf("reading opencode config: %w", err)
	}
	if !mutate(cfg) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating opencode config dir: %w", err)
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling opencode config: %w", err)
	}
	return writeConfigAtomically(path, append(out, '\n'))
}

// writeConfigAtomically replaces the developer's config in one step: a
// temporary file in the same directory, then a rename.
//
// A plain write truncates first, so a crash, a full disk or a killed terminal
// between truncate and write leaves a half-written opencode.json — a file
// nav-pilot owns a few keys in and the developer owns the rest of, which
// opencode then refuses to parse and nav-pilot's own merge refuses to touch.
// Same directory because a rename is only atomic within a filesystem.
func writeConfigAtomically(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".opencode.json.*")
	if err != nil {
		return fmt.Errorf("creating temporary opencode config: %w", err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("setting opencode config permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing opencode config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing opencode config: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replacing opencode config: %w", err)
	}
	return nil
}

// chunkTimeoutMS is the gap opencode tolerates between streamed tokens, in
// milliseconds. One function because two places state it: the provider block
// configures it and [LocalDispatchPolicy] tells the main agent how long to
// wait, and a dispatcher that gives up first can duplicate an edit that is
// still in flight.
func chunkTimeoutMS(m local.Model) int {
	return localParamInt(m, "MLX_OPENCODE_CHUNK_TIMEOUT", 600000)
}

// localParamInt reads one MLX_ param as an integer, falling back to a
// conservative default. A malformed value is not fatal: the manifest's job is
// to tune this, and a bad number should cost tuning, not the session.
func localParamInt(m local.Model, key string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(m.Params[key])); err == nil && n > 0 {
		return n
	}
	return fallback
}

// localPolicyFileName is the dispatch policy's name in opencode's config
// directory. It is nav-pilot's file and nobody else's: nothing merges into it,
// `alpha local off` deletes it, and it sits at the root of that directory
// rather than under agents/ or instructions/ so the Nav context sync never
// counts it as one of its own and never deletes it out from under a session.
const localPolicyFileName = "nav-pilot-lokal-dispatch.md"

func localPolicyPath() string {
	return filepath.Join(filepath.Dir(openCodeConfigPath()), localPolicyFileName)
}

// LocalDispatchPolicy is what the main agent is told about the local worker:
// that it exists, what it is good at, and how it fails.
//
// Generated rather than shipped as a file, because everything load-bearing in
// it moves. The model id and the role and expect prose come from the resolved
// manifest entry, which is a file in another repo; the threshold is the
// developer's local_loop_guard. A hand-written copy is wrong the first time any
// of them changes, and wrong in the direction that costs: what is safe to
// dispatch is a property of the model behind the endpoint, and we have measured
// two that fail in opposite directions — one declines to edit, the other loops.
// Naming the model is also what makes a transcript readable later, when someone
// reports an edit that went wrong.
//
// A pure function of its inputs, which is the point: opencode reads the file
// into the system prompt, and the 99.3–99.5% prompt-cache reuse a local session
// depends on holds only while that prefix is byte-identical from turn to turn.
func LocalDispatchPolicy(m local.Model, loopGuard int) string {
	var b strings.Builder
	b.WriteString("# Lokal arbeider på denne maskinen\n\n")
	fmt.Fprintf(&b, "Agenten `lokal-arbeider` kjører på %s her på maskinen. Den trekker ingen AI-credits: alt den genererer er gratis, uansett hvor mange tokens det blir. Det er hele poenget med å sende noe dit.\n\n", m.Model)
	if m.Role != "" {
		fmt.Fprintf(&b, "Rollen modellen er valgt for: %s\n", m.Role)
	}
	if m.Expect != "" {
		fmt.Fprintf(&b, "Hva den faktisk leverer: %s\n", m.Expect)
	}
	if m.Role != "" || m.Expect != "" {
		b.WriteString("\n")
	}
	b.WriteString("Send dit: oppslag i koden, kommentarer, loggsetninger, en enkelt testfil, og mekaniske endringer som følger ett mønster — en omdøping treffer kallstedene i flere filer og hører likevel hjemme her.\n")
	b.WriteString("Beskriv endringen ferdig når du sender: hvilken fil, hvilken linje, hva den skal bli. Modellen utfører en avgjørelse godt og tar den dårlig, så er du i tvil om den klarer oppgaven, ta den selv — målingene sier at du vurderer det riktig.\n")
	b.WriteString("Ikke send dit: endringer som krever en egen vurdering per fil, oppgaver som krever mange runder, endringer der en feil endring er dyr.\n\n")
	fmt.Fprintf(&b, "Den svarer som regel på sekunder, men ett enkelt token er målt til tre og et halvt minutt under last. Klienten gir opp av seg selv etter %d minutter uten svar — vent til den gjør det. Avbryter du før, kan du duplisere en endring som fortsatt er underveis.\n\n", max(1, chunkTimeoutMS(m)/60000))
	b.WriteString("Den feiler på to måter. Begge er billige å oppdage, og begge betyr at du tar oppgaven selv i stedet for å sende den om igjen:\n")
	b.WriteString("- Den sier ofte nei og endrer ingenting. Sjekk at filen faktisk er endret. Er den ikke det, har du tapt noen sekunder og ingen credits.\n")
	fmt.Fprintf(&b, "- Den kan gjenta samme verktøykall til nav-pilot avslutter turen etter %d like kall på rad.\n", loopGuard)
	return b.String()
}

// EnsureOpenCodeLocalPolicy provisions the dispatch policy beside the worker
// agent and points opencode at it, once per launch — not per turn, which is
// what a stable prompt prefix requires.
//
// It writes its own file and registers it in the config's "instructions" array
// rather than appending to AGENTS.md. opencode loads exactly one global
// AGENTS.md, ~/.config/opencode/AGENTS.md, and that is the file
// [EnsureOpenCodeNavContext] materializes Nav's own context into and refuses to
// overwrite once the developer has edited it — so appending there would either
// clobber someone's file or be dropped as a conflict. The instructions array is
// the additive hook opencode does offer, it lives in the same config the
// provider block already merges into, and it leaves the developer's own entries
// alone.
//
// The registered path is absolute because opencode resolves a relative
// instructions entry upward from the project directory, not from the config
// directory the file lives in — a relative name would resolve for nobody.
//
// It takes the worker's model, not the session's, and is called only once
// [localWorker] has found one. Taking it back out is [RemoveOpenCodeLocalPolicy],
// which [startLocalDispatch] calls whenever there is no worker: the entry lives
// in a config file that outlives the session that wrote it, so a developer who
// turns the alpha off — or launches with no server up — would otherwise keep
// reading "draws no AI credits" about a worker that is not there. The
// ~650 developers who never turn the alpha on have nothing to unregister, so
// their launch stays byte-identical to the one they have.
//
// The binding goes in the same mutate as the entry, so the claim and the thing
// that makes it true are one write: there is no config in which the fragment is
// registered and the worker is not pinned to the local model.
func EnsureOpenCodeLocalPolicy(m local.Model) error {
	path := localPolicyPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating opencode config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(LocalDispatchPolicy(m, local.LoopGuardRepeat())), 0o600); err != nil {
		return fmt.Errorf("writing the local dispatch policy: %w", err)
	}
	return mutateOpenCodeConfig(func(cfg map[string]any) bool {
		bound := bindLocalWorker(cfg, m)
		entries, _ := cfg["instructions"].([]any)
		if slices.Contains(entries, any(path)) {
			return bound
		}
		cfg["instructions"] = append(entries, path)
		return true
	})
}

// RemoveOpenCodeLocalPolicy takes the dispatch policy back out, which is what
// `alpha local off` owes the developer for the same reason
// [RemoveOpenCodeLocalProvider] does: the instructions entry lives in a file
// opencode reads on its own, so leaving it there keeps telling every session —
// including a hosted one — to dispatch to a worker that is no longer reachable.
// The next launch with local on writes both back.
func RemoveOpenCodeLocalPolicy() error {
	path := localPolicyPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing the local dispatch policy: %w", err)
	}
	return mutateOpenCodeConfig(func(cfg map[string]any) bool {
		entries, _ := cfg["instructions"].([]any)
		kept := slices.DeleteFunc(slices.Clone(entries), func(e any) bool { return e == any(path) })
		if len(kept) == len(entries) {
			return false
		}
		// An empty "instructions": [] left behind is nav-pilot's litter in
		// someone else's file, so it goes too — but only when nav-pilot
		// emptied it.
		if len(kept) == 0 {
			delete(cfg, "instructions")
		} else {
			cfg["instructions"] = kept
		}
		return true
	})
}

// LaunchOpenCode launches opencode inside the cplt sandbox with the resolved config.
// Before launching, it materializes Nav context into opencode's user config directory.
// cplt sandboxes the opencode binary, so opencode must also be installed on PATH.
func LaunchOpenCode(resolved domain.ResolvedConfig) error {
	if _, err := exec.LookPath("opencode"); err != nil {
		return fmt.Errorf("opencode not found in PATH — install it first: https://opencode.ai")
	}

	env := os.Environ()
	if telemetry.CopilotOTelEndpointConfigured(env) {
		if err := EnsureOpenCodeOTelConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "%s Warning: could not configure opencode OTel: %v\n", domain.Yellow("⚠"), err)
		}
	}

	navSummary, ctxErr := EnsureOpenCodeNavContext()
	if ctxErr != nil {
		fmt.Fprintf(os.Stderr, "%s Warning: could not materialize Nav context for opencode: %v\n", domain.Yellow("⚠"), ctxErr)
	}

	for _, msg := range OpenCodeUnsupportedConfigWarnings(resolved) {
		fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), msg)
	}

	launchEnv, _ := telemetry.ApplyOpenCodeOTelEnv(env, cliVersion)

	// Local dispatch, whether or not this session's own model is local: a cloud
	// main agent handing focused tasks to a local worker is the case the
	// feature exists for.
	guard, err := startLocalDispatch(resolved.Model)
	if err != nil {
		return err
	}
	if guard != nil {
		defer guard.Close()
		fmt.Fprintf(os.Stderr, "%s Local dispatch: nav-pilot ends a turn after %d identical tool calls in a row.\n",
			domain.Dim("ℹ"), local.LoopGuardRepeat())
	}

	suffix := ""
	if navSummary != "" {
		suffix = fmt.Sprintf(" with Nav context (%s)", navSummary)
	}

	return launchViaCplt(cpltLaunch{
		agent:         "opencode",
		agentArgs:     openCodeAgentArgs(resolved),
		env:           launchEnv,
		displayName:   "opencode",
		messageSuffix: suffix,
	})
}

// ensureOwnServer is [local.EnsureOwnServer] behind a variable, the same seam
// local/guard.go keeps for the same call and for the same reason: the proof
// needs a recorded process that is still alive and still holding a fixed port,
// which a test cannot arrange without taking that port on the machine it runs
// on.
var ensureOwnServer = local.EnsureOwnServer

// localWorker resolves the model a dispatched task actually reaches on this
// machine: the one the recorded server is serving. Not the session's model —
// the point of the alpha is a cloud main agent handing focused tasks to a local
// worker, so the session model is hosted in the normal case and says nothing
// about what the worker runs.
//
// The zero Model with a nil error means local dispatch is off, which is the
// state of every nav-pilot that never ran the alpha. A non-nil error means it
// is on but there is nothing of nav-pilot's own to dispatch to, and says why.
//
// [local.EnsureOwnServer] is that proof, and it gates what the launch writes
// and not only the guard because all of it is a claim about a server on this
// machine. The guard forwards to a fixed 127.0.0.1:8080, so a server that died
// hours ago and left the port to whatever bound it next would have the
// session's dispatches proxied to a stranger with nothing on screen to say so.
// Server.Start refuses a port nav-pilot does not own; this is the same rule
// where the prompts actually flow. The guard re-proves it per completion behind
// a short cache, because this call only covers the instant of the launch and
// the session it starts runs for hours.
func localWorker() (local.Model, error) {
	if !local.Enabled() {
		return local.Model{}, nil
	}
	if err := ensureOwnServer(); err != nil {
		return local.Model{}, err
	}
	st, _, err := local.LoadState()
	if err != nil {
		return local.Model{}, err
	}
	m, found := local.Lookup(st.Model)
	if !found {
		// EnsureOwnServer proved the process. The manifest is what carries the
		// limits the provider block declares and the prose the dispatch policy
		// quotes, so a model it does not name is one nav-pilot cannot describe
		// honestly to a main agent.
		return local.Model{}, fmt.Errorf(
			"the running local server serves %q, which this nav-pilot's model manifest does not name.\n\n  Start it again:\n\n    %s\n    %s",
			st.Model, domain.Bold("nav-pilot alpha local stop"), domain.Bold("nav-pilot alpha local start"))
	}
	return m, nil
}

// startLocalDispatch sets local dispatch up for one session: the opencode
// provider block, the worker agent's binding to the local model, the dispatch
// policy that tells the main agent what the worker is for, and the loop guard
// every completion passes through. launchViaCplt blocks until the client exits,
// so the returned guard lives exactly as long as the dispatch it guards and
// needs no daemon.
//
// The client reaches the guard by address, not by environment: the provider
// block written here points at the guard's fixed port, because opencode selects
// a backend through its provider config and has no base-URL variable to
// override.
//
// Gated on [local.Enabled] — dispatch turned on and the environment
// provisioned — and not on the session's model. Gating on the session model was
// the defect: a hosted main agent is the normal case for this feature, so the
// binding and the dispatch fragment were written only for a developer who had
// at some point launched a local model themselves, and removed again by their
// next hosted launch. Everyone else got a `lokal-arbeider` with no model of its
// own, which opencode runs on the session's model — every "free" dispatch
// billed to the cloud, beside a policy file saying it was free.
//
// A session with no worker refuses only when the session model is the local one,
// because then there is nothing else for the prompts to run on. A hosted session
// loses the worker and carries on: a developer who left dispatch on and has not
// started a server today still has a session worth launching, and refusing it
// would make `alpha local off` something you have to remember before every
// cloud launch.
//
// The gate is a function rather than an `if` at the call site so a test can
// hold it: with local disabled nothing here listens and nothing here writes,
// pinned by TestHostedLaunchStartsNoLoopGuard, and moving the guard out from
// behind the gate now fails a test instead of nothing.
func startLocalDispatch(sessionModel string) (*local.Guard, error) {
	// Same refusal the Copilot path makes, for the same reason: a session
	// configured for a local model with dispatch off would otherwise be sent to
	// the cloud provider under a Hugging Face model id.
	if local.DisabledLocalModel(sessionModel) {
		return nil, fmt.Errorf(
			"%s runs on this machine, but local inference is off for this install.\n\n  Turn it on:\n\n    %s",
			domain.Bold(sessionModel), domain.Bold("nav-pilot alpha local init"))
	}
	worker, err := localWorker()
	if err != nil {
		if local.IsLocal(sessionModel) {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "%s No local worker this session — %v\n", domain.Yellow("⚠"), err)
	}
	if worker.Model == "" {
		// Off, or on with nothing behind it. Either way the dispatch policy
		// goes: it tells every session the worker draws no AI credits,
		// and there is now no worker for that to be true about. Nothing is
		// written when there is nothing to remove, which is what keeps a launch
		// with the alpha off byte-identical to the one it has always been.
		if err := RemoveOpenCodeLocalPolicy(); err != nil {
			fmt.Fprintf(os.Stderr, "%s Warning: could not remove the local dispatch policy from opencode: %v\n", domain.Yellow("⚠"), err)
		}
		return nil, nil
	}
	// Both non-fatal, like the Nav context above: a session missing them
	// dispatches badly, a refused launch dispatches nothing at all.
	// `alpha local start` writes the provider block too — writing it here as
	// well is what makes a launch self-healing when the developer's config was
	// edited, or written by an older nav-pilot, since the block is what the
	// binding below names.
	if err := EnsureOpenCodeLocalProvider(worker); err != nil {
		fmt.Fprintf(os.Stderr, "%s Warning: could not register the local model with opencode: %v\n", domain.Yellow("⚠"), err)
	}
	if err := EnsureOpenCodeLocalPolicy(worker); err != nil {
		fmt.Fprintf(os.Stderr, "%s Warning: could not provision the local dispatch policy for opencode: %v\n", domain.Yellow("⚠"), err)
	}
	return local.StartGuard(local.ServerURL())
}

// LaunchPi launches pi inside the cplt sandbox. pi must also be installed on
// PATH (cplt sandboxes the pi binary). Nav-pilot config is not forwarded: pi
// uses its own defaults, and PiUnsupportedConfigWarnings says which settings
// were dropped. Nav context is still available via AGENTS.md in the project
// root. Pass-through arguments after "--" are forwarded, as they are for every
// other client: without them `nav-pilot --client pi -- run "..."` started pi
// with no request at all. cplt is required; if it is absent, launchViaCplt
// fails with guidance.
func LaunchPi(resolved domain.ResolvedConfig) error {
	if _, err := exec.LookPath("pi"); err != nil {
		return fmt.Errorf("pi not found in PATH — install it first, or set a different client with: nav-pilot config set client copilot")
	}

	for _, msg := range PiUnsupportedConfigWarnings(resolved) {
		fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), msg)
	}

	return launchViaCplt(cpltLaunch{
		agent:       "pi",
		displayName: "pi",
		agentArgs:   resolved.ExtraArgs,
	})
}

// clientForwardsModel reports whether launching a client puts the resolved
// model on its command line. Only pi does not: [LaunchPi] passes no nav-pilot
// config at all, so a launch notice naming a model for pi would contradict the
// warning [PiUnsupportedConfigWarnings] prints one line later, and would name a
// model the session does not run on.
//
// One place, next to the launch that does the dropping, so the predicate cannot
// drift from it.
func clientForwardsModel(client string) bool { return client != "pi" }

// PiUnsupportedConfigWarnings reports nav-pilot config that a pi launch drops,
// so users understand pi will use its own defaults instead.
//
// The list mirrors the settings LaunchPi does not put on the command line: the
// full launch-relevant half of domain.ResolvedConfig, not just model and mode.
// The silent ones were the dangerous ones: allow_all_tools and ask_user read
// as permission settings, and a user who turned them off had no way to see that
// pi never received them.
func PiUnsupportedConfigWarnings(resolved domain.ResolvedConfig) []string {
	var warnings []string
	add := func(setting, value string) {
		warnings = append(warnings, fmt.Sprintf("%s %s is not forwarded to pi yet — pi will use its own default", setting, value))
	}
	if resolved.Model != "" {
		add("model", fmt.Sprintf("%q", resolved.Model))
	}
	if resolved.Mode != "" && resolved.Mode != "default" {
		add("mode", fmt.Sprintf("%q", resolved.Mode))
	}
	if resolved.ReasoningEffort != "" {
		add("reasoning_effort", fmt.Sprintf("%q", resolved.ReasoningEffort))
	}
	if resolved.ContextTier != "" && resolved.ContextTier != "default" {
		add("context_tier", fmt.Sprintf("%q", resolved.ContextTier))
	}
	if resolved.AllowAllTools {
		add("allow_all_tools", "true")
	}
	if !resolved.AskUser {
		add("ask_user", "false")
	}
	if resolved.LogLevel != "" {
		add("log_level", fmt.Sprintf("%q", resolved.LogLevel))
	}
	return warnings
}
