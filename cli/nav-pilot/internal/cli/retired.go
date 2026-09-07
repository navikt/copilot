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

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// retiredManifestPath is where the source publishes the record of what it has
// retired, relative to the source checkout.
const retiredManifestPath = ".nav-pilot/retired-artifacts.json"

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
func findRetiredOrphans(scope *InstallScope, sourceDir string) []retiredOrphan {
	m := loadRetired(sourceDir)
	if m == nil {
		return nil
	}
	var found []retiredOrphan
	for srcPath, blobs := range m.Paths {
		dir, file := path2DirFile(srcPath)
		kind := kindForDir(dir)
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

// path2DirFile splits "agents/auth.agent.md" into its directory and file name.
func path2DirFile(p string) (string, string) {
	i := strings.IndexByte(p, '/')
	if i < 0 {
		return "", p
	}
	return p[:i], p[i+1:]
}

// removeRetiredOrphans deletes the given files, returning how many went away.
// Best effort per file: one unremovable leftover must not fail a sync that has
// already done its real work.
func removeRetiredOrphans(orphans []retiredOrphan, quiet bool) int {
	removed := 0
	for _, o := range orphans {
		if err := os.Remove(o.Local); err != nil && !os.IsNotExist(err) {
			if !quiet {
				fmt.Printf("  %s %s could not be removed: %v\n", yellow("⚠"), o.Path, err)
			}
			continue
		}
		if !quiet {
			fmt.Printf("  %s %s\n", red("×"), o.Path)
		}
		removed++
	}
	return removed
}

// kindForDir maps a source directory to its artifact kind. Derived from
// AllKinds rather than written out, the lesson of #649, #650 and #708: three
// separate hardcoded kind lists have gone stale in this CLI already.
func kindForDir(dir string) *source.ArtifactKind {
	for _, k := range AllKinds {
		if k.Dir == dir {
			return k
		}
	}
	return nil
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
