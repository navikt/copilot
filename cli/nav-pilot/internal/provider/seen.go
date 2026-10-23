package provider

import (
	"fmt"
	"os"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
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

// CpltSessionFlags is [cpltSessionFlags], for the command nav-pilot prints
// when it does not launch.
func CpltSessionFlags() []string { return cpltSessionFlags() }

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

// warnUnsupported prints a client's unsupported-setting warnings, each once
// per (client, setting, value): a message starts with its setting's name and
// carries the value, so the message is what SeenChanged keeps for that
// setting. A changed value or client says it again; --verbose every time.
func warnUnsupported(client string, msgs []string) {
	for _, msg := range msgs {
		setting, _, _ := strings.Cut(msg, " ")
		if SeenChanged("unsupported-"+client+"-"+setting, msg) || Verbose {
			fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), msg)
		}
	}
}
