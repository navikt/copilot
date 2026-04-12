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
}

func (s *Source) Cleanup() {
	if s.TempDir != "" {
		os.RemoveAll(s.TempDir)
	}
}

// resolveSource finds the navikt/copilot source. Priority:
//  1. Explicit --ref flag
//  2. Local repo (walk up from CWD — dev mode)
//  3. Clone from the release tag matching this binary's version
//  4. Clone from HEAD (only if version is "dev")
func resolveSource(ref string) (*Source, error) {
	if ref != "" {
		return cloneRemote(ref)
	}

	// Try local: walk up from CWD to find the navikt/copilot repo
	if wd, err := os.Getwd(); err == nil {
		for d := wd; ; d = filepath.Dir(d) {
			candidate := filepath.Join(d, ".github", "collections")
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				sha := getGitSHA(d)
				fmt.Printf("%s Using local source (%s)\n", dim("→"), dim(d))
				return &Source{Dir: d, SHA: sha}, nil
			}
			if d == filepath.Dir(d) {
				break
			}
		}
	}

	// For released binaries, clone from the matching release tag
	if version != "dev" {
		return cloneRemote("nav-pilot/" + version)
	}

	return cloneRemote("")
}

func cloneRemote(ref string) (*Source, error) {
	tmpDir, err := os.MkdirTemp("", "nav-pilot-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	args := []string{"clone", "--depth", "1", "--quiet"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, "https://github.com/navikt/copilot.git", tmpDir)

	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		if ref != "" {
			return nil, fmt.Errorf("cloning navikt/copilot@%s: %w", ref, err)
		}
		return nil, fmt.Errorf("cloning navikt/copilot: %w", err)
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
