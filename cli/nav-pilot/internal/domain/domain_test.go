package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// --- ValidateModelValue ---

func TestValidateModelValue(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "claude-opus-4.8", false},
		{"valid with slash", "anthropic/claude-3-5-sonnet", false},
		{"valid auto", "auto", false},
		{"empty", "", true},
		{"leading whitespace", " gpt-4", true},
		{"trailing whitespace", "gpt-4 ", true},
		{"invalid char exclamation", "gpt!4", true},
		{"invalid char at", "gpt@4", true},
		{"starts with dot", ".gpt4", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateModelValue(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateModelValue(%q) = nil, want error", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateModelValue(%q) = %v, want nil", tt.input, err)
			}
		})
	}
}

// --- ValidateOptionalModel ---

func TestValidateOptionalModel(t *testing.T) {
	if err := ValidateOptionalModel(""); err != nil {
		t.Errorf("ValidateOptionalModel(empty) = %v, want nil", err)
	}
	if err := ValidateOptionalModel("   "); err != nil {
		t.Errorf("ValidateOptionalModel(blank) = %v, want nil", err)
	}
	if err := ValidateOptionalModel("claude-opus-4.8"); err != nil {
		t.Errorf("ValidateOptionalModel(valid) = %v, want nil", err)
	}
	if err := ValidateOptionalModel("bad model!"); err == nil {
		t.Error("ValidateOptionalModel(invalid) = nil, want error")
	}
}

// --- ContainsStr ---

func TestContainsStr(t *testing.T) {
	list := []string{"a", "b", "c"}
	if !ContainsStr(list, "b") {
		t.Error("ContainsStr(found) = false, want true")
	}
	if ContainsStr(list, "z") {
		t.Error("ContainsStr(missing) = true, want false")
	}
	if ContainsStr(nil, "a") {
		t.Error("ContainsStr(nil, a) = true, want false")
	}
}

// --- ScopeRepo ---

func TestScopeRepo(t *testing.T) {
	s := ScopeRepo("/some/repo")
	if s.Name != "repo" {
		t.Errorf("Name = %q, want repo", s.Name)
	}
	if s.RootDir != "/some/repo" {
		t.Errorf("RootDir = %q, want /some/repo", s.RootDir)
	}
	if s.PathPrefix != ".github/" {
		t.Errorf("PathPrefix = %q, want .github/", s.PathPrefix)
	}
	if !s.SupportsType("agent") {
		t.Error("SupportsType(agent) = false, want true")
	}
	if !s.SupportsType("prompt") {
		t.Error("SupportsType(prompt) = false, want true")
	}
	if s.SupportsType("unknown") {
		t.Error("SupportsType(unknown) = true, want false")
	}
}

// --- ScopeUser ---

func TestScopeUser(t *testing.T) {
	s, err := ScopeUser()
	if err != nil {
		t.Fatalf("ScopeUser() error = %v, want nil", err)
	}
	if s.Name != "user" {
		t.Errorf("Name = %q, want user", s.Name)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".copilot")
	if s.RootDir != want {
		t.Errorf("RootDir = %q, want %q", s.RootDir, want)
	}
	if s.PathPrefix != "" {
		t.Errorf("PathPrefix = %q, want empty", s.PathPrefix)
	}
	if s.SupportsType("prompt") {
		t.Error("SupportsType(prompt) for user = true, want false")
	}
	if !s.SupportsType("instruction") {
		t.Error("SupportsType(instruction) for user = false, want true")
	}
}

// --- SupportsType ---

func TestSupportsType(t *testing.T) {
	s := ScopeRepo("/r")
	for _, typ := range []string{"agent", "skill", "instruction", "prompt"} {
		if !s.SupportsType(typ) {
			t.Errorf("ScopeRepo.SupportsType(%q) = false, want true", typ)
		}
	}
}

// --- DstPath and RelPath ---

func TestDstPathAndRelPath_Repo(t *testing.T) {
	s := ScopeRepo("/repo")
	// repo scope always prepends .github/
	dst := s.DstPath("agents", "nav-pilot.agent.md")
	wantDst := filepath.Join("/repo", ".github/", "agents", "nav-pilot.agent.md")
	if dst != wantDst {
		t.Errorf("DstPath = %q, want %q", dst, wantDst)
	}

	rel := s.RelPath("agents", "nav-pilot.agent.md")
	wantRel := filepath.Join(".github/", "agents", "nav-pilot.agent.md")
	if rel != wantRel {
		t.Errorf("RelPath = %q, want %q", rel, wantRel)
	}
}

