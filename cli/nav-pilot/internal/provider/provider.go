package provider

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// FetchLatestVersion can be injected by package main for staleness checks.
var FetchLatestVersion func() (string, string, error)

var telemetryRecorder telemetry.Recorder = telemetry.NoopRecorder{}

// ProviderSyncResult captures the outcome of a provider-driven context sync.
// Managed is true when the provider has existing managed context and ran the sync.
// Err is non-nil when the sync ran but encountered an error.
type ProviderSyncResult struct {
	Managed bool
	Err     error
}

// ProviderContextStatus holds the data needed to display provider context status.
// Returned by Provider.ContextStatus; nil means the provider has no managed context.
type ProviderContextStatus struct {
	State     *domain.StateFile
	OutputDir string
	ScopeName string
}

// Provider is the strategy abstraction for a coding-agent backend.
type Provider interface {
	ID() string
	DisplayName() string
	Available() bool
	Launch(resolved domain.ResolvedConfig) error
	DefaultModel() string
	KnownModels() []domain.ModelChoice
	ValidateModel(model string) error
	ModelAdvisory(model string) string
	UnsupportedConfigWarnings(resolved domain.ResolvedConfig) []string
	Bootstrap() (string, error)
	SyncContext(ref, sourceRepo string, jsonOutput, hasPrevOutput bool) ProviderSyncResult
	ContextStatus() *ProviderContextStatus
	PrintContextStatus()
}

// OpenCodeDefaultModel is the Nav-curated default model for opencode. opencode
// is always launched inside cplt, which connects it to the GitHub Copilot
// provider, so the model id uses the github-copilot/<id> form (see models.dev).
const OpenCodeDefaultModel = "github-copilot/claude-sonnet-4.5"

// OpenCodeAgentPersona is the materialized opencode primary agent that loads
// Nav's context and persona. Mirrors CopilotAgentPersona for the copilot client.
const OpenCodeAgentPersona = "nav-pilot"

// openCodeProviderPrefix is the opencode provider that cplt authenticates
// opencode against. Bare Copilot-style model ids are mapped under it.
const openCodeProviderPrefix = "github-copilot/"

var knownCopilotModels = []domain.ModelChoice{
	{ID: "auto", Label: "Auto (let Copilot pick)"},
	{ID: "claude-sonnet-4.6", Label: "Claude Sonnet 4.6 (default)"},
	{ID: "claude-haiku-4.5", Label: "Claude Haiku 4.5"},
	{ID: "claude-opus-4.8", Label: "Claude Opus 4.8"},
	{ID: "claude-opus-4.6", Label: "Claude Opus 4.6"},
	{ID: "gpt-5.5", Label: "GPT-5.5"},
	{ID: "gpt-5.4", Label: "GPT-5.4"},
	{ID: "gpt-5.3-codex", Label: "GPT-5.3-Codex"},
	{ID: "gpt-5.4-mini", Label: "GPT-5.4 mini"},
	{ID: "gpt-5-mini", Label: "GPT-5 mini"},
	{ID: "gemini-3.1-pro-preview", Label: "Gemini 3.1 Pro (Preview)"},
	{ID: "gemini-3.5-flash", Label: "Gemini 3.5 Flash"},
}

var knownOpenCodeModels = []domain.ModelChoice{
	{ID: OpenCodeDefaultModel, Label: "Claude Sonnet 4.5 (Nav default)"},
	{ID: "github-copilot/claude-sonnet-4.6", Label: "Claude Sonnet 4.6"},
	{ID: "github-copilot/claude-opus-4.8", Label: "Claude Opus 4.8"},
	{ID: "github-copilot/claude-haiku-4.5", Label: "Claude Haiku 4.5"},
	{ID: "github-copilot/gpt-5.5", Label: "GPT-5.5"},
	{ID: "github-copilot/gpt-5.4", Label: "GPT-5.4"},
}

// ToOpenCodeModel maps a configured model id to an opencode model id for the
// github-copilot provider that cplt connects opencode to. Empty or "auto" use
// the Nav default; ids that already carry a provider ("/") pass through; bare
// Copilot-style ids (e.g. "claude-sonnet-4.6") gain the github-copilot prefix.
func ToOpenCodeModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" || model == "auto" {
		return OpenCodeDefaultModel
	}
	if strings.Contains(model, "/") {
		return model
	}
	return openCodeProviderPrefix + model
}

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

// IsKnownCopilotModel reports whether id is in the curated Copilot model list.
func IsKnownCopilotModel(id string) bool { return isKnownCopilotModel(id) }

// KnownCopilotModelIDs returns the curated Copilot model ids as a comma-separated list.
func KnownCopilotModelIDs() string { return knownCopilotModelIDs() }

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

// IsKnownOpenCodeModel reports whether id is in the curated opencode model list.
func IsKnownOpenCodeModel(id string) bool { return isKnownOpenCodeModel(id) }

