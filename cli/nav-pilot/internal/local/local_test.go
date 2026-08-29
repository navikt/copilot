package local

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// modelJSON renders one manifest entry. Tests vary only the fields the rules
// look at; everything else is filler so the entry looks like a real one.
func modelJSON(key, id string, isDefault bool) string {
	return fmt.Sprintf(
		`{"key":%q,"name":"A Model","model":%q,"backend":"mlx-lm","default":%t,"weights_gb":25,"min_ram_gb":48,"params":{"MLX_MODEL":%q}}`,
		key, id, isDefault, id)
}

// paramsJSON renders one default entry whose params are stated verbatim, for
// the rules that look at the server environment rather than at the model id.
func paramsJSON(key, params string) string {
	return fmt.Sprintf(
		`{"key":%q,"name":"A Model","model":%q,"backend":"mlx-lm","default":true,"weights_gb":25,"min_ram_gb":48,"wired_limit_gb":36,"params":%s}`,
		key, okModel, params)
}

// manifestJSON renders a manifest with a raw schema_version literal, so a test
// can hand over a version the type system would not let it build.
func manifestJSON(version string, models ...string) []byte {
	return []byte(fmt.Sprintf(`{"schema_version":%s,"channel":"alpha","models":[%s]}`,
		version, strings.Join(models, ",")))
}

const okModel = "mlx-community/Qwen3.6-35B-A3B-OptiQ-4bit"

// stubFetch replaces the network half of Resolve for the duration of a test, so
// no test ever touches the network.
func stubFetch(t *testing.T, data []byte, err error) {
	t.Helper()
	orig := fetchManifest
	fetchManifest = func(string) ([]byte, error) { return data, err }
	t.Cleanup(func() { fetchManifest = orig })
}

// stubCache points the last-known-good cache at a temp file, seeded with data
// when it is non-nil. Returns the path so a test can read back what was cached.
func stubCache(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "local-models.json")
	if data != nil {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("seeding the cache: %v", err)
		}
	}
	orig := cachePath
	cachePath = func() string { return path }
	t.Cleanup(func() { cachePath = orig })
	return path
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		isErr   bool
		wantErr string // substring the message must carry
	}{
		{
			name: "the contract example",
			data: manifestJSON("1", modelJSON("qwen", okModel, true)),
		},
		{
			// A minor bump adds fields; it does not change what the fields
			// this binary reads mean, so it must keep working.
			name: "unknown minor version is accepted",
			data: manifestJSON("1.7", modelJSON("qwen", okModel, true)),
		},
		{
			// Forward compatibility: the generator must be able to add a field
			// without every developer upgrading nav-pilot first.
			name: "unknown fields are ignored",
			data: []byte(`{"schema_version":1,"channel":"alpha","note":"generated","models":[
				{"key":"qwen","name":"A Model","model":"` + okModel + `","default":true,"tokens_per_second":42}]}`),
		},
		{
			name:    "unknown major version is rejected",
			data:    manifestJSON("2", modelJSON("qwen", okModel, true)),
			isErr:   true,
			wantErr: "schema_version",
		},
		{
			name:    "a missing schema version is rejected",
			data:    []byte(`{"channel":"alpha","models":[` + modelJSON("qwen", okModel, true) + `]}`),
			isErr:   true,
			wantErr: "schema_version",
		},
		{
			// The trust boundary: an unreviewed publisher means unreviewed
			// weights on a developer's machine.
			name:    "a disallowed publisher is rejected",
			data:    manifestJSON("1", modelJSON("sketchy", "some-random-org/Qwen3-4bit", true)),
			isErr:   true,
			wantErr: "allowed publisher",
		},
		{
			name:    "a model id with no publisher is rejected",
			data:    manifestJSON("1", modelJSON("bare", "Qwen3-4bit", true)),
			isErr:   true,
			wantErr: "allowed publisher",
		},
		{
			// The other half of the same boundary: params become the server
			// process's environment, so PYTHONPATH runs code the manifest
			// chose before the model id is ever looked at.
			name:    "a param that injects code into the process is rejected",
			data:    manifestJSON("1", paramsJSON("injected", `{"MLX_MODEL":"x","PYTHONPATH":"/tmp/evil"}`)),
			isErr:   true,
			wantErr: "PYTHONPATH",
		},
		{
			// The allow-list validates the model *name*; the environment
			// decides which host serves the bytes under it.
			name:    "a param that redirects the weights download is rejected",
			data:    manifestJSON("1", paramsJSON("redirected", `{"HF_ENDPOINT":"https://weights.example.com"}`)),
			isErr:   true,
			wantErr: "MLX_ namespace",
		},
		{
			name:    "a param outside the MLX_ namespace by case is rejected",
			data:    manifestJSON("1", paramsJSON("lower", `{"mlx_top_k":"20"}`)),
			isErr:   true,
			wantErr: "mlx_top_k",
		},
		{
			name: "the generator's own MLX_ knobs are accepted",
			data: manifestJSON("1", paramsJSON("knobs", `{"MLX_MODEL":"x","MLX_TOP_P":"0.95","MLX_CACHE_BYTES":"12884901888"}`)),
		},
		{
			name: "the second allowed publisher is accepted",
			data: manifestJSON("1", modelJSON("lms", "lmstudio-community/Qwen3-4bit", true)),
		},
		{
			// One bad entry refuses the whole file: a manifest carrying an
			// entry this binary would not run is not the file the generator
			// meant to publish.
			name: "one disallowed entry rejects the whole manifest",
			data: manifestJSON("1",
				modelJSON("qwen", okModel, true),
				modelJSON("sketchy", "some-random-org/Qwen3-4bit", false)),
			isErr:   true,
			wantErr: "sketchy",
		},
		{
			name:    "an invalid model id is rejected",
			data:    manifestJSON("1", modelJSON("spaced", "mlx-community/Qwen3 4bit", true)),
			isErr:   true,
			wantErr: "not a valid identifier",
		},
		{
			name:    "zero defaults is rejected",
			data:    manifestJSON("1", modelJSON("qwen", okModel, false)),
			isErr:   true,
			wantErr: "exactly one",
		},
		{
			name: "two defaults is rejected",
			data: manifestJSON("1",
				modelJSON("qwen", okModel, true),
				modelJSON("lms", "lmstudio-community/Qwen3-4bit", true)),
			isErr:   true,
			wantErr: "exactly one",
		},
		{
			name:    "an empty model list has no default",
			data:    manifestJSON("1"),
			isErr:   true,
			wantErr: "exactly one",
		},
		{
			name:    "not JSON",
			data:    []byte("<html>404</html>"),
			isErr:   true,
			wantErr: "not valid JSON",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.data)
			if tc.isErr {
				if err == nil {
					t.Fatalf("Parse() = %+v, want error", got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("Parse() error = %q, want it to mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() errored: %v", err)
			}
			if len(got.Models) == 0 {
				t.Fatal("Parse() returned no models")
			}
		})
	}
}

