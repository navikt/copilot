package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
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
// write the developer's own config, and HOME at a temp directory so the user
// scope they read — install state, and the pinned revisions the launch path
// looks up — is this test's and not the developer's. A test that wants a
// specific home sets HOME again after calling this.
func isolatedConfig(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
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

// ─── Tier 2-only agentpakker ─────────────────────────────────────────────────

const tier2ManifestJSON = `{
  "contractVersion": "1",
  "name": "grillmester",
  "description": "Grillmester agentpakke, pre-built",
  "clients": {"copilot": {"payloads": {"full": {"path": "dist/copilot/full", "primaryAgents": ["grillmester"]}}}}
}`

// tier2SourceTree builds a conforming Tier 2 agentpakke: one payload tree with
// a real file at a real digest and a declared mode, its payload manifest, and
// no layout. An empty files map would make every check below unfalsifiable —
// nothing would be materialized and nothing could fail to verify.
func tier2SourceTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), tier2ManifestJSON)
	writeTier2Payload(t, filepath.Join(dir, "dist", "copilot", "full"), "copilot-full")
	return dir
}

// TestTier2PakkeInstallsIntoUserScopeOnly is the inversion of the refusal this
// test used to pin: every install path now routes a payload-only agentpakke to
// the revision pin, `list` reports it as the zero-item install it is, and only
// the scope that could not hold a pin still refuses.
func TestTier2PakkeInstallsIntoUserScopeOnly(t *testing.T) {
	newTier2Source := func(t *testing.T) *Source {
		t.Helper()
		src := &Source{Dir: tier2SourceTree(t), SHA: "abc1234", Version: "dev", Repo: "navikt/grillmester"}
		if err := attachPakke(src); err != nil {
			t.Fatalf("attachPakke on a conforming Tier 2 manifest: %v", err)
		}
		if !payloadOnly(src) {
			t.Fatalf("fixture is not Tier 2-only: %+v", src.Pakke)
		}
		return src
	}

	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	installs := []struct {
		name string
		run  func(scope *InstallScope, src *Source) error
	}{
		{"collection install", func(scope *InstallScope, src *Source) error {
			return cmdInstallFromSource("grillmester", src, scope, false, false, false)
		}},
		{"install all", func(scope *InstallScope, src *Source) error {
			return installAllFromSource(scope, src, nil, false, false, false)
		}},
		{"interactive user install", func(scope *InstallScope, src *Source) error {
			return interactiveUserInstallFromSource(scope, src, "")
		}},
	}
	for _, tt := range installs {
		t.Run(tt.name, func(t *testing.T) {
			scope := pinEnv(t)
			src := newTier2Source(t)

			if err := tt.run(scope, src); err != nil {
				t.Fatalf("%s of a Tier 2 agentpakke = %v, want a pinned install", tt.name, err)
			}

			state, err := readScopedState(scope)
			if err != nil || state == nil {
				t.Fatalf("readScopedState = (%v, %v), want the pin", state, err)
			}
			if state.Collection != "grillmester" || state.SourceRepo != src.Repo || state.SourceSHA != src.SHA {
				t.Errorf("state = %+v, want a pin on %s@%s named grillmester", state, src.Repo, src.SHA)
			}
			if len(state.Files) != 0 {
				t.Errorf("a Tier 2 install recorded %d files; it materializes none", len(state.Files))
			}
			dir := filepath.Join(pakkeRevisionDir(src.Repo, src.SHA), "copilot", "full")
			if err := agentpakke.VerifyPayloadExact(dir, filepath.Join(dir, agentpakke.PayloadManifestFile)); err != nil {
				t.Errorf("the pinned payload does not verify: %v", err)
			}
		})
	}

	t.Run("list", func(t *testing.T) {
		pinEnv(t)
		src := newTier2Source(t)
		stubResolveSource(t, src)

		if err := cmdList(nil, "", "", false, true); err != nil {
			t.Fatalf("cmdList over a Tier 2 agentpakke = %v, want the zero-item listing", err)
		}
		m, err := pakkeContents(resolverFor(src.Dir, src.Pakke), src)
		if err != nil {
			t.Fatalf("pakkeContents = %v, want a zero-item manifest", err)
		}
		if n := len(m.Agents) + len(m.Skills) + len(m.Instructions) + len(m.Prompts); n != 0 {
			t.Errorf("pakkeContents returned %d items, want 0", n)
		}
		if m.Name != "grillmester" {
			t.Errorf("manifest name = %q, want grillmester", m.Name)
		}
	})

	t.Run("repo scope is refused", func(t *testing.T) {
		pinEnv(t)
		src := newTier2Source(t)
		target := repoTarget(t)

		err := cmdInstallFromSource("grillmester", src, ScopeRepo(target), false, false, false)
		if err == nil {
			t.Fatal("a repo-scope Tier 2 install was accepted, want a refusal")
		}
		if !strings.Contains(err.Error(), "nav-pilot install --user") {
			t.Errorf("error %q does not point at the scope that works", err)
		}
		if strings.Contains(err.Error(), "declares a layout") || strings.Contains(err.Error(), "no agents, skills, or instructions found") {
			t.Errorf("error %q is still the misleading layout/empty-source message", err)
		}
		if entries, _ := os.ReadDir(filepath.Join(target, ".github")); len(entries) > 0 {
			t.Errorf("refused Tier 2 install left %d entries under .github/", len(entries))
		}
	})
}

