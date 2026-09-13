package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// markerSource writes a minimal agentpakke checkout holding one agent and
// returns its directory. The agent's name is the marker the assertions read: it
// says which source a materialization actually came from.
func markerSource(t *testing.T, agent string) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "agents", agent+".agent.md"),
		"---\nname: "+agent+"\ndescription: Marker agent\n---\n\nI am "+agent+".\n")
	return dir
}

// TestBootstrapMaterializesConfiguredSource is #813: a first launch has no prior
// sync state, so a provider handed no source resolved the built-in default and
// materialized stock nav-pilot over the pakke the user had configured. Both Tier
// 1 providers are covered, because the construction — and the bug — was shared.
func TestBootstrapMaterializesConfiguredSource(t *testing.T) {
	// Away from the navikt/copilot checkout the tests run inside, so a
	// source-less resolve reaches the clone hook rather than auto-detecting it.
	t.Chdir(t.TempDir())

	builtin := markerSource(t, "builtin-agent")
	origClone := source.CloneRemoteFn
	t.Cleanup(func() { source.CloneRemoteFn = origClone })
	source.CloneRemoteFn = func(_, _ string) (*source.Source, error) {
		return &source.Source{Dir: builtin, SHA: "builtin"}, nil
	}

	// An absolute path resolves without cloning, which is exactly what a
	// developer pointing `source` at a local pakke checkout gets.
	configured := markerSource(t, "configured-agent")

	assertConfigured := func(t *testing.T, outputDir string) {
		t.Helper()
		if _, err := os.Stat(filepath.Join(outputDir, "agents", "configured-agent.md")); err != nil {
			t.Errorf("configured source not materialized: %v", err)
		}
		if _, err := os.Stat(filepath.Join(outputDir, "agents", "builtin-agent.md")); err == nil {
			t.Error("built-in source materialized instead of the configured one")
		}
	}

	t.Run("opencode", func(t *testing.T) {
		outputDir := t.TempDir()
		NavContextDirOverride = outputDir
		// Bootstrap also writes opencode's OTel config; without this it lands
		// in the developer's real ~/.config/opencode.
		ConfigPathOverride = filepath.Join(t.TempDir(), "opencode.json")
		t.Cleanup(func() { NavContextDirOverride = ""; ConfigPathOverride = "" })

		if _, err := (openCodeProvider{}).Bootstrap(domain.ResolvedConfig{Source: configured}); err != nil {
			t.Fatalf("Bootstrap() error: %v", err)
		}
		assertConfigured(t, outputDir)
	})

	// The other half of the precedence rule the fix introduced: the caller's
	// source wins, and only then does whatever the last sync recorded apply.
	t.Run("an explicit source outranks the recorded one", func(t *testing.T) {
		outputDir := t.TempDir()
		NavContextDirOverride = outputDir
		ConfigPathOverride = filepath.Join(t.TempDir(), "opencode.json")
		t.Cleanup(func() { NavContextDirOverride = ""; ConfigPathOverride = "" })

		if _, err := (openCodeProvider{}).Bootstrap(domain.ResolvedConfig{Source: configured}); err != nil {
			t.Fatalf("first Bootstrap() error: %v", err)
		}
		switched := markerSource(t, "switched-agent")
		if _, err := (openCodeProvider{}).Bootstrap(domain.ResolvedConfig{Source: switched}); err != nil {
			t.Fatalf("second Bootstrap() error: %v", err)
		}
		if _, err := os.Stat(filepath.Join(outputDir, "agents", "switched-agent.md")); err != nil {
			t.Errorf("state-recorded source outranked the configured one: %v", err)
		}
	})

	t.Run("pi", func(t *testing.T) {
		SetActivePakke(&agentpakke.Manifest{
			Name:    "p",
			Clients: map[string]agentpakke.ClientEntry{"pi": {PrimaryAgents: []string{"configured-agent"}}},
			Layout:  &agentpakke.Layout{Agents: "agents", Skills: "skills"},
		})
		outputDir := t.TempDir()
		PiNavContextDirOverride = outputDir
		t.Cleanup(func() { SetActivePakke(nil); PiNavContextDirOverride = "" })

		if _, err := (piProvider{}).Bootstrap(domain.ResolvedConfig{Source: configured}); err != nil {
			t.Fatalf("Bootstrap() error: %v", err)
		}
		assertConfigured(t, outputDir)
	})
}
