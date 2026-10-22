package provider

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

const pinnedAgent = "---\nname: grillmester\ndescription: griller\nmodel: GPT-6 Sol\n---\n\nGrill.\n"

func envValue(env []string, key string) string {
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, key+"="); ok {
			return v
		}
	}
	return ""
}

// The user's model wins over the primary agent's pin on Copilot CLI by
// running a copy of the agent without the pin; with no user model the
// launch is the one it always was.
func TestCopilotUserModelBeatsAgentPin(t *testing.T) {
	home, _ := os.UserHomeDir()
	mustWrite(t, filepath.Join(home, ".copilot", "agents", "grillmester.agent.md"), pinnedAgent)
	project := t.TempDir()

	args := BuildCopilotArgs("cplt", domain.ResolvedConfig{Persona: "grillmester", Model: "gpt-6.1-sol", ProjectDir: project})
	if !slices.Contains(args, copilotSessionPlugin+":grillmester") || !slices.Contains(args, "--plugin-dir") {
		t.Fatalf("user model set: want the session copy of the agent, got %q", args)
	}
	copyFile := filepath.Join(home, ".copilot", copilotSessionPlugin, "agents", "grillmester.agent.md")
	data, err := os.ReadFile(copyFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "model:") || !strings.Contains(string(data), "Grill.") || !strings.Contains(string(data), "description: griller") {
		t.Errorf("the copy must keep everything but the pin:\n%s", data)
	}

	// Control: no user model, the pinned agent runs as it is.
	args = BuildCopilotArgs("cplt", domain.ResolvedConfig{Persona: "grillmester", ProjectDir: project})
	if slices.Contains(args, "--plugin-dir") || !slices.Contains(args, "grillmester") {
		t.Errorf("no user model: want --agent grillmester and no plugin, got %q", args)
	}
}

func TestStagedCopilotUserModelBeatsAgentPin(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "agents", "grillmester.agent.md"), pinnedAgent)
	staged := StagedLaunch{Dir: dir, PakkeName: "grillmester", Context: "full"}

	spec := buildStagedSpec(t, "copilot", domain.ResolvedConfig{Model: "gpt-6.1-sol", AskUser: true}, staged)
	if !slices.Contains(spec.agentArgs, copilotSessionPlugin+":grillmester") {
		t.Errorf("user model set: want the session copy, got %q", spec.agentArgs)
	}
	if i := slices.Index(spec.cpltArgs, filepath.Join(os.Getenv("HOME"), ".copilot", copilotSessionPlugin)); i < 1 || spec.cpltArgs[i-1] != "--allow-read" {
		t.Errorf("cplt must be allowed to read the session plugin: %q", spec.cpltArgs)
	}

	spec = buildStagedSpec(t, "copilot", domain.ResolvedConfig{AskUser: true}, staged)
	if !slices.Contains(spec.agentArgs, "grillmester:grillmester") {
		t.Errorf("no user model: want the pakke's agent, got %q", spec.agentArgs)
	}
}

// opencode: the user's model is set on the launched agent in
// OPENCODE_CONFIG_CONTENT, which both majors merge after the agent files.
func TestOpenCodeUserModelBeatsAgentPin(t *testing.T) {
	env := withUserAgentModel(nil, domain.ResolvedConfig{Model: "gpt-6.1-sol"}, "grillmester")
	if got := envValue(env, openCodeConfigContentEnv); !strings.Contains(got, `"grillmester":{"model":"github-copilot/gpt-6.1-sol"}`) {
		t.Errorf("want agent.grillmester.model = github-copilot/gpt-6.1-sol, got %q", got)
	}
	for _, r := range []domain.ResolvedConfig{{}, {Model: "auto"}} {
		if env := withUserAgentModel(nil, r, "grillmester"); len(env) != 0 {
			t.Errorf("model %q is no user model: want no config, got %q", r.Model, env)
		}
	}
	if env := withUserAgentModel(nil, domain.ResolvedConfig{Model: "gpt-6.1-sol"}, "plan"); len(env) != 0 {
		t.Errorf("the built-in plan agent pins nothing: got %q", env)
	}
}

func TestStagedOpenCodeUserModelBeatsAgentPin(t *testing.T) {
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })
	staged := StagedLaunch{Dir: t.TempDir(), PakkeName: "grillmester", Context: "full"}

	spec := buildStagedSpec(t, "opencode", domain.ResolvedConfig{Model: "gpt-6.1-sol", AskUser: true}, staged)
	if got := envValue(spec.env, openCodeConfigContentEnv); !strings.Contains(got, `"grillmester":{"model":"github-copilot/gpt-6.1-sol"}`) {
		t.Errorf("user model set: want it on the agent, got %q", got)
	}
	spec = buildStagedSpec(t, "opencode", domain.ResolvedConfig{AskUser: true}, staged)
	if got := envValue(spec.env, openCodeConfigContentEnv); strings.Contains(got, `"agent"`) {
		t.Errorf("no user model: want no agent override, got %q", got)
	}
}

// pi has no agent model: the persona is text, and --model carries the
// user's choice as it always did.
func TestPiUserModelIsTheSessionModel(t *testing.T) {
	if got := piModelArg("gpt-6.1-sol"); !slices.Equal(got, []string{"--model", "github-copilot/gpt-6.1-sol"}) {
		t.Errorf("user model: got %q", got)
	}
	if got := piModelArg(""); got != nil {
		t.Errorf("no user model: want no flag, got %q", got)
	}
}

func TestRecommendedModel(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.agent.md")
	mustWrite(t, f, pinnedAgent)
	if got := RecommendedModel("copilot", f); got != "gpt-6-sol" {
		t.Errorf("label pin: got %q, want gpt-6-sol", got)
	}
	mustWrite(t, f, "---\nname: a\nmodel: github-copilot/gpt-6.1-sol\n---\n")
	if got := RecommendedModel("opencode", f); got != "gpt-6.1-sol" {
		t.Errorf("opencode id pin: got %q, want gpt-6.1-sol", got)
	}
}
