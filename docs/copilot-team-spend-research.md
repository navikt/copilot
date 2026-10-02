# Copilot spend by team: research log

## Goal

Show teams their GitHub Copilot spend so they can relate AI usage to personal, team and organizational value. This is cost transparency, not a leaderboard or a way to shame users. Reconcile organization totals to GitHub billing. Keep gross usage, discounts, net usage charges and seats distinct.

## Findings

September 2026 is the closed month used for the checks below. `copilot-dev-e17a.copilot_metrics` has all 30 days in `user_metrics`, `user_teams` and `billing_usage_daily_model`. `user_budget_snapshots` exists but is empty.

| September measure | USD |
| --- | ---: |
| Per-user `ai_credits_used` × $0.01, gross | 79,489.15 |
| Enterprise Copilot AI credits and cloud-agent usage, gross | 79,735.69 |
| Same usage, net after discounts | 64,558.85 |
| `copilot_for_business` seats, net, separate from usage | 14,744.63 |

GitHub's enterprise AI-credit report agrees with BigQuery's daily model billing to rounding. The per-user gross is $89.83 higher than the `Copilot AI Credits` SKU alone; including the $336.37 `Copilot Cloud Agent` SKU leaves it $246.54 lower than total Copilot usage gross. Nine days differ from the AI-credit SKU by more than $10, in both directions. Do not treat credits at $0.01 as net billed dollars. GitHub's budget `consumed_amount` is a current-month measure, not a historical net charge.

| September per-user gross bucket, excluding `nav-it-github-users` | USD |
| --- | ---: |
| Exactly one specific GitHub team on the usage day | 25,730.72 |
| Multiple specific GitHub teams | 44,742.08 |
| No specific GitHub team | 9,016.35 |

Of the unassigned amount, $1,810.76 has no same-day team record; joining by login instead of user ID did not recover it. The `user-teams-1-day` records contain membership IDs and slugs, but no primary or owning-team field. `v_team_daily_summary.sql` and `GetTeamUsageSummary` count a multi-team user's full usage for every team. Summing those team figures duplicates costs. GitHub's only configured cost center, `big spenders`, covers nearly all Copilot charges and is not a team map.

The GitHub enterprise AI-credit API accepts `user`, `day` and `cost_center_id` filters. For the 12 highest-gross users in September, per-user GitHub gross and BigQuery gross both sum to $9,237.64; GitHub net sums to $7,882.20. A second sample of six users in each of three cohorts, selected by stable hash ordering, also matched per-user gross to within $0.0001. The cohorts covered low spend under $5, other spend with a team, and users with a missing specific team on at least one day. All 30 requests succeeded. This tests gross coverage across several usage levels, not every user or the enterprise net reconciliation.

Teamkatalogen is a candidate roster, not yet a cost owner. `navikt/teamkatalogen-to-bq` publishes current `team` and `team_member` tables to `styring-av-sky-prod-77f0.teamkatalogen` on weekdays. The member export has 3,078 rows, 2,089 distinct Nav identities and 393 teams. All 3,078 `members_teamPercent` values are 0 and all membership start dates are null; 678 identities belong to multiple teams. The export has Nav identity and email, not GitHub login. GitHub SAML GraphQL resolved a current login to a name ID in a presence check, but a bulk or historical identity bridge is unverified.

Teamkatalogen's `naisTeams` maps 106 of the 179 specific GitHub team slugs seen in September to exactly one catalog team; 31 slugs map to multiple teams and 42 to none. Even one-to-one slug matches do not establish the person's owning team. `navikt/teamkatalogen-historikk` describes snapshots back to April 2025, but reading its snapshot table is denied to the current identity.

### Data needed to browse spend

| Data | BigQuery now | Gap for a team spend list |
| --- | --- | --- |
| Daily user usage and GitHub team memberships | `user_metrics` from 2025-10-10; `user_teams` from 2026-05-06 | No same-day team allocation before 2026-05-06; some users still have no specific team |
| Enterprise Copilot usage charges by day and model | `billing_usage_daily_model` from 2025-10-10 | No GitHub user key; cannot recover actual user net charges by joining this table |
| Actual per-user net usage charges | No table | GitHub's `ai_credit/usage?user=...` works but would need ingestion, historical backfill where available and a reconciliation to enterprise SKUs |
| Copilot seat/license costs | Enterprise `copilot_for_business` total is accessible from GitHub billing summary | No historical per-user seat charge table; current seat API is not a monthly billing history |
| Other billing tables | `billing_usage` stops at 2026-06-01; `billing_usage_reports` contains only $37.56 of September Copilot net usage and seats | Neither substitutes for enterprise daily model billing or per-user charges; `user_budget_snapshots` has zero rows |

