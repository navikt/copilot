package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// validateModelForClient validates a model identifier by delegating to the
// Provider implementation for the given client id. Kept as a free function for
// use in validateConfigProblems and tests.
func validateModelForClient(model, client string) error {
	p, err := providerFor(client)
	if err != nil {
		// Unknown provider: fall back to base shape validation.
		return validateModelValue(model)
	}
	return p.ValidateModel(model)
}

// configPath returns the path to the user config file.
// Honors NAV_PILOT_CONFIG env var if set.
// A leading ~ is expanded and a relative path made absolute, so every command
// names the same file whatever directory it runs in. $XDG_CONFIG_HOME is not
// read: the file lives in ~/.nav-pilot next to the rest of nav-pilot's state.
func configPath() string {
	home, _ := os.UserHomeDir()
	p := os.Getenv("NAV_PILOT_CONFIG")
	if p == "" {
		return filepath.Join(home, ".nav-pilot", "config.toml")
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		p = filepath.Join(home, p[1:])
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return p
}

// readConfig reads and parses the config file at configPath().
// Returns (nil, nil) if the file does not exist (fail-soft).
// Returns an error if the file is not valid TOML. A key of the wrong type is
// left out (so its default applies); loadConfig reports it.
func readConfig() (*Config, error) {
	cfg, _, err := loadConfig()
	return cfg, err
}

// loadConfig reads the config file and returns it with every problem found
// in it: keys of the wrong type, unknown keys, and invalid values, in that
// order. Only TOML that does not parse is an error. A missing file is
// (nil, nil, nil).
func loadConfig() (*Config, []string, error) {
	cfg, problems, err := readConfigFile()
	if cfg != nil {
		problems = append(problems, validateConfigProblems(cfg)...)
	}
	return cfg, problems, err
}

// readConfigFile is loadConfig without the value checks: the problems are
// keys of the wrong type and unknown keys.
func readConfigFile() (*Config, []string, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	raw := map[string]any{}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	// A value of the wrong type is a problem to report, not a reason to stop
	// reading: take it out and decode the rest.
	var problems []string
	for _, kd := range configKeyDefs {
		if v, ok := raw[kd.name]; ok && !valueFitsKind(v, kd.kind) {
			problems = append(problems, typeProblem(&kd, v))
			delete(raw, kd.name)
		}
	}
	text := string(data)
	if len(problems) > 0 {
		var b strings.Builder
		if err := toml.NewEncoder(&b).Encode(raw); err == nil {
			text = b.String()
		}
	}
	var cfg Config
	meta, err := toml.Decode(text, &cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	for _, k := range meta.Undecoded() {
		if key := strings.Join(k, "."); !ignoredConfigKeys[key] {
			problems = append(problems, unknownKeyProblem(key))
		}
	}
	return &cfg, problems, nil
}

func valueFitsKind(v any, kind keyKind) bool {
	switch kind {
	case keyKindInt:
		_, ok := v.(int64)
		return ok
	case keyKindBool:
		_, ok := v.(bool)
		return ok
	default:
		_, ok := v.(string)
		return ok
	}
}

// typeProblem says what a key of the wrong type should look like, with the
// line to write: `local_loop_guard = "8"` becomes local_loop_guard = 8.
func typeProblem(kd *configKeyDef, v any) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	switch kd.kind {
	case keyKindInt:
		example := kd.defaultVal
		if _, err := strconv.Atoi(s); err == nil {
			example = s
		}
		if example == "" {
			example = "1"
		}
		return fmt.Sprintf("%s must be a number: write %s = %s", kd.name, kd.name, example)
	case keyKindBool:
		example := kd.defaultVal
		switch strings.ToLower(s) {
		case "true", "yes", "on", "1":
			example = "true"
		case "false", "no", "off", "0":
			example = "false"
		}
		if example == "" {
			example = "false"
		}
		return fmt.Sprintf("%s must be true or false: write %s = %s", kd.name, kd.name, example)
	default:
		return fmt.Sprintf("%s must be a string in quotes: write %s = %s", kd.name, kd.name, tomlString(s))
	}
}

// configuredSourceRepo returns the source persisted in the config file, or the
// empty string when none is set (meaning the built-in default).
func configuredSourceRepo() (string, error) {
	cfg, err := readConfig()
	if err != nil {
		return "", err
	}
	if cfg == nil || cfg.Source == nil {
		return "", nil
	}
	return strings.TrimSpace(*cfg.Source), nil
}

// sourceRepoFor applies the source precedence for content resolution:
// explicit --source flag > config file `source` > built-in default (B1).
// An empty result means the default, which resolveSource already implements.
func sourceRepoFor(flagSource string) (string, error) {
	if flagSource != "" {
		return flagSource, nil
	}
	configured, err := configuredSourceRepo()
	if err != nil {
		return "", fmt.Errorf("%w\n\nFix %s or run `nav-pilot config validate`", err, configPath())
	}
	if configured == "" {
		return "", nil
	}
	if err := validateSourceValue(configured); err != nil {
		return "", fmt.Errorf("config key %s is invalid: %w\n\nFix %s, or clear it with `nav-pilot config set source \"\"`",
			bold("source"), err, configPath())
	}
	return configured, nil
}

// validateConfigProblems checks semantic correctness of a parsed config and
// returns a list of human-readable problem strings (empty = valid).
// It does NOT check for unknown TOML keys — use MetaData.Undecoded() for that.
func validateConfigProblems(cfg *Config) []string {
	if cfg == nil {
		return nil
	}
	var problems []string

	switch cfg.Version {
	case 0, 1: // a missing version reads as 1; configAdvice says so
	default:
		problems = append(problems, fmt.Sprintf("version must be 1 (got %d)", cfg.Version))
	}
	if cfg.Client != nil && !containsStr(validProviderIDs, *cfg.Client) {
		problems = append(problems, fmt.Sprintf("client %q is not valid (allowed: %s)",
			*cfg.Client, strings.Join(validProviderIDs, ", ")))
	}
	if cfg.Model != nil {
		if problem, _ := modelAdvice(*cfg.Model, cfgClient(cfg), cfg.LocalModel != nil, false); problem != "" {
			problems = append(problems, problem)
		}
	}
	// An empty source is how the key is cleared back to the default, so only a
	// non-empty value is checked for shape.
	if cfg.Source != nil && strings.TrimSpace(*cfg.Source) != "" {
		if err := validateSourceValue(*cfg.Source); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if cfg.Mode != nil && !containsStr(validModes, *cfg.Mode) {
		problems = append(problems, fmt.Sprintf("mode %q is not valid (allowed: %s)",
			*cfg.Mode, strings.Join(validModes, ", ")))
	}
	if cfg.ReasoningEffort != nil && !containsStr(validReasoningEffort, *cfg.ReasoningEffort) {
		problems = append(problems, fmt.Sprintf("reasoning_effort %q is not valid (allowed: %s)",
			*cfg.ReasoningEffort, strings.Join(validReasoningEffort, ", ")))
	}
	if cfg.ContextTier != nil && !containsStr(validContextTiers, *cfg.ContextTier) {
		problems = append(problems, fmt.Sprintf("context_tier %q is not valid (allowed: %s)",
			*cfg.ContextTier, strings.Join(validContextTiers, ", ")))
	}
	if cfg.LogLevel != nil && !containsStr(validLogLevels, *cfg.LogLevel) {
		problems = append(problems, fmt.Sprintf("log_level %q is not valid (allowed: %s)",
			*cfg.LogLevel, strings.Join(validLogLevels, ", ")))
	}
	// A threshold below 2 is not a loop guard: one tool call is not a loop, and
	// tripping on the first call disables local dispatch rather than guarding
	// it. Caught here rather than clamped silently, so the config says what the
	// binary does.
	if cfg.LocalLoopGuard != nil && *cfg.LocalLoopGuard < 2 {
		problems = append(problems, fmt.Sprintf(
			"local_loop_guard must be at least 2 (got %d) — one tool call is not a loop", *cfg.LocalLoopGuard))
	}
	problems = append(problems, endpointProblems(cfg)...)
	if cfg.OtelLogLevel != nil && !containsStr(validOtelLogLevels, *cfg.OtelLogLevel) {
		problems = append(problems, fmt.Sprintf("otel_log_level %q is not valid (allowed: %s)",
			*cfg.OtelLogLevel, strings.Join(validOtelLogLevels, ", ")))
	}
	if cfg.LocalDispatch != nil && !containsStr(validLocalDispatch, *cfg.LocalDispatch) {
		problems = append(problems, fmt.Sprintf("local_dispatch %q is not valid (allowed: %s)",
			*cfg.LocalDispatch, strings.Join(validLocalDispatch, ", ")))
	}
	if cfg.HookActionCheck != nil && !containsStr(validHookActionCheck, *cfg.HookActionCheck) {
		problems = append(problems, fmt.Sprintf("hook_action_check %q is not valid (allowed: %s)",
			*cfg.HookActionCheck, strings.Join(validHookActionCheck, ", ")))
	}
	if cfg.MCPHosts != nil && !containsStr(validMCPHosts, *cfg.MCPHosts) {
		problems = append(problems, fmt.Sprintf("mcp_hosts %q is not valid (allowed: %s)",
			*cfg.MCPHosts, strings.Join(validMCPHosts, ", ")))
	}
	if cfg.Autonomy != nil && !containsStr(validAutonomy, *cfg.Autonomy) {
		problems = append(problems, fmt.Sprintf("autonomy %q is not valid (allowed: %s)",
			*cfg.Autonomy, strings.Join(validAutonomy, ", ")))
	}
	if cfg.CopilotAuthMode != nil && !containsStr(validCopilotAuthModes, *cfg.CopilotAuthMode) {
		problems = append(problems, fmt.Sprintf("copilot_auth_mode %q is not valid (allowed: %s)",
			*cfg.CopilotAuthMode, strings.Join(validCopilotAuthModes, ", ")))
	}
	return problems
}

// endpointProblems checks local_endpoint and local_endpoint_model: an address
// nav-pilot may send code to, and a model to ask it for. One without the other
// is refused, since neither means anything alone.
func endpointProblems(cfg *Config) []string {
	endpoint, model := "", ""
	if cfg.LocalEndpoint != nil {
		endpoint = strings.TrimSpace(*cfg.LocalEndpoint)
	}
	if cfg.LocalEndpointModel != nil {
		model = strings.TrimSpace(*cfg.LocalEndpointModel)
	}
	var problems []string
	if endpoint != "" {
		if _, err := local.ValidateEndpoint(endpoint); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if model != "" {
		if err := validateModelValue(model); err != nil {
			problems = append(problems, "local_endpoint_model: "+err.Error())
		}
	}
	switch {
	case endpoint != "" && model == "":
		problems = append(problems, "local_endpoint is set but local_endpoint_model is not: nav-pilot config set local_endpoint_model <model id>")
	case endpoint == "" && model != "":
		problems = append(problems, "local_endpoint_model is set but local_endpoint is not: nav-pilot config set local_endpoint http://127.0.0.1:11434/v1")
	}
	return problems
}

// validateConfig checks semantic correctness of a parsed config.
// Returns an error listing all problems found, or nil if valid.
// It does NOT check for unknown TOML keys — use MetaData.Undecoded() for that.
func validateConfig(cfg *Config) error {
	problems := validateConfigProblems(cfg)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("config validation failed:\n  - %s", strings.Join(problems, "\n  - "))
}

// launchConfig is the file config with the launch's --client and --model in
// place of the file's values, or nil when there is neither.
func launchConfig(file *Config, cli CLIOverrides) *Config {
	if cli.Client == "" && cli.Model == "" {
		return file
	}
	eff := Config{Version: 1}
	if file != nil {
		eff = *file
	}
	if cli.Client != "" {
		eff.Client = &cli.Client
	}
	if eff.Client == nil && file == nil {
		c := defaultClient(nil)
		eff.Client = &c
	}
	if cli.Model != "" {
		eff.Model = &cli.Model
	}
	return &eff
}

// versionMissingAdvice is the one line a config without a version gets.
const versionMissingAdvice = "version is missing, so nav-pilot reads the file as version 1. Add it to silence this: nav-pilot config set version 1"

func cfgClient(cfg *Config) string {
	if cfg == nil || cfg.Client == nil {
		return defaultClient(cfg)
	}
	return *cfg.Client
}

// defaultClient is the client when neither --client nor config.toml names
// one (#1022). A new install, a machine with no config.toml and no recorded
// client, gets opencode. Anyone who has run nav-pilot before gets copilot,
// the default they had, so an upgrade moves nobody (#1029): a config.toml
// without a client line means copilot. A new install without opencode on
// PATH gets copilot too, so the default is never a client that is missing.
//
// Only in a terminal: a headless or CI run with no config.toml cannot be told
// from an existing user who never had one, so it keeps copilot.
func defaultClient(file *Config) string {
	if newInstall(file) && isInteractive() && opencodeInstalled() {
		return "opencode"
	}
	return "copilot"
}

// newInstall reports whether this machine has neither a config.toml nor the
// marker recordEffectiveClient leaves.
func newInstall(file *Config) bool {
	if file != nil {
		return false
	}
	// The marker sits beside config.toml (telemetry.GetConfigDir), read
	// here without GetConfigDir's MkdirAll: resolve runs in every hook.
	_, err := os.Stat(filepath.Join(filepath.Dir(configPath()), "seen-client-recorded"))
	return errors.Is(err, os.ErrNotExist)
}

// opencodeInstalled reports whether opencode is on PATH.
func opencodeInstalled() bool {
	p, err := providerFor("opencode")
	return err == nil && p.Available()
}

// modelAdvice is everything nav-pilot says about a model id for a client:
// problem makes the config invalid, advice does not. config set, config
// validate, the launch and the init template's wording all follow it, so they
// cannot disagree. atLaunch leaves out the github-copilot/ note, which the
// launch's session-model line already gives.
func modelAdvice(model, client string, localModelSet, atLaunch bool) (problem, advice string) {
	if client == "" {
		client = "copilot"
	}
	if err := validateModelForClient(model, client); err != nil {
		return err.Error(), ""
	}
	// model naming a local id is legal and means "run the session locally".
	// It is also the mistake people make when they meant to pick which model
	// the worker loads, so it is said out loud rather than guessed at — but
	// only while local_model is unset: someone who has set both has chosen.
	if !localModelSet {
		if _, ok := local.Lookup(model); ok {
			return "", fmt.Sprintf(
				"model %q runs this session on the local model. To choose which model the local server loads, set local_model instead: %s.",
				model, bold("nav-pilot alpha local use <key>"))
		}
	}
	// A local manifest key (qwen3.6-35b-a3b-optiq) is what alpha local use
	// takes; model wants a model id, so the key would go to Copilot as-is.
	for _, m := range local.Active().Models {
		if m.Key == model && m.Model != model {
			return "", fmt.Sprintf("%q is a local model key, not a model id. To run it on this machine: %s",
				model, bold("nav-pilot alpha local use "+model))
		}
	}
	if client == "copilot" && !atLaunch {
		if note := providerpkg.CopilotModelNote(model); note != "" {
			return "", fmt.Sprintf("model %q: %s.", model, note)
		}
	}
	if p, err := providerFor(client); err == nil {
		advisory := p.ModelAdvisory(model)
		if advisory != "" {
			if hint := modelHint(model, p.KnownModels()); hint != "" {
				return "", fmt.Sprintf("model %q is not a recognized model id. Did you mean %s? It will be sent as-is; nav-pilot models lists the known ids.", model, hint)
			}
		}
		return "", advisory
	}
	return "", ""
}

// modelHint names the known ids an unknown one was probably meant as: a typo
// (claude-sonet-5) or a family name (opus). Empty when nothing is close.
func modelHint(model string, known []domain.ModelChoice) string {
	ids := make([]string, 0, len(known))
	for _, m := range known {
		ids = append(ids, m.ID)
	}
	if hint := suggest(model, ids); hint != "" {
		return hint
	}
	var matches []string
	for _, id := range ids {
		if len(model) >= 3 && strings.Contains(id, strings.ToLower(model)) {
			matches = append(matches, id)
		}
	}
	if len(matches) == 0 || len(matches) > 8 {
		return ""
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return "one of " + strings.Join(matches, ", ")
}

// configAdvice returns the non-fatal notes for a parsed config: a missing
// version, and what modelAdvice says about the model.
func configAdvice(cfg *Config, atLaunch bool) []string {
	if cfg == nil {
		return nil
	}
	var advice []string
	if cfg.Version == 0 {
		advice = append(advice, versionMissingAdvice)
	}
	if cfg.Model != nil {
		if _, a := modelAdvice(*cfg.Model, cfgClient(cfg), cfg.LocalModel != nil, atLaunch); a != "" {
			advice = append(advice, a)
		}
	}
	return advice
}

// loadConfigForLaunch reads, validates, and resolves the user config ahead of a
// launch. Hard validation errors (unknown keys, invalid enum values, wrong version,
// malformed model) cause it to refuse with an error so nav-pilot does not start
// with a broken config. Non-fatal advisories (unrecognized model ids) are printed
// to stderr but do not block the launch.
func loadConfigForLaunch(cli CLIOverrides) (ResolvedConfig, error) {
	file, problems, err := readConfigFile()
	if err != nil {
		return ResolvedConfig{}, fmt.Errorf("%w\n\n%s", err, configFixHint())
	}
	// Values are checked as this launch will use them: --client and --model
	// replace the file's, so the model is checked against the client that
	// actually starts.
	effective := launchConfig(file, cli)
	problems = append(problems, validateConfigProblems(effective)...)
	// Every problem, the same list config validate prints: fixing one only to
	// be shown the next is a loop.
	if len(problems) > 0 {
		return ResolvedConfig{}, fmt.Errorf("config has %d problem(s):\n  - %s\n\n%s",
			len(problems), strings.Join(problems, "\n  - "), configFixHint())
	}
	for _, w := range configAdvice(effective, true) {
		fmt.Fprintf(os.Stderr, "%s %s\n", yellow("⚠"), w)
	}
	resolved := resolve(file, cli)
	telemetry.RecordConfig(
		resolved.Client,
		resolved.Mode,
		configModelLabel(resolved.Model),
		resolved.ReasoningEffort,
		resolved.ContextTier,
		resolved.OtelLogLevel,
		resolved.AllowAllTools,
		resolved.AskUser,
	)
	return resolved, nil
}

// ignoredConfigKeys are retired keys an old config file may still hold. They
// load without a word and nothing reads them: nav-pilot wrote them itself to
// remember the rtk setup prompt, which is gone (#1321).
var ignoredConfigKeys = map[string]bool{"rtk_prompted_client": true, "rtk_prompted_at": true}

// renamedConfigKeys maps a retired config key to the key that replaced it.
var renamedConfigKeys = map[string]string{"agent": "client"}

// unknownKeyProblem describes a key the config does not know. A renamed key
// names its successor and the command that moves its value over; `config set`
// drops the old line when it writes the new one.
func unknownKeyProblem(key string) string {
	next, ok := renamedConfigKeys[key]
	if !ok {
		return "unknown key: " + key
	}
	value := "<value>"
	var raw map[string]any
	if _, err := toml.DecodeFile(configPath(), &raw); err == nil {
		if v, ok := raw[key].(string); ok {
			value = v
		}
	}
	return fmt.Sprintf("%s was renamed to %s: nav-pilot config set %s %s", key, next, next, value)
}

// configFixHint is the way out of a config nav-pilot refuses to launch with.
func configFixHint() string {
	return fmt.Sprintf("Fix %s, then check it with nav-pilot config validate", configPath())
}

// navPilotUsedBefore reports whether nav-pilot has run on this machine
// before, config.toml or not: the marker the first cplt launch on a terminal
// leaves in the config directory (provider.PrintCpltSandboxHint), or a
// user-scope install's state file. Such a user keeps conservative autonomy
// until they choose; only a new user starts on sandbox.
func navPilotUsedBefore() bool {
	if dir, err := telemetrypkg.GetConfigDir(); err == nil {
		if _, err := os.Stat(filepath.Join(dir, "seen-cplt-hint")); err == nil {
			return true
		}
	}
	if scope, err := domain.ScopeUser(); err == nil {
		if _, err := os.Stat(scope.StatePath()); err == nil {
			return true
		}
	}
	return false
}

// resolve builds a ResolvedConfig from file config and CLI overrides.
// Precedence: CLI flag > file value > built-in default.
func resolve(file *Config, cli CLIOverrides) ResolvedConfig {
	r := ResolvedConfig{
		Client:            defaultClient(file),
		Mode:              "default",
		AskUser:           true,
		AutoLaunch:        true,
		Surveys:           true,
		News:              true,
		OtelLogLevel:      "none",
		LocalDispatch:     "balanced",
		CopilotAuthMode:   "auto",
		HookLoopGuard:     true,
		HookRedactSecrets: true,
		HookRedactFNR:     true,
		HookInjectionNote: true,
		HookActionCheck:   "log",
		MCPHosts:          "ask",
	}

	// No config.toml and no sign of an earlier run is a new user, who gets
	// the new default. A file without the key, or a machine nav-pilot has run
	// on without one, is someone from before this existed, who keeps the
	// prompts they had.
	r.Autonomy = "conservative"
	if file == nil && !navPilotUsedBefore() {
		r.Autonomy = "sandbox"
	}

	// Apply file values.
	if file != nil {
		if file.Autonomy != nil {
			r.Autonomy = *file.Autonomy
		}
		if file.Client != nil {
			r.Client = *file.Client
		}
		if file.Source != nil {
			r.Source = strings.TrimSpace(*file.Source)
		}
		if file.Model != nil {
			r.Model = *file.Model
		}
		if file.Mode != nil {
			r.Mode = *file.Mode
		}
		if file.ReasoningEffort != nil {
			r.ReasoningEffort = *file.ReasoningEffort
		}
		if file.ContextTier != nil {
			r.ContextTier = *file.ContextTier
		}
		if file.AllowAllTools != nil {
			r.AllowAllTools = *file.AllowAllTools
		}
		if file.AskUser != nil {
			r.AskUser = *file.AskUser
		}
		if file.AutoLaunch != nil {
			r.AutoLaunch = *file.AutoLaunch
		}
		if file.AutoUpdate != nil {
			r.AutoUpdate = *file.AutoUpdate
		}
		if file.Surveys != nil {
			r.Surveys = *file.Surveys
		}
		if file.News != nil {
			r.News = *file.News
		}
		if file.LogLevel != nil {
			r.LogLevel = *file.LogLevel
		}
		if file.OtelLogLevel != nil {
			r.OtelLogLevel = *file.OtelLogLevel
		}
		if file.LocalAutostart != nil {
			r.LocalAutostart = *file.LocalAutostart
		}
		if file.LocalEnabled != nil {
			r.LocalEnabled = *file.LocalEnabled
		}
		if file.LocalLoopGuard != nil {
			r.LocalLoopGuard = *file.LocalLoopGuard
		}
		if file.LocalModel != nil {
			r.LocalModel = strings.TrimSpace(*file.LocalModel)
		}
		if file.LocalEndpoint != nil {
			r.LocalEndpoint = strings.TrimSpace(*file.LocalEndpoint)
		}
		if file.LocalEndpointModel != nil {
			r.LocalEndpointModel = strings.TrimSpace(*file.LocalEndpointModel)
		}
		if file.LocalDispatch != nil && containsStr(validLocalDispatch, *file.LocalDispatch) {
			r.LocalDispatch = *file.LocalDispatch
		}
		if file.CopilotAuthMode != nil {
			r.CopilotAuthMode = *file.CopilotAuthMode
		}
		if file.HookLoopGuard != nil {
			r.HookLoopGuard = *file.HookLoopGuard
		}
		if file.HookRedactSecrets != nil {
			r.HookRedactSecrets = *file.HookRedactSecrets
		}
		if file.HookRedactFNR != nil {
			r.HookRedactFNR = *file.HookRedactFNR
		}
		if file.HookInjectionNote != nil {
			r.HookInjectionNote = *file.HookInjectionNote
		}
		if file.HookActionCheck != nil && containsStr(validHookActionCheck, *file.HookActionCheck) {
			r.HookActionCheck = *file.HookActionCheck
		}
		if file.MCPHosts != nil && containsStr(validMCPHosts, *file.MCPHosts) {
			r.MCPHosts = *file.MCPHosts
		}
	}

	// Apply CLI overrides (higher precedence than file).
	if cli.Client != "" {
		r.Client = cli.Client
	}
	if cli.Source != "" {
		r.Source = cli.Source
	}
	if cli.PayloadContext != "" {
		r.PayloadContext = cli.PayloadContext
	}
	if cli.Persona != "" {
		r.Persona = cli.Persona
	}
	if cli.Model != "" {
		r.Model = cli.Model
	}
	if cli.Mode != "" {
		r.Mode = cli.Mode
	}
	if cli.ReasoningEffort != "" {
		r.ReasoningEffort = cli.ReasoningEffort
	}
	if cli.ContextTier != "" {
		r.ContextTier = cli.ContextTier
	}
	if cli.AllowAllTools != nil {
		r.AllowAllTools = *cli.AllowAllTools
	}
	if cli.AskUser != nil {
		r.AskUser = *cli.AskUser
	}
	if cli.AutoLaunch != nil {
		r.AutoLaunch = *cli.AutoLaunch
	}
	if cli.LogLevel != "" {
		r.LogLevel = cli.LogLevel
	}
	if cli.OtelLogLevel != "" {
		r.OtelLogLevel = cli.OtelLogLevel
	}
	if cli.LocalDispatch != "" {
		r.LocalDispatch = cli.LocalDispatch
	}
	r.ProjectDir = cli.ProjectDir
	r.NoSandbox = cli.NoSandbox
	r.ExtraArgs = cli.ExtraArgs
	return r
}
