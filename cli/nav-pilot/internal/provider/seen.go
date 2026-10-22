package provider

import (
	"fmt"
	"os"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
)

// Verbose is --verbose on a launch: say what nav-pilot does before the client
// starts (sandbox directory, client, agent). Without it a launch that needs
// nothing from the user prints nothing.
var Verbose bool

// AllowClipboard is --allow-clipboard on a launch: pass cplt's
// --allow-clipboard for this session only, so the agent may read and write
// the macOS clipboard. cplt denies it by default.
var AllowClipboard bool

// CpltArgs are the values of repeated --cplt on a launch: cplt flags for this
// session only, added to the ones nav-pilot sets.
var CpltArgs []string

// cpltSessionFlags are the cplt flags set by launch flags for this session.
func cpltSessionFlags() []string {
	var flags []string
	if AllowClipboard {
		flags = append(flags, "--allow-clipboard")
	}
	return append(flags, CpltArgs...)
}

// printCpltSessionFlags names the session's extra cplt flags under --verbose,
// so a bug report shows the sandbox the agent really ran in.
func printCpltSessionFlags() {
	if flags := cpltSessionFlags(); len(flags) > 0 {
		fmt.Fprintf(os.Stderr, "Extra cplt flags for this session: %s\n\n", strings.Join(flags, " "))
	}
}

// SeenChanged is [artifacts.SeenChanged].
func SeenChanged(name, value string) bool { return artifacts.SeenChanged(name, value) }