// The developer reading this error cannot fix the served file, so it has to
// name the version they have, the versions this binary reads, and the fact that
// the session keeps working from the cache.
func TestUnknownMajorErrorNamesBothVersionsAndTheFallback(t *testing.T) {
	_, err := Parse(manifestJSON("2", modelJSON("qwen", okModel, true)))
	if err == nil {
		t.Fatal("Parse() accepted schema_version 2")
	}
	for _, want := range []string{`"2"`, "1", "cached"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveCachesASuccessfulFetch(t *testing.T) {
	served := manifestJSON("1", modelJSON("qwen", okModel, true))
	stubFetch(t, served, nil)
	path := stubCache(t, nil)

	m, src, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() reported %v, want a clean fetch", err)
	}
	if src != SourceNetwork {
		t.Errorf("Resolve() source = %q, want %q", src, SourceNetwork)
	}
	if m.Models[0].Model != okModel {
		t.Errorf("Resolve() returned %q, want the served model", m.Models[0].Model)
	}
	cached, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("the fetch did not write the cache: %v", readErr)
	}
	if string(cached) != string(served) {
		t.Errorf("cached %q, want the served bytes", cached)
	}
}

func TestResolveFallsBackToTheCache(t *testing.T) {
	cachedModel := "mlx-community/Cached-4bit"
	tests := []struct {
		name  string
		data  []byte
		err   error
		wantE string
	}{
		{
			name: "the network is unreachable",
			err:  errors.New("dial tcp: no route to host"),
		},
		{
			// Rule 1's actual behaviour, not only its wording: a manifest on a
			// schema this binary cannot read is not used.
			name:  "the served manifest is on an unknown major",
			data:  manifestJSON("2", modelJSON("qwen", okModel, true)),
			wantE: "schema_version",
		},
		{
			name:  "the served manifest breaks a rule",
			data:  manifestJSON("1", modelJSON("sketchy", "some-random-org/Qwen3-4bit", true)),
			wantE: "allowed publisher",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubFetch(t, tc.data, tc.err)
			seeded := manifestJSON("1", modelJSON("cached", cachedModel, true))
			path := stubCache(t, seeded)

			m, src, err := Resolve()
			if src != SourceCache {
				t.Fatalf("Resolve() source = %q, want %q", src, SourceCache)
			}
			if m.Models[0].Model != cachedModel {
				t.Errorf("Resolve() returned %q, want the cached model", m.Models[0].Model)
			}
			// The fallback is reported, never silent: the caller has to be
			// able to say the manifest may be stale.
			if err == nil {
				t.Fatal("Resolve() fell back to the cache without reporting it")
			}
			if !strings.Contains(err.Error(), "cached") {
				t.Errorf("Resolve() error = %q, want it to say the cache was used", err)
			}
			if tc.wantE != "" && !strings.Contains(err.Error(), tc.wantE) {
				t.Errorf("Resolve() error = %q, want it to mention %q", err, tc.wantE)
			}
			// The refused manifest must not have replaced the good cache.
			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("reading the cache back: %v", readErr)
			}
			if string(after) != string(seeded) {
				t.Errorf("the cache was overwritten with %s, want the last known good manifest", after)
			}
		})
	}
}

