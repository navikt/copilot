# copilot-metrics

Nightly job to fetch GitHub Copilot usage metrics from the GitHub API and store them in BigQuery.

## Overview

This application addresses the migration from the deprecated `GET /orgs/{org}/copilot/metrics` API (shutdown April 2, 2026) to the new Usage Metrics API with BigQuery storage.

### Architecture

```text
GitHub Usage Metrics API → copilot-metrics (Naisjob) → BigQuery
                                                           ↓
                                                    my-copilot dashboard
```

## Features

- **Nightly ingestion**: Fetches missing metrics daily at 06:00 UTC
- **Automatic gap filling**: Detects missing days in BigQuery and backfills them automatically
- **Enterprise-level data**: Uses enterprise API for richer data (CLI usage, agent modes, LoC metrics)
- **Fallback to org-level**: Gracefully degrades to organization API if enterprise fails
- **Historical backfill**: CLI flag to load historical data from Oct 10, 2025
- **Idempotent**: Re-runs delete and re-insert data for the same day
- **Raw JSON storage**: Schema changes in GitHub API don't break the pipeline
- **Billing usage report ingestion**: Daily organization billing usage rows with one-off backfill support
- **Slack alerts**: Notifies on ingestion failures when `SLACK_WEBHOOK_URL` is configured

## Usage

### Mise backfill tasks (recommended)

Explicit tasks so "backfill" is unambiguous:

```bash
# Usage metrics only
bash -lc 'cd apps/copilot-metrics && mise backfill:usage'

# Monthly billing only
bash -lc 'cd apps/copilot-metrics && mise backfill:billing-monthly'

# Daily billing usage reports only
bash -lc 'cd apps/copilot-metrics && mise backfill:billing-daily-report'

# Daily model billing usage only
bash -lc 'cd apps/copilot-metrics && mise backfill:billing-model-daily'

# Per-repository usage metrics only
bash -lc 'cd apps/copilot-metrics && mise backfill:repo-metrics'

# Everything
bash -lc 'cd apps/copilot-metrics && mise backfill:all'
```

Prod variants:

```bash
bash -lc 'cd apps/copilot-metrics && mise backfill:usage:prod'
bash -lc 'cd apps/copilot-metrics && mise backfill:billing-monthly:prod'
bash -lc 'cd apps/copilot-metrics && mise backfill:billing-daily-report:prod'
bash -lc 'cd apps/copilot-metrics && mise backfill:billing-model-daily:prod'
bash -lc 'cd apps/copilot-metrics && mise backfill:repo-metrics:prod'
bash -lc 'cd apps/copilot-metrics && mise backfill:all:prod'
```

### Nightly job (default)

Runs as a Kubernetes CronJob via NAIS. Automatically detects missing days in BigQuery and fills gaps:

```bash
copilot-metrics --run-once
```

### Automatic user billing

`--user-billing-sync` resumes unfinished closed UTC months from October 2026.
Manual `--user-billing-month` remains limited to the historical months before
October 2026; future months must use the identity- and coverage-checked worker.
Months before June 2026 were billed as premium requests, so the command reads
the `premium_request` endpoint for them (per user and for the enterprise total)
and stores the SKUs `Copilot Premium Request` and `Coding Agent Premium
Request` in the same table.
It runs separately from daily ingestion, for at most 50 minutes. Each execution
allows at most 2,000 identity/billing requests and stops with 500 requests left
in the token's reported quota. These limits include retries; other token users
can still consume quota concurrently.

The deployed worker uses the existing `copilot-metrics` secret and runs nightly
at 09:00 UTC in dev and 08:00 UTC in prod. A month becomes eligible on day 3 of
the next month, so October 2026 is first collected on November 3. Every publish
stores the enterprise total and the residual against the per-user rows in
`billing_user_monthly_runs`; `--user-billing-publish=YYYY-MM` reruns only that
step for a completed month. Set `USER_BILLING_SYNC_ENABLED=false` to disable collection.
`concurrencyPolicy: Forbid` prevents overlapping scheduled runs.
Before a manual billing load, disable the scheduled worker and wait for active
pods and their BigQuery jobs to finish. This rule also applies to local binaries.

Source failures and unresolved GitHub identities keep a month unpublished and
trigger the existing Slack error notification. Completed user checkpoints resume
next night. Enterprise totals are stored with publication; team timing remains
an estimate derived from live daily usage and membership reports.

Completed months are observations at collection time, not final invoices.
Automatic correction refresh is not implemented. Never delete completion
markers to refresh a published month; a correction needs a staged replacement.

