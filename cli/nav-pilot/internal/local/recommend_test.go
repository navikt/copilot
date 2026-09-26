package local

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

const (
	oldID           = "mlx-community/Qwen3.8-27B-4bit"
	newID           = "mlx-community/Qwen3.8-27B-OptiQ-4bit"
	replacementSlug = "q38-optiq"
)

// nudgeManifest has a default recommended for untrusted-evidence decide, the
// replacement entry (plain, or with extra fields such as a min_nav_pilot), and
// whatever top-level suffix the test passes, normally the replaced map.
func nudgeManifest(t *testing.T, replacementExtra string, replaced string) *Manifest {
	t.Helper()
	data := `{"schema_version":1,"channel":"alpha","models":[
		{"key":"q36-default","name":"Default","model":"` + okModel + `","backend":"mlx-lm","default":true,"params":{"MLX_MODEL":"` + okModel + `"},
		 "recommended_for":["decide-untrusted-evidence","from-the-future"]},
		{"key":"` + replacementSlug + `","name":"OptiQ","model":"` + newID + `","backend":"mlx-lm","weights_gb":19,"params":{"MLX_MODEL":"` + newID + `"}` + replacementExtra + `}]` +
		replaced + `}`
	m, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return m
}

const replacedMap = `,"replaced":{"` + oldID + `":"` + replacementSlug + `"}`

