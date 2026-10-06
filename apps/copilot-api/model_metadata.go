package main

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
)

//go:generate node ../../scripts/generate-api-model-metadata.mjs

//go:embed model_metadata.json
var modelMetadataJSON []byte

type modelMetadata struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
	Category string `json:"category"`
}

const unclassified = "Unclassified"

var pricingSuffix = regexp.MustCompile(`(?i)\s*\((default(?:, [^()]*)?|long context, [^()]*|fast mode|preview)\)$`)

func modelKey(name string) string {
	name = strings.TrimSpace(name)
	for {
		stripped := pricingSuffix.ReplaceAllString(name, "")
		if stripped == name {
			break
		}
		name = stripped
	}
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

var metadataByModel = func() map[string]modelMetadata {
	var document struct {
		Models []modelMetadata `json:"models"`
	}
	if err := json.Unmarshal(modelMetadataJSON, &document); err != nil {
		panic(err)
	}
	result := map[string]modelMetadata{}
	for _, model := range document.Models {
		key := modelKey(model.Model)
		if previous, ok := result[key]; ok && (previous.Provider != model.Provider || previous.Category != model.Category) {
			panic("conflicting model metadata: " + key)
		}
		result[key] = model
	}
	// Historical pricing: https://github.com/navikt/copilot/blob/fe4a06af7b269b49d3a5fb6582ab679b113c36a5/apps/my-copilot/src/lib/model-pricing.ts
	for _, model := range []modelMetadata{
		{"Claude Opus 4.6", "Anthropic", "Powerful"},
		{"Claude Sonnet 4.5", "Anthropic", "Versatile"},
		{"Gemini 3.1 Pro", "Google", "Powerful"},
		{"Raptor mini", "GitHub", "Versatile"},
		{"GPT-5.2", "OpenAI", "Versatile"},
		{"GPT-5.2-Codex", "OpenAI", "Powerful"},
		{"Claude Opus 4.5", "Anthropic", "Powerful"},
	} {
		result[modelKey(model.Model)] = model
	}
	// Historical pricing: https://github.com/navikt/copilot/blob/9aa25ffa/apps/my-copilot/src/lib/model-pricing.ts
	result["gpt-4.1"] = modelMetadata{"GPT-4.1", "OpenAI", "Versatile"}
	// Historical pricing: https://github.com/navikt/copilot/blob/f72001e3/apps/my-copilot/src/lib/model-pricing.ts
	result["gemini-2.5-pro"] = modelMetadata{"Gemini 2.5 Pro", "Google", "Powerful"}
	result["gemini-3-flash"] = modelMetadata{"Gemini 3 Flash", "Google", "Lightweight"}
	// Historical pricing: https://github.com/navikt/copilot/blob/458f4af6/apps/my-copilot/src/lib/model-pricing.ts
	for _, model := range []modelMetadata{
		{"Claude Opus 4.7", "Anthropic", "Powerful"},
		{"Gemini 3.5 Flash (Default)", "Google", "Lightweight"},
		{"Gemini 3.6 Flash (Default)", "Google", "Versatile"},
		{"Kimi K2.7 Code", "Moonshot AI", "Versatile"},
	} {
		result[modelKey(model.Model)] = model
	}
	// Raw Claude aliases use version-first names; pricing uses family-first names.
	// Pricing names: https://github.com/navikt/copilot/blob/458f4af6185cce851de93a3b419eaa06b56e7237/apps/my-copilot/src/lib/model-pricing.ts
	result["claude-4.6-sonnet"] = result["claude-sonnet-4.6"]
	result["claude-4.5-haiku"] = result["claude-haiku-4.5"]
	// Provider only, not category: https://github.com/openai/openai-python/blob/e5de2e5656fb3d4fa70f050195382e6a4d59f806/src/openai/types/shared/chat_model.py
	// The raw gpt-3.5 alias refers to the GPT-3.5 family, listed as gpt-3.5-turbo.
	for _, name := range []string{"gpt-4o", "gpt-3.5"} {
		result[name] = modelMetadata{name, "OpenAI", unclassified}
	}
	return result
}()

func classifyModel(name string) modelMetadata {
	if model, ok := metadataByModel[modelKey(name)]; ok {
		return model
	}
	return modelMetadata{name, unclassified, unclassified}
}
