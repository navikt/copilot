# Template: Service / API

README template for a Nais-deployed service or API. Replace `{placeholders}` with actual values.

---

# {Project Name}

{One-sentence description of what this service does — under 120 characters.}

## Kom i gang

**Forutsetninger:** {e.g., mise, Docker, VPN}

```bash
git clone https://github.com/navikt/{project-name}
cd {project-name}
{install-command}
{dev-command}        # {e.g., http://localhost:8080}
{test-command}
```

## Tech stack

- **Språk:** {e.g., Kotlin 2.0}
- **Rammeverk:** {e.g., Ktor 3.0}
- **Database:** {e.g., PostgreSQL via Kotliquery + HikariCP}
- **Plattform:** Nais (Kubernetes on GCP)
- **Auth:** {e.g., Azure AD + TokenX}

## API

| Method | Path | Description |
|--------|------|-------------|
| {GET} | {/api/resource} | {Description} |
| {GET} | `/health` | Health check |
| {GET} | `/metrics` | Prometheus metrics |

{If > 10 endpoints: link to OpenAPI spec instead.}

## Konfigurasjon

| Variabel | Beskrivelse | Påkrevd | Standard |
|----------|-------------|---------|----------|
| {`ENV_VAR`} | {Description} | {Ja/Nei} | {default or —} |

## Deploy

Deployes til Nais via GitHub Actions.

- **Dev:** {e.g., Automatisk ved push til `main`}
- **Prod:** {e.g., Manuell dispatch}
- **Manifester:** `.nais/dev.yaml`, `.nais/prod.yaml`

## Observability

- **Dashboard:** [Grafana]({grafana-url})
- **Logger:** [Kibana]({kibana-url})
- **Alerts:** {Slack channel}
- **Metrics:** `/metrics` (Prometheus)

## Team

- **Team:** {#team-name}
- **Slack:** [{#team-channel}]({slack-url})
- **Kontakt:** {email}

## Bidra

1. Lag branch fra `main`
2. Kjør `{check-command}` før du pusher
3. Opprett PR

## Lisens

[{License}](LICENSE)
