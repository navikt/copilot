# Team insight handoff

## Updated status (2026-10-03)

The original handoff below describes the start of this session. This section
supersedes its implementation status and next steps.

- PR #1419 is pushed at `842e6c4f`, with green CI and successful dev deployments. Still draft and unmerged. It closes #1421.
- Enabled `--user-billing-sync` runs separately at 08:00 UTC prod and 09:00 UTC dev using the existing secret. It discovers closed months from October 2026; first collection November 1. Checkpointed, bounded, identity- and source-validated. Correction refresh remains deferred.
- Daily user/team replacements and coverage receipts are atomic. Invalid weights, partial downloads and ambiguous empty-link reports cannot certify completeness.
- Exact GPT-6 Astra's final focused correction review is CLEAN. Broader legacy resilience findings remain in #1422.
- August dev completed after resolving the remaining nine accounts through immutable GitHub IDs and validated atomic writes. All 728 source users have checkpoints, no duplicates or missing source users. Known net $32,163.25; enterprise net $32,295.17; residual $131.92. Live API test confirms August/September net availability with 113/119 visible teams. Production August and older history remain in #1425; no production writes were made.
- #1423 tracks safe billing-correction refresh; #1424 tracks remaining team analysis and signed-in/accessibility validation. #946 owns authoritative segment/cost-center allocation.
- Signed-in browser verification remains unavailable here. Unauthenticated dev access redirects to sign-in; local navigation/table tests pass.
- See `docs/copilot-team-spend-research.md` for current evidence and issue ownership. Do not dispatch new production writes or merge without a clear request.

## Start here

- Repo: `navikt/copilot`; branch `feat/team-spend-insight`.
- Draft PR: https://github.com/navikt/copilot/pull/1419
- Latest pushed commit: `50f80178` (`fix(team): default to current month and simplify navigation`). Check status and PR checks again before working; do not assume this remains HEAD.
- Working record: `docs/copilot-team-spend-research.md`. It is chronological; some older findings and plans are superseded by later decisions.
- Next requested work: fill historical per-user net billing, starting with **August 2026**, so September can show a comparable month-over-month change. The last user message before requesting this handoff asked what needs backfilling, not to launch another production job.

Read root `AGENTS.md` and `SECURITY.md`. Do not merge or dispatch more production writes without a clear request. Commits and pushes to the existing PR were authorized during the session; verify the next session's scope.

## Agreed product rules

- Goal: transparent Copilot consumption and cost tied to useful work, not a leaderboard or individual shaming.
- Dedicated private page `/innsikt/team`, linked from public `/innsikt`. The former `/statistikk` team tab is removed; `/statistikk#team` redirects through `legacy-anchors.ts`.
- All signed-in users may browse team aggregates. No cross-user spending details.
- Exclude only GitHub's catch-all `nav-it-github-users`.
- Each eligible team shows the **full charge of each member on their usage days**. Multi-team members appear in each team. Team rows overlap and must never be summed as Nav's bill.
- Suppress named teams with fewer than five contributors for the selected month. Gross mode counts positive gross contributors; net mode counts positive estimated net contributors. The hidden-team union is deduplicated by user/day and may overlap visible teams.
- Seats/licenses remain outside team consumption. Include Copilot AI Credits and Copilot Cloud Agent; not Code Quality.
- Residual billing and genuine missing memberships are acceptable, shown separately, not spread across teams.
- Personal current-team discovery is a navigation hint, not historical ownership.

## Current implementation

### Backend

`apps/copilot-api/team_spend.go`:

- `GET /api/v1/copilot/usage/team-gross?month=YYYY-MM`: full-member gross from `user_metrics.ai_credits_used * $0.01`, same-day `user_teams`, SQL suppression and deduplicated hidden bucket.
- `GET /api/v1/copilot/usage/team-net?month=YYYY-MM`: serves closed months only with `billing_user_monthly_runs.status='complete'`. Monthly user net is weighted across that user's daily gross usage, then shown in every eligible team on each day. Timing is an estimate; no daily billed net is claimed. Returns null for absent tables/completed months.
- `GET /api/v1/copilot/usage/my-teams`: mandatory identity resolution through existing SAML chain, matching the caller's login on the latest enterprise membership day. Historical-login-to-ID fallback was removed after Astra identified login-reuse disclosure. No trusted current immutable-ID resolution is implemented yet.
- Distinct known-user net plus signed enterprise residual reconciles to enterprise billing, independently of overlapping team rows.

