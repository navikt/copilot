# Copilot spend by team: research log

## Current delivery status (2026-10-03)

This section is the current plan. Dated entries below preserve earlier evidence;
statements that tables or features did not yet exist describe those earlier checks.

| Work | Status |
| --- | --- |
| Monthly team page, full-member overlap and API suppression | Implemented in draft PR #1419. Current-month and mixed gross/net comparisons remain unavailable. |
| September user net billing | Complete in dev and prod: 759 historical IDs, $64,388.33 known-user net, $170.51 enterprise-only residual. Production used the earlier binary; it was not silently rewritten. |
| August user net billing | Dev complete: 728 users, $32,163.25 known net, $32,295.17 enterprise net, $131.92 residual. Remaining nine accounts resolved by immutable GitHub IDs and validated atomic writes. Live API query exposes 113 August and 119 September teams for net-to-net comparison. Production August has not been dispatched; remaining history is tracked in #1425. |
| Automatic future monthly ingestion, #1421 | Committed and pushed as `842e6c4f` on `feat/team-spend-insight`. Separate enabled nightly worker discovers unfinished closed UTC months from October 2026. First eligible collection is November 1. |
| Monthly worker deployment | Workflow deploys both manifests. Same `copilot-metrics` secret as existing ingestion, 08:00 UTC prod and 09:00 UTC dev, `concurrencyPolicy: Forbid`, 50-minute runtime inside a one-hour pod deadline. No Kubernetes secret extraction is needed. |
| Restart and request limits | Existing atomic user checkpoints resume nightly. At most 2,000 identity/billing requests per execution, including retries; stop with 500 requests left in reported quota. Pending months share runtime/request budgets. |
| Source and identity integrity | Transactional daily user/team replacement and successful-report receipts, including valid empty reports. Pending-month source repair extends beyond seven days. Invalid weights, partial downloads and identity mismatches cannot complete a month. Manual loads cannot publish October 2026 or later through the weaker historical path. |
| Published reconciliation | New completed months store enterprise Copilot totals alongside user amounts. API excludes the enterprise sentinel from user totals and retains historical daily-table fallback. Monthly timing remains estimated from daily usage and memberships. |
| Billing corrections | Deferred. Completed months are as-collected observations, not guaranteed final invoices. A later correction needs a staged replacement that preserves published data on failure. |

### Latest verification and review

- Both Go app `mise check` gates pass. Metrics race tests and workflow syntax checks pass.
- Opt-in live BigQuery checks execute actual API queries and transactional replacement SQL, including empty reports and injected rollback. They do not modify the August or September billing loads.
- A bounded live probe passed immutable identity lookup, user-filtered September billing and enterprise September billing in three requests. It used the interactive CLI token, not the deployed job token. Same-secret wiring is established; deployed execution remains to be observed.
- `mise all` generated and built every app. Its check stage failed on existing frontend `.next/dev/types` imports and stale model-pricing data. These are not passing release checks.
- Exact GPT-6 Astra comprehensively reviewed metrics correctness/resilience and returned **BLOCK for activation before fixes**, with concerns for changed daily ingestion even when sync was disabled. Corrections prevent the future-month manual bypass, invalid credit-weight certification and checkpoint-masked failures. Additional targeted fixes cover org-token fallback, transient-error classification, secret-bearing transport logs, insert after failed deletion, partial budget census replacement, backdated live budget observations and month-end previous-month selection. A focused follow-up found write-timeout failure masking; corrected with tests for both write exits. The final narrow Astra review returned **CLEAN**, with no remaining confirmed blocker in those corrections. Empty download-link lists now fail closed; a downloadable empty NDJSON report can still receive a receipt.
- Remaining pre-existing interior-gap recovery, non-atomic legacy writes, false-success reporting and credential-boundary documentation are tracked in #1422. That issue is not a reason to add an ingestion framework to this PR; assess any finding that affects changed paths during final review.

### Wrap-up work for this PR

1. Focused Astra follow-up is complete. The live contract probe and source review do not prove the entire deployed scheduler, source-repair, notification and publication lifecycle.
2. August dev recovery and reconciliation are complete. #1425 retains production August and older-history work, each a separately authorized data operation rather than an automatic effect of merge.
3. The requester reports successful browser end-to-end testing. Local page/navigation tests pass. Keyboard and narrow-screen checks were not separately reported; this environment has no browser connector or signed-in session.
4. Implementation is pushed at `842e6c4f`; fresh CI and dev deployments passed. PR #1419 includes `Closes #1421`. It remains draft and unmerged. Obtain required human review before merging.

