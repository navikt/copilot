package provider

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

func TestFindCopilotCLI(t *testing.T) {
	path, name := FindCopilotCLI()
	if path != "" {
		if name != "cplt" && name != "copilot" {
			t.Errorf("expected name 'cplt' or 'copilot', got %q", name)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("returned path %q does not exist: %v", path, err)
		}
	} else if name != "" {
		t.Errorf("expected empty name when path is empty, got %q", name)
	}
}

func TestCLIDisplayName(t *testing.T) {
	if got := CLIDisplayName("cplt"); got != "Copilot Sandbox (cplt)" {
		t.Errorf("CLIDisplayName(cplt) = %q", got)
	}
	if got := CLIDisplayName("copilot"); got != "copilot" {
		t.Errorf("CLIDisplayName(copilot) = %q", got)
	}
}

func TestCopilotAgentArgs(t *testing.T) {
	tests := []struct {
		agent string
		want  []string
	}{
		{"nav-pilot", nil},
		{"auth", nil},
		{"", nil},
	}
	for _, tt := range tests {
		got := copilotAgentArgs(tt.agent)
		if len(got) != len(tt.want) {
			t.Errorf("copilotAgentArgs(%q) = %v, want %v", tt.agent, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("copilotAgentArgs(%q)[%d] = %q, want %q", tt.agent, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIsCplt(t *testing.T) {
	dir := t.TempDir()
	fakeCplt := filepath.Join(dir, "fake-cplt")
	_ = os.WriteFile(fakeCplt, []byte("#!/bin/sh\necho 'cplt version 1.0.43'"), 0o755)
	if !isCplt(fakeCplt) {
		t.Error("expected isCplt=true for binary that outputs 'cplt'")
	}

	fakeCopilot := filepath.Join(dir, "fake-copilot")
	_ = os.WriteFile(fakeCopilot, []byte("#!/bin/sh\necho 'GitHub Copilot CLI 1.0.0'"), 0o755)
	if isCplt(fakeCopilot) {
		t.Error("expected isCplt=false for binary that outputs 'GitHub Copilot CLI'")
	}
}

func TestBuildCopilotArgs(t *testing.T) {
	tests := []struct {
		name     string
		cliName  string
		resolved domain.ResolvedConfig
		want     []string
	}{
		{
			name:     "cplt pins copilot sandbox agent and emits nav-pilot persona",
			cliName:  "cplt",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", AskUser: true},
			want:     []string{"--agent", "copilot", "--", "--agent", "nav-pilot", "--model", "gpt-6-sol"},
		},
		{
			name:     "copilot always emits nav-pilot persona",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", AskUser: true},
			want:     []string{"--agent", "nav-pilot", "--model", "gpt-6-sol"},
		},
		{
			name:     "resolved.Client=copilot still emits --agent nav-pilot (not --agent copilot)",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", AskUser: true},
			want:     []string{"--agent", "nav-pilot", "--model", "gpt-6-sol"},
		},
		{
			name:     "copilot with model and mode",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Model: "gpt-4o", Mode: "plan", AskUser: true},
			want:     []string{"--agent", "nav-pilot", "--model", "gpt-4o", "--mode", "plan"},
		},
		{
			name:    "cplt with all flags",
			cliName: "cplt",
			resolved: domain.ResolvedConfig{
				Client:          "copilot",
				Model:           "gpt-4o",
				Mode:            "plan",
				ReasoningEffort: "high",
				ContextTier:     "long_context",
				AllowAllTools:   true,
				AskUser:         false,
				LogLevel:        "debug",
			},
			want: []string{"--agent", "copilot", "--", "--agent", "nav-pilot", "--model", "gpt-4o",
				"--mode", "plan", "--effort", "high", "--context", "long_context",
				"--allow-all-tools", "--no-ask-user", "--log-level", "debug"},
		},
		{
			name:     "copilot with allow-all-tools and no-ask-user",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", AllowAllTools: true, AskUser: false},
			want:     []string{"--agent", "nav-pilot", "--model", "gpt-6-sol", "--allow-all-tools", "--no-ask-user"},
		},
		{
			name:     "default mode not emitted",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", AskUser: true},
			want:     []string{"--agent", "nav-pilot", "--model", "gpt-6-sol"},
		},
		{
			name:     "default context not emitted",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", ContextTier: "default", AskUser: true},
			want:     []string{"--agent", "nav-pilot", "--model", "gpt-6-sol"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCopilotArgs(tt.cliName, tt.resolved)
			if len(got) != len(tt.want) {
				t.Fatalf("BuildCopilotArgs(%q, ...) = %v, want %v", tt.cliName, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("BuildCopilotArgs(%q, ...)[%d] = %q, want %q", tt.cliName, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestUserInstructionsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := userInstructionsDir(); got != "" {
		t.Errorf("expected empty for no customizations, got %q", got)
	}

	// Copilot reads ~/.copilot/agents itself; agents alone need no injection.
	agentsDir := filepath.Join(home, ".copilot", "agents")
	_ = os.MkdirAll(agentsDir, 0o755)
	_ = os.WriteFile(filepath.Join(agentsDir, "nav-pilot.agent.md"), []byte("test"), 0o644)
	if got := userInstructionsDir(); got != "" {
		t.Errorf("expected empty for agents-only, got %q", got)
	}

	instrDir := filepath.Join(home, ".copilot", ".github", "instructions")
	_ = os.MkdirAll(instrDir, 0o755)
	_ = os.WriteFile(filepath.Join(instrDir, "golang.instructions.md"), []byte("test"), 0o644)
	if got := userInstructionsDir(); got != instrDir {
		t.Errorf("expected %q with instructions installed, got %q", instrDir, got)
	}
}

// TestCopilotEnvScopesInstructionsDir pins that the injected directory is
// nav-pilot's instructions directory, never ~/.copilot. Copilot searches the
// directory recursively, and ~/.copilot/session-state holds other sessions'
// worktrees with their own *.instructions.md files.
func TestCopilotEnvScopesInstructionsDir(t *testing.T) {
	const key = "COPILOT_CUSTOM_INSTRUCTIONS_DIRS"
	home := t.TempDir()
	t.Setenv("HOME", home)
	instrDir := filepath.Join(home, ".copilot", ".github", "instructions")
	worktree := filepath.Join(home, ".copilot", "session-state", "abc", "files", "repo-worktree", "instructions")
	for _, d := range []string{instrDir, worktree} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "golang.instructions.md"), []byte("# Go\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name, existing, want string
	}{
		{"unset", "", instrDir},
		{"appends to user value", "/team/instructions", "/team/instructions," + instrDir},
		{"already present", "/a, " + instrDir, "/a, " + instrDir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(key, tt.existing)
			got := telemetry.LookupEnvValue(CopilotEnv(""), key)
			if got != tt.want {
				t.Errorf("%s = %q, want %q", key, got, tt.want)
			}
			for _, p := range strings.Split(got, ",") {
				if strings.TrimSpace(p) == filepath.Join(home, ".copilot") {
					t.Errorf("%s contains ~/.copilot, which pulls in session-state worktrees", key)
				}
			}
		})
	}
}

// TestLaunchCopilotResolved_EnvOnlyWithoutToken_DoesNotLaunchCplt pins the
// fail-closed half of copilot_auth_mode: env_only refuses to launch rather than
// letting cplt fall back to `gh auth token`.
func TestLaunchCopilotResolved_EnvOnlyWithoutToken_DoesNotLaunchCplt(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "cplt-launched")
	fakeCplt := filepath.Join(dir, "cplt")
	if err := os.WriteFile(fakeCplt, []byte("#!/bin/sh\necho launched > \"$NAV_PILOT_LAUNCH_MARKER\"\n"), 0o755); err != nil {
		t.Fatalf("write fake cplt: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("NAV_PILOT_LAUNCH_MARKER", marker)
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("COPILOT_GITHUB_TOKEN", "")
	t.Setenv("NAV_PILOT_CPLT_HINT", "0")

	err := LaunchCopilotResolved(domain.ResolvedConfig{
		Client:          "copilot",
		AskUser:         true,
		CopilotAuthMode: "env_only",
		OtelLogLevel:    "none",
	})
	if err == nil {
		t.Fatal("expected launch to fail for env_only without a token")
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("expected cplt not to launch for env_only without a token")
	}
}

func TestApplyCopilotAuthMode(t *testing.T) {
	withToken := []string{"PATH=/bin", "GITHUB_TOKEN=abc"}

	// auto leaves the environment alone; cplt decides.
	got, err := applyCopilotAuthMode(withToken, "auto")
	if err != nil || len(got) != len(withToken) {
		t.Fatalf("auto: got %v, %v", got, err)
	}

	// An unresolved mode behaves as auto so a hand-built ResolvedConfig cannot
	// hard-fail a launch.
	if _, err := applyCopilotAuthMode(withToken, ""); err != nil {
		t.Fatalf("empty mode: %v", err)
	}

	// env_only accepts any of the three names, blank values excluded.
	if _, err := applyCopilotAuthMode(withToken, "env_only"); err != nil {
		t.Fatalf("env_only with GITHUB_TOKEN: %v", err)
	}
	if _, err := applyCopilotAuthMode([]string{"PATH=/bin", "GH_TOKEN=  "}, "env_only"); err == nil {
		t.Fatal("env_only: blank GH_TOKEN must not count as a token")
	}

	// gh_only removes every token variable so cplt has nothing to inherit.
	stripped, err := applyCopilotAuthMode(
		[]string{"PATH=/bin", "GH_TOKEN=a", "GITHUB_TOKEN=b", "COPILOT_GITHUB_TOKEN=c"}, "gh_only")
	if err != nil {
		t.Fatalf("gh_only: %v", err)
	}
	if len(stripped) != 1 || stripped[0] != "PATH=/bin" {
		t.Fatalf("gh_only: expected only PATH to survive, got %v", stripped)
	}

	if _, err := applyCopilotAuthMode(withToken, "nonsense"); err == nil {
		t.Fatal("expected an unknown auth mode to be rejected")
	}
}

// TestApplyCopilotAuthMode_WindowsCaseInsensitivity pins that the matcher
// follows the OS rule for environment variable names. On Windows a lower-case
// gh_token is the same variable, so gh_only has to strip it and env_only has to
// accept it; everywhere else it is a different variable and must be left alone.
func TestApplyCopilotAuthMode_WindowsCaseInsensitivity(t *testing.T) {
	mixed := []string{"PATH=/bin", "gh_token=abc"}

	orig := envNamesCaseInsensitive
	t.Cleanup(func() { envNamesCaseInsensitive = orig })

	envNamesCaseInsensitive = true
	if _, err := applyCopilotAuthMode(mixed, "env_only"); err != nil {
		t.Fatalf("windows env_only should accept gh_token: %v", err)
	}
	stripped, err := applyCopilotAuthMode(mixed, "gh_only")
	if err != nil {
		t.Fatalf("windows gh_only: %v", err)
	}
	if len(stripped) != 1 {
		t.Fatalf("windows gh_only must strip gh_token, got %v", stripped)
	}

	envNamesCaseInsensitive = false
	if _, err := applyCopilotAuthMode(mixed, "env_only"); err == nil {
		t.Fatal("unix env_only must not accept gh_token as GH_TOKEN")
	}
	kept, err := applyCopilotAuthMode(mixed, "gh_only")
	if err != nil {
		t.Fatalf("unix gh_only: %v", err)
	}
	if len(kept) != 2 {
		t.Fatalf("unix gh_only must leave gh_token alone, got %v", kept)
	}
}
