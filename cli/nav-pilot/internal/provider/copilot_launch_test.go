package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
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
			want:     []string{"--agent", "copilot", "--", "--agent", "nav-pilot"},
		},
		{
			name:     "copilot always emits nav-pilot persona",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", AskUser: true},
			want:     []string{"--agent", "nav-pilot"},
		},
		{
			name:     "resolved.Client=copilot still emits --agent nav-pilot (not --agent copilot)",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", AskUser: true},
			want:     []string{"--agent", "nav-pilot"},
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
			want:     []string{"--agent", "nav-pilot", "--allow-all-tools", "--no-ask-user"},
		},
		{
			name:     "default mode not emitted",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", AskUser: true},
			want:     []string{"--agent", "nav-pilot"},
		},
		{
			name:     "default context not emitted",
			cliName:  "copilot",
			resolved: domain.ResolvedConfig{Client: "copilot", Mode: "default", ContextTier: "default", AskUser: true},
			want:     []string{"--agent", "nav-pilot"},
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

func TestUserCopilotDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got := userCopilotDir(); got != "" {
		t.Errorf("expected empty for no customizations, got %q", got)
	}

	agentsDir := filepath.Join(home, ".copilot", "agents")
	_ = os.MkdirAll(agentsDir, 0o755)
	_ = os.WriteFile(filepath.Join(agentsDir, "nav-pilot.agent.md"), []byte("test"), 0o644)

	expected := filepath.Join(home, ".copilot")
	if got := userCopilotDir(); got != expected {
		t.Errorf("expected %q for agents-only, got %q", expected, got)
	}

	_ = os.RemoveAll(agentsDir)
	instrDir := filepath.Join(home, ".copilot", ".github", "instructions")
	_ = os.MkdirAll(instrDir, 0o755)
	_ = os.WriteFile(filepath.Join(instrDir, "golang.instructions.md"), []byte("test"), 0o644)

	if got := userCopilotDir(); got != expected {
		t.Errorf("expected %q for instructions-only, got %q", expected, got)
	}
}
