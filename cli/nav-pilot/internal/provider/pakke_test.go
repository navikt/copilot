package provider

import (
	"slices"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"strings"
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
	if got := ToOpenCodeModel(""); got != "" {
		t.Errorf("after SetActivePakke(nil): ToOpenCodeModel(\"\") = %q, want \"\" (opencode picks its own default)", got)
	}
}

// TestActivePakkeLegacyAutoIsNormalized covers a pakke that still declares the
// pre-fix opencode default: it must resolve to "" (nothing pinned, opencode
// picks) rather than forwarding the broken alias.
func TestActivePakkeLegacyAutoIsNormalized(t *testing.T) {
	t.Cleanup(func() { SetActivePakke(nil) })
	SetActivePakke(&agentpakke.Manifest{
		Name: "legacy",
		Clients: map[string]agentpakke.ClientEntry{
			"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: "github-copilot/auto"},
		},
	})
	if got := ToOpenCodeModel(""); got != "" {
		t.Errorf("ToOpenCodeModel(\"\") = %q, want \"\"", got)
	}
}

// TestActivePakkeBareDeclarationGetsPrefixed covers a Tier 1 pakke that
// declares a bare Copilot id for opencode: it must gain the provider prefix,
// not reach opencode unqualified.
func TestActivePakkeBareDeclarationGetsPrefixed(t *testing.T) {
	t.Cleanup(func() { SetActivePakke(nil) })
	SetActivePakke(&agentpakke.Manifest{
		Name: "legacy",
		Clients: map[string]agentpakke.ClientEntry{
			"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: "gpt-5.6-terra"},
		},
	})
	if got := ToOpenCodeModel(""); got != "github-copilot/gpt-5.6-terra" {
		t.Errorf("ToOpenCodeModel(\"\") = %q, want github-copilot/gpt-5.6-terra", got)
	}
}