func TestDstPathAndRelPath_UserAgents(t *testing.T) {
	home, _ := os.UserHomeDir()
	s := &InstallScope{
		Name:    "user",
		RootDir: filepath.Join(home, ".copilot"),
	}
	// user agents do NOT need .github/ prefix
	dst := s.DstPath("agents", "nav-pilot.agent.md")
	wantDst := filepath.Join(s.RootDir, "agents", "nav-pilot.agent.md")
	if dst != wantDst {
		t.Errorf("DstPath(agents) = %q, want %q", dst, wantDst)
	}
	rel := s.RelPath("agents", "nav-pilot.agent.md")
	if rel != filepath.Join("agents", "nav-pilot.agent.md") {
		t.Errorf("RelPath(agents) = %q", rel)
	}
}

func TestDstPathAndRelPath_UserInstructions(t *testing.T) {
	home, _ := os.UserHomeDir()
	s := &InstallScope{
		Name:    "user",
		RootDir: filepath.Join(home, ".copilot"),
	}
	// instructions need .github/ in user scope
	dst := s.DstPath("instructions", "foo.instructions.md")
	wantDst := filepath.Join(s.RootDir, ".github", "instructions", "foo.instructions.md")
	if dst != wantDst {
		t.Errorf("DstPath(instructions) = %q, want %q", dst, wantDst)
	}
	rel := s.RelPath("instructions", "foo.instructions.md")
	wantRel := filepath.Join(".github", "instructions", "foo.instructions.md")
	if rel != wantRel {
		t.Errorf("RelPath(instructions) = %q, want %q", rel, wantRel)
	}
}

// --- StatePath ---

func TestStatePath(t *testing.T) {
	s := ScopeRepo("/repo")
	want := filepath.Join("/repo", ".github/.nav-pilot-state.json")
	if got := s.StatePath(); got != want {
		t.Errorf("StatePath() = %q, want %q", got, want)
	}
}

// --- ValidateStatePath ---

func TestValidateStatePath_Repo(t *testing.T) {
	s := ScopeRepo("/repo")
	tests := []struct {
		path    string
		wantErr bool
	}{
		{".github/agents/foo.agent.md", false},
		{".github/instructions/foo.instructions.md", false},
		{"agents/foo.agent.md", true},   // outside .github/
		{"/etc/passwd", true},           // absolute
		{"../etc/passwd", true},         // traversal
		{".github/../etc/passwd", true}, // traversal
	}
	for _, tt := range tests {
		err := s.ValidateStatePath(tt.path)
		if tt.wantErr && err == nil {
			t.Errorf("ValidateStatePath(%q) = nil, want error", tt.path)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ValidateStatePath(%q) = %v, want nil", tt.path, err)
		}
	}
}

func TestValidateStatePath_User(t *testing.T) {
	home, _ := os.UserHomeDir()
	s := &InstallScope{Name: "user", RootDir: filepath.Join(home, ".copilot")}
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"agents/foo.agent.md", false},
		{"skills/foo.skill.md", false},
		{".github/instructions/foo.instructions.md", false},
		{"prompts/foo.prompt.md", true}, // not allowed in user scope
		{"/absolute/path", true},        // absolute
		{"../escape", true},             // traversal
	}
	for _, tt := range tests {
		err := s.ValidateStatePath(tt.path)
		if tt.wantErr && err == nil {
			t.Errorf("ValidateStatePath(%q) user = nil, want error", tt.path)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ValidateStatePath(%q) user = %v, want nil", tt.path, err)
		}
	}
}

// --- CleanupDirs ---

