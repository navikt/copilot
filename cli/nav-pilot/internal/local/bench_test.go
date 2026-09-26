package local

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const benchModel = "Accio-Lab/occamy-1.0-MLX-4bit"

// captureBenchWarnings collects what the override prints, fresh per test.
func captureBenchWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	orig := benchWarnings
	benchWarnings = &buf
	benchWarnSeen = map[string]bool{}
	t.Cleanup(func() { benchWarnings = orig; benchWarnSeen = map[string]bool{} })
	return &buf
}

func TestBenchOverride(t *testing.T) {
	unvetted := manifestJSON("1", modelJSON("occamy", benchModel, true))

	t.Run("manifest and orgs together accept the publisher, and say so", func(t *testing.T) {
		warn := captureBenchWarnings(t)
		path := filepath.Join(t.TempDir(), "bench.json")
		if err := os.WriteFile(path, unvetted, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv(BenchManifestEnv, path)
		t.Setenv(BenchAllowOrgsEnv, " Accio-Lab ")
		stubFetch(t, nil, os.ErrDeadlineExceeded)
		cached := manifestJSON("1", modelJSON("qwen", okModel, true))
		cache := stubCache(t, cached)
		for name, get := range map[string]func() (*Manifest, Source, error){"Cached": Cached, "Resolve": Resolve} {
			m, src, err := get()
			if err != nil || m == nil || src != SourceBench || m.Models[0].Model != benchModel {
				t.Fatalf("%s: got %v %q %v, want the bench manifest", name, m, src, err)
			}
		}
		if got, _ := os.ReadFile(cache); !bytes.Equal(got, cached) {
			t.Fatalf("the bench manifest reached the cache: %s", got)
		}
		if got := warn.String(); got != "bench override: allowing unvetted publisher Accio-Lab\n" {
			t.Fatalf("warning = %q", got)
		}
	})

	t.Run("orgs without the bench manifest are ignored", func(t *testing.T) {
		warn := captureBenchWarnings(t)
		t.Setenv(BenchManifestEnv, "")
		t.Setenv(BenchAllowOrgsEnv, "Accio-Lab")
		// The served manifest and the cache both name the unvetted model: the
		// network must never be able to use the override.
		stubFetch(t, unvetted, nil)
		stubCache(t, unvetted)
		m, src, err := Resolve()
		if src != SourceEmbedded || m == nil || err == nil || !strings.Contains(err.Error(), "allowed publisher") {
			t.Fatalf("Resolve = %q %v, want the embedded copy and the publisher refusal", src, err)
		}
		if strings.Contains(warn.String(), "allowing") || !strings.Contains(warn.String(), "is ignored without "+BenchManifestEnv) {
			t.Fatalf("warning = %q", warn.String())
		}
	})

	t.Run("the bench manifest without orgs keeps the allow-list", func(t *testing.T) {
		captureBenchWarnings(t)
		path := filepath.Join(t.TempDir(), "bench.json")
		if err := os.WriteFile(path, unvetted, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv(BenchManifestEnv, path)
		t.Setenv(BenchAllowOrgsEnv, "")
		m, src, err := Cached()
		if m != nil || src != SourceBench || err == nil || !strings.Contains(err.Error(), "allowed publisher") {
			t.Fatalf("Cached = %v %q %v, want a refusal and no fallback", m, src, err)
		}
	})

	t.Run("the orgs widen the publisher rule and nothing else", func(t *testing.T) {
		captureBenchWarnings(t)
		t.Setenv(BenchAllowOrgsEnv, "Accio-Lab")
		for name, entry := range map[string]string{
			"backend": strings.Replace(modelJSON("occamy", benchModel, true), `"mlx-lm"`, `"sh"`, 1),
			"param":   strings.Replace(modelJSON("occamy", benchModel, true), `"MLX_MODEL"`, `"PYTHONPATH"`, 1),
		} {
			path := filepath.Join(t.TempDir(), "bench.json")
			if err := os.WriteFile(path, manifestJSON("1", entry), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv(BenchManifestEnv, path)
			if m, _, err := Cached(); m != nil || err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("%s: Cached = %v %v, want a refusal", name, m, err)
			}
		}
	})

	t.Run("a missing bench manifest is an error, not the default model", func(t *testing.T) {
		captureBenchWarnings(t)
		t.Setenv(BenchManifestEnv, filepath.Join(t.TempDir(), "nope.json"))
		if m, _, err := Cached(); m != nil || err == nil {
			t.Fatalf("Cached = %v %v, want nil and an error", m, err)
		}
	})
}
