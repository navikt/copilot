package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestClassifyModel(t *testing.T) {
	for _, tc := range []struct {
		name, provider, category string
	}{
		{"claude-sonnet-5", "Anthropic", "Versatile"},
		{"gpt-5.6-luna", "OpenAI", "Lightweight"},
		{"claude-opus-5", "Anthropic", "Powerful"},
		{"gpt-5.6-sol", "OpenAI", "Powerful"},
		{"claude-opus-5.5", "Anthropic", "Powerful"},
		{"claude-opus-4.8", "Anthropic", "Powerful"},
		{"gpt-5.6-terra", "OpenAI", "Versatile"},
		{"gpt-5.3-codex", "OpenAI", "Powerful"},
		{"gpt-6-luna", "OpenAI", "Lightweight"},
		{"gpt-6-astra", "OpenAI", "Powerful"},
		{"gpt-6-sol", "OpenAI", "Powerful"},
		{"kimi-k3", "Moonshot AI", "Powerful"},
		{"gpt-5.4-mini", "OpenAI", "Lightweight"},
		{"gemini-3.8-flash", "Google", "Versatile"},
		{"claude-sonnet-5.5", "Anthropic", "Versatile"},
		{"gemini-3.7-flash", "Google", "Versatile"},
		{"kimi-k2.7-code", "Moonshot AI", "Versatile"},
		{"gpt-5.5", "OpenAI", "Powerful"},
		{"gpt-5.4", "OpenAI", "Versatile"},
		{"gpt-5-mini", "OpenAI", "Lightweight"},
		{"claude-4.6-sonnet", "Anthropic", "Versatile"},
		{"claude-4.5-haiku", "Anthropic", "Versatile"},
		{"claude-opus-4.6", "Anthropic", "Powerful"},
		{"claude-sonnet-4.5", "Anthropic", "Versatile"},
		{"gemini-3.1-pro", "Google", "Powerful"},
		{"raptor-mini", "GitHub", "Versatile"},
		{"gpt-4o", "OpenAI", unclassified},
		{"gpt-3.5", "OpenAI", unclassified},
		{"gpt-4.1", "OpenAI", "Versatile"},
		{"gpt-5.2", "OpenAI", "Versatile"},
		{"gpt-5.2-codex", "OpenAI", "Powerful"},
		{"claude-opus-4.5", "Anthropic", "Powerful"},
		{"gemini-2.5-pro", "Google", "Powerful"},
		{"gemini-3-flash", "Google", "Lightweight"},
		{"GPT-5.6 Sol (Long context, 272K)", "OpenAI", "Powerful"},
		{"Claude Opus 4.8 (fast mode) (preview)", "Anthropic", "Powerful"},
		{"mai-code-1.1-flash", "Microsoft", "Lightweight"},
		{"unknown", unclassified, unclassified},
		{"others", unclassified, unclassified},
		{"auto", unclassified, unclassified},
		{"", unclassified, unclassified},
		{"gpt-5.6-sol (unrecognized)", unclassified, unclassified},
		{"gpt-56-sol", unclassified, unclassified},
		{"future-model", unclassified, unclassified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyModel(tc.name)
			if got.Provider != tc.provider || got.Category != tc.category {
				t.Fatalf("classification = %+v, want %s/%s", got, tc.provider, tc.category)
			}
		})
	}
}

