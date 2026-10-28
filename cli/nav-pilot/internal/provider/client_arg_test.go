package provider

import (
	"slices"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// --client-arg values reach the client after nav-pilot's own flags, so a
// client that takes the last value lets the user's --model win, and they come
// before the arguments given after --. ExtraArgs is what the CLI resolves
// them to: --client-arg values first, then those after --.
func TestClientArgsReachCpltArgv(t *testing.T) {
	user := []string{"--model", "user-model", "--foo", "-p", "hi"}
	r := domain.ResolvedConfig{Client: "copilot", Mode: "default", AskUser: true, Model: "nav-model", ExtraArgs: user}
	dir := t.TempDir()
	for name, agentArgs := range map[string][]string{
		"copilot":  BuildCopilotArgs("cplt", r),
		"opencode": openCodeAgentArgs(r),
		"pi":       piLaunchArgs(dir, "a", r),
	} {
		argv := cpltArgv(cpltLaunch{agent: name, agentArgs: agentArgs})
		if !slices.Equal(argv[len(argv)-len(user):], user) {
			t.Errorf("%s: argv %q does not end with %q", name, argv, user)
		}
		// OpenCode gets the model through its config, not argv.
		if n := countOf(argv, "--model"); name != "opencode" && n != 2 {
			t.Errorf("%s: argv %q has %d --model, want nav-pilot's then the user's", name, argv, n)
		}
	}
}

func countOf(s []string, v string) (n int) {
	for _, x := range s {
		if x == v {
			n++
		}
	}
	return n
}
