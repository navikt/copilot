package provider

import (
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