### Follow-up ownership

| Issue | Remaining work |
| --- | --- |
| #1425 | Resolve August HTTP 404, complete/reconcile dev, authorize production and assess older history. |
| #1423 | Evidence-backed correction policy and safe staged monthly replacements. |
| #1424 | Signed-in product/accessibility validation, comparable trends, cost drivers, descriptive buckets and value context. |
| #1422 | Legacy interior gaps, non-atomic writers, failure reporting and credential-boundary documentation. |
| #946 | Authoritative organizational segments and exclusive cost-center allocation, distinct from overlapping team consumption. |

Recommendation: start wrapping up. No more team insight features are needed in
this PR. Automatic restatements, older monthly history, trends, cost-driver
breakdowns, team buckets, value context and the broader #1422 resilience work
remain follow-ups. Merge deploys changed daily ingestion and the enabled billing
schedule to dev and prod; the latter performs no monthly collection before November.

## Selectable usage columns: measured feasibility (2026-10-03)

Read-only dev BigQuery analysis of August and September used distinct
same-day `(day,user_id,team_slug)` memberships, excluded only
`nav-it-github-users`, and joined by immutable user ID. The measured cohort is
the gross page's positive-credit, minimum-five-contributor teams: 121 in August
and 122 in September. The net cohort differs and requires its own final filter.
No team names or individual results are recorded here.

| Optional column | Ranking measure | September teams with any publishable named category | Teams whose true winner is publishable |
| --- | --- | ---: | ---: |
| Up to three models | User-initiated interactions | 66 / 122 | 58 / 122 |
| Feature | User-initiated interactions | 71 / 122 | 71 / 122 |
| Language | Code-generation activities | 88 / 122 | 64 / 122 |
| IDE | User-initiated interactions | 21 / 122 | 20 / 122 |
| Skill | Reported interaction count | 48 / 122 | 48 / 122 |
| MCP | Reported interaction count | 48 / 122 | 38 / 122 |
| Custom agent | Reported interaction count | 11 / 122 | 11 / 122 |
| Plugin | Reported interaction count | 1 / 122 | 1 / 122 |
| Slash command | Reported interaction count | 15 / 122 | 15 / 122 |

Publishable means at least five distinct team members have positive activity for
that category. Exclude `others` and `unknown` from named rankings, but do not
erase their activity from coverage. Parent-team suppression alone is insufficient.
Thirty September teams have three publishable model categories, but only 18
can publish their actual top three. Returning the top categories after suppression
must be labeled "most used among categories that can be shown", not an
unqualified claim about the true winner. No suppressed names, counts or ranks
should reach the browser. Percentages/complements need a separate disclosure
review; omit them in the first version.

### What the source supports

- Model interactions sum to 98,385 of 98,539 top-level September interactions,
  a gap of 154, matching the feature-only `vscode_agent` interactions. August
  reconciles exactly at 63,134. Keep unknown/unattributed coverage visible.
- Feature interactions reconcile exactly to top-level totals in both months.
  September CLI has 73,387 interactions, 74.5% of the total, versus 38,364,
  60.8%, in August. IDE agent mode has 19,307, 19.6%, versus 18,809, 29.8%.
  These are enterprise-level activity shares, not overlapping-team totals or cost shares.
- All language-feature generation counts reconcile to top-level generations:
  207,781 in September and 186,323 in August. Model generations cover only
  132,079 of September's 207,781. Do not rank models by adding interactions,
  generations and acceptances; those are different events and need not be disjoint.
- Leading September language-generation counts are Kotlin 62,150, TSX 36,381,
  TypeScript 24,392, Markdown 21,606 and Java 15,377. This describes Copilot
  code-generation activity, not the team's full repository language mix.
- IDE breakdown records only 24,929 of 98,539 September interactions, 25.3%,
  and 96,632 generations. CLI/app activity is not fully represented. Some IDE
  labels look like version strings; define known-label normalization before display.
