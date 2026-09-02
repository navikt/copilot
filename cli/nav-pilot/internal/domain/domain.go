package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
)

// Config holds user-specific nav-pilot configuration read from ~/.nav-pilot/config.toml.
// Pointer types are used for optional fields to distinguish "unset" from zero-value,
// enabling correct per-field precedence in resolve().
type Config struct {
	Version         int     `toml:"version"`
	Client          *string `toml:"client"`
	Source          *string `toml:"source"`
	Model           *string `toml:"model"`
	Mode            *string `toml:"mode"`
	ReasoningEffort *string `toml:"reasoning_effort"`
	ContextTier     *string `toml:"context_tier"`
	AllowAllTools   *bool   `toml:"allow_all_tools"`
	AskUser         *bool   `toml:"ask_user"`
	// AutoLaunch controls whether nav-pilot starts the coding agent by itself
	// after an install/sync. Defaults to true; false means never launch.
	AutoLaunch        *bool   `toml:"auto_launch"`
	LogLevel          *string `toml:"log_level"`
	OtelLogLevel      *string `toml:"otel_log_level"`
	RtkPromptedClient *string `toml:"rtk_prompted_client"`
	RtkPromptedAt     *string `toml:"rtk_prompted_at"`
	AutoUpdate        *bool   `toml:"auto_update"`
	// LocalEnabled is the alpha opt-in for local inference. Unset and false
	// both mean off, and off means a developer sees no trace of it anywhere:
	// no local models in the picker, no branch taken on any launch path.
	// `nav-pilot alpha local init` sets it; `alpha local off` clears it.
	LocalEnabled *bool `toml:"local_enabled"`
	// LocalAutostart starts the local server on demand at launch, when local
	// dispatch is on and nothing is running. Off by default, because starting a
	// 21 GB process is not something to do without being asked.
	LocalAutostart *bool `toml:"local_autostart"`
	// LocalLoopGuard is how many identical consecutive tool calls end a local
	// turn. Unset means the built-in default. It is a knob because the right
	// number depends on the model and the task, not because anyone should
	// have to set it.
	LocalLoopGuard *int `toml:"local_loop_guard"`
	// LocalModel is which model `alpha local start` loads and serves. Empty
	// means the manifest's default. It is separate from Model because Model is
	// the session model: a developer running a cloud main agent with a local
	// worker has to be able to name both.
	LocalModel *string `toml:"local_model"`
}

// ResolvedConfig holds the final configuration after applying precedence:
// CLI flag > file value > built-in default.
type ResolvedConfig struct {
	Client string
	// Source is the agentpakke content source: a git repo "owner/name" or an
	// absolute path. Empty means the built-in default (navikt/copilot).
	Source string
	// PayloadContext selects which Tier 2 payload context an agentpakke
	// launch stages ("full", "focused", …). Empty means the context the
	// manifest declares as default. Unrelated to ContextTier, which is
	// Copilot's own long-context setting; the two coexist on one command line.
	PayloadContext    string
	Model             string // empty = use agent default
	Mode              string
	ReasoningEffort   string // empty = unset
	ContextTier       string // empty = unset
	AllowAllTools     bool
	AskUser           bool
	AutoLaunch        bool     // launch the coding agent automatically after install/sync
	LogLevel          string   // empty = unset
	OtelLogLevel      string   // always set; defaults to "none"
	RtkPromptedClient string   // comma-separated list of clients where the RTK setup was prompted
	RtkPromptedAt     string   // RFC3339 timestamp of when the user was last prompted
	AutoUpdate        bool     // true to bypass upgrade prompt
	LocalEnabled      bool     // local inference opt-in (alpha)
	LocalAutostart    bool     // start the local server on demand at launch
	LocalLoopGuard    int      // identical consecutive tool calls that end a local turn; 0 = built-in default
	LocalModel        string   // local model id to serve; empty = the manifest default
	ExtraArgs         []string // pass-through arguments for the client
}

// CLIOverrides holds optional CLI flag values. Empty string means "not provided via CLI".
type CLIOverrides struct {
	Client string
	// Source is the --source flag value (agentpakke repo "owner/name" or an
	// absolute path). It takes precedence over the config file's source key.
	Source string
	// PayloadContext is the --payload-context flag value (Tier 2 payload
	// context id). It has no config-file key: the persistent default is the
	// agentpakke manifest's defaultContext.
	PayloadContext  string
	Model           string
	Mode            string
	ReasoningEffort string
	ContextTier     string
	AllowAllTools   *bool
	AskUser         *bool
	AutoLaunch      *bool
	LogLevel        string
	OtelLogLevel    string
	ExtraArgs       []string
}

