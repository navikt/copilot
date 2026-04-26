# Template: Naisjob (Batch Job)

README template for a Nais-deployed batch job (naisjob). Replace `{placeholders}` with actual values.

---

# {Job Name}

{One-sentence description of what this job does — under 120 characters.}

## Kjøreplan

| | |
|---|---|
| **Schedule** | `{cron-expression}` ({human-readable, e.g., daglig kl. 06:00}) |
| **Trigger** | {Cron / manuell / event-basert} |
| **Varighet** | {Typisk kjøretid, e.g., 2–5 minutter} |
| **Idempotent** | {Ja/Nei — trygt å kjøre flere ganger?} |

## Hva gjør jobben?

{2–3 sentences describing the job's purpose and what data it processes.}

1. {Step 1 — e.g., Henter data fra BigQuery}
2. {Step 2 — e.g., Transformerer og validerer}
3. {Step 3 — e.g., Skriver til database / sender til Kafka}

## Feilhåndtering

| Scenario | Håndtering |
|----------|-----------|
| Transient feil (nettverkstimeout) | {e.g., 3 retry med eksponentiell backoff} |
| Permanent feil (ugyldig data) | {e.g., Logger og hopper over, varsler team} |
| Jobb feiler helt | {e.g., Slack-varsel til #team-kanal, manuell rekjøring via Nais Console} |

## Konfigurasjon

| Variabel | Beskrivelse | Påkrevd | Standard |
|----------|-------------|---------|----------|
| {`ENV_VAR`} | {Description} | {Ja/Nei} | {default or —} |

## Observability

- **Dashboard:** [Grafana]({grafana-url})
- **Logger:** [Kibana]({kibana-url})
- **Alerts:** {Slack channel for job failure alerts}
- **Metrics:** `/metrics` (Prometheus)

## Deploy

- **Plattform:** Nais (naisjob)
- **Dev:** {e.g., Automatisk ved push til `main`}
- **Prod:** {e.g., Manuell dispatch}
- **Manifester:** `.nais/dev.yaml`, `.nais/prod.yaml`

## Utvikling

```bash
git clone https://github.com/navikt/{job-name}
cd {job-name}
{install-command}
{test-command}
```

### Kjøre lokalt

```bash
{command to run the job locally, e.g.:}
{env-vars} {run-command}
```

## Team

- **Team:** {#team-name}
- **Slack:** [{#team-channel}]({slack-url})
- **Kontakt:** {email}

## Lisens

[{License}](LICENSE)
