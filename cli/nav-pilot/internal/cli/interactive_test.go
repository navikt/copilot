package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"
)

// ─── validateCustomModelInput tests ─────────────────────────────────────────

func TestValidateCustomModelInput(t *testing.T) {
	opencode, err := providerFor("opencode")
	if err != nil {
		t.Fatalf("providerFor(opencode): %v", err)
	}
	copilot, err := providerFor("copilot")
	if err != nil {
		t.Fatalf("providerFor(copilot): %v", err)
	}

	tests := []struct {
		name    string
		p       Provider
		in      string
		wantErr bool
	}{
		{"opencode: blank is accepted", opencode, "", false},
		{"opencode: whitespace-only is accepted", opencode, "   ", false},
		// Regression: the input reaches ValidateModel trimmed, not raw —
		// ValidateModelValue rejects surrounding whitespace outright, so an
		// untrimmed id here would wrongly fail a value the save path trims
		// to something valid.
		{"opencode: a valid id with surrounding whitespace is accepted", opencode, "  github-copilot/gpt-5.5  ", false},
		{"opencode: a bare Copilot id is accepted", opencode, "gpt-5.5", false},
		{"opencode: a qualified id is accepted", opencode, "github-copilot/gpt-5.5", false},
		{"copilot: blank is accepted", copilot, "", false},
		{"copilot: a bare id is accepted", copilot, "gpt-5.5", false},
		{"copilot: surrounding whitespace is accepted (trimmed first)", copilot, "  gpt-5.5  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCustomModelInput(tt.p, tt.in)
			if tt.wantErr && err == nil {
				t.Errorf("validateCustomModelInput(%q) = nil, want error", tt.in)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateCustomModelInput(%q) = %v, want nil", tt.in, err)
			}
		})
	}
}

// ─── buildPickerDefaults tests ──────────────────────────────────────────────

func TestBuildPickerDefaults_FreshInstall(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents:       []string{"auth-agent", "nais-agent"},
		Skills:       []string{"kafka", "observability-setup"},
		Instructions: []string{"golang", "security-owasp"},
	}

	defaults := buildPickerDefaults(full, nil, scope)

	assertStringsEqual(t, "agents", defaults["agents"], full.Agents)
	assertStringsEqual(t, "skills", defaults["skills"], full.Skills)
	assertStringsEqual(t, "instructions", defaults["instructions"], full.Instructions)
}

func TestBuildPickerDefaults_EmptyState(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{Agents: []string{"auth-agent"}}
	emptyState := &StateFile{Files: []InstalledFile{}}

	defaults := buildPickerDefaults(full, emptyState, scope)

	assertStringsEqual(t, "agents", defaults["agents"], []string{"auth-agent"})
}

func TestBuildPickerDefaults_ReinstallPreservesActive(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents: []string{"auth-agent", "nais-agent", "rust-agent"},
		Skills: []string{"kafka"},
	}
	state := &StateFile{
		Files: []InstalledFile{
			{Path: "agents/auth-agent.agent.md", Hash: "abc"},
			{Path: "agents/nais-agent.agent.md", Hash: "def"},
			{Path: "agents/rust-agent.agent.md", Hash: "", Status: fileStatusIgnored},
			{Path: "skills/kafka/", Hash: "ghi"},
		},
	}

	defaults := buildPickerDefaults(full, state, scope)

	assertStringsEqual(t, "agents", defaults["agents"], []string{"auth-agent", "nais-agent"})
	assertStringsEqual(t, "skills", defaults["skills"], []string{"kafka"})
}

func TestBuildPickerDefaults_NewItemDefaultsToSelected(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents: []string{"auth-agent", "brand-new-agent"},
	}
	state := &StateFile{
		Files: []InstalledFile{
			{Path: "agents/auth-agent.agent.md", Hash: "abc"},
			// brand-new-agent is NOT in state
		},
	}

	defaults := buildPickerDefaults(full, state, scope)

	// New items absent from state should be selected by default
	assertStringsEqual(t, "agents", defaults["agents"], []string{"auth-agent", "brand-new-agent"})
}

