package provider

import (
	"fmt"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// --- SetVersion ---

func TestSetVersion(t *testing.T) {
	old := cliVersion
	defer func() { cliVersion = old }()

	SetVersion("1.2.3")
	if cliVersion != "1.2.3" {
		t.Errorf("cliVersion = %q, want 1.2.3", cliVersion)
	}
	SetVersion("dev")
	if cliVersion != "dev" {
		t.Errorf("cliVersion = %q, want dev", cliVersion)
	}
}

// --- SetTelemetry ---

func TestSetTelemetry(t *testing.T) {
	orig := telemetryRecorder
	defer func() { telemetryRecorder = orig }()

	SetTelemetry(nil)
	if _, ok := telemetryRecorder.(telemetry.NoopRecorder); !ok {
		t.Error("SetTelemetry(nil) should set NoopRecorder")
	}
	SetTelemetry(telemetry.NoopRecorder{})
	if _, ok := telemetryRecorder.(telemetry.NoopRecorder); !ok {
		t.Error("SetTelemetry(NoopRecorder{}) should set NoopRecorder")
	}
}

// --- copilotProvider.DisplayName ---

func TestCopilotProvider_DisplayName(t *testing.T) {
	var p Provider = copilotProvider{}
	name := p.DisplayName()
	// Name is environment-dependent; just verify it's a non-empty string.
	if name == "" {
		// No copilot CLI on PATH is acceptable; CLIDisplayName("") returns ""
		// which is the defined behavior. We only check it doesn't panic.
		return
	}
	_ = name
}

// --- openCodeProvider.DisplayName and KnownModels ---

func TestOpenCodeProvider_DisplayNameAndKnownModels(t *testing.T) {
	var p Provider = openCodeProvider{}
	if p.DisplayName() != "opencode" {
		t.Errorf("DisplayName() = %q, want opencode", p.DisplayName())
	}
	models := p.KnownModels()
	if len(models) == 0 {
		t.Error("KnownModels() is empty")
	}
	found := false
	for _, m := range models {
		if m.ID == OpenCodeDefaultModel {
			found = true
		}
	}
	if !found {
		t.Errorf("KnownModels() missing default model %q", OpenCodeDefaultModel)
	}
}

// --- openCodeProvider.PrintContextStatus with no state ---

func TestOpenCodeProvider_PrintContextStatusNoState(t *testing.T) {
	old := NavContextDirOverride
	NavContextDirOverride = t.TempDir()
	defer func() { NavContextDirOverride = old }()

	var p Provider = openCodeProvider{}
	// Should not panic even with no state file present.
	p.PrintContextStatus()
}

// --- piProvider.DisplayName, DefaultModel, ValidateModel, UnsupportedConfigWarnings ---

func TestPiProvider_DisplayName(t *testing.T) {
	var p Provider = piProvider{}
	if p.DisplayName() != "pi" {
		t.Errorf("DisplayName() = %q, want pi", p.DisplayName())
	}
}

func TestPiProvider_DefaultModel(t *testing.T) {
	var p Provider = piProvider{}
	if p.DefaultModel() != "" {
		t.Errorf("DefaultModel() = %q, want empty", p.DefaultModel())
	}
}

func TestPiProvider_ValidateModel(t *testing.T) {
	var p Provider = piProvider{}
	if err := p.ValidateModel("gpt-4"); err != nil {
		t.Errorf("ValidateModel(valid) = %v, want nil", err)
	}
	if err := p.ValidateModel(""); err == nil {
		t.Error("ValidateModel(empty) = nil, want error")
	}
}

func TestPiProvider_UnsupportedConfigWarnings(t *testing.T) {
	var p Provider = piProvider{}
	r := domain.ResolvedConfig{Mode: "autopilot", ContextTier: "long_context"}
	if w := p.UnsupportedConfigWarnings(r); len(w) != 0 {
		t.Errorf("piProvider.UnsupportedConfigWarnings() = %v, want empty", w)
	}
}

func TestPiProvider_PrintContextStatus(t *testing.T) {
	var p Provider = piProvider{}
	// Should not panic.
	p.PrintContextStatus()
}

// --- openCodeProvider.SyncContext error propagation (G2) ---

func TestOpenCodeProvider_SyncContext_PropagatesSourceError(t *testing.T) {
	outputDir := t.TempDir()
	old := NavContextDirOverride
	NavContextDirOverride = outputDir
	defer func() { NavContextDirOverride = old }()

	// Write an opencode state file so SyncContext knows this scope is managed.
	state := &domain.StateFile{
		Collection: "opencode-export",
		Version:    "2026.01.01",
		Scope:      "opencode",
		SourceSHA:  "abc123",
	}
	if err := artifacts.WriteOpenCodeState(outputDir, state); err != nil {
		t.Fatalf("WriteOpenCodeState: %v", err)
	}

	origClone := source.CloneRemoteFn
	defer func() { source.CloneRemoteFn = origClone }()
	source.CloneRemoteFn = func(ref, sourceRepo string) (*source.Source, error) {
		return nil, fmt.Errorf("network unavailable")
	}

	var p Provider = openCodeProvider{}
	res := p.SyncContext("", "", true, false)

	if !res.Managed {
		t.Error("SyncContext() Managed = false, want true")
	}
	if res.Err == nil {
		t.Error("SyncContext() Err = nil, want non-nil when source resolution fails")
	}
}

func TestRecordFreshness_Smoke(t *testing.T) {
	orig := telemetryRecorder
	defer func() { telemetryRecorder = orig }()
	SetTelemetry(telemetry.NoopRecorder{})

	// should not panic with various result strings
	a1 := assessStaleness("dev")
	RecordFreshness("copilot", "repo", a1)

	a2 := assessStaleness("")
	RecordFreshness("opencode", "user", a2)
}

// --- Available() methods ---

func TestCopilotProvider_Available(t *testing.T) {
	var p Provider = copilotProvider{}
	// Available() depends on PATH; just ensure it doesn't panic.
	_ = p.Available()
}

func TestOpenCodeProvider_Available(t *testing.T) {
	var p Provider = openCodeProvider{}
	_ = p.Available()
}

func TestPiProvider_Available(t *testing.T) {
	var p Provider = piProvider{}
	_ = p.Available()
}