The September source tables have full day coverage, but that does not imply complete per-user billing coverage or a team for every user. A team amount needs a reconciled source, a chosen allocation rule and a separate unassigned bucket. An estimated gross usage view can be prototyped from BigQuery alone; an actual net-spend list cannot yet be served from it.

## Decisions

- Agreed goal: team and personal cost awareness tied to value, with no leaderboard or shaming.
- Approved by the requester: each eligible GitHub team sees the **full** usage charge of each member on the usage day. Exclude only `nav-it-github-users`. These are overlapping member-spend views, **not additive team allocations or chargeback**. The enterprise bill is computed separately from distinct users; never sum team rows to represent it. Users without an eligible team remain unassigned in the organization view.
- Approved by the requester: all signed-in users may browse aggregate team spend. Named teams with fewer than five distinct contributing users in the selected month are suppressed. The combined small-team bucket counts each user only once across suppressed teams; it can overlap named teams and must not be added to the enterprise total. No individual cross-team spend view was requested.
- Approved by the requester: usage charges first. Show seat/license charges only at organization level until historical per-user seats are available. The $246.54 gross/$170.51 net September AI-credit residual may remain an explicit enterprise-only amount; a perfect attribution is not a release condition. Missing BigQuery data that GitHub supplies should be fixed in the ingestion pipeline.
- Reporting goal: net billed USD for spend, separate gross usage and discounts, alphabetical team list, clear coverage and source freshness. Neither GitHub-team membership nor a team total proves who created organizational value.

## Delivery plan after decisions (2026-10-02)

### Adversarial review before implementation

| Failure mode | Finding and response |
| --- | --- |
| Incorrect money | September `ai_credits_used` includes Cloud Agent and AI Credits, and the $170.51 net residual has no known user. Do not label gross as billed net, or distribute residual among teams. |
| Multiplication and privacy | Full membership exposure is non-additive. The small-team union can overlap named teams; suppressing only in the browser or allowing arbitrary month/day slicing can reveal an individual. Aggregate and suppress in the API at a fixed monthly grain. |
| Fragile ingestion | A complete 823-user census used the interactive `gh` credential, then repeat calls hit a rate limit. A nightly full refill or one GitHub request per user/day would exhaust the budget. Check the job token and persist progress before enabling automatic user billing ingestion. |
| False completeness | Existing supplementary ingestion treats any rows on a day as complete; it cannot know whether GitHub omitted some users. Compare report counts to the source report and mark incomplete months separately. Empty responses and renamed logins need explicit handling. |
| Historical identity | Current seats are not September seats, and login changed for one GitHub user ID. Preserve the GitHub ID for joins; unassigned data remains visible at organization level. |

Verdict: **concerns for a net-spend release** until the job credential, resumable ingestion and API-level suppression are verified. An aggregate gross-usage prototype can ship as gross only, with the same privacy rules.

### Scope and arithmetic

Show each team's members' **full net Copilot usage charges** in a monthly, alphabetical list and trend. A user in two eligible teams appears in both, and both rows include the user's full charge. Label this **member spend, overlapping across teams**. Do not label the sum of team rows "Nav spend". The organization view shows distinct per-user net usage, unassigned users, an enterprise-only billing residual, and the enterprise bill. Those organization components are additive; named team rows and the small-team bucket are not. Show Copilot AI Credits and Cloud Agent separately where per-user billing has SKU detail. Seats and Code Quality remain separate from team usage.

The existing `docs/copilot-team-spend-allocation.sql` implements the previously proposed equal split and is retained as a **historical research check**, not the approved reporting rule. It must not be used as the team spend query. September gross exposure under the approved full-member rule sums to $184,515.26 across 177 named team candidates, versus $79,489.15 distinct per-user gross. This is expected overlap, not excess billing. Fifty-two teams have fewer than five September contributors. Their row sum is $15,354.84 gross; the requested deduplicated small-team bucket is $11,582.25 gross for 126 distinct users across those teams. That bucket may also include users who appear in named teams. All these figures are gross research checks, not net spend.

### Ordered work and verification

