# Repository-level Copilot usage metrics — integration design

> Design for adopting GitHub's GA **repository-level Copilot usage metrics**
> (`repos-1-day`) into the `copilot-metrics` → BigQuery → `copilot-api` →
> `my-copilot` pipeline. Grounds every choice in the existing code. Tracking
> issue: [#373](https://github.com/navikt/copilot/issues/373).

## 1. Summary & motivation

Today the pipeline ingests only **entity-level** Copilot reports —
`enterprise-1-day` / `organization-1-day` (aggregate), plus the supplementary
`users-1-day` and `user-teams-1-day` reports (see
`apps/copilot-metrics/github.go`). We can answer "how is Copilot used across
Nav" and "per user / per team", but **not** "which repositories benefit most".

On **2026-07-17** GitHub made **repository-level** usage metrics generally
available. These deliver per-repository, per-day pull-request lifecycle activity
— pull requests created and merged by the Copilot coding agent, and PRs reviewed
by Copilot code review (with suggestion counts broken down by comment type).

Repo-level granularity unlocks:

- **Per-repo adoption & impact** — rank repos by Copilot-authored/-reviewed PRs
  and by median time-to-merge for Copilot-authored PRs.
- **"Which repos benefit most"** — correlate Copilot PR activity with delivery
  velocity to target enablement where it pays off.
- **Coding-agent & code-review footprint** — the only place these two surfaces
  are attributed to a concrete repository.
- A natural drill-down under the existing team/adoption views on the
  **Statistikk** page (`apps/my-copilot/src/app/statistikk/page.tsx`).

Sources:
[changelog](https://github.blog/changelog/2026-07-17-repository-level-github-copilot-usage-metrics-generally-available/),
[REST reference](https://docs.github.com/en/rest/copilot/copilot-usage-metrics),
[Copilot usage-metrics reference](https://docs.github.com/en/copilot/reference/copilot-usage-metrics/copilot-usage-metrics).

## 2. The API

### Endpoints & report-type slug

Two endpoints, same report-type slug `repos-1-day`, available at **both**
organization and enterprise scope (this mirrors every other report the pipeline
already fetches in `github.go`):

```
GET https://api.github.com/enterprises/{enterprise}/copilot/metrics/reports/repos-1-day?day=YYYY-MM-DD
GET https://api.github.com/orgs/{org}/copilot/metrics/reports/repos-1-day?day=YYYY-MM-DD
```

- **Query param:** `day` (`YYYY-MM-DD`), exactly like `enterprise-1-day` /
  `users-1-day` in `fetchDailyReport` (`github.go:444`).
- **Response envelope:** identical to the existing reports — a JSON object with
  `download_links` (array of pre-signed URLs) and `report_day`. This is already
  modelled by `MetricsReportResponse` (`github.go:30-33`); each download link is
  an **NDJSON** file with one record per repository, parsed by
  `downloadAndParseNDJSON` (`github.go:280`). No new transport code is required.
- **API version header:** reuse the value the code already sends for metrics
  reports — `X-GitHub-Api-Version: 2026-03-10` (`github.go:224`).

### Per-repository record schema (`repos-1-day`)

Each NDJSON record (field paths, to be extracted with `JSON_VALUE` in a view):

| Field path | Type | Notes |
| --- | --- | --- |
| `day` | string (date) | Report day |
| `enterprise_id` | string | Present on enterprise-scope reports |
| `organization_id` | string | |
| `repo_id` | integer | Stable repository identifier |
| `repo_owner_name` | string | Org/owner login |
| `repo_name` | string | Repository name |
| `repo_visibility` | string | `PRIVATE` \| `INTERNAL` \| `PUBLIC` |
| `pull_requests.total_created` | integer | |
| `pull_requests.total_reviewed` | integer | |
| `pull_requests.total_merged` | integer | |
| `pull_requests.median_minutes_to_merge` | number (nullable) | |
| `pull_requests.total_suggestions` | integer | All PR review suggestions |
| `pull_requests.total_applied_suggestions` | integer | |
| `pull_requests.total_created_by_copilot` | integer | Coding-agent authored |
| `pull_requests.total_reviewed_by_copilot` | integer | Copilot code review |
| `pull_requests.total_merged_created_by_copilot` | integer | |
| `pull_requests.total_merged_reviewed_by_copilot` | integer | |
| `pull_requests.median_minutes_to_merge_copilot_authored` | number (nullable) | |
| `pull_requests.median_minutes_to_merge_copilot_reviewed` | number (nullable) | |
| `pull_requests.total_copilot_suggestions` | integer | |
| `pull_requests.total_copilot_applied_suggestions` | integer | |
| `pull_requests.copilot_suggestions_by_comment_type[]` | array | `{ comment_type, total_copilot_suggestions, total_copilot_applied_suggestions }` |

### How repo-level differs from org/enterprise reports

- **PR-only.** The report contains **only** pull-request lifecycle activity
  (coding agent + code review). It has **no** IDE/chat/completion activity, no
  `daily_active_users`, no `totals_by_ide` / `totals_by_language_*` /
  `totals_by_cli` breakdowns that `v_daily_summary.sql` reads from the entity
  report. Notably the `pull_requests.*` sub-object is the **same shape** already
  extracted from the entity report in `v_daily_summary.sql:19-33` — so the field
  mapping is a known quantity.
- **One row per repository per day** (vs. one aggregate row per day for the
  entity report), i.e. thousands of rows/day for an org the size of `navikt`.
- The report "can contain data even when IDE usage metrics are absent" — so it
  behaves like the supplementary reports (may lag, must not fail the main job).

### Permissions, availability, limits

- **Access:** enterprise owner / billing manager, org owner, or a custom
  org/enterprise role with **"View Copilot Metrics"**; the Copilot usage-metrics
  policy must be enabled. For the GitHub App this is the same permission the job
  already relies on — `enterprise_copilot_metrics: read` (enterprise) or
  `organization_copilot_metrics: read` (org fallback), see
  `apps/copilot-metrics/README.md:222-227`. The existing dual-installation setup
  (`GitHubAppOrgInstallationID`, `github.go:66-83`) already handles the org path.
- **Availability window:** GA on 2026-07-17. The earliest fetchable `day` is not
  documented; treat "no data before GA" as expected and rely on the existing
  `ErrReportNotAvailable` handling (`github.go:18-20,182-190`) rather than a hard
  start date.
- **Rate limits:** not separately documented; the standard REST budget applies.
  The backfill loop already paces at 1 req/s (`backfill.go:11-13,88`), which is
  sufficient — repo-level adds one report per day fetched.

> Open verification (see §9): the exact field names/nullability and the earliest
> available `day` must be confirmed against a live response; the docs pages do
> not fully enumerate nullability or the historical window.

## 3. Ingestion design

### Fetch — follow the existing `github.go` pattern

`github.go` already has a generic `fetchDailyReport(ctx, day, reportType)`
(`github.go:444`) that does enterprise-first-then-org fallback and returns a
`FetchResult{Records, Scope, ScopeID}`. Repo-level fetch is a one-liner on top
of it, exactly like `FetchDailyUserMetrics` (`github.go:439-441`):

```go
// FetchDailyRepoMetrics fetches the repos-1-day report for the given day.
func (c *GitHubClient) FetchDailyRepoMetrics(ctx context.Context, day time.Time) (*FetchResult, error) {
    return c.fetchDailyReport(ctx, day, "repos-1-day")
}
```

Add `FetchDailyRepoMetrics` to the `MetricsFetcher` interface
(`apps/copilot-metrics/interfaces.go`) so `main.go`/tests can use it.

### BigQuery table strategy — **new table `repository_metrics`** (recommended)

**Recommendation: add a dedicated raw-JSON table `repository_metrics`, reusing
the exact 5-column schema, rather than adding a `repository` value to `scope` in
`usage_metrics`.**

Rationale, grounded in the code:

1. **`scope` already has a fixed meaning.** In `bigquery.go:84` the column is
   documented as `"enterprise or organization"` — it identifies the *entity we
   fetched from*, not the record granularity. Every view and the k-anonymity
   query filter on `scope = 'enterprise'` / `scope = 'organization'`
   (`v_daily_summary.sql`, `bigquery_stats.go:932`). Overloading `scope` with
   `repository` would silently change those semantics and risk double-counting.
2. **Row-shape and volume differ.** `usage_metrics` holds one aggregate row/day;
   repo-level is thousands of rows/day with its own `repo_id` identity. Keeping
   them apart preserves partition/cluster efficiency and keeps
   `v_daily_summary`'s `FROM usage_metrics` scan cheap.
3. **There is already a precedent.** The supplementary reports live in their own
   tables — `user_teams`, `user_metrics` (`config.go:23-24,42-43`) — created by
   the same generic `ensureMetricsTable` helper (`bigquery.go:70`). Repo-level
   is the same kind of per-entity NDJSON report and should follow suit.

For `repository_metrics`, the standard schema fits unchanged: `day`, `scope`
(`enterprise`/`organization` — the fetch source), `scope_id` (`nav`/`navikt`),
`raw_record` (the per-repo JSON, incl. `repo_id`/`repo_name`), `loaded_at`;
partition by `day`, cluster by `scope`,`scope_id` (`bigquery.go:82-99`). Repo
identity lives inside `raw_record`, exactly as user identity does in
`user_metrics`.

Concretely:

- **config.go:** add `BigQueryRepoMetricsTable` with
  `getEnv("BIGQUERY_REPO_METRICS_TABLE", "repository_metrics")`
  (`config.go:23-24,42-43`).
- **bigquery.go:** add field `repoMetricsTable`; add
  `EnsureRepoMetricsTableExists` → `ensureMetricsTable(ctx, c.repoMetricsTable,
  "GitHub Copilot per-repository usage metrics from the Usage Metrics API")`;
  add `InsertRepoMetrics`, `RepoMetricsDayExists`, `DeleteRepoMetricsDay` as
  thin wrappers over the existing `insertRecords` / `dayExistsInTable` /
  `deleteDayFromTable` (`bigquery.go:110-218`). No new SQL primitives.

### Idempotency, gap-fill, backfill

Repo-level is a lagging, PR-only supplementary report — treat it exactly like
`users-1-day` / `user-teams-1-day`:

- **Wire into `ingestSupplementary`** (`main.go:399`) using the existing
  `upsertReport` helper (`main.go:524`), which does check-exists → delete →
  insert keyed on `(day, scope_id)` — the same per-day idempotency the streaming-
  buffer-aware `ErrStreamingBuffer` path already handles (`bigquery.go:16-18`).
- **Gap-fill** via `ingestMissingSupplementary` (`main.go:457`): add a
  `RepoMetricsDayExists` check over the same 7-day lookback so days that had no
  repo report at first ingest get filled once GitHub generates it.
- **Backfill is free.** `runBackfill` → `ingestDay` → `ingestSupplementary`
  (`backfill.go:61`, `main.go:391`), so `backfill:usage` will pull repo-level
  history automatically. Because data starts at GA, `ErrReportNotAvailable`
  simply skips earlier days. Optionally add a dedicated
  `backfill:repos` mise task mirroring `backfill:usage`
  (`apps/copilot-metrics/.mise.toml:92`) if a repo-only re-run is ever needed;
  not required for the first release.

Add `EnsureRepoMetricsTableExists` to `main.go` startup alongside the other
`Ensure*TableExists` calls (`main.go:71-84`).

## 4. Views

Add one view, `v_repository_usage`, registered in `views.go`.

- **Register:** append `{name: "v_repository_usage", filename:
  "views/v_repository_usage.sql"}` to the `views` slice (`views.go:19-31`), and
  add a `{{repository_metrics}}` placeholder substitution in
  `createOrReplaceView` next to `{{user_teams}}` / `{{user_metrics}}`
  (`views.go:71-75`):

  ```go
  repoMetricsRef := fmt.Sprintf("`%s.%s.%s`", c.projectID, c.dataset, c.repoMetricsTable)
  sql = strings.ReplaceAll(sql, "{{repository_metrics}}", repoMetricsRef)
  ```

- **`views/v_repository_usage.sql`** — mirrors the `pull_requests.*` extraction
  already proven in `v_daily_summary.sql:19-33`, but per repo:

  ```sql
  CREATE OR REPLACE VIEW `%s.%s.v_repository_usage` AS
  SELECT
    day,
    scope,
    scope_id,
    CAST(JSON_VALUE(raw_record, '$.repo_id') AS INT64)       AS repo_id,
    JSON_VALUE(raw_record, '$.repo_owner_name')              AS repo_owner,
    JSON_VALUE(raw_record, '$.repo_name')                    AS repo_name,
    JSON_VALUE(raw_record, '$.repo_visibility')              AS repo_visibility,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_created')                       AS INT64) AS pr_total_created,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_merged')                        AS INT64) AS pr_total_merged,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_reviewed')                      AS INT64) AS pr_total_reviewed,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_created_by_copilot')            AS INT64) AS pr_created_by_copilot,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_reviewed_by_copilot')           AS INT64) AS pr_reviewed_by_copilot,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_merged_created_by_copilot')     AS INT64) AS pr_merged_copilot_authored,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_merged_reviewed_by_copilot')    AS INT64) AS pr_merged_copilot_reviewed,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.median_minutes_to_merge')             AS FLOAT64) AS pr_median_minutes_to_merge,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.median_minutes_to_merge_copilot_authored') AS FLOAT64) AS pr_median_minutes_to_merge_copilot,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_copilot_suggestions')           AS INT64) AS pr_copilot_suggestions,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_copilot_applied_suggestions')   AS INT64) AS pr_copilot_applied_suggestions
  FROM {{repository_metrics}}
  -- Deduplicate against the enterprise/org fallback: prefer enterprise rows.
  -- (One row per repo per day per scope.)
  ORDER BY day DESC, pr_created_by_copilot DESC;
  ```

  Note: because the fetch falls back enterprise→org, a day is stored under a
  single scope, so no cross-scope dedup is needed in practice — but querying
  callers should still pin `scope = 'enterprise'` first (as
  `bigquery_stats.go:932` does) for consistency.

A second aggregate view (e.g. `v_repository_usage_28d`, top-N repos by
Copilot-authored PRs over a trailing window) can be added later; the base view
is enough for phase 1.

## 5. copilot-api — read endpoint & stats struct

Follow the `team-summary` path end to end (`bigquery_stats.go:163`,
`bigquery_handlers.go:114`, `handlers.go:81`, `bigquery.go:530`):

- **Struct** (`bigquery_stats.go`, near `TeamUsageSummary`):

  ```go
  type RepositoryUsageSummary struct {
      RepoName                 string  `bigquery:"repo_name" json:"repo_name"`
      RepoVisibility           string  `bigquery:"repo_visibility" json:"repo_visibility"`
      PRCreatedByCopilot       int64   `bigquery:"pr_created_by_copilot" json:"pr_created_by_copilot"`
      PRReviewedByCopilot      int64   `bigquery:"pr_reviewed_by_copilot" json:"pr_reviewed_by_copilot"`
      PRMergedCopilotAuthored  int64   `bigquery:"pr_merged_copilot_authored" json:"pr_merged_copilot_authored"`
      CopilotSuggestions       int64   `bigquery:"copilot_suggestions" json:"copilot_suggestions"`
      CopilotAppliedSuggestions int64  `bigquery:"copilot_applied_suggestions" json:"copilot_applied_suggestions"`
      MedianMinutesToMergeCopilot *float64 `bigquery:"pr_median_minutes_to_merge_copilot" json:"pr_median_minutes_to_merge_copilot"`
      DaysWithData             int64   `bigquery:"days_with_data" json:"days_with_data"`
  }
  ```

- **Query method** `GetRepositoryUsage(ctx, days int) ([]RepositoryUsageSummary,
  error)` on `*BigQueryClient`, reading `viewRef(bq.metricsDataset,
  "v_repository_usage")` (`bigquery.go:174`), summing the copilot PR columns
  grouped by `repo_name`, filtered to a trailing `days` window — the same shape
  as `GetTeamUsageSummary` (`bigquery_stats.go:163`). Apply the privacy filter
  from §7 inside this query.
- **Cache passthrough:** add `GetRepositoryUsage` to the `BigQueryQuerier`
  interface and to `CachedBigQueryClient` (`bigquery.go:530`) so the 1 h backend
  cache covers it.
- **Handler** `handleRepositoryUsage` (`bigquery_handlers.go`), copying
  `handleTeamUsageSummary`: `requireMethod` GET, optional `days` param, `cacheControl(w, 3600, false)`, `respondJSON`.
- **Route** in `makeAPIRouter` (`handlers.go:81`):

  ```go
  mux.HandleFunc("GET /api/v1/copilot/usage/repositories", bq(nilSafe(bqHandlers, func(h *BigQueryHandlers) http.HandlerFunc { return h.handleRepositoryUsage })))
  ```

## 6. my-copilot dashboard

Surface repo-level data on the existing **Statistikk** page
(`apps/my-copilot/src/app/statistikk/page.tsx`), which already composes tabbed
sections and a `TeamUsageTable`.

- **Types** (`apps/my-copilot/src/lib/types.ts`): add `RepositoryUsageSummary`
  mirroring the Go struct (and keep the existing privacy note style at
  `types.ts:445`).
- **Fetcher** (`apps/my-copilot/src/lib/cached-bigquery.ts`): add
  `getRepositoryUsage(token)` using `fetchWithFallback` and `backendRequest(
  "/api/v1/copilot/usage/repositories", token)` — a direct copy of
  `getTeamUsage` (`cached-bigquery.ts:109-117`).
- **UI:** a new "Repositorier" section/tab with a sortable
  `RepositoryUsageTable` (clone of `team-usage-table`) plus 2–3 `MetricCard`s
  (total Copilot-authored PRs, total Copilot-reviewed PRs, applied-suggestion
  rate). Default sort: Copilot-authored PRs desc — the "which repos benefit
  most" view. Render inside a `<Suspense>` boundary like the other cached data
  blocks on the page.

UX sketch:

```
Statistikk ▸ [ Oversikt | Team | Repositorier ]
 Repositorier
  ┌ Copilot-PR-er ─────┐ ┌ Copilot-reviews ──┐ ┌ Applied-rate ─┐
  │       1 240        │ │        980        │ │     62 %      │
  └────────────────────┘ └───────────────────┘ └───────────────┘
  Repo                    Copilot-PR  Merged  Reviews  Median→merge
  navikt/foo                    142     130      88      3t 12m
  navikt/bar                     97      90      54      5t 40m
  … (repos below privacy threshold hidden, see note)
```

## 7. Privacy & aggregation

The existing guard is `minUsersForDistribution = 5` in
`bigquery_stats.go:865-867`, enforced in `GetUsageDistribution`
(`bigquery_stats.go:906-910`) and mirrored client-side in
`UsageDistributionChart.tsx:47-52` and documented at `types.ts:445`. It suppresses
aggregates — and even the exact small count — when too few users could be
re-identified.

Repo-level data creates a **new** re-identification risk the current guard does
not cover: a repository with only one or two contributors makes
`total_created_by_copilot` effectively attributable to a named individual, and
`repo_name` itself can be sensitive for private repos. The report gives **no
distinct-author count**, so we cannot apply the k=5 rule directly.

**Recommendations (defence in depth):**

1. **Exclude private repos from any user-facing surface.** Filter the view/query
   to `repo_visibility IN ('PUBLIC','INTERNAL')`. Private-repo activity is the
   most likely to map to a small, identifiable team.
2. **Suppress low-activity repos.** Only expose repos whose trailing-window
   `pr_total_created` (all authors, not just Copilot) is `>= minReposActivity`
   (start at **5**, matching the existing k). Below that, a single person's
   behaviour dominates the numbers. Encode this as a named constant next to
   `minUsersForDistribution` so the two thresholds stay visibly related.
3. **Aggregate over a window, never per-day per-repo.** The `/repositories`
   endpoint should return trailing-window sums (like `team-summary`), not raw
   daily rows, so one active day can't be singled out.
4. **Keep it behind the same auth** as the other `/api/v1/copilot/usage/*`
   BigQuery endpoints (`handlers.go`) — no public/unauthenticated exposure.
5. **Mirror the guard client-side** with the same "For få …" empty-state
   treatment used in `UsageDistributionChart.tsx`, and add a privacy note in
   `types.ts` next to the new type.

The raw `repository_metrics` table itself keeps full fidelity (private repos
included) for authorised BigQuery analysis; suppression happens only in the
view/API layer that feeds the dashboard — the same split the pipeline already
uses for user-level data.

## 8. Phased implementation plan

Ordering follows the data flow: ingestion → views → api → dashboard, with
backfill folded into phase 1. Estimates are rough dev-days.

| Phase | Scope | Key files | Est. | Depends on |
| --- | --- | --- | --- | --- |
| **1. Ingestion** | `FetchDailyRepoMetrics`; `repository_metrics` table + `Ensure/Insert/Exists/Delete` wrappers; wire into `ingestSupplementary` + `ingestMissingSupplementary`; `main.go` startup; config env; unit tests | `github.go`, `bigquery.go`, `config.go`, `main.go`, `interfaces.go` | 1.5–2 | — |
| **2. Backfill verify** | Confirm `backfill:usage` pulls repo history; confirm earliest `day`; optional `backfill:repos` mise task; README table update | `.mise.toml`, `README.md` | 0.5 | 1 |
| **3. View** | `v_repository_usage.sql` + register + `{{repository_metrics}}` placeholder; `views_test.go` | `views.go`, `views/v_repository_usage.sql` | 0.5–1 | 1 |
| **4. API** | `RepositoryUsageSummary`, `GetRepositoryUsage` (+ privacy filter), handler, route, interface + cached client, tests | `bigquery_stats.go`, `bigquery_handlers.go`, `handlers.go`, `bigquery.go` | 1.5 | 3 |
| **5. Privacy hardening** | Named thresholds, private-repo exclusion, window-only aggregation; tests asserting suppression | `bigquery_stats.go` (+ client mirror) | 0.5–1 | 4 |
| **6. Dashboard** | Type, fetcher, `RepositoryUsageTable`, MetricCards, tab on Statistikk | `types.ts`, `cached-bigquery.ts`, `statistikk/page.tsx`, new component | 1.5–2 | 4 |

**Total ≈ 6–8 dev-days.** Phases 1–3 are independently shippable (data lands in
BigQuery and is queryable) before any API/UI work.

## 9. Risks & open questions

- **Schema fidelity (live verification):** the exact field names, nullability,
  and whether `enterprise_id`/`organization_id` are always present must be
  confirmed against a real `repos-1-day` response before finalising the view
  casts. The docs pages don't fully enumerate these.
- **Historical window:** earliest fetchable `day` is undocumented; backfill must
  tolerate `ErrReportNotAvailable` for pre-GA days (it already does).
- **Enterprise vs org coverage:** verify the enterprise report actually returns
  all `navikt` repos (and that the org fallback isn't silently narrower) using
  the existing dual-installation tokens (`github.go:66-83`).
- **Volume / cost:** thousands of repo rows/day — confirm partition+cluster keeps
  `v_repository_usage` scans cheap; consider a materialised 28-day rollup if the
  dashboard query gets expensive.
- **Privacy threshold value (product decision):** is k=5 on PR count the right
  bar, and should private repos be fully excluded or shown only to a narrower
  audience? Needs a call from the product/privacy owner before phase 6 ships.
- **Coding-agent attribution:** `total_created_by_copilot` counts coding-agent
  PRs — confirm this is what we want to badge as "Copilot benefit" vs.
  human-authored-with-Copilot-assist, which this report does **not** capture.

## References

- Changelog — repository-level metrics GA:
  https://github.blog/changelog/2026-07-17-repository-level-github-copilot-usage-metrics-generally-available/
- REST API — Copilot usage metrics:
  https://docs.github.com/en/rest/copilot/copilot-usage-metrics
- Copilot usage-metrics reference (report schemas):
  https://docs.github.com/en/copilot/reference/copilot-usage-metrics/copilot-usage-metrics
- Existing pipeline: `apps/copilot-metrics/README.md`, `docs/bigquery-export-plan.md`
- Tracking issue: [#373](https://github.com/navikt/copilot/issues/373)
