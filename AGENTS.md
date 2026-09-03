# AGENTS.md for navikt/copilot

Minimal guide for agents working in this repository.

## What this repo is

Monorepo for Nav's Copilot ecosystem:

- `apps/my-copilot` (Next.js/TypeScript web app)
- `apps/copilot-api` (Go backend API)
- `apps/copilot-metrics` (Go metrics job)
- `apps/mcp-onboarding` (Go MCP reference server)
- `apps/mcp-registry` (Go MCP registry API)

`SECURITY.md` has the security model and trust boundaries.

## Efficiency rule

Keep tool output small. Run targeted commands, do not dump whole files or full build logs into context, and reach for deterministic tools (`rg`, `git`, `gh`) before asking the model to guess.

`rtk` is an optional CLI proxy that filters terminal output before it reaches the model. Use it if you want to (`rtk git status`). Nothing here requires or assumes it, and public controlled measurement has not reproduced its advertised savings ([JetBrains study](https://blog.jetbrains.com/ai/2026/07/rtk-claude-code-token-savings/)).

## Standard commands

From repo root:

```bash
mise check
mise test
mise build
mise all
```

After editing an app, run `mise check` in that app's directory. Run `mise all` when the change reaches across the repo.

## Conventions

- Start with the smallest safe change and keep diffs task-focused.
- Reuse existing patterns before adding new abstractions.
- In `my-copilot`, use Aksel spacing tokens, not Tailwind `p-*/m-*` utilities.
- Do not commit secrets.
- Do not push unless explicitly asked.

## Running the nav-pilot binary in tests or verification

`nav-pilot` writes to the developer's own environment: `~/.copilot/`,
`~/.nav-pilot/` and the repository you are standing in. A verification that runs
the real binary without isolation changes a real setup, and it has: an install
run during testing pointed a user scope at a temporary worktree and left 23 of 52
files in conflict, which `sync` then silently skipped. Nothing had been edited by
hand.

Whenever you run the binary — directly, or through a test that calls `run()`:

- Set `HOME` to a temporary directory. That is the one that matters, because
  everything resolves through `os.UserHomeDir`. `nav-pilot` does **not** read
  `COPILOT_HOME`. Also set `NAV_PILOT_CONFIG` (relocates
  `~/.nav-pilot/config.toml`, see `internal/cli/config.go`) and
  `XDG_CONFIG_HOME` (honoured on the opencode export path, see
  `internal/provider/opencode_launch.go`). In Go tests, `isolatedConfig(t)` does
  this for you — use it.
- Do not assume a command only reads. `install`, `sync`, `config` and the launch
  paths all write.
- Hash `~/.copilot` and `~/.nav-pilot` before and after, and report any
  difference. Isolation that is set up wrong looks exactly like isolation until
  someone checks.
- Say so loudly if you did touch something. A silent change to a developer's
  setup is worse than a failed test.