var (
	ValidModes           = []string{"default", "plan", "autopilot"}
	ValidReasoningEffort = []string{"none", "low", "medium", "high", "xhigh", "max"}
	ValidContextTiers    = []string{"default", "long_context"}
	ValidLogLevels       = []string{"none", "error", "warning", "info", "debug", "all", "default"}
	ValidOtelLogLevels   = []string{"none", "error", "warning", "warn", "info", "debug", "verbose", "all"}
)

// ModelChoice pairs a model id (the --model value) with a human-readable label.
type ModelChoice struct {
	ID    string
	Label string
}

// OpenCodeProviderPrefix is the opencode provider that cplt authenticates
// opencode against. Bare Copilot-style model ids are qualified under it.
const OpenCodeProviderPrefix = "github-copilot/"

// KnownCopilotModels is the curated Copilot model list. Source of truth for
// ids: models.dev github-copilot provider. Keep pinned (no dynamic fetch):
// update it when Copilot ships new models.
//
// It lives here rather than in internal/provider because two packages need the
// same pairing and cannot import each other: provider builds the launch flags
// and the pickers from it, and internal/artifacts resolves an agent's
// frontmatter model label against it while materializing opencode agents.
// provider imports artifacts, so domain, which both import, is the only home
// that keeps one table instead of two.
var KnownCopilotModels = []ModelChoice{
	{ID: "auto", Label: "Auto (let Copilot pick)"},
	{ID: "claude-opus-5", Label: "Claude Opus 5"},
	{ID: "claude-fable-5", Label: "Claude Fable 5"},
	{ID: "claude-sonnet-5", Label: "Claude Sonnet 5"},
	{ID: "claude-sonnet-4.6", Label: "Claude Sonnet 4.6"},
	{ID: "claude-haiku-4.5", Label: "Claude Haiku 4.5"},
	{ID: "claude-opus-4.8", Label: "Claude Opus 4.8"},
	{ID: "claude-opus-4.6", Label: "Claude Opus 4.6"},
	{ID: "gpt-5.6-sol", Label: "GPT-5.6 Sol"},
	{ID: "gpt-5.6-terra", Label: "GPT-5.6 Terra"},
	{ID: "gpt-5.6-luna", Label: "GPT-5.6 Luna"},
	{ID: "gpt-5.5", Label: "GPT-5.5"},
	{ID: "gpt-5.4", Label: "GPT-5.4"},
	{ID: "gpt-5.3-codex", Label: "GPT-5.3-Codex"},
	{ID: "gpt-5.4-mini", Label: "GPT-5.4 mini"},
	{ID: "gpt-5-mini", Label: "GPT-5 mini"},
	{ID: "gemini-3.6-flash", Label: "Gemini 3.6 Flash"},
	{ID: "gemini-3.1-pro-preview", Label: "Gemini 3.1 Pro (Preview)"},
	{ID: "gemini-3.5-flash", Label: "Gemini 3.5 Flash"},
	{ID: "kimi-k2.7-code", Label: "Kimi K2.7 Code"},
	{ID: "kimi-k3", Label: "Kimi K3"},
}

// OpenCodeModelForLabel maps a model name as written in Nav agent frontmatter
// to the provider-qualified opencode model id. Frontmatter carries display
// names ("Claude Sonnet 4.6"); opencode needs "github-copilot/claude-sonnet-4.6".
// A known id is accepted in the same position, so an agent author who writes
// the id instead of the label is not silently ignored.
//
// It returns "" for anything not in [KnownCopilotModels]. That is the point:
// the caller must then emit no model line at all rather than guess an id that
// the client would reject at launch.
func OpenCodeModelForLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for _, m := range KnownCopilotModels {
		if strings.EqualFold(m.Label, name) || strings.EqualFold(m.ID, name) {
			return OpenCodeProviderPrefix + m.ID
		}
	}
	return ""
}

// ModelValuePattern restricts model identifiers to a sane character set that
// covers Copilot ids (e.g. "claude-opus-4.8", "gpt-5.5") and opencode
// provider/model ids (e.g. "anthropic/claude-3-5-sonnet").
var ModelValuePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// ValidateModelValue applies strong format validation to a model identifier.
// The model catalog is dynamic (Copilot validates server-side), so this checks
// shape rather than membership: non-empty, no surrounding/inner whitespace, and
// a restricted character set. This rejects typos and garbage while remaining
// correct as the model catalog evolves.
func ValidateModelValue(model string) error {
	if strings.TrimSpace(model) != model {
		return fmt.Errorf("model %q must not have leading or trailing whitespace", model)
	}
	if model == "" {
		return errors.New("model must not be empty (omit the key to use the agent default)")
	}
	if !ModelValuePattern.MatchString(model) {
		return fmt.Errorf("model %q is not a valid identifier (allowed characters: letters, digits, '.', '_', '-', '/')", model)
	}
	return nil
}