// ─── B4: sync says which source it actually used ─────────────────────────────

// TestSyncNotesRecordedSourceWins covers MINOR-5: the recorded source keeps
// winning, but the user is told when that means their configured source is not
// the one being read.
func TestSyncNotesRecordedSourceWins(t *testing.T) {
	tests := []struct {
		name          string
		configuredSrc string
		stateSource   string
		wantNote      bool
	}{
		{name: "configured source differs", configuredSrc: "navikt/grillmester", stateSource: defaultSourceRepo, wantNote: true},
		{name: "configured source matches", configuredSrc: defaultSourceRepo, stateSource: defaultSourceRepo},
		{name: "configured source matches case-insensitively", configuredSrc: "Navikt/Copilot", stateSource: defaultSourceRepo},
		{name: "nothing configured", stateSource: "navikt/grillmester"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := isolatedConfig(t)
			content := "version = 1\n"
			if tt.configuredSrc != "" {
				content += "source = \"" + tt.configuredSrc + "\"\n"
			}
			mustWrite(t, path, content)

			target := repoTarget(t)
			scope := ScopeRepo(target)
			if err := writeScopedState(scope, &StateFile{
				Collection: "fullstack",
				Version:    "dev",
				Scope:      scope.Name,
				SourceRepo: tt.stateSource,
				SourceSHA:  "abc1234",
			}); err != nil {
				t.Fatal(err)
			}

			srcDir := legacySourceTree(t)
			orig := resolveSourceForSync
			t.Cleanup(func() { resolveSourceForSync = orig })
			var gotSourceRepo string
			resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
				gotSourceRepo = sourceRepo
				return &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: sourceRepo}, nil
			}

			out := captureStdoutFor(t, func() {
				if err := cmdSync(scope, "", "", false, false); err != nil {
					t.Fatalf("cmdSync: %v", err)
				}
			})

			if gotSourceRepo != tt.stateSource {
				t.Errorf("sync resolved source %q, want the recorded %q", gotSourceRepo, tt.stateSource)
			}
			gotNote := strings.Contains(out, "is not used here")
			if gotNote != tt.wantNote {
				t.Errorf("informational line present = %v, want %v. Output:\n%s", gotNote, tt.wantNote, out)
			}
			if tt.wantNote && !strings.Contains(out, tt.stateSource) {
				t.Errorf("informational line does not name the source actually used:\n%s", out)
			}
		})
	}
}

