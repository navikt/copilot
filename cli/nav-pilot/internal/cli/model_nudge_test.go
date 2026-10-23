package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// nudgeFixture is a staged payload whose primary agent pins pin, a config
// with model = user (none when ""), and a terminal that answers answer.
func nudgeFixture(t *testing.T, user string) (stagedDir string, asked *int, answer *string) {
	t.Helper()
	cfg := "version = 1\n"
	if user != "" {
		cfg += "model = \"" + user + "\"\n"
	}
	writeTestConfig(t, cfg)
	providerpkg.SetActivePakke(&agentpakke.Manifest{ContractVersion: "1", Name: "grill", Clients: map[string]agentpakke.ClientEntry{"copilot": {PrimaryAgents: []string{"grillmester"}}}})
	t.Cleanup(func() { providerpkg.SetActivePakke(nil) })
	stagedDir = t.TempDir()
	setPin(t, stagedDir, "GPT-6 Sol")

	prevI, prevAsk, prevP := isInteractive, askModelNudge, sessionPrompted
	t.Cleanup(func() { isInteractive, askModelNudge, sessionPrompted = prevI, prevAsk, prevP })
	isInteractive = func() bool { return true }
	n, a := 0, "keep"
	askModelNudge = func(_ string, _ []huh.Option[string], choice *string) error {
		n++
		*choice = a
		return nil
	}
	return stagedDir, &n, &a
}

