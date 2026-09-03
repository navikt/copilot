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

// stagedFixturePakke mirrors the shape of grillmester's manifest as the
// contract requires it after the WP7 roster correction: defaultModel
// "inherit", defaultContext "full", two payload contexts, and a roster on each
// *payload* rather than on the client entry. The rosters are the real ones —
// the full payloads lead with grillmester, both focused payloads ship only
// barista and grill-inspektor (#437, comment 5437575432) — which is exactly
// why a client-level roster could not answer for both. WP6 replaces the
// fixture with the vendored real manifest at the new pinned SHA.
//
// The client-level PrimaryAgents is left deliberately wrong here: it is the
// old, full-context-only list, and no staged assertion in this file may match
// it for the focused context. A fallback would show up as barista turning back
// into grillmester.
func stagedFixturePakke() *agentpakke.Manifest {
	entry := func() agentpakke.ClientEntry {
		return agentpakke.ClientEntry{
			PrimaryAgents:  []string{"grillmester", "barista", "designer", "doctor-who"},
			DefaultModel:   agentpakke.InheritModel,
			DefaultContext: "full",
			Payloads: map[string]agentpakke.Payload{
				"full": {
					Path:          "plugin",
					PrimaryAgents: []string{"grillmester", "barista", "designer", "doctor-who"},
				},
				"focused": {
					Path:          "targets/copilot-cli-focused-v1",
					PrimaryAgents: []string{"barista", "grill-inspektor"},
				},
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
//	line 698-699   opencode client args, no forwarded arguments: --agent <agent>
//	line 700-701   opencode client args, "run …": run --agent <agent> …
//	line 702-703   opencode client args, another opencode subcommand: forwarded
//	               unchanged, with no --agent bound to it
//	line 704       opencode client args, anything else: --agent <agent> …
//	line 679-684   copilot client args: --plugin-dir <plugin> --agent grillmester:<agent>
//
// The persona itself is not transcribed from the reference: it is the first
// entry of the launched context's own primaryAgents roster (WP7), which is why
// the two focused rows expect barista where the full rows expect grillmester.
//
// Not adopted, deliberately: --project-dir (lines 666-667) — nav-pilot treats
// the working directory as the project scope, which is what the client inherits
// anyway. No --model either: the fixture declares "inherit", and the reference
// forwards no model at all.
func TestStagedLaunchSpecs(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })

	fullDir := filepath.Join(t.TempDir(), "grillmester-copilot-full-abc123")
	focusedDir := filepath.Join(t.TempDir(), "grillmester-opencode-focused-def456")

	tests := []struct {
		name          string
		client        string
		staged        StagedLaunch
		extraArgs     []string
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
			// The focused payload ships no grillmester agent: its roster is
			// barista first, so the persona is grillmester:barista.
			name:          "copilot/focused",
			client:        "copilot",
			staged:        StagedLaunch{Dir: focusedDir, PakkeName: "grillmester", Context: "focused"},
			wantCpltAgent: "copilot",
			wantCpltArgs:  []string{"--allow-read", focusedDir},
			wantAgentArgs: []string{"--plugin-dir", focusedDir, "--agent", "grillmester:barista"},
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
			// Same for opencode: the focused roster's first element wins.
			name:          "opencode/focused",
			client:        "opencode",
			staged:        StagedLaunch{Dir: focusedDir, PakkeName: "grillmester", Context: "focused"},
			wantCpltAgent: "opencode",
			wantCpltArgs:  []string{"--allow-read", focusedDir, "--pass-env", "OPENCODE_CONFIG_DIR"},
			wantAgentArgs: []string{"--agent", "barista"},
			wantConfigDir: focusedDir,
		},
		{
			// Reference lines 700-701: "run" keeps its leading position and the
			// agent is bound after it.
			name:          "opencode/run reorders the agent binding",
			client:        "opencode",
			staged:        StagedLaunch{Dir: fullDir, PakkeName: "grillmester", Context: "full"},
			extraArgs:     []string{"run", "hi"},
			wantCpltAgent: "opencode",
			wantCpltArgs:  []string{"--allow-read", fullDir, "--pass-env", "OPENCODE_CONFIG_DIR"},
			wantAgentArgs: []string{"run", "--agent", "grillmester", "hi"},
			wantConfigDir: fullDir,
		},
		{
			// Reference lines 702-703: a bare opencode subcommand is not a
			// session entry point, so no --agent may be bound to it.
			name:          "opencode/subcommand takes no agent binding",
			client:        "opencode",
			staged:        StagedLaunch{Dir: fullDir, PakkeName: "grillmester", Context: "full"},
			extraArgs:     []string{"auth", "login"},
			wantCpltAgent: "opencode",
			wantCpltArgs:  []string{"--allow-read", fullDir, "--pass-env", "OPENCODE_CONFIG_DIR"},
			wantAgentArgs: []string{"auth", "login"},
			wantConfigDir: fullDir,
		},
		{
			// Reference line 704: anything that is not a subcommand is session
			// arguments, which follow the agent binding.
			name:          "opencode/flags follow the agent binding",
			client:        "opencode",
			staged:        StagedLaunch{Dir: fullDir, PakkeName: "grillmester", Context: "full"},
			extraArgs:     []string{"--port", "4096"},
			wantCpltAgent: "opencode",
			wantCpltArgs:  []string{"--allow-read", fullDir, "--pass-env", "OPENCODE_CONFIG_DIR"},
			wantAgentArgs: []string{"--agent", "grillmester", "--port", "4096"},
			wantConfigDir: fullDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// AskUser: true keeps the copilot flag tail empty, so the vector is
			// exactly the reference shape; the tail itself is pinned below.
			r := domain.ResolvedConfig{Client: tt.client, AskUser: true, ExtraArgs: tt.extraArgs}
			spec := buildStagedSpec(t, tt.client, r, tt.staged)

			if spec.agent != tt.wantCpltAgent {
				t.Errorf("cplt --agent = %q, want %q", spec.agent, tt.wantCpltAgent)
			}
			if !slices.Equal(spec.cpltArgs, tt.wantCpltArgs) {
				t.Errorf("cpltArgs\n got: %q\nwant: %q", spec.cpltArgs, tt.wantCpltArgs)
			}
			// The reference emits --no-audit (line 663); nav-pilot does not.
			// The flag was only ever there because cplt's parent-side audit
			// could execute repository-controlled Git helpers outside the
			// sandbox, which navikt/cplt#211 fixed and minStagedCpltStamp now
			// requires. The vector must lead straight with --agent.
			argv := cpltArgv(spec)
			if slices.Contains(argv, "--no-audit") {
				t.Errorf("staged vector must not suppress cplt's audit\n got: %q", argv)
			}
			if len(argv) == 0 || argv[0] != "--agent" {
				t.Errorf("cplt vector must lead with --agent\n got: %q", argv)
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

// TestStagedSpecDoesNotFallBackToClientRoster pins the WP7 rule that the
// payload roster has no fallback: a client entry with a perfectly good
// client-level primaryAgents, launched for a context it declares no payload
// for, must refuse rather than launch the client-level persona against a tree
// that may not ship it. The schema makes this unreachable for a loaded
// manifest; this is the code-level guard behind it.
func TestStagedSpecDoesNotFallBackToClientRoster(t *testing.T) {
	t.Cleanup(func() { SetActivePakke(nil) })
	SetActivePakke(&agentpakke.Manifest{
		Name: "grillmester",
		Clients: map[string]agentpakke.ClientEntry{
			"copilot": {
				PrimaryAgents: []string{"grillmester"},
				Payloads:      map[string]agentpakke.Payload{"full": {Path: "plugin", PrimaryAgents: []string{"grillmester"}}},
			},
			"opencode": {
				PrimaryAgents: []string{"grillmester"},
				Payloads:      map[string]agentpakke.Payload{"full": {Path: "plugin", PrimaryAgents: []string{"grillmester"}}},
			},
		},
	})

	staged := StagedLaunch{Dir: "/staged/x", PakkeName: "grillmester", Context: "focused"}
	for _, tc := range []struct {
		client string
		build  func(domain.ResolvedConfig, StagedLaunch) (cpltLaunch, error)
	}{
		{"copilot", buildStagedCopilotSpec},
		{"opencode", buildStagedOpenCodeSpec},
	} {
		spec, err := tc.build(domain.ResolvedConfig{}, staged)
		if err == nil {
			t.Errorf("%s: undeclared context launched %q instead of failing", tc.client, spec.agentArgs)
			continue
		}
		if !strings.Contains(err.Error(), "focused") {
			t.Errorf("%s: error should name the context, got: %v", tc.client, err)
		}
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

	// The launcher too, all the way to the exec. A PATH carrying opencode but
	// no cplt gets it past its own lookup — which is where the legacy path does
	// its config work — and stops it at launchViaCplt instead.
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "opencode"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("writing fake opencode: %v", err)
	}
	t.Setenv("PATH", binDir)
	if err := LaunchOpenCodeStaged(domain.ResolvedConfig{Client: "opencode"}, staged); err == nil {
		t.Fatal("LaunchOpenCodeStaged should fail without cplt on PATH")
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

// TestGoldenCpltArgvWithCpltArgs pins the assembled vector when cpltArgs is
// non-empty: the sandbox flags belong *before* the "--" separator, where cplt
// reads them, and the client arguments after it. Without this the separator
// could drift past the sandbox grant and hand --allow-read to the client.
func TestGoldenCpltArgvWithCpltArgs(t *testing.T) {
	spec := cpltLaunch{
		agent:     "opencode",
		cpltArgs:  []string{"--allow-read", "/staged/x", "--pass-env", "OPENCODE_CONFIG_DIR"},
		agentArgs: []string{"--agent", "grillmester", "-p", "hei"},
	}
	want := []string{
		"--agent", "opencode",
		"--allow-read", "/staged/x",
		"--pass-env", "OPENCODE_CONFIG_DIR",
		"--",
		"--agent", "grillmester", "-p", "hei",
	}
	if got := cpltArgv(spec); !slices.Equal(got, want) {
		t.Errorf("cpltArgv\n got: %q\nwant: %q", got, want)
	}

	// And with no agent arguments the separator is still the last element, so
	// the sandbox flags cannot slide behind it.
	copilot := cpltLaunch{agent: "copilot", cpltArgs: []string{"--allow-read", "/staged/x"}}
	wantCopilot := []string{"--agent", "copilot", "--allow-read", "/staged/x", "--"}
	if got := cpltArgv(copilot); !slices.Equal(got, wantCopilot) {
		t.Errorf("cpltArgv(copilot)\n got: %q\nwant: %q", got, wantCopilot)
	}
}

// TestStagedCopilotDoesNotInjectUserInstructions pins that a staged Tier 2
// session is not silently given the user's own ~/.copilot content: the pakke
// author never tested against it, and no manifest field declares that the pakke
// accepts it (see pakkeAcceptsUserContext). The legacy launch still injects it.
func TestStagedCopilotDoesNotInjectUserInstructions(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })

	home := t.TempDir()
	instructions := filepath.Join(home, ".copilot", ".github", "instructions")
	if err := os.MkdirAll(instructions, 0o755); err != nil {
		t.Fatalf("creating user instructions: %v", err)
	}
	if err := os.WriteFile(filepath.Join(instructions, "nav.instructions.md"), []byte("# nav\n"), 0o600); err != nil {
		t.Fatalf("writing user instruction: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_CUSTOM_INSTRUCTIONS_DIRS", "")

	const key = "COPILOT_CUSTOM_INSTRUCTIONS_DIRS"
	if got := telemetry.LookupEnvValue(CopilotEnv(""), key); got == "" {
		t.Fatalf("precondition: the legacy copilot launch should inject %s for this HOME", key)
	}

	staged := StagedLaunch{Dir: t.TempDir(), PakkeName: "grillmester", Context: "full"}
	spec := buildStagedSpec(t, "copilot", domain.ResolvedConfig{Client: "copilot", AskUser: true}, staged)
	if got := telemetry.LookupEnvValue(spec.env, key); got != "" {
		t.Errorf("staged copilot launch injected %s = %q; a pakke gets the user's own context only when it declares that it accepts it", key, got)
	}
}

// TestStagedLaunchRejectsReservedClientArguments pins the guard transcribed
// from the reference launcher's _reject_reserved_arguments (grillmester.py
// lines 633-643 at 3573b93cc8b7568516117263562d073cae9ee7fc):
//
//	line 633-636  client --agent        both clients
//	line 637-640  client --project-dir  both clients
//	line 641-643  client --plugin-dir   copilot only
//
// These select what the session runs, which on a Tier 2 launch is fixed by the
// digest-verified payload. Without the guard,
// `nav-pilot --client copilot -- --plugin-dir /tmp/unverified` appends an
// unverified plugin directory to a verified session.
//
// Both spellings the reference accepts are covered: "--opt value" and
// "--opt=value" (_contains_option, lines 602-603).
func TestStagedLaunchRejectsReservedClientArguments(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })

	staged := StagedLaunch{Dir: t.TempDir(), PakkeName: "grillmester", Context: "full"}

	tests := []struct {
		name      string
		client    string
		extraArgs []string
		wantErr   string
	}{
		{name: "copilot --agent", client: "copilot", extraArgs: []string{"--agent", "other"}, wantErr: "--agent"},
		{name: "copilot --agent=", client: "copilot", extraArgs: []string{"--agent=other"}, wantErr: "--agent"},
		{name: "copilot --project-dir", client: "copilot", extraArgs: []string{"--project-dir", "/tmp"}, wantErr: "--project-dir"},
		{name: "copilot --plugin-dir", client: "copilot", extraArgs: []string{"--plugin-dir", "/tmp/unverified"}, wantErr: "--plugin-dir"},
		{name: "copilot --plugin-dir=", client: "copilot", extraArgs: []string{"--plugin-dir=/tmp/unverified"}, wantErr: "--plugin-dir"},
		{name: "opencode --agent", client: "opencode", extraArgs: []string{"--agent", "other"}, wantErr: "--agent"},
		{name: "opencode --project-dir", client: "opencode", extraArgs: []string{"--project-dir", "/tmp"}, wantErr: "--project-dir"},
		// Even behind an opencode subcommand, which openCodeClientArgs
		// forwards unchanged: the reference rejects before it dispatches.
		{name: "opencode run --agent", client: "opencode", extraArgs: []string{"run", "--agent", "other"}, wantErr: "--agent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := domain.ResolvedConfig{Client: tt.client, ExtraArgs: tt.extraArgs}
			var err error
			if tt.client == "opencode" {
				_, err = buildStagedOpenCodeSpec(r, staged)
			} else {
				_, err = buildStagedCopilotSpec(r, staged)
			}
			if err == nil {
				t.Fatalf("%v must be refused, not silently accepted", tt.extraArgs)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error should name %s, got: %v", tt.wantErr, err)
			}
		})
	}
}

// TestOpenCodeAcceptsPluginDir: --plugin-dir is a copilot flag, so the
// per-client rule of the reference (line 641, `client == "copilot"`) must not
// be flattened into a single list for both clients.
func TestOpenCodeAcceptsPluginDir(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })

	staged := StagedLaunch{Dir: t.TempDir(), PakkeName: "grillmester", Context: "full"}
	spec, err := buildStagedOpenCodeSpec(
		domain.ResolvedConfig{Client: "opencode", ExtraArgs: []string{"--plugin-dir", "/tmp/x"}}, staged)
	if err != nil {
		t.Fatalf("opencode --plugin-dir is not reserved: %v", err)
	}
	if !slices.Contains(spec.agentArgs, "--plugin-dir") {
		t.Errorf("forwarded argument dropped: %v", spec.agentArgs)
	}
}
