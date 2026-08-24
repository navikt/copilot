package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// ─── source-tree fixtures ────────────────────────────────────────────────────

// legacySourceTree builds a manifest-less source in the canonical layout: the
// shape every source has today.
func legacySourceTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "collections", "fullstack", "manifest.json"),
		`{"name":"fullstack","description":"Full stack","agents":["test-a"],"skills":["test-s"]}`)
	mustWrite(t, filepath.Join(dir, "agents", "test-a.agent.md"), "---\nname: test-a\ndescription: A\n---\nBody A\n")
	mustWrite(t, filepath.Join(dir, "skills", "test-s", "SKILL.md"), "# Skill S\n")
	return dir
}

// pakkeSourceTree builds a manifest-bearing source whose content lives at
// non-canonical layout paths, so a resolver that ignored the manifest would
// find nothing.
func pakkeSourceTree(t *testing.T, manifestJSON string) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), manifestJSON)
	mustWrite(t, filepath.Join(dir, "plugin", "agents", "grillmester.agent.md"),
		"---\nname: grillmester\ndescription: Chef\n---\nBody G\n")
	mustWrite(t, filepath.Join(dir, "plugin", "skills", "grilling", "SKILL.md"), "# Grilling\n")
	return dir
}

const tier1ManifestJSON = `{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "Grillmester agentpakke",
  "clients": {"copilot": {"primaryAgents": ["grillmester"]}},
  "layout": {"agents": "plugin/agents", "skills": "plugin/skills"}
}`

// repoTarget makes a git-looking target dir for repo-scope installs.
func repoTarget(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// isolatedConfig points NAV_PILOT_CONFIG at a temp file so tests never read or
// write the developer's own config.
func isolatedConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)
	return path
}

// ─── C3: the legacy path must not change ─────────────────────────────────────

func TestLegacyInstallUnchanged(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	target := repoTarget(t)

	src := &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke on a manifest-less source: %v", err)
	}
	if src.Pakke != nil {
		t.Fatalf("attachPakke attached %+v, want nil for a source without a manifest", src.Pakke)
	}

	scope := ScopeRepo(target)
	if err := cmdInstallFromSource("fullstack", src, scope, false, false, false); err != nil {
		t.Fatalf("cmdInstallFromSource: %v", err)
	}

	state, err := readScopedState(scope)
	if err != nil {
		t.Fatalf("readScopedState: %v", err)
	}
	if state.Collection != "fullstack" {
		t.Errorf("state.Collection = %q, want %q (legacy label must round-trip verbatim)", state.Collection, "fullstack")
	}
	if state.SourceRepo != defaultSourceRepo {
		t.Errorf("state.SourceRepo = %q, want %q", state.SourceRepo, defaultSourceRepo)
	}
	if state.Scope != "repo" {
		t.Errorf("state.Scope = %q, want repo", state.Scope)
	}
	wantFiles := []string{".github/agents/test-a.agent.md", ".github/skills/test-s/"}
	var gotFiles []string
	for _, f := range state.Files {
		gotFiles = append(gotFiles, f.Path)
	}
	if strings.Join(gotFiles, ",") != strings.Join(wantFiles, ",") {
		t.Errorf("state files = %v, want %v", gotFiles, wantFiles)
	}

	// Installed bytes must be the source bytes, unchanged.
	got, err := os.ReadFile(filepath.Join(target, ".github", "agents", "test-a.agent.md"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(srcDir, "agents", "test-a.agent.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("installed agent bytes = %q, want %q", got, want)
	}
}

func TestLegacyAdapterKeepsCanonicalLayout(t *testing.T) {
	srcDir := legacySourceTree(t)
	resolver := resolverFor(srcDir, pakkeFor(&Source{Dir: srcDir}, "fullstack"))
	art, ok := resolver.Get(KindAgent, "test-a")
	if !ok {
		t.Fatal("legacy adapter resolver did not find agents/test-a.agent.md")
	}
	if art.RelPath != filepath.Join("agents", "test-a.agent.md") {
		t.Errorf("RelPath = %q, want agents/test-a.agent.md", art.RelPath)
	}
}

func TestStateCollection(t *testing.T) {
	pakkeSrc := &Source{Pakke: agentpakke.SynthesizeLegacy("grillmester")}
	tests := []struct {
		name       string
		src        *Source
		collection string
		want       string
	}{
		{"legacy collection", &Source{}, "fullstack", "fullstack"},
		{"legacy all", &Source{}, CollectionAll, CollectionAll},
		{"agentpakke supersedes", pakkeSrc, "fullstack", "grillmester"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stateCollection(tt.src, tt.collection); got != tt.want {
				t.Errorf("stateCollection = %q, want %q", got, tt.want)
			}
		})
	}
}

