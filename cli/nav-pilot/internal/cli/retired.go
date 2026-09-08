package cli

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// retiredManifestPath is where the source publishes the record of what it has
// retired, relative to the source checkout.
//
// Aliased from the contract rather than restated: the constant the validator
// uses and the constant the reader uses must be the same one, or nav-pilot can
// end up validating a file it does not read.
const retiredManifestPath = agentpakke.RetiredRecordPath

// retiredManifest is the generated record: artifact path to every content hash
// that path ever held before it was deleted.
//
// The hashes are git blob ids, which is what the generator can read out of
// history. Computing one needs no git: it is sha1 of "blob <len>\x00" followed
// by the bytes.
type retiredManifest struct {
	Paths map[string][]string `json:"paths"`
}

// loadRetired reads the source's retired-artifact record.
//
// A missing file is not an error and not a warning. An agentpakke that is not
// this repo has no such file, an older revision of this repo has none either,
// and neither case is a problem: nothing is removed, exactly as before.
func loadRetired(sourceDir string) *retiredManifest {
	data, err := os.ReadFile(filepath.Join(sourceDir, retiredManifestPath))
	if err != nil {
		return nil
	}
	// Validated against the published schema, like both manifests, because this
	// file now belongs to the contract: any agentpakke may publish one, and a
	// malformed record must not lead to a deletion (#729). A record that does
	// not conform is treated as absent, which removes nothing. Failing the sync
	// instead would let a third party's broken file block an update that has
	// nothing to do with it.
	if err := agentpakke.ValidateRetired(data); err != nil {
		return nil
	}
	var m retiredManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	if len(m.Paths) == 0 {
		return nil
	}
	return &m
}

