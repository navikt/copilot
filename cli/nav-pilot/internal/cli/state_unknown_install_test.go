package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stateWithUnknownKeys is a repo state file as a newer nav-pilot wrote it: a
// top-level key and a per-file key this binary does not know, on the two paths
// legacySourceTree installs.
const stateWithUnknownKeys = `{
  "collection": "fullstack",
  "version": "dev",
  "scope": "repo",
  "source_repo": "navikt/copilot",
  "source_sha": "old1234",
  "installed_at": "2026-01-01T00:00:00Z",
  "files": [
    {
      "path": ".github/agents/test-a.agent.md",
      "hash": "sha256:aaa",
      "installed_by": "2.0.0"
    },
    {
      "path": ".github/skills/test-s/",
      "hash": "sha256:bbb"
    }
  ],
  "schema_version": 2
}
`

func seedState(t *testing.T, scope *InstallScope, body string) {
	t.Helper()
	path := scope.StatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertStateKeeps(t *testing.T, scope *InstallScope, want ...string) {
	t.Helper()
	out, err := os.ReadFile(scope.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range want {
		if !strings.Contains(string(out), w) {
			t.Errorf("rewritten state lost %s\n%s", w, out)
		}
	}
}

// TestInstallOverExistingStateKeepsUnknownKeys is #588 through `install`: the
// state is rebuilt from scratch, not read-modify-written, so the keys a newer
// nav-pilot recorded have to be carried over explicitly.
func TestInstallOverExistingStateKeepsUnknownKeys(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	scope := ScopeRepo(repoTarget(t))
	seedState(t, scope, stateWithUnknownKeys)

	src := &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	if err := cmdInstallFromSource("fullstack", src, scope, false, true, false); err != nil {
		t.Fatalf("cmdInstallFromSource: %v", err)
	}
	assertStateKeeps(t, scope, `"schema_version": 2`, `"installed_by": "2.0.0"`)
}

// TestInstallAllOverExistingStateKeepsUnknownKeys is the same for `install`
// without a collection, which builds its StateFile at a second site.
func TestInstallAllOverExistingStateKeepsUnknownKeys(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	scope := ScopeRepo(repoTarget(t))
	seedState(t, scope, stateWithUnknownKeys)

	src := &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	if err := installAllFromSource(scope, src, nil, false, true, false); err != nil {
		t.Fatalf("installAllFromSource: %v", err)
	}
	assertStateKeeps(t, scope, `"schema_version": 2`, `"installed_by": "2.0.0"`)
}

// TestPinRevisionKeepsUnknownKeys covers the Tier 2 pin: it reads the state it
// replaces for the source-switch cleanup, and used to throw the rest away.
func TestPinRevisionKeepsUnknownKeys(t *testing.T) {
	scope := pinEnv(t)
	seedState(t, scope, `{
  "collection": "grillmester",
  "version": "dev",
  "scope": "user",
  "source_repo": "navikt/grillmester",
  "source_sha": "old1234",
  "installed_at": "2026-01-01T00:00:00Z",
  "schema_version": 2
}
`)

	src := &Source{Dir: tier2SourceTree(t), SHA: "abc1234", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	if _, err := pinRevision(scope, src, true); err != nil {
		t.Fatalf("pinRevision: %v", err)
	}
	assertStateKeeps(t, scope, `"schema_version": 2`)
}
