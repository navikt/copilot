package provider

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// stagedFixturePakke mirrors the shape of grillmester's manifest at the pinned
// reference SHA 3573b93cc8b7568516117263562d073cae9ee7fc: both clients declare
// the same primaries, defaultModel "inherit", defaultContext "full", and two
// payload contexts. WP6 replaces the fixture with the vendored real manifest.
func stagedFixturePakke() *agentpakke.Manifest {
	entry := func() agentpakke.ClientEntry {
		return agentpakke.ClientEntry{
			PrimaryAgents:  []string{"grillmester", "barista", "designer", "doctor-who"},
			DefaultModel:   agentpakke.InheritModel,
			DefaultContext: "full",
			Payloads: map[string]agentpakke.Payload{
				"full":    {Path: "plugin"},
				"focused": {Path: "targets/copilot-cli-focused-v1"},
			},
		}
	}
	return &agentpakke.Manifest{
		ContractVersion: "1",
		Name:            "grillmester",
		Clients: map[string]agentpakke.ClientEntry{
			"copilot":  entry(),
			"opencode": entry(),
		},
	}
}

func buildStagedSpec(t *testing.T, client string, r domain.ResolvedConfig, s StagedLaunch) cpltLaunch {
	t.Helper()
	var (
		spec cpltLaunch
		err  error
	)
	if client == "opencode" {
		spec, err = buildStagedOpenCodeSpec(r, s)
	} else {
		spec, err = buildStagedCopilotSpec(r, s)
	}
	if err != nil {
		t.Fatalf("building staged %s spec: %v", client, err)
	}
	return spec
}

// TestStagedLaunchSpecs is the four-scenario invocation table: the two clients
// times the two payload contexts a Tier 2 agentpakke declares.
//
// Every expected value is transcribed from the reference launcher, grillmester
// at 3573b93cc8b7568516117263562d073cae9ee7fc, scripts/grillmester.py
// build_launch_command:
//
//	line 664-665   cplt --agent <client>
//	line 668-669   --allow-read <payload>   (payload = plugin for copilot,
//	               opencode_target for opencode; both clients read exactly the
//	               one staged tree)
//	line 673       OPENCODE_CONFIG_DIR=<payload>       (opencode only)
//	line 674       --pass-env OPENCODE_CONFIG_DIR      (opencode only)
//	line 687       the "--" separator
//	line 698-699   opencode client args: --agent <agent>
//	line 679-684   copilot client args: --plugin-dir <plugin> --agent grillmester:<agent>
//
// Not adopted, deliberately: --no-audit (line 663) and --project-dir (lines
// 666-667), which are launcher policy rather than payload contract. No --model
// either: the fixture declares "inherit", and the reference forwards no model
// at all.
func TestStagedLaunchSpecs(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })

	fullDir := filepath.Join(t.TempDir(), "grillmester-copilot-full-abc123")
	focusedDir := filepath.Join(t.TempDir(), "grillmester-opencode-focused-def456")

	tests := []struct {
		name          string
		client        string
		staged        StagedLaunch
		wantCpltAgent string
		wantCpltArgs  []string
		wantAgentArgs []string
		wantConfigDir string // OPENCODE_CONFIG_DIR, "" when it must be unset
	}{
		{
			name:          "copilot/full",
			client:        "copilot",
			staged:        StagedLaunch{Dir: fullDir, PakkeName: "grillmester", Context: "full"},
			wantCpltAgent: "copilot",
			wantCpltArgs:  []string{"--allow-read", fullDir},
			wantAgentArgs: []string{"--plugin-dir", fullDir, "--agent", "grillmester:grillmester"},
		},
		{
			name:          "copilot/focused",
			client:        "copilot",
			staged:        StagedLaunch{Dir: focusedDir, PakkeName: "grillmester", Context: "focused"},
			wantCpltAgent: "copilot",
			wantCpltArgs:  []string{"--allow-read", focusedDir},
			wantAgentArgs: []string{"--plugin-dir", focusedDir, "--agent", "grillmester:grillmester"},
		},
		{
			name:          "opencode/full",
			client:        "opencode",
			staged:        StagedLaunch{Dir: fullDir, PakkeName: "grillmester", Context: "full"},
			wantCpltAgent: "opencode",
			wantCpltArgs:  []string{"--allow-read", fullDir, "--pass-env", "OPENCODE_CONFIG_DIR"},
			wantAgentArgs: []string{"--agent", "grillmester"},
			wantConfigDir: fullDir,
		},
		{
			name:          "opencode/focused",
			client:        "opencode",
			staged:        StagedLaunch{Dir: focusedDir, PakkeName: "grillmester", Context: "focused"},
			wantCpltAgent: "opencode",
			wantCpltArgs:  []string{"--allow-read", focusedDir, "--pass-env", "OPENCODE_CONFIG_DIR"},
			wantAgentArgs: []string{"--agent", "grillmester"},
			wantConfigDir: focusedDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// AskUser: true keeps the copilot flag tail empty, so the vector is
			// exactly the reference shape; the tail itself is pinned below.
			r := domain.ResolvedConfig{Client: tt.client, AskUser: true}
			spec := buildStagedSpec(t, tt.client, r, tt.staged)

			if spec.agent != tt.wantCpltAgent {
				t.Errorf("cplt --agent = %q, want %q", spec.agent, tt.wantCpltAgent)
			}
			if !slices.Equal(spec.cpltArgs, tt.wantCpltArgs) {
				t.Errorf("cpltArgs\n got: %q\nwant: %q", spec.cpltArgs, tt.wantCpltArgs)
			}
			if !slices.Equal(spec.agentArgs, tt.wantAgentArgs) {
				t.Errorf("agentArgs\n got: %q\nwant: %q", spec.agentArgs, tt.wantAgentArgs)
			}
			if got := telemetry.LookupEnvValue(spec.env, "OPENCODE_CONFIG_DIR"); got != tt.wantConfigDir {
				t.Errorf("OPENCODE_CONFIG_DIR = %q, want %q", got, tt.wantConfigDir)
			}
			if !strings.Contains(spec.messageSuffix, tt.staged.Context) {
				t.Errorf("message suffix %q should name the payload context", spec.messageSuffix)
			}
		})
	}
}

