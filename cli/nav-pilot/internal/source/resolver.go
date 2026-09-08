package source

import (
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// ArtifactKind describes the filesystem shape of one artifact type.
type ArtifactKind struct {
	Name     string // singular: "agent", "skill", "instruction", "prompt", "hook", "extension"
	Dir      string // plural directory: "agents", "skills", "instructions", "prompts", "hooks", "extensions"
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

	// KindHook is the one artifact kind that is executable code rather than
	// text the model reads: a preToolUse script the CLI runs on every matching
	// tool call. Installing a package therefore installs code that runs, which
	// is a different trust decision from the other four (#569). The script is
	// the artifact; the entry that activates it is written by internal/source's
	// hook merge, and hooks/<name>.hook.json carries its matcher.
	KindHook = &ArtifactKind{Name: "hook", Dir: "hooks", Suffix: ".py"}

	// KindExtension is the second kind that is executable code: a directory
	// holding extension.mjs, which the client loads and runs. Same trust
	// decision as a hook, and the same reason it needs a type at all: a team
	// that has written one today cannot distribute it, because nav-pilot does
	// not know the shape exists (#572).
	//
	// A directory rather than a file, because an extension is rarely one module:
	// watson-developer's auto-format ships helpers beside its entry point. The
	// marker is what makes a directory an extension rather than a stray folder,
	// exactly as SKILL.md does for a skill.
	KindExtension = &ArtifactKind{Name: "extension", Dir: "extensions", IsDir: true, Marker: "extension.mjs"}

	// AllKinds lists all artifact kinds for iteration.
	AllKinds = []*ArtifactKind{KindAgent, KindSkill, KindInstruction, KindPrompt, KindHook, KindExtension}

	// KindByName maps singular names to their ArtifactKind.
	// KindByName maps a singular kind name to its kind, derived from AllKinds
	// rather than written out beside it. A second hand-maintained list is how
	// `hook` ended up missing from three separate places after it was added
	// (#649, #650, #708); adding a kind should mean editing AllKinds and
	// nothing else.
	KindByName = func() map[string]*ArtifactKind {
		m := make(map[string]*ArtifactKind, len(AllKinds))
		for _, k := range AllKinds {
			m[k.Name] = k
		}
		return m
	}()
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
func (k *ArtifactKind) RelPathForName(scope *domain.InstallScope, name string) string {
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
type SourceResolver struct {
	sourceDir string

	// layout remaps a canonical artifact directory ("agents", "skills", …) to
	// the repo-relative directory an agentpakke manifest declares for it. It is
	// nil (and every lookup falls back to the canonical name) for sources that
	// ship content at the canonical paths — which is what the legacy adapter
	// synthesizes, so manifest-less sources resolve byte-for-byte as before.
	layout map[string]string

	// base is the agentpakke this one reuses, resolved at its pinned revision,
	// or nil for the overwhelming majority that reuse nothing. It is consulted
	// only after this source misses, which is the whole of the collision rule:
	// a pakke that ships its own agent named "grillmester" shadows the reused
	// one without saying anything, the same way `overrides` in
	// .github/copilot-sync.json lets a repo keep its own copy of a synced file.
	base *SourceResolver
}

// WithBase returns a resolver that falls back to base for artifacts this source
// does not ship itself. A base may carry its own base; the chain is built
// depth-first by the caller, which refuses a declaration cycle before it
// recurses.
func (r *SourceResolver) WithBase(base *SourceResolver) *SourceResolver {
	chained := *r
	chained.base = base
	return &chained
}

// SourceRootFor returns the source directory that actually holds a
// source-relative path, looking through the reuse chain, and whether any of
// them does.
//
// Sync needs this because a tracked file may have come from a reused pakke: it
// is not under this source's directory, and reading that absence as "deleted
// upstream" deletes what install just wrote. The same reason `add --source`
// files are left alone (#571), one level up.
func (r *SourceResolver) SourceRootFor(rel string) (string, bool) {
	abs := filepath.Join(r.sourceDir, rel)
	if _, err := r.checkSafePath(abs); err == nil {
		if _, err := os.Stat(abs); err == nil {
			return r.sourceDir, true
		}
	}
	if r.base != nil {
		return r.base.SourceRootFor(rel)
	}
	return "", false
}

// Base returns the reused resolver, or nil.
func (r *SourceResolver) Base() *SourceResolver { return r.base }

// NewSourceResolver creates a resolver for the given source directory, reading
// content from the canonical directories (agents/, skills/, …).
func NewSourceResolver(sourceDir string) *SourceResolver {
	return &SourceResolver{sourceDir: sourceDir}
}

// NewSourceResolverForLayout creates a resolver that reads content from an
// agentpakke manifest's layout paths (D1). A nil layout, or one that only
// repeats the canonical directory names, behaves exactly like
// [NewSourceResolver].
func NewSourceResolverForLayout(sourceDir string, layout *agentpakke.Layout) *SourceResolver {
	r := &SourceResolver{sourceDir: sourceDir}
	if layout == nil {
		return r
	}
	declared := map[string]string{
		KindAgent.Dir:       layout.Agents,
		KindSkill.Dir:       layout.Skills,
		KindInstruction.Dir: layout.Instructions,
		KindPrompt.Dir:      layout.Prompts,
		KindHook.Dir:        layout.Hooks,
		KindExtension.Dir:   layout.Extensions,
	}
	for canonical, dir := range declared {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		clean := path.Clean(dir)
		if clean == canonical {
			continue
		}
		if r.layout == nil {
			r.layout = make(map[string]string, len(declared))
		}
		r.layout[canonical] = filepath.FromSlash(clean)
	}
	return r
}

// SourceDir returns the checkout the resolver reads from. Callers that need to
// join a resolved RelPath back to an absolute path use this instead of carrying
// the directory alongside the resolver.
func (r *SourceResolver) SourceDir() string { return r.sourceDir }

// dirFor returns the source-relative directory that holds a canonical artifact
// directory's content.
func (r *SourceResolver) dirFor(canonical string) string {
	if dir, ok := r.layout[canonical]; ok {
		return dir
	}
	return canonical
}

// Get finds a single named artifact. Checks root first, then .github/.
func (r *SourceResolver) Get(kind *ArtifactKind, name string) (Resolved, bool) {
	if res, ok := r.get(kind, name); ok {
		return res, true
	}
	// Only now the reused pakke: this source's own content wins every
	// collision, and it wins by being asked first.
	if r.base != nil {
		return r.base.Get(kind, name)
	}
	return Resolved{}, false
}

func (r *SourceResolver) get(kind *ArtifactKind, name string) (Resolved, bool) {
	if kind.IsDir {
		return r.getDir(kind, name)
	}
	if kind.CanBeDir {
		return r.getCanBeDir(kind, name)
	}
	return r.getSimpleFile(kind, name)
}

// checkSafePath refuses to hand back anything the source checkout does not
// actually contain: a path that is textually outside it, a symlink, or a path
// that leaves the checkout through a symlinked parent directory (an agentpakke
// whose layout.agents is a link to somewhere else on the machine, say). The
// containment check covers the canonical directories too, not only
// manifest-declared ones.
func (r *SourceResolver) checkSafePath(abs string) (os.FileInfo, error) {
	rel, err := filepath.Rel(r.sourceDir, abs)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return nil, os.ErrNotExist
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, os.ErrNotExist
	}
	if !domain.PathWithinRoot(r.sourceDir, abs) {
		return nil, os.ErrNotExist
	}
	return info, nil
}

func (r *SourceResolver) getSimpleFile(kind *ArtifactKind, name string) (Resolved, bool) {
	fileName := name + kind.Suffix
	rel := filepath.Join(r.dirFor(kind.Dir), fileName)
	abs := filepath.Join(r.sourceDir, rel)
	if _, err := r.checkSafePath(abs); err == nil {
		return Resolved{Kind: kind, Name: name, AbsPath: abs, RelPath: rel, IsDir: false}, true
	}
	return Resolved{}, false
}

func (r *SourceResolver) getDir(kind *ArtifactKind, name string) (Resolved, bool) {
	rel := filepath.Join(r.dirFor(kind.Dir), name)
	abs := filepath.Join(r.sourceDir, rel)

	if _, err := r.checkSafePath(abs); err != nil {
		return Resolved{}, false
	}

	if kind.Marker != "" {
		if _, err := r.checkSafePath(filepath.Join(abs, kind.Marker)); err == nil {
			return Resolved{Kind: kind, Name: name, AbsPath: abs, RelPath: rel, IsDir: true}, true
		}
	} else {
		if info, err := r.checkSafePath(abs); err == nil && info.IsDir() {
			return Resolved{Kind: kind, Name: name, AbsPath: abs, RelPath: rel, IsDir: true}, true
		}
	}
	return Resolved{}, false
}

func (r *SourceResolver) getCanBeDir(kind *ArtifactKind, name string) (Resolved, bool) {
	dirRel := filepath.Join(r.dirFor(kind.Dir), name)
	dirAbs := filepath.Join(r.sourceDir, dirRel)
	if info, err := r.checkSafePath(dirAbs); err == nil && info.IsDir() {
		return Resolved{Kind: kind, Name: name, AbsPath: dirAbs, RelPath: dirRel, IsDir: true}, true
	}
	fileRel := filepath.Join(r.dirFor(kind.Dir), name+kind.Suffix)
	fileAbs := filepath.Join(r.sourceDir, fileRel)
	if _, err := r.checkSafePath(fileAbs); err == nil {
		return Resolved{Kind: kind, Name: name, AbsPath: fileAbs, RelPath: fileRel, IsDir: false}, true
	}
	return Resolved{}, false
}

// GetFile resolves a specific file by typeDir + fileName.
func (r *SourceResolver) GetFile(typeDir, fileName string) (absPath, relPath string, ok bool) {
	rel := filepath.Join(r.dirFor(typeDir), fileName)
	abs := filepath.Join(r.sourceDir, rel)
	if _, err := r.checkSafePath(abs); err == nil {
		return abs, rel, true
	}
	// Then the reused pakke, for the same reason Get consults it. Without
	// this, MapLocalPath could not name an inherited file's source path and
	// fell back to the local path, which sync then looked for in the wrong
	// repo and reported as deleted upstream.
	if r.base != nil {
		return r.base.GetFile(typeDir, fileName)
	}
	return "", "", false
}

// List discovers all artifacts of a kind.
//
// The local worker agent is not listed while local dispatch is off, which is
// every machine that has never run `nav-pilot alpha local init`. Its description
// promises work that draws no AI credits, and it can only keep that promise once
// a launch has bound it to the local provider; offered anywhere else it is a
// worker that quietly runs on the session's own model and bills for it.
//
// Filtered here rather than at each caller because there are several: the
// materializer, `install --all`, `list`, and the sync that reports what is new.
// A gate in one of them is a gate in none, which is how the agent reached
// machines that never opted in.
func (r *SourceResolver) List(kind *ArtifactKind) []Resolved {
	names := r.discoverNames(kind)
	// A reused pakke's artifacts are listed too, minus the ones this source
	// shadows. Get resolves each name, so a shadowed one still comes from
	// here. The union decides what exists, Get decides where it comes from.
	if r.base != nil {
		seen := make(map[string]bool, len(names))
		for _, name := range names {
			seen[name] = true
		}
		for _, inherited := range r.base.List(kind) {
			if !seen[inherited.Name] {
				seen[inherited.Name] = true
				names = append(names, inherited.Name)
			}
		}
		sort.Strings(names)
	}
	var results []Resolved
	for _, name := range names {
		if kind == KindAgent && name == local.WorkerAgent && !local.Enabled() {
			continue
		}
		if art, ok := r.Get(kind, name); ok {
			results = append(results, art)
		}
	}
	return results
}

func (r *SourceResolver) discoverNames(kind *ArtifactKind) []string {
	seen := make(map[string]bool)
	var names []string

	for _, base := range [1]string{
		filepath.Join(r.sourceDir, r.dirFor(kind.Dir)),
	} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			var name string
			switch {
			case kind.IsDir:
				if !e.IsDir() {
					continue
				}
				name = e.Name()
			case kind.CanBeDir:
				if e.IsDir() {
					name = e.Name()
				} else if strings.HasSuffix(e.Name(), kind.Suffix) {
					name = strings.TrimSuffix(e.Name(), kind.Suffix)
				} else {
					continue
				}
			default:
				if !strings.HasSuffix(e.Name(), kind.Suffix) {
					continue
				}
				name = strings.TrimSuffix(e.Name(), kind.Suffix)
			}
			if seen[name] {
				continue
			}
			if ValidateName(name) == nil {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	sort.Strings(names)
	return names
}

// MapLocalPath maps an installed/state path back to the source path.
func (r *SourceResolver) MapLocalPath(localPath string, isUserScope bool) string {
	sp := filepath.ToSlash(localPath)
	hasSuffix := strings.HasSuffix(sp, "/")

	var rest string
	var hadPrefix bool
	if strings.HasPrefix(sp, ".github/") {
		rest = strings.TrimPrefix(sp, ".github/")
		hadPrefix = true
	} else if isUserScope {
		rest = sp
	} else {
		return sp
	}

	for _, kind := range AllKinds {
		if !strings.HasPrefix(rest, kind.Dir+"/") {
			continue
		}
		remainder := strings.TrimPrefix(rest, kind.Dir+"/")

		if kind.IsDir {
			name := strings.TrimSuffix(remainder, "/")
			if art, ok := r.Get(kind, name); ok {
				if hasSuffix {
					return art.RelPath + "/"
				}
				return art.RelPath
			}
		} else {
			if _, relPath, ok := r.GetFile(kind.Dir, remainder); ok {
				if hasSuffix && !strings.HasSuffix(relPath, "/") {
					return relPath + "/"
				}
				return relPath
			}
		}
		break
	}

	if isUserScope && !hadPrefix {
		result := filepath.ToSlash(filepath.Join(".github", sp))
		if hasSuffix && !strings.HasSuffix(result, "/") {
			result += "/"
		}
		return result
	}

	return sp
}
