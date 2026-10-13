# nav-pilot's hooks in OpenCode

Issue: #1025, part of the parity plan in #1022.

Copilot CLI runs nav-pilot's hooks from `~/.copilot/hooks/` and `.github/hooks/copilot-hooks.json`. An OpenCode session launched by nav-pilot runs the same hooks through one plugin, `internal/provider/hooks-bridge.js` (`hooks-bridge-v2.js` on opencode 2). The plugin runs the same commands with the same JSON payloads and applies the same answers. There is no second implementation of any hook.

| Hook | Copilot CLI | OpenCode (plugin hook) | On failure |
|---|---|---|---|
| Redaction (secrets, fødselsnummer, injection note): `nav-pilot hook redact` | postToolUse | `tool.execute.after` | OpenCode: **fails closed**, output withheld. Copilot: fails open |
| Loop guard: `nav-pilot hook loop-guard` | postToolUse | `tool.execute.after`, before redaction | fails open |
| Gates nav-pilot installed (`ask-first-aria`, `gh-poll-gate`, `klarsprak-gate`, agentpakke hooks) | preToolUse | `tool.execute.before`; a deny is thrown | fails open; a gate with `failClosed` in its sidecar denies (both clients) |

## How a launch wires it

- nav-pilot writes the plugin to `~/.local/share/nav-pilot/opencode-plugin/nav-pilot-hooks.js` (or `$XDG_DATA_HOME/nav-pilot/…`). This directory belongs to nav-pilot. Nothing is written to `~/.config/opencode` for the hooks.
- The launch names the plugin in `OPENCODE_CONFIG_CONTENT` (`"plugin": ["file://…"]`). OpenCode merges that config after the user's and the project's, and appends to any value the user already set. `OPENCODE_CONFIG_DIR` was not used because a Tier 2 launch already points it at the verified payload.
- The hooks to run go to the plugin as JSON in `NAV_PILOT_OPENCODE_HOOKS`:
  - the built-in hooks, with the same `key=value` settings their Copilot entries carry, because the sandbox denies `~/.nav-pilot`;
  - nav-pilot's own `preToolUse` entries, read from the user's Copilot hook files and from the repo's `copilot-hooks.json` (entries without the `navPilot` marker are the user's and are not read).
- Under cplt, the launch passes the plugin's directory as `--allow-read` and the hook state directory (`…/opencode-hook-state`, via `NAV_PILOT_HOOK_STATE_DIR`) as `--allow-write`. When a user-scope gate is installed, `~/.copilot/hooks` is also `--allow-read`. The three variables are passed with `--pass-env`.
- A plain `opencode` started without nav-pilot does not load the plugin.

The plugin maps OpenCode's tools to the names and argument keys Copilot uses, because that is what the gates and their matchers expect:

- `bash` → `bash {command}`
- `edit` → `edit {path, old_str, new_str}`
- `write` → `create {path, file_text}`
- `read` → `view {path}`
- `apply_patch` → `apply_patch {input}`

Any other tool keeps its own name and arguments.

## Why redaction fails closed here

In OpenCode, a plugin hook's answer controls what the model reads. Copilot's postToolUse can only ignore a hook that fails. So the OpenCode bridge withholds the output when redaction cannot run: the binary cannot be started, the hook times out (5 s), the answer is not JSON, or `nav-pilot hook redact` reports an error. The bridge asks for that report with `NAV_PILOT_HOOK_FAIL_CLOSED=1`, since otherwise the hook answers `{}` on every failure. The model is told the output was withheld and to tell the user. How to turn redaction off is kept out of that text, as the loop guard's message keeps its threshold out. The loop guard and the gates fail open, as under Copilot, because a broken guard must not stop work. The exception is a gate whose `<name>.hook.json` sets `failClosed`: its entry carries the flag, and the bridge denies the call when the gate times out, fails or answers something that is not JSON. Under Copilot the installed command itself prints the deny; Copilot still allows the call if the hook process does not start before its own deadline.

The hooks run the nav-pilot binary by absolute path: the one on PATH at launch, or the running binary. Under cplt the session may execute only from a few trees (Homebrew, `/usr/local`, `~/.local/bin`, `~/go/bin` and similar). A nav-pilot outside them cannot start, which would withhold every output, so the launch warns when the binary lives elsewhere.

## Two more hops: thrown errors and interrupted calls