### Historical backfill

One-time operation to load historical data:

```bash
copilot-metrics --backfill
copilot-metrics --backfill --backfill-from=2025-10-10
```

### Billing usage backfill

One-time operation to load premium request billing data per model (requires `GITHUB_BILLING_TOKEN`):

```bash
copilot-metrics --billing-monthly-backfill
copilot-metrics --billing-monthly-backfill --billing-monthly-from=2025-01
copilot-metrics --billing-monthly-backfill --billing-monthly-from=2025-01 --force
```

### Billing usage report backfill (daily rows)

One-time operation to load daily organization billing usage report rows (requires `GITHUB_BILLING_TOKEN`):

```bash
copilot-metrics --billing-daily-report-backfill
copilot-metrics --billing-daily-report-backfill --billing-daily-report-from=2025-10-10
copilot-metrics --billing-daily-report-backfill --billing-daily-report-from=2025-10-10 --force
```

### Billing daily model backfill

One-time operation to load daily model-level premium request billing data (requires `GITHUB_BILLING_TOKEN`):

```bash
copilot-metrics --billing-model-daily-backfill
copilot-metrics --billing-model-daily-backfill --billing-model-daily-from=2025-10-10
copilot-metrics --billing-model-daily-backfill --billing-model-daily-from=2025-10-10 --force
```

### Repository metrics backfill

One-time operation to fill gaps in the `repository_metrics` table without
touching `usage_metrics`, `user_metrics` or `user_teams`.

The nightly job only gap-fills the last 7 days, so days between the report's
GA date (2026-07-17) and the deploy of repository ingestion fall outside that
window permanently. This flag closes those gaps.

```bash
copilot-metrics --repo-metrics-backfill
copilot-metrics --repo-metrics-backfill --repo-metrics-from=2026-07-17
copilot-metrics --repo-metrics-backfill --repo-metrics-from=2026-07-17 --force
```

Days that already have data are skipped before the API call, so the command is
cheap to re-run. Unlike `--backfill`, it resumes per day rather than from a
single high-water mark, so interior gaps are filled. Use `--force` to re-ingest
days that already exist. Days before GA are reported as unavailable and skipped.

### Local development

```bash
export GITHUB_APP_ID=123456
export GITHUB_APP_PRIVATE_KEY="$(cat private-key.pem)"
export GITHUB_APP_INSTALLATION_ID=789
export GCP_TEAM_PROJECT_ID=my-project
export LOG_LEVEL=DEBUG

go run . --run-once
```

### Mimir daily copy

The `copilot-mimir-daily` Naisjob ([.nais/mimir.yaml](.nais/mimir.yaml), 03:00 UTC, prod only) copies yesterday's aggregates to the configured authoritative source table `<GCP_TEAM_PROJECT_ID>.copilot_metrics_staging.mimir_daily`.

