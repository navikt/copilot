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
