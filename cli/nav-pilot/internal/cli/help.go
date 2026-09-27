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
	"survey": `Usage: nav-pilot survey [--json]

Answer an open user survey. In a terminal it opens the survey (or asks which,
if there are several). Without a terminal, or with --json, it lists the open
surveys.

It works even with automatic survey prompts turned off (surveys = false,
DO_NOT_TRACK) and after you said never: you asked for it. Answering needs a
GitHub sign-in (nav-pilot auth login), and each person can answer once.

Flags:
  --json   List the open surveys as JSON
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
	"doctor": `Usage: nav-pilot doctor

Check this machine and say what to fix: the config file, what is installed in
~/.copilot and this repository, hooks, the clients (copilot, opencode, pi) and
cplt, model pins, the cplt sandbox and git. Each problem comes with the command
that fixes it. Reads only; changes nothing.

Exits 0 even when it finds problems; read the Solution lines.
`,
	"env": `Usage: nav-pilot env

Print the shell export that makes the Copilot CLI read the instructions installed
in ~/.copilot. Add it to your shell profile:

  eval "$(nav-pilot env)"

Until instructions are installed (nav-pilot install --user) it prints nothing
on stdout.
`,
	"init": `Usage: nav-pilot init [flags]

Create starter files in this git repository: AGENTS.md,
.github/copilot-instructions.md and .github/copilot-review-instructions.md,
filled in from the stack it detects. Existing files are left alone.

Flags:
  -t, --target <dir>      Another repository instead of this one
  -f, --force             Overwrite existing files
  -n, --dry-run           Show what would be created
`,
	"export": `Usage: nav-pilot export opencode [flags]

Write the agentpakke's agents, skills and instructions to .opencode/ in this
repository, in opencode's format, so they can be committed. You do not need
this to use opencode: nav-pilot --client opencode sets up its own context.

Flags:
  -t, --target <dir>      Another repository instead of this one
  -s, --source <repo>     Export from another agentpakke
  -r, --ref <ref>         Git branch or tag to export from
  -f, --force             Overwrite an existing .opencode/
  -n, --dry-run           Show what would be written
  --json                  Output results as JSON
`,
	"feedback": `Usage: nav-pilot feedback [--feature]

Open a new GitHub issue in navikt/copilot in your browser, with version and
system details filled in. When no browser opens it prints the URL. Nothing is
sent until you submit the issue.

Flags:
  -F, --feature           A feature request instead of a bug report
`,
	"validate": `Usage: nav-pilot validate [--source <repo>|<path>] [--ref <ref>] [--json]

Check an agentpakke repository against the contract: manifest, layout and
names. Without --source it checks the default source. To check your own config
file, use nav-pilot config validate.

Flags:
  -s, --source <repo>     Repository (owner/name) or absolute path to check
  -r, --ref <ref>         Git branch or tag
  --json                  Output results as JSON

Exit codes: 0 valid, 1 violations found.
`,
	"ignore": `Usage: nav-pilot ignore <type> <name> --user

Stop sync from offering an item that is new in the agentpakke. Types: agent,
skill, instruction, hook. User scope only.

Flags:
  -u, --user              Required: the user scope (~/.copilot)
  --json                  Output results as JSON

Example:
  nav-pilot ignore instruction nextjs-aksel --user
`,
	"add": `Usage: nav-pilot add <type> <name> [flags]

Deprecated: use nav-pilot install <name> --type <type>. Takes the same flags
as install (nav-pilot help install).
`,
	"auth": `Usage: nav-pilot auth <login|status|logout> [--json]

Sign in with GitHub so nav-pilot usage can look up your Copilot usage.

Subcommands:
  login                   Sign in with the GitHub device flow; the token goes in the OS keychain
  status [--json]         Show whether you are signed in, and as whom
  logout                  Remove the token from the keychain
`,
	"usage": `Usage: nav-pilot usage [--json|--tmux]

Show your GitHub Copilot usage. Needs nav-pilot auth login first.

Flags:
  --json                  Output the usage as JSON
  --tmux                  One compact line, for a tmux status bar
`,
	"version": `Usage: nav-pilot version [--json]

Print the version, commit and build date.
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

