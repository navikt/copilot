package source

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// Source holds a resolved source directory and optional temp dir to clean up.
type Source struct {
	Dir     string
	TempDir string
	SHA     string
	Version string // release version (e.g. "2026.04.14-..."), empty for local dev
}

// CloneRemoteFn is overridable in tests.
var CloneRemoteFn = cloneRemote

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
		return CloneRemoteFn(ref, sourceRepo)
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
		return src, nil
	}

	// Try local: walk up from CWD to find the navikt/copilot repo.
	// Stop at the git root to avoid matching unrelated repos.
	if wd, err := os.Getwd(); err == nil {
		gitRoot := FindGitRoot(wd)
		if gitRoot != "" {
			candidate := filepath.Join(gitRoot, ".github", "collections")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				sha := getGitSHA(gitRoot)
				fmt.Fprintf(os.Stderr, "%s Using local source (%s)\n", domain.Dim("→"), domain.Dim(gitRoot))
				return &Source{Dir: gitRoot, SHA: sha, Version: cliVersion}, nil
			}
		}
	}

	// Always clone HEAD of main to get the latest content regardless of binary version
	src, err := CloneRemoteFn("", "")
	if err != nil {
		return nil, err
	}
	src.Version = cliVersion
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

	repoURL := "https://github.com/navikt/copilot.git"
	if sourceRepo != "" {
		repoURL = "https://github.com/" + sourceRepo + ".git"
	}

	label := "navikt/copilot"
	if sourceRepo != "" {
		label = sourceRepo
	}
	if ref != "" {
		fmt.Fprintf(os.Stderr, "%s Fetching %s@%s...\n", domain.Dim("→"), label, ref)
	} else {
		fmt.Fprintf(os.Stderr, "%s Fetching %s...\n", domain.Dim("→"), label)
	}

	args := []string{"-c", "advice.detachedHead=false", "clone", "--depth", "1", "--quiet"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, repoURL, tmpDir)

	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		if ref != "" {
			return nil, fmt.Errorf("could not clone navikt/copilot@%s — check that the ref exists and you have network access.\n  See releases: https://github.com/navikt/copilot/releases", ref)
		}
		return nil, fmt.Errorf("could not clone navikt/copilot — check your network connection")
	}

	sha := getGitSHA(tmpDir)
	return &Source{Dir: tmpDir, TempDir: tmpDir, SHA: sha}, nil
}

func getGitSHA(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