// TestSyncAdoptsSourceForPreTrackingScope covers the pre-source-tracking
// policy: a scope installed before nav-pilot recorded sources is synced from
// the effective source, told so once, and has that source recorded afterwards
// so the normal B3 guard applies from the next run on.
func TestSyncAdoptsSourceForPreTrackingScope(t *testing.T) {
	tests := []struct {
		name          string
		configuredSrc string
		jsonOutput    bool
		syncErr       error
		wantSource    string // recorded in state after the sync
		wantNote      bool
	}{
		{
			name:          "sourceless scope adopts the configured source",
			configuredSrc: "navikt/grillmester",
			wantSource:    "navikt/grillmester",
			wantNote:      true,
		},
		{
			name:          "json output stays machine-readable",
			configuredSrc: "navikt/grillmester",
			jsonOutput:    true,
			wantSource:    "navikt/grillmester",
		},
		{
			name:          "a failed sync records nothing",
			configuredSrc: "navikt/grillmester",
			syncErr:       errors.New("could not clone"),
			wantNote:      true,
		},
		{
			name: "nothing configured, nothing adopted",
		},
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
			writeGuardState(t, scope, "")

			srcDir := legacySourceTree(t)
			orig := resolveSourceForSync
			t.Cleanup(func() { resolveSourceForSync = orig })
			resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
				if tt.syncErr != nil {
					return nil, tt.syncErr
				}
				return &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: sourceRepo}, nil
			}

			out := captureStdoutFor(t, func() {
				err := cmdSync(scope, "", "", false, tt.jsonOutput)
				if tt.syncErr == nil && err != nil {
					t.Errorf("cmdSync = %v, want nil", err)
				}
				if tt.syncErr != nil && err == nil {
					t.Error("cmdSync succeeded, want the resolve failure")
				}
			})

			gotNote := strings.Contains(out, "predates source tracking")
			if gotNote != tt.wantNote {
				t.Errorf("informational line present = %v, want %v. Output:\n%s", gotNote, tt.wantNote, out)
			}
			if tt.wantNote && !strings.Contains(out, tt.configuredSrc) {
				t.Errorf("informational line does not name the source being adopted:\n%s", out)
			}

			state, err := readScopedState(scope)
			if err != nil {
				t.Fatalf("readScopedState: %v", err)
			}
			if state.SourceRepo != tt.wantSource {
				t.Errorf("state.SourceRepo = %q, want %q", state.SourceRepo, tt.wantSource)
			}
		})
	}
}

// TestSyncAdoptedSourceIsGuardedAfterwards covers the point of adopting: the
// healed scope is an ordinary recorded-source scope, so a later configured
// source change is refused on install instead of silently mixed in.
func TestSyncAdoptedSourceIsGuardedAfterwards(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\nsource = \""+defaultSourceRepo+"\"\n")

	scope := ScopeRepo(repoTarget(t))
	writeGuardState(t, scope, "")

	srcDir := legacySourceTree(t)
	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
		return &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: sourceRepo}, nil
	}
	captureStdoutFor(t, func() {
		if err := cmdSync(scope, "", "", false, false); err != nil {
			t.Fatalf("cmdSync: %v", err)
		}
	})

	// The user now points the config at another agentpakke.
	mustWrite(t, path, "version = 1\nsource = \"navikt/grillmester\"\n")
	if err := guardScopeSource(scope, ""); err == nil {
		t.Error("install into the healed scope was not guarded against the new source")
	}
	adopted, err := adoptSyncSource(scope, "")
	if err != nil || adopted != "" {
		t.Errorf("adoptSyncSource on the healed scope = (%q, %v), want (\"\", nil)", adopted, err)
	}
}

// ─── root TUI staleness is a default-source question ─────────────────────────

