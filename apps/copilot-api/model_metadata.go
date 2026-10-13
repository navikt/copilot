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
		Models  []modelMetadata `json:"models"`
		Retired []modelMetadata `json:"retired"`
	}
	if err := json.Unmarshal(modelMetadataJSON, &document); err != nil {
		panic(err)
	}
	result := map[string]modelMetadata{}
	// Models the pricing page has dropped; the generator keeps them so past
	// usage stays classified. Current pricing below overrides on a key clash.
	for _, model := range document.Retired {
		key := modelKey(model.Model)
		if previous, ok := result[key]; ok && (previous.Provider != model.Provider || previous.Category != model.Category) {
			panic("conflicting retired model metadata: " + key)
		}
		result[key] = model
	}
	current := map[string]modelMetadata{}
	for _, model := range document.Models {
		key := modelKey(model.Model)
		if previous, ok := current[key]; ok && (previous.Provider != model.Provider || previous.Category != model.Category) {
			panic("conflicting model metadata: " + key)
		}
		current[key] = model
		result[key] = model
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
