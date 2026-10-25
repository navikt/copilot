package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNudgeOnce: a line shows once, a new line shows once, and a line in the
// legacy ~/.nav-pilot/local-models.json.seen list stays shown after upgrade.
func TestNudgeOnce(t *testing.T) {
	setupTestCache(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacy := filepath.Join(home, ".nav-pilot", "local-models.json.seen")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if nudgeOnce("old") {
		t.Error("a line seen before the upgrade showed again")
	}
	if !nudgeOnce("a") || nudgeOnce("a") {
		t.Error("the same line showed twice, or not at all")
	}
	if !nudgeOnce("b") || nudgeOnce("a") || nudgeOnce("b") {
		t.Error("a new line did not show once, or an old one came back")
	}
	if nudgeOnce("") {
		t.Error("an empty line was shown")
	}
}
