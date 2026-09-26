package local

import (
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

const (
	oldID  = "mlx-community/Qwen3.8-27B-4bit"
	newID  = "mlx-community/Qwen3.8-27B-OptiQ-4bit"
	replacementSlug = "q38-optiq"
)

// nudgeManifest has a default recommended for untrusted-evidence decide, the
// replacement entry (plain, or gated to a newer nav-pilot), and a replaced map
// pointing the removed plain 4-bit at it.
func nudgeManifest(t *testing.T, replacementExtra string, replaced string) *Manifest {
	t.Helper()
	data := `{"schema_version":1,"channel":"alpha","models":[
		{"key":"q36-default","name":"Default","model":"` + okModel + `","backend":"mlx-lm","default":true,"params":{},
		 "recommended_for":["default","decide-untrusted-evidence","from-the-future"]},
		{"key":"` + replacementSlug + `","name":"OptiQ","model":"` + newID + `","backend":"mlx-lm","params":{}` + replacementExtra + `}]` +
		replaced + `}`
	m, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return m
}

const replacedMap = `,"replaced":{"` + oldID + `":"` + replacementSlug + `"}`

func TestReplacedResolution(t *testing.T) {
	t.Cleanup(func() { agentpakke.SetVersion("dev"); SetSelectedModel("") })
	SetSelectedModel(oldID)

	t.Run("offered replacement is chosen and announced", func(t *testing.T) {
		m := nudgeManifest(t, "", replacedMap)
		if got, _ := Chosen(m); got.Model != newID {
			t.Errorf("Chosen = %q, want the replacement %q", got.Model, newID)
		}
		n := ReplacedNotice(m)
		for _, want := range []string{oldID + " was replaced by " + replacementSlug, "nav-pilot alpha local use " + replacementSlug, "make it explicit"} {
			if !strings.Contains(n, want) {
				t.Errorf("notice %q lacks %q", n, want)
			}
		}
	})

	t.Run("withheld replacement falls back to the default", func(t *testing.T) {
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
		m := nudgeManifest(t, "", `,"replaced":{"`+oldID+`":"gone"}`)
		if got, _ := Chosen(m); got.Model != okModel {
			t.Errorf("Chosen = %q, want the default", got.Model)
		}
		if n := ReplacedNotice(m); n != "" {
			t.Errorf("notice = %q, want none", n)
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
	e := Model{RecommendedFor: []string{"from-the-future", "decide-untrusted-evidence", "default"}}
	var keys []string
	for _, r := range e.Recommended() {
		keys = append(keys, r.Key)
	}
	if strings.Join(keys, ",") != "default,decide-untrusted-evidence" {
		t.Errorf("Recommended = %v, want [default decide-untrusted-evidence]", keys)
	}
	for _, r := range Recommendations {
		if r.Key == "" || r.Short == "" || r.For == "" {
			t.Errorf("recommendation %+v lacks words", r)
		}
	}
}

func TestPinnedAdvisoryOncePerVersion(t *testing.T) {
	t.Cleanup(func() { SetSelectedModel("") })
	stubCache(t, nil)
	m := nudgeManifest(t, "", "")

	SetSelectedModel(okModel)
	if a := PinnedAdvisory(m); a != "" {
		t.Errorf("pinned to the default: advisory = %q, want none", a)
	}

	SetSelectedModel(newID)
	a := PinnedAdvisory(m)
	for _, want := range []string{"You use " + replacementSlug, "The default q36-default is recommended for decide on evidence you don't control", "nav-pilot alpha local use q36-default"} {
		if !strings.Contains(a, want) {
			t.Errorf("advisory %q lacks %q", a, want)
		}
	}
	if strings.Contains(a, "general use") {
		t.Errorf("advisory %q names the implied default key", a)
	}
	if again := PinnedAdvisory(m); again != "" {
		t.Errorf("second advisory = %q, want it shown once", again)
	}

	// A manifest that changes what the default is recommended for is a new
	// version of the advice, so it is shown again, once.
	m.Models[0].RecommendedFor = append(m.Models[0].RecommendedFor, "decide-nuanced")
	if a := PinnedAdvisory(m); !strings.Contains(a, "nuanced") {
		t.Errorf("after a manifest change, advisory = %q, want it shown again", a)
	}
	if again := PinnedAdvisory(m); again != "" {
		t.Errorf("repeat after the change = %q, want none", again)
	}

	// A default recommended for nothing beyond being the default says nothing.
	m.Models[0].RecommendedFor = []string{"default"}
	if a := PinnedAdvisory(m); a != "" {
		t.Errorf("no recommendation to name: advisory = %q, want none", a)
	}
}
