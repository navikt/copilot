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

- **Nightly ingestion**: Fetches yesterday's metrics daily at 06:00 UTC
- **Enterprise-level data**: Uses enterprise API for richer data (CLI usage, agent modes, LoC metrics)
- **Fallback to org-level**: Gracefully degrades to organization API if enterprise fails
- **Historical backfill**: CLI flag to load historical data from Oct 10, 2025
- **Idempotent**: Re-runs delete and re-insert data for the same day
- **Raw JSON storage**: Schema changes in GitHub API don't break the pipeline

## Usage

### Nightly job (default)

Runs as a Kubernetes CronJob via NAIS:

```bash
copilot-metrics --run-once
```

### Historical backfill

One-time operation to load historical data:

```bash
copilot-metrics --backfill
copilot-metrics --backfill --backfill-from=2025-10-10
```

### Local development

```bash
export GITHUB_APP_ID=123456
export GITHUB_APP_PRIVATE_KEY="$(cat private-key.pem)"
export GITHUB_APP_INSTALLATION_ID=789
export GCP_TEAM_PROJECT_ID=my-project
export LOG_LEVEL=DEBUG

go run . --run-once
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
| `GCP_TEAM_PROJECT_ID`        | GCP project (from NAIS)              | (required)        |
| `BIGQUERY_DATASET`           | BigQuery dataset name                | `copilot_metrics` |
| `BIGQUERY_TABLE`             | BigQuery table name                  | `usage_metrics`   |

## BigQuery Schema

| Column       | Type      | Description                    |
| ------------ | --------- | ------------------------------ |
| `day`        | DATE      | Calendar day of the metrics    |
| `scope`      | STRING    | `enterprise` or `organization` |
| `scope_id`   | STRING    | Enterprise/org identifier      |
| `raw_record` | JSON      | Full NDJSON record as-is       |
| `loaded_at`  | TIMESTAMP | When the row was inserted      |

Table is partitioned by `day` and clustered by `scope`, `scope_id`.

## GitHub App Permissions

The GitHub App requires:

- `enterprise_copilot_metrics: read` (for enterprise-level data)
- Or `organization_copilot_metrics: read` (fallback)

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