func TestResolveFallsBackToTheEmbeddedManifest(t *testing.T) {
	stubFetch(t, nil, errors.New("offline"))
	stubCache(t, nil) // no cache file exists

	m, src, err := Resolve()
	if src != SourceEmbedded {
		t.Fatalf("Resolve() source = %q, want %q", src, SourceEmbedded)
	}
	if m == nil || len(m.Models) == 0 {
		t.Fatal("Resolve() returned no embedded models")
	}
	if err == nil {
		t.Error("Resolve() fell back to the embedded manifest without reporting it")
	}
}

func TestResolveIgnoresAnUnparsableCache(t *testing.T) {
	stubFetch(t, nil, errors.New("offline"))
	stubCache(t, []byte("{ truncated"))

	m, src, err := Resolve()
	if src != SourceEmbedded {
		t.Fatalf("Resolve() source = %q, want %q", src, SourceEmbedded)
	}
	if m == nil || len(m.Models) == 0 {
		t.Fatal("Resolve() returned no embedded models")
	}
	if err == nil {
		t.Error("Resolve() fell back without reporting it")
	}
}

// The embedded copy is the last fallback, so a defect in it only shows up on a
// developer's machine with no network and no cache — unless a test holds it to
// the same rules as a served one.
func TestEmbeddedManifestIsValid(t *testing.T) {
	m, err := Parse(embeddedManifest)
	if err != nil {
		t.Fatalf("the embedded manifest does not pass validation: %v", err)
	}
	if len(m.Models) == 0 {
		t.Fatal("the embedded manifest names no models")
	}
}

func TestIsLocalIsFalseUntilLocalIsEnabled(t *testing.T) {
	m, err := Parse(embeddedManifest)
	if err != nil {
		t.Fatalf("embedded manifest: %v", err)
	}
	SetActive(m)
	t.Cleanup(func() { SetActive(nil); SetEnabled(false) })

	SetEnabled(false)
	if IsLocal(m.Models[0].Model) {
		t.Error("IsLocal answered true with local dispatch disabled — a developer who never opted in would be routed to a local model")
	}
	if _, ok := Lookup(m.Models[0].Model); !ok {
		t.Error("Lookup must stay ungated: the commands that enable local have to read the manifest first")
	}
	SetEnabled(true)
	if !IsLocal(m.Models[0].Model) {
		t.Error("IsLocal answered false with local dispatch enabled")
	}
}

func TestLookupAndIsLocal(t *testing.T) {
	m, err := Parse(manifestJSON("1", modelJSON("qwen", okModel, true)))
	if err != nil {
		t.Fatalf("Parse() errored: %v", err)
	}
	SetActive(m)
	SetEnabled(true)
	t.Cleanup(func() { SetActive(nil); SetEnabled(false) })

	if !IsLocal(okModel) {
		t.Errorf("IsLocal(%q) = false, want true", okModel)
	}
	if IsLocal("claude-opus-5") {
		t.Error("IsLocal(\"claude-opus-5\") = true, want false")
	}
	got, ok := Lookup(okModel)
	if !ok {
		t.Fatalf("Lookup(%q) missed", okModel)
	}
	if got.Key != "qwen" || got.Params["MLX_MODEL"] != okModel {
		t.Errorf("Lookup(%q) = %+v, want the manifest entry", okModel, got)
	}

	// A nil manifest restores the embedded copy rather than emptying the
	// predicate.
	SetActive(nil)
	if len(Active().Models) == 0 {
		t.Error("SetActive(nil) left no models, want the embedded manifest")
	}
}
