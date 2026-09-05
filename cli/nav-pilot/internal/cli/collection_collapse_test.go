package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// ─── the five collections collapse into the one agentpakke (#468) ────────────

// collapsedManifestJSON is the shape navikt/copilot ships after the collapse:
// one pakke, canonical layout, no collections/.
const collapsedManifestJSON = `{
  "contractVersion": "1",
  "name": "nav-pilot",
  "description": "Nav's default agents, skills, instructions, and prompts",
  "clients": {"copilot": {"primaryAgents": ["nav-pilot"]}},
  "layout": {"agents": "agents", "skills": "skills"}
}`

// collapseSource turns a legacy source tree into the post-collapse one: the
// manifest appears, and the pool holds artifacts no collection ever named.
func collapseSource(t *testing.T, dir string) {
	t.Helper()
	mustWrite(t, filepath.Join(dir, ".nav-pilot", "agentpakke.json"), collapsedManifestJSON)
	mustWrite(t, filepath.Join(dir, "agents", "test-b.agent.md"), "---\nname: test-b\ndescription: B\n---\nBody B\n")
	mustWrite(t, filepath.Join(dir, "skills", "test-t", "SKILL.md"), "# Skill T\n")
}

// syncFrom points sync at dir as the default source, manifest attached.
func syncFrom(t *testing.T, dir string) {
	t.Helper()
	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
		src := &Source{Dir: dir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
		return src, attachPakke(src)
	}
}

// TestSyncAdoptsCollectionScopeIntoPakke: a scope installed from a collection
// whose source now ships an agentpakke manifest is rewritten to the pakke
// identity — loudly. The user's files are untouched, and the rest of the pool
// is recorded as ignored so the install they chose stays exactly the install
// they have (the #465 §4 migration).
func TestSyncAdoptsCollectionScopeIntoPakke(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)

	src := &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	captureStdoutFor(t, func() {
		if err := cmdInstallFromSource("fullstack", src, scope, false, false, false); err != nil {
			t.Fatalf("legacy install: %v", err)
		}
	})

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != nil {
		t.Fatalf("cmdSync = %v, want nil (files are unchanged)", err)
	}
	if !strings.Contains(out, "fullstack") || !strings.Contains(out, "nav-pilot") {
		t.Errorf("adoption is silent; output must name both identities:\n%s", out)
	}

	state, _ := readScopedState(scope)
	if state == nil || state.Collection != "nav-pilot" {
		t.Fatalf("state.Collection = %v, want the pakke name", state)
	}
	wantIgnored := map[string]bool{
		".github/agents/test-b.agent.md": false,
		".github/skills/test-t/":         false,
	}
	for _, f := range state.Files {
		if _, ok := wantIgnored[f.Path]; ok {
			wantIgnored[f.Path] = f.Status == fileStatusIgnored
			continue
		}
		if f.Status == fileStatusIgnored {
			t.Errorf("adoption ignored %q, an artifact the user had installed", f.Path)
		}
	}
	for path, ok := range wantIgnored {
		if !ok {
			t.Errorf("%q is not recorded as ignored after adoption", path)
		}
	}
	if strings.Contains(out, "new item(s)") {
		t.Errorf("adoption still reports the ignored pool as new items:\n%s", out)
	}

	// The adoption happens once: a second sync is quiet about it.
	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != nil {
		t.Fatalf("second cmdSync = %v", err)
	}
	if strings.Contains(out, "fullstack") {
		t.Errorf("second sync still talks about the retired collection:\n%s", out)
	}
}

// TestSyncAdoptsAllInstallWithoutIgnores: "(all)" meant everything, so its
// adoption is a rename only — pool growth keeps being reported as new items,
// exactly as before the manifest shipped.
func TestSyncAdoptsAllInstallWithoutIgnores(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(scope.RootDir, "agents", "test-a.agent.md"),
		"---\nname: test-a\ndescription: A\n---\nBody A\n")
	if err := writeScopedState(scope, &StateFile{
		Collection: CollectionAll,
		Version:    "dev",
		Scope:      scope.Name,
		SourceRepo: defaultSourceRepo,
		SourceSHA:  "abc1234",
		Files:      []InstalledFile{{Path: "agents/test-a.agent.md"}},
	}); err != nil {
		t.Fatal(err)
	}

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	var syncErr error
	out := captureStdoutFor(t, func() { syncErr = cmdSync(scope, "", "", false, false) })
	if syncErr != nil {
		t.Fatalf("cmdSync = %v", syncErr)
	}

	state, _ := readScopedState(scope)
	if state == nil || state.Collection != "nav-pilot" {
		t.Fatalf("state.Collection = %v, want the pakke name", state)
	}
	for _, f := range state.Files {
		if f.Status == fileStatusIgnored {
			t.Errorf("an (all) adoption must not ignore anything, got %q ignored", f.Path)
		}
	}
	if !strings.Contains(out, "new item(s)") {
		t.Errorf("(all) scope no longer hears about pool growth:\n%s", out)
	}
}

// TestSyncLeavesALaCarteScopeAlone: an à-la-carte install never claimed a
// collection, so the collapse has nothing to migrate.
func TestSyncLeavesALaCarteScopeAlone(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)
	mustWrite(t, filepath.Join(target, ".github", "agents", "test-a.agent.md"),
		"---\nname: test-a\ndescription: A\n---\nBody A\n")
	if err := writeScopedState(scope, &StateFile{
		Collection: "(à la carte)",
		Version:    "dev",
		Scope:      scope.Name,
		SourceRepo: defaultSourceRepo,
		SourceSHA:  "abc1234",
		Files:      []InstalledFile{{Path: ".github/agents/test-a.agent.md"}},
	}); err != nil {
		t.Fatal(err)
	}

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	captureStdoutFor(t, func() {
		if err := cmdSync(scope, "", "", false, false); err != nil {
			t.Fatalf("cmdSync = %v", err)
		}
	})

	state, _ := readScopedState(scope)
	if state == nil || state.Collection != "(à la carte)" {
		t.Fatalf("state.Collection = %v, want the à-la-carte label untouched", state)
	}
	if len(state.Files) != 1 || state.Files[0].Status != "" {
		t.Fatalf("state.Files = %+v, want the single active entry untouched", state.Files)
	}
}

// TestInstallNamesTheFoldForLegacyCollection: `nav-pilot install frontend`
// against the post-collapse source refuses with the mapping, not a bare "not
// found" — the one command every collection user has in muscle memory.
func TestInstallNamesTheFoldForLegacyCollection(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	collapseSource(t, srcDir)

	orig := resolveSource
	t.Cleanup(func() { resolveSource = orig })
	resolveSource = func(ref, sourceRepo string) (*Source, error) {
		src := &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
		return src, attachPakke(src)
	}

	scope := ScopeRepo(repoTarget(t))
	var err error
	captureStdoutFor(t, func() {
		err = cmdInstallAuto("frontend", "", scope, "", "", false, false, false)
	})
	if err == nil {
		t.Fatal("install frontend succeeded against a source that no longer ships it")
	}
	for _, want := range []string{"frontend", "nav-pilot install nav-pilot"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err, want)
		}
	}
}