// TestStagedLaunchModel pins the F1 manifest-default-model half: "inherit"
// forwards no --model (as the reference does), a concrete manifest default is
// forwarded, and a user pin always wins.
func TestStagedLaunchModel(t *testing.T) {
	t.Cleanup(func() { SetActivePakke(nil) })
	staged := StagedLaunch{Dir: "/staged/x", PakkeName: "grillmester", Context: "full"}

	concrete := func(model string) *agentpakke.Manifest {
		m := stagedFixturePakke()
		for id, entry := range m.Clients {
			entry.DefaultModel = model
			m.Clients[id] = entry
		}
		return m
	}

	tests := []struct {
		name       string
		pakke      *agentpakke.Manifest
		userModel  string
		wantOpen   []string
		wantCopilo []string
	}{
		{
			name:       "inherit forwards no model",
			pakke:      stagedFixturePakke(),
			wantOpen:   []string{"--agent", "grillmester"},
			wantCopilo: []string{"--plugin-dir", staged.Dir, "--agent", "grillmester:grillmester"},
		},
		{
			name:       "manifest default model is forwarded",
			pakke:      concrete("github-copilot/claude-opus-5"),
			wantOpen:   []string{"--agent", "grillmester", "--model", "github-copilot/claude-opus-5"},
			wantCopilo: []string{"--plugin-dir", staged.Dir, "--agent", "grillmester:grillmester", "--model", "github-copilot/claude-opus-5"},
		},
		{
			name:       "user pin wins over the manifest default",
			pakke:      concrete("github-copilot/claude-opus-5"),
			userModel:  "gpt-5.5",
			wantOpen:   []string{"--agent", "grillmester", "--model", "github-copilot/gpt-5.5"},
			wantCopilo: []string{"--plugin-dir", staged.Dir, "--agent", "grillmester:grillmester", "--model", "gpt-5.5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetActivePakke(tt.pakke)
			r := domain.ResolvedConfig{AskUser: true, Model: tt.userModel}

			if got := buildStagedSpec(t, "opencode", r, staged).agentArgs; !slices.Equal(got, tt.wantOpen) {
				t.Errorf("opencode agentArgs\n got: %q\nwant: %q", got, tt.wantOpen)
			}
			if got := buildStagedSpec(t, "copilot", r, staged).agentArgs; !slices.Equal(got, tt.wantCopilo) {
				t.Errorf("copilot agentArgs\n got: %q\nwant: %q", got, tt.wantCopilo)
			}
		})
	}
}

