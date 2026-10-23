package provider

import (
	"slices"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// --allow-clipboard is a cplt flag, so it has to land before the "--" on both
// seams: after it the client gets a flag it does not know and exits.
func TestAllowClipboardBothSeams(t *testing.T) {
	t.Parallel()
	flags := []string{"--allow-clipboard"}
	got := cpltArgv(cpltLaunch{agent: "opencode", agentArgs: []string{"--agent", "x"}, sessionFlags: flags})
	if want := []string{"--agent", "opencode", "--allow-clipboard", "--", "--agent", "x"}; !slices.Equal(got, want) {
		t.Errorf("cpltArgv\n got: %q\nwant: %q", got, want)
	}

	got = copilotLaunchArgs("cplt", domain.ResolvedConfig{AskUser: true, CpltFlags: flags}, false, "")
	if i, j := slices.Index(got, "--allow-clipboard"), slices.Index(got, "--"); i < 0 || i > j {
		t.Errorf("copilot launch: --allow-clipboard missing or after --: %q", got)
	}
}

// The session flags go last before the "--", in the order resolved.
func TestCpltArgsSessionFlags(t *testing.T) {
	t.Parallel()
	got := cpltArgv(cpltLaunch{agent: "pi", sessionFlags: []string{"--allow-clipboard", "--allow-port=8080", "--allow-env-files"}})
	want := []string{"--agent", "pi", "--allow-clipboard", "--allow-port=8080", "--allow-env-files", "--"}
	if !slices.Equal(got, want) {
		t.Errorf("\n got: %q\nwant: %q", got, want)
	}
}