Two kinds of tool text reach the model without `tool.execute.after`:
- the error message of a tool that threw;
- the partial output of a call the user interrupted, which OpenCode keeps in `metadata.output` and sends on the next turn.

The plugin also implements `experimental.chat.messages.transform`, which OpenCode runs over the whole history before every model call and before compaction. It redacts both kinds of text there, fail-closed like the rest, and keeps each answer so a text is checked once per session. A session resumed from a plain `opencode` run keeps the completed outputs it had then; only what nav-pilot's launch saw was redacted. This was verified against opencode 1.18.32: a `read` of a missing file whose path held a GitHub token reached the model with the token masked. The hook is marked experimental in OpenCode, so a release that drops it reopens these two gaps; the version range check (#1027) is where that gets caught.

## Subagents

`tool.execute.before` and `after` fire in every session, a subagent's child session included (verified against opencode 1.18.32: a `general` subagent's `cat` of a secret reached its model redacted). Redaction and the loop guard run there on purpose, because a secret a subagent reads reaches a model the same way. The gates also run there, as they do for Copilot subagents. The loop guard keeps one run per session and skips sessions on the local provider (`mlx`), where the guard proxy already ends the turn.

A gate that answers `ask` is treated as `deny` with its reason. Under Copilot, `ask` asks the human, and a plugin has no one to ask. None of the shipped gates answers `ask`.

## Known gaps

Read in the OpenCode 1.18.32 source (`packages/opencode/src/session/tools.ts`, `session/message-v2.ts`, `session/processor.ts`, `tool/code-mode.ts`):

- **`--pure`** loads no plugins, so no hooks run. nav-pilot says so on stderr at launch.
- **Code mode** (`experimentalCodeMode`): a program sees an MCP result's `structuredContent` and `resource_link` names before redaction. What the program returns is redacted.
- **Attachments** (images, PDFs from `read` or an MCP resource) go to the model as they are. Redaction works on text.
- **The terminal** shows the human the unredacted output (`metadata`), as with Copilot, because redaction changes what the model reads, not the TUI.
- **The hook state directory is writable from the session**, as Copilot's session directory is. An agent could reset its own loop guard, and spooled telemetry lines are enum-checked before they are recorded.
- **The hooks' stderr is discarded**, because inheriting it would draw over the TUI. A tripped loop guard is still counted in telemetry.
- An MCP result with many text items runs the hooks once per item.
- Repo gates run without Copilot's folder-trust check. OpenCode has no such check and already loads a repo's `.opencode/plugins` as code, so a repo's gate entries give it nothing new. With `OPENCODE_DISABLE_PROJECT_CONFIG` set, repo gates are not run either.

## Repo plugins are code, in opencode 1 and 2

OpenCode loads a repo's `.opencode/plugin(s)` and the `plugin` entries in its `opencode.json` as code, before the model sees anything. Under cplt such a plugin can do what the agent can: write in the project, run commands as the user, and reach the network through cplt's proxy. It cannot write outside the project. The hooks protect what the model reads (redaction, gates, the MCP block). They were never the boundary against a repo running its own code. cplt is.

