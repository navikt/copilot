package provider

import (
	"slices"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func TestSetActivePakke(t *testing.T) {
	t.Cleanup(func() { SetActivePakke(nil) })

	if got := PrimaryAgent("copilot"); got != "nav-pilot" {
		t.Fatalf("default PrimaryAgent(copilot) = %q, want nav-pilot", got)
	}

	SetActivePakke(&agentpakke.Manifest{
		Name: "grillmester",
		Clients: map[string]agentpakke.ClientEntry{
			"copilot":  {PrimaryAgents: []string{"grillmester"}},
			"opencode": {PrimaryAgents: []string{"grillmester", "sous"}, DefaultModel: "github-copilot/claude-opus-5"},
			// No primaryAgents. There is no fallback to the built-in
			// default's persona (WP3): the manifest is authoritative, and
			// schemas/agentpakke-v1.json makes this shape unrepresentable in a
			// loaded manifest anyway (primaryAgents is required, minItems 1).
			"pi": {},
		},
	})

	if got := PrimaryAgent("copilot"); got != "grillmester" {
		t.Errorf("PrimaryAgent(copilot) = %q, want grillmester", got)
	}
	if got := PrimaryAgent("opencode"); got != "grillmester" {
		t.Errorf("PrimaryAgent(opencode) = %q, want grillmester (first primary)", got)
	}
	if got := PrimaryAgent("pi"); got != "" {
		t.Errorf("PrimaryAgent(pi) = %q, want \"\" — no fallback to the built-in persona", got)
	}
	if got := ToOpenCodeModel(""); got != "github-copilot/claude-opus-5" {
		t.Errorf("ToOpenCodeModel(\"\") = %q, want the active pakke's default model", got)
	}
	if got := OpenCodeArgs(domain.ResolvedConfig{}); got[3] != "grillmester" {
		t.Errorf("OpenCodeArgs persona = %q, want grillmester", got[3])
	}

	SetActivePakke(nil)
	if got := PrimaryAgent("copilot"); got != "nav-pilot" {
		t.Errorf("after SetActivePakke(nil): PrimaryAgent(copilot) = %q, want nav-pilot", got)
	}
	if got := ToOpenCodeModel(""); got != OpenCodeDefaultModel {
		t.Errorf("after SetActivePakke(nil): ToOpenCodeModel(\"\") = %q, want %q", got, OpenCodeDefaultModel)
	}
}

// TestBuildCopilotArgsPakkeModel pins the Tier 1 copilot fallback added
// alongside the copilot DefaultModel declaration: the legacy launch consults
// the active agentpakke exactly like the staged Tier 2 path
// (buildStagedCopilotSpec) and like Tier 1 opencode (ToOpenCodeModel).
//
// The built-in default case is the no-behaviour-change proof: it declares
// agentpakke.InheritModel, and inherit must emit no --model at all.
func TestBuildCopilotArgsPakkeModel(t *testing.T) {
	tests := []struct {
		name      string
		declared  string // "" means leave the built-in default active
		userModel string
		want      []string
	}{
		{
			name: "built-in default declares inherit and emits no model",
			want: []string{"--agent", "nav-pilot"},
		},
		{
			name:     "pakke declaration supplies the default model",
			declared: "claude-opus-5",
			want:     []string{"--agent", "nav-pilot", "--model", "claude-opus-5"},
		},
		{
			name:     "explicit inherit emits no model",
			declared: agentpakke.InheritModel,
			want:     []string{"--agent", "nav-pilot"},
		},
		{
			name:      "the user's own model beats the declaration",
			declared:  "claude-opus-5",
			userModel: "gpt-5.5",
			want:      []string{"--agent", "nav-pilot", "--model", "gpt-5.5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { SetActivePakke(nil) })
			if tt.declared != "" {
				SetActivePakke(&agentpakke.Manifest{
					Name: "grillmester",
					Clients: map[string]agentpakke.ClientEntry{
						"copilot": {PrimaryAgents: []string{"nav-pilot"}, DefaultModel: tt.declared},
					},
				})
			}
			got := BuildCopilotArgs("copilot", domain.ResolvedConfig{AskUser: true, Model: tt.userModel})
			if !slices.Equal(got, tt.want) {
				t.Errorf("BuildCopilotArgs\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// TestResolvedModelNotice pins the launch line: which model, and where it came
// from. The distinction that matters is a Nav default the user never chose
// versus their own setting, since only one of those is theirs to change.
func TestResolvedModelNotice(t *testing.T) {
	navPakke := &agentpakke.Manifest{
		Name: "nav-pilot",
		Clients: map[string]agentpakke.ClientEntry{
			"copilot":  {PrimaryAgents: []string{"nav-pilot"}, DefaultModel: agentpakke.InheritModel},
			"opencode": {PrimaryAgents: []string{"nav-pilot"}, DefaultModel: OpenCodeDefaultModel},
		},
	}
	pinningPakke := &agentpakke.Manifest{
		Name: "grillmester",
		Clients: map[string]agentpakke.ClientEntry{
			"copilot":  {PrimaryAgents: []string{"grillmester"}, DefaultModel: "claude-opus-5"},
			"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: "github-copilot/claude-opus-5"},
		},
	}
	inheritPakke := &agentpakke.Manifest{
		Name: "grillmester",
		Clients: map[string]agentpakke.ClientEntry{
			"copilot":  {PrimaryAgents: []string{"grillmester"}, DefaultModel: agentpakke.InheritModel},
			"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: agentpakke.InheritModel},
		},
	}

	tests := []struct {
		name   string
		pakke  *agentpakke.Manifest
		client string
		model  string
		want   string
	}{
		{
			name:   "opencode on the Nav default",
			pakke:  navPakke,
			client: "opencode",
			want:   "Model: github-copilot/auto (nav-pilot default)",
		},
		{
			name:   "opencode with the user's own model",
			pakke:  navPakke,
			client: "opencode",
			model:  "claude-opus-5",
			want:   "Model: github-copilot/claude-opus-5 (your setting)",
		},
		{
			name:   "copilot with the user's own model",
			pakke:  navPakke,
			client: "copilot",
			model:  "claude-opus-5",
			want:   "Model: claude-opus-5 (your setting)",
		},
		{
			// Nothing pinned anywhere: the client picks, and there is nothing
			// meaningful to print.
			name:   "copilot on inherit says nothing",
			pakke:  navPakke,
			client: "copilot",
			want:   "",
		},
		{
			name:   "a pakke's own declaration is named after that pakke",
			pakke:  pinningPakke,
			client: "copilot",
			want:   "Model: claude-opus-5 (grillmester default)",
		},
		{
			// The active pakke declares nothing, so the model is the built-in
			// Nav one and the line must not credit grillmester for it.
			name:   "an inherit pakke falls back to the Nav default by name",
			pakke:  inheritPakke,
			client: "opencode",
			want:   "Model: github-copilot/auto (nav-pilot default)",
		},
		{
			name:   "pi names no model",
			pakke:  navPakke,
			client: "pi",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { SetActivePakke(nil) })
			SetActivePakke(tt.pakke)
			got := ResolvedModelNotice(tt.client, domain.ResolvedConfig{Client: tt.client, Model: tt.model})
			if got != tt.want {
				t.Errorf("ResolvedModelNotice(%q, model=%q) = %q, want %q", tt.client, tt.model, got, tt.want)
			}
		})
	}
}
