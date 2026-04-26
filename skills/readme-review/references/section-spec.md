# Section Spec — Required and Optional Sections by Project Type

Sections are listed in **cognitive funneling order**: broad context first, details later. This is the recommended order for README sections.

## Section Matrix

| # | Section | Service | Library | Monorepo | Naisjob | Notes |
|---|---------|:-------:|:-------:|:--------:|:-------:|-------|
| 1 | Title + one-liner | **R** | **R** | **R** | **R** | Self-explanatory name + < 120 char description |
| 2 | Badges | O | O | O | O | 2–4 max (CI, coverage, license). No badge walls. |
| 3 | Quick start | **R** | **R** | **R** | O | Copy-paste commands to run locally |
| 4 | Tech stack | **R** | O | **R** | O | Languages, frameworks, key dependencies |
| 5 | Structure map | O | — | **R** | — | Directory layout with descriptions |
| 6 | API / endpoints | **R** | **R** | — | — | Table of endpoints or link to OpenAPI. For libraries: exported API. |
| 7 | Configuration | **R** | O | — | **R** | Table: env var, description, required, default |
| 8 | Deployment | **R** | — | **R** | **R** | Platform, environments, manifest location |
| 9 | Schedule / triggers | — | — | — | **R** | Cron expression, trigger mechanism, frequency |
| 10 | Failure semantics | — | — | — | **R** | What happens on failure? Retry? Alert? Manual intervention? |
| 11 | Auth / integrations | C | — | O | C | Which auth mechanism, which services consumed/exposed |
| 12 | Observability | **R** | — | O | **R** | Dashboards, logs, alerts, metrics |
| 13 | Install / compatibility | — | **R** | — | — | Package manager install, supported versions/platforms |
| 14 | Usage examples | O | **R** | — | — | Code samples showing common use cases |
| 15 | Contributing | O | **R** | **R** | O | How to run tests, submit PRs |
| 16 | Team / ownership | **R** | O | **R** | **R** | Owning team, Slack channel, contact |
| 17 | License | **R** | **R** | **R** | **R** | License name and link |
| 18 | Table of Contents | >100 | >100 | **R** | >100 | Required if README exceeds 100 lines |

**Legend:** **R** = Required, O = Optional, C = Conditional (include if relevant), — = Not applicable, >100 = Required if README > 100 lines

## Section Details

### 1. Title + one-liner

```markdown
# my-service

Behandler søknader om dagpenger og sender vedtak til Arena.
```

- Title must match repository or directory name
- One-liner directly below title, no heading, < 120 characters
- Say what it **does**, not what it **is** ("Behandler søknader" not "En tjeneste for søknadsbehandling")

### 2. Badges

```markdown
[![CI](https://github.com/navikt/my-service/actions/workflows/ci.yml/badge.svg)](...)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](...)
```

- Max 2–4 badges. CI status and license are most useful.
- Don't add badges for metrics nobody checks in the README.

### 3. Quick start

```markdown
## Kom i gang

**Forutsetninger:** [mise](https://mise.jdx.dev)

mise install    # Install tools
mise dev        # Start dev server (http://localhost:8080)
mise test       # Run tests
```

- Must contain runnable commands, not just prose
- List prerequisites explicitly (versions if unusual)
- Show the shortest path from `git clone` to working app

### 4. Tech stack

```markdown
## Tech stack

- **Backend:** Kotlin 2.0, Ktor 3.0
- **Database:** PostgreSQL (via Kotliquery + HikariCP)
- **Platform:** Nais (Kubernetes on GCP)
- **Auth:** Azure AD + TokenX
```

- Bullet list with specific versions
- Only list what matters for understanding the project

### 5. Structure map (monorepo)

```markdown
## Struktur

| Directory | Description |
|-----------|-------------|
| `apps/frontend/` | Next.js innbygger-frontend |
| `apps/backend/` | Kotlin/Ktor API |
| `libs/shared/` | Delte typer og utility-funksjoner |
```

- Use a table for > 3 directories
- Link to per-app READMEs if they exist

### 6. API / endpoints

```markdown
## API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/vedtak` | List vedtak for current user |
| GET | `/api/vedtak/{id}` | Get vedtak by ID |
| POST | `/api/vedtak` | Create new vedtak |
| GET | `/health` | Health check |
```

- Table for < 10 endpoints; link to OpenAPI/Swagger for more
- For libraries: document exported functions/types

### 7. Configuration

```markdown
## Konfigurasjon

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `DB_URL` | PostgreSQL connection string | Yes | — |
| `PORT` | Server port | No | `8080` |
| `LOG_LEVEL` | Log level | No | `INFO` |
```

- Table format with required/default columns
- Don't list Nais-injected variables that are standard (e.g., `NAIS_CLUSTER_NAME`)

### 8. Deployment

```markdown
## Deploy

Deployes til Nais via GitHub Actions.

- **Dev:** Automatisk ved push til `main`
- **Prod:** Manuell dispatch eller tag
- **Manifester:** `.nais/dev.yaml`, `.nais/prod.yaml`
```

### 9. Schedule / triggers (naisjob)

```markdown
## Kjøreplan

- **Cron:** `0 6 * * *` (daglig kl. 06:00)
- **Trigger:** Kan også kjøres manuelt via `kubectl create job`
- **Varighet:** Typisk 2–5 minutter
```

### 10. Failure semantics (naisjob)

```markdown
## Feilhåndtering

- **Retry:** 3 forsøk med eksponentiell backoff
- **Alarm:** Slack-varsling til #team-kanal ved feil
- **Manuell:** Kan rekjøres via Nais Console
- **Idempotent:** Ja — trygt å kjøre flere ganger
```

### 11. Auth / integrations

```markdown
## Auth og integrasjoner

- **Innkommende:** Azure AD (saksbehandler-frontend)
- **Utgående:** PDL via TokenX, Dokarkiv via Maskinporten
- **Access policy:** Se `.nais/dev.yaml`
```

- Only include if the service has non-trivial auth or external integrations

### 12. Observability

```markdown
## Observability

- **Dashboard:** [Grafana](https://grafana.nav.cloud.nais.io/d/...)
- **Logger:** [Kibana](https://logs.adeo.no/...)
- **Alerts:** Slack #team-dagpenger-alerts
- **Metrics:** `/metrics` (Prometheus)
```

### 13. Install / compatibility (library)

```markdown
## Installasjon

npm install @navikt/my-lib

# or
dependencies {
    implementation("no.nav:my-lib:1.2.3")
}
```

- Show the package manager command
- Note compatible versions/platforms if relevant

### 14. Usage examples (library)

```markdown
## Bruk

import { validateFnr } from '@navikt/my-lib'

const result = validateFnr('12345678901')
// { valid: true, type: 'fnr' }
```

- Show the simplest working example first
- Link to more examples if complex

### 15. Contributing

```markdown
## Bidra

1. Fork og klon repoet
2. Kjør `mise install` for å sette opp verktøy
3. Lag en branch fra `main`
4. Kjør `mise check` før du pusher
5. Opprett en PR

Se [CONTRIBUTING.md](CONTRIBUTING.md) for mer detaljer.
```

### 16. Team / ownership

```markdown
## Team

- **Team:** #team-dagpenger
- **Slack:** [#team-dagpenger](https://nav-it.slack.com/archives/...)
- **Kontakt:** dagpenger@nav.no
```

### 17. License

```markdown
## Lisens

[MIT](LICENSE)
```

### 18. Table of Contents

Required if README exceeds 100 lines. Place after title + one-liner, before first H2.