func setPin(t *testing.T, dir, pin string) {
	t.Helper()
	f := filepath.Join(dir, "agents", "grillmester.agent.md")
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte("---\nname: grillmester\nmodel: "+pin+"\n---\nGrill.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runModelNudge(dir, model string) ResolvedConfig {
	sessionPrompted = false
	return maybeModelNudge(ResolvedConfig{Client: "copilot", Model: model}, "grillmester", dir)
}

func TestModelNudgeOncePerRecommendation(t *testing.T) {
	dir, asked, _ := nudgeFixture(t, "gpt-5.5")
	runModelNudge(dir, "gpt-5.5")
	runModelNudge(dir, "gpt-5.5")
	if *asked != 1 {
		t.Fatalf("asked %d times for one recommendation, want 1", *asked)
	}
	setPin(t, dir, "gpt-6.1-sol")
	runModelNudge(dir, "gpt-5.5")
	if *asked != 2 {
		t.Errorf("a new recommendation must be asked about: asked %d, want 2", *asked)
	}
}

func TestModelNudgeNotWithoutDifferentUserModel(t *testing.T) {
	dir, asked, _ := nudgeFixture(t, "")
	runModelNudge(dir, "")
	if *asked != 0 {
		t.Errorf("no user model: asked %d", *asked)
	}

	dir, asked, _ = nudgeFixture(t, "gpt-6-sol")
	runModelNudge(dir, "gpt-6-sol")
	if *asked != 0 {
		t.Errorf("user model equals the recommendation: asked %d", *asked)
	}
}

func TestModelNudgeAnswers(t *testing.T) {
	dir, _, answer := nudgeFixture(t, "gpt-5.5")
	*answer = "switch"
	if got := runModelNudge(dir, "gpt-5.5"); got.Model != "gpt-6-sol" {
		t.Errorf("switch: this launch runs %q, want gpt-6-sol", got.Model)
	}
	if cfg, _ := readConfig(); cfg.Model == nil || *cfg.Model != "gpt-6-sol" {
		t.Errorf("switch must set model in config, got %v", cfg.Model)
	}

	setPin(t, dir, "gpt-6.1-sol")
	*answer = "follow"
	if got := runModelNudge(dir, "gpt-6-sol"); got.Model != "" {
		t.Errorf("follow: this launch runs %q, want the pakke's", got.Model)
	}
	if cfg, _ := readConfig(); cfg.Model != nil {
		t.Errorf("follow must unset model, got %q", *cfg.Model)
	}
}

// --model on this run is the user's choice for it, even when it equals the
// saved model: not questioned, config untouched.
func TestModelNudgeNotForModelFlag(t *testing.T) {
	dir, asked, _ := nudgeFixture(t, "gpt-5.5")
	sessionPrompted = false
	maybeModelNudge(ResolvedConfig{Client: "copilot", Model: "gpt-5.5", ModelFromFlag: true}, "grillmester", dir)
	if *asked != 0 {
		t.Errorf("asked %d times for an explicit --model", *asked)
	}
}

// No agent file: nothing is known to recommend.
func TestModelNudgeNoAgentFile(t *testing.T) {
	_, asked, _ := nudgeFixture(t, "gpt-5.5")
	providerpkg.SetActivePakke(&agentpakke.Manifest{ContractVersion: "1", Name: "grill", Clients: map[string]agentpakke.ClientEntry{"copilot": {PrimaryAgents: []string{"grillmester"}, DefaultModel: "gpt-6-sol"}}})
	runModelNudge(t.TempDir(), "gpt-5.5")
	if *asked != 0 {
		t.Errorf("asked %d times without an agent file", *asked)
	}
}

// pi has no agent files: the pakke's defaultModel is the recommendation
// (#1931). The control, the same model, is not asked about.
func TestModelNudgePiDefaultModel(t *testing.T) {
	for _, tc := range []struct {
		user string
		want int
	}{{"gpt-5.5", 1}, {"gpt-6-sol", 0}} {
		_, asked, _ := nudgeFixture(t, tc.user)
		providerpkg.SetActivePakke(&agentpakke.Manifest{ContractVersion: "1", Name: "grill", Clients: map[string]agentpakke.ClientEntry{"pi": {DefaultModel: "gpt-6-sol"}}})
		sessionPrompted = false
		maybeModelNudge(ResolvedConfig{Client: "pi", Model: tc.user}, "", "")
		if *asked != tc.want {
			t.Errorf("pi, model %s: asked %d, want %d", tc.user, *asked, tc.want)
		}
	}
	// The key "@pi" is a safe name that no agent file answers to.
	if !providerpkg.SafePersona("@pi") || providerpkg.PrimaryAgentFile("pi", "@pi", "", "") != "" {
		t.Error(`"@pi" must pass SafePersona and name no agent file`)
	}
}

// pi and another client with the same persona but different recommendations
// keep separate records: after Keep, alternating asks neither again.
func TestModelNudgePiKeyApart(t *testing.T) {
	dir, asked, _ := nudgeFixture(t, "gpt-5.5")
	providerpkg.SetActivePakke(&agentpakke.Manifest{ContractVersion: "1", Name: "grill", Clients: map[string]agentpakke.ClientEntry{
		"copilot": {PrimaryAgents: []string{"grillmester"}},
		"pi":      {PrimaryAgents: []string{"grillmester"}, DefaultModel: "gpt-6.1-sol"},
	}})
	for range 3 {
		sessionPrompted = false
		maybeModelNudge(ResolvedConfig{Client: "copilot", Model: "gpt-5.5"}, "grillmester", dir)
		sessionPrompted = false
		maybeModelNudge(ResolvedConfig{Client: "pi", Model: "gpt-5.5"}, "grillmester", "")
	}
	if *asked != 2 {
		t.Errorf("asked %d times alternating pi and copilot, want 2 (once each)", *asked)
	}
}

// A persona that is not a plain file name is never used in a path or key.
func TestModelNudgeUnsafePersona(t *testing.T) {
	dir, asked, _ := nudgeFixture(t, "gpt-5.5")
	for _, p := range []string{"../grillmester", ".grillmester", "a/grillmester"} {
		sessionPrompted = false
		maybeModelNudge(ResolvedConfig{Client: "copilot", Model: "gpt-5.5"}, p, dir)
	}
	if *asked != 0 {
		t.Errorf("asked %d times for unsafe personas", *asked)
	}
}

func TestModelNudgeNoTerminalNotice(t *testing.T) {
	dir, asked, _ := nudgeFixture(t, "gpt-5.5")
	isInteractive = func() bool { return false }
	out := captureStderr(func() { runModelNudge(dir, "gpt-5.5") })
	if *asked != 0 || !strings.Contains(out, "nav-pilot config set model gpt-6-sol") || !strings.Contains(out, "nav-pilot config unset model") || strings.Count(out, "\n") != 1 {
		t.Errorf("want one line naming both commands, asked %d:\n%s", *asked, out)
	}
	if out := captureStderr(func() { runModelNudge(dir, "gpt-5.5") }); out != "" {
		t.Errorf("said twice:\n%s", out)
	}
}