- September reports contain real skill activity for 310 users, MCP for 360,
  custom agents for 215, plugins for 19 and slash commands for 255. August
  contains no measured customization activity in these arrays. That is a
  reporting/coverage boundary, not evidence that nobody used them in August.
- Across gross-visible teams, the average actual top-three-model share fell
  from 82.0% in August to 69.6% in September; teams with one model above 50%
  fell from 42/121 to 26/122. This suggests a broader reported model mix, but
  changing membership, reporting and model availability prevent a causal claim.

### Smallest useful UI increment

Keep the existing cost columns as defaults. Add opt-in model, feature and
language columns through a small column selector; retain selection locally if
needed, without a server preference system. Use up to three model names and
one feature/language, with explicit ranking measures and a suppression/coverage
explanation. No new ingestion or GitHub API calls are needed: query stored JSON
and apply same-day joins and subgroup suppression in the API.

IDE and named customizations belong later or in team detail because their sparse
and uneven coverage would make the main table misleading. Repository reports
remain PR-only with no user/team join; a separate ownership map cannot prove
where members worked. Findings and selectable-column scope are tracked in #1424.

The requester subsequently approved implementation in this branch. Month, search
and column selection now share one responsive toolbar outside the loading boundary.
The five cost columns remain defaults; model, feature and language names are
opt-in. The gross API includes same-day usage summaries for visible team IDs,
with five positive-activity contributors per named category, unknown-category
exclusion and ranking after suppression. No counts or percentages are exposed.
Controls remain available during loading and empty/error responses. The loading
skeleton matches explanatory text and table rows; changing month resets its boundary.

Verification: API `mise check` and live August/September query tests pass. Live
September composition counts match independent analysis: 66 model, 71 feature
and 88 language summaries. Eleven focused frontend tests pass, including unified
month/search controls and column/header alignment. Frontend `mise check` passes
lint then fails only on the previously recorded stale `.next/dev/types` imports.
These UI/API additions have not yet received independent review or browser E2E;
the requester's earlier browser confirmation covers the prior page.

## Goal

### Provider and model-category mix proposal

Aggregate raw model activity into providers and pricing-catalog categories before
applying subgroup suppression. Individual model summaries cannot be regrouped
after their top-three limit: that would omit most activity and undercount contributors.
Count each positive-activity user once per team/month/group across all its models.

The generated `src/lib/model-pricing.ts` catalog provides provider and category
metadata, but display names differ from report IDs and older models are absent.
Use explicit, tested report-ID aliases and retain an unclassified category.
Do not derive tiers from model-name substrings: Claude Haiku 4.5 is Versatile
in the current catalog, while Gemini 3.5 Flash is Lightweight and Gemini 3.6
Flash is Versatile. Categories describe this catalog, not measured task quality.

Preferred optional columns are provider mix and category mix, measured by
user-initiated interactions. Wider groups should increase coverage but cannot
guarantee publishable distributions for every visible team. Apply minimum-five
contributors independently to each group. Do not expose percentages renormalized
over visible groups as a complete mix or reveal a suppressed share by subtraction.
An initial names-only summary is less informative but follows existing disclosure
rules; a full percentage/bar design requires an explicit denominator and
complementary-suppression policy. Unknown attribution must not become "other
providers" because unknown is not a verified provider.

The follow-up live coverage measurement was blocked by expired GCP authentication.
No provider/tier coverage numbers have been established yet. Reauthenticate, then
compare provider/tier coverage with the September 66-model-summary baseline
before choosing the final presentation. Broader model categories do not resolve
the separate deployed-response problem that made every optional column empty.

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
| Actual per-user net usage charges | `billing_user_monthly` and `billing_user_monthly_runs`; September complete in dev/prod | August dev pending; older history and correction refresh remain follow-ups |
| Copilot seat/license costs | Enterprise `copilot_for_business` total is accessible from GitHub billing summary | No historical per-user seat charge table; current seat API is not a monthly billing history |
| Other billing tables | `billing_usage` stops at 2026-06-01; `billing_usage_reports` contains only $37.56 of September Copilot net usage and seats | Neither substitutes for enterprise daily model billing or per-user charges; `user_budget_snapshots` has zero rows |

The September source tables have full day coverage, but that does not imply a team for every user or that every billed account appears in historical reports. September net can now be served from completed per-user billing with an explicit enterprise residual and separate unassigned bucket.

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
- Refresh of a closed, completed billing month is not implemented. Automatic initial collection can proceed without restatement; a revised-bill refresh needs a staged replacement before that capability is enabled.

