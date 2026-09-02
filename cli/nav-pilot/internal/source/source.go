package source

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// DefaultRepo is the content source used when neither --source nor the config
// file's source key names one.
const DefaultRepo = "navikt/copilot"

// activePakke is the agentpakke whose declarations — launch persona, opencode
// primary-agent allowlist, model defaults — drive launch and materialization.
// It defaults to the in-memory legacy adapter, so no default path reads a file
// to learn them.
//
// It is held here rather than in internal/provider because internal/artifacts
// needs the same declarations when it materializes agent frontmatter, and
// artifacts cannot import provider. [provider.SetActivePakke] is the seam
// callers use.
var activePakke = agentpakke.Default()

// SetActivePakke sets the agentpakke read by launch and materialization. A nil
// manifest restores the built-in default. Call it through
// provider.SetActivePakke.
func SetActivePakke(m *agentpakke.Manifest) {
	if m == nil {
		m = agentpakke.Default()
	}
	activePakke = m
}

// ActivePakke returns the active agentpakke. Never nil.
func ActivePakke() *agentpakke.Manifest { return activePakke }

// Source holds a resolved source directory and optional temp dir to clean up.
type Source struct {
	Dir     string
	TempDir string
	SHA     string
	Version string // release version (e.g. "2026.04.14-..."), empty for local dev
	Repo    string // git repository owner/name (e.g. "navikt/copilot")

	// Pakke is the agentpakke manifest shipped by this checkout, or nil when it
	// ships none. It is attached by the CLI's source funnel
	// (internal/cli/aliases.go) rather than by ResolveSource, so that
	// `nav-pilot validate` can resolve a non-conforming source and report its
	// findings instead of failing at resolution.
	//
	// nil is not "no agentpakke": callers turn it into a Manifest with
	// agentpakke.SynthesizeLegacy so everything downstream reads one type (see
	// the agentpakke package doc's migration path).
	Pakke *agentpakke.Manifest
}

// ValidateSourceValue checks a persisted or flag-provided source value. Accepted
// forms mirror what ResolveSource understands: a GitHub "owner/name" repo, or an
// absolute path to a local checkout.
func ValidateSourceValue(v string) error {
	if v != strings.TrimSpace(v) {
		return fmt.Errorf("source %q must not be padded with whitespace", v)
	}
	if v == "" {
		return fmt.Errorf("source must not be empty")
	}
	if filepath.IsAbs(v) {
		return nil
	}
	if strings.HasPrefix(v, "~") || strings.HasPrefix(v, ".") {
		return fmt.Errorf("source %q must be a GitHub repo (owner/name) or an absolute path", v)
	}
	owner, name, ok := strings.Cut(v, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("source %q must be a GitHub repo (owner/name) or an absolute path", v)
	}
	return nil
}

// CloneRemoteFn is overridable in tests.
var CloneRemoteFn = cloneRemote

// RemoteURLFn maps a source repo to the git URL it is fetched from. It is a
// variable so tests can point the real clone path at a repository on disk and
// exercise git for real; nothing else overrides it.
var RemoteURLFn = func(sourceRepo string) string {
	if sourceRepo == "" {
		sourceRepo = DefaultRepo
	}
	return "https://github.com/" + sourceRepo + ".git"
}

func (s *Source) Cleanup() {
	if s.TempDir != "" {
		os.RemoveAll(s.TempDir)
	}
}

// ResolveSource finds the navikt/copilot source. Priority:
//  1. Explicit --ref flag
//  2. Local repo (walk up from CWD to git root — dev mode)
//  3. Clone HEAD of main (always gets latest content)
func ResolveSource(ref, sourceRepo, cliVersion string) (*Source, error) {
	// If a custom source repo is specified, always clone remote
	if sourceRepo != "" {
		if filepath.IsAbs(sourceRepo) {
			if info, err := os.Stat(sourceRepo); err == nil && info.IsDir() {
				sha := getGitSHA(sourceRepo)
				return &Source{Dir: sourceRepo, SHA: sha, Version: cliVersion, Repo: sourceRepo}, nil
			}
		}
		src, err := CloneRemoteFn(ref, sourceRepo)
		if err != nil {
			return nil, err
		}
		src.Version = cliVersion
		src.Repo = sourceRepo
		return src, nil
	}

	if ref != "" {
		src, err := CloneRemoteFn(ref, "")
		if err != nil {
			return nil, err
		}
		// Extract version from nav-pilot/<version> style refs
		if v := strings.TrimPrefix(ref, "nav-pilot/"); v != ref {
			src.Version = v
		}
		src.Repo = DefaultRepo
		return src, nil
	}

	// Try local: walk up from CWD to find the navikt/copilot repo.
	// Stop at the git root to avoid matching unrelated repos.
	if wd, err := os.Getwd(); err == nil {
		gitRoot := FindGitRoot(wd)
		if gitRoot != "" {
			candidate := filepath.Join(gitRoot, "collections")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				sha := getGitSHA(gitRoot)
				fmt.Fprintf(os.Stderr, "%s Using local source (%s)\n", domain.Dim("→"), domain.Dim(gitRoot))
				return &Source{Dir: gitRoot, SHA: sha, Version: cliVersion, Repo: gitRoot}, nil
			}
		}
	}

	// Always clone HEAD of main to get the latest content regardless of binary version
	src, err := CloneRemoteFn("", "")
	if err != nil {
		return nil, err
	}
	src.Version = cliVersion
	src.Repo = DefaultRepo
	return src, nil
}