// blobHash is git's object id for a file's contents: sha1 over the header
// "blob <length>\x00" and then the bytes. Reimplemented rather than shelled
// out to, because the scope being cleaned is not a git repository and the
// source may be a shallow clone with no history to ask.
func blobHash(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// retiredOrphan is an installed file the source has retired and whose content
// nav-pilot published.
type retiredOrphan struct {
	Path  string // path as the source names it, e.g. "agents/auth.agent.md"
	Local string // absolute path in the scope
}

// findRetiredOrphans returns the installed artifacts the source has retired,
// keeping only those whose bytes match a revision the source once published.
//
// The content check is the whole safety argument. "The source no longer ships
// an agent by this name" is not permission to delete: a developer may have
// written their own agent at that path, and it is theirs. A byte match against
// a published revision is proof nav-pilot wrote the file and that the user has
// not since changed it, which is the same standard the ordinary delete path
// uses (#716).
func findRetiredOrphans(scope *InstallScope, sourceDir string, pakke *agentpakke.Manifest) []retiredOrphan {
	m := loadRetired(sourceDir)
	if m == nil {
		return nil
	}
	var layout *agentpakke.Layout
	if pakke != nil {
		layout = pakke.Layout
	}
	var found []retiredOrphan
	for srcPath, blobs := range m.Paths {
		kind, file := kindForPath(srcPath, layout)
		if kind == nil {
			continue
		}
		local := scope.DstPath(kind.Dir, file)
		data, err := os.ReadFile(local)
		if err != nil {
			continue
		}
		have := blobHash(data)
		for _, b := range blobs {
			if b == have {
				found = append(found, retiredOrphan{Path: srcPath, Local: local})
				break
			}
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found
}

// kindForPath maps a retired source path to its artifact kind and the file
// under it, or nil when the path belongs to no kind this scope knows.
//
// Matching is by directory prefix rather than by splitting on the first slash.
// A declared layout may nest, so "content/agents/gammel.agent.md" has to resolve
// to the agent kind with "gammel.agent.md" under it; splitting on the first
// slash gave "content" and matched nothing (#728).
//
// The kinds come from AllKinds rather than a written-out list, the lesson of
// #649, #650 and #708: three hardcoded kind lists have gone stale in this CLI
// already. The directory names come from the manifest when it declares a
// layout, and from the canonical names otherwise, because a Tier 1 agentpakke
// may put its content anywhere. Matching only the canonical names made the
// retired-artifact record usable by this repo alone.
func kindForPath(srcPath string, layout *agentpakke.Layout) (*source.ArtifactKind, string) {
	type candidate struct {
		dir  string
		kind *source.ArtifactKind
	}
	var candidates []candidate
	// Derived from the layout's own field list, not written out again: this
	// was the list that did not get extensions (#739), so a retired extension
	// under a declared layout was never found. Layout.Dirs names each field
	// as "layout.<dir>", which is the kind's canonical directory.
	if layout != nil {
		byDir := make(map[string]*source.ArtifactKind, len(AllKinds))
		for _, k := range AllKinds {
			byDir[k.Dir] = k
		}
		for _, d := range layout.Dirs() {
			kind, ok := byDir[strings.TrimPrefix(d.Field, "layout.")]
			if !ok || d.Value == "" {
				continue
			}
			candidates = append(candidates, candidate{strings.Trim(d.Value, "/"), kind})
		}
	}
	for _, k := range AllKinds {
		candidates = append(candidates, candidate{k.Dir, k})
	}
	// Longest first, so a layout that nests one kind inside another cannot be
	// shadowed by the shorter prefix.
	sort.Slice(candidates, func(i, j int) bool { return len(candidates[i].dir) > len(candidates[j].dir) })
	for _, c := range candidates {
		if rest, ok := strings.CutPrefix(srcPath, c.dir+"/"); ok && rest != "" {
			return c.kind, rest
		}
	}
	return nil, ""
}

// removeRetiredOrphans deletes the given files, returning how many went away.
// Best effort per file: one unremovable leftover must not fail a sync that has
// already done its real work.
func removeRetiredOrphans(scope *InstallScope, orphans []retiredOrphan, quiet bool) int {
	removed := 0
	for _, o := range orphans {
		err := os.Remove(o.Local)
		if err != nil && !os.IsNotExist(err) {
			if !quiet {
				fmt.Printf("  %s %s could not be removed: %v\n", yellow("⚠"), o.Path, err)
			}
			continue
		}
		// A file the deletion pass already took is not one this pass removed.
		// Counting it here reported the same artifact twice: once as deleted
		// upstream and again as retired.
		if os.IsNotExist(err) {
			continue
		}
		afterArtifactRemoved(scope, o.Local, quiet)
		if !quiet {
			fmt.Printf("  %s %s\n", red("×"), o.Path)
		}
		removed++
	}
	return removed
}

// retiredPaths is the orphan list as source paths, for the JSON document.
func retiredPaths(orphans []retiredOrphan) []string {
	if len(orphans) == 0 {
		return nil
	}
	out := make([]string, 0, len(orphans))
	for _, o := range orphans {
		out = append(out, o.Path)
	}
	return out
}

// reportRetired prints the retired-artifact section.
func reportRetired(orphans []retiredOrphan, apply bool) {
	fmt.Printf("%s %d artifact(s) retired in the source are still installed\n\n",
		yellow("⚠"), len(orphans))
	for _, o := range orphans {
		fmt.Printf("  %s %s\n", dim("⊘"), o.Path)
	}
	fmt.Println()
	if !apply {
		fmt.Printf("%s removes them. Each one's content matches a revision nav-pilot published,\n", bold("nav-pilot sync --apply"))
		fmt.Printf("so nothing you wrote yourself is touched.\n\n")
	}
}

// ignoredButInstalled lists artifacts the state marks as ignored while the file
// sits in the scope.
//
// That combination is one nobody chooses (#724). `nav-pilot ignore` marks
// something the user does not want, and it is normally not on disk; the
// collection fold-in in #468 marked every artifact missing from state, which
// included files an older nav-pilot had installed without recording. Sync then
// skips them by design, so they never update: one carried a model GitHub had
// withdrawn for weeks, and only doctor's catalogue check found it.
//
// Reported, never removed. The file may be exactly what someone wants; what is
// wrong is that nothing says it has stopped being maintained.
func ignoredButInstalled(scope *InstallScope) []string {
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return nil
	}
	var found []string
	for _, f := range state.Files {
		if f.Status != fileStatusIgnored {
			continue
		}
		if _, err := os.Stat(filepath.Join(scope.RootDir, f.Path)); err == nil {
			found = append(found, f.Path)
		}
	}
	sort.Strings(found)
	return found
}

// reportIgnoredButInstalled prints the section for those artifacts.
func reportIgnoredButInstalled(paths []string) {
	fmt.Printf("%s %d installed artifact(s) are marked ignored, so sync leaves them as they are\n\n",
		yellow("⚠"), len(paths))
	for _, p := range paths {
		fmt.Printf("  %s %s\n", dim("⊘"), p)
	}
	fmt.Printf("\n  They keep whatever version they were installed with, including a model pin\n")
	fmt.Printf("  that may no longer exist. %s takes the source's version and\n", bold("nav-pilot add <type> <name> --force"))
	fmt.Printf("  starts tracking it again.\n\n")
}

// installedRevisions maps each tracked path to the source revision it was
// written from, for the entries that carry one (#729).
//
// A path with no recorded revision is absent from the map rather than present
// with an empty value: the caller must be able to tell "installed from an older
// revision" from "we do not know", and those lead to different sentences.
func installedRevisions(scope *InstallScope) map[string]string {
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		return nil
	}
	out := make(map[string]string, len(state.Files))
	for _, f := range state.Files {
		if f.Revision != "" {
			out[f.Path] = f.Revision
		}
	}
	return out
}

// sameRevision reports whether two revision strings name the same commit.
//
// Prefix comparison, the way git resolves an abbreviated object id, because the
// two sides come from different places: a state file may carry a short sha
// written by an older nav-pilot or by hand, while the source resolves to the
// full 40 characters. Comparing them for equality reported every such file as
// "installed from <rev>" even when it came from exactly the revision the scope
// is on, which is the false positive this suffix exists to avoid.
//
// An empty string matches nothing: absence is not agreement. So is a prefix
// shorter than git's own minimum, since four hex characters agree by accident
// often enough to be worthless as evidence.
func sameRevision(a, b string) bool {
	const minAbbrev = 7
	if a == "" || b == "" {
		return false
	}
	if strings.EqualFold(a, b) {
		return true
	}
	short, long := a, b
	if len(short) > len(long) {
		short, long = long, short
	}
	if len(short) < minAbbrev {
		return false
	}
	return strings.EqualFold(short, long[:len(short)])
}
