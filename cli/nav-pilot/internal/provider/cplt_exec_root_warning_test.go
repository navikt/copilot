package provider

import (
	"path/filepath"
	"strings"
	"testing"
)

// The warning for a nav-pilot the sandbox may not run says what the user sees
// and what to do, in one line (#1962).
func TestCpltExecRootWarningNamesEffectAndFix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	w := cpltExecRootWarning("/srv/checkout/bin/nav-pilot")
	for _, want := range []string{"/srv/checkout/bin", "sees no tool output", "Fix: install nav-pilot in ~/.local/bin"} {
		if !strings.Contains(w, want) {
			t.Errorf("warning %q lacks %q", w, want)
		}
	}
	if strings.Contains(w, "\n") || strings.Contains(w, "redaction withholds") {
		t.Errorf("warning %q: want one line without the mechanism", w)
	}
	// Control: a binary where the sandbox runs programs from gets nothing.
	if w := cpltExecRootWarning(filepath.Join(home, ".local", "bin", "nav-pilot")); w != "" {
		t.Errorf("~/.local/bin: %q, want nothing", w)
	}
}
