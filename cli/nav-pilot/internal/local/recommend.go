package local

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
	// For completes "For …, the default is recommended" in the advisory to
	// pinned users.
	For string
}

// Recommendations is the allow-list of recommended_for keys. A key not listed
// here is ignored, so a newer generator can add one without breaking older
// binaries; adding one is a nav-pilot change because it needs a sentence.
//
//   - decide-untrusted-evidence: `alpha decide` on evidence the caller does not
//     control (issue text, PR bodies): the model least moved by injected claims.
//   - decide-nuanced: `alpha decide` on nuanced yes/no questions over evidence
//     the caller trusts: the most accurate model there.
//
// Being the general-purpose choice is what "default": true already says, so
// it has no key.
var Recommendations = []Recommendation{
	{"decide-untrusted-evidence", "untrusted decide", "decide on evidence you don't control"},
	{"decide-nuanced", "nuanced decide", "decide on nuanced yes/no questions"},
}

// Recommended returns the entry's known recommended_for keys in
// [Recommendations] order. Unknown keys are dropped.
func (e Model) Recommended() []Recommendation {
	var out []Recommendation
	for _, r := range Recommendations {
		if slices.Contains(e.RecommendedFor, r.Key) {
			out = append(out, r)
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

// ReplacedBy is the offered entry that replaces a removed model id, for `use`
// to point at.
func (m *Manifest) ReplacedBy(id string) (Model, bool) { return m.replacement(id) }

// legacy is the entry a replaced id still runs as, beside its replacement.
//
// "replaced" records a swap of weights under the same settings (the plain
// Qwen3.8 4-bit and its OptiQ build share one profile), so the old weights run
// with the replacement's params. The old id is held to the same publisher rule
// as any entry: it names weights this machine loads.
func (m *Manifest) legacy(id string) (old, repl Model, ok bool) {
	repl, ok = m.replacement(id)
	if !ok || domain.ValidateModelValue(id) != nil {
		return Model{}, Model{}, false
	}
	if publisher, _, _ := strings.Cut(id, "/"); !slices.Contains(allowedPublishers, publisher) {
		return Model{}, Model{}, false
	}
	old = repl
	old.Key, old.Name, old.Model = id, id, id
	old.Default, old.Role, old.Expect, old.RecommendedFor = false, "", "", nil
	old.Params = maps.Clone(repl.Params)
	if old.Params != nil {
		old.Params["MLX_MODEL"] = id
	}
	return old, repl, true
}

// ReplacedIDs are the removed model ids the manifest still knows, for purge.
func (m *Manifest) ReplacedIDs() []string {
	if m == nil {
		return nil
	}
	return slices.Sorted(maps.Keys(m.Replaced))
}

// keepLegacy decides between a replaced id and its replacement: keep the old
// weights while they are here and the replacement's are not, so a replacement
// never turns a working start into a download.
func keepLegacy(old, repl Model) bool {
	oldHere, _ := WeightsPresent(old.Model)
	replHere, _ := WeightsPresent(repl.Model)
	return oldHere && !replHere
}

// ReplacedNotice is the line to show when the configured local_model was
// removed from the manifest and listed as replaced: either nav-pilot still
// runs it because the replacement is not downloaded, or it runs the
// replacement now. The config is never rewritten. Plain text, for
// [ShowOnce].
func ReplacedNotice(m *Manifest) string {
	if m == nil || slices.ContainsFunc(m.Models, func(e Model) bool { return e.Model == selectedModel }) {
		return ""
	}
	old, repl, ok := m.legacy(selectedModel)
	if !ok {
		return ""
	}
	if keepLegacy(old, repl) {
		size := ""
		if repl.WeightsGB > 0 {
			size = fmt.Sprintf("%d GB, ", repl.WeightsGB)
		}
		return fmt.Sprintf("%s is replaced by %s (%snot downloaded). Still using %s. Switch when ready: nav-pilot alpha local use %s && nav-pilot alpha local init",
			old.Model, repl.Key, size, old.Model, repl.Key)
	}
	return fmt.Sprintf("%s was replaced by %s; nav-pilot uses %s now. To pin it: nav-pilot alpha local use %s",
		old.Model, repl.Key, repl.Key, repl.Key)
}

// PinnedAdvisory is the nudge for a developer whose local_model pins an
// offered model that is not the default: what the default is recommended
// for, first, then the command to switch. "" when there is nothing to say.
// Plain text, for [ShowOnce]; the local commands show it, launches and `alpha
// decide` never do.
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
		fors = append(fors, r.For)
	}
	if len(fors) == 0 {
		return ""
	}
	return fmt.Sprintf("For %s, the default %s is recommended; you use %s. Switch: nav-pilot alpha local use %s",
		strings.Join(fors, " and "), def.Key, pinned.Key, def.Key)
}

// seenPath holds every nudge line already shown on this machine, next to the
// manifest cache so tests that move the cache move it too. A line comes back
// only when the manifest changes what it says.
func seenPath() string {
	if p := cachePath(); p != "" {
		return p + ".seen"
	}
	return ""
}

func seen(path, line string) bool {
	data, _ := os.ReadFile(path)
	return slices.Contains(strings.Split(string(data), "\n"), line)
}

// ShowOnce reports whether line has not been shown on this machine yet, and
// records it as shown. The caller checks that someone is there to see it
// first: a line recorded while stderr went to a pipe is never shown.
//
// ponytail: the file only grows, by one line per distinct message; prune it
// if messages ever start carrying per-run values.
func ShowOnce(line string) bool {
	path := seenPath()
	if line == "" || (path != "" && seen(path, line)) {
		return false
	}
	MarkSeen(line)
	return true
}

// MarkSeen records line as shown without showing it: `use` calls it for the
// advisory its own choice would trigger, since the developer just decided.
// Best effort: a marker that cannot be written means the line shows again,
// which is the lesser failure.
func MarkSeen(line string) {
	path := seenPath()
	if line == "" || path == "" || seen(path, line) {
		return
	}
	data, _ := os.ReadFile(path)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = writeFileAtomic(path, append(data, []byte(line+"\n")...), 0o644)
}