func TestCleanupDirs_Repo_RemovesEmptyDirs(t *testing.T) {
	tmp := t.TempDir()
	s := ScopeRepo(tmp)

	// create empty agents dir
	agentsDir := filepath.Join(tmp, ".github", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// non-empty instructions dir
	instrDir := filepath.Join(tmp, ".github", "instructions")
	if err := os.MkdirAll(instrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instrDir, "keep.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s.CleanupDirs()

	if _, err := os.Stat(agentsDir); !os.IsNotExist(err) {
		t.Error("empty agents dir should have been removed")
	}
	if _, err := os.Stat(instrDir); err != nil {
		t.Error("non-empty instructions dir should remain")
	}
}

func TestCleanupDirs_User_RemovesEmptyDirs(t *testing.T) {
	tmp := t.TempDir()
	s := &InstallScope{Name: "user", RootDir: tmp}

	agentsDir := filepath.Join(tmp, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	instrDir := filepath.Join(tmp, ".github", "instructions")
	if err := os.MkdirAll(instrDir, 0o755); err != nil {
		t.Fatal(err)
	}

	s.CleanupDirs()

	if _, err := os.Stat(agentsDir); !os.IsNotExist(err) {
		t.Error("empty agents dir should have been removed")
	}
	if _, err := os.Stat(instrDir); !os.IsNotExist(err) {
		t.Error("empty instructions dir should have been removed")
	}
	if _, err := os.Stat(filepath.Join(tmp, ".github")); !os.IsNotExist(err) {
		t.Error("empty .github dir should have been removed")
	}
}

// --- Label ---

func TestLabel(t *testing.T) {
	u := &InstallScope{Name: "user", RootDir: "/home/foo/.copilot"}
	if got := u.Label(); !strings.Contains(got, "user-wide") {
		t.Errorf("user Label() = %q, want contains user-wide", got)
	}

	r := ScopeRepo("/some/project")
	if got := r.Label(); got != "/some/project" {
		t.Errorf("repo Label() = %q, want /some/project", got)
	}
}

// --- IsUser ---

func TestIsUser(t *testing.T) {
	u := &InstallScope{Name: "user"}
	if !u.IsUser() {
		t.Error("IsUser() = false for user scope")
	}
	r := ScopeRepo("/r")
	if r.IsUser() {
		t.Error("IsUser() = true for repo scope")
	}
}

// --- Color helpers ---

func TestColorHelpers_WithColor(t *testing.T) {
	orig := UseColor
	UseColor = true
	defer func() { UseColor = orig }()

	got := Color("31", "hello")
	if !strings.Contains(got, "hello") || !strings.Contains(got, "\033[") {
		t.Errorf("Color(31, hello) = %q, expected ANSI wrapping", got)
	}
	if Red("x") != Color("31", "x") {
		t.Error("Red != Color(31)")
	}
	if Green("x") != Color("32", "x") {
		t.Error("Green != Color(32)")
	}
	if Yellow("x") != Color("33", "x") {
		t.Error("Yellow != Color(33)")
	}
	if Dim("x") != Color("2", "x") {
		t.Error("Dim != Color(2)")
	}
	if Bold("x") != Color("1", "x") {
		t.Error("Bold != Color(1)")
	}
}

func TestColorHelpers_NoColor(t *testing.T) {
	orig := UseColor
	UseColor = false
	defer func() { UseColor = orig }()

	if got := Color("31", "hello"); got != "hello" {
		t.Errorf("Color with UseColor=false = %q, want plain message", got)
	}
	if Red("msg") != "msg" {
		t.Error("Red with UseColor=false should return plain msg")
	}
}

// TestOpenCodeModelForLabel pins the frontmatter-name to opencode-id mapping
// that materialization depends on. The unknown case is the important one: it
// must resolve to nothing, so the caller writes no model line instead of an id
// the client would reject.
func TestOpenCodeModelForLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "display name", in: "Claude Sonnet 4.6", want: "github-copilot/claude-sonnet-4.6"},
		{name: "display name with digits and hyphens", in: "GPT-5.3-Codex", want: "github-copilot/gpt-5.3-codex"},
		{name: "the id itself is accepted", in: "claude-opus-4.6", want: "github-copilot/claude-opus-4.6"},
		{name: "case insensitive", in: "claude sonnet 4.6", want: "github-copilot/claude-sonnet-4.6"},
		{name: "surrounding space", in: "  Claude Opus 5  ", want: "github-copilot/claude-opus-5"},
		{name: "empty", in: "", want: ""},
		{name: "unknown name resolves to nothing", in: "Claude Sonnet 9000", want: ""},
		{name: "already qualified is not a known name", in: "github-copilot/claude-opus-5", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := OpenCodeModelForLabel(tt.in); got != tt.want {
				t.Errorf("OpenCodeModelForLabel(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestAgentFrontmatterModelsAreKnown checks the repo's own agents against the
// table: a display name nobody recognises materializes without a model line, so
// a typo would silently disable per-agent model selection for that agent.
func TestAgentFrontmatterModelsAreKnown(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	var files []string
	for _, pat := range []string{
		filepath.Join(root, "agents", "*.agent.md"),
		filepath.Join(root, "prompts", "*.prompt.md"),
	} {
		found, err := filepath.Glob(pat)
		if err != nil {
			t.Fatalf("globbing %s: %v", pat, err)
		}
		files = append(files, found...)
	}
	if len(files) == 0 {
		t.Skip("agents/ and prompts/ not reachable from this checkout")
	}
	re := regexp.MustCompile(`(?m)^model:\s*(.+?)\s*$`)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		m := re.FindSubmatch(data)
		if m == nil {
			continue
		}
		if got := OpenCodeModelForLabel(string(m[1])); got == "" {
			t.Errorf("%s declares model %q, which maps to no Copilot model", filepath.Base(f), m[1])
		}
	}
}

// legacyFile stands in for a nav-pilot built before per-file `source` (#571)
// but with the unknown-key preservation of #588 in place. It is how we check
// the claim that matters: such a binary rewrites a state file it does not fully
// understand without losing the stamp a newer one put there.
type legacyFile struct {
	Path    string                     `json:"path"`
	Hash    string                     `json:"hash"`
	Status  string                     `json:"status,omitempty"`
	Unknown map[string]json.RawMessage `json:"-"`
}

func (f *legacyFile) UnmarshalJSON(b []byte) error {
	type fields legacyFile
	var known fields
	if err := json.Unmarshal(b, &known); err != nil {
		return err
	}
	unknown, err := UnknownJSONKeys(b, KnownJSONKeys(legacyFile{}))
	if err != nil {
		return err
	}
	*f = legacyFile(known)
	f.Unknown = unknown
	return nil
}

func (f legacyFile) MarshalJSON() ([]byte, error) {
	type fields legacyFile
	b, err := json.Marshal(fields(f))
	if err != nil {
		return nil, err
	}
	return AppendUnknownKeys(b, f.Unknown), nil
}

// TestOlderDecoderKeepsPerFileSource is the #588 scenario from the other side:
// the older binary is the one that rewrites the shared state file.
func TestOlderDecoderKeepsPerFileSource(t *testing.T) {
	const stamped = `{"path":".github/agents/foo.md","hash":"sha256:aaa","source":"navikt/annen-pakke"}`

	var f legacyFile
	if err := json.Unmarshal([]byte(stamped), &f); err != nil {
		t.Fatal(err)
	}
	f.Status = FileStatusIgnored // the mutation an `ignore` run makes

	out, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}

	// The stamp must still be there, and still readable as a Source.
	var back InstalledFile
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back.Source != "navikt/annen-pakke" {
		t.Errorf("older decoder dropped the source stamp: %s", out)
	}
	if back.Status != FileStatusIgnored || back.Hash != "sha256:aaa" {
		t.Errorf("older decoder lost known fields: %s", out)
	}
}

// TestKnownJSONKeysUntaggedField guards the fallback in KnownJSONKeys: a field
// with no json name is still matched by encoding/json under its Go name, so
// treating it as unknown would duplicate it on every write. A `json:"-"` field
// is skipped, because encoding/json never writes it at all.
func TestKnownJSONKeysUntaggedField(t *testing.T) {
	type sample struct {
		Tagged   string `json:"tagged"`
		Untagged string
		OmitOnly string `json:",omitempty"`
		Skipped  string `json:"-"`
	}

	keys := KnownJSONKeys(sample{})
	for _, want := range []string{"tagged", "Untagged", "OmitOnly"} {
		if !keys[want] {
			t.Errorf("KnownJSONKeys did not report %q as known; encoding/json matches it", want)
		}
	}
	if keys["Skipped"] || keys["-"] {
		t.Errorf(`a json:"-" field must not be known under any name: %v`, keys)
	}
}
