# AGENTS.md — navikt/copilot

Minimal guide for agents working in this repository.

## What this repo is

Monorepo for Nav’s Copilot ecosystem:

- `apps/my-copilot` (Next.js/TypeScript web app)
- `apps/copilot-api` (Go backend API)
- `apps/copilot-metrics` (Go metrics job)
- `apps/mcp-onboarding` (Go MCP reference server)
- `apps/mcp-registry` (Go MCP registry API)

Security model and trust boundaries: see `SECURITY.md`.

## Efficiency rule

Keep tool output small: run targeted commands, avoid dumping whole files or full
build logs into context, and prefer deterministic tools over asking the model to
guess.

`rtk` is an optional CLI proxy that filters terminal output before it reaches the
model. It is available if you want it (`rtk git status`), but it is not required
and not assumed anywhere in this repo — public controlled measurement has not
reproduced its advertised savings
([JetBrains study](https://blog.jetbrains.com/ai/2026/07/rtk-claude-code-token-savings/)).

## Standard commands

From repo root:

```bash
mise check
mise test
mise build
mise all
```

Per app: run `mise check` in the app directory after edits.

## Repo conventions that matter

- Keep diffs small and task-focused (minimal editing).
- Reuse existing patterns before adding new abstractions.
- In `my-copilot`, use Aksel spacing tokens (not Tailwind `p-*/m-*` utilities).
- Do not commit secrets.
- Do not push unless explicitly asked.

## When in doubt

- Start with the smallest safe change.
- Validate with existing checks (`mise check`, or `mise all` for cross-repo impact).
- Prefer deterministic tools first (`rg`, `git`, `gh`), then LLM synthesis.
