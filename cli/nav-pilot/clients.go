package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// Client is the strategy abstraction for a coding-agent backend.
// Each implementation owns its own metadata (display name, model catalog),
// launch logic, availability probe, and config-warning logic.
// Adding a new client requires one struct + one registry entry; no other
// files need to branch on client id.
type Client interface {
	// ID returns the canonical string identifier used in config ("copilot", etc.).
	ID() string
	// DisplayName returns the user-facing name used in prompts and launch messages.
	DisplayName() string
	// Available returns true when the client binary is found in PATH.
	Available() bool
	// Launch launches the client with the resolved config.
	Launch(resolved ResolvedConfig) error
	// DefaultModel returns the model id that should be used when none is set.
	// Returns "" when the client handles its own default (e.g. Copilot).
	DefaultModel() string
	// KnownModels returns the Nav-curated model list for the setup picker.
	// Returns nil for clients without a curated list.
	KnownModels() []modelChoice
	// ValidateModel validates a model identifier for this client.
	// Applies client-specific shape rules on top of the base validateModelValue check.
	ValidateModel(model string) error
	// ModelAdvisory returns a soft-warning string when the model is valid but not
	// in the curated list, or "" when no advisory is needed.
	ModelAdvisory(model string) string
	// UnsupportedConfigWarnings returns informational warning strings for config
	// fields that are explicitly set but have no equivalent for this client.
	UnsupportedConfigWarnings(resolved ResolvedConfig) []string
}

// ──────────────────────────────────────────────────────────────────────────────
// Model metadata — single source of truth per client
// ──────────────────────────────────────────────────────────────────────────────

// openCodeDefaultModel is the Nav-curated default model for opencode, applied
// when the user launches opencode without an explicit model set.
// Source: https://opencode.ai/docs/models — current Claude Sonnet 4.5 via
// the Anthropic provider (anthropic/claude-sonnet-4-5).
const openCodeDefaultModel = "anthropic/claude-sonnet-4-5"

// knownCopilotModels lists the models the Copilot CLI commonly offers.
// Used by the first-run wizard picker and by advisories.
// The Copilot CLI validates --model server-side, so this is a convenience
// list — unrecognized but well-formed ids still work.
var knownCopilotModels = []modelChoice{
	{"auto", "Auto (let Copilot pick)"},
	{"claude-sonnet-4.6", "Claude Sonnet 4.6 (default)"},
	{"claude-haiku-4.5", "Claude Haiku 4.5"},
	{"claude-opus-4.8", "Claude Opus 4.8"},
	{"claude-opus-4.6", "Claude Opus 4.6"},
	{"gpt-5.5", "GPT-5.5"},
	{"gpt-5.4", "GPT-5.4"},
	{"gpt-5.3-codex", "GPT-5.3-Codex"},
	{"gpt-5.4-mini", "GPT-5.4 mini"},
	{"gpt-5-mini", "GPT-5 mini"},
	{"gemini-3.1-pro-preview", "Gemini 3.1 Pro (Preview)"},
	{"gemini-3.5-flash", "Gemini 3.5 Flash"},
}

// knownOpenCodeModels lists Nav-blessed opencode provider/model ids.
// Sources: https://opencode.ai/docs/models, https://deepwiki.com/sst/opencode/4.4-supported-providers
var knownOpenCodeModels = []modelChoice{
	{openCodeDefaultModel, "Claude Sonnet 4.5 (Nav default)"},
	{"anthropic/claude-opus-4-5", "Claude Opus 4.5"},
	{"anthropic/claude-haiku-4-5", "Claude Haiku 4.5"},
	{"openai/gpt-4o", "GPT-4o"},
	{"google/gemini-2-0-flash", "Gemini 2.0 Flash"},
}

// ──────────────────────────────────────────────────────────────────────────────
// Membership helpers (used by configModelLabel and advisory logic)
// ──────────────────────────────────────────────────────────────────────────────

func isKnownCopilotModel(id string) bool {
	for _, m := range knownCopilotModels {
		if strings.EqualFold(m.ID, id) {
			return true
		}
	}
	return false
}

func knownCopilotModelIDs() string {
	ids := make([]string, len(knownCopilotModels))
	for i, m := range knownCopilotModels {
		ids[i] = m.ID
	}
	return strings.Join(ids, ", ")
}

func isKnownOpenCodeModel(id string) bool {
	for _, m := range knownOpenCodeModels {
		if strings.EqualFold(m.ID, id) {
			return true
		}
	}
	return false
}

func knownOpenCodeModelIDs() string {
	ids := make([]string, len(knownOpenCodeModels))
	for i, m := range knownOpenCodeModels {
		ids[i] = m.ID
	}
	return strings.Join(ids, ", ")
}

