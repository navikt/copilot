package provider

import (
	"slices"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// Golden launch-argument tests (C3).
//
// These pin the exact argument vectors nav-pilot passes to the clients today,
// byte for byte. They exist so the data-driven persona refactor (C1/C2) is
// provably a no-op for every existing user: if any of these vectors changes,
// some real launch changed, and the change is a regression until proven
// otherwise. They must never be "updated to match" a refactor.

func TestGoldenBuildCopilotArgs(t *testing.T) {
	tests := []struct {
		name     string
		cliName  string
		resolved domain.ResolvedConfig
		want     []string
	}{
		{
			name:    "cplt/zero config",
			cliName: "cplt",
			// AskUser is false in a zero ResolvedConfig, hence --no-ask-user.
			resolved: domain.ResolvedConfig{},
			want:     []string{"--agent", "copilot", "--", "--agent", "nav-pilot", "--no-ask-user"},
		},
		{
			name:     "cplt/ask user",
			cliName:  "cplt",
			resolved: domain.ResolvedConfig{AskUser: true},
			want:     []string{"--agent", "copilot", "--", "--agent", "nav-pilot"},
		},
		{
			name:    "cplt/every field set",
			cliName: "cplt",
			resolved: domain.ResolvedConfig{
				Client:          "copilot",
				Model:           "claude-opus-5",
				Mode:            "autopilot",
				ReasoningEffort: "high",
				ContextTier:     "large",
				AllowAllTools:   true,
				AskUser:         true,
				LogLevel:        "debug",
				ExtraArgs:       []string{"-p", "hei"},
			},
			want: []string{
				"--agent", "copilot", "--",
				"--agent", "nav-pilot",
				"--model", "claude-opus-5",
				"--mode", "autopilot",
				"--effort", "high",
				"--context", "large",
				"--allow-all-tools",
				"--log-level", "debug",
				"-p", "hei",
			},
		},
		{
			name:    "cplt/default mode and context tier are omitted",
			cliName: "cplt",
			resolved: domain.ResolvedConfig{
				Mode:        "default",
				ContextTier: "default",
				AskUser:     true,
			},
			want: []string{"--agent", "copilot", "--", "--agent", "nav-pilot"},
		},
		{
			name:     "copilot/zero config",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{},
			want:     []string{"--agent", "nav-pilot", "--no-ask-user"},
		},
		{
			name:    "copilot/every field set",
			cliName: "copilot",
			resolved: domain.ResolvedConfig{
				Model:           "gpt-5.5",
				Mode:            "plan",
				ReasoningEffort: "low",
				ContextTier:     "small",
				AllowAllTools:   true,
				AskUser:         false,
				LogLevel:        "error",
				ExtraArgs:       []string{"--foo"},
			},
			want: []string{
				"--agent", "nav-pilot",
				"--model", "gpt-5.5",
				"--mode", "plan",
				"--effort", "low",
				"--context", "small",
				"--allow-all-tools",
				"--no-ask-user",
				"--log-level", "error",
				"--foo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCopilotArgs(tt.cliName, tt.resolved)
			if !slices.Equal(got, tt.want) {
				t.Errorf("BuildCopilotArgs(%q, %+v)\n got: %q\nwant: %q", tt.cliName, tt.resolved, got, tt.want)
			}
		})
	}
}

func TestGoldenOpenCodeArgs(t *testing.T) {
	tests := []struct {
		name     string
		resolved domain.ResolvedConfig
		want     []string
	}{
		{
			// No model pinned means no --model: the Nav default reaches
			// opencode through its config (EnsureOpenCodeSessionModel), below
			// each agent's own model line rather than above it.
			name:     "zero config emits no model flag",
			resolved: domain.ResolvedConfig{},
			want:     []string{"--agent", "nav-pilot"},
		},
		{
			name:     "explicit auto model",
			resolved: domain.ResolvedConfig{Model: "auto"},
			want:     []string{"--model", "github-copilot/auto", "--agent", "nav-pilot"},
		},
		{
			name:     "bare copilot model id gains the provider prefix",
			resolved: domain.ResolvedConfig{Model: "claude-sonnet-4.6"},
			want:     []string{"--model", "github-copilot/claude-sonnet-4.6", "--agent", "nav-pilot"},
		},
		{
			name:     "provider-qualified model passes through",
			resolved: domain.ResolvedConfig{Model: "anthropic/claude-opus-5"},
			want:     []string{"--model", "anthropic/claude-opus-5", "--agent", "nav-pilot"},
		},
		{
			name:     "plan mode selects opencode's built-in plan agent",
			resolved: domain.ResolvedConfig{Mode: "plan"},
			want:     []string{"--agent", "plan"},
		},
		{
			name: "every field set",
			resolved: domain.ResolvedConfig{
				Model:           "claude-opus-5",
				Mode:            "autopilot",
				ReasoningEffort: "high",
				ContextTier:     "large",
				AllowAllTools:   true,
				AskUser:         true,
				LogLevel:        "debug",
				ExtraArgs:       []string{"--ignored"},
			},
			want: []string{
				"--model", "github-copilot/claude-opus-5",
				"--agent", "nav-pilot",
				"--variant", "high",
				"--dangerously-skip-permissions",
				"--log-level", "DEBUG",
			},
		},
		{
			name:     "warning log level maps to WARN",
			resolved: domain.ResolvedConfig{LogLevel: "warning"},
			want:     []string{"--agent", "nav-pilot", "--log-level", "WARN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OpenCodeArgs(tt.resolved)
			if !slices.Equal(got, tt.want) {
				t.Errorf("OpenCodeArgs(%+v)\n got: %q\nwant: %q", tt.resolved, got, tt.want)
			}
		})
	}
}

// TestGoldenOpenCodeLaunchAgent pins the cplt agent opencode is launched under,
// alongside the client args above: together they are the full opencode
// invocation LaunchOpenCode builds.
func TestGoldenOpenCodeLaunchAgent(t *testing.T) {
	if got := (openCodeProvider{}).DefaultModel(); got != "github-copilot/auto" {
		t.Errorf("opencode DefaultModel() = %q, want %q", got, "github-copilot/auto")
	}
	if got := ToOpenCodeModel(""); got != "github-copilot/auto" {
		t.Errorf("ToOpenCodeModel(\"\") = %q, want %q", got, "github-copilot/auto")
	}
}
