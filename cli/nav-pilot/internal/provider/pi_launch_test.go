package provider

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// TestPiAcceptsTheFlagsWeSend runs the real pi binary and checks it accepts the
// flags nav-pilot passes. Skipped when pi is absent, which is every machine that
// has not installed it; CI installs it so a flag rename upstream fails here
// rather than in a user's session.
//
// It cannot verify a launch: that needs cplt and a model credential. What it
// does verify is the one thing most likely to break silently — that --skill,
// --append-system-prompt and --model still exist and still take these shapes.
func TestPiAcceptsTheFlagsWeSend(t *testing.T) {
	if _, err := exec.LookPath("pi"); err != nil {
		t.Skip("pi not installed; this check exists for CI, where it is")
	}

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "skills", "s"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "s", "SKILL.md"), []byte("# s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	persona := filepath.Join(dir, "agents", "a.md")
	if err := os.WriteFile(persona, []byte("be terse\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	args := append(piSkillArgs(dir, "a"), piModelArg("claude-sonnet-4.6")...)
	args = append(args, "--print", "say ok")

	out, err := exec.Command("pi", args...).CombinedOutput()
	// A missing credential or a refused model is fine: pi got past its parser.
	// An unknown or malformed flag is not, and that is what this catches.
	for _, bad := range []string{"unknown option", "unknown argument", "unrecognized", "Unknown flag"} {
		if strings.Contains(strings.ToLower(string(out)), strings.ToLower(bad)) {
			t.Fatalf("pi rejected a flag nav-pilot sends (%v): %s\nargs: %v", err, out, args)
		}
	}
}

// TestPiLaunchArgsModel pins the Tier 1 model resolution: a user pin wins, and
// an unpinned launch falls back to the active agentpakke's defaultModel — the
// model ResolvedModelNotice has always announced, and which the launch used to
// drop, leaving pi on its own default.
func TestPiLaunchArgsModel(t *testing.T) {
	SetActivePakke(&agentpakke.Manifest{
		Name:    "p",
		Clients: map[string]agentpakke.ClientEntry{"pi": {PrimaryAgents: []string{"a"}, DefaultModel: "claude-opus-5"}},
	})
	t.Cleanup(func() { SetActivePakke(nil) })

	// An empty context dir: no skills, no persona file, no AGENTS.md, so the
	// vector is the model and nothing else.
	dir := t.TempDir()

	t.Run("the pakke default is applied and matches the notice", func(t *testing.T) {
		r := domain.ResolvedConfig{}
		want := []string{"--model", ToOpenCodeModel("claude-opus-5")}
		if got := piLaunchArgs(dir, "a", r); !slices.Equal(got, want) {
			t.Errorf("piLaunchArgs\n got: %q\nwant: %q", got, want)
		}
		if notice := ResolvedModelNotice("pi", r); !strings.Contains(notice, "claude-opus-5") {
			t.Errorf("notice %q should name the model the launch passes", notice)
		}
	})

	t.Run("a user pin wins", func(t *testing.T) {
		want := []string{"--model", ToOpenCodeModel("claude-sonnet-4.6")}
		if got := piLaunchArgs(dir, "a", domain.ResolvedConfig{Model: "claude-sonnet-4.6"}); !slices.Equal(got, want) {
			t.Errorf("piLaunchArgs\n got: %q\nwant: %q", got, want)
		}
	})

	t.Run("extra args come last", func(t *testing.T) {
		want := []string{"--model", ToOpenCodeModel("claude-opus-5"), "--print", "hi"}
		got := piLaunchArgs(dir, "a", domain.ResolvedConfig{ExtraArgs: []string{"--print", "hi"}})
		if !slices.Equal(got, want) {
			t.Errorf("piLaunchArgs\n got: %q\nwant: %q", got, want)
		}
	})
}

func TestPiModelArgOmitsNormalizedAutoModels(t *testing.T) {
	for _, model := range []string{"", "auto", "github-copilot/auto"} {
		t.Run(model, func(t *testing.T) {
			if got := piModelArg(model); got != nil {
				t.Errorf("piModelArg(%q) = %q, want no --model argument", model, got)
			}
		})
	}
}

func TestPiModelArgDoesNotInheritOpenCodeDefault(t *testing.T) {
	SetActivePakke(&agentpakke.Manifest{
		Name: "p",
		Clients: map[string]agentpakke.ClientEntry{
			"opencode": {DefaultModel: "claude-opus-5"},
			"pi":       {DefaultModel: agentpakke.InheritModel},
		},
	})
	t.Cleanup(func() { SetActivePakke(nil) })

	if got := piModelArg(""); got != nil {
		t.Errorf("piModelArg(\"\") = %q, want no --model argument", got)
	}
}

// TestPiSkillArgsPersonaFilenames: both spellings of an agent file reach
// piSkillArgs — Tier 1 materialization renames agents to <name>.md, a staged
// payload keeps the canonical <name>.agent.md.
func TestPiSkillArgsPersonaFilenames(t *testing.T) {
	for _, name := range []string{"a.agent.md", "a.md"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			persona := filepath.Join(dir, "agents", name)
			mustWrite(t, persona, "be terse\n")

			want := []string{"--append-system-prompt", persona}
			if got := piSkillArgs(dir, "a"); !slices.Equal(got, want) {
				t.Errorf("piSkillArgs\n got: %q\nwant: %q", got, want)
			}
		})
	}
}
