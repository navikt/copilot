package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func TestLaunchViaCplt_CpltNotFound(t *testing.T) {
	// Empty PATH so neither cplt nor copilot is resolvable.
	t.Setenv("PATH", t.TempDir())

	err := launchViaCplt(cpltLaunch{
		agent:       "opencode",
		agentArgs:   []string{"--model", "anthropic/claude-sonnet-4-5"},
		displayName: "opencode",
	})
	if err == nil {
		t.Fatal("launchViaCplt must return an error when cplt is not on PATH")
	}
	if !strings.Contains(err.Error(), "cplt") {
		t.Errorf("error should mention cplt, got: %v", err)
	}
	if !strings.Contains(err.Error(), "opencode") {
		t.Errorf("error should mention the client display name, got: %v", err)
	}
}

func TestLaunchOpenCode_RequiresCplt(t *testing.T) {
	// Make opencode resolvable but cplt absent: a temp dir on PATH containing
	// only an executable named "opencode".
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "opencode"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("writing fake opencode: %v", err)
	}
	t.Setenv("PATH", dir)
	// Avoid writing Nav context into the real ~/.config/opencode.
	NavContextDirOverride = t.TempDir()
	t.Cleanup(func() { NavContextDirOverride = "" })

	err := LaunchOpenCode(domain.ResolvedConfig{Client: "opencode", Mode: "default"})
	if err == nil {
		t.Fatal("LaunchOpenCode must return an error when cplt is not on PATH")
	}
	if !strings.Contains(err.Error(), "cplt") {
		t.Errorf("expected cplt-not-found error, got: %v", err)
	}
}