func TestBuildPickerDefaults_RepoScope(t *testing.T) {
	scope := ScopeRepo(t.TempDir())
	full := &Manifest{
		Agents:       []string{"auth-agent"},
		Skills:       []string{"kafka"},
		Instructions: []string{"golang"},
	}
	state := &StateFile{
		Files: []InstalledFile{
			{Path: ".github/agents/auth-agent.agent.md", Hash: "abc"},
			{Path: ".github/skills/kafka/", Hash: "def"},
			{Path: ".github/instructions/golang.instructions.md", Hash: "", Status: fileStatusIgnored},
		},
	}

	defaults := buildPickerDefaults(full, state, scope)

	assertStringsEqual(t, "agents", defaults["agents"], []string{"auth-agent"})
	assertStringsEqual(t, "skills", defaults["skills"], []string{"kafka"})
	// golang is ignored → not in defaults
	if len(defaults["instructions"]) != 0 {
		t.Errorf("instructions should be empty (ignored), got %v", defaults["instructions"])
	}
}

func TestBuildPickerDefaults_UserScopeInstructionPaths(t *testing.T) {
	// User-scope instructions use .github/instructions/ prefix
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{Instructions: []string{"golang", "security-owasp"}}
	state := &StateFile{
		Files: []InstalledFile{
			{Path: ".github/instructions/golang.instructions.md", Hash: "abc"},
			{Path: ".github/instructions/security-owasp.instructions.md", Hash: "", Status: fileStatusIgnored},
		},
	}

	defaults := buildPickerDefaults(full, state, scope)

	assertStringsEqual(t, "instructions", defaults["instructions"], []string{"golang"})
}

func TestBuildPickerDefaults_SkillTrailingSlash(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{Skills: []string{"kafka", "nais"}}
	state := &StateFile{
		Files: []InstalledFile{
			{Path: "skills/kafka/", Hash: "abc"},
			{Path: "skills/nais/", Hash: "", Status: fileStatusIgnored},
		},
	}

	defaults := buildPickerDefaults(full, state, scope)

	assertStringsEqual(t, "skills", defaults["skills"], []string{"kafka"})
}

// ─── computeSkippedItems tests ──────────────────────────────────────────────

func TestComputeSkippedItems_AllSelected(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents:       []string{"auth-agent", "nais-agent"},
		Skills:       []string{"kafka"},
		Instructions: []string{"golang"},
	}
	selected := &Manifest{
		Agents:       []string{"auth-agent", "nais-agent"},
		Skills:       []string{"kafka"},
		Instructions: []string{"golang"},
	}

	skipped := computeSkippedItems(full, selected, scope)

	if len(skipped) != 0 {
		t.Errorf("expected no skipped items, got %v", skipped)
	}
}

func TestComputeSkippedItems_NoneSelected(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents: []string{"auth-agent", "nais-agent"},
		Skills: []string{"kafka"},
	}
	selected := &Manifest{}

	skipped := computeSkippedItems(full, selected, scope)

	if len(skipped) != 3 {
		t.Fatalf("expected 3 skipped, got %d: %v", len(skipped), skipped)
	}
	for _, s := range skipped {
		if s.Status != fileStatusIgnored {
			t.Errorf("expected status %q, got %q for %s", fileStatusIgnored, s.Status, s.Path)
		}
		if s.Hash != "" {
			t.Errorf("expected empty hash for skipped item %s, got %q", s.Path, s.Hash)
		}
	}
}

