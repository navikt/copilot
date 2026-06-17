package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ProviderSyncResult captures the outcome of a provider-driven context sync.
// Managed is true when the provider has existing managed context and ran the sync.
type ProviderSyncResult struct {
	Managed bool
}

// ProviderContextStatus holds the data needed to display provider context status.
// Returned by Provider.ContextStatus; nil means the provider has no managed context.
type ProviderContextStatus struct {
	State     *StateFile
	OutputDir string
	ScopeName string
}

// Provider is the strategy abstraction for a coding-agent backend.
// Each implementation owns its own metadata (display name, model catalog),
// launch logic, availability probe, config-warning logic, and context lifecycle.
// Adding a new provider requires one struct + one registry entry; no other
// files need to branch on provider id.
type Provider interface {
	// ID returns the canonical string identifier used in config ("copilot", etc.).
	ID() string
	// DisplayName returns the user-facing name used in prompts and launch messages.
	DisplayName() string
	// Available returns true when the provider binary is found in PATH.
	Available() bool
	// Launch launches the provider with the resolved config.
	Launch(resolved ResolvedConfig) error
	// DefaultModel returns the model id that should be used when none is set.
	// Returns "" when the provider handles its own default (e.g. Copilot).
	DefaultModel() string
	// KnownModels returns the Nav-curated model list for the setup picker.
	// Returns nil for providers without a curated list.
	KnownModels() []modelChoice
	// ValidateModel validates a model identifier for this provider.
	// Applies provider-specific shape rules on top of the base validateModelValue check.
	ValidateModel(model string) error
	// ModelAdvisory returns a soft-warning string when the model is valid but not
	// in the curated list, or "" when no advisory is needed.
	ModelAdvisory(model string) string
	// UnsupportedConfigWarnings returns informational warning strings for config
	// fields that are explicitly set but have no equivalent for this provider.
	UnsupportedConfigWarnings(resolved ResolvedConfig) []string

	// Context lifecycle — absorbs provider-specific setup and sync logic so
	// generic callers never branch on provider id.

	// Bootstrap performs first-run setup for the provider (e.g. writing OTel config,
	// seeding Nav context). Returns a short summary string for display, or ("", nil)
	// if nothing was done. Non-fatal errors are printed to stderr internally.
	Bootstrap() (string, error)
	// SyncContext syncs provider-specific context artifacts. The method handles
	// all its own output (separator, header, conflict list, success/failure messages).
	// hasPrevOutput indicates whether any scope output was already printed, so the
	// provider can emit a blank-line separator before its own header.
	// Returns a ProviderSyncResult indicating whether the provider had managed context.
	SyncContext(ref, sourceRepo string, jsonOutput, hasPrevOutput bool) ProviderSyncResult
	// ContextStatus returns the provider's managed context status for display,
	// or nil if the provider has no managed context.
	ContextStatus() *ProviderContextStatus
	// PrintContextStatus prints the text-format status block for the provider.
	// Only called when ContextStatus() returns non-nil.
	PrintContextStatus()
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
// copilotProvider
// ──────────────────────────────────────────────────────────────────────────────

type copilotProvider struct{}

func (copilotProvider) ID() string { return "copilot" }

func (copilotProvider) DisplayName() string {
	_, name := findCopilotCLI()
	return cliDisplayName(name)
}

func (copilotProvider) Available() bool {
	path, _ := findCopilotCLI()
	return path != ""
}

func (copilotProvider) Launch(r ResolvedConfig) error { return launchCopilotResolved(r) }

func (copilotProvider) DefaultModel() string { return "" } // agent picks its own default

func (copilotProvider) KnownModels() []modelChoice { return knownCopilotModels }

func (copilotProvider) ValidateModel(model string) error { return validateModelValue(model) }

func (copilotProvider) ModelAdvisory(model string) string {
	if validateModelValue(model) != nil || isKnownCopilotModel(model) {
		return ""
	}
	return fmt.Sprintf(
		"model %q is not a recognized Copilot model id; it will be sent as-is and may be rejected by the server (known ids: %s)",
		model, knownCopilotModelIDs())
}

func (copilotProvider) UnsupportedConfigWarnings(_ ResolvedConfig) []string { return nil }

func (copilotProvider) Bootstrap() (string, error) { return "", nil }
func (copilotProvider) SyncContext(_, _ string, _, _ bool) ProviderSyncResult {
	return ProviderSyncResult{}
}
func (copilotProvider) ContextStatus() *ProviderContextStatus { return nil }
func (copilotProvider) PrintContextStatus()                   {}

// ──────────────────────────────────────────────────────────────────────────────
// openCodeProvider
// ──────────────────────────────────────────────────────────────────────────────

type openCodeProvider struct{}

func (openCodeProvider) ID() string          { return "opencode" }
func (openCodeProvider) DisplayName() string { return "opencode" }

func (openCodeProvider) Available() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}