### Ingestion

`apps/copilot-metrics/{billing.go,billing_user.go,billing_user_bigquery.go}`:

- `--user-billing-month=YYYY-MM` is a separate manual **closed-month** mode. It only needs `GCP_TEAM_PROJECT_ID` and `GITHUB_BILLING_TOKEN`, not GitHub App credentials.
- Source users: monthly enterprise `user_metrics`/`user_teams`, resolved by GitHub user ID with team-report login preferred. Source does not include extra current-seat-only users from the original research census.
- Fetches enterprise `ai_credit/usage?user=...`; validates top-level user, enterprise, year/month, day=0 and non-null `usageItems` before writes.
- Aggregates per-user SKU/model items into user/SKU totals. Atomic transaction replaces that user's month rows and `sku='done'` marker. Resume skips done users. Month marker is written only after every seeded user succeeds.
- BigQuery STRUCT parameters use `bigquery` tag names; SQL must select `month,scope_id,user_id,github_login,sku,gross_amount,net_amount,loaded_at` from `UNNEST(@rows)`, not Go field names.
- Two-second pauses, sequential requests, stops on request error. The existing scheduled job remains `--run-once`, **does not load user billing**, and has a one-hour deadline. Do not trigger it expecting a billing backfill.
- Completed months currently exit immediately. No safe correction refresh, ongoing monthly schedule, concurrent-run lock or full source-completeness monitor is implemented. Do not delete markers ad hoc to refresh a month.

### Frontend

`apps/my-copilot/src/app/(nb)/innsikt/team/page.tsx`, `components/team-{gross-usage,month-picker}.tsx`:

- Default is **current UTC month**, preserving an explicit valid `?month=` between May 2026 and current month. August in a URL remains August.
- Month control: Aksel select with Norwegian names and Vis button. Previous/next text links were removed at the user's request. Picker is outside the async data component, so empty/error months remain navigable; keyed input resets after client navigation.
- Own teams first, other teams alphabetic by default, search and sorting on every column.
- Labels: Team, Medlemmer, Forbruk (net) or Forbruk før fradrag (gross), Per medlem, Endring. Medlemmer counts contributors with consumption, not full roster. Per medlem = row amount / those contributors.
- Short `Om kostnadene` explains total per team, not an average, and overlap. Technical reconciliation prose removed from the visible page; API still exposes totals.
- Red increases/green decreases only at **at least 10 USD AND 10%** versus prior amount. Prior zero with at least 10 USD change highlights. Signed text remains visible. Threshold is provisional for user review, not a value/performance score.
- Comparison requires both full closed months, same gross/net basis, and a visible suppressed-safe row in both. September currently has net but August only gross, so it shows a dash with one reason. October does not compare partial consumption with a full month.

## Data and access

Projects: dev `copilot-dev-e17a`, prod `copilot-prod-c697`; dataset `copilot_metrics`.

**September 2026 completed in both projects**:

| SKU | Known-user net USD | Enterprise net USD | Residual USD |
| --- | ---: | ---: | ---: |
| Copilot AI Credits | 64,127.05 | 64,297.57 | 170.51 |
| Copilot Cloud Agent | 261.28 | 261.28 | 0.00 |
| Combined | 64,388.33 | 64,558.85 | 170.51 |

759 seeded GitHub IDs with done markers; 683 AI Credit SKU rows and 125 Cloud Agent rows. Distinct gross $79,489.15. Thirty September days of usage/team/billing source data in both projects. Corrected thresholds: 122 visible gross teams/55 hidden; 119 visible net teams/58 hidden. A completed full research census of 823 logins, including current seats, had the same residual.

Prod was explicitly authorized and ran as a detached **local binary**, not Naisjob. PID 35535 was the process; it finished. Temporary helper/log under `/var/folders/8q/l5h25q1s1tn8wp0p_d2nc4qc0000gn/T/opencode/`. Both backfills used the interactive `gh` token and local ADC, not the pod's PAT. They ran before response validation/atomic replacement fixes; those fixes were subsequently tested prospectively, not by silently overwriting September.

At the last attempted live query, gcloud could not refresh and required interactive `gcloud auth login`. Check application-default access too before using the Go client. Do not automate interactive authentication or print tokens.

Manual command from `apps/copilot-metrics`, once access is restored and execution approved:

