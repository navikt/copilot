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

## Å kjøre nav-pilot-binæren under testing

`nav-pilot` skriver til brukerens eget miljø: `~/.copilot/`, `~/.nav-pilot/` og
det repoet du står i. En verifisering som kjører den ekte binæren uten isolasjon
endrer utviklerens faktiske oppsett — og det har skjedd: en installasjon under
testing pekte et brukerscope mot en midlertidig worktree og etterlot 23 av 52
filer i konflikt, som `sync` deretter hoppet over i det stille.

Derfor, hver gang du kjører binæren for å sjekke oppførsel:

- Sett `HOME` og `COPILOT_HOME` til en midlertidig katalog. Ikke stol på at
  kommandoen «bare leser» — `install`, `sync`, `config` og oppstartsveiene
  skriver alle sammen.
- Ta en kopi av `~/.copilot` og `~/.nav-pilot` før kjøringen, sammenlign etterpå,
  og rapporter enhver forskjell. Isolasjon som er satt opp feil ser ut som
  isolasjon helt til noen ser etter.
- Rapporter det høyt hvis du likevel rørte noe. En stille endring i et
  brukeroppsett er verre enn en feilet test.
