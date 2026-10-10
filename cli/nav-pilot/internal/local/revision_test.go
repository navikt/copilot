package local

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