// ──────────────────────────────────────────────────────────────────────────────
// copilotClient
// ──────────────────────────────────────────────────────────────────────────────

type copilotClient struct{}

func (copilotClient) ID() string { return "copilot" }

func (copilotClient) DisplayName() string {
	_, name := findCopilotCLI()
	return cliDisplayName(name)
}

func (copilotClient) Available() bool {
	path, _ := findCopilotCLI()
	return path != ""
}

func (copilotClient) Launch(r ResolvedConfig) error { return launchCopilotResolved(r) }

func (copilotClient) DefaultModel() string { return "" } // agent picks its own default

func (copilotClient) KnownModels() []modelChoice { return knownCopilotModels }

func (copilotClient) ValidateModel(model string) error { return validateModelValue(model) }

func (copilotClient) ModelAdvisory(model string) string {
	if validateModelValue(model) != nil || isKnownCopilotModel(model) {
		return ""
	}
	return fmt.Sprintf(
		"model %q is not a recognized Copilot model id; it will be sent as-is and may be rejected by the server (known ids: %s)",
		model, knownCopilotModelIDs())
}

func (copilotClient) UnsupportedConfigWarnings(_ ResolvedConfig) []string { return nil }

// ──────────────────────────────────────────────────────────────────────────────
// openCodeClient
// ──────────────────────────────────────────────────────────────────────────────

type openCodeClient struct{}

func (openCodeClient) ID() string          { return "opencode" }
func (openCodeClient) DisplayName() string { return "opencode" }

func (openCodeClient) Available() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}

func (openCodeClient) Launch(r ResolvedConfig) error { return launchOpenCode(r) }

func (openCodeClient) DefaultModel() string { return openCodeDefaultModel }

func (openCodeClient) KnownModels() []modelChoice { return knownOpenCodeModels }

func (openCodeClient) ValidateModel(model string) error {
	if err := validateModelValue(model); err != nil {
		return err
	}
	if strings.Count(model, "/") != 1 || strings.HasSuffix(model, "/") {
		return fmt.Errorf("model %q must be in provider/model format for opencode (e.g. %q)", model, openCodeDefaultModel)
	}
	return nil
}

func (c openCodeClient) ModelAdvisory(model string) string {
	if c.ValidateModel(model) != nil || isKnownOpenCodeModel(model) {
		return ""
	}
	return fmt.Sprintf(
		"model %q is not a Nav-curated opencode model id; it will be passed as-is (Nav default: %s, known ids: %s)",
		model, openCodeDefaultModel, knownOpenCodeModelIDs())
}

func (openCodeClient) UnsupportedConfigWarnings(r ResolvedConfig) []string {
	return openCodeUnsupportedConfigWarnings(r)
}

// ──────────────────────────────────────────────────────────────────────────────
// piClient
// ──────────────────────────────────────────────────────────────────────────────

type piClient struct{}

func (piClient) ID() string          { return "pi" }
func (piClient) DisplayName() string { return "pi" }

func (piClient) Available() bool {
	_, err := exec.LookPath("pi")
	return err == nil
}

func (piClient) Launch(_ ResolvedConfig) error { return launchPi() }

func (piClient) DefaultModel() string                                { return "" }
func (piClient) KnownModels() []modelChoice                          { return nil }
func (piClient) ValidateModel(model string) error                    { return validateModelValue(model) }
func (piClient) ModelAdvisory(_ string) string                       { return "" }
func (piClient) UnsupportedConfigWarnings(_ ResolvedConfig) []string { return nil }

// ──────────────────────────────────────────────────────────────────────────────
// Registry
// ──────────────────────────────────────────────────────────────────────────────

// clientRegistry is the ordered list of all supported clients.
// Adding a new client means adding one entry here — no other files need changing.
var clientRegistry = []Client{
	copilotClient{},
	openCodeClient{},
	piClient{},
}

// clientFor returns the Client implementation for the given id, or an error
// if the id is unknown.
func clientFor(id string) (Client, error) {
	for _, c := range clientRegistry {
		if c.ID() == id {
			return c, nil
		}
	}
	return nil, fmt.Errorf("unknown client %q", id)
}

// allClients returns all registered clients in registry order.
func allClients() []Client {
	return clientRegistry
}

// validClients is derived from the registry so there is a single source of truth
// for which client ids are accepted. It is used for config validation and help text.
var validClients = func() []string {
	ids := make([]string, len(clientRegistry))
	for i, c := range clientRegistry {
		ids[i] = c.ID()
	}
	return ids
}()