// TestScopeStalenessOnlyForDefaultSource covers MINOR-4: nav-pilot's release
// feed describes navikt/copilot, so a scope installed from another agentpakke
// is neither assessed against it nor offered a sync to nav-pilot/<latest>.
func TestScopeStalenessOnlyForDefaultSource(t *testing.T) {
	orig := assessStaleness
	t.Cleanup(func() { assessStaleness = orig })
	assessed := 0
	assessStaleness = func(installedVersion string) artifacts.StalenessAssessment {
		assessed++
		return artifacts.StalenessAssessment{LatestVersion: "2099.01.01-000000-abc1234"}
	}

	tests := []struct {
		name        string
		sourceRepo  string
		wantLatest  string
		wantAssess  int
		description string
	}{
		{name: "default source is assessed", sourceRepo: defaultSourceRepo, wantLatest: "2099.01.01-000000-abc1234", wantAssess: 1},
		{name: "untracked source is assessed as the default", sourceRepo: "", wantLatest: "2099.01.01-000000-abc1234", wantAssess: 1},
		{name: "other agentpakke is left alone", sourceRepo: "navikt/grillmester", wantLatest: "", wantAssess: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assessed = 0
			scope := ScopeRepo(repoTarget(t))
			state := &StateFile{Collection: "grillmester", Version: "2026.01.01-000000-abc1234", SourceRepo: tt.sourceRepo}

			got := scopeStaleness(scope, state)
			if got != tt.wantLatest {
				t.Errorf("scopeStaleness = %q, want %q", got, tt.wantLatest)
			}
			if assessed != tt.wantAssess {
				t.Errorf("assessStaleness called %d time(s), want %d — a non-default source must not be checked against nav-pilot releases", assessed, tt.wantAssess)
			}
		})
	}
}

// ─── new-item reminders for agentpakke scopes ────────────────────────────────

// TestDetectNewItemsForPakkeScope covers MINOR-8: a pakke is installed whole, so
// content it grows is new content for that scope — the reminder is not limited
// to the legacy "(all)" user-scope install.
func TestDetectNewItemsForPakkeScope(t *testing.T) {
	isolatedConfig(t)
	srcDir := pakkeSourceTree(t, tier1ManifestJSON)
	target := repoTarget(t)
	scope := ScopeRepo(target)

	src := &Source{Dir: srcDir, SHA: "def5678", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke: %v", err)
	}
	if err := cmdInstallFromSource("grillmester", src, scope, false, false, false); err != nil {
		t.Fatalf("cmdInstallFromSource: %v", err)
	}

	resolver := resolverFor(srcDir, src.Pakke)
	if items := detectNewItems(scope, resolver, src); len(items) != 0 {
		t.Errorf("freshly installed pakke reported new items: %v", items)
	}

	// The agentpakke grows an agent.
	mustWrite(t, filepath.Join(srcDir, "plugin", "agents", "sausage.agent.md"),
		"---\nname: sausage\ndescription: S\n---\nBody\n")

	items := detectNewItems(scope, resolver, src)
	if len(items) != 1 || !strings.Contains(items[0], "sausage") {
		t.Errorf("detectNewItems = %v, want the new agent from the pakke layout", items)
	}
	if got := installCommandFor(scope, src); got != "nav-pilot install grillmester" {
		t.Errorf("installCommandFor = %q, want the pakke install command", got)
	}

	// A user's deselected item stays out of the reminder.
	state, err := readScopedState(scope)
	if err != nil {
		t.Fatal(err)
	}
	state.Files = append(state.Files, InstalledFile{
		Path:   KindAgent.RelPathForName(scope, "sausage"),
		Status: fileStatusIgnored,
	})
	if err := writeScopedState(scope, state); err != nil {
		t.Fatal(err)
	}
	if items := detectNewItems(scope, resolver, src); len(items) != 0 {
		t.Errorf("detectNewItems reported an ignored item: %v", items)
	}
}

