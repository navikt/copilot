package cli

import (
	"strings"
	"testing"
)

// ─── launchPi ─────────────────────────────────────────────────────────────────

func TestLaunchPi_ReturnsErrorWhenPiNotInPath(t *testing.T) {
	// Empty PATH so exec.LookPath("pi") fails.
	t.Setenv("PATH", t.TempDir())
	err := launchPi(ResolvedConfig{Client: "pi"})
	if err == nil {
		t.Fatal("launchPi() must return a non-nil error when pi is not in PATH")
	}
}

func TestLaunchPi_ErrorMentionsPi(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := launchPi(ResolvedConfig{Client: "pi"})
	if err == nil {
		t.Fatal("launchPi() must return a non-nil error when pi is not in PATH")
	}
	if !strings.Contains(err.Error(), "pi") {
		t.Errorf("launchPi() error should mention 'pi', got: %v", err)
	}
}

// ─── launchClient ─────────────────────────────────────────────────────────────

func TestLaunchClient_Pi(t *testing.T) {
	// Empty PATH so pi is not resolvable and launch fails deterministically.
	t.Setenv("PATH", t.TempDir())
	err := launchClient(ResolvedConfig{Client: "pi"})
	if err == nil {
		t.Fatal("launchClient({Client:\"pi\"}) must return a non-nil error when pi is not in PATH")
	}
}

func TestLaunchClient_OpenCodeNotInPath(t *testing.T) {
	// Point PATH to an empty temp dir so exec.LookPath("opencode") fails.
	t.Setenv("PATH", t.TempDir())

	err := launchClient(ResolvedConfig{Client: "opencode", Mode: "default"})
	if err == nil {
		t.Fatal("launchClient(opencode) must return error when opencode is not in PATH")
	}
	if !strings.Contains(err.Error(), "opencode") {
		t.Errorf("expected 'opencode' in error message, got: %v", err)
	}
}