#### Next page iteration: plan and review

1. Resolve the signed-in person's current GitHub teams from the existing SAML identity lookup and a new ownership-checked, latest-day membership read. Do not infer team membership from a seven-day usage report: a member may have no usage that week. If identity lookup fails, keep browsing available and say why personal highlighting is missing.
2. Put the **distinct Nav bill** in its own summary, with separate known-user, unassigned and enterprise-only amounts. Keep the overlapping team table below it, alphabetically ordered, searchable and with the viewer's eligible teams first. Any displayed percentage must be labelled as a team's members' spend relative to the distinct Nav total; percentages across teams do not sum to 100 %.
3. Compare to the previous closed month only where both months have the same gross/net basis and the team meets five-contributor suppression in both months. Otherwise say that a comparison is unavailable. Use the month picker to navigate; do not expose individual users or add an unsuppressed daily series.

Adversarial check before implementation: a missing personal lookup must never hide aggregate data; an old login after a rename may fail to match a team slug and should not be presented as an absence of membership; percentages of overlapping rows must not be labelled budget shares; the suppressed bucket overlaps visible teams and cannot be added to the bill; a previous-month amount must never bypass that month's five-contributor threshold. Keep the page's money source explicit when billing ingestion is absent.

Implemented in the draft PR: `/usage/my-teams` resolves only the authenticated caller and reads the latest enterprise `user_teams` day, with a recent `user_metrics.user_id` fallback for renamed logins. `/innsikt/team` keeps the organization bill apart from team rows and puts matching current memberships first. It compares amounts to the previous month only when that month has all calendar days, the same gross/net basis and a visible five-contributor row; otherwise the cell says "Ikke tilgjengelig". A missing caller identity does not block browsing. The component has focused tests for overlapping totals, own-team discovery, missing identity and comparison suppression. A current team match is a navigation hint, not proof of the person's team in the selected historical month.

#### CI cleanup and live API-query verification

Removed the unused previous team activity component, its fetchers and unused TypeScript team-summary type after CI's unused-code check caught them. `knip` now passes. Corrected the page to use net-mode suppression counts when displaying net amounts and disabled full-month comparisons for the current or incomplete selected month. Added tests for correct net suppression totals and same-basis comparisons.

`VERIFY_TEAM_SPEND_BIGQUERY=true mise test:short` passes in `apps/copilot-api`. The opt-in read-only test executes the actual Go BigQuery client and row decoder against September dev data, then the cached HTTP handler using `httptest`. It verifies 122 gross-visible teams, 119 net-visible teams, the $64,388.33 known-user net and $170.51 residual, and that August with no billing completion marker returns no net result. This validates the query/handler, not deployed Azure OBO authentication or a real browser session. Focused frontend tests pass (81 tests). The broader suite has 690 passing tests and only the existing old-domain links in local `copilot-intern` files failing. Local full `mise check` still hits stale `.next/dev/types`; CI's clean workspace will recheck this after push.

#### Independent GPT-6 Astra review and corrections

An external Copilot CLI session confirmed `currentModel: gpt-6-astra`. Its initial adversarial review returned **BLOCK**: missing billing user/month/envelope validation, historical-login reuse disclosing a different account's teams, interrupted SKU upserts preserving stale charges, and empty/error months removing navigation. The initial review read the diff but was incomplete on affected callers and runtime verification; do not treat it as a completed independent six-axis clearance.

Corrections: validate billing response user, enterprise, year/month, absence of a daily filter, and a present non-null `usageItems` array before writing. Atomically delete/replace one user's month/SKU snapshot with its `done` marker. Remove the historical-login-to-user-ID expansion from personal team lookup; rename handling remains unavailable until a trusted current immutable ID is resolved. Keep the month picker outside the data component and key the input to the validated month so client navigation discards unsubmitted edits. The two GitHub review comments were CodeQL findings about query errors reaching logs; changed handlers now log fixed messages without those potentially user-derived errors. No Copilot-authored review was present when checked.