func TestTeamCompositionAggregatesBeforeRanking(t *testing.T) {
	rows := []teamCompositionRow{
		{"visible", "model", "claude-opus-5", 100},
		{"visible", "model", "gemini-3.8-flash", 90},
		{"visible", "model", "kimi-k3", 80},
		{"visible", "model", "gpt-6-sol", 70},
		{"visible", "model", "gpt-5.6-sol", 60},
		{"visible", "model", "gpt-5-mini", 1},
		{"visible", "model", "unknown", 1},
		{"visible", "model", "others", 1},
		{"visible", "model", "gpt-4o", 1},
		{"visible", "feature", "chat", 0},
		{"visible", "language", "go", 0},
		{"hidden", "model", "claude-opus-5", 1000},
	}
	composition := aggregateTeamComposition(rows)
	want := TeamUsageComposition{
		Providers:  []string{"OpenAI", "Anthropic", "Google", "Moonshot AI", unclassified},
		Categories: []string{"Powerful", "Versatile", unclassified, "Lightweight"},
		Feature:    "chat", Language: "go",
	}
	if !reflect.DeepEqual(composition["visible"], want) {
		t.Fatalf("composition = %+v, want %+v", composition["visible"], want)
	}
	// Gross visibility is the only five-user threshold for provider/category summaries.
	got := teamGrossOverview("2026-09", []teamGrossRow{
		{TeamID: "visible", Users: 5},
		{TeamID: "hidden", Users: 4},
		{TeamID: "zero", Users: 5},
	}, composition, nil)
	if len(got.Teams) != 2 || len(got.Usage) != 2 || !reflect.DeepEqual(got.Usage["visible"], want) {
		t.Fatalf("unexpected visible teams/composition: %+v", got)
	}
	if !reflect.DeepEqual(got.Usage["zero"], TeamUsageComposition{Providers: []string{}, Categories: []string{}}) {
		t.Fatalf("zero interaction fallback = %+v", got.Usage["zero"])
	}
}

func TestCompositionRankingTiesAndUnknown(t *testing.T) {
	got := aggregateTeamComposition([]teamCompositionRow{
		{"a", "model", "gpt-5-mini", 1},
		{"a", "model", "claude-sonnet-5", 1},
		{"a", "model", "auto", 1},
		{"four", "model", "gpt-6-sol", 4},
		{"four", "model", "claude-sonnet-5", 3},
		{"four", "model", "gpt-5-mini", 2},
		{"four", "model", "unknown", 1},
		{"b", "model", "gpt-6-sol", 0},
		{"b", "feature", "chat", 1},
	})
	if !reflect.DeepEqual(got["a"].Providers, []string{"Anthropic", "OpenAI", unclassified}) ||
		!reflect.DeepEqual(got["a"].Categories, []string{"Lightweight", unclassified, "Versatile"}) {
		t.Fatalf("ties or unknown lost: %+v", got["a"])
	}
	if len(got["b"].Providers) != 0 || len(got["b"].Categories) != 0 {
		t.Fatalf("zero activity counted: %+v", got["b"])
	}
	if !reflect.DeepEqual(got["four"].Categories, []string{"Powerful", "Versatile", "Lightweight", unclassified}) {
		t.Fatalf("fourth Unclassified category lost: %+v", got["four"])
	}
}

// Every model the generated metadata names, current or retired, must classify
// as itself. Derived from the file, so a pricing sync that drops a model needs
// no test edit, and a generator that forgets one fails here.
func TestClassifyModelCoversGeneratedMetadata(t *testing.T) {
	var document struct {
		Models  []modelMetadata `json:"models"`
		Retired []modelMetadata `json:"retired"`
	}
	if err := json.Unmarshal(modelMetadataJSON, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Models) == 0 {
		t.Fatal("model_metadata.json lists no models")
	}
	current := map[string]bool{}
	for _, model := range document.Models {
		current[modelKey(model.Model)] = true
	}
	expected := document.Models
	for _, model := range document.Retired {
		if !current[modelKey(model.Model)] { // otherwise the current row wins
			expected = append(expected, model)
		}
	}
	for _, want := range expected {
		if got := classifyModel(want.Model); got.Provider != want.Provider || got.Category != want.Category {
			t.Errorf("%q classified as %s/%s, want %s/%s", want.Model, got.Provider, got.Category, want.Provider, want.Category)
		}
	}
}

// Fixed names from usage history. The test above derives from the file, so a
// generator that loses `retired` wholesale would pass it; this would not.
func TestClassifyModelKeepsHistoricalModels(t *testing.T) {
	for name, want := range map[string]modelMetadata{
		"GPT-4.1":         {"GPT-4.1", "OpenAI", "Versatile"},
		"Claude Opus 4.5": {"Claude Opus 4.5", "Anthropic", "Powerful"},
		"Kimi K2.7 Code":  {"Kimi K2.7 Code", "Moonshot AI", "Versatile"},
	} {
		if got := classifyModel(name); got.Provider != want.Provider || got.Category != want.Category {
			t.Errorf("%q classified as %s/%s, want %s/%s", name, got.Provider, got.Category, want.Provider, want.Category)
		}
	}
}
