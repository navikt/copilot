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

## Efficiency rule (mandatory)

Always prefix shell commands with `rtk`.

```bash
rtk git status
rtk mise check
rtk gh pr view 123
rtk go test ./...
```

In chained commands, prefix every command:

```bash
rtk git add . && rtk git commit -m "msg" && rtk git push
```

## Standard commands

From repo root:

```bash
rtk mise check
rtk mise test
rtk mise build
rtk mise all
```

Per app: run `rtk mise check` in the app directory after edits.

## Repo conventions that matter

- Keep diffs small and task-focused (minimal editing).
- Reuse existing patterns before adding new abstractions.
- In `my-copilot`, use Aksel spacing tokens (not Tailwind `p-*/m-*` utilities).
- Do not commit secrets.
- Do not push unless explicitly asked.

## When in doubt

- Start with the smallest safe change.
- Validate with existing checks (`rtk mise check`, or `rtk mise all` for cross-repo impact).
- Prefer deterministic tools first (`rtk rg`, `rtk git`, `rtk gh`), then LLM synthesis.