// KnownOpenCodeModelIDs returns the curated opencode model ids as a comma-separated list.
func KnownOpenCodeModelIDs() string { return knownOpenCodeModelIDs() }

type copilotProvider struct{}

func (copilotProvider) ID() string { return "copilot" }

func (copilotProvider) DisplayName() string {
	_, name := FindCopilotCLI()
	return CLIDisplayName(name)
}

func (copilotProvider) Available() bool {
	path, _ := FindCopilotCLI()
	return path != ""
}

func (copilotProvider) Launch(r domain.ResolvedConfig) error { return LaunchCopilotResolved(r) }
func (copilotProvider) DefaultModel() string                 { return "" }
func (copilotProvider) KnownModels() []domain.ModelChoice    { return knownCopilotModels }
func (copilotProvider) ValidateModel(model string) error     { return domain.ValidateModelValue(model) }

func (copilotProvider) ModelAdvisory(model string) string {
	if domain.ValidateModelValue(model) != nil || isKnownCopilotModel(model) {
		return ""
	}
	return fmt.Sprintf(
		"model %q is not a recognized Copilot model id; it will be sent as-is and may be rejected by the server (known ids: %s)",
		model, knownCopilotModelIDs())
}

func (copilotProvider) UnsupportedConfigWarnings(_ domain.ResolvedConfig) []string { return nil }
func (copilotProvider) Bootstrap() (string, error)                                 { return "", nil }
func (copilotProvider) SyncContext(_, _ string, _, _ bool) ProviderSyncResult {
	return ProviderSyncResult{}
}
func (copilotProvider) ContextStatus() *ProviderContextStatus { return nil }
func (copilotProvider) PrintContextStatus()                   {}

type openCodeProvider struct{}

func (openCodeProvider) ID() string          { return "opencode" }
func (openCodeProvider) DisplayName() string { return "opencode" }

// Available reports whether opencode can be launched: both the opencode binary
// and cplt (the sandbox launcher) must be present on PATH.
func (openCodeProvider) Available() bool {
	if _, err := exec.LookPath("opencode"); err != nil {
		return false
	}
	_, name := FindCopilotCLI()
	return name == "cplt"
}

func (openCodeProvider) Launch(r domain.ResolvedConfig) error { return LaunchOpenCode(r) }
func (openCodeProvider) DefaultModel() string                 { return OpenCodeDefaultModel }
func (openCodeProvider) KnownModels() []domain.ModelChoice    { return knownOpenCodeModels }

func (openCodeProvider) ValidateModel(model string) error {
	if err := domain.ValidateModelValue(model); err != nil {
		return err
	}
	if strings.Count(model, "/") != 1 || strings.HasSuffix(model, "/") {
		return fmt.Errorf("model %q must be in provider/model format for opencode (e.g. %q)", model, OpenCodeDefaultModel)
	}
	return nil
}

func (p openCodeProvider) ModelAdvisory(model string) string {
	if p.ValidateModel(model) != nil || isKnownOpenCodeModel(model) {
		return ""
	}
	return fmt.Sprintf(
		"model %q is not a Nav-curated opencode model id; it will be passed as-is (Nav default: %s, known ids: %s)",
		model, OpenCodeDefaultModel, knownOpenCodeModelIDs())
}

func (openCodeProvider) UnsupportedConfigWarnings(r domain.ResolvedConfig) []string {
	return OpenCodeUnsupportedConfigWarnings(r)
}

func (openCodeProvider) Bootstrap() (string, error) {
	if err := EnsureOpenCodeOTelConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not configure opencode OTel: %v\n", domain.Yellow("⚠"), err)
	}
	summary, err := EnsureOpenCodeNavContext()
	if err != nil {
		return "", err
	}
	return summary, nil
}

func (openCodeProvider) SyncContext(ref, sourceRepo string, jsonOutput, hasPrevOutput bool) ProviderSyncResult {
	ocOutputDir := openCodeNavContextDir()
	ocState, _ := artifacts.ReadOpenCodeState(ocOutputDir)
	if ocState == nil {
		return ProviderSyncResult{}
	}

	if !jsonOutput {
		if hasPrevOutput {
			fmt.Println()
		}
		fmt.Printf("%s Syncing %s scope...\n", domain.Dim("→"), domain.Bold("opencode"))
	}

	assessment := assessStaleness(ocState.Version)
	recordFreshness("opencode", artifacts.OpenCodeScopeName, assessment)

	ocSrc, ocSrcErr := source.ResolveSourceForSync(ref, sourceRepo, cliVersion)
	if ocSrcErr != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "%s Opencode sync failed: could not resolve source: %v\n", domain.Yellow("⚠"), ocSrcErr)
			fmt.Printf("%s Opencode scope sync failed.\n", domain.Yellow("⚠"))
		}
		return ProviderSyncResult{Managed: true, Err: ocSrcErr}
	}
	defer ocSrc.Cleanup()

	_, _, _, _, ocConflicts, ocErr := artifacts.SyncOpenCodeArtifacts(ocSrc.Dir, ocOutputDir, ocSrc.Version, ocSrc.SHA)
	if ocErr != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "%s Opencode sync error: %v\n", domain.Yellow("⚠"), ocErr)
			fmt.Printf("%s Opencode scope sync failed.\n", domain.Yellow("⚠"))
		}
		return ProviderSyncResult{Managed: true, Err: ocErr}
	}

	if !jsonOutput {
		for _, c := range ocConflicts {
			fmt.Printf("  %s %s (conflict — not overwritten)\n", domain.Yellow("⊘"), c)
		}
		if len(ocConflicts) > 0 {
			fmt.Printf("%s Opencode scope synced (%d conflict(s)).\n", domain.Yellow("⚠"), len(ocConflicts))
		} else {
			fmt.Printf("%s Opencode scope synced.\n", domain.Green("✓"))
		}
	}

	return ProviderSyncResult{Managed: true}
}