| Step | Action | Pass condition |
| --- | --- | --- |
| 1. Fix the pipeline's coverage signals | In `copilot-metrics`, compare daily user and user-team report counts and identity coverage, not just whether a day has rows. Alert and retry incomplete days within GitHub's report availability window. Check whether the September 438 user-days lacking any team record and the 944 with only the catch-all are missing source data or real GitHub membership. Backfill recoverable days; keep genuine nonmembers unassigned. | Rerun the coverage check and document source-vs-BigQuery counts, which gaps were repaired, and which are source limitations. No silent zero or stale day can be labeled complete. Historical team data before its May 2026 availability cannot be manufactured. |
| 2. Ingest per-user billed usage | Use the existing `copilot-metrics` billing credential to fetch user-filtered AI-credit usage by Copilot SKU. Test the job credential and measured rate budget after the current GitHub limit resets. Prefer day/user/SKU gross and net if feasible; otherwise use month/user/SKU with daily usage as a documented *estimated timing* of monthly net spend. Seed the month from historical user/team reports plus the available seat list; use GitHub user ID when linking reports to handle renamed logins. Store enterprise totals and ingestion status alongside per-user amounts. On 403/429, stop, keep the prior complete month, and retry later without restarting every successful request. | A completed September load reproduces the 823-user census, $64,127.05 net Copilot AI Credits plus $261.28 net Cloud Agent for fetched users, and exposes the $170.51 AI-credit residual. Source freshness, attempts, failures and completeness are visible. A second run produces no duplicates. Do not use `user_budget_snapshots` as billed net history. |
| 3. Aggregate without double-counting the bill | Add a month-bounded query in `copilot-api`: deduplicate user/team/day, exclude only `nav-it-github-users`, and give each eligible team the member's full charge. Use day/user/SKU when available; for monthly billing, distribute each user's combined monthly net over their observed usage days by gross weight only after validating that daily `ai_credits_used` covers the included SKUs. Keep net with no usable day or eligible team in unassigned. Count distinct monthly contributors for small-team suppression; produce a deduplicated union of users covered by suppressed teams, not a sum of hidden team rows. | Each named team's total equals the full eligible member charges under the approved timing rule. Distinct per-user charges plus explicit enterprise-only residual equal the billed Copilot SKU totals. Suppressing teams changes neither the organization bill nor other named team figures. Tests cover multi-team and mixed named/suppressed memberships, rename, missing team, zero-weight and moved-team cases. |
| 4. Display safely in `/statistikk` | Publish aggregate team rows to all signed-in users. Suppress team names and direct per-team routes for fewer than five distinct contributors for the month in the API itself, and show the deduplicated small-team bucket as overlapping exposure. Add month selector, alphabetical searchable team list, a monthly trend and separate organization reconciliation with the enterprise total and seat line. Keep cross-team individual amounts unavailable. | A signed-in user can browse eligible teams, understand that rows overlap, find the exact month and SKU, and distinguish a team member view from the distinct Nav bill. API responses cannot reveal a suppressed team through filtering, details or different time windows. Small-team trend points follow the same threshold. |

### Remaining source limitations and checks

- The $246.54 gross/$170.51 net September Copilot AI-credit residual is acceptable as a named enterprise-only amount. Do not divide it among teams. A census of known logins cannot prove whether a billed account exists outside those seed lists; review org/cost-center reports only if that would change the ingestion user source.
- A completed read-only 823-login census worked, but a repeat hit GitHub's limit. `gh` access is not proof that the Naisjob credential can sustain a daily or monthly sweep. Gate the new fetcher on a small job-credential probe, a rate budget and incremental retry behavior rather than repeated full backfills.
- `user_metrics.ai_credits_used` includes AI Credits and Cloud Agent together. If daily per-user SKU detail is impractical, a monthly user/SKU amount can be shown by team using a validated time-weighting assumption. Show it as an estimate when team memberships change during the month; keep SKU organization totals exact.
- A team with fewer than five users must not leak its figure through a direct API request, a chart point or a subtractable breakdown. The approved deduplicated small-team bucket overlaps other team rows; neither it nor the sum of visible teams reconciles to the enterprise bill. Use a distinct-user organization reconciliation instead.
- The current `/statistikk` filters team data to the viewer's teams, and its small-team message is not enforced by `copilot-api`. The new aggregate route needs its own access and suppression checks. Existing `v_team_daily_summary` counts full usage per team but is not a net-billing source.

## Later follow-ups

- Research historical seat charges and plan types before allocating licenses to teams; the current seat API is not a historical invoice.
- If the organization needs chargeback rather than a labeled GitHub-team allocation, ask Teamkatalogen owners for a dated owning-team source and a controlled GitHub-login-to-Nav-identity mapping. The current `teamPercent` and start dates cannot provide this; request history-table access only if it answers that question.

