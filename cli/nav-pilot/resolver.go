package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ArtifactKind describes the filesystem shape of one artifact type.
// Differences between types are captured here, not in code branches.
type ArtifactKind struct {
	Name     string // singular: "agent", "skill", "instruction", "prompt"
	Dir      string // plural directory: "agents", "skills", "instructions", "prompts"
	Suffix   string // file extension: ".agent.md", ".instructions.md", ".prompt.md"
	IsDir    bool   // always a directory (skills)
	CanBeDir bool   // may be file or directory (prompts)
	Marker   string // required file inside directory: "SKILL.md"
}

var (
	KindAgent       = &ArtifactKind{Name: "agent", Dir: "agents", Suffix: ".agent.md"}
	KindSkill       = &ArtifactKind{Name: "skill", Dir: "skills", IsDir: true, Marker: "SKILL.md"}
	KindInstruction = &ArtifactKind{Name: "instruction", Dir: "instructions", Suffix: ".instructions.md"}
	KindPrompt      = &ArtifactKind{Name: "prompt", Dir: "prompts", Suffix: ".prompt.md", CanBeDir: true}

	// AllKinds lists all artifact kinds for iteration.
	AllKinds = []*ArtifactKind{KindAgent, KindSkill, KindInstruction, KindPrompt}

	// kindByName maps singular names to their ArtifactKind.
	kindByName = map[string]*ArtifactKind{
		"agent":       KindAgent,
		"skill":       KindSkill,
		"instruction": KindInstruction,
		"prompt":      KindPrompt,
	}
)

// Resolved represents a found artifact in the source repo.
type Resolved struct {
	Kind    *ArtifactKind
	Name    string // bare name without suffix
	AbsPath string // full filesystem path
	RelPath string // relative to source root (e.g. "agents/foo.agent.md")
	IsDir   bool   // actual shape on disk
}

// FileName returns the name used for destination paths.
// Directories return the bare name; files return name + suffix.
func (r Resolved) FileName() string {
	if r.IsDir {
		return r.Name
	}
	return r.Name + r.Kind.Suffix
}

// RelPathForName returns the state-file relative path for a named item in this scope.
// Centralizes the name→path mapping used by picker defaults, skipped items, and sync.
func (k *ArtifactKind) RelPathForName(scope *InstallScope, name string) string {
	fileName := name + k.Suffix
	if k.IsDir {
		fileName = name
	}
	relPath := scope.RelPath(k.Dir, fileName)
	if k.IsDir {
		relPath += "/"
	}
	return relPath
}

// SourceResolver centralizes all source-repo path resolution.
// Artifacts may live at root level (matching github/awesome-copilot convention)
// or under .github/ (legacy). Root wins when present.
type SourceResolver struct {
	sourceDir string
}

// NewSourceResolver creates a resolver for the given source directory.
func NewSourceResolver(sourceDir string) *SourceResolver {
	return &SourceResolver{sourceDir: sourceDir}
}

// Get finds a single named artifact. Checks root first, then .github/.
func (r *SourceResolver) Get(kind *ArtifactKind, name string) (Resolved, bool) {
	if kind.IsDir {
		return r.getDir(kind, name)
	}
	if kind.CanBeDir {
		return r.getCanBeDir(kind, name)
	}
	return r.getSimpleFile(kind, name)
}

// getSimpleFile resolves a single-file artifact (agents, instructions).
// Checks root/<dir>/<name><suffix> then .github/<dir>/<name><suffix>.
func (r *SourceResolver) getSimpleFile(kind *ArtifactKind, name string) (Resolved, bool) {
	fileName := name + kind.Suffix
	for _, prefix := range [2]string{"", ".github"} {
		rel := filepath.Join(prefix, kind.Dir, fileName)
		abs := filepath.Join(r.sourceDir, rel)
		if _, err := os.Stat(abs); err == nil {
			return Resolved{Kind: kind, Name: name, AbsPath: abs, RelPath: rel, IsDir: false}, true
		}
	}
	return Resolved{}, false
}

// getDir resolves a directory artifact with a required marker file (skills).
// Checks root/<dir>/<name>/<marker> then .github/<dir>/<name>/<marker>.
func (r *SourceResolver) getDir(kind *ArtifactKind, name string) (Resolved, bool) {
	for _, prefix := range [2]string{"", ".github"} {
		rel := filepath.Join(prefix, kind.Dir, name)
		abs := filepath.Join(r.sourceDir, rel)
		if kind.Marker != "" {
			if _, err := os.Stat(filepath.Join(abs, kind.Marker)); err == nil {
				return Resolved{Kind: kind, Name: name, AbsPath: abs, RelPath: rel, IsDir: true}, true
			}
		} else {
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				return Resolved{Kind: kind, Name: name, AbsPath: abs, RelPath: rel, IsDir: true}, true
			}
		}
	}
	return Resolved{}, false
}