// ResolveSourceForSync resolves source for sync checks.
// Unlike ResolveSource, it skips local repo auto-detection when no ref/source
// is provided, so sync compares against upstream content by default.
func ResolveSourceForSync(ref, sourceRepo, cliVersion string) (*Source, error) {
	if sourceRepo != "" || ref != "" {
		return ResolveSource(ref, sourceRepo, cliVersion)
	}
	src, err := CloneRemoteFn("", "")
	if err != nil {
		return nil, err
	}
	src.Version = cliVersion
	return src, nil
}

// FindGitRoot walks up from dir to find the nearest .git directory.
func FindGitRoot(dir string) string {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if d == filepath.Dir(d) {
			return ""
		}
	}
}

func cloneRemote(ref, sourceRepo string) (*Source, error) {
	tmpDir, err := os.MkdirTemp("", "nav-pilot-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	repoURL := RemoteURLFn(sourceRepo)

	label := DefaultRepo
	if sourceRepo != "" {
		label = sourceRepo
	}
	msg := fmt.Sprintf("Fetching %s...", label)
	if ref != "" {
		msg = fmt.Sprintf("Fetching %s@%s...", label, ref)
	}

	done := make(chan struct{})
	go func() {
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "\r%s %s", domain.Dim(frames[i%len(frames)]), msg)
				i++
			}
		}
	}()

	var stderr bytes.Buffer
	err = fetchRevision(tmpDir, repoURL, ref, &stderr)

	close(done)
	fmt.Fprintf(os.Stderr, "\r\033[K")
	if err != nil {
		os.RemoveAll(tmpDir)
		gitErr := strings.TrimSpace(stderr.String())
		if gitErr != "" {
			gitErr = "\n\n  " + strings.ReplaceAll(gitErr, "\n", "\n  ")
		}
		isAuthFailure := strings.Contains(gitErr, "Authentication failed") ||
			strings.Contains(gitErr, "could not read Username") ||
			strings.Contains(gitErr, "Permission denied")
		if isAuthFailure {
			return nil, fmt.Errorf("could not clone %s: authentication failed. Check your SSH keys/git credentials or set GITHUB_TOKEN.%s", label, gitErr)
		}
		if ref != "" {
			return nil, fmt.Errorf("could not clone %s@%s — check that the ref exists and you have network access%s", label, ref, gitErr)
		}
		return nil, fmt.Errorf("could not clone %s — check your network connection%s", label, gitErr)
	}

	sha := getGitSHA(tmpDir)
	repo := DefaultRepo
	if sourceRepo != "" {
		repo = sourceRepo
	}
	return &Source{Dir: tmpDir, TempDir: tmpDir, SHA: sha, Repo: repo}, nil
}

// fetchRevision materializes one revision of repoURL into dir.
//
// It is a fetch rather than a `git clone --branch <ref>`, because --branch
// takes a branch or tag name and never a commit SHA — which made the pinned
// revision in a repo's declaration impossible to install back. Fetching the ref
// directly and checking out FETCH_HEAD resolves all three of branch, tag and
// full commit SHA through one path, and keeps the shallow-clone cheapness.
//
// An abbreviated SHA is not fetchable: git wants a full object id in a fetch
// request. That is why [getGitSHA] records all forty characters.
func fetchRevision(dir, repoURL, ref string, stderr *bytes.Buffer) error {
	// An empty ref means the remote's default branch, which is what HEAD names.
	if ref == "" {
		ref = "HEAD"
	}
	steps := [][]string{
		{"init", "--quiet", "-b", "main"},
		{"remote", "add", "origin", repoURL},
		{"fetch", "--depth", "1", "--quiet", "origin", ref},
		{"-c", "advice.detachedHead=false", "checkout", "--quiet", "FETCH_HEAD"},
	}
	for _, args := range steps {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		// Suppressed during the fetch so it does not overwrite the spinner;
		// the caller prints it when something actually failed.
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

// getGitSHA reports the full commit id of a checkout. Full, not abbreviated:
// this value is written to a repo's declaration as a pin, and git can only
// fetch a commit named at full length.
func getGitSHA(dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
