package provider

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
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
	if err := testhome.WriteExec(fakeCplt, "#!/bin/sh\necho 'cplt version 1.0.43'"); err != nil {
		t.Fatal(err)
	}
	if !IsCplt(fakeCplt) {
		t.Error("expected IsCplt=true for binary that outputs 'cplt'")
	}

	fakeCopilot := filepath.Join(dir, "fake-copilot")
	if err := testhome.WriteExec(fakeCopilot, "#!/bin/sh\necho 'GitHub Copilot CLI 1.0.0'"); err != nil {
		t.Fatal(err)
	}
	if IsCplt(fakeCopilot) {
		t.Error("expected IsCplt=false for binary that outputs 'GitHub Copilot CLI'")
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
			// Without cplt allow_all_tools is not passed on.
			name:     "copilot with allow-all-tools and no-ask-user",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", AllowAllTools: true, AskUser: false},
			want:     []string{"--agent", "nav-pilot", "--model", "gpt-6-sol", "--no-ask-user"},
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
	if err := testhome.WriteExec(fakeCplt, "#!/bin/sh\necho launched > \"$NAV_PILOT_LAUNCH_MARKER\"\n"); err != nil {
		t.Fatalf("write fake cplt: %v", err)
	}
	if err := testhome.WriteExec(filepath.Join(dir, "copilot"), "#!/bin/sh\nexit 0\n"); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)
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
	if err == nil || !strings.Contains(err.Error(), "copilot_auth_mode=env_only") {
		t.Fatalf("expected env_only refusal, got %v", err)
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

// Capture the cplt process's environment, not just the filter's return value:
// the legacy and staged paths build their environments and exec separately.
func TestCopilotLaunchAuthEnvironment(t *testing.T) {
	for _, staged := range []bool{false, true} {
		for _, tc := range []struct {
			mode   string
			tokens [3]string
		}{
			{"auto", [3]string{}},
			{"env_only", [3]string{"gh-env", "github-env", "copilot-env"}},
			{"gh_only", [3]string{"gh-env", "github-env", "copilot-env"}},
		} {
			path := "legacy/"
			if staged {
				path = "staged/"
			}
			t.Run(path+tc.mode, func(t *testing.T) {
				isolateHome(t)
				SetActivePakke(stagedFixturePakke())
				t.Cleanup(func() { SetActivePakke(nil) })
				stubProbes(t, okCplt, nil, "", nil)
				dir := t.TempDir()
				out := filepath.Join(dir, "auth-env")
				// The unset sentinel distinguishes removal from an empty value.
				script := "#!/bin/sh\nprintf '%s\\n' \"${GH_TOKEN-UNSET}\" \"${GITHUB_TOKEN-UNSET}\" \"${COPILOT_GITHUB_TOKEN-UNSET}\" \"$AUTH_TEST_KEEP\" > \"$AUTH_TEST_OUT\"\n"
				if err := testhome.WriteExec(filepath.Join(dir, "cplt"), script); err != nil {
					t.Fatal(err)
				}
				if err := testhome.WriteExec(filepath.Join(dir, "copilot"), "#!/bin/sh\nexit 0\n"); err != nil {
					t.Fatal(err)
				}
				if err := testhome.WriteExec(filepath.Join(dir, "gh"), "#!/bin/sh\nprintf called > \"$AUTH_TEST_OUT.gh\"\nexit 1\n"); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", dir)
				t.Setenv("AUTH_TEST_OUT", out)
				t.Setenv("AUTH_TEST_KEEP", "unrelated")
				for i, name := range ghTokenVars {
					t.Setenv(name, tc.tokens[i])
				}
				r := domain.ResolvedConfig{Client: "copilot", AskUser: true, CopilotAuthMode: tc.mode, OtelLogLevel: "none"}
				diagnosticPath := filepath.Join(dir, "diagnostics")
				diagnostics, err := os.Create(diagnosticPath)
				if err != nil {
					t.Fatal(err)
				}
				stderr := os.Stderr
				os.Stderr = diagnostics
				t.Cleanup(func() { os.Stderr = stderr; diagnostics.Close() })
				if staged {
					err = LaunchCopilotStaged(r, StagedLaunch{Dir: dir, PakkeName: "grillmester", Context: "full"})
				} else {
					err = LaunchCopilotResolved(r)
				}
				if err != nil {
					t.Fatal(err)
				}
				messages, err := os.ReadFile(diagnosticPath)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(messages), "filters only cplt's input environment") != (tc.mode != "auto") {
					t.Fatalf("auth mode warning: %s", messages)
				}
				if strings.Contains(string(messages), "gh auth login") {
					t.Fatalf("launch must not predict authentication: %s", messages)
				}
				if _, err := os.Stat(out + ".gh"); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("launch must not run gh auth commands: %v", err)
				}
				got, err := os.ReadFile(out)
				if err != nil {
					t.Fatal(err)
				}
				want := strings.Join(tc.tokens[:], "\n") + "\nunrelated\n"
				if tc.mode == "gh_only" {
					want = "UNSET\nUNSET\nUNSET\nunrelated\n"
				}
				if string(got) != want {
					t.Fatalf("cplt environment = %q, want %q", got, want)
				}
			})
		}
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

// Neither cplt nor copilot on PATH: the error names what is missing and the
// command that installs it (#1051). It used to be "copilot cli not found".
func TestLaunchCopilotResolved_NoCLINamesTheInstallCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	err := LaunchCopilotResolved(domain.ResolvedConfig{Client: "copilot", AskUser: true, OtelLogLevel: "none"})
	if err == nil || !strings.Contains(err.Error(), "not on PATH") || !strings.Contains(err.Error(), CopilotInstallCommand) {
		t.Fatalf("want the missing binary and %q, got %v", CopilotInstallCommand, err)
	}
}