// getCanBeDir resolves a file-or-directory artifact (prompts).
// Precedence: root dir > root file > legacy dir > legacy file.
func (r *SourceResolver) getCanBeDir(kind *ArtifactKind, name string) (Resolved, bool) {
	for _, prefix := range [2]string{"", ".github"} {
		dirRel := filepath.Join(prefix, kind.Dir, name)
		dirAbs := filepath.Join(r.sourceDir, dirRel)
		if info, err := os.Stat(dirAbs); err == nil && info.IsDir() {
			return Resolved{Kind: kind, Name: name, AbsPath: dirAbs, RelPath: dirRel, IsDir: true}, true
		}
		fileRel := filepath.Join(prefix, kind.Dir, name+kind.Suffix)
		fileAbs := filepath.Join(r.sourceDir, fileRel)
		if _, err := os.Stat(fileAbs); err == nil {
			return Resolved{Kind: kind, Name: name, AbsPath: fileAbs, RelPath: fileRel, IsDir: false}, true
		}
	}
	return Resolved{}, false
}

// GetFile resolves a specific file by typeDir + fileName.
// Checks root/<typeDir>/<fileName> then .github/<typeDir>/<fileName>.
func (r *SourceResolver) GetFile(typeDir, fileName string) (absPath, relPath string, ok bool) {
	for _, prefix := range [2]string{"", ".github"} {
		rel := filepath.Join(prefix, typeDir, fileName)
		abs := filepath.Join(r.sourceDir, rel)
		if _, err := os.Stat(abs); err == nil {
			return abs, rel, true
		}
	}
	return "", "", false
}

// List discovers all artifacts of a kind. Scans both root and .github/
// locations for candidate names, then calls Get() for each to apply
// consistent precedence. Results sorted by name.
func (r *SourceResolver) List(kind *ArtifactKind) []Resolved {
	names := r.discoverNames(kind)
	var results []Resolved
	for _, name := range names {
		if art, ok := r.Get(kind, name); ok {
			results = append(results, art)
		}
	}
	return results
}

// discoverNames scans both locations for candidate artifact names.
// Returns deduplicated, sorted names that pass validateName.
// Does not validate markers — Get() handles that.
func (r *SourceResolver) discoverNames(kind *ArtifactKind) []string {
	seen := make(map[string]bool)
	var names []string

	for _, base := range [2]string{
		filepath.Join(r.sourceDir, kind.Dir),
		filepath.Join(r.sourceDir, ".github", kind.Dir),
	} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			var name string
			switch {
			case kind.IsDir:
				// Skills: only directory entries
				if !e.IsDir() {
					continue
				}
				name = e.Name()
			case kind.CanBeDir:
				// Prompts: directories or files matching suffix
				if e.IsDir() {
					name = e.Name()
				} else if strings.HasSuffix(e.Name(), kind.Suffix) {
					name = strings.TrimSuffix(e.Name(), kind.Suffix)
				} else {
					continue
				}
			default:
				// Agents, instructions: only files matching suffix
				if !strings.HasSuffix(e.Name(), kind.Suffix) {
					continue
				}
				name = strings.TrimSuffix(e.Name(), kind.Suffix)
			}
			if seen[name] {
				continue
			}
			if validateName(name) == nil {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	sort.Strings(names)
	return names
}

// MapLocalPath maps an installed/state path back to the source path.
// It parses the type directory from the path, then probes root vs .github/
// to find where the artifact actually lives in the source repo.
//
// On miss (artifact removed from source), returns the original path unchanged.
// The caller (sync) detects this at hash time and reports the error.
func (r *SourceResolver) MapLocalPath(localPath string, isUserScope bool) string {
	sp := filepath.ToSlash(localPath)
	hasSuffix := strings.HasSuffix(sp, "/")

	// Determine the path without .github/ prefix
	var rest string
	var hadPrefix bool
	if strings.HasPrefix(sp, ".github/") {
		rest = strings.TrimPrefix(sp, ".github/")
		hadPrefix = true
	} else if isUserScope {
		// User scope: agents/, skills/ have no prefix;
		// instructions always have .github/ prefix
		rest = sp
	} else {
		// Repo scope without .github/ — unexpected, return as-is
		return sp
	}

	// Match against known type directories
	for _, kind := range AllKinds {
		if !strings.HasPrefix(rest, kind.Dir+"/") {
			continue
		}
		remainder := strings.TrimPrefix(rest, kind.Dir+"/")

		if kind.IsDir {
			// Skills: resolve via Get for SKILL.md validation
			name := strings.TrimSuffix(remainder, "/")
			if art, ok := r.Get(kind, name); ok {
				if hasSuffix {
					return art.RelPath + "/"
				}
				return art.RelPath
			}
		} else {
			// Files (agents, instructions, prompts): raw existence check.
			// filepath.Join inside GetFile strips any trailing slash,
			// so this works for both files and prompt directories.
			if _, relPath, ok := r.GetFile(kind.Dir, remainder); ok {
				if hasSuffix && !strings.HasSuffix(relPath, "/") {
					return relPath + "/"
				}
				return relPath
			}
		}
		break // matched type dir, stop searching
	}

	// User scope without .github/ prefix that didn't resolve: prepend .github/
	if isUserScope && !hadPrefix {
		result := filepath.ToSlash(filepath.Join(".github", sp))
		if hasSuffix && !strings.HasSuffix(result, "/") {
			result += "/"
		}
		return result
	}

	return sp
}
