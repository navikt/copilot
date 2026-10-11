package local

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

const testRev = "70a3aa32c7feef511182bf16aa332f37e8d82014"

func revManifest(rev string) []byte {
	return []byte(`{"schema_version":1,"channel":"alpha","models":[
		{"key":"qwen","name":"A Model","model":"` + okModel + `","default":true,"revision":"` + rev + `"}]}`)
}

func TestRevision(t *testing.T) {
	m, err := Parse(revManifest(testRev))
	if err != nil {
		t.Fatalf("Parse() with a commit SHA errored: %v", err)
	}
	if got := m.Models[0].Revision; got != testRev {
		t.Errorf("Revision = %q, want %q", got, testRev)
	}
	for _, bad := range []string{"main", "v1.0", testRev[:12], strings.ToUpper(testRev), testRev + "; rm"} {
		if _, err := Parse(revManifest(bad)); err == nil || !strings.Contains(err.Error(), "commit SHA") {
			t.Errorf("Parse() with revision %q: err = %v, want a commit SHA refusal", bad, err)
		}
	}

	stubDirs(t)
	var got []string
	orig := runStreaming
	runStreaming = func(_ context.Context, _ string, args []string, _ []string, _ func(string)) error {
		got = args
		return nil
	}
	t.Cleanup(func() { runStreaming = orig })
	for rev, want := range map[string][]string{
		"":      {"download", okModel},
		testRev: {"download", okModel, "--revision", testRev},
	} {
		if err := DownloadWeights(context.Background(), okModel, rev, nil); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, want) {
			t.Errorf("DownloadWeights(rev %q) args = %q, want %q", rev, got, want)
		}
	}

	// Weights from another commit do not satisfy a pin; unpinned, any do.
	snap := filepath.Join(modelCacheDir(okModel), "snapshots", "0123456789abcdef0123456789abcdef01234567")
	if err := os.MkdirAll(snap, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model.safetensors", "config.json"} {
		if err := os.WriteFile(filepath.Join(snap, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if ok, _ := WeightsPresent(okModel, ""); !ok {
		t.Error("WeightsPresent(unpinned) = false with a complete snapshot")
	}
	if ok, _ := WeightsPresent(okModel, testRev); ok {
		t.Error("WeightsPresent(pinned) = true for another commit's snapshot")
	}

	if s := serverScript(Model{Model: okModel}); !strings.Contains(s, `_MODEL, _REVISION = "`+okModel+`", None`) {
		t.Errorf("unpinned serverScript does not set _REVISION to None:\n%s", s[:120])
	}
	if s := serverScript(Model{Model: okModel, Revision: testRev}); !strings.Contains(s, `_MODEL, _REVISION = "`+okModel+`", "`+testRev+`"`) {
		t.Errorf("pinned serverScript does not carry the revision:\n%s", s[:160])
	}
}

// A replaced id runs with the replacement's params, never its pin: the SHA
// names a commit in the replacement's repository.
func TestLegacyDropsRevision(t *testing.T) {
	m := nudgeManifest(t, `,"revision":"`+testRev+`"`, replacedMap)
	old, repl, ok := m.legacy(oldID)
	if !ok {
		t.Fatal("legacy() found no replacement")
	}
	if repl.Revision != testRev || old.Revision != "" {
		t.Errorf("legacy() revisions = old %q, repl %q; want old empty, repl %q", old.Revision, repl.Revision, testRev)
	}
}

// The pinned script loads the primary model at the pin and any other id
// (a draft model) as before, run against a fake mlx_lm.server.
func TestServerScriptPinsLoad(t *testing.T) {
	py := testhome.Python3(t)
	pkg := filepath.Join(t.TempDir(), "mlx_lm")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := `class APIHandler:
    def do_POST(self): pass
def load(p, **k):
    print(p, k.get("revision"))
def main():
    load("` + okModel + `")
    load("other/draft")
    return 0
`
	for name, body := range map[string]string{"__init__.py": "", "server.py": fake} {
		if err := os.WriteFile(filepath.Join(pkg, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(py, "-c", serverScript(Model{Model: okModel, Revision: testRev}))
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Dir(pkg))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pinned script failed: %v\n%s", err, out)
	}
	if want := okModel + " " + testRev + "\nother/draft None\n"; string(out) != want {
		t.Errorf("load calls = %q, want %q", out, want)
	}
}

// A recorded server on another commit of the chosen model is stale; an old
// state file without a revision loads and reads as unpinned.
func TestStaleRevision(t *testing.T) {
	var st State
	if err := json.Unmarshal([]byte(`{"pid":1,"model":"`+okModel+`"}`), &st); err != nil {
		t.Fatal(err)
	}
	if StaleRevision(st, Model{Model: okModel}) != "" {
		t.Error("unpinned model flagged against an old state file")
	}
	if StaleRevision(st, Model{Model: okModel, Revision: testRev}) == "" {
		t.Error("a new pin was not flagged against a server without one")
	}
	st.Revision = testRev
	if StaleRevision(st, Model{Model: okModel, Revision: testRev}) != "" {
		t.Error("matching revision flagged")
	}
	if StaleRevision(st, Model{Model: "other/model"}) != "" {
		t.Error("a different model is not a revision mismatch")
	}
}