func (openCodeProvider) Launch(r ResolvedConfig) error { return launchOpenCode(r) }

func (openCodeProvider) DefaultModel() string { return openCodeDefaultModel }

func (openCodeProvider) KnownModels() []modelChoice { return knownOpenCodeModels }

func (openCodeProvider) ValidateModel(model string) error {
	if err := validateModelValue(model); err != nil {
		return err
	}
	if strings.Count(model, "/") != 1 || strings.HasSuffix(model, "/") {
		return fmt.Errorf("model %q must be in provider/model format for opencode (e.g. %q)", model, openCodeDefaultModel)
	}
	return nil
}

func (p openCodeProvider) ModelAdvisory(model string) string {
	if p.ValidateModel(model) != nil || isKnownOpenCodeModel(model) {
		return ""
	}
	return fmt.Sprintf(
		"model %q is not a Nav-curated opencode model id; it will be passed as-is (Nav default: %s, known ids: %s)",
		model, openCodeDefaultModel, knownOpenCodeModelIDs())
}

func (openCodeProvider) UnsupportedConfigWarnings(r ResolvedConfig) []string {
	return openCodeUnsupportedConfigWarnings(r)
}

// Bootstrap writes the OTel config and seeds Nav context into opencode's user
// config dir so the user is immediately ready after first-run setup.
// The OTel config error is non-fatal and printed to stderr; the context error
// is returned to the caller.
func (openCodeProvider) Bootstrap() (string, error) {
	if err := ensureOpenCodeOTelConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not configure opencode OTel: %v\n", yellow("⚠"), err)
	}
	summary, err := ensureOpenCodeNavContext()
	if err != nil {
		return "", err
	}
	return summary, nil
}

// SyncContext syncs Nav context artifacts into opencode's user config dir.
// It handles all output (separator, header, conflicts, success/failure) internally
// so the generic caller is free of opencode-specific printing logic.
// If no opencode state exists yet (context never materialized), returns Managed: false.
func (openCodeProvider) SyncContext(ref, sourceRepo string, jsonOutput, hasPrevOutput bool) ProviderSyncResult {
	ocOutputDir := openCodeNavContextDir()
	ocState, _ := readOpenCodeState(ocOutputDir)
	if ocState == nil {
		return ProviderSyncResult{}
	}

	if !jsonOutput {
		if hasPrevOutput {
			fmt.Println()
		}
		fmt.Printf("%s Syncing %s scope...\n", dim("→"), bold("opencode"))
	}

	assessment := assessStaleness(ocState.Version)
	recordFreshness("opencode", openCodeScopeName, assessment)

	ocSrc, ocSrcErr := resolveSourceForSync(ref, sourceRepo)
	if ocSrcErr != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "%s Opencode sync failed: could not resolve source: %v\n", yellow("⚠"), ocSrcErr)
			fmt.Printf("%s Opencode scope sync failed.\n", yellow("⚠"))
		}
		return ProviderSyncResult{Managed: true}
	}
	defer ocSrc.Cleanup()

	_, _, _, _, ocConflicts, ocErr := syncOpenCodeArtifacts(ocSrc.Dir, ocOutputDir, ocSrc.Version, ocSrc.SHA)
	if ocErr != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "%s Opencode sync error: %v\n", yellow("⚠"), ocErr)
			fmt.Printf("%s Opencode scope sync failed.\n", yellow("⚠"))
		}
		return ProviderSyncResult{Managed: true}
	}

	if !jsonOutput {
		for _, c := range ocConflicts {
			fmt.Printf("  %s %s (conflict — not overwritten)\n", yellow("⊘"), c)
		}
		if len(ocConflicts) > 0 {
			fmt.Printf("%s Opencode scope synced (%d conflict(s)).\n", yellow("⚠"), len(ocConflicts))
		} else {
			fmt.Printf("%s Opencode scope synced.\n", green("✓"))
		}
	}

	return ProviderSyncResult{Managed: true}
}