// ValidateOptionalModel is a huh form validator: it accepts a blank value
// (meaning "unset / agent default") and otherwise applies ValidateModelValue.
func ValidateOptionalModel(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return ValidateModelValue(s)
}

func ContainsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// InstallScope encapsulates the differences between repo-level and user-level installs.
type InstallScope struct {
	Name           string   // "repo" or "user"
	RootDir        string   // git root (repo) or ~/.copilot (user)
	StateFile      string   // path relative to RootDir
	PathPrefix     string   // ".github/" (repo) or "" (user)
	SupportedTypes []string // artifact types that can be installed
}

// ScopeRepo creates a scope for repo-level installs (.github/).
func ScopeRepo(targetDir string) *InstallScope {
	return &InstallScope{
		Name:           "repo",
		RootDir:        targetDir,
		StateFile:      ".github/.nav-pilot-state.json",
		PathPrefix:     ".github/",
		SupportedTypes: []string{"agent", "skill", "instruction", "prompt"},
	}
}

// ScopeUser creates a scope for user-level installs (~/.copilot/).
func ScopeUser() (*InstallScope, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	rootDir := filepath.Join(home, ".copilot")
	return &InstallScope{
		Name:           "user",
		RootDir:        rootDir,
		StateFile:      ".nav-pilot-state.json",
		PathPrefix:     "",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}, nil
}

// SupportsType returns true if this scope supports the given artifact type.
func (s *InstallScope) SupportsType(itemType string) bool {
	for _, t := range s.SupportedTypes {
		if t == itemType {
			return true
		}
	}
	return false
}

// DstPath returns the full destination path for an artifact.
// For repo: <rootDir>/.github/agents/name.agent.md
// For user: <rootDir>/agents/name.agent.md
// For user instructions: <rootDir>/.github/instructions/name.instructions.md
//
//	(cplt requires .github/instructions/ inside COPILOT_CUSTOM_INSTRUCTIONS_DIRS)
func (s *InstallScope) DstPath(parts ...string) string {
	if s.PathPrefix != "" {
		return filepath.Join(append([]string{s.RootDir, s.PathPrefix}, parts...)...)
	}
	if s.needsGitHubPrefix(parts) {
		return filepath.Join(append([]string{s.RootDir, ".github"}, parts...)...)
	}
	return filepath.Join(append([]string{s.RootDir}, parts...)...)
}

// RelPath returns the relative path for state tracking.
// For repo: .github/agents/name.agent.md
// For user: agents/name.agent.md
// For user instructions: .github/instructions/name.instructions.md
func (s *InstallScope) RelPath(parts ...string) string {
	if s.PathPrefix != "" {
		return filepath.Join(append([]string{s.PathPrefix}, parts...)...)
	}
	if s.needsGitHubPrefix(parts) {
		return filepath.Join(append([]string{".github"}, parts...)...)
	}
	return filepath.Join(parts...)
}

// needsGitHubPrefix returns true when user-scope artifacts require a .github/ prefix.
// Instructions need this because COPILOT_CUSTOM_INSTRUCTIONS_DIRS expects
// .github/instructions/**/*.instructions.md inside the directory.
func (s *InstallScope) needsGitHubPrefix(parts []string) bool {
	return s.Name == "user" && len(parts) > 0 && parts[0] == "instructions"
}

// StatePath returns the full path to the state file.
func (s *InstallScope) StatePath() string {
	return filepath.Join(s.RootDir, s.StateFile)
}

// ValidateStatePath checks that a path from the state file is safe for this scope.
func (s *InstallScope) ValidateStatePath(p string) error {
	// Normalize to forward slashes so checks work on all platforms.
	p = filepath.ToSlash(p)

	if filepath.IsAbs(p) {
		return fmt.Errorf("absolute path not allowed: %s", p)
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("path traversal not allowed: %s", p)
	}

	if s.Name == "repo" {
		if !strings.HasPrefix(p, ".github/") {
			return fmt.Errorf("path outside .github/ not allowed in repo scope: %s", p)
		}
		return nil
	}

	// User scope: agents/, skills/, and .github/instructions/ allowed
	if !strings.HasPrefix(p, "agents/") && !strings.HasPrefix(p, "skills/") && !strings.HasPrefix(p, ".github/instructions/") {
		return fmt.Errorf("path outside agents/, skills/, or .github/instructions/ not allowed in user scope: %s", p)
	}
	return nil
}

