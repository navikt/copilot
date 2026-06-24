package provider

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func requirePOSIXShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX shell test scripts")
	}
}

func TestLaunchViaCplt_CpltNotFound(t *testing.T) {
	requirePOSIXShell(t)
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
	requirePOSIXShell(t)
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

func TestLaunchViaCplt_UsesRTKWhenOptedInAndInteractive(t *testing.T) {
	requirePOSIXShell(t)
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	if err := os.WriteFile(filepath.Join(dir, "cplt"), []byte("#!/bin/sh\nprintf '%s' \"$*\" > \""+argvFile+"\"\n"), 0o755); err != nil {
		t.Fatalf("writing fake cplt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rtk"), []byte("#!/bin/sh\nprintf '%s' \"$*\" > \""+argvFile+"\"\n"), 0o755); err != nil {
		t.Fatalf("writing fake rtk: %v", err)
	}

	t.Setenv("PATH", dir)
	rtk := rtkDeps{
		getenv:        func(string) string { return "1" },
		lookPath:      func(string) (string, error) { return filepath.Join(dir, "rtk"), nil },
		isInteractive: func() bool { return true },
	}

	err := launchViaCpltWithDeps(cpltLaunch{
		agent:       "opencode",
		agentArgs:   []string{"--model", "github-copilot/gpt-5.4"},
		env:         os.Environ(),
		displayName: "opencode",
	}, rtk)
	if err != nil {
		t.Fatalf("launchViaCplt() error: %v", err)
	}

	got, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("reading argv file: %v", err)
	}
	want := filepath.Join(dir, "cplt") + " --agent opencode -- --model github-copilot/gpt-5.4"
	if string(got) != want {
		t.Errorf("rtk argv = %q, want %q", string(got), want)
	}
}

func TestLaunchViaCplt_SkipsRTKWhenNotInteractive(t *testing.T) {
	requirePOSIXShell(t)
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")

	if err := os.WriteFile(filepath.Join(dir, "cplt"), []byte("#!/bin/sh\nprintf '%s' \"$*\" > \""+argvFile+"\"\n"), 0o755); err != nil {
		t.Fatalf("writing fake cplt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rtk"), []byte("#!/bin/sh\nprintf '%s' \"$*\" > \""+argvFile+"\"\n"), 0o755); err != nil {
		t.Fatalf("writing fake rtk: %v", err)
	}

	t.Setenv("PATH", dir)
	rtk := rtkDeps{
		getenv:        func(string) string { return "1" },
		lookPath:      func(string) (string, error) { return filepath.Join(dir, "rtk"), nil },
		isInteractive: func() bool { return false },
	}

	err := launchViaCpltWithDeps(cpltLaunch{
		agent:       "pi",
		displayName: "pi",
		env:         os.Environ(),
	}, rtk)
	if err != nil {
		t.Fatalf("launchViaCplt() error: %v", err)
	}

	got, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("reading argv file: %v", err)
	}
	want := "--agent pi --"
	if string(got) != want {
		t.Errorf("cplt argv = %q, want %q", string(got), want)
	}
}