func (openCodeProvider) ContextStatus() *ProviderContextStatus {
	ocOutputDir := openCodeNavContextDir()
	ocState, _ := artifacts.ReadOpenCodeState(ocOutputDir)
	if ocState == nil {
		return nil
	}
	return &ProviderContextStatus{
		State:     ocState,
		OutputDir: ocOutputDir,
		ScopeName: artifacts.OpenCodeScopeName,
	}
}

func (openCodeProvider) PrintContextStatus() {
	ocOutputDir := openCodeNavContextDir()
	if ocState, _ := artifacts.ReadOpenCodeState(ocOutputDir); ocState != nil {
		artifacts.PrintOpenCodeStatusBlock(ocOutputDir, ocState)
	}
}

type piProvider struct{}

func (piProvider) ID() string                           { return "pi" }
func (piProvider) DisplayName() string                  { return "pi" }
func (piProvider) Launch(_ domain.ResolvedConfig) error { return LaunchPi() }
func (piProvider) DefaultModel() string                 { return "" }
func (piProvider) KnownModels() []domain.ModelChoice    { return nil }
func (piProvider) ValidateModel(model string) error     { return domain.ValidateModelValue(model) }
func (piProvider) ModelAdvisory(_ string) string        { return "" }
func (piProvider) UnsupportedConfigWarnings(_ domain.ResolvedConfig) []string {
	return nil
}
func (piProvider) Bootstrap() (string, error)                            { return "", nil }
func (piProvider) SyncContext(_, _ string, _, _ bool) ProviderSyncResult { return ProviderSyncResult{} }
func (piProvider) ContextStatus() *ProviderContextStatus                 { return nil }
func (piProvider) PrintContextStatus()                                   {}

func (piProvider) Available() bool {
	_, err := exec.LookPath("pi")
	return err == nil
}

var providerRegistry = []Provider{
	copilotProvider{},
	openCodeProvider{},
	piProvider{},
}

// ProviderFor returns the Provider implementation for the given id.
func ProviderFor(id string) (Provider, error) {
	for _, p := range providerRegistry {
		if p.ID() == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("unknown client %q", id)
}

// AllProviders returns all registered providers in registry order.
func AllProviders() []Provider {
	return providerRegistry
}

// ValidProviderIDs is derived from the registry and used for validation/help text.
var ValidProviderIDs = func() []string {
	ids := make([]string, len(providerRegistry))
	for i, p := range providerRegistry {
		ids[i] = p.ID()
	}
	return ids
}()

func assessStaleness(installedVersion string) artifacts.StalenessAssessment {
	fetchFn := FetchLatestVersion
	if fetchFn == nil {
		fetchFn = func() (string, string, error) {
			return "", "", fmt.Errorf("no fetch function")
		}
	}
	return artifacts.AssessStaleness(installedVersion, fetchFn)
}

func recordFreshness(component, scope string, a artifacts.StalenessAssessment) {
	telemetryRecorder.RecordStalenessCheck(component, scope, a.Result)
	switch a.Result {
	case "lookup_failed", "dev", "no_install", "corrupted":
		return
	}
	if a.LatestVersion == "" {
		return
	}
	telemetryRecorder.RecordUpToDate(component, scope, a.UpToDate)
	if a.HasSkew {
		telemetryRecorder.RecordVersionSkewDays(component, scope, a.SkewDays)
	}
}

// RecordFreshness records freshness telemetry for provider-managed artifacts.
func RecordFreshness(component, scope string, a artifacts.StalenessAssessment) {
	recordFreshness(component, scope, a)
}

// SetTelemetry sets the recorder used for provider freshness telemetry.
func SetTelemetry(r telemetry.Recorder) {
	if r == nil {
		telemetryRecorder = telemetry.NoopRecorder{}
		return
	}
	telemetryRecorder = r
}