// TestDetectNewItemsLegacyUnchanged pins the pre-existing rule: without an
// agentpakke, only the "(all)" user-scope install gets reminders.
func TestDetectNewItemsLegacyUnchanged(t *testing.T) {
	srcDir := legacySourceTree(t)
	resolver := resolverFor(srcDir, pakkeFor(nil, CollectionAll))

	tests := []struct {
		name       string
		collection string
		userScope  bool
		want       int
	}{
		{name: "single collection in repo scope", collection: "fullstack", want: 0},
		{name: "single collection in user scope", collection: "fullstack", userScope: true, want: 0},
		{name: "(all) in repo scope", collection: CollectionAll, want: 0},
		{name: "(all) in user scope", collection: CollectionAll, userScope: true, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			scope := ScopeRepo(repoTarget(t))
			if tt.userScope {
				var err error
				if scope, err = ScopeUser(); err != nil {
					t.Fatal(err)
				}
			}
			if err := writeScopedState(scope, &StateFile{
				Collection: tt.collection,
				Scope:      scope.Name,
				SourceRepo: defaultSourceRepo,
			}); err != nil {
				t.Fatal(err)
			}
			if got := detectNewItems(scope, resolver, nil); len(got) != tt.want {
				t.Errorf("detectNewItems = %v (%d), want %d item(s)", got, len(got), tt.want)
			}
		})
	}
}

// ─── containment: symlinked layout directories ───────────────────────────────

