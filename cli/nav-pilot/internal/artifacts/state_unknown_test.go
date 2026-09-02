package artifacts

import (
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

// TestStateRewriteIsByteStable guards against a spurious diff in every Nav repo:
// writing an unchanged state twice must produce identical bytes.
func TestStateRewriteIsByteStable(t *testing.T) {
	scope, path := writeStateFixture(t, stateWithUnknownKeys)

	state, err := ReadScopedState(scope)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := WriteScopedState(scope, state); err != nil {
		t.Fatalf("write: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	again, err := ReadScopedState(scope)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if err := WriteScopedState(scope, again); err != nil {
		t.Fatalf("re-write: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("write is not byte-stable:\n--- first ---\n%s\n--- second ---\n%s", first, second)
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
