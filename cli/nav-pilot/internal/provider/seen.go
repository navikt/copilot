package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// Verbose is --verbose on a launch: say what nav-pilot does before the client
// starts (sandbox directory, client, agent). Without it a launch that needs
// nothing from the user prints nothing.
var Verbose bool

// printCpltSessionFlags names the session's extra cplt flags under --verbose,
// so a bug report shows the sandbox the agent really ran in.
func printCpltSessionFlags(flags []string) {
	if len(flags) > 0 {
		fmt.Fprintf(os.Stderr, "Extra cplt flags for this session: %s\n\n", strings.Join(flags, " "))
	}
}

// SeenChanged is [artifacts.SeenChanged].
func SeenChanged(name, value string) bool { return artifacts.SeenChanged(name, value) }

// FirstTime reports whether the one-time notice name has not been shown on
// this machine yet, and records that it now has been: a marker file named
// seen-<name> beside SeenChanged's markers (nav-pilot's config directory in
// production). When the marker cannot be written it answers false, so a notice
// can go missing but never nags.
func FirstTime(name string) bool {
	p := artifacts.CacheFilePath()
	if p == "" {
		return false
	}
	dir := filepath.Dir(p)
	if os.MkdirAll(dir, 0o700) != nil {
		return false
	}
	f, err := os.OpenFile(filepath.Join(dir, "seen-"+name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	return f.Close() == nil
}

// warnUnsupported prints a client's unsupported-setting warnings once per
// set: each message names its setting and value, so a changed value, setting
// or client says them again. Called with an empty set too, so a warning that
// went away and comes back is said again. --verbose says them every time.
func warnUnsupported(client string, msgs []string) {
	if SeenChanged("unsupported-"+client, strings.Join(msgs, "\n")) || Verbose {
		for _, msg := range msgs {
			fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), msg)
		}
	}
}

// sayConflicts names the context files the user changed and nav-pilot left
// alone: once per set on a launch (reset when the set is empty), every time
// for a command the user ran or with --verbose.
func sayConflicts(key, format string, conflicts []string) {
	changed := SeenChanged(key, strings.Join(conflicts, "\n"))
	if !(changed || Verbose || !artifacts.LaunchNotices) {
		return
	}
	for _, c := range conflicts {
		fmt.Fprintf(os.Stderr, "%s "+format+"\n", domain.Yellow("⚠"), c)
	}
}
