package source

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
		src, err := CloneRemoteFn(ref, sourceRepo)
		if err != nil {
			return nil, err
		}
		src.Version = cliVersion
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

	args := []string{"-c", "advice.detachedHead=false", "clone", "--depth", "1", "--quiet"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, repoURL, tmpDir)

	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// Suppress stderr during clone so it doesn't overwrite the spinner unless there's an error
	err = cmd.Run()

	close(done)
	fmt.Fprintf(os.Stderr, "\r\033[K")
	if err != nil {
		os.RemoveAll(tmpDir)
		gitErr := strings.TrimSpace(stderr.String())
		if gitErr != "" {
			gitErr = "\n\n  " + strings.ReplaceAll(gitErr, "\n", "\n  ")
		}
		if ref != "" {
			return nil, fmt.Errorf("could not clone %s@%s — check that the ref exists and you have network access%s", label, ref, gitErr)
		}
		return nil, fmt.Errorf("could not clone %s — check your network connection%s", label, gitErr)
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