// TestActivePakkeBareAutoIsNormalized covers a pakke declaring bare "auto"
// (valid for Copilot CLI, invalid for opencode): it must not recurse and must
// resolve to "".
func TestActivePakkeBareAutoIsNormalized(t *testing.T) {
	t.Cleanup(func() { SetActivePakke(nil) })
	SetActivePakke(&agentpakke.Manifest{
		Name: "legacy",
		Clients: map[string]agentpakke.ClientEntry{
			"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: "auto"},
		},
	})
	if got := ToOpenCodeModel(""); got != "" {
		t.Errorf("ToOpenCodeModel(\"\") = %q, want \"\"", got)
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

// TestResolvedModelNotice pins the launch line: which model the launch sets for
// the session, and where it came from. The distinction that matters is a Nav
// default the user never chose versus their own setting, since only one of
// those is theirs to change.
func TestResolvedModelNotice(t *testing.T) {
	navPakke := &agentpakke.Manifest{
		Name: "nav-pilot",
		Clients: map[string]agentpakke.ClientEntry{
			"copilot":  {PrimaryAgents: []string{"nav-pilot"}, DefaultModel: agentpakke.InheritModel},
			"opencode": {PrimaryAgents: []string{"nav-pilot"}, DefaultModel: agentpakke.InheritModel},
		},
	}
	pinningPakke := &agentpakke.Manifest{
		Name: "grillmester",
		Clients: map[string]agentpakke.ClientEntry{
			"copilot":  {PrimaryAgents: []string{"grillmester"}, DefaultModel: "claude-opus-5"},
			"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: "github-copilot/claude-opus-5"},
			"pi":       {PrimaryAgents: []string{"grillmester"}, DefaultModel: "claude-opus-5"},
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
			// Nothing pinned anywhere: opencode picks for itself, and there is
			// nothing meaningful to print.
			name:   "opencode on inherit says nothing",
			pakke:  navPakke,
			client: "opencode",
			want:   "",
		},
		{
			name:   "opencode with the user's own model",
			pakke:  navPakke,
			client: "opencode",
			model:  "claude-opus-5",
			want:   "Session model: github-copilot/claude-opus-5 (your setting)",
		},
		{
			name:   "copilot with the user's own model",
			pakke:  navPakke,
			client: "copilot",
			model:  "claude-opus-5",
			want:   "Session model: claude-opus-5 (your setting)",
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
			want:   "Session model: claude-opus-5 (grillmester default)",
		},
		{
			// "inherit" means the staged launch passes no --model at all, and
			// OPENCODE_CONFIG_DIR points at the payload, whose own config picks.
			// Naming the built-in Nav model here would announce a model the
			// launch never asks for.
			name:   "an inherit pakke names no model",
			pakke:  inheritPakke,
			client: "opencode",
			want:   "",
		},
		{
			// pi takes --model and LaunchPi forwards it, so the notice names the
			// model the session will actually run on, as it does for the others.
			name:   "pi names the model the user set",
			pakke:  navPakke,
			client: "pi",
			model:  "claude-opus-5",
			want:   "Session model: claude-opus-5 (your setting)",
		},
		{
			name:   "pi names the model the pakke declares",
			pakke:  pinningPakke,
			client: "pi",
			want:   "Session model: claude-opus-5 (grillmester default)",
		},
		{
			// A pakke still declaring the legacy alias must be reported as
			// naming nothing, same as inherit, not as the broken id it wrote
			// down.
			name: "a pakke's legacy auto declaration says nothing",
			pakke: &agentpakke.Manifest{
				Name: "grillmester",
				Clients: map[string]agentpakke.ClientEntry{
					"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: "github-copilot/auto"},
				},
			},
			client: "opencode",
			want:   "",
		},
		{
			// A bare id also gets the provider prefix, same as a user setting.
			name: "a pakke's bare declaration gains the provider prefix",
			pakke: &agentpakke.Manifest{
				Name: "grillmester",
				Clients: map[string]agentpakke.ClientEntry{
					"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: "claude-opus-5"},
				},
			},
			client: "opencode",
			want:   "Session model: github-copilot/claude-opus-5 (grillmester default)",
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

// TestPiNoticeNamesTheForwardedModel is the inverse of what this test pinned
// before. pi takes --model, LaunchPi forwards it, so the notice naming the model
// is accurate and the warning list must not claim it was dropped. The two lines
// print on the same launch and used to contradict each other.
func TestPiNoticeNamesTheForwardedModel(t *testing.T) {
	t.Cleanup(func() { SetActivePakke(nil) })

	r := domain.ResolvedConfig{Client: "pi", Model: "claude-opus-5", AskUser: true}
	for _, w := range PiUnsupportedConfigWarnings(r) {
		if strings.Contains(w, "model") {
			t.Errorf("pi must not warn about a model it forwards, got %q", w)
		}
	}
	if got := ResolvedModelNotice("pi", r); got == "" {
		t.Error("ResolvedModelNotice(pi) = \"\", want the model the launch forwards")
	}
}

// TestInheritPakkeNoticeMatchesTheStagedLaunch pins the other half: an inherit
// declaration makes the staged opencode launch pass no --model, so the notice
// has no model to name either.
func TestInheritPakkeNoticeMatchesTheStagedLaunch(t *testing.T) {
	t.Cleanup(func() { SetActivePakke(nil) })

	SetActivePakke(&agentpakke.Manifest{
		Name: "grillmester",
		Clients: map[string]agentpakke.ClientEntry{
			"opencode": {
				DefaultModel: agentpakke.InheritModel,
				Payloads: map[string]agentpakke.Payload{
					"full": {Path: "dist/opencode/full", PrimaryAgents: []string{"grillmester"}},
				},
			},
		},
	})

	spec, err := buildStagedOpenCodeSpec(
		domain.ResolvedConfig{Client: "opencode"},
		StagedLaunch{Dir: t.TempDir(), PakkeName: "grillmester", Context: "full"},
	)
	if err != nil {
		t.Fatalf("buildStagedOpenCodeSpec: %v", err)
	}
	if slices.Contains(spec.agentArgs, "--model") {
		t.Fatalf("the staged launch passed a model: %v", spec.agentArgs)
	}
	if got := ResolvedModelNotice("opencode", domain.ResolvedConfig{Client: "opencode"}); got != "" {
		t.Errorf("ResolvedModelNotice = %q, want \"\" when the launch passes no --model", got)
	}
}

// --persona must accept any primary the pakke declares and refuse anything
// else, naming what is on offer. The name is passed to the client verbatim, so
// an unchecked value fails inside the client instead of here (#798).
func TestResolvePersona(t *testing.T) {
	SetActivePakke(&agentpakke.Manifest{
		Name: "p",
		Clients: map[string]agentpakke.ClientEntry{
			"copilot": {PrimaryAgents: []string{"implementer", "reviewer"}},
		},
	})
	t.Cleanup(func() { SetActivePakke(nil) })

	got, err := ResolvePersona("copilot", "")
	if err != nil || got != "implementer" {
		t.Fatalf("empty override must give the first primary, got %q %v", got, err)
	}
	got, err = ResolvePersona("copilot", "reviewer")
	if err != nil || got != "reviewer" {
		t.Fatalf("a declared primary must be accepted, got %q %v", got, err)
	}
	_, err = ResolvePersona("copilot", "nope")
	if err == nil {
		t.Fatal("an undeclared persona must be refused")
	}
	if !strings.Contains(err.Error(), "implementer, reviewer") {
		t.Fatalf("the refusal must name what is on offer, got %q", err)
	}
}