opencode 2 adds one risk. It keeps the first plugin with a given id and drops later ones, and the bridge rides in `OPENCODE_CONFIG_CONTENT`, which loads last. A repo plugin with the bridge's id would win and turn nav-pilot's hooks off. The bridge therefore takes a per-launch random id (`NAV_PILOT_OPENCODE_PLUGIN_ID`, passed with `--pass-env`). A repo plugin that already runs can read that id from its environment and claim it before the bridge loads ([anomalyco/opencode#53721](https://github.com/anomalyco/opencode/issues/53721)). That is a small extra risk on top of the code execution the repo already has, and it is accepted.

For a repo you do not trust, set `OPENCODE_DISABLE_PROJECT_CONFIG=1`. OpenCode then loads no project plugins, MCP servers or config, and nav-pilot runs no repo gates. It also turns off the repo's legitimate `.opencode` config.

opencode 2 launches only:

- on macOS or Linux; elsewhere nav-pilot says so and points to opencode 1;
- on Linux, under cplt 2026.10.08-092800 or newer (navikt/cplt#740), which runs opencode 2 under bubblewrap. cplt refuses a host without `bwrap` and says how to install it. Port isolation needs kernel 6.7 or newer (Landlock ABI 4); on an older kernel cplt warns and the session relies on the service password. Linux has not been tested live with nav-pilot yet;
- on macOS, under cplt 2026.10.08-081501 or newer: the session's service starts inside the sandbox (navikt/cplt#716), cplt reads the config directories opencode 2 discovers (navikt/cplt#720), and Ctrl-C no longer leaves a `serve --service` process behind (navikt/cplt#722). The floor is 2026.10.08-081501 for navikt/cplt#736 (ancestor AGENTS.md grants, and the fix for a credential leak through planted `.agents`, `.claude` or `.opencode` symlinks) and #735 (fails closed on an unreadable opencode version). An opencode whose version nav-pilot cannot read also needs this cplt.

The opencode 2 TUI rejects `-m`/`--model`, so nav-pilot sets the model in the launch's config instead.

nav-pilot refuses opencode 3 and newer until the bridge has been tested against it.

## MCP servers outside Nav's registry (#1027)

A launch turns off every configured MCP server the registry does not list, with `enabled: false`. That only decides how the session starts. OpenCode's `/mcp` dialog can still connect one. The hooks bridge therefore also refuses, in `tool.execute.before`, every tool whose name starts with a turned-off server's name. OpenCode names MCP tools `<server>_<tool>`. The longest matching server name decides, and built-in tool names are never refused.

What the launch does not cover:
- a server added in the middle of a session (`POST /mcp`, the `/mcp` dialog);
- config from a `.well-known/opencode` or console org endpoint the user has signed in to, which OpenCode merges after nav-pilot's;
- macOS managed preferences, which merge last and win.

The launch reads the MCP servers from the same files OpenCode reads, in OpenCode's order: the global file in `~/.config/opencode/`, the file `OPENCODE_CONFIG` points to, `opencode.json` and `.opencode/opencode.json` from the repo root down to the current directory, `~/.opencode/`, `OPENCODE_CONFIG_DIR` and `OPENCODE_CONFIG_CONTENT`. It asks GitHub with the user's gh login (`gh api /copilot/mcp_registry`) which registry the policy points to and fetches the list from there. A remote server must have the registry's URL; a local one must start a package the registry lists (`npx @playwright/mcp`). The check never edits the MCP entries in `opencode.json`. With no MCP servers configured it makes no network call; without `gh`, or when GitHub or the registry does not answer, it turns nothing off and prints a warning.

## What else a launch sets

The user-facing summary is on [the klienter page](https://ki-utvikling.nav.no/nav-pilot/klienter#opencode). The details:

- **Agentpakke cache.** The launch does not wait for GitHub. nav-pilot uses the copy in `~/.nav-pilot/sources/` and refreshes it while the session runs, at most once an hour. Only the first launch waits for the download, up to 30 seconds; if that fails, the next launches within the hour start without the agentpakke and fetch it in the background. `~/.config/opencode/.nav-pilot-state.json` records what nav-pilot installed; a file the user has edited is left alone.
- **Model.** Without a model from the user, nav-pilot uses the agentpakke's default (GPT-6 Sol for agentpakke nav-pilot). If the agentpakke names none, OpenCode chooses. A Copilot model id without a prefix gets `github-copilot/`.
- **`share` and `autoupdate`.** Every launched session gets `"share": "disabled"` and `"autoupdate": "notify"` in the launch config, whatever `opencode.json` says. nav-pilot also writes `"share": "disabled"` into `~/.config/opencode/opencode.json` when the file says nothing about sharing, so it holds for sessions started without nav-pilot. That happens on a Tier 1 launch and in setup; a Tier 2 launch does not touch `opencode.json`. A file that says `"auto"` gets a warning. A file with comments is left alone, since a rewrite would drop them; the settings then hold only for launched sessions, and when something must leave the file (`nav-pilot alpha local off`), nav-pilot says what to remove by hand.
- **`external_directory`.** The instructions, agents and skills nav-pilot installs live in `~/.config/opencode/`, outside the project. A launched session may read them without an `external_directory` prompt, but not edit them; that is `nav-pilot sync`'s job. Without the grant OpenCode would prompt when the model opens one, and `opencode run` would answer no and end the session. Other directories outside the project keep the user's own setting. With `external_directory` set to `"deny"`, nav-pilot adds nothing.
- **Telemetry.** The launch configures OpenTelemetry for OpenCode unless telemetry is turned off.