A fresh focused Astra review returned **CONCERNS**, with no confirmed blocker in the corrected paths. It flagged the stale picker value and insufficient transaction/resume evidence; both were addressed. SDK inspection confirmed STRUCT query parameters use `bigquery` tag names, so the replacement SQL selects snake-case fields. `VERIFY_TEAM_SPEND_BIGQUERY=true go test -run '^TestUserBillingSnapshotTransaction$' -count=1 -v` passes against a temporary dev table using the production replacement SQL and an injected rollback failure. Unit checks reject wrong/missing response envelopes without writes, verify replacement of absent SKUs, and resume only unfinished users after a storage failure. Both Go `mise check` gates pass; 83 focused frontend tests, lint and unused-code checks pass. Independent review does not prove deployed OBO authentication, browser accessibility or immunity to inference across overlapping groups; keep the PR draft pending those remaining checks and CI on the corrections.

Production September backfill has completed all 759 seeded users. A post-run reconciliation matches dev: $64,127.05 AI Credits net, $261.28 Cloud Agent net, and $170.51 enterprise-only residual. It ran the older per-SKU upsert binary, before response validation and atomic replacement were added. These revisions were tested prospectively, not by rerunning production. Do not silently overwrite that completed run as part of review fixes.

## Evidence and checks

### Page wording, navigation and next insights

Latest navigation correction: the month selector and Vis button are the only controls; previous/next text links were removed. An unfiltered `/innsikt/team` now defaults to the current UTC month, not the preceding closed month. Explicit `?month=` selections are preserved. September cannot show a net change against August until August has completed per-user net billing. Current-month changes stay unavailable because partial month totals cannot be compared with full-month totals; the page explains this briefly. No gross-to-net or partial-to-full difference is displayed as a real change.

The month picker now lists Norwegian month names and offers both previous and next navigation, bounded by May 2026 and the current month. It remains outside the data component, including on empty/error months. Table columns are Team, Medlemmer, Forbruk (or Forbruk før fradrag), Per medlem and Endring, all sortable; alphabetical order is the default. Medlemmer counts contributors with positive amounts, not all licensed team members. Per medlem is the row amount divided by those contributors, not a mean over the full roster. Missing comparisons display a dash and one explanation rather than repeated technical text. September has completed net billing but August has only gross usage; no net-to-gross delta is calculated.

Provisional highlight rule: an increase is red and a decrease green only if the absolute change is at least $10 and at least 10% of the preceding amount. A preceding zero with an absolute change of at least $10 also highlights. Signed amounts remain readable without color; this is an investigation cue, not a quality or value score. Threshold is an assumption for requester review.

Next insight work, in order:

1. Comparable monthly history: resolve the interrupted August dev load and reconcile it under #1425. Production August and older months need separately authorized backfills. Automatic future collection is implemented under #1421; correction refresh remains separate under #1423. A later team detail can show a 6–12 month series, amount per contributing member and changes in contributor count, using the same gross/net basis and five-contributor suppression at each point. Missing or suppressed points are gaps, never zeros.
2. Cost-driver breakdown: aggregate SKU/model gross, discounts and net at a team/month grain, suppressing subgroups with fewer than five contributors. Membership overlap still applies; do not present team sums as budget shares. Current per-user ingestion stores SKU totals, not model detail, so model breakdown requires retaining that source detail first.
3. Team buckets: start with descriptive, overlapping categories such as increased/stable/decreased consumption and team-size bands; use absolute plus relative thresholds and compare matched, complete periods. An overview may count teams but should not sum their spend as organizational spend. Product-area buckets need a verified team-to-area map; Teamkatalogen links are not one-to-one. No leaderboard or performance label.
4. Value context: link a team's spend discussion to its work and outcomes, with a short team-provided explanation rather than claiming activity counts are ROI. Do not infer time saved or productivity from credits, generated lines or request counts. Distinguish recurring work, experimentation and shared service usage where teams can supply that context.

- Code: `apps/copilot-metrics/billing.go`, `apps/copilot-metrics/views/v_team_daily_summary.sql`, `apps/copilot-api/bigquery_stats.go`, `apps/my-copilot/src/app/(nb)/statistikk/page.tsx`; read-only prototype: `docs/copilot-team-spend-allocation.sql`.
- Data: read-only BigQuery queries of `copilot_metrics` in dev and prod, completed September billing loads in both, and the interrupted August dev load. No user identities or individual figures are recorded here.