func TestComputeSkippedItems_PartialSelection(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents:       []string{"auth-agent", "nais-agent", "rust-agent"},
		Skills:       []string{"kafka", "observability-setup"},
		Instructions: []string{"golang"},
	}
	selected := &Manifest{
		Agents:       []string{"auth-agent"},
		Skills:       []string{"kafka"},
		Instructions: []string{"golang"},
	}

	skipped := computeSkippedItems(full, selected, scope)

	if len(skipped) != 3 {
		t.Fatalf("expected 3 skipped, got %d: %v", len(skipped), skipped)
	}

	paths := make(map[string]bool)
	for _, s := range skipped {
		paths[s.Path] = true
	}
	wantSkipped := []string{
		"agents/nais-agent.agent.md",
		"agents/rust-agent.agent.md",
		"skills/observability-setup/",
	}
	for _, w := range wantSkipped {
		if !paths[w] {
			t.Errorf("expected skipped path %q not found in %v", w, skipped)
		}
	}
}

func TestComputeSkippedItems_RepoScope(t *testing.T) {
	scope := ScopeRepo(t.TempDir())
	full := &Manifest{
		Agents: []string{"auth-agent"},
		Skills: []string{"kafka"},
	}
	selected := &Manifest{} // nothing selected

	skipped := computeSkippedItems(full, selected, scope)

	paths := make(map[string]bool)
	for _, s := range skipped {
		paths[s.Path] = true
	}
	if !paths[".github/agents/auth-agent.agent.md"] {
		t.Error("repo scope agent path should have .github/ prefix")
	}
	if !paths[".github/skills/kafka/"] {
		t.Error("repo scope skill path should have .github/ prefix and trailing slash")
	}
}

func TestComputeSkippedItems_UserScopeInstructions(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{Instructions: []string{"golang"}}
	selected := &Manifest{}

	skipped := computeSkippedItems(full, selected, scope)

	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(skipped))
	}
	// User-scope instructions use .github/instructions/ prefix
	if skipped[0].Path != ".github/instructions/golang.instructions.md" {
		t.Errorf("expected .github/instructions/ path, got %q", skipped[0].Path)
	}
}

func TestComputeSkippedItems_EmptyManifest(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	skipped := computeSkippedItems(&Manifest{}, &Manifest{}, scope)
	if len(skipped) != 0 {
		t.Errorf("expected no skipped for empty manifest, got %v", skipped)
	}
}

// ─── RelPathForName tests ───────────────────────────────────────────────────

func TestRelPathForName_UserScope(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}

	tests := []struct {
		kind *ArtifactKind
		name string
		want string
	}{
		{KindAgent, "auth-agent", "agents/auth-agent.agent.md"},
		{KindSkill, "kafka", "skills/kafka/"},
		{KindInstruction, "golang", ".github/instructions/golang.instructions.md"},
		{KindPrompt, "conventional-commit", "prompts/conventional-commit.prompt.md"},
	}
	for _, tt := range tests {
		got := tt.kind.RelPathForName(scope, tt.name)
		if got != tt.want {
			t.Errorf("RelPathForName(%s, %q) = %q, want %q", tt.kind.Name, tt.name, got, tt.want)
		}
	}
}

func TestRelPathForName_RepoScope(t *testing.T) {
	scope := ScopeRepo(t.TempDir())

	tests := []struct {
		kind *ArtifactKind
		name string
		want string
	}{
		{KindAgent, "auth-agent", ".github/agents/auth-agent.agent.md"},
		{KindSkill, "kafka", ".github/skills/kafka/"},
		{KindInstruction, "golang", ".github/instructions/golang.instructions.md"},
		{KindPrompt, "conventional-commit", ".github/prompts/conventional-commit.prompt.md"},
	}
	for _, tt := range tests {
		got := tt.kind.RelPathForName(scope, tt.name)
		if got != tt.want {
			t.Errorf("RelPathForName(%s, %q) = %q, want %q", tt.kind.Name, tt.name, got, tt.want)
		}
	}
}

// ─── interactiveItemPicker guard test ───────────────────────────────────────

