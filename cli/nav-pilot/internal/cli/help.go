package cli

import (
	"fmt"
	"io"
)

// commandHelp is the page `nav-pilot help <cmd>` and `nav-pilot <cmd> --help`
// print. A command without one gets the top-level page.
var commandHelp = map[string]string{
	"install": `Usage: nav-pilot install <name> [flags]
       nav-pilot install --user --all

Install the agentpakke, or one agent, skill, instruction or prompt from it.
Without --user or --repo it asks where to install when there is a terminal.

Flags:
  -u, --user              Install to ~/.copilot (agents, skills & instructions)
  --repo                  Install to this repository's .github/
  -t, --target <dir>      Install to another repository
  --all                   Install everything without prompting (with --user or --repo)
  --type <type>           agent, skill, instruction or prompt
  -s, --source <repo>     Install from another agentpakke (owner/name or an absolute path)
  -r, --ref <ref>         Git branch or tag to install from
  --frozen                Install exactly what .nav-pilot/agentpakke.lock.json pins, or fail
  -f, --force             Overwrite files that differ from the source
  -n, --dry-run           Show what would happen
  --json                  Output results as JSON

Examples:
  nav-pilot install nav-pilot --repo
  nav-pilot install --user --all
  nav-pilot install security-champion --type agent
`,
	"sync": `Usage: nav-pilot sync [flags]

Check installed scopes for updates. Without --user, --repo or --target it
checks every scope it finds; --apply installs the updates.

Flags:
  --apply                 Apply available updates
  -u, --user              Only the user scope (~/.copilot)
  --repo                  Only this repository
  -t, --target <dir>      Only another repository
  --updates <mode>        How a pinned agentpakke takes new stable releases: auto, ask or keep
  -s, --source <repo>     Sync from another agentpakke
  -r, --ref <ref>         Git branch or tag to sync to
  --json                  Output results as JSON

Exit codes: 0 up to date, 1 updates available, 2 sync failed.

To sync and then launch the client, use nav-pilot --sync.
`,
	"list": `Usage: nav-pilot list [flags]

List the agentpakke and its items, or what is installed.

Flags:
  --installed             Show what is installed (every scope found, or the one named)
  --items                 List every item
  -u, --user              The user scope (~/.copilot)
  --repo                  This repository
  -s, --source <repo>     List another agentpakke
  -r, --ref <ref>         Git branch or tag
  --json                  Output results as JSON
`,
	"config": `Usage: nav-pilot config [subcommand]

Manage ~/.nav-pilot/config.toml. With no subcommand in a terminal, it opens
the settings page.

Subcommands:
  init                    Create the file with every option commented out
  setup [--force]         Run the setup wizard (--force replaces an existing file)
  show                    Print the effective configuration
  path                    Print the config file path
  get <key>               Print one value
  set <key> <value>       Set one value
  validate                Check syntax, keys and values
  explain [key]           Describe the keys
  sandbox                 Configure the cplt sandbox profile
`,
}

// printHelp prints the page for command, or the top-level page when it has none.
func printHelp(w io.Writer, command string) {
	if page, ok := commandHelp[command]; ok {
		fmt.Fprint(w, page)
		return
	}
	usage(w)
}
