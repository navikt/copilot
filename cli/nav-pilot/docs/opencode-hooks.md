# nav-pilot's hooks in OpenCode

Issue: #1025, part of the parity plan in #1022.

Copilot CLI runs nav-pilot's hooks from `~/.copilot/hooks/` and `.github/hooks/copilot-hooks.json`. An OpenCode session launched by nav-pilot runs the same hooks through one plugin, `internal/provider/hooks-bridge.js`. The plugin runs the same commands with the same JSON payloads and applies the same answers. There is no second implementation of any hook.

| Hook | Copilot CLI | OpenCode (plugin hook) | On failure |
|---|---|---|---|
| Redaction (secrets, fødselsnummer, injection note): `nav-pilot hook redact` | postToolUse | `tool.execute.after` | OpenCode: **fails closed**, output withheld. Copilot: fails open |
| Loop guard: `nav-pilot hook loop-guard` | postToolUse | `tool.execute.after`, before redaction | fails open |
| Gates nav-pilot installed (`ask-first-aria`, `gh-poll-gate`, `klarsprak-gate`, agentpakke hooks) | preToolUse | `tool.execute.before`; a deny is thrown | fails open |

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

In OpenCode, a plugin hook's answer controls what the model reads. Copilot's postToolUse can only ignore a hook that fails. So the OpenCode bridge withholds the output when redaction cannot run: the binary cannot be started, the hook times out (5 s), the answer is not JSON, or `nav-pilot hook redact` reports an error. The bridge asks for that report with `NAV_PILOT_HOOK_FAIL_CLOSED=1`, since otherwise the hook answers `{}` on every failure. The model is told the output was withheld and to tell the user. How to turn redaction off is kept out of that text, as the loop guard's message keeps its threshold out. The loop guard and the gates fail open, as under Copilot, because a broken guard must not stop work.

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

## MCP servers outside Nav's registry (#1027)

A launch turns such servers off with `enabled: false`. That only decides how the session starts: OpenCode's `/mcp` dialog can still connect one. The hooks bridge therefore also refuses, in `tool.execute.before`, every tool whose name starts with a turned-off server's name. OpenCode names MCP tools `<server>_<tool>`. The longest matching server name decides, and built-in tool names are never refused.

What the launch does not cover:
- a server added in the middle of a session (`POST /mcp`, the `/mcp` dialog);
- config from a `.well-known/opencode` or console org endpoint the user has signed in to, which OpenCode merges after nav-pilot's;
- macOS managed preferences, which merge last and win.
