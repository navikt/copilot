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