// localHelp is the page `nav-pilot alpha local <cmd> --help` prints.
var localHelp = map[string]string{
	"init": `Usage: nav-pilot alpha local init [--yes]

Set up local inference: the Python environment, the model's weights, the
wired-memory limit and a running server. Says how much it downloads and asks
before it downloads or runs sudo. With local_endpoint set, it checks that
server instead and downloads nothing.

Flags:
  --yes                   Answer yes, for a script

Exit codes: 0 done, 1 failed, 2 no terminal to ask and no --yes.
`,
	"start": `Usage: nav-pilot alpha local start

Start the local server on local_model and wait until it answers a real
completion. Asks before raising the wired-memory limit with sudo; without a
terminal it prints the command instead.
`,
	"restart": `Usage: nav-pilot alpha local restart

Stop the server and start it again on local_model. Run it after
nav-pilot alpha local use <key>.
`,
	"stop": `Usage: nav-pilot alpha local stop

Stop the local server. Weights and settings stay.
`,
	"status": `Usage: nav-pilot alpha local status

Show the model, whether the server answers, its resident memory, the
wired-memory limit, the log file and what it has done.
`,
	"models": `Usage: nav-pilot alpha local models

List the local models on offer: size, context, whether each is downloaded or
running, and which one is in use.
`,
	"use": `Usage: nav-pilot alpha local use <key|model-id>

Pick the model the server loads (sets local_model). Then run
nav-pilot alpha local init to download it, or nav-pilot alpha local restart if
it is already downloaded.

Exit codes: 0 set, 1 unknown model, 2 no model named.
`,
	"ask": `Usage: nav-pilot alpha local ask -p "<question>"
       echo "<question>" | nav-pilot alpha local ask

Put one question to the running local model and print its answer, with token
counts and time. Needs a running server: nav-pilot alpha local start.

Flags:
  -p, --prompt <text>     The question; plain words after ask, or stdin, also work
`,
	"on": `Usage: nav-pilot alpha local on

Turn dispatch back on after off: nav-pilot hands sessions or tasks to the
local model again. Downloads nothing.
`,
	"off": `Usage: nav-pilot alpha local off

Stop dispatching to the local model: sessions go to the hosted model. The
weights stay on disk; nav-pilot alpha local on turns it back on.
`,
	"purge": `Usage: nav-pilot alpha local purge [--yes] [--all]

Remove the Python environment and the chosen model's weights. Without --yes it
lists what it would remove and how big, and removes nothing. Stop the server
first.

Flags:
  --yes                   Remove without asking
  --all                   Every downloaded model's weights, not only the chosen one
`,
	"setup": `Usage: nav-pilot alpha local setup [flags]

Use a server you run yourself (Ollama, llama-server, LM Studio): finds it,
picks a model, fixes Ollama's context, checks it and saves it as
local_endpoint. Asks first.

Flags:
  --endpoint <url>        The server's OpenAI-compatible URL, instead of searching
  --model <id>            The model to use on it
  --pull                  Pull the recommended Ollama model
  --fix-context           Raise Ollama's context window
  --yes                   Save without asking; needed without a terminal

Exit codes: 0 saved; 1 a check failed or no server found; 2 bad flags, or no
terminal and no --yes.
`,
	"doctor": `Usage: nav-pilot alpha local doctor

Check your own server (local_endpoint): tool calls, logprobs, context length
and time to first token. The context check can take minutes on a CPU.

Exit codes: 0 no FAIL lines, 1 a check failed or local_endpoint is not set.
`,
}

// alphaHelp prints the page for the alpha command args name (the words after
// alpha), or the alpha overview when it has none.
func alphaHelp(w io.Writer, args []string) {
	if len(args) > 0 && args[0] == "decide" {
		fmt.Fprint(w, decideHelp)
		return
	}
	if len(args) > 1 && args[0] == "local" {
		if page, ok := localHelp[args[1]]; ok {
			fmt.Fprint(w, page)
			return
		}
	}
	alphaUsage(w)
}

// wantsHelp reports whether args, the words after alpha local sub, ask for
// help. For ask the words are the question, so only a leading -h/--help is
// help: "ask what does -h mean" asks. A -h after -p is the question too.
func wantsHelp(sub string, args []string) bool {
	if sub == "ask" && len(args) > 1 {
		args = args[:1]
	}
	for i, a := range args {
		if (a == "-h" || a == "--help") && (i == 0 || (args[i-1] != "-p" && args[i-1] != "--prompt")) {
			return true
		}
	}
	return false
}
