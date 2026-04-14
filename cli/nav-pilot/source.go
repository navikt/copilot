package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Source holds a resolved source directory and optional temp dir to clean up.
type Source struct {
	Dir     string
	TempDir string
	SHA     string
	Version string // release version (e.g. "2026.04.14-..."), empty for local dev
}

func (s *Source) Cleanup() {
	if s.TempDir != "" {
		os.RemoveAll(s.TempDir)
	}
}

// resolveSource finds the navikt/copilot source. Priority:
//  1. Explicit --ref flag
//  2. Local repo (walk up from CWD to git root — dev mode)
//  3. Clone from the release tag matching this binary's version
//  4. Clone from HEAD (only if version is "dev")
func resolveSource(ref, sourceRepo string) (*Source, error) {
	// If a custom source repo is specified, always clone remote
	if sourceRepo != "" {
		return cloneRemote(ref, sourceRepo)
	}

	if ref != "" {
		return cloneRemote(ref, "")
	}

	// Try local: walk up from CWD to find the navikt/copilot repo.
	// Stop at the git root to avoid matching unrelated repos.
	if wd, err := os.Getwd(); err == nil {
		gitRoot := findGitRoot(wd)
		if gitRoot != "" {
			candidate := filepath.Join(gitRoot, ".github", "collections")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				sha := getGitSHA(gitRoot)
				fmt.Fprintf(os.Stderr, "%s Using local source (%s)\n", dim("→"), dim(gitRoot))
				return &Source{Dir: gitRoot, SHA: sha, Version: version}, nil
			}
		}
	}

	// For released binaries, clone from the matching release tag
	if version != "dev" {
		src, err := cloneRemote("nav-pilot/"+version, "")
		if err != nil {
			return nil, err
		}
		src.Version = version
		return src, nil
	}

	src, err := cloneRemote("", "")
	if err != nil {
		return nil, err
	}
	// Propagate the build-time version so state files always have a version.
	src.Version = version
	return src, nil
}

// findGitRoot walks up from dir to find the nearest .git directory.
func findGitRoot(dir string) string {
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
