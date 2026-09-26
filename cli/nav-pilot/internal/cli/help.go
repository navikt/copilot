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
Without a terminal it installs a whole agentpakke only with --yes, --all
or --frozen, and says what it would write otherwise (exit 2).

Flags:
  -u, --user              Install to ~/.copilot (agents, skills & instructions)
  --repo                  Install to this repository's .github/
  -t, --target <dir>      Install to another repository
  --all                   Install everything without prompting (with --user or --repo)
  --type <type>           agent, skill, instruction or prompt
  -s, --source <repo>     Install from another agentpakke (owner/name or an absolute path).
                          Sync then keeps using it for this scope; other commands
                          need --source again unless you pass --save-source
  -r, --ref <ref>         Git branch or tag to install from
  --frozen                Install exactly what .nav-pilot/agentpakke.lock.json pins, or fail
  --yes                   Install without asking, also without a terminal
  --save-source           Make --source the default source for later commands
  -f, --force             Overwrite files that differ from the source (yours are saved as .orig)
  -n, --dry-run           Show what would happen
  --json                  Output results as JSON

Examples:
  nav-pilot install nav-pilot --repo
  nav-pilot install --user --all
  nav-pilot install security-champion --type agent
  nav-pilot install plattform --source navikt/plattform --repo
`,
	"sync": `Usage: nav-pilot sync [flags]

Check installed scopes for updates. Without --user, --repo or --target it
checks every scope it finds; --apply installs the updates.

Flags:
  --apply                 Apply available updates. A file changed here since
                          nav-pilot installed it is replaced too, and your copy
                          is saved as <file>.orig
  -u, --user              Only the user scope (~/.copilot)
  --repo                  Only this repository
  -t, --target <dir>      Only another repository
  --updates <mode>        How a pinned agentpakke takes new stable releases: auto, ask or keep
  -s, --source <repo>     Sync from another agentpakke
  -r, --ref <ref>         Git branch or tag to sync to
  -n, --dry-run           Report only, even with --apply (the same as leaving --apply out)
  --yes                   With --apply, do not ask before removing files the source deleted
  --json                  One JSON document on stdout: {"scopes": [...]} when
                          several scopes are synced, one scope's document when
                          --user, --repo or --target names it. A scope that
                          fails has {"scope": ..., "error": ...}

Exit codes: 0 up to date, 1 updates available, 2 sync failed.

To sync and then launch the client, use nav-pilot --sync.
`,
	"uninstall": `Usage: nav-pilot uninstall [flags]

Remove what nav-pilot installed in one scope: this repository's .github/ by
default, or ~/.copilot with --user. It lists everything it removes, the state
file and the lock file included, and in a terminal asks first. A file that
changed since nav-pilot installed it is left in place and named.

Flags:
  -u, --user              Uninstall from ~/.copilot instead of this repository
  -t, --target <dir>      Uninstall from another repository
  -f, --force             Remove files that changed since nav-pilot installed them too
  -n, --dry-run           List what would be removed, and remove nothing
  --yes                   Do not ask, also in a terminal
`,
	"rollback": `Usage: nav-pilot rollback [--json]

Move the agentpakke pinned in your user scope (~/.copilot) back to the
previous revision on this machine. No network, nothing deleted: the revision
it leaves is still there for sync --apply --ref.

A repository's pin is .nav-pilot/agentpakke.lock.json, and it moves with git:
  git log -p -- .nav-pilot/agentpakke.lock.json     # earlier pins
  git checkout <commit> -- .nav-pilot/agentpakke.lock.json
  nav-pilot install <name> --frozen                 # install that pin again

Flags:
  --json                  Output results as JSON
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

Manage %s. With no subcommand in a terminal, it opens
the settings page.

The file is ~/.nav-pilot/config.toml unless NAV_PILOT_CONFIG names another
one. $XDG_CONFIG_HOME is not read.

Subcommands:
  init                    Create the file with every option commented out
  setup [--force]         Run the setup wizard (--force replaces an existing file)
  show [--json]           Print every key with its value and where it comes from
  path [--json]           Print the config file path
  get <key> [--json]      Print one value
  set <key> <value>       Set one value
  unset <key>             Remove a key so its default applies
  validate [--json]       Check syntax, keys and values
  explain [key]           Describe the keys
  sandbox                 Configure the cplt sandbox profile
`,
	"models": `Usage: nav-pilot models [filter...] [flags]

List the models nav-pilot knows for the client: the curated Copilot list, plus
the local models when alpha local is on. The model you have set is marked *.
Words after models filter the list: nav-pilot models claude opus.

The list is nav-pilot's, not GitHub's: what you can use also depends on your
Copilot plan and your organization's policy.

Flags:
  --client <name>         List for copilot, opencode or pi instead of your client
  --json                  Output the list as JSON

Set the model: nav-pilot config set model <id>
`,
	"upgrade": `Usage: nav-pilot upgrade [flags]

Replace this nav-pilot with the latest release, after checking its SHA-256
checksum. It never asks. A Homebrew or apt install is left to its package
manager: upgrade prints the command that updates it instead.

upgrade installs the latest release only. To pin a version, use your package
manager, or download it from ` + releasesPage + `.

Flags:
  -n, --dry-run           Only check: print current → latest and change nothing.
                          Exit 0 when up to date, 1 when an update is available
  -y, --yes               Upgrade without asking (upgrade never asks; for scripts)

With auto_update = true, nav-pilot upgrades itself before other commands. A
failed auto-update warns on stderr, runs the version you have and waits 24
hours before it tries again. Turn it off: nav-pilot config set auto_update false
`,
}

// printHelp prints the page for command, or the top-level page when it has none.
func printHelp(w io.Writer, command string) {
	if page, ok := commandHelp[command]; ok {
		if command == "config" {
			page = fmt.Sprintf(page, configPath())
		}
		fmt.Fprint(w, page)
		return
	}
	usage(w)
}
