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
rtk bash -lc 'cd apps/copilot-metrics && rtk mise backfill:usage'

# Monthly billing only
rtk bash -lc 'cd apps/copilot-metrics && rtk mise backfill:billing-monthly'

# Daily billing usage reports only
rtk bash -lc 'cd apps/copilot-metrics && rtk mise backfill:billing-daily-report'

# Daily model billing usage only
rtk bash -lc 'cd apps/copilot-metrics && rtk mise backfill:billing-model-daily'

# Everything
rtk bash -lc 'cd apps/copilot-metrics && rtk mise backfill:all'
```

Prod variants:

```bash
rtk bash -lc 'cd apps/copilot-metrics && rtk mise backfill:usage:prod'
rtk bash -lc 'cd apps/copilot-metrics && rtk mise backfill:billing-monthly:prod'
rtk bash -lc 'cd apps/copilot-metrics && rtk mise backfill:billing-daily-report:prod'
rtk bash -lc 'cd apps/copilot-metrics && rtk mise backfill:billing-model-daily:prod'
rtk bash -lc 'cd apps/copilot-metrics && rtk mise backfill:all:prod'
```

### Nightly job (default)

Runs as a Kubernetes CronJob via NAIS. Automatically detects missing days in BigQuery and fills gaps:

```bash
rtk copilot-metrics --run-once
```

### Historical backfill

One-time operation to load historical data:

```bash
rtk copilot-metrics --backfill
rtk copilot-metrics --backfill --backfill-from=2025-10-10
```

### Billing usage backfill

One-time operation to load premium request billing data per model (requires `GITHUB_BILLING_TOKEN`):

```bash
rtk copilot-metrics --billing-monthly-backfill
rtk copilot-metrics --billing-monthly-backfill --billing-monthly-from=2025-01
rtk copilot-metrics --billing-monthly-backfill --billing-monthly-from=2025-01 --force
```

### Billing usage report backfill (daily rows)

One-time operation to load daily organization billing usage report rows (requires `GITHUB_BILLING_TOKEN`):

```bash
rtk copilot-metrics --billing-daily-report-backfill
rtk copilot-metrics --billing-daily-report-backfill --billing-daily-report-from=2025-10-10
rtk copilot-metrics --billing-daily-report-backfill --billing-daily-report-from=2025-10-10 --force
```

### Billing daily model backfill

One-time operation to load daily model-level premium request billing data (requires `GITHUB_BILLING_TOKEN`):

```bash
rtk copilot-metrics --billing-model-daily-backfill
rtk copilot-metrics --billing-model-daily-backfill --billing-model-daily-from=2025-10-10
rtk copilot-metrics --billing-model-daily-backfill --billing-model-daily-from=2025-10-10 --force
```

### Local development

```bash
export GITHUB_APP_ID=123456
export GITHUB_APP_PRIVATE_KEY="$(cat private-key.pem)"
export GITHUB_APP_INSTALLATION_ID=789
export GCP_TEAM_PROJECT_ID=my-project
export LOG_LEVEL=DEBUG

rtk go run . --run-once
```

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

## GitHub App Permissions

The GitHub App requires:

- `enterprise_copilot_metrics: read` (for enterprise-level data)
- Or `organization_copilot_metrics: read` (fallback)

## Billing API Access

The premium request billing endpoint (`/enterprises/{enterprise}/settings/billing/premium_request/usage`) **cannot** be accessed by GitHub App tokens. It requires a classic Personal Access Token (PAT) with:

- `admin:enterprise` scope
- User must be an enterprise admin or billing manager

Set `GITHUB_BILLING_TOKEN` in the `copilot-metrics` secret to enable billing ingestion.

## Deployment

Deployed as a NAIS Job in the `copilot` namespace:

```bash
rtk kubectl apply -f .nais/naisjob.yaml -f .nais/dev.yaml
```

Manual trigger:

```bash
rtk kubectl create job --from=cronjob/copilot-metrics copilot-metrics-manual -n copilot
```

## Related

- [Issue #93](https://github.com/navikt/copilot/issues/93) - Migration tracking issue
- [GitHub Copilot Metrics API Documentation](https://docs.github.com/en/rest/copilot/copilot-usage)
