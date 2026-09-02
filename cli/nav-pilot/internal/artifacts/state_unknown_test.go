package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// stateWithUnknownKeys is a repo-scope state file as a newer nav-pilot would
// write it: a top-level key and a per-file key that this binary does not know.
const stateWithUnknownKeys = `{
  "collection": "all",
  "version": "1.2.3",
  "scope": "repo",
  "source_repo": "navikt/copilot",
  "source_sha": "abc123",
  "installed_at": "2026-01-01T00:00:00Z",
  "files": [
    {
      "path": ".github/agents/foo.md",
      "hash": "sha256:aaa",
      "source": "navikt/annen-pakke",
      "installed_by": "2.0.0"
    }
  ],
  "schema_version": 2
}
`

func writeStateFixture(t *testing.T, body string) (*domain.InstallScope, string) {
	t.Helper()
	dir := t.TempDir()
	scope := domain.ScopeRepo(dir)
	path := scope.StatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return scope, path
}

// TestStateRewritePreservesUnknownKeys is the #588 regression: a read-modify-write
// cycle must not drop keys the running binary does not understand, neither inside
// a files entry nor at the top level.
func TestStateRewritePreservesUnknownKeys(t *testing.T) {
	scope, path := writeStateFixture(t, stateWithUnknownKeys)

	state, err := ReadScopedState(scope)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// A mutation of the kind sync/ignore/add make.
	state.SourceSHA = "def456"
	state.Files[0].Status = domain.FileStatusIgnored
	if err := WriteScopedState(scope, state); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		`"source": "navikt/annen-pakke"`,
		`"installed_by": "2.0.0"`,
		`"schema_version": 2`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewritten state lost %s\n%s", want, got)
		}
	}
	if !strings.Contains(got, `"source_sha": "def456"`) || !strings.Contains(got, `"status": "ignored"`) {
		t.Errorf("rewritten state lost the mutation\n%s", got)
	}
}

// realCommittedState is a verbatim copy of a checked-in .github/.nav-pilot-state.json
// from a Nav repo (navikt/copilot-intern), non-ASCII collection name and all. The
// bytes matter: this is what the byte-stability test is actually protecting.
const realCommittedState = `{
  "collection": "(à la carte)",
  "version": "dev",
  "scope": "repo",
  "source_sha": "385e6f8",
  "installed_at": "2026-05-06T09:41:57Z",
  "files": [
    {
      "path": ".github/agents/forfatter.agent.md",
      "hash": "4e55f4d189619dcb"
    },
    {
      "path": ".github/agents/forfatter.metadata.json",
      "hash": "415648fcdc323588"
    }
  ]
}
`

// TestStateRewriteIsByteStable guards against a spurious diff in every Nav repo:
// reading a state file and writing it straight back must reproduce the original
// bytes exactly. Comparing two of our own writes would pass even if both differed
// from what is on disk, which is the diff a colleague would actually see.
func TestStateRewriteIsByteStable(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"real committed state", realCommittedState},
		{"with unknown keys", stateWithUnknownKeys},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scope, path := writeStateFixture(t, tt.body)

			state, err := ReadScopedState(scope)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if err := WriteScopedState(scope, state); err != nil {
				t.Fatalf("write: %v", err)
			}
			out, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != tt.body {
				t.Errorf("rewrite changed the file:\n--- on disk ---\n%s\n--- rewritten ---\n%s", tt.body, out)
			}
		})
	}
}

// TestStatePredatingPerFileSourceRoundTrips is the other half of #588: state as an
// older nav-pilot wrote it (no source key anywhere) must survive untouched.
func TestStatePredatingPerFileSourceRoundTrips(t *testing.T) {
	const old = `{
  "collection": "all",
  "version": "1.0.0",
  "scope": "repo",
  "source_sha": "abc123",
  "installed_at": "2026-01-01T00:00:00Z",
  "files": [
    {
      "path": ".github/agents/foo.md",
      "hash": "sha256:aaa"
    }
  ]
}
`
	scope, path := writeStateFixture(t, old)
	state, err := ReadScopedState(scope)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := WriteScopedState(scope, state); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != old {
		t.Errorf("round trip changed the file:\n--- want ---\n%s\n--- got ---\n%s", old, out)
	}
}

// TestSyncOpenCodeArtifactsKeepsUnknownKeys is #588 through the opencode sync:
// every nav-pilot-owned entry is rebuilt from the source on each run, so a
// per-file key on a path the sync still owns — and the top-level one — has to be
// carried over from the state being replaced.
func TestSyncOpenCodeArtifactsKeepsUnknownKeys(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	if _, _, _, _, _, err := SyncOpenCodeArtifacts(sourceDir, "", outputDir, "1.0.0", "abc123", ""); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	// Stamp the written state the way a newer nav-pilot would have.
	path := filepath.Join(outputDir, ".nav-pilot-state.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	doc["schema_version"] = json.RawMessage(`2`)
	var files []map[string]json.RawMessage
	if err := json.Unmarshal(doc["files"], &files); err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("first sync recorded no files")
	}
	stamped := string(files[0]["path"])
	files[0]["installed_by"] = json.RawMessage(`"2.0.0"`)
	if doc["files"], err = json.Marshal(files); err != nil {
		t.Fatal(err)
	}
	if raw, err = json.Marshal(doc); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, _, _, _, err := SyncOpenCodeArtifacts(sourceDir, "", outputDir, "1.0.1", "def456", ""); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	state, err := ReadOpenCodeState(outputDir)
	if err != nil || state == nil {
		t.Fatalf("ReadOpenCodeState = (%v, %v)", state, err)
	}
	if _, ok := state.Unknown["schema_version"]; !ok {
		t.Errorf("sync dropped the top-level schema_version key: %+v", state.Unknown)
	}
	for _, f := range state.Files {
		if `"`+f.Path+`"` != stamped {
			continue
		}
		if _, ok := f.Unknown["installed_by"]; !ok {
			t.Errorf("sync dropped installed_by from %s, an entry it rebuilt: %+v", f.Path, f.Unknown)
		}
		return
	}
	t.Errorf("the stamped path %s is gone from the state entirely", stamped)
}