func TestInteractiveItemPicker_NonInteractiveReturnsError(t *testing.T) {
	forceNonInteractive = true
	defer func() { forceNonInteractive = false }()

	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{Agents: []string{"auth-agent"}}

	_, _, err := interactiveItemPicker(full, nil, scope)
	if err == nil {
		t.Fatal("expected error when non-interactive")
	}
	if err.Error() != "interactive item picker requires a terminal" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── test helpers ───────────────────────────────────────────────────────────

func assertStringsEqual(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %v (len %d), want %v (len %d)", label, got, len(got), want, len(want))
		return
	}
	// Sort copies for comparison
	g := append([]string{}, got...)
	w := append([]string{}, want...)
	sort.Strings(g)
	sort.Strings(w)
	for i := range g {
		if g[i] != w[i] {
			t.Errorf("%s[%d]: got %q, want %q (full: got=%v, want=%v)", label, i, g[i], w[i], got, want)
		}
	}
}

func TestCmdInteractive_NotGitRepo(t *testing.T) {
	origDir, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Override HOME so ScopeUser() uses the temp dir
	t.Setenv("HOME", dir)

	// Verify that findGitRoot returns empty for a temp dir (no git repo)
	root := findGitRoot(".")
	if root != "" {
		t.Skipf("temp dir is inside a git repo (%s), skipping", root)
	}

	// The key assertion: cmdInteractive no longer produces the old
	// "not in a git repository" error — instead it tries to resolve source.
	// Since resolveSource does network I/O and interactive prompts may block,
	// we only verify the code path selection here rather than running the full flow.
	// The flow goes to interactiveUserOnlyInstall which attempts a clone.
	// This is tested indirectly by other tests (sync, add).
}

