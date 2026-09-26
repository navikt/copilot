package local

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// Recommendation is one key a manifest entry may list in "recommended_for",
// with the words nav-pilot says about it. Same rule as [Capabilities]: the
// manifest carries enum keys only, and every sentence is written here.
type Recommendation struct {
	Key string
	// Short is the models-table cell.
	Short string
	// For completes "recommended for …" in the advisory to pinned users.
	For string
}

// Recommendations is the allow-list of recommended_for keys. A key not listed
// here is ignored, so a newer generator can add one without breaking older
// binaries; adding one is a nav-pilot change because it needs a sentence.
//
//   - default: the general local worker. Implied by being the default, so the
//     pinned-user advisory leaves it out.
//   - decide-untrusted-evidence: `alpha decide` on evidence the caller does not
//     control (issue text, PR bodies): the model least moved by injected claims.
//   - decide-nuanced: `alpha decide` on nuanced yes/no questions over evidence
//     the caller trusts: the most accurate model there.
var Recommendations = []Recommendation{
	{"default", "general", "general use"},
	{"decide-untrusted-evidence", "decide: untrusted input", "decide on evidence you don't control"},
	{"decide-nuanced", "decide: nuanced", "decide on nuanced yes/no questions"},
}

// Recommended returns the entry's known recommended_for keys in
// [Recommendations] order. Unknown keys are dropped.
func (e Model) Recommended() []Recommendation {
	var out []Recommendation
	for _, r := range Recommendations {
		for _, k := range e.RecommendedFor {
			if k == r.Key {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// replacement is the offered entry a removed model id points at through the
// manifest's "replaced" map, matched by key or model id. Not found when the id
// is not replaced, or its replacement is withheld or not in the manifest.
func (m *Manifest) replacement(id string) (Model, bool) {
	if m == nil || id == "" {
		return Model{}, false
	}
	to, ok := m.Replaced[id]
	if !ok {
		return Model{}, false
	}
	for _, e := range m.Models {
		if e.Key == to || e.Model == to {
			return e, true
		}
	}
	return Model{}, false
}

// ReplacedNotice is the line to print when the configured local_model was
// removed from the manifest and [Chosen] resolved it to its replacement. The
// config is never rewritten: the developer makes it explicit, or not.
func ReplacedNotice(m *Manifest) string {
	for _, e := range m.Models {
		if e.Model == selectedModel {
			return ""
		}
	}
	to, ok := m.replacement(selectedModel)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s was replaced by %s; run %s to make it explicit.",
		selectedModel, to.Key, domain.Bold("nav-pilot alpha local use "+to.Key))
}

// advisedPath is the seen-marker for [PinnedAdvisory], next to the manifest
// cache so tests that move the cache move it too.
func advisedPath() string {
	if p := cachePath(); p != "" {
		return p + ".advised"
	}
	return ""
}

// PinnedAdvisory is the one-line nudge for a developer whose local_model pins
// an offered model that is not the default, naming what the default is
// recommended for. It returns "" when there is nothing to say, and also once
// the same line has been shown on this machine: the marker holds the last line
// shown, so it comes back only when the manifest changes what it would say.
// Callers are the local commands and launches, never `alpha decide`, which runs
// in hooks and scripts and must stay quiet.
func PinnedAdvisory(m *Manifest) string {
	if m == nil || selectedModel == "" {
		return ""
	}
	var def, pinned Model
	for _, e := range m.Models {
		if e.Default {
			def = e
		}
		if e.Model == selectedModel {
			pinned = e
		}
	}
	if pinned.Model == "" || pinned.Default {
		return ""
	}
	var fors []string
	for _, r := range def.Recommended() {
		if r.Key != "default" {
			fors = append(fors, r.For)
		}
	}
	if len(fors) == 0 {
		return ""
	}
	line := fmt.Sprintf("You use %s. The default %s is recommended for %s. Switch: ",
		pinned.Key, def.Key, strings.Join(fors, " and "))
	switchCmd := "nav-pilot alpha local use " + def.Key
	// The marker holds the unstyled line: whether Bold emits escapes depends
	// on the terminal, and that must not make the line show twice.
	if path := advisedPath(); path != "" {
		if seen, err := os.ReadFile(path); err == nil && string(seen) == line+switchCmd {
			return ""
		}
		// Best effort: a marker that cannot be written means the line shows
		// again, which is the lesser failure.
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = writeFileAtomic(path, []byte(line+switchCmd), 0o644)
	}
	return line + domain.Bold(switchCmd)
}