// ─── supersede install path ──────────────────────────────────────────────────

func TestPakkeInstallUsesLayoutAndRecordsName(t *testing.T) {
	isolatedConfig(t)
	srcDir := pakkeSourceTree(t, tier1ManifestJSON)
	target := repoTarget(t)

	src := &Source{Dir: srcDir, SHA: "def5678", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke: %v", err)
	}
	if src.Pakke == nil || src.Pakke.Name != "grillmester" {
		t.Fatalf("attachPakke did not load the manifest: %+v", src.Pakke)
	}

	scope := ScopeRepo(target)
	if err := cmdInstallFromSource("grillmester", src, scope, false, false, false); err != nil {
		t.Fatalf("cmdInstallFromSource: %v", err)
	}

	if _, err := os.Stat(filepath.Join(target, ".github", "agents", "grillmester.agent.md")); err != nil {
		t.Errorf("agent from layout path plugin/agents was not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, ".github", "skills", "grilling", "SKILL.md")); err != nil {
		t.Errorf("skill from layout path plugin/skills was not installed: %v", err)
	}

	state, err := readScopedState(scope)
	if err != nil {
		t.Fatalf("readScopedState: %v", err)
	}
	if state.Collection != "grillmester" {
		t.Errorf("state.Collection = %q, want the manifest name %q", state.Collection, "grillmester")
	}
	if state.SourceRepo != "navikt/grillmester" {
		t.Errorf("state.SourceRepo = %q, want navikt/grillmester", state.SourceRepo)
	}
}

func TestAttachPakkeFailsClosed(t *testing.T) {
	tests := []struct {
		name         string
		manifestJSON string
		wantContains string
	}{
		{
			name:         "unsupported contract version",
			manifestJSON: `{"contractVersion":"99","name":"x","description":"d","clients":{"copilot":{"primaryAgents":["a"]}},"layout":{"agents":"a","skills":"s"}}`,
			wantContains: "contractVersion",
		},
		{
			name:         "tier 1 without layout",
			manifestJSON: `{"contractVersion":"1","name":"x","description":"d","clients":{"copilot":{"primaryAgents":["a"]}}}`,
			wantContains: "layout",
		},
		{
			name:         "malformed json",
			manifestJSON: `{"contractVersion":`,
			wantContains: "agentpakke manifest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), tt.manifestJSON)
			src := &Source{Dir: dir, Repo: "navikt/broken"}
			err := attachPakke(src)
			if err == nil {
				t.Fatal("attachPakke = nil, want a fail-closed error")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Errorf("error %q does not mention %q", err, tt.wantContains)
			}
			if !strings.Contains(err.Error(), "Nothing was installed") {
				t.Errorf("error %q is not actionable about what happened", err)
			}
		})
	}
}

func TestValidatePakkeSourceLeavesNothingInstalled(t *testing.T) {
	// A manifest whose layout points at a directory the repo does not ship.
	manifest := `{
	  "contractVersion": "1",
	  "name": "broken",
	  "description": "d",
	  "clients": {"copilot": {"primaryAgents": ["a"]}},
	  "layout": {"agents": "missing/agents", "skills": "missing/skills"}
	}`
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), manifest)
	target := repoTarget(t)

	src := &Source{Dir: dir, SHA: "aaa", Version: "dev", Repo: "navikt/broken"}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke: %v", err)
	}
	scope := ScopeRepo(target)
	err := cmdInstallFromSource("broken", src, scope, false, false, false)
	if err == nil {
		t.Fatal("install from a non-conforming agentpakke succeeded, want fail-closed")
	}
	if !strings.Contains(err.Error(), "does not conform") {
		t.Errorf("error %q does not name the conformance failure", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(target, ".github")); len(entries) > 0 {
		t.Errorf("fail-closed install left %d entries under .github/", len(entries))
	}
	if state, _ := readScopedState(scope); state != nil {
		t.Error("fail-closed install wrote a state file")
	}
}

// ─── B1: source precedence ───────────────────────────────────────────────────

