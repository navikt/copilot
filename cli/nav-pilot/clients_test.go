package main

import (
	"strings"
	"testing"
)

// ─── clientFor ────────────────────────────────────────────────────────────────

func TestClientFor_KnownIDs(t *testing.T) {
	for _, id := range []string{"copilot", "opencode", "pi"} {
		cl, err := clientFor(id)
		if err != nil {
			t.Errorf("clientFor(%q) error = %v, want nil", id, err)
			continue
		}
		if cl.ID() != id {
			t.Errorf("clientFor(%q).ID() = %q, want %q", id, cl.ID(), id)
		}
	}
}

func TestClientFor_Unknown(t *testing.T) {
	_, err := clientFor("cursor")
	if err == nil {
		t.Fatal("clientFor(unknown) = nil error, want error")
	}
	if !strings.Contains(err.Error(), "cursor") {
		t.Errorf("error = %q, want mention of \"cursor\"", err.Error())
	}
}

// ─── allClients / validClients ────────────────────────────────────────────────

func TestAllClients_Coverage(t *testing.T) {
	all := allClients()
	if len(all) == 0 {
		t.Fatal("allClients() returned empty list")
	}
	for _, id := range []string{"copilot", "opencode", "pi"} {
		found := false
		for _, cl := range all {
			if cl.ID() == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("allClients() missing client %q", id)
		}
	}
}

func TestValidClientsDerivesFromRegistry(t *testing.T) {
	// validClients must equal allClients() IDs in the same order.
	all := allClients()
	if len(validClients) != len(all) {
		t.Fatalf("len(validClients) = %d, len(allClients()) = %d", len(validClients), len(all))
	}
	for i, cl := range all {
		if validClients[i] != cl.ID() {
			t.Errorf("validClients[%d] = %q, allClients()[%d].ID() = %q", i, validClients[i], i, cl.ID())
		}
	}
}

// ─── copilotClient ────────────────────────────────────────────────────────────

func TestCopilotClient_Metadata(t *testing.T) {
	var cl Client = copilotClient{}
	if cl.ID() != "copilot" {
		t.Errorf("ID() = %q, want copilot", cl.ID())
	}
	if cl.DefaultModel() != "" {
		t.Errorf("DefaultModel() = %q, want empty", cl.DefaultModel())
	}
	models := cl.KnownModels()
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

func TestCopilotClient_ValidateModel(t *testing.T) {
	var cl Client = copilotClient{}
	if err := cl.ValidateModel("claude-sonnet-4.6"); err != nil {
		t.Errorf("ValidateModel(valid) = %v, want nil", err)
	}
	if err := cl.ValidateModel(""); err == nil {
		t.Error("ValidateModel(empty) = nil, want error")
	}
}

func TestCopilotClient_ModelAdvisory(t *testing.T) {
	var cl Client = copilotClient{}
	if msg := cl.ModelAdvisory("claude-sonnet-4.6"); msg != "" {
		t.Errorf("ModelAdvisory(known) = %q, want empty", msg)
	}
	if msg := cl.ModelAdvisory("sonnet"); msg == "" {
		t.Error("ModelAdvisory(unknown) = empty, want advisory")
	}
}

func TestCopilotClient_UnsupportedConfigWarnings(t *testing.T) {
	var cl Client = copilotClient{}
	r := ResolvedConfig{Mode: "autopilot", AskUser: false}
	if w := cl.UnsupportedConfigWarnings(r); len(w) != 0 {
		t.Errorf("copilotClient.UnsupportedConfigWarnings() = %v, want empty", w)
	}
}

// ─── openCodeClient ───────────────────────────────────────────────────────────

func TestOpenCodeClient_Metadata(t *testing.T) {
	var cl Client = openCodeClient{}
	if cl.ID() != "opencode" {
		t.Errorf("ID() = %q, want opencode", cl.ID())
	}
	if cl.DefaultModel() != openCodeDefaultModel {
		t.Errorf("DefaultModel() = %q, want %q", cl.DefaultModel(), openCodeDefaultModel)
	}
}

func TestOpenCodeClient_ValidateModel(t *testing.T) {
	var cl Client = openCodeClient{}
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
		err := cl.ValidateModel(tt.model)
		if tt.wantErr && err == nil {
			t.Errorf("ValidateModel(%q) = nil, want error", tt.model)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ValidateModel(%q) = %v, want nil", tt.model, err)
		}
	}
}

func TestOpenCodeClient_ModelAdvisory(t *testing.T) {
	var cl Client = openCodeClient{}
	if msg := cl.ModelAdvisory(openCodeDefaultModel); msg != "" {
		t.Errorf("ModelAdvisory(known) = %q, want empty", msg)
	}
	if msg := cl.ModelAdvisory("anthropic/claude-3-5-sonnet"); msg == "" {
		t.Error("ModelAdvisory(uncurated valid shape) = empty, want advisory")
	}
	// invalid shape → no advisory (the shape check itself provides the error)
	if msg := cl.ModelAdvisory("claude-opus-4.8"); msg != "" {
		t.Errorf("ModelAdvisory(invalid shape) = %q, want empty", msg)
	}
}

func TestOpenCodeClient_UnsupportedConfigWarnings(t *testing.T) {
	var cl Client = openCodeClient{}
	r := ResolvedConfig{Mode: "autopilot", ContextTier: "long_context", AskUser: false}
	w := cl.UnsupportedConfigWarnings(r)
	if len(w) != 3 {
		t.Errorf("UnsupportedConfigWarnings() len = %d, want 3: %v", len(w), w)
	}
}

// ─── piClient ─────────────────────────────────────────────────────────────────

func TestPiClient_Metadata(t *testing.T) {
	var cl Client = piClient{}
	if cl.ID() != "pi" {
		t.Errorf("ID() = %q, want pi", cl.ID())
	}
	if models := cl.KnownModels(); len(models) != 0 {
		t.Errorf("KnownModels() = %v, want empty", models)
	}
}

func TestPiClient_ModelAdvisory(t *testing.T) {
	var cl Client = piClient{}
	if msg := cl.ModelAdvisory("anything"); msg != "" {
		t.Errorf("ModelAdvisory() = %q, want empty", msg)
	}
}