## September gross-allocation proof (2026-10-02)

`docs/copilot-team-spend-allocation.sql` is the reproducible read-only query. From the repo root, run:

```sh
bq query --project_id=copilot-dev-e17a --use_legacy_sql=false --max_rows=1000 "$(<docs/copilot-team-spend-allocation.sql)"
```

It reports **allocated gross usage**, not billed spend. Its two date filters are both pinned to September; change them together to inspect another month. The unassigned row is last. `bq query` defaults to 100 output rows, so `--max_rows=1000` is needed for all 178 rows. Do not publish the row list until eligible teams and small-team visibility are decided.

| Verification | September result |
| --- | ---: |
| Source user-days and users | 10,234; 719 |
| Team rows after excluding the catch-all | 30,550, all unique by `(day, user_id, team_id)` |
| Teams with attributed usage | 177 |
| Team-allocated gross | $70,472.80 |
| Unassigned gross | $9,016.35 |
| Team-allocated + unassigned, summed before rounding | $79,489.15 |
| Per-user source gross, summed before rounding | $79,489.15 |
| Unrounded allocation difference from source | $0.00 to eight decimal places |

No September `user_metrics` credit or user ID is null; all September source rows have enterprise scope `nav`, and each user-day is unique. Team IDs map to one slug each in the period. The 1,382 unassigned user-days comprise 438 with no team record ($1,810.76) and 944 with only the catch-all ($7,205.60). Of the team-allocated gross, $44,742.08 came from multi-team user-days. **Summing the rounded amounts displayed for each of the 178 rows gives $79,489.13, two cents less than the rounded unrounded total.** Reconciliation must use unrounded amounts, and a future UI must explain any display-rounding difference.

The same allocation logic reconciled August: source gross $54,466.22, allocated $50,594.24 and unassigned $3,871.98 across 176 teams. The August unassigned amount was $23.74 with no team record and $3,848.23 with only the catch-all. September's larger unassigned amount merits investigation, especially the growth in missing team records. In September, 27 users with $4,268.81 of monthly gross had different eligible team sets on different days; using only their latest team would misallocate historical usage. Fifty-two of 177 teams with attributed usage had fewer than five distinct users, accounting for $6,321.58 gross. A statement in the current team UI that such teams are hidden must not be taken as enforcement.

**Result:** Step 1's arithmetic and technical join are proven against current BigQuery data. The team-exclusion and visibility reviews remain open. Step 2, a complete per-user *net billing* reconciliation, is still required before showing this as team spend. GitHub enterprise September Copilot gross including cloud agent was $79,735.69 and net was $64,558.85; the query makes neither claim about billed net charges nor seat costs.

## September per-user billing census (2026-10-02)

`docs/copilot-team-spend-reconcile.py` is a read-only aggregate-only check. A completed run queried 823 unique logins from September user/team reports and the then-current seat list, with zero failed requests. Joining user and team reports by GitHub user ID rather than login resolved one rename that otherwise caused a 404. The then-current seat list is not a historical September census.

| Source or SKU | September gross USD | September net USD |
| --- | ---: | ---: |
| Fetched users, Copilot AI Credits | 79,152.78 | 64,127.05 |
| Enterprise, Copilot AI Credits | 79,399.32 | 64,297.57 |
| Enterprise less fetched users, Copilot AI Credits | **246.54** | **170.51** |
| Fetched users and enterprise, Copilot Cloud Agent | 336.37 | 261.28 |

Per-user BigQuery metrics gross was $79,489.15, matching fetched Copilot AI Credits plus Cloud Agent gross to cents. Thus daily `ai_credits_used` does not by itself isolate the AI-credit SKU. The $246.54 gross/$170.51 net AI-credit residual is real in this census and has not been attributed to a team. Code Quality AI Credits ($174.52 gross/$132.21 net) were not returned by the user-filtered requests and are out of the first team's Copilot scope. A further full census must not be run without a rate-limit budget and a way to handle user accounts absent from all three seed lists.

After the completed run, a repeat was terminated after ten minutes. The script previously set no per-request subprocess timeout and only printed at completion. A later GitHub request returned `403 API rate limit exceeded`, and `gh auth status` reported the token invalid during the limit; that status alone does not prove the token was revoked. No further GitHub requests were made after that diagnosis. The script now sets a 45-second subprocess timeout, reports progress every 100 results and stops on 403/429 instead of retrying the limit. It has not completed another full run with those changes. Wait for the limit to reset before checking access; do not treat the failed repeat as new reconciliation evidence. Ingestion remains gated on explaining or explicitly carrying the residual and on confirming usable rate limits for the production job credential.

