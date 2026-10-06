package provider

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

// grantsAllowAll is the test's own reading of an allow-all flag, independent
// of isAllowAllFlag so a bug there cannot hide itself.
func grantsAllowAll(a string) bool {
	return strings.HasPrefix(a, "--allow-all") || strings.HasPrefix(a, "--yolo")
}

// agentArgs is the part of a vector that reaches copilot: after cplt's "--".
func agentArgs(args []string) []string {
	if i := slices.Index(args, "--"); i >= 0 {
		return args[i+1:]
	}
	return args
}

// TestCopilotAutonomyArgs pins what each autonomy setting grants, with and
// without cplt. Allow-all without the sandbox is the one thing that must never
// happen, whatever the config or the command line says.
func TestCopilotAutonomyArgs(t *testing.T) {
	allowAll := []string{"--allow-all-tools", "--allow-all-paths", "--allow-all-urls"}
	tests := []struct {
		name     string
		cliName  string
		resolved domain.ResolvedConfig
		want     []string // allow-all flags copilot gets
	}{
		{"sandbox under cplt", "cplt", domain.ResolvedConfig{Autonomy: "sandbox", AskUser: true}, allowAll},
		{"conservative under cplt", "cplt", domain.ResolvedConfig{Autonomy: "conservative", AskUser: true}, nil},
		{"conservative keeps allow_all_tools under cplt", "cplt", domain.ResolvedConfig{Autonomy: "conservative", AllowAllTools: true, AskUser: true}, []string{"--allow-all-tools"}},
		{"sandbox without cplt", "copilot", domain.ResolvedConfig{Autonomy: "sandbox", AskUser: true}, nil},
		{"allow_all_tools without cplt", "copilot", domain.ResolvedConfig{AllowAllTools: true, AskUser: true}, nil},
		{"extra args without cplt", "copilot", domain.ResolvedConfig{AskUser: true, ExtraArgs: []string{"--yolo", "--allow-all", "--allow-all-tools=true", "--allow-all-paths", "--allow-all-urls", "-p", "hei"}}, nil},
		{"extra args under cplt", "cplt", domain.ResolvedConfig{AskUser: true, ExtraArgs: []string{"--yolo"}}, []string{"--yolo"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agentArgs(BuildCopilotArgs(tt.cliName, tt.resolved))
			granted := slices.DeleteFunc(slices.Clone(got), func(a string) bool { return !grantsAllowAll(a) })
			if !slices.Equal(granted, tt.want) {
				t.Errorf("allow-all flags = %q, want %q (argv %q)", granted, tt.want, got)
			}
			if slices.Contains(tt.resolved.ExtraArgs, "-p") && !slices.Contains(got, "hei") {
				t.Errorf("a non-allow-all extra arg was dropped: %q", got)
			}
			if slices.Contains(got, "--no-ask-user") {
				t.Errorf("ask_user = true must not pass --no-ask-user: %q", got)
			}
		})
	}
}

// TestAutopilotTakesAskUserAway: autopilot answers ask_user itself, so the
// tool goes, and the launch says why.
func TestAutopilotTakesAskUserAway(t *testing.T) {
	for _, cliName := range []string{"cplt", "copilot"} {
		got := BuildCopilotArgs(cliName, domain.ResolvedConfig{Mode: "autopilot", AskUser: true, Autonomy: "sandbox"})
		if !slices.Contains(got, "--no-ask-user") {
			t.Errorf("%s: autopilot without --no-ask-user: %q", cliName, got)
		}
	}
	if autopilotNote(domain.ResolvedConfig{Mode: "autopilot"}) == "" {
		t.Error("autopilot prints no warning")
	}
	if autopilotNote(domain.ResolvedConfig{Mode: "default", ExtraArgs: []string{"--autopilot"}}) == "" {
		t.Error("-- --autopilot prints no warning")
	}
	if autopilotNote(domain.ResolvedConfig{Mode: "default"}) != "" {
		t.Error("default mode prints the autopilot warning")
	}
}

// TestCopilotLaunchSpawnsOnlyTheClient guards the launch hot path: starting a
// session runs cplt and nothing else. PATH holds a logging stand-in for every
// command on the real PATH, so any new subprocess found by name shows up in
// the log. A new spawn here costs every user every launch; if one is really
// needed, measure it and extend the allowed list on purpose.
func TestCopilotLaunchSpawnsOnlyTheClient(t *testing.T) {
	isolateHome(t)
	t.Setenv("NAV_PILOT_CPLT_HINT", "0")
	dir := t.TempDir()
	log := filepath.Join(dir, "spawned.log")
	logger := filepath.Join(t.TempDir(), "logger")
	if err := testhome.WriteExec(logger, "#!/bin/sh\necho \"${0##*/}\" >> "+log+"\n"); err != nil {
		t.Fatal(err)
	}
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		entries, _ := os.ReadDir(p)
		for _, e := range entries {
			_ = os.Symlink(logger, filepath.Join(dir, e.Name())) // first one wins, as on PATH
		}
	}
	// cplt and copilot as real files: nav-pilot hands off to cplt only when a
	// different copilot is beside it.
	for _, name := range []string{"cplt", "copilot"} {
		_ = os.Remove(filepath.Join(dir, name))
		if err := testhome.WriteExec(filepath.Join(dir, name), "#!/bin/sh\necho "+name+" >> "+log+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)

	for _, r := range []domain.ResolvedConfig{
		{Client: "copilot", AskUser: true, Autonomy: "sandbox", OtelLogLevel: "none"},
		{Client: "copilot", AskUser: true, Autonomy: "conservative", AllowAllTools: true, Mode: "autopilot", OtelLogLevel: "none"},
	} {
		_ = os.Remove(log)
		if err := LaunchCopilotResolved(r); err != nil {
			t.Fatalf("LaunchCopilotResolved: %v", err)
		}
		raw, _ := os.ReadFile(log)
		if got := strings.Fields(string(raw)); !slices.Equal(got, []string{"cplt"}) {
			t.Errorf("launch spawned %q, want only cplt", got)
		}
	}
}

// TestLaunchWithoutCpltFails: with a plain copilot and no cplt on PATH the
// launch fails with install guidance, and copilot never runs.
func TestLaunchWithoutCpltFails(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "argv.txt")
	if err := testhome.WriteExec(filepath.Join(dir, "copilot"), "#!/bin/sh\ntouch "+out+"\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	err := LaunchCopilotResolved(domain.ResolvedConfig{Client: "copilot", OtelLogLevel: "none"})
	if err == nil || !strings.Contains(err.Error(), "install cplt") {
		t.Fatalf("want install guidance, got %v", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Fatal("copilot ran without cplt")
	}
}