// symlinkedLayoutTree builds an agentpakke whose layout.agents directory is a
// symlink to content outside the checkout — the escape an Lstat of the final
// path component alone does not catch.
func symlinkedLayoutTree(t *testing.T) string {
	t.Helper()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.agent.md"), "---\nname: secret\ndescription: S\n---\nBody\n")

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), tier1ManifestJSON)
	if err := os.MkdirAll(filepath.Join(dir, "plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "plugin", "agents")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	mustWrite(t, filepath.Join(dir, "plugin", "skills", "grilling", "SKILL.md"), "# Grilling\n")
	return dir
}

func TestSymlinkedLayoutDirIsRefused(t *testing.T) {
	isolatedConfig(t)
	srcDir := symlinkedLayoutTree(t)
	target := repoTarget(t)

	src := &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke: %v", err)
	}

	// validate reports it...
	_, _, findings := validateSourceTree(src)
	if len(findings) == 0 {
		t.Error("validate accepted a layout directory symlinked outside the repo")
	}
	var joined string
	for _, f := range findings {
		joined += f.Error() + "\n"
	}
	if !strings.Contains(joined, "symlink") {
		t.Errorf("findings do not name the symlink:\n%s", joined)
	}

	// ...and install refuses before writing anything.
	err := cmdInstallFromSource("grillmester", src, ScopeRepo(target), false, false, false)
	if err == nil {
		t.Fatal("install from a source with a symlinked layout directory succeeded, want a refusal")
	}
	if entries, _ := os.ReadDir(filepath.Join(target, ".github")); len(entries) > 0 {
		t.Errorf("refused install left %d entries under .github/", len(entries))
	}

	// The resolver behind reads and copies must not see the escaped content
	// either, whichever way it is asked.
	resolver := resolverFor(srcDir, src.Pakke)
	if art, ok := resolver.Get(KindAgent, "secret"); ok {
		t.Errorf("resolver resolved %q through a symlinked layout directory", art.AbsPath)
	}
	if items := resolver.List(KindAgent); len(items) != 0 {
		t.Errorf("resolver listed %d agent(s) from outside the checkout", len(items))
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

func TestAdoptSyncSource(t *testing.T) {
	tests := []struct {
		name            string
		configuredSrc   string
		flagSource      string
		stateSourceRepo string
		wantAdopt       string
	}{
		{name: "recorded source wins, nothing to adopt", configuredSrc: "navikt/grillmester", stateSourceRepo: defaultSourceRepo},
		{name: "same source", configuredSrc: defaultSourceRepo, stateSourceRepo: defaultSourceRepo},
		{name: "no configured source", stateSourceRepo: ""},
		{name: "sourceless scope adopts the configured source", configuredSrc: "navikt/grillmester", stateSourceRepo: "", wantAdopt: "navikt/grillmester"},
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

			got, err := adoptSyncSource(scope, tt.flagSource)
			if err != nil {
				t.Fatalf("adoptSyncSource = %v, want no error", err)
			}
			if got != tt.wantAdopt {
				t.Fatalf("adoptSyncSource = %q, want %q", got, tt.wantAdopt)
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
	// Sync leaves both alone: each scope syncs from its own recorded source,
	// so neither has anything to adopt.
	if adopted, err := adoptSyncSource(navScope, ""); err != nil || adopted != "" {
		t.Errorf("adoptSyncSource on a scope with a recorded source = (%q, %v), want (\"\", nil)", adopted, err)
	}
}

// ─── B3 in the interactive paths ─────────────────────────────────────────────

// stubResolveSource points the CLI's source funnel at an already-prepared
// checkout, so interactive flows can be driven without cloning anything.
func stubResolveSource(t *testing.T, src *Source) {
	t.Helper()
	orig := resolveSource
	t.Cleanup(func() { resolveSource = orig })
	resolveSource = func(ref, sourceRepo string) (*Source, error) { return src, nil }
}

// pakkeSource builds a resolved, manifest-bearing source over a fixture tree.
func pakkeSource(t *testing.T, repo string) *Source {
	t.Helper()
	src := &Source{Dir: pakkeSourceTree(t, tier1ManifestJSON), SHA: "def5678" + strings.Repeat("0", 33), Version: "dev", Repo: repo}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke: %v", err)
	}
	return src
}

// TestInteractiveInstallGuardsCrossSource covers B3 in the prompt-driven install
// paths: picking a scope in a TUI is not a way around the cross-source guard.
func TestInteractiveInstallGuardsCrossSource(t *testing.T) {
	tests := []struct {
		name string
		// run installs into a scope whose state records defaultSourceRepo while
		// the configured source is navikt/grillmester.
		run func(t *testing.T, src *Source, home, target string) error
	}{
		{
			name: "repo collection picker",
			run: func(t *testing.T, src *Source, home, target string) error {
				return interactiveRepoInstall(src, ScopeRepo(target), "")
			},
		},
		{
			name: "user item picker",
			run: func(t *testing.T, src *Source, home, target string) error {
				scope, err := ScopeUser()
				if err != nil {
					t.Fatal(err)
				}
				return interactiveUserInstallFromSource(scope, src, "")
			},
		},
		{
			name: "bare install in a repo",
			run: func(t *testing.T, src *Source, home, target string) error {
				stubResolveSource(t, src)
				return cmdInstallInteractive(target, "", "")
			},
		},
		{
			name: "bare install outside a repo",
			run: func(t *testing.T, src *Source, home, target string) error {
				stubResolveSource(t, src)
				return cmdInstallInteractive("", "", "")
			},
		},
		{
			name: "install --user",
			run: func(t *testing.T, src *Source, home, target string) error {
				stubResolveSource(t, src)
				scope, err := ScopeUser()
				if err != nil {
					t.Fatal(err)
				}
				return cmdInstallAll(scope, "", "", false, false, false)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forceNonInteractive = true
			t.Cleanup(func() { forceNonInteractive = false })

			path := isolatedConfig(t)
			mustWrite(t, path, "version = 1\nsource = \"navikt/grillmester\"\n")

			home := t.TempDir()
			t.Setenv("HOME", home)
			target := repoTarget(t)

			userScope, err := ScopeUser()
			if err != nil {
				t.Fatal(err)
			}
			writeGuardState(t, userScope, defaultSourceRepo)
			writeGuardState(t, ScopeRepo(target), defaultSourceRepo)

			err = tt.run(t, pakkeSource(t, "navikt/grillmester"), home, target)
			if err == nil {
				t.Fatal("interactive install from a different source succeeded, want the B3 refusal")
			}
			if !strings.Contains(err.Error(), "will not silently mix") {
				t.Errorf("error %q is not the cross-source refusal", err)
			}
		})
	}
}

// TestInteractiveInstallAllowsExplicitSource is the other half: an explicit
// --source is the user answering the guard's question, so the interactive path
// installs.
func TestInteractiveInstallAllowsExplicitSource(t *testing.T) {
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\nsource = \"navikt/grillmester\"\n")

	target := repoTarget(t)
	scope := ScopeRepo(target)
	writeGuardState(t, scope, defaultSourceRepo)

	src := pakkeSource(t, "navikt/grillmester")
	if err := interactiveRepoInstall(src, scope, "navikt/grillmester"); err != nil {
		t.Fatalf("interactiveRepoInstall with an explicit --source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, ".github", "agents", "grillmester.agent.md")); err != nil {
		t.Errorf("explicit --source install did not write the agent: %v", err)
	}
}

// ─── B2: a cancelled install persists nothing ────────────────────────────────

// TestCancelledInteractiveInstallDoesNotPersistSource covers MAJOR-2: cancelling
// a prompt must be distinguishable from a successful install, or --source gets
// saved for an install that never happened.
func TestCancelledInteractiveInstallDoesNotPersistSource(t *testing.T) {
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n")

	target := repoTarget(t)
	src := pakkeSource(t, "navikt/grillmester")
	stubResolveSource(t, src)

	// promptInstallScope cancelling is the cheapest cancel to simulate: it is
	// the same nil-scope path a user takes by pressing Esc.
	origPrompt := promptInstallScopeFn
	t.Cleanup(func() { promptInstallScopeFn = origPrompt })
	promptInstallScopeFn = func(string) (*InstallScope, error) { return nil, nil }

	err := cmdInstallInteractive(target, "", "navikt/grillmester")
	if !errors.Is(err, errInstallCancelled) {
		t.Fatalf("cancelled install returned %v, want errInstallCancelled", err)
	}

	// finishInstall is what run() wraps every install in: it must turn the
	// sentinel into a clean exit and persist nothing.
	if err := finishInstall(err, "navikt/grillmester", false, true); err != nil {
		t.Errorf("finishInstall(cancelled) = %v, want nil (clean exit)", err)
	}
	if got, _ := configuredSourceRepo(); got != "" {
		t.Errorf("cancelled install persisted source = %q, want none", got)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(data), "source =") {
		t.Errorf("cancelled install wrote a source key:\n%s", data)
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

// TestSameSourceRepoResolvesSymlinks covers the false cross-source refusal: a
// configured path and a recorded symlink to the same checkout are one source.
func TestSameSourceRepoResolvesSymlinks(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "checkout")
	other := filepath.Join(root, "other")
	for _, d := range []string{real, other} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "symlink and its target are the same source", a: link, b: real, want: true},
		{name: "symlink and its target, other way round", a: real, b: link, want: true},
		{name: "symlink into a subpath of the target", a: filepath.Join(link, "."), b: real, want: true},
		{name: "different directories still differ", a: link, b: other},
		{name: "nonexistent paths fall back to the cleaned path", a: filepath.Join(root, "gone"), b: filepath.Join(root, "gone") + "/", want: true},
		{name: "nonexistent paths that differ still differ", a: filepath.Join(root, "gone"), b: filepath.Join(root, "elsewhere")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameSourceRepo(tt.a, tt.b); got != tt.want {
				t.Errorf("sameSourceRepo(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
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

// TestFinishInstall_SingleArtifactDoesNotPersistSource: pulling one artifact
// out of another agentpakke with `install <name> --type <t> --source X` does
// not make X the scope's agentpakke. Persisting it would refuse every later
// plain add (B3) and let sync adopt X for a pre-tracking scope.
func TestFinishInstall_SingleArtifactDoesNotPersistSource(t *testing.T) {
	isolatedConfig(t)

	if err := finishInstall(nil, "navikt/grillmester", false, false); err != nil {
		t.Fatalf("finishInstall = %v, want nil", err)
	}
	if got, _ := configuredSourceRepo(); got != "" {
		t.Errorf("single-artifact install persisted source = %q, want none", got)
	}

	if err := finishInstall(nil, "navikt/grillmester", false, true); err != nil {
		t.Fatalf("finishInstall = %v, want nil", err)
	}
	if got, _ := configuredSourceRepo(); got != "navikt/grillmester" {
		t.Errorf("scope-defining install persisted %q, want %q", got, "navikt/grillmester")
	}
}