// PathWithinRoot reports whether abs is still inside root once every symlink on
// the way to it has been resolved.
//
// Lstat on the final component is not containment: a source repo whose agents/
// directory is a symlink to /etc still Lstats agents/passwd as a regular file,
// and reading or copying it leaves the checkout. Every read of source content —
// the content resolver, and the agentpakke conformance checks — asks this first,
// so an intermediate symlink cannot hand nav-pilot a file from outside the
// source it thinks it is reading.
//
// Both sides are resolved, so a checkout that itself sits behind a symlink
// (macOS /tmp, /var) is compared like with like. A path that does not exist yet
// is resolved as far as its existing ancestors go and judged on those: the
// parents are what decide containment.
func PathWithinRoot(root, abs string) bool {
	rel, err := filepath.Rel(resolveSymlinks(root), resolveSymlinks(abs))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolveSymlinks resolves p, falling back to its longest existing ancestor so
// a not-yet-existing path is still judged by the parents that do exist.
func resolveSymlinks(p string) string {
	p = filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	return filepath.Join(resolveSymlinks(parent), filepath.Base(p))
}

// CleanupDirs removes empty artifact directories after uninstall.
func (s *InstallScope) CleanupDirs() {
	if s.Name == "repo" {
		for _, sub := range []string{"agents", "skills", "instructions", "prompts"} {
			dir := filepath.Join(s.RootDir, ".github", sub)
			entries, err := os.ReadDir(dir)
			if err == nil && len(entries) == 0 {
				os.Remove(dir)
			}
		}
		return
	}
	// User scope
	for _, sub := range []string{"agents", "skills"} {
		dir := filepath.Join(s.RootDir, sub)
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			os.Remove(dir)
		}
	}
	// Instructions live under .github/instructions/ in user scope
	instrDir := filepath.Join(s.RootDir, ".github", "instructions")
	if entries, err := os.ReadDir(instrDir); err == nil && len(entries) == 0 {
		os.Remove(instrDir)
		// Remove .github/ if now empty too
		if entries, err := os.ReadDir(filepath.Join(s.RootDir, ".github")); err == nil && len(entries) == 0 {
			os.Remove(filepath.Join(s.RootDir, ".github"))
		}
	}
}

// Label returns a display label for UI output.
func (s *InstallScope) Label() string {
	if s.Name == "user" {
		return "~/.copilot (user-wide)"
	}
	return s.RootDir
}

// IsUser returns true for user-scope installs.
func (s *InstallScope) IsUser() bool {
	return s.Name == "user"
}

// StateFile tracks what was installed, for safe updates and uninstall.
type StateFile struct {
	Collection  string          `json:"collection"`
	Version     string          `json:"version"`
	Scope       string          `json:"scope,omitempty"`       // "repo" or "user"; empty means "repo" (backwards compat)
	SourceRepo  string          `json:"source_repo,omitempty"` // git repository owner/name (e.g. "navikt/copilot")
	SourceSHA   string          `json:"source_sha"`
	InstalledAt string          `json:"installed_at"`
	Files       []InstalledFile `json:"files"`
	// Unknown carries every top-level key this binary does not understand, so a
	// read-modify-write does not silently drop what a newer nav-pilot wrote.
	// See [InstalledFile.Unknown].
	Unknown map[string]json.RawMessage `json:"-"`
}

// InstalledFile records a single installed file with its content hash.
type InstalledFile struct {
	Path   string `json:"path"`
	Hash   string `json:"hash"`
	Status string `json:"status,omitempty"` // "" = active, FileStatusIgnored = intentionally excluded, FileStatusConflict = exists with local modifications
	// Source names the agentpakke this file came from, when that is not the
	// scope's own SourceRepo. Empty means "this scope's source", which is what
	// every state file written before per-file origins says about all of its
	// files — so they keep syncing exactly as they did.
	Source string `json:"source,omitempty"`
	// Unknown carries every key of this entry that the running binary does not
	// understand, and writes it back out unchanged.
	//
	// The repo-scope state file is committed and shared across a team, so a
	// colleague on an older nav-pilot rewrites it on every sync, ignore or add.
	// Without this, the decoder drops what it does not know: that is how
	// per-file `source` (#571) went missing and the next sync deleted the file
	// as "gone upstream" (#588). Keeping unknown keys fixes that for every
	// field added after this binary was built, not just for `source`.
	Unknown map[string]json.RawMessage `json:"-"`
}