Configuration and overrides:
- `BIGQUERY_DATASET` defaults to `copilot_metrics` across the service for GitHub usage reports ([config.go](config.go#L42)), but [.nais/mimir.yaml](.nais/mimir.yaml#L30) explicitly overrides it to `copilot_metrics_staging` for the Mimir job.
- `apps/copilot-api` configures `COPILOT_MIMIR_DATASET` with a default of `copilot_metrics_staging` ([config.go](../copilot-api/config.go#L68)) to query this table.

Schedule, evaluation window, and retention:
- Schedule: Runs daily at 03:00 UTC ([.nais/mimir.yaml](.nais/mimir.yaml#L9)) to leave a margin for late sample acceptance in Mimir.
- Windows: Evaluates daily batches strictly UTC midnight to midnight (`t = Day + 1 at 00:00:00 UTC`). Most PromQL queries evaluate `[1d]` range vectors ([mimir_daily.go](mimir_daily.go#L636-L637)). However, several queries evaluate wider rolling windows: `nav_pilot_session` device cohorts evaluate distinct devices over `[7d]` ([mimir_daily.go](mimir_daily.go#L441)), and `nav_pilot_reach` queries evaluate rolling `[7d]` and `[28d]` windows for adoption alongside `[1d]` ([mimir_daily.go](mimir_daily.go#L546-L556)).
- Retention: Source comments assume Mimir retains around 70 days; this is a source assumption unless verified at runtime. The CLI flag `--mimir-from` enforces an explicit lookback limit of 60 days ([main.go](main.go#L74)) to stay within that assumed retention.
- Storage retention: The daily scratch load table (`mimir_daily_load_<date>`) has an explicit 6-hour expiration ([mimir_daily.go](mimir_daily.go#L742)). The target table `mimir_daily` has no explicit partition expiration configured in repository schema metadata. Lack of a configured TTL in source definitions does not guarantee permanent retention, as GCP dataset or project lifecycle policies may apply at runtime.

Query temporality (#902) and counter reliability:
- Session telemetry (`nav_pilot_session_*`, `nav_pilot_subagent_selected_total`) originates from `newOnlyExporter`, emitting one delta per completed session. Queries aggregate with `sum by (...) (sum_over_time(...[1d]))` ([mimir_daily.go](mimir_daily.go#L405)).
- Counter vs. duration reliability: Standard OTel counters (`token_usage`, `inference_tokens`, `code_lines`, and the `github_copilot_tool_call_count_total` counter in `tool_call`) suffer from counter-reset artifacts in Mimir when concurrent developer sessions reset; rows for these counters are marked `counter_reliable = false` ([mimir_daily.go](mimir_daily.go#L52-L66)). In contrast, duration histograms (`operation_duration`, `execute_tool`, and `github_copilot_tool_call_duration_seconds` in `tool_call`) as well as session rows have `counter_reliable = true` ([mimir_daily.go](mimir_daily.go#L59-L61)).

Aggregation thresholds and suppression:
- Families with device labels require at least five distinct devices (`nav_pilot_device_id`). Rows under five devices merge configured dimensions into `andre`. If an `andre` row still has fewer than five devices, it is dropped entirely ([mimir_daily.go](mimir_daily.go#L269-L272), [mimir_daily.go](mimir_daily.go#L292-L293)). Quantiles (`p50`, `p95`) are omitted for `andre`.
- Session device cohorts: Distinct devices are counted over a 7-day window (`[7d]`, [mimir_daily.go](mimir_daily.go#L441)). Merge dimensions extend beyond `model` and `tool` to include `operation`, `source`, and `error_type` ([mimir_daily.go](mimir_daily.go#L107-L118), [mimir_daily.go](mimir_daily.go#L472-L482)). When no merge dimensions are configured for a metric (such as gate, ready, and hook redact), cohorts with fewer than five devices are dropped outright ([mimir_daily.go](mimir_daily.go#L473-L475)).
- Config and reach families: `nav_pilot_config` and `nav_pilot_reach` evaluate devices across `1d`, `7d`, and `28d` windows and drop cohorts with fewer than five devices outright without merging ([mimir_daily.go](mimir_daily.go#L532-L535), [mimir_daily.go](mimir_daily.go#L567-L569)).
- Device-less families: `copilot_chat` (grouped by edit outcomes and sources) and version-bearing `nav_pilot_session` metrics (grouped by `version` and `client`, from `copilot_session.go`) carry no device labels and do not share a single schema grain; both aggregate Nav-wide without a device threshold.

Interpreting missing rows:
Absence of a row in `mimir_daily` cannot be inferred interchangeably as zero, uninstrumented, or suppressed:
1. Pre-instrumentation: Sessions prior to client instrumentation (code introduced in `nav-pilot` on 2026-10-12 via commits `ee4b37e5` and `14c5bde6`) emit no session metrics. Commit timestamps record code introduction, not deployment across developer machines.
2. Observed zero: An observed zero requires a returned sample or row with an explicit value of 0 plus known collection coverage. Absent rows have an unknown cause: they may reflect no events, but absence alone cannot prove zero because existing counters can expose explicit zeros.
3. Threshold suppression: Cohorts with fewer than five distinct devices are merged into `andre` where configured, or dropped outright in reach, config, and unmerged session metrics.
4. Source or export failure: If Mimir returns no rows for a day, the job fails with an error and leaves BigQuery untouched ([mimir_daily.go](mimir_daily.go#L675-L677)), preventing an empty day from wiping history.

Live coverage limitation:
Source contracts define the expected schema, queries, and constraints. Live production coverage, historical gap status, and end-to-end table contents are not verified from source alone and require runtime queries.

Rerun missed days (inclusive):

```bash
MIMIR_DAILY_ENABLED=true GCP_TEAM_PROJECT_ID=copilot-prod-c697 BIGQUERY_DATASET=copilot_metrics_staging \
  go run . --mimir-daily --mimir-from 2026-10-01 --mimir-to 2026-10-05
```

Locally this needs naisdevice to reach `mimir.nav.cloud.nais.io`.

## Configuration

| Variable                     | Description                          | Default           |
| ---------------------------- | ------------------------------------ | ----------------- |
| `PORT`                       | HTTP server port                     | `8080`            |
| `LOG_LEVEL`                  | Log level (DEBUG, INFO, WARN, ERROR) | `INFO`            |
| `GITHUB_ENTERPRISE_SLUG`     | GitHub Enterprise slug               | `nav`             |
| `GITHUB_ORG`                 | GitHub organization                  | `navikt`          |
| `GITHUB_APP_ID`              | GitHub App ID                        | (required)        |
| `GITHUB_APP_PRIVATE_KEY`     | GitHub App private key (PEM)         | (required)        |
| `GITHUB_APP_INSTALLATION_ID` | GitHub App installation ID           | (required)        |
| `GITHUB_BILLING_TOKEN`       | Classic PAT with `admin:enterprise` for billing API | (optional) |
| `GCP_TEAM_PROJECT_ID`        | GCP project (from NAIS)              | (required)        |
| `BIGQUERY_DATASET`           | BigQuery dataset name                | `copilot_metrics` |
| `BIGQUERY_TABLE`             | BigQuery table name                  | `usage_metrics`   |
| `BIGQUERY_REPO_METRICS_TABLE` | Per-repository metrics table name   | `repository_metrics` |
| `SLACK_WEBHOOK_URL`          | Slack webhook for failure alerts     | (optional)        |

## BigQuery Schema

### `usage_metrics` table

| Column       | Type      | Description                    |
| ------------ | --------- | ------------------------------ |
| `day`        | DATE      | Calendar day of the metrics    |
| `scope`      | STRING    | `enterprise` or `organization` |
| `scope_id`   | STRING    | Enterprise/org identifier      |
| `raw_record` | JSON      | Full NDJSON record as-is       |
| `loaded_at`  | TIMESTAMP | When the row was inserted      |

Table is partitioned by `day` and clustered by `scope`, `scope_id`.

### `repository_metrics` table

Per-repository, PR-only Copilot lifecycle activity from the `repos-1-day`
Usage Metrics report (coding-agent authored/merged PRs and Copilot code-review
suggestions). One row per repository per day. Ingested as a supplementary report
alongside `users-1-day` / `user-teams-1-day`, so it flows through the nightly
gap-fill (last 7 days) and historical backfill automatically (data available
from GitHub's GA on 2026-07-17). Days older than the gap-fill window are filled
with `--repo-metrics-backfill`.

| Column       | Type      | Description                    |
| ------------ | --------- | ------------------------------ |
| `day`        | DATE      | Calendar day of the metrics    |
| `scope`      | STRING    | `enterprise` or `organization` (fetch source) |
| `scope_id`   | STRING    | Enterprise/org identifier      |
| `raw_record` | JSON      | Full per-repo NDJSON record as-is (includes `repo_id`, `repo_name`) |
| `loaded_at`  | TIMESTAMP | When the row was inserted      |

Table is partitioned by `day` and clustered by `scope`, `scope_id`.

### `billing_usage` table

Per-model premium request billing data from the Enhanced Billing API.

| Column         | Type      | Description                           |
| -------------- | --------- | ------------------------------------- |
| `day`          | DATE      | First day of the billing month        |
| `year`         | INTEGER   | Billing year                          |
| `month`        | INTEGER   | Billing month                         |
| `scope_id`     | STRING    | Enterprise slug                       |
| `product`      | STRING    | Product (e.g. `Copilot`)              |
| `sku`          | STRING    | SKU (e.g. `Copilot Premium Request`)  |
| `model`        | STRING    | Model name (e.g. `Claude Opus 4.7`)   |
| `unit_type`    | STRING    | Unit type (e.g. `requests`)           |
| `price_per_unit` | FLOAT   | Price per unit in USD                 |
| `gross_quantity` | FLOAT   | Total quantity before discounts       |
| `gross_amount` | FLOAT     | Total USD before discounts            |
| `net_quantity`  | FLOAT    | Billed quantity                       |
| `net_amount`   | FLOAT     | Billed USD                            |
| `raw_record`   | JSON      | Full API record                       |
| `loaded_at`    | TIMESTAMP | When the row was inserted             |

Table is partitioned by month and clustered by `scope_id`, `model`.

### `billing_usage_reports` table

Daily organization billing usage report rows from GitHub's billing usage API.

| Column | Type | Description |
| --- | --- | --- |
| `report_day` | DATE | Day covered by the report row |
| `organization` | STRING | Organization login |
| `repository_name` | STRING | Repository when row is repository-scoped |
| `product` | STRING | Product name |
| `sku` | STRING | SKU name |
| `quantity` | FLOAT | Quantity for the line item |
| `unit_type` | STRING | Unit type |
| `price_per_unit` | FLOAT | Price per unit in USD |
| `gross_amount` | FLOAT | Gross amount in USD |
| `discount_amount` | FLOAT | Discount amount in USD |
| `net_amount` | FLOAT | Net amount in USD |
| `raw_record` | JSON | Full API row as JSON |
| `loaded_at` | TIMESTAMP | Insert timestamp |

Table is partitioned by month (`report_day`) and clustered by `organization`, `product`, `sku`.

### `billing_usage_daily_model` table

Daily model-level premium request usage from the enhanced billing endpoint.

| Column | Type | Description |
| --- | --- | --- |
| `day` | DATE | Day for the usage line |
| `scope_id` | STRING | Enterprise slug |
| `product` | STRING | Product name |
| `sku` | STRING | SKU name |
| `model` | STRING | Model name |
| `unit_type` | STRING | Unit type |
| `price_per_unit` | FLOAT | Price per unit in USD |
| `gross_quantity` | FLOAT | Quantity before discounts |
| `discount_quantity` | FLOAT | Discounted quantity |
| `net_quantity` | FLOAT | Billed quantity |
| `gross_amount` | FLOAT | Gross amount in USD |
| `discount_amount` | FLOAT | Discount amount in USD |
| `net_amount` | FLOAT | Net amount in USD |
| `raw_record` | JSON | Full API row as JSON |
| `loaded_at` | TIMESTAMP | Insert timestamp |

Table is partitioned by day (`day`) and clustered by `scope_id`, `model`.

### `seat_counts` table

Daily Copilot seat totals from the billing seats API (enterprise first, org
fallback). Totals only, no per-person rows. One row per day and `scope_id`,
written with `MERGE`, so a re-run on the same day replaces the row. There is no
backfill: the API only reports the current state, so history starts on the day
this table was first deployed. `v_seat_counts_monthly` keeps the last row per month.

| Column                       | Type      | Description                          |
| ---------------------------- | --------- | ------------------------------------ |
| `snapshot_date`              | DATE      | Day the count was taken              |
| `scope`                      | STRING    | `enterprise` or `organization`       |
| `scope_id`                   | STRING    | Enterprise/org identifier            |
| `total_seats`                | INTEGER   | `total_seats` from the API           |
| `active_seats`               | INTEGER   | Seats not pending cancellation       |
| `pending_cancellation_seats` | INTEGER   | Seats with a pending cancellation    |
| `loaded_at`                  | TIMESTAMP | When the row was written             |

### `copilot_seats` table

Daily snapshot of every Copilot seat from the same seats API: `snapshot_date`,
`scope_id`, `user_id`, `login`, `plan_type`, `created_at` (when the current seat
was assigned) and `pending_cancellation_date` (the first day of the next billing
cycle, so the seat's last day is the day before). Partitioned by day. Written
with a load job that truncates the day's partition, so a re-run replaces the
day. Partitions expire after 400 days. No backfill. copilot-api reads it for the
licence estimate in the team year view.

### `team_members` table

Daily snapshot of every team's members in the org (`GET /orgs/{org}/teams` and
`/teams/{slug}/members`), regardless of Copilot use. Columns `date`, `team_slug`,
`login`. Partitioned by day; partitions expire after 400 days. A re-run replaces
the day with `MERGE`. Failures are logged and do not fail the job.

## GitHub App Permissions

The GitHub App requires:

- `enterprise_copilot_metrics: read` (for enterprise-level data)
- Or `organization_copilot_metrics: read` (fallback)
- Organization `members: read` (for `team_members`). The org installation must approve the new permission; until then each run logs a 403 and writes no `team_members` rows (#1566).

## Billing API Access

The premium request billing endpoint (`/enterprises/{enterprise}/settings/billing/premium_request/usage`) **cannot** be accessed by GitHub App tokens. It requires a classic Personal Access Token (PAT) with:

- `admin:enterprise` scope
- User must be an enterprise admin or billing manager

Set `GITHUB_BILLING_TOKEN` in the `copilot-metrics` secret to enable billing ingestion.

## Deployment

Deployed as a NAIS Job in the `copilot` namespace:

```bash
kubectl apply -f .nais/naisjob.yaml -f .nais/dev.yaml
```

Manual trigger:

```bash
kubectl create job --from=cronjob/copilot-metrics copilot-metrics-manual -n copilot
```

## Related

- [Issue #93](https://github.com/navikt/copilot/issues/93) - Migration tracking issue
- [GitHub Copilot Metrics API Documentation](https://docs.github.com/en/rest/copilot/copilot-usage)