func TestSourceRepoFor(t *testing.T) {
	tests := []struct {
		name       string
		configLine string
		flag       string
		want       string
		wantErr    bool
	}{
		{name: "no config, no flag → default", want: ""},
		{name: "flag only", flag: "navikt/grillmester", want: "navikt/grillmester"},
		{name: "config only", configLine: `source = "navikt/grillmester"`, want: "navikt/grillmester"},
		{name: "flag wins over config", configLine: `source = "navikt/a"`, flag: "navikt/b", want: "navikt/b"},
		{name: "cleared config → default", configLine: `source = ""`, want: ""},
		{name: "invalid config value", configLine: `source = "not a repo"`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := isolatedConfig(t)
			content := "version = 1\n"
			if tt.configLine != "" {
				content += tt.configLine + "\n"
			}
			mustWrite(t, path, content)

			got, err := sourceRepoFor(tt.flag)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("sourceRepoFor(%q) = %q, want error", tt.flag, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("sourceRepoFor(%q) = %v", tt.flag, err)
			}
			if got != tt.want {
				t.Errorf("sourceRepoFor(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

func TestValidateSourceValue(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{"navikt/copilot", false},
		{"navikt/grillmester", false},
		{"/abs/path/to/checkout", false},
		{"", true},
		{"navikt", true},
		{"navikt/copilot/extra", true},
		{"./relative", true},
		{"~/home", true},
		{" navikt/copilot", true},
	}
	for _, tt := range tests {
		err := validateSourceValue(tt.value)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateSourceValue(%q) = %v, wantErr = %v", tt.value, err, tt.wantErr)
		}
	}
}

// ─── B2: persistence ─────────────────────────────────────────────────────────

func TestPersistInstalledSource(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n")

	persistInstalledSource("navikt/grillmester", true) // dry run: never persists
	if got, _ := configuredSourceRepo(); got != "" {
		t.Errorf("dry-run install persisted source = %q, want none", got)
	}

	persistInstalledSource("", false) // no explicit --source: nothing to persist
	if got, _ := configuredSourceRepo(); got != "" {
		t.Errorf("install without --source persisted source = %q, want none", got)
	}

	persistInstalledSource("navikt/grillmester", false)
	got, err := configuredSourceRepo()
	if err != nil {
		t.Fatalf("configuredSourceRepo: %v", err)
	}
	if got != "navikt/grillmester" {
		t.Errorf("persisted source = %q, want navikt/grillmester", got)
	}

	// Clearing goes back to the default.
	if err := cmdConfigSet("source", ""); err != nil {
		t.Fatalf("config set source \"\": %v", err)
	}
	if got, _ := configuredSourceRepo(); got != "" {
		t.Errorf("cleared source = %q, want empty (default %s)", got, defaultSourceRepo)
	}
	if effective, _ := sourceRepoFor(""); effective != "" {
		t.Errorf("after clearing, effective source = %q, want the default", effective)
	}
}

func TestConfigSetSourceValidates(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n")

	if err := cmdConfigSet("source", "not a repo"); err == nil {
		t.Error("config set source with a malformed value succeeded, want error")
	}
	if err := cmdConfigSet("source", "navikt/grillmester"); err != nil {
		t.Errorf("config set source = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `source = "navikt/grillmester"`) {
		t.Errorf("config file does not contain the source key:\n%s", data)
	}
}

func TestConfigShowIncludesSource(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\nsource = \"navikt/grillmester\"\n")

	out := captureStdoutFor(t, func() {
		if err := cmdConfigShow(true); err != nil {
			t.Fatalf("cmdConfigShow: %v", err)
		}
	})
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parsing config show --json output %q: %v", out, err)
	}
	if got["source"] != "navikt/grillmester" {
		t.Errorf("config show source = %v, want navikt/grillmester", got["source"])
	}
}

func TestConfigShowSourceDefaults(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n")

	out := captureStdoutFor(t, func() {
		if err := cmdConfigShow(true); err != nil {
			t.Fatalf("cmdConfigShow: %v", err)
		}
	})
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parsing config show --json output %q: %v", out, err)
	}
	if got["source"] != defaultSourceRepo {
		t.Errorf("config show source = %v, want %s", got["source"], defaultSourceRepo)
	}
}

// ─── B3/B4: cross-source guard ───────────────────────────────────────────────

func TestGuardScopeSource(t *testing.T) {
	tests := []struct {
		name            string
		configuredSrc   string
		flagSource      string
		stateSourceRepo string // "-" means: write no state file at all
		wantErr         bool
	}{
		{name: "no configured source", stateSourceRepo: defaultSourceRepo},
		{name: "no install yet", configuredSrc: "navikt/grillmester", stateSourceRepo: "-"},
		{name: "same source", configuredSrc: defaultSourceRepo, stateSourceRepo: defaultSourceRepo},
		{name: "case-insensitive repo match", configuredSrc: "Navikt/Copilot", stateSourceRepo: defaultSourceRepo},
		{name: "explicit flag overrides", configuredSrc: "navikt/grillmester", flagSource: "navikt/grillmester", stateSourceRepo: defaultSourceRepo},
		{name: "cross-source refused", configuredSrc: "navikt/grillmester", stateSourceRepo: defaultSourceRepo, wantErr: true},
		{name: "unknown recorded source is not guarded on install", configuredSrc: "navikt/grillmester", stateSourceRepo: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := isolatedConfig(t)
			content := "version = 1\n"
			if tt.configuredSrc != "" {
				content += "source = \"" + tt.configuredSrc + "\"\n"
			}
			mustWrite(t, path, content)

			scope := ScopeRepo(repoTarget(t))
			if tt.stateSourceRepo != "-" {
				writeGuardState(t, scope, tt.stateSourceRepo)
			}

			err := guardScopeSource(scope, tt.flagSource)
			if (err != nil) != tt.wantErr {
				t.Fatalf("guardScopeSource = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "--source") {
				t.Errorf("guard error %q does not tell the user how to proceed", err)
			}
		})
	}
}

func TestGuardScopeSyncSource(t *testing.T) {
	tests := []struct {
		name            string
		configuredSrc   string
		flagSource      string
		stateSourceRepo string
		wantErr         bool
	}{
		{name: "recorded source wins, no error", configuredSrc: "navikt/grillmester", stateSourceRepo: defaultSourceRepo},
		{name: "same source", configuredSrc: defaultSourceRepo, stateSourceRepo: defaultSourceRepo},
		{name: "no configured source", stateSourceRepo: ""},
		{name: "unknown recorded source is refused", configuredSrc: "navikt/grillmester", stateSourceRepo: "", wantErr: true},
		{name: "explicit flag overrides", configuredSrc: "navikt/grillmester", flagSource: defaultSourceRepo, stateSourceRepo: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := isolatedConfig(t)
			content := "version = 1\n"
			if tt.configuredSrc != "" {
				content += "source = \"" + tt.configuredSrc + "\"\n"
			}
			mustWrite(t, path, content)

			scope := ScopeRepo(repoTarget(t))
			writeGuardState(t, scope, tt.stateSourceRepo)

			err := guardScopeSyncSource(scope, tt.flagSource)
			if (err != nil) != tt.wantErr {
				t.Fatalf("guardScopeSyncSource = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestGuardIsPerScope covers B4: two scopes may track different agentpakker.
func TestGuardIsPerScope(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\nsource = \"navikt/grillmester\"\n")

	pakkeScope := ScopeRepo(repoTarget(t))
	writeGuardState(t, pakkeScope, "navikt/grillmester")
	navScope := ScopeRepo(repoTarget(t))
	writeGuardState(t, navScope, defaultSourceRepo)

	if err := guardScopeSource(pakkeScope, ""); err != nil {
		t.Errorf("scope on the configured source was guarded: %v", err)
	}
	if err := guardScopeSource(navScope, ""); err == nil {
		t.Error("scope on a different source was not guarded")
	}
	// Sync leaves both alone: each scope syncs from its own recorded source.
	if err := guardScopeSyncSource(navScope, ""); err != nil {
		t.Errorf("sync guard fired for a scope with a recorded source: %v", err)
	}
}

func writeGuardState(t *testing.T, scope *InstallScope, sourceRepo string) {
	t.Helper()
	state := &StateFile{
		Collection: "fullstack",
		Version:    "dev",
		Scope:      scope.Name,
		SourceRepo: sourceRepo,
		SourceSHA:  "abc1234",
	}
	if err := writeScopedState(scope, state); err != nil {
		t.Fatal(err)
	}
}

func TestSameSourceRepo(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"navikt/copilot", "navikt/copilot", true},
		{"navikt/copilot", "Navikt/Copilot", true},
		{"navikt/copilot", "navikt/grillmester", false},
		{"/tmp/checkout", "/tmp/checkout/", true},
		{"/tmp/checkout", "/tmp/other", false},
	}
	for _, tt := range tests {
		if got := sameSourceRepo(tt.a, tt.b); got != tt.want {
			t.Errorf("sameSourceRepo(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// ─── A5: nav-pilot validate ──────────────────────────────────────────────────

func TestValidateSourceTree(t *testing.T) {
	brokenLayout := `{
	  "contractVersion": "1",
	  "name": "grillmester",
	  "description": "d",
	  "clients": {"copilot": {"primaryAgents": ["grillmester"]}},
	  "layout": {"agents": "nope/agents", "skills": "nope/skills"}
	}`

	tests := []struct {
		name         string
		tree         func(t *testing.T) string
		wantKind     string
		wantFindings bool
		wantNote     string
	}{
		{
			name:     "legacy collection source validates as legacy",
			tree:     legacySourceTree,
			wantKind: "legacy",
			wantNote: "no manifest (legacy collection source)",
		},
		{
			name: "empty source has nothing to install",
			tree: func(t *testing.T) string { return t.TempDir() },

			wantKind:     "legacy",
			wantFindings: true,
			wantNote:     "no manifest (legacy collection source)",
		},
		{
			name:     "conforming agentpakke",
			tree:     func(t *testing.T) string { return pakkeSourceTree(t, tier1ManifestJSON) },
			wantKind: "agentpakke",
			wantNote: "agentpakke: grillmester",
		},
		{
			name:         "agentpakke with missing layout dirs",
			tree:         func(t *testing.T) string { return pakkeSourceTree(t, brokenLayout) },
			wantKind:     "agentpakke",
			wantFindings: true,
		},
		{
			name: "collection listing a missing agent",
			tree: func(t *testing.T) string {
				dir := t.TempDir()
				mustWrite(t, filepath.Join(dir, "collections", "x", "manifest.json"),
					`{"name":"x","description":"d","agents":["ghost"]}`)
				return dir
			},
			wantKind:     "legacy",
			wantFindings: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &Source{Dir: tt.tree(t), SHA: "abc", Repo: "navikt/test"}
			kind, notes, findings := validateSourceTree(src)
			if kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", kind, tt.wantKind)
			}
			if (len(findings) > 0) != tt.wantFindings {
				t.Errorf("findings = %v, wantFindings = %v", findings, tt.wantFindings)
			}
			if tt.wantNote != "" && !strings.Contains(strings.Join(notes, "\n"), tt.wantNote) {
				t.Errorf("notes %v do not mention %q", notes, tt.wantNote)
			}
		})
	}
}

func TestCmdValidateExitsNonZeroOnViolation(t *testing.T) {
	isolatedConfig(t)
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile),
		`{"contractVersion":"1","name":"x","description":"d","clients":{"copilot":{"primaryAgents":["a"]}},"layout":{"agents":"gone","skills":"gone"}}`)

	orig := resolveSourceRaw
	t.Cleanup(func() { resolveSourceRaw = orig })
	resolveSourceRaw = func(ref, sourceRepo string) (*Source, error) {
		return &Source{Dir: dir, SHA: "abc", Repo: "navikt/x"}, nil
	}

	out := captureStdoutFor(t, func() {
		if err := cmdValidate("", "", false); err == nil {
			t.Error("cmdValidate on a non-conforming source = nil, want an error (non-zero exit)")
		}
	})
	if !strings.Contains(out, "problem") {
		t.Errorf("validate output does not list problems:\n%s", out)
	}
}

func TestCmdValidateLegacySourcePasses(t *testing.T) {
	isolatedConfig(t)
	dir := legacySourceTree(t)

	orig := resolveSourceRaw
	t.Cleanup(func() { resolveSourceRaw = orig })
	resolveSourceRaw = func(ref, sourceRepo string) (*Source, error) {
		return &Source{Dir: dir, SHA: "abc", Repo: defaultSourceRepo}, nil
	}

	out := captureStdoutFor(t, func() {
		if err := cmdValidate("", "", false); err != nil {
			t.Errorf("cmdValidate on a legacy source = %v, want nil", err)
		}
	})
	if !strings.Contains(out, "legacy collection source") {
		t.Errorf("validate output does not report the legacy case:\n%s", out)
	}
}

// captureStdoutFor runs fn with stdout redirected to a pipe and returns what it
// printed.
func captureStdoutFor(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()

	fn()
	w.Close()
	os.Stdout = orig
	out := <-done
	r.Close()
	return out
}