// TestStagedLaunchForwardsResolvedConfig pins the rest of each client's tail:
// copilot reuses the shared resolved-flag helper (the same one BuildCopilotArgs
// emits, so the golden legacy vectors stay valid), opencode keeps mapping plan
// mode to its built-in plan agent, and both append ExtraArgs last.
func TestStagedLaunchForwardsResolvedConfig(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })
	staged := StagedLaunch{Dir: "/staged/x", PakkeName: "grillmester", Context: "full"}

	r := domain.ResolvedConfig{
		Mode:            "autopilot",
		ReasoningEffort: "high",
		ContextTier:     "large",
		AllowAllTools:   true,
		AskUser:         false,
		LogLevel:        "debug",
		ExtraArgs:       []string{"-p", "hei"},
	}
	want := []string{
		"--plugin-dir", staged.Dir,
		"--agent", "grillmester:grillmester",
		"--mode", "autopilot",
		"--effort", "high",
		"--context", "large",
		"--allow-all-tools",
		"--no-ask-user",
		"--log-level", "debug",
		"-p", "hei",
	}
	if got := buildStagedSpec(t, "copilot", r, staged).agentArgs; !slices.Equal(got, want) {
		t.Errorf("copilot agentArgs\n got: %q\nwant: %q", got, want)
	}

	plan := domain.ResolvedConfig{Mode: "plan", AskUser: true, ExtraArgs: []string{"--foo"}}
	wantPlan := []string{"--agent", "plan", "--foo"}
	if got := buildStagedSpec(t, "opencode", plan, staged).agentArgs; !slices.Equal(got, wantPlan) {
		t.Errorf("opencode agentArgs (plan mode)\n got: %q\nwant: %q", got, wantPlan)
	}
}

// TestStagedCopilotRequiresCplt pins the decision that a staged copilot launch
// never falls back to a plain, unsandboxed copilot binary.
func TestStagedCopilotRequiresCplt(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })

	// A PATH with a plain `copilot` that is not cplt: the legacy path would
	// launch it, the staged path must refuse.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "copilot"), []byte("#!/bin/sh\necho copilot 1.2.3\n"), 0o755); err != nil {
		t.Fatalf("writing fake copilot: %v", err)
	}
	t.Setenv("PATH", dir)

	err := LaunchCopilotStaged(domain.ResolvedConfig{Client: "copilot", AskUser: true},
		StagedLaunch{Dir: dir, PakkeName: "grillmester", Context: "full"})
	if err == nil {
		t.Fatal("LaunchCopilotStaged must refuse to launch without cplt")
	}
	if !strings.Contains(err.Error(), "brew install navikt/tap/cplt") {
		t.Errorf("error should name the install command, got: %v", err)
	}
}

// TestStagedSpecRequiresDeclaredPrimary covers call-site drift: a pakke that
// does not declare the client fails loudly instead of passing --agent "".
func TestStagedSpecRequiresDeclaredPrimary(t *testing.T) {
	t.Cleanup(func() { SetActivePakke(nil) })
	SetActivePakke(&agentpakke.Manifest{Name: "empty", Clients: map[string]agentpakke.ClientEntry{}})

	staged := StagedLaunch{Dir: "/staged/x", PakkeName: "empty", Context: "full"}
	if _, err := buildStagedCopilotSpec(domain.ResolvedConfig{}, staged); err == nil {
		t.Error("buildStagedCopilotSpec must fail when the pakke declares no copilot primary")
	}
	if _, err := buildStagedOpenCodeSpec(domain.ResolvedConfig{}, staged); err == nil {
		t.Error("buildStagedOpenCodeSpec must fail when the pakke declares no opencode primary")
	}
}

