# 📦 Recommended selections

Nav's default content is one agentpakke: `nav-pilot`, declared in
[`.nav-pilot/agentpakke.json`](../.nav-pilot/agentpakke.json). Installing it gives you every
agent, skill, instruction, and prompt in this repository:

```bash
nav-pilot install nav-pilot
```

That is deliberately everything. Instructions are glob-scoped (`*.kt` files never activate the
Next.js rules), skills load on demand, and only the `nav-pilot` personas are primary agents —
so "everything" costs a directory of markdown, not always-on context.

The five curated collections (`frontend`, `nextjs-frontend`, `kotlin-backend`, `fullstack`,
`platform`) were folded into this single pakke
([#468](https://github.com/navikt/copilot/issues/468)). Existing installs migrate on their next
`nav-pilot sync`: the scope keeps exactly the files it had, and the rest of the pool is
recorded as ignored.

## Installing less

Deselect in the interactive picker (`nav-pilot install`, or reinstall any time). For
user-scope installs you can also suppress single items afterwards:

```bash
nav-pilot ignore instruction nextjs-aksel --user
```

Deselections are recorded in the scope's state and survive sync and reinstall.

## What to keep, by team type

Guidance, not mechanism — start from everything and deselect what your stack never touches:

| Team type | Worth keeping | Safe to deselect |
| --- | --- | --- |
| **Kotlin backend** | `kotlin-*`, `spring-boot-*`, `ktor-*`, `flyway-migration`, `kafka`, `postgresql-review`, `tokenx-auth` | `nextjs-aksel`, `aksel-*`, `performance`, `testing-typescript`, frontend prompts |
| **Frontend (non-Next.js)** | `aksel-*`, `accessibility`, `playwright-testing`, `nav-dekoratoren`, `testing-typescript`, `norwegian-text` | `nextjs-aksel`, `nextjs-api-route`, all `kotlin-*`/`ktor-*`/`spring-*`, `kafka`, `flyway-migration` |
| **Next.js frontend** | The frontend set plus `nextjs-aksel`, `performance`, `nextjs-api-route` | All `kotlin-*`/`ktor-*`/`spring-*`, `kafka`, `flyway-migration` |
| **Fullstack** | Backend and frontend sets combined | Little — this was the union already |
| **Platform / DevOps** | `nais`, `observability-*`, `security-*`, `golang`, `rust-development`, `threat-model`, `workstation-security` | Framework-specific frontend and Kotlin content |

The shared core — `code-review`, `deliberate-ai-use`, the `nav-plan`/`nav-troubleshoot`
planning skills, `security-owasp`, `conventional-commit`, `klarsprak`, `terse-mode` — was in
every collection and belongs in every selection.

## Exporting for other tools

If you use [OpenCode](https://github.com/anomalyco/opencode) instead of GitHub Copilot,
`nav-pilot` materializes the same content there; see
[README.nav-pilot.md](README.nav-pilot.md#eksport-til-andre-verktøy).
