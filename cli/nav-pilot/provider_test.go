package main

import (
	"strings"
	"testing"
)

// ─── providerFor ──────────────────────────────────────────────────────────────

func TestProviderFor_KnownIDs(t *testing.T) {
	for _, id := range []string{"copilot", "opencode", "pi"} {
		p, err := providerFor(id)
		if err != nil {
			t.Errorf("providerFor(%q) error = %v, want nil", id, err)
			continue
		}
		if p.ID() != id {
			t.Errorf("providerFor(%q).ID() = %q, want %q", id, p.ID(), id)
		}
	}
}

func TestProviderFor_Unknown(t *testing.T) {
	_, err := providerFor("cursor")
	if err == nil {
		t.Fatal("providerFor(unknown) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "cursor") {
		t.Errorf("error = %q, want mention of \"cursor\"", err.Error())
	}
}

// ─── allProviders / validProviderIDs ─────────────────────────────────────────

func TestAllProviders_Coverage(t *testing.T) {
	all := allProviders()
	if len(all) == 0 {
		t.Fatal("allProviders() returned empty list")
	}
	for _, id := range []string{"copilot", "opencode", "pi"} {
		found := false
		for _, p := range all {
			if p.ID() == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("allProviders() missing provider %q", id)
		}
	}
}

func TestValidProviderIDsDerivesFromRegistry(t *testing.T) {
	// validProviderIDs must equal allProviders() IDs in the same order.
	all := allProviders()
	if len(validProviderIDs) != len(all) {
		t.Fatalf("len(validProviderIDs) = %d, len(allProviders()) = %d", len(validProviderIDs), len(all))
	}
	for i, p := range all {
		if validProviderIDs[i] != p.ID() {
			t.Errorf("validProviderIDs[%d] = %q, allProviders()[%d].ID() = %q", i, validProviderIDs[i], i, p.ID())
		}
	}
}

// ─── copilotProvider ──────────────────────────────────────────────────────────

func TestCopilotProvider_Metadata(t *testing.T) {
	var p Provider = copilotProvider{}
	if p.ID() != "copilot" {
		t.Errorf("ID() = %q, want copilot", p.ID())
	}
	if p.DefaultModel() != "" {
		t.Errorf("DefaultModel() = %q, want empty", p.DefaultModel())
	}
	models := p.KnownModels()
	if len(models) == 0 {
		t.Fatal("KnownModels() is empty")
	}
	hasAuto := false
	for _, m := range models {
		if m.ID == "auto" {
			hasAuto = true
		}
	}
	if !hasAuto {
		t.Error("KnownModels() missing \"auto\"")
	}
}

func TestCopilotProvider_ValidateModel(t *testing.T) {
	var p Provider = copilotProvider{}
	if err := p.ValidateModel("claude-sonnet-4.6"); err != nil {
		t.Errorf("ValidateModel(valid) = %v, want nil", err)
	}
	if err := p.ValidateModel(""); err == nil {
		t.Error("ValidateModel(empty) = nil, want error")
	}
}

func TestCopilotProvider_ModelAdvisory(t *testing.T) {
	var p Provider = copilotProvider{}
	if msg := p.ModelAdvisory("claude-sonnet-4.6"); msg != "" {
		t.Errorf("ModelAdvisory(known) = %q, want empty", msg)
	}
	if msg := p.ModelAdvisory("sonnet"); msg == "" {
		t.Error("ModelAdvisory(unknown) = empty, want advisory")
	}
}

func TestCopilotProvider_UnsupportedConfigWarnings(t *testing.T) {
	var p Provider = copilotProvider{}
	r := ResolvedConfig{Mode: "autopilot", AskUser: false}
	if w := p.UnsupportedConfigWarnings(r); len(w) != 0 {
		t.Errorf("copilotProvider.UnsupportedConfigWarnings() = %v, want empty", w)
	}
}

func TestCopilotProvider_ContextLifecycle(t *testing.T) {
	var p Provider = copilotProvider{}
	summary, err := p.Bootstrap()
	if err != nil || summary != "" {
		t.Errorf("Bootstrap() = (%q, %v), want (\"\", nil)", summary, err)
	}
	res := p.SyncContext("", "", false, false)
	if res.Managed {
		t.Error("SyncContext().Managed = true, want false")
	}
	if cs := p.ContextStatus(); cs != nil {
		t.Errorf("ContextStatus() = %v, want nil", cs)
	}
}

// ─── openCodeProvider ─────────────────────────────────────────────────────────

func TestOpenCodeProvider_Metadata(t *testing.T) {
	var p Provider = openCodeProvider{}
	if p.ID() != "opencode" {
		t.Errorf("ID() = %q, want opencode", p.ID())
	}
	if p.DefaultModel() != openCodeDefaultModel {
		t.Errorf("DefaultModel() = %q, want %q", p.DefaultModel(), openCodeDefaultModel)
	}
}

func TestOpenCodeProvider_ValidateModel(t *testing.T) {
	var p Provider = openCodeProvider{}
	tests := []struct {
		model   string
		wantErr bool
	}{
		{"anthropic/claude-sonnet-4-5", false},
		{"openai/gpt-4o", false},
		{"claude-opus-4.8", true}, // bare id invalid for opencode
		{"gpt-5.5", true},         // bare id invalid for opencode
		{"anthropic/", true},      // trailing slash
		{"a/b/c", true},           // double slash
		{"", true},
	}
	for _, tt := range tests {
		err := p.ValidateModel(tt.model)
		if tt.wantErr && err == nil {
			t.Errorf("ValidateModel(%q) = nil, want error", tt.model)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ValidateModel(%q) = %v, want nil", tt.model, err)
		}
	}
}

func TestOpenCodeProvider_ModelAdvisory(t *testing.T) {
	var p Provider = openCodeProvider{}
	if msg := p.ModelAdvisory(openCodeDefaultModel); msg != "" {
		t.Errorf("ModelAdvisory(known) = %q, want empty", msg)
	}
	if msg := p.ModelAdvisory("anthropic/claude-3-5-sonnet"); msg == "" {
		t.Error("ModelAdvisory(uncurated valid shape) = empty, want advisory")
	}
	// invalid shape → no advisory (the shape check itself provides the error)
	if msg := p.ModelAdvisory("claude-opus-4.8"); msg != "" {
		t.Errorf("ModelAdvisory(invalid shape) = %q, want empty", msg)
	}
}

func TestOpenCodeProvider_UnsupportedConfigWarnings(t *testing.T) {
	var p Provider = openCodeProvider{}
	r := ResolvedConfig{Mode: "autopilot", ContextTier: "long_context", AskUser: false}
	w := p.UnsupportedConfigWarnings(r)
	if len(w) != 3 {
		t.Errorf("UnsupportedConfigWarnings() len = %d, want 3: %v", len(w), w)
	}
}

func TestOpenCodeProvider_ContextStatusNoState(t *testing.T) {
	// With no state on disk, ContextStatus must return nil.
	old := openCodeNavContextDirOverride
	openCodeNavContextDirOverride = t.TempDir()
	defer func() { openCodeNavContextDirOverride = old }()

	var p Provider = openCodeProvider{}
	if cs := p.ContextStatus(); cs != nil {
		t.Errorf("ContextStatus() = %v, want nil (no state file)", cs)
	}
	res := p.SyncContext("", "", false, false)
	if res.Managed {
		t.Error("SyncContext().Managed = true, want false (no state file)")
	}
}

// ─── piProvider ───────────────────────────────────────────────────────────────

func TestPiProvider_Metadata(t *testing.T) {
	var p Provider = piProvider{}
	if p.ID() != "pi" {
		t.Errorf("ID() = %q, want pi", p.ID())
	}
	if models := p.KnownModels(); len(models) != 0 {
		t.Errorf("KnownModels() = %v, want empty", models)
	}
}

func TestPiProvider_ModelAdvisory(t *testing.T) {
	var p Provider = piProvider{}
	if msg := p.ModelAdvisory("anything"); msg != "" {
		t.Errorf("ModelAdvisory() = %q, want empty", msg)
	}
}

func TestPiProvider_ContextLifecycle(t *testing.T) {
	var p Provider = piProvider{}
	summary, err := p.Bootstrap()
	if err != nil || summary != "" {
		t.Errorf("Bootstrap() = (%q, %v), want (\"\", nil)", summary, err)
	}
	res := p.SyncContext("", "", false, false)
	if res.Managed {
		t.Error("SyncContext().Managed = true, want false")
	}
	if cs := p.ContextStatus(); cs != nil {
		t.Errorf("ContextStatus() = %v, want nil", cs)
	}
}
