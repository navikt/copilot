package provider

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
