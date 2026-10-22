package provider

import "github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"

// Verbose is --verbose on a launch: say what nav-pilot does before the client
// starts (sandbox directory, client, agent). Without it a launch that needs
// nothing from the user prints nothing.
var Verbose bool

// AllowClipboard is --allow-clipboard on a launch: pass cplt's
// --allow-clipboard for this session only, so the agent may read and write
// the macOS clipboard. cplt denies it by default.
var AllowClipboard bool

// cpltSessionFlags are the cplt flags set by launch flags for this session.
func cpltSessionFlags() []string {
	if AllowClipboard {
		return []string{"--allow-clipboard"}
	}
	return nil
}

// SeenChanged is [artifacts.SeenChanged].
func SeenChanged(name, value string) bool { return artifacts.SeenChanged(name, value) }
