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

In OpenCode, a plugin hook's answer controls what the model reads. Copilot's postToolUse can only ignore a hook that fails. So the OpenCode bridge withholds the output when redaction cannot run: the binary is missing, the hook times out (5 s), the answer is not JSON, or `nav-pilot hook redact` reports an error. The bridge asks for that report with `NAV_PILOT_HOOK_FAIL_CLOSED=1`, since otherwise the hook answers `{}` on every failure. The model gets a line that names the failure and how to turn redaction off. The loop guard and the gates fail open, as under Copilot, because a broken guard must not stop work.

## Subagents

`tool.execute.before` and `after` fire in every session, a subagent's child session included (verified against opencode 1.18.32: a `general` subagent's `cat` of a secret reached its model redacted). Redaction and the loop guard run there on purpose, because a secret a subagent reads reaches a model the same way. The gates also run there, as they do for Copilot subagents. The loop guard keeps one run per session and skips sessions on the local provider (`mlx`), where the guard proxy already ends the turn.

## Known gaps

Read in the OpenCode 1.18.32 source (`packages/opencode/src/session/tools.ts`, `session/message-v2.ts`, `tool/code-mode.ts`):

- **A tool that throws skips `tool.execute.after`.** The error message reaches the model unredacted. Built-in tools mostly throw with a path or a status; the case that matters is an MCP tool reporting `isError` in code mode, whose error text is its content.
- **An interrupted `bash` call** (the user cancels it) hands the model the partial output from `metadata.output` without `tool.execute.after`.
- **`--pure`** loads no plugins, so no hooks run. nav-pilot says so on stderr at launch.
- The human sees unredacted output in the TUI (`metadata`), as with Copilot, because redaction changes what the model reads, not the terminal.
- Repo gates run without Copilot's folder-trust check. OpenCode has no such check and already loads a repo's `.opencode/plugins` as code, so a repo's gate entries give it nothing new.
