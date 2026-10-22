package provider

import (
	"slices"
	"testing"
)

// nav-pilot --resume <id> / --continue reach OpenCode 2 in argv: only --model
// and --agent move into the config (#1919).
func TestOpenCodeV2ArgsKeepSessionFlags(t *testing.T) {
	for _, flags := range [][]string{{"--session", "ses_1"}, {"--continue"}} {
		args, _ := openCodeV2Args(openCodeClientArgs([]string{"--agent", "build"}, flags, ""), nil)
		if !slices.Equal(args[len(args)-len(flags):], flags) {
			t.Errorf("%q: args = %q", flags, args)
		}
	}
}