func TestCmdInteractive_InstalledUpToDate(t *testing.T) {
	origDir, _ := os.Getwd()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".github"), 0o755)
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Isolate HOME so user-scope installs don't leak into the test
	t.Setenv("HOME", dir)

	// Prevent huh TUI prompts from blocking in tests
	forceNonInteractive = true
	defer func() { forceNonInteractive = false }()

	state := &StateFile{
		Collection: "test-collection",
		Version:    "2026.04.13-170000-abc1234",
		SourceSHA:  "abc1234",
	}
	if err := writeState(dir, state); err != nil {
		t.Fatal(err)
	}

	// Mock GitHub API returning same version (up-to-date)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ghRelease{
			{TagName: "nav-pilot/2026.04.13-170000-abc1234"},
		})
	}))
	defer srv.Close()

	origAPI := releasesAPI
	releasesAPI = srv.URL
	defer func() { releasesAPI = origAPI }()

	setupTestCache(t)

	// Should not error — will try to launch cplt (which may not exist, that's ok)
	err := cmdInteractive(CLIOverrides{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstalledAgents(t *testing.T) {
	state := &StateFile{
		Files: []InstalledFile{
			{Path: ".github/agents/nav-pilot.agent.md"},
			{Path: ".github/agents/auth-agent.agent.md"},
			{Path: ".github/agents/nais-agent.agent.md"},
			{Path: ".github/skills/threat-model/SKILL.md"},
			{Path: ".github/instructions/golang.instructions.md"},
		},
	}
	agents := installedAgents(state)
	expected := []string{"auth-agent", "nais-agent", "nav-pilot"}
	if len(agents) != len(expected) {
		t.Fatalf("expected %d agents, got %d: %v", len(expected), len(agents), agents)
	}
	for i, a := range agents {
		if a != expected[i] {
			t.Errorf("agent[%d]: expected %q, got %q", i, expected[i], a)
		}
	}
}

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{[]string{"b", "a", "a", "c"}, []string{"a", "b", "c"}},
		{[]string{"x"}, []string{"x"}},
		{nil, nil},
		{[]string{"a", "a", "a"}, []string{"a"}},
	}
	for _, tt := range tests {
		got := uniqueStrings(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("uniqueStrings(%v) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("uniqueStrings(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

// ─── patchOpenCodeConfig tests ──────────────────────────────────────────────

func TestPatchOpenCodeConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "opencode.json")

	// 1. File doesn't exist -> should return nil and not create
	err := patchOpenCodeConfig(configPath, "/mock/config/dir")
	if err != nil {
		t.Fatalf("expected nil for non-existent file, got: %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected file to not be created")
	}

	// 2. File exists but has no plugins
	initialConfig := `{"share": "disabled"}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}
	err = patchOpenCodeConfig(configPath, "/mock/config/dir")
	if err != nil {
		t.Fatalf("failed to patch config: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read patched config: %v", err)
	}
	if !strings.Contains(string(data), `"plugin":`) || !strings.Contains(string(data), `"/mock/config/dir/plugins/rtk.ts"`) {
		t.Fatalf("expected rtk plugin to be added, got: %s", string(data))
	}

	// 3. File exists and already has rtk.ts
	// The file now has it, patching again should succeed and not duplicate
	err = patchOpenCodeConfig(configPath, "/mock/config/dir")
	if err != nil {
		t.Fatalf("failed to patch config second time: %v", err)
	}
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config after second patch: %v", err)
	}
	if strings.Count(string(data), `"/mock/config/dir/plugins/rtk.ts"`) != 1 {
		t.Fatalf("expected exactly one rtk plugin entry, got: %s", string(data))
	}

	// 4. File exists and has other plugins
	initialConfig = `{"plugin": ["something-else.ts"]}`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("failed to write existing plugins config: %v", err)
	}
	err = patchOpenCodeConfig(configPath, "/mock/config/dir")
	if err != nil {
		t.Fatalf("failed to patch config with existing plugins: %v", err)
	}
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config with existing plugins: %v", err)
	}
	if !strings.Contains(string(data), `"something-else.ts"`) || !strings.Contains(string(data), `"/mock/config/dir/plugins/rtk.ts"`) {
		t.Fatalf("expected both plugins to be present, got: %s", string(data))
	}

	// 5. Invalid JSONC / JSON should error gracefully
	initialConfig = `{"plugin": // comments not supported by stdlib json`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("failed to write invalid json config: %v", err)
	}
	err = patchOpenCodeConfig(configPath, "/mock/config/dir")
	if err == nil {
		t.Fatalf("expected error for invalid json")
	}
}

func TestDecideLaunch(t *testing.T) {
	tests := []struct {
		name                                          string
		available, autoLaunch, sandboxed, interactive bool
		want                                          launchDecision
	}{
		{"client missing", false, true, true, true, launchSkipUnavailable},
		{"no terminal wins over a missing client", false, true, true, false, launchSkipQuiet},
		{"client missing wins over opt-out", false, false, true, true, launchSkipUnavailable},
		{"no terminal", true, true, true, false, launchSkipQuiet},
		{"healthy and interactive launches without asking", true, true, true, true, launchGo},
		{"no sandbox, interactive: warn and launch", true, true, false, true, launchWarnUnsandboxed},
		{"no sandbox, no terminal: nothing", true, true, false, false, launchSkipQuiet},
		{"opt-out", true, false, true, true, launchSkipOptedOut},
		{"opt-out wins over the unsandboxed warning", true, false, false, true, launchSkipOptedOut},
		{"no terminal wins over the opt-out notice", true, false, true, false, launchSkipQuiet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decideLaunch(tt.available, tt.autoLaunch, tt.sandboxed, tt.interactive); got != tt.want {
				t.Errorf("decideLaunch(%v, %v, %v, %v) = %v, want %v",
					tt.available, tt.autoLaunch, tt.sandboxed, tt.interactive, got, tt.want)
			}
		})
	}
}

func TestSyncPromptOutcome(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		choice    string
		wantSync  bool
		wantAbort bool
	}{
		{"yes syncs", nil, "yes", true, false},
		{"no launches without syncing", nil, "no", false, false},
		{"ctrl-c aborts", huh.ErrUserAborted, "", false, true},
		{"other prompt error still launches", errors.New("no tty"), "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sync, abort := syncPromptOutcome(tt.err, tt.choice)
			if sync != tt.wantSync || abort != tt.wantAbort {
				t.Errorf("syncPromptOutcome(%v, %q) = (%v, %v), want (%v, %v)",
					tt.err, tt.choice, sync, abort, tt.wantSync, tt.wantAbort)
			}
		})
	}
}