// The alias types below borrow the struct tags without the JSON methods, so the
// custom marshalling can lean on encoding/json for the known fields.
type (
	stateFileFields     StateFile
	installedFileFields InstalledFile
)

var (
	stateFileKnownKeys     = knownJSONKeys(StateFile{})
	installedFileKnownKeys = knownJSONKeys(InstalledFile{})
)

func (s *StateFile) UnmarshalJSON(b []byte) error {
	var known stateFileFields
	if err := json.Unmarshal(b, &known); err != nil {
		return err
	}
	unknown, err := unknownJSONKeys(b, stateFileKnownKeys)
	if err != nil {
		return err
	}
	*s = StateFile(known)
	s.Unknown = unknown
	return nil
}

func (s StateFile) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(stateFileFields(s))
	if err != nil {
		return nil, err
	}
	return appendUnknownKeys(b, s.Unknown), nil
}

func (f *InstalledFile) UnmarshalJSON(b []byte) error {
	var known installedFileFields
	if err := json.Unmarshal(b, &known); err != nil {
		return err
	}
	unknown, err := unknownJSONKeys(b, installedFileKnownKeys)
	if err != nil {
		return err
	}
	*f = InstalledFile(known)
	f.Unknown = unknown
	return nil
}

func (f InstalledFile) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(installedFileFields(f))
	if err != nil {
		return nil, err
	}
	return appendUnknownKeys(b, f.Unknown), nil
}

// knownJSONKeys is the set of JSON names a struct type declares.
func knownJSONKeys(v any) map[string]bool {
	t := reflect.TypeOf(v)
	keys := make(map[string]bool, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		// An empty tag name is not "no key": encoding/json still matches the
		// field by its Go name, so leaving it out here would file a known
		// field under the unknown keys and duplicate it on the way out.
		if name == "" {
			name = f.Name
		}
		keys[name] = true
	}
	return keys
}

// unknownJSONKeys returns the members of a JSON object that known does not name.
func unknownJSONKeys(b []byte, known map[string]bool) (map[string]json.RawMessage, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, err
	}
	for k := range all {
		if known[k] {
			delete(all, k)
		}
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all, nil
}

// appendUnknownKeys puts the preserved keys back at the end of the object, in a
// fixed order: the same state must always serialise to the same bytes, or every
// checked-in state file picks up a spurious diff on the next sync.
func appendUnknownKeys(b []byte, unknown map[string]json.RawMessage) []byte {
	if len(unknown) == 0 {
		return b
	}
	out := b[:len(b)-1] // drop the closing brace
	for _, k := range slices.Sorted(maps.Keys(unknown)) {
		if len(out) > 1 {
			out = append(out, ',')
		}
		key, _ := json.Marshal(k)
		out = append(out, key...)
		out = append(out, ':')
		out = append(out, unknown[k]...)
	}
	return append(out, '}')
}

// PreserveUnknownFrom carries the unknown keys of the state being replaced over
// to s: the top-level ones, and the per-file ones for every path still tracked.
//
// The read-modify-write paths (sync, add, ignore) get this for free from the
// struct copy. Every path that builds a fresh StateFile over an existing one —
// install, pinRevision, the opencode sync — has to ask for it, or it drops
// exactly the keys #588 is about.
func (s *StateFile) PreserveUnknownFrom(prior *StateFile) {
	if s == nil || prior == nil {
		return
	}
	if s.Unknown == nil {
		s.Unknown = prior.Unknown
	}
	perFile := make(map[string]map[string]json.RawMessage, len(prior.Files))
	for _, f := range prior.Files {
		if len(f.Unknown) > 0 {
			perFile[f.Path] = f.Unknown
		}
	}
	for i, f := range s.Files {
		if f.Unknown == nil {
			s.Files[i].Unknown = perFile[f.Path]
		}
	}
}

// FileStatusIgnored marks a file as intentionally excluded by the user.
// Sync and status skip files with this status.
const FileStatusIgnored = "ignored"

// FileStatusConflict marks a file that existed with local modifications at install time.
// The user declined to overwrite it, so sync should not touch it until resolved.
const FileStatusConflict = "conflict"

var UseColor = true

func init() {
	if os.Getenv("NO_COLOR") != "" {
		UseColor = false
	}
}

func Color(code, msg string) string {
	if !UseColor {
		return msg
	}
	return fmt.Sprintf("\033[%sm%s\033[0m", code, msg)
}

func Red(msg string) string    { return Color("31", msg) }
func Green(msg string) string  { return Color("32", msg) }
func Yellow(msg string) string { return Color("33", msg) }
func Dim(msg string) string    { return Color("2", msg) }
func Bold(msg string) string   { return Color("1", msg) }