```sh
GCP_TEAM_PROJECT_ID=copilot-dev-e17a \
GITHUB_BILLING_TOKEN="$(gh auth token)" \
go run . --user-billing-month=2026-08
```

For prod use `copilot-prod-c697`. Load **August, July, June** one at a time, verifying all source-day coverage first. May membership reports begin May 6, so May is not fully attributable from same-day team data. Before June, verify historical SKU/billing availability; the new backfill filters only the two AI-credit-era Copilot SKUs. Earlier premium-request pricing is not automatically supported. October remains partial and the command refuses it until November.

Every backfill must verify source users, done markers, completed-month status, no duplicate `(month,scope_id,user_id,sku)` and totals/residual by SKU. Invalid/failed users must not be marked done. Long dev runs exceeded one hour due to per-user BigQuery work; persisted checkpoints allowed resume. Avoid repeated full API censuses and watch request limits. The production job credential remains unverified.

## Reviews and verification

- Draft PR #1419; never marked ready or merged.
- Latest completed CI before navigation-only change was green, including `ci-ok`, frontend, Go, link guard and dev deployments. Recheck checks for latest HEAD; no permanent current-CI claim.
- Two GitHub review comments were **CodeQL**, not Copilot-authored, on handlers logging query errors. Fixed by logging constant messages and replies posted to both threads.
- User explicitly asked for GPT-6 Astra. An external Copilot CLI review confirmed `currentModel='gpt-6-astra'`. Initial partial review **BLOCK** found missing billing envelope validation, historical login reuse, stale SKUs on resume and picker disappearance. All addressed.
- Focused follow-up Astra **CONCERNS**, no confirmed blocker, flagged picker stale default and transaction evidence. Fixed with keyed picker, atomic production SQL and rollback test. Initial broader review was incomplete; do not claim independent full clearance. Evidence directories under approved temp dir: `astra-review-1419-97148fb0/` and `astra-focused-1419-current/`.
- `mise check` passed both Go apps after review fixes.
- Live opt-in API test: `VERIFY_TEAM_SPEND_BIGQUERY=true mise test:short` in `apps/copilot-api`, checks actual dev queries, Go row decoder and cached HTTP handler through httptest. Not a live authenticated Azure OBO browser session.
- Live transactional SQL test: `VERIFY_TEAM_SPEND_BIGQUERY=true go test -run '^TestUserBillingSnapshotTransaction$' -count=1 -v` in metrics, against dev **temporary table**, including rollback injection. ADC sometimes returned EOF/timeout before succeeding.
- Latest navigation checks: frontend lint and knip pass; 30 focused picker/table/month tests pass. Earlier larger focused suite passed 86 tests. Tests cover sortable columns, averages, change thresholds, suppressed comparisons, gross/net mismatch, default month and selected-month preservation.
- Local `mise check` frontend fails on stale `.next/dev/types` imports of unrelated removed routes. `next build` compiles/passes TypeScript then fails prerendering unrelated `/praksis` or `/nav-pilot/guider` and public video-feed fetch. Do not disable gates. Clean CI passed before latest commit.
- Local full Vitest previously had one old-domain-link failure in gitignored `copilot-intern` files; CI link guard passed. Do not edit those files as incidental cleanup.

## Next work in order

1. Check clean worktree, PR checks/review threads and auth. Restore interactive GCP access as needed.
2. With execution authorization, backfill August in dev using corrected validation/atomic code. Reconcile and exercise September net-to-net Endring. Then prod August and July/June sequentially, not a blind wide date sweep.
3. Real signed-in `/innsikt/team` browser check: empty/error-month recovery, default/current month and URL selection, own teams, sorting/search, narrow screens/keyboard, average denominator, overlap explanation and comparison basis. No browser tooling was available in the prior session.
4. Supervised recurring closed-month ingestion, bounded request budget, latest-source coverage and safe refresh for billing corrections. Existing `--run-once` must not silently spend hundreds of billing calls each night.
5. Team trends only on comparable periods, suppress every point and leave gaps rather than zeros. Model cost drivers need retaining per-user model source detail first; current rows store SKU totals. Team-size/change buckets are descriptive, not ranks; cannot add overlapping bucket spend to Nav bill.

Do not launch another Astra session just to repeat checks without a new review request. User authorized previous delegation explicitly; normal repo instruction says no agents unless requested.