// TestStagedOpenCodeLeavesSharedConfigAlone is the G2 no-regression pin: the
// staged opencode path must not read-modify-write the user's shared
// ~/.config/opencode/opencode.json the way LaunchOpenCode does via
// EnsureOpenCodeOTelConfig / EnsureOpenCodeNavContext.
func TestStagedOpenCodeLeavesSharedConfigAlone(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })

	configDir := t.TempDir()
	sentinelPath := filepath.Join(configDir, "opencode.json")
	sentinel := []byte(`{"theme":"mine"}`)
	if err := os.WriteFile(sentinelPath, sentinel, 0o600); err != nil {
		t.Fatalf("writing sentinel config: %v", err)
	}
	ConfigPathOverride = sentinelPath
	NavContextDirOverride = configDir
	t.Cleanup(func() { ConfigPathOverride, NavContextDirOverride = "", "" })

	staged := StagedLaunch{Dir: t.TempDir(), PakkeName: "grillmester", Context: "full"}
	buildStagedSpec(t, "opencode", domain.ResolvedConfig{Client: "opencode"}, staged)

	// The launcher too, up to the point where it would exec: an empty PATH
	// stops it at the opencode lookup, after any config work would have run.
	t.Setenv("PATH", t.TempDir())
	if err := LaunchOpenCodeStaged(domain.ResolvedConfig{Client: "opencode"}, staged); err == nil {
		t.Fatal("LaunchOpenCodeStaged should fail with an empty PATH")
	}

	got, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatalf("reading sentinel config back: %v", err)
	}
	if string(got) != string(sentinel) {
		t.Errorf("staged launch modified the shared opencode config:\n got: %s\nwant: %s", got, sentinel)
	}

	// And it must not create the file when the user has none.
	absent := filepath.Join(t.TempDir(), "nested", "opencode.json")
	ConfigPathOverride = absent
	buildStagedSpec(t, "opencode", domain.ResolvedConfig{Client: "opencode"}, staged)
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Errorf("staged launch created %s; it must never touch the shared config", absent)
	}
}

// TestGoldenCpltArgvWithoutCpltArgs pins that the new pre-"--" slot is inert
// for every legacy launch: with no cpltArgs, cplt receives exactly today's
// vector.
func TestGoldenCpltArgvWithoutCpltArgs(t *testing.T) {
	spec := cpltLaunch{agent: "opencode", agentArgs: OpenCodeArgs(domain.ResolvedConfig{})}
	want := []string{"--agent", "opencode", "--", "--model", "github-copilot/auto", "--agent", "nav-pilot"}
	if got := cpltArgv(spec); !slices.Equal(got, want) {
		t.Errorf("cpltArgv\n got: %q\nwant: %q", got, want)
	}

	pi := cpltLaunch{agent: "pi"}
	wantPi := []string{"--agent", "pi", "--"}
	if got := cpltArgv(pi); !slices.Equal(got, wantPi) {
		t.Errorf("cpltArgv(pi)\n got: %q\nwant: %q", got, wantPi)
	}
}

// TestOpenCodeDefaultModelFollowsPakke covers the WP2 review finding that the
// provider's advertised default and the launch fallback could disagree.
func TestOpenCodeDefaultModelFollowsPakke(t *testing.T) {
	t.Cleanup(func() { SetActivePakke(nil) })

	SetActivePakke(&agentpakke.Manifest{
		Name:    "grillmester",
		Clients: map[string]agentpakke.ClientEntry{"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: "github-copilot/claude-opus-5"}},
	})
	if got := (openCodeProvider{}).DefaultModel(); got != "github-copilot/claude-opus-5" {
		t.Errorf("DefaultModel() = %q, want the active pakke's declaration", got)
	}

	SetActivePakke(&agentpakke.Manifest{
		Name:    "grillmester",
		Clients: map[string]agentpakke.ClientEntry{"opencode": {PrimaryAgents: []string{"grillmester"}, DefaultModel: agentpakke.InheritModel}},
	})
	if got := (openCodeProvider{}).DefaultModel(); got != OpenCodeDefaultModel {
		t.Errorf("DefaultModel() under an inherit pakke = %q, want the built-in %q", got, OpenCodeDefaultModel)
	}
	if got := ToOpenCodeModel(""); got != OpenCodeDefaultModel {
		t.Errorf("ToOpenCodeModel(\"\") under an inherit pakke = %q, want the built-in %q", got, OpenCodeDefaultModel)
	}
}
