// nav-pilot manages Nav's Copilot toolkit — agents, skills, instructions, and prompts.
// It installs curated collections or individual items from navikt/copilot
// and tracks installed state for safe updates, sync, and uninstall.
//
// Usage:
//
//	nav-pilot install <name>               # install a collection or single artifact
//	nav-pilot sync                         # check for updates
//	nav-pilot list                         # list available collections and items
//	nav-pilot list --installed             # show installed state
//	nav-pilot uninstall                    # remove installed files
package main

import "github.com/navikt/copilot/cli/nav-pilot/internal/cli"

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	cli.Main(cli.BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
	})
}