## Implementation in progress

### Page placement (revised)

The site's refactored navigation has `/innsikt` as an overview page with links to separate insight pages. The private `/innsikt/team` page holds the new team cost and usage table. `/statistikk` retains its organization and repository tabs; its team tab is removed. `/innsikt` links to the new page. Wonderwall ignores the page path so Next.js handles redirects, and `proxy.ts` requires a validated sign-in. The month selector submits to `/innsikt/team`. The previous activity table was not moved: its backend endpoint has no five-contributor suppression and can reveal small-team rows. Add activity context later through an endpoint with the same suppression rule.

Route migration check: `/statistikk#team` maps to `/innsikt/team#teamkostnad` through `legacy-anchors.ts`. The link inventory has the new route and page anchor. Middleware and nav-item tests pass. The full link-inventory test still fails on two existing `copilot-intern` links to the old site domain, unrelated to the moved page. `mise check` in my-copilot still fails on stale `.next/dev/types` for routes unrelated to `/innsikt/team`. `next build` compiled and passed TypeScript, then failed prerendering unrelated `/praksis` (uncached data outside Suspense) and the public video feed (ECONNREFUSED); an earlier build also flagged `/nav-pilot/guider`. These failures were observed without editing those pages.

### Verification round

- Read-only BigQuery check of the gross endpoint's SQL for September 2026: 30 days, $79,489.15 distinct gross, $9,016.35 without an eligible team. The five-user rule must count **contributors with positive gross**, not every seat or zero-usage row. With this corrected rule, 122 team names appear and 55 are suppressed. The suppressed union has 133 contributing users and $12,740.44 gross, deduplicated by user and day. August has 121 named and 55 suppressed teams, with $8,180.90 in the suppressed union. Earlier 125/52 and $11,582.25 results used all user IDs, including zero-use users, so they are not the publication thresholds.
- The historical user seed query returns 759 distinct GitHub IDs for September, none with a blank login. This excludes current-seat-only accounts used in the earlier 823-login research census. The BigQuery tables for new per-user net billing and completed-month markers do **not** yet exist in dev. The net SQL therefore cannot be checked against real stored data; no live GitHub backfill was attempted while rate limits were uncertain.
- The old team activity table was removed from `/innsikt/team`. Its endpoint lacks suppression and could show teams under five contributors despite the new page's privacy rule. Only the new aggregate gross/net endpoints supply the page now. Adding activity context requires the same server-side monthly suppression.
- `mise check` passes in `apps/copilot-api` and `apps/copilot-metrics`. Focused frontend middleware, navigation and month tests pass (76 tests). Eleven of twelve link-inventory tests pass; the remaining test flags old-domain links in `copilot-intern/godkjente-agenter.md` and `copilot-intern/retningslinjer.md`. Frontend lint and Prettier pass for touched files. Frontend build compiles and passes TypeScript, then fails prerendering `/praksis` and fetching the public video feed. `mise check` still fails on stale `.next/dev/types` references to unrelated missing routes. Those build failures predate this page and were not bypassed.

### Live September backfill and net verification

On 2026-10-02, `gh` had `admin:enterprise` and local gcloud had read/write access to `copilot-dev-e17a`. Local fnox lacked `GITHUB_BILLING_TOKEN` and GitHub App credentials. The manual `--user-billing-month` path now needs only `GITHUB_BILLING_TOKEN` and `GCP_TEAM_PROJECT_ID`, rather than starting the unrelated usage-ingestion GitHub App client. A `gh auth token` was passed to that path in the process environment without printing or storing it. This tests the manual backfill with the interactive token, **not** the production job's PAT.

The first Go BigQuery token exchange failed with `EOF`. A retry created `billing_user_monthly` and `billing_user_monthly_runs` in dev and fetched 21 users before the tool timeout. A resumed run reached 601 completed users before its one-hour tool timeout. The next resume completed all **759 historical GitHub IDs** and set `status='complete'` for September; a further invocation returned "already complete" without fetching users. No incomplete run was shown as net spend. This was a live dev dataset write, not a production backfill.