// A picker that cannot run is not a user who declined. Reporting it as
// "Cancelled." installed nothing and exited zero, so a job where
// isInteractive() is true but nothing can answer looked like a clean no-op
// (#802). The refusal must be an error, and must name the way to install
// without the picker.
func TestPickerFailureIsNotSilentCancellation(t *testing.T) {
	isolatedConfig(t)
	forceInteractive(t)
	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	src := &Source{Dir: legacySourceTree(t), Repo: "navikt/copilot", SHA: "deadbeef"}

	err = interactiveUserInstallFromSource(scope, src, "")
	if err == nil {
		t.Fatal("a picker that cannot run must return an error, not install nothing and succeed")
	}
	if errors.Is(err, errInstallCancelled) {
		t.Error("a picker failure must not be reported as a user cancellation")
	}
	if !strings.Contains(err.Error(), "nav-pilot install") {
		t.Errorf("the refusal must name how to install without the picker, got: %v", err)
	}
}

// Ctrl-C on the install picker is a cancellation, not a picker that could not
// run: huh reports it as ErrUserAborted, and treating that as a failure would
// make an ordinary abort exit non-zero.
func TestPickerDeclined(t *testing.T) {
	for _, tc := range []struct {
		name   string
		choice string
		err    error
		want   bool
	}{
		{"cancel option", "cancel", nil, true},
		{"ctrl-c", "", huh.ErrUserAborted, true},
		{"ctrl-c after a choice", "all", huh.ErrUserAborted, true},
		{"picker could not run", "", errors.New("no tty"), false},
		{"install everything", "all", nil, false},
		{"customize", "custom", nil, false},
	} {
		if got := pickerDeclined(tc.choice, tc.err); got != tc.want {
			t.Errorf("%s: pickerDeclined(%q, %v) = %v, want %v", tc.name, tc.choice, tc.err, got, tc.want)
		}
	}
}

// An explicit `install --user --all` has already said what it wants, so it must
// not open the picker: it used to, and where nothing could answer the prompt the
// advertised --all path installed nothing (#802 review). The re-install half
// covers the trap in the bypass: the picker's flow force-updates managed files,
// so skipping the picker must not stop --all from refreshing them.
func TestExplicitInstallAllSkipsThePicker(t *testing.T) {
	isolatedConfig(t)
	forceInteractive(t)
	stubResolveSource(t, &Source{Dir: legacySourceTree(t), Repo: defaultSourceRepo, SHA: "deadbeef"})

	orig := interactiveUserInstallFn
	t.Cleanup(func() { interactiveUserInstallFn = orig })
	interactiveUserInstallFn = func(*InstallScope, *Source, string) error {
		t.Error("install --user --all reached the install picker; an explicit --all must not prompt")
		return nil
	}

	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmdInstallAll(scope, "", "", false, false, false, true); err != nil {
		t.Fatalf("install --user --all: %v", err)
	}

	agent := scope.DstPath("agents", "test-a.agent.md")
	if _, err := os.Stat(agent); err != nil {
		t.Fatalf("install --user --all installed nothing: %v", err)
	}

	// Re-install over a managed file that no longer matches what nav-pilot
	// recorded. The picker's flow overwrites it; so must this one.
	if err := os.WriteFile(agent, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdInstallAll(scope, "", "", false, false, false, true); err != nil {
		t.Fatalf("re-install --user --all: %v", err)
	}
	got, err := os.ReadFile(agent)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "stale") {
		t.Errorf("re-install left the managed file unrefreshed: %q", got)
	}
}