// fakeWeights puts a complete download of id into the Hugging Face cache.
func fakeWeights(t *testing.T, hf, id string) {
	t.Helper()
	snap := filepath.Join(hf, "hub", "models--"+strings.ReplaceAll(id, "/", "--"), "snapshots", "abc")
	if err := os.MkdirAll(snap, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"config.json", "model.safetensors"} {
		if err := os.WriteFile(filepath.Join(snap, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReplacedResolution(t *testing.T) {
	t.Cleanup(func() { agentpakke.SetVersion("dev"); SetSelectedModel("") })
	SetSelectedModel(oldID)

	t.Run("replacement not downloaded, old weights here: keep the old pin", func(t *testing.T) {
		hf := t.TempDir()
		t.Setenv("HF_HOME", hf)
		fakeWeights(t, hf, oldID)
		m := nudgeManifest(t, "", replacedMap)
		got, _ := Chosen(m)
		if got.Model != oldID || got.Params["MLX_MODEL"] != oldID || got.Params["MLX_MODEL"] == m.Models[1].Params["MLX_MODEL"] {
			t.Errorf("Chosen = %q with MLX_MODEL %q, want the old weights on the replacement's settings", got.Model, got.Params["MLX_MODEL"])
		}
		want := oldID + " is replaced by " + replacementSlug + " (19 GB, not downloaded). Still using " + oldID +
			". Switch when ready: nav-pilot alpha local use " + replacementSlug + " && nav-pilot alpha local init"
		if n := ReplacedNotice(m); n != want {
			t.Errorf("notice = %q, want %q", n, want)
		}
		SetActive(m)
		defer SetActive(nil)
		if e, ok := Lookup(oldID); !ok || e.Model != oldID {
			t.Errorf("Lookup(old) = %q/%v, want the kept pin, so a launch can describe its server", e.Model, ok)
		}
	})

	t.Run("replacement downloaded: resolve to it", func(t *testing.T) {
		hf := t.TempDir()
		t.Setenv("HF_HOME", hf)
		fakeWeights(t, hf, oldID)
		fakeWeights(t, hf, newID)
		m := nudgeManifest(t, "", replacedMap)
		if got, _ := Chosen(m); got.Model != newID {
			t.Errorf("Chosen = %q, want the replacement %q", got.Model, newID)
		}
		want := oldID + " was replaced by " + replacementSlug + "; nav-pilot uses " + replacementSlug + " now. To pin it: nav-pilot alpha local use " + replacementSlug
		if n := ReplacedNotice(m); n != want {
			t.Errorf("notice = %q, want %q", n, want)
		}
	})

	t.Run("neither downloaded: resolve to the replacement", func(t *testing.T) {
		t.Setenv("HF_HOME", t.TempDir())
		m := nudgeManifest(t, "", replacedMap)
		if got, _ := Chosen(m); got.Model != newID {
			t.Errorf("Chosen = %q, want the replacement %q", got.Model, newID)
		}
	})

	t.Run("withheld replacement falls back to the default", func(t *testing.T) {
		t.Setenv("HF_HOME", t.TempDir())
		agentpakke.SetVersion("2026.09.20-080000-1111111")
		defer agentpakke.SetVersion("dev")
		m := nudgeManifest(t, `,"min_nav_pilot":"2026.09.24-110317-abc1234"`, replacedMap)
		if got, _ := Chosen(m); got.Model != okModel {
			t.Errorf("Chosen = %q, want the default", got.Model)
		}
		if n := ReplacedNotice(m); n != "" {
			t.Errorf("notice = %q, want none when the replacement is withheld", n)
		}
	})

	t.Run("replacement missing from the manifest falls back to the default", func(t *testing.T) {
		t.Setenv("HF_HOME", t.TempDir())
		m := nudgeManifest(t, "", `,"replaced":{"`+oldID+`":"gone"}`)
		if got, _ := Chosen(m); got.Model != okModel {
			t.Errorf("Chosen = %q, want the default", got.Model)
		}
		if n := ReplacedNotice(m); n != "" {
			t.Errorf("notice = %q, want none", n)
		}
	})

	t.Run("a replaced id outside the allowed publishers is ignored", func(t *testing.T) {
		t.Setenv("HF_HOME", t.TempDir())
		SetSelectedModel("evil-org/Model")
		defer SetSelectedModel(oldID)
		m := nudgeManifest(t, "", `,"replaced":{"evil-org/Model":"`+replacementSlug+`"}`)
		if got, _ := Chosen(m); got.Model != okModel {
			t.Errorf("Chosen = %q, want the default", got.Model)
		}
	})

	t.Run("manifest without the field", func(t *testing.T) {
		m := nudgeManifest(t, "", "")
		if got, _ := Chosen(m); got.Model != okModel {
			t.Errorf("Chosen = %q, want the default", got.Model)
		}
	})
}

// TestRecommendedAllowList: only keys nav-pilot has words for survive, in the
// allow-list's order, and an unknown key neither shows nor refuses the manifest.
func TestRecommendedAllowList(t *testing.T) {
	e := Model{RecommendedFor: []string{"from-the-future", "decide-nuanced", "default", "decide-untrusted-evidence"}}
	var keys []string
	for _, r := range e.Recommended() {
		keys = append(keys, r.Key)
	}
	if strings.Join(keys, ",") != "decide-untrusted-evidence,decide-nuanced" {
		t.Errorf("Recommended = %v, want [decide-untrusted-evidence decide-nuanced]", keys)
	}
	for _, r := range Recommendations {
		if r.Key == "" || r.Short == "" || r.For == "" {
			t.Errorf("recommendation %+v lacks words", r)
		}
	}
}

func TestPinnedAdvisory(t *testing.T) {
	t.Cleanup(func() { SetSelectedModel("") })
	m := nudgeManifest(t, "", "")

	SetSelectedModel(okModel)
	if a := PinnedAdvisory(m); a != "" {
		t.Errorf("pinned to the default: advisory = %q, want none", a)
	}

	SetSelectedModel(newID)
	want := "For decide on evidence you don't control, the default q36-default is recommended; you use " + replacementSlug +
		". Switch: nav-pilot alpha local use q36-default"
	if a := PinnedAdvisory(m); a != want {
		t.Errorf("advisory = %q, want %q", a, want)
	}

	m.Models[0].RecommendedFor = []string{"from-the-future"}
	if a := PinnedAdvisory(m); a != "" {
		t.Errorf("no recommendation to name: advisory = %q, want none", a)
	}
}

// TestShowOnce: a line shows once per machine, a changed line shows again, and
// MarkSeen suppresses a line without showing it.
func TestShowOnce(t *testing.T) {
	stubCache(t, nil)
	if !ShowOnce("a") || ShowOnce("a") {
		t.Error("the same line showed twice, or not at all")
	}
	if !ShowOnce("b") || ShowOnce("a") || ShowOnce("b") {
		t.Error("a new line did not show once, or an old one came back")
	}
	MarkSeen("c")
	if ShowOnce("c") {
		t.Error("a line marked seen was shown")
	}
	if ShowOnce("") {
		t.Error("an empty line was shown")
	}
}