| September billed usage | Stored user net USD | Enterprise net USD | Enterprise-only residual USD |
| --- | ---: | ---: | ---: |
| Copilot AI Credits | 64,127.05 | 64,297.57 | 170.51 |
| Copilot Cloud Agent | 261.28 | 261.28 | 0.00 |
| Both Copilot SKUs | 64,388.33 | 64,558.85 | 170.51 |

Stored rows: 683 AI Credit users, 125 Cloud Agent users, and 759 `done` markers. A read-only run of the net API's SQL against populated BigQuery produced 122 named teams with **$135,982.27 overlapping net member spend**, 55 suppressed teams with **$9,802.10** deduplicated net across 133 contributors, $7,215.43 unassigned net, $64,388.33 distinct known-user net, and the $170.51 enterprise-only residual. No user with a stored net charge lacked positive daily gross in this month. These are aggregate SQL results; the authenticated HTTP route and rendered page have not been exercised against the live API.

Remaining verification: the production `copilot-metrics` PAT must be tested separately. The manual backfill's source list omits seat-only accounts from the earlier 823-login census, though the measured residual matches that census. A completed run skips a closed month rather than refreshing later billing corrections; decide a safe refresh mechanism before operationalizing monthly loads. The gross and net endpoints still need a real HTTP/page check with a signed-in session. The frontend's unrelated full-check failures remain as noted above.

### Production backfill readiness check

Read-only queries of `copilot-prod-c697.copilot_metrics` show all 30 September days in `user_metrics`, `user_teams`, and `billing_usage_daily_model`, matching dev: 719 usage users, $79,489.15 distinct gross, 122 teams meeting the five-contributor threshold, 55 suppressed teams, and $64,297.57 net AI Credits plus $261.28 net Cloud Agent. Production does not yet have `billing_user_monthly` or `billing_user_monthly_runs`. As in dev, 438 user-days for 57 users have no team record. These must remain unassigned; the daily GitHub report may genuinely omit membership.

The checked-in Naisjob runs `--run-once` with `activeDeadlineSeconds: 3600`, shorter than the dev backfill. Triggering it cannot start the new backfill mode. `copilot-api` has READ and `copilot-metrics` has READWRITE access to the production dataset. The pod's `GITHUB_BILLING_TOKEN` has not been tested against the per-user endpoint. Before making monthly billing ingestion a recurring Naisjob, verify that credential and define a supervised resume and refresh schedule.

### Production backfill dispatched from local machine (2026-10-02)

The requester authorized a background production backfill. The checked-in Naisjob runs `--run-once` on a one-hour deadline and cannot run the unmerged backfill code by triggering it, so this run uses the locally built `copilot-metrics` binary, `gh auth token` for GitHub and local GCP application-default credentials for `copilot-prod-c697`. No token was printed or stored in the repo. The launch helper and process log are under `/var/folders/8q/l5h25q1s1tn8wp0p_d2nc4qc0000gn/T/opencode/`. The first process failed before writes with a transient Google OAuth `EOF`. A second process started as PID 35535 and created `billing_user_monthly` and `billing_user_monthly_runs` in prod. At the first check it had five completed user markers; a later check found 24. The process is independent of this terminal session; check its PID, log and BigQuery completion marker rather than assuming it will finish. It is not a Naisjob and will not be supervised by Nais. The net API returns no monthly net result until the completion marker exists. The prod job credential and a production Nais backfill route remain unverified.

### Follow-up: presentation and interaction

- Review `/innsikt/team` with users after the API deploy: month selection, search, the small-team bucket, and the distinction between overlapping team rows and distinct organization billing. Validate keyboard access and narrow screens. Add a monthly trend only with the same five-contributor suppression for every point.
- Existing gross prototype `docs/copilot-team-spend-allocation.sql` implements the rejected equal-split calculation and is retained only as research. The API uses full-member overlap.
- Refresh of a closed, completed billing month is not implemented; the manual backfill intentionally exits instead of overwriting a completed month. A revised-bill refresh needs an explicit safe procedure before scheduling recurring ingestion.

## Evidence and checks

- Code: `apps/copilot-metrics/billing.go`, `apps/copilot-metrics/views/v_team_daily_summary.sql`, `apps/copilot-api/bigquery_stats.go`, `apps/my-copilot/src/app/(nb)/statistikk/page.tsx`; read-only prototype: `docs/copilot-team-spend-allocation.sql`.
- Data: read-only BigQuery queries of `copilot_metrics` in dev and prod, and a completed dev manual billing backfill. A local production backfill is in progress as described above. No user identities or individual figures are recorded here.
