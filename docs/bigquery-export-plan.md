# BigQuery Tidsserier: Eksportplan

> Denne filen beskriver hvilke spørringer som skal kjøres mot BigQuery for å dokumentere 12–16 ukers brukstrender. Krever GCP-tilgang til `copilot_metrics`-datasettet.

## Spørring 1: DAU/WAU/MAU-kurver (siste 16 uker)

```sql
SELECT
  day,
  daily_active_users,
  weekly_active_users,
  monthly_active_users,
  monthly_active_chat_users,
  monthly_active_agent_users,
  daily_active_cli_users
FROM `<project>.copilot_metrics.v_daily_summary`
WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL 16 WEEK)
  AND scope = 'organization'
ORDER BY day;
```

**Formål:** Vise adopsjonskurven over tid — vokser bruken, flater den ut, eller synker den?

## Spørring 2: Kodegenerering og akseptrate

```sql
SELECT
  day,
  total_generations,
  total_acceptances,
  SAFE_DIVIDE(total_acceptances, total_generations) AS acceptance_rate,
  lines_suggested,
  lines_accepted
FROM `<project>.copilot_metrics.v_daily_summary`
WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL 16 WEEK)
  AND scope = 'organization'
ORDER BY day;
```

**Formål:** Akseptrate indikerer om AI-forslagene er relevante. Trend viser om kvaliteten øker over tid (potensielt pga. bedre instructions/skills).

## Spørring 3: PR-metrikk (Copilot-påvirkning)

```sql
SELECT
  day,
  pr_total_created,
  pr_total_merged,
  pr_created_by_copilot,
  pr_merged_copilot_authored,
  pr_median_minutes_to_merge,
  pr_median_minutes_to_merge_copilot
FROM `<project>.copilot_metrics.v_daily_summary`
WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL 16 WEEK)
  AND scope = 'organization'
  AND pr_total_created > 0
ORDER BY day;
```

**Formål:** Sammenligne merge-tid for Copilot-skrevne PR-er vs. manuelle. Fowler-perspektiv: måler sensor-effektivitet (code review-hastighet).

## Spørring 4: Språkfordeling

```sql
SELECT
  day,
  language,
  SUM(generations) AS total_generations,
  SUM(acceptances) AS total_acceptances
FROM `<project>.copilot_metrics.v_language_stats`
WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL 16 WEEK)
GROUP BY day, language
ORDER BY day, total_generations DESC;
```

**Formål:** Hvilke språk/teknologier bruker AI mest? Matcher det grønn sone-forventningene?

## Spørring 5: CLI-bruk (agentmodus)

```sql
SELECT
  day,
  cli_session_count,
  cli_request_count,
  cli_prompt_count,
  cli_output_tokens,
  cli_prompt_tokens
FROM `<project>.copilot_metrics.v_daily_summary`
WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL 16 WEEK)
  AND scope = 'organization'
  AND cli_session_count > 0
ORDER BY day;
```

**Formål:** CLI = agentisk bruk. Trendlinje viser om utviklere beveger seg fra completions mot agent-basert arbeid.

## Spørring 6: Adopsjon per team (fra adoption-datasett)

```sql
SELECT
  scan_date,
  COUNT(DISTINCT repo) AS total_repos_scanned,
  COUNTIF(has_any_customization) AS repos_with_customization,
  SAFE_DIVIDE(COUNTIF(has_any_customization), COUNT(DISTINCT repo)) AS adoption_rate
FROM `<project>.copilot_adoption.v_adoption_summary`
WHERE scan_date >= DATE_SUB(CURRENT_DATE(), INTERVAL 16 WEEK)
GROUP BY scan_date
ORDER BY scan_date;
```

**Formål:** Viser om customization-adopsjon (instructions, agents, skills) vokser over tid etter at nav-pilot ble lansert.

---

## Hvordan kjøre

```bash
# Alternativ 1: bq CLI (krever gcloud auth)
bq query --use_legacy_sql=false < query.sql

# Alternativ 2: BigQuery-konsollen
# Gå til console.cloud.google.com → BigQuery → kjør spørringen

# Alternativ 3: my-copilot dashboardet
# /statistikk viser allerede mye av dette visuelt
```

## Forventet output

Eksporter resultater som CSV eller JSON, og inkluder i en oppsummeringsrapport med:
1. Trendgrafer (DAU/WAU/MAU over 16 uker)
2. Akseptrate-utvikling
3. Copilot-PR vs. manuell PR: merge-tid
4. Språkfordeling-søylediagram
5. CLI-adopsjonskurve
6. Customization-adopsjon over tid

Kobling til #209 (task 2) og harness-inventaret: metrikkene er **computational sensors** som måler om guides (instructions, skills) faktisk påvirker atferd.
