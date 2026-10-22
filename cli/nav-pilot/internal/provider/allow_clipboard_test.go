package provider

import (
	"slices"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// --allow-clipboard is a cplt flag, so it has to land before the "--" on both
// seams: after it the client gets a flag it does not know and exits.
func TestAllowClipboardBothSeams(t *testing.T) {
	AllowClipboard = true
	t.Cleanup(func() { AllowClipboard = false })

	got := cpltArgv(cpltLaunch{agent: "opencode", agentArgs: []string{"--agent", "x"}})
	if want := []string{"--agent", "opencode", "--allow-clipboard", "--", "--agent", "x"}; !slices.Equal(got, want) {
		t.Errorf("cpltArgv\n got: %q\nwant: %q", got, want)
	}

	got = copilotLaunchArgs("cplt", domain.ResolvedConfig{AskUser: true}, false, "")
	if i, j := slices.Index(got, "--allow-clipboard"), slices.Index(got, "--"); i < 0 || i > j {
		t.Errorf("copilot launch: --allow-clipboard missing or after --: %q", got)
	}
}

// --cplt values follow --allow-clipboard, in the order given.
func TestCpltArgsSessionFlags(t *testing.T) {
	AllowClipboard, CpltArgs = true, []string{"--allow-port=8080", "--allow-env-files"}
	t.Cleanup(func() { AllowClipboard, CpltArgs = false, nil })

	got := cpltArgv(cpltLaunch{agent: "pi"})
	want := []string{"--agent", "pi", "--allow-clipboard", "--allow-port=8080", "--allow-env-files", "--"}
	if !slices.Equal(got, want) {
		t.Errorf("\n got: %q\nwant: %q", got, want)
	}
}