// ContextStatus returns the opencode managed context status, or nil if no state exists.
func (openCodeProvider) ContextStatus() *ProviderContextStatus {
	ocOutputDir := openCodeNavContextDir()
	ocState, _ := readOpenCodeState(ocOutputDir)
	if ocState == nil {
		return nil
	}
	return &ProviderContextStatus{
		State:     ocState,
		OutputDir: ocOutputDir,
		ScopeName: openCodeScopeName,
	}
}

// PrintContextStatus prints the text-format integrity status block for opencode.
func (openCodeProvider) PrintContextStatus() {
	ocOutputDir := openCodeNavContextDir()
	if ocState, _ := readOpenCodeState(ocOutputDir); ocState != nil {
		printOpenCodeStatusBlock(ocOutputDir, ocState)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// piProvider
// ──────────────────────────────────────────────────────────────────────────────

type piProvider struct{}

func (piProvider) ID() string          { return "pi" }
func (piProvider) DisplayName() string { return "pi" }

func (piProvider) Available() bool {
	_, err := exec.LookPath("pi")
	return err == nil
}

func (piProvider) Launch(_ ResolvedConfig) error { return launchPi() }

func (piProvider) DefaultModel() string                                { return "" }
func (piProvider) KnownModels() []modelChoice                          { return nil }
func (piProvider) ValidateModel(model string) error                    { return validateModelValue(model) }
func (piProvider) ModelAdvisory(_ string) string                       { return "" }
func (piProvider) UnsupportedConfigWarnings(_ ResolvedConfig) []string { return nil }

func (piProvider) Bootstrap() (string, error)                            { return "", nil }
func (piProvider) SyncContext(_, _ string, _, _ bool) ProviderSyncResult { return ProviderSyncResult{} }
func (piProvider) ContextStatus() *ProviderContextStatus                 { return nil }
func (piProvider) PrintContextStatus()                                   {}

// ──────────────────────────────────────────────────────────────────────────────
// Registry
// ──────────────────────────────────────────────────────────────────────────────

// providerRegistry is the ordered list of all supported providers.
// Adding a new provider means adding one entry here — no other files need changing.
var providerRegistry = []Provider{
	copilotProvider{},
	openCodeProvider{},
	piProvider{},
}

// providerFor returns the Provider implementation for the given id, or an error
// if the id is unknown.
func providerFor(id string) (Provider, error) {
	for _, p := range providerRegistry {
		if p.ID() == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("unknown client %q", id)
}

// allProviders returns all registered providers in registry order.
func allProviders() []Provider {
	return providerRegistry
}

// validProviderIDs is derived from the registry so there is a single source of truth
// for which provider ids are accepted. It is used for config validation and help text.
var validProviderIDs = func() []string {
	ids := make([]string, len(providerRegistry))
	for i, p := range providerRegistry {
		ids[i] = p.ID()
	}
	return ids
}()