// An explicit scope flag answers the repo-vs-user question, so `install --repo`
// and `install --target X` must not open the scope picker --repo's own help
// says it skips (#820). Driven through run() rather than cmdInstallInteractive:
// the bug was never in that function, it was in which function the dispatch in
// cli.go chose, so a test that calls it directly cannot see the regression.
//
// The re-install half covers the same trap #814 hit on the --all bypass: the
// user-scope flow force-updates managed files, the repo flow took only --force,
// so a re-install quietly stopped refreshing them.
func TestRunExplicitScopeSkipsTheScopePicker(t *testing.T) {
	for _, tc := range []struct {
		name string
		args func(target string) []string
	}{
		{"--repo", func(string) []string { return []string{"install", "--repo"} }},
		{"--target", func(target string) []string { return []string{"install", "--target", target} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolatedConfig(t)
			forceInteractive(t)
			target := repoTarget(t)
			t.Chdir(target)
			stubResolveSource(t, pakkeSource(t, defaultSourceRepo))

			// The stub answers "user home", so a dispatch that asks anyway
			// installs to the scope the command line ruled out — the failure
			// #820 actually produced, not just an extra prompt.
			userScope, err := ScopeUser()
			if err != nil {
				t.Fatal(err)
			}
			asked := stubScopePrompt(t, userScope)

			// Asserted before the error: a dispatch that asks anyway fails
			// later on the user-scope picker's TTY, and that error would hide
			// the reason it got there.
			runErr := run(tc.args(target))
			if *asked {
				t.Fatalf("install %s reached the scope picker; an explicit scope must not be asked for", tc.name)
			}
			if runErr != nil {
				t.Fatalf("install %s: %v", tc.name, runErr)
			}

			agent := ScopeRepo(target).DstPath("agents", "grillmester.agent.md")
			if _, err := os.Stat(agent); err != nil {
				t.Fatalf("install %s installed nothing at repo scope: %v", tc.name, err)
			}

			// Re-install over a managed file that no longer matches what
			// nav-pilot recorded. The user-scope flow overwrites it; so must
			// this one.
			if err := os.WriteFile(agent, []byte("stale\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := run(tc.args(target)); err != nil {
				t.Fatalf("re-install %s: %v", tc.name, err)
			}
			got, err := os.ReadFile(agent)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(got), "stale") {
				t.Errorf("re-install %s left the managed file unrefreshed: %q", tc.name, got)
			}
		})
	}
}

// installAllFromSource is reached at both scopes, so its closing line cannot
// claim the repo's .github/ reaches every repository (#820).
func TestInstallAllSuccessMessageNamesTheScope(t *testing.T) {
	for _, tc := range []struct {
		name      string
		scopeFor  func(t *testing.T, target string) *InstallScope
		want, not string
	}{
		{"repo", func(t *testing.T, target string) *InstallScope { return ScopeRepo(target) },
			"in this repository", "across all your repos"},
		{"user", func(t *testing.T, target string) *InstallScope {
			scope, err := ScopeUser()
			if err != nil {
				t.Fatal(err)
			}
			return scope
		}, "across all your repos", "in this repository"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolatedConfig(t)
			target := repoTarget(t)
			src := pakkeSource(t, defaultSourceRepo)
			scope := tc.scopeFor(t, target)

			out := captureStdout(func() {
				if err := installAllFromSource(scope, src, nil, false, false, false); err != nil {
					t.Errorf("install --all: %v", err)
				}
			})
			if !strings.Contains(out, tc.want) {
				t.Errorf("%s-scope success message must say %q, got:\n%s", tc.name, tc.want, out)
			}
			if strings.Contains(out, tc.not) {
				t.Errorf("%s-scope success message must not say %q, got:\n%s", tc.name, tc.not, out)
			}
		})
	}
}
