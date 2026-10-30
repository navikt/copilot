# Innsikt restructure plan

Status: revised 2026-10-19 after Fable review. Scope: `apps/my-copilot/src/app/(nb)/innsikt/**`
and the Innsikt overview cards.

## Problem

| Page | Lines | Mixes |
|---|---|---|
| `bruk` | 371 | key figures, cost, adoption phases, PRs/review, repo table |
| `tilpasninger` | 511 | repo customisation inventory + nav-pilot telemetry (`#faktisk-bruk`) |
| `lokale-modeller` | 652 | reference (approved/rejected models) + benchmark results + observed usage |

- Public cost appears on Bruk (`#kostnad`) and Trender (`#modellfamilier`,
  `#ai-credits-per-bruker`). Internal cost lives on `/admin/kostnad`.
- PRs: Bruk "Pull requests og code review" and Trender "Copilot i pull requests"
  use different labels for related numbers.
- Card titles and texts are uneven ("Teaminnsikt", vague Tilpasninger card).

Not problems (checked):

- Bruk "KI-adopsjonsfaser" (`/adoption/cohorts`, phases by tool use) and Trender
  "Blir brukerne værende?" (`/usage/cohort-retention`) answer different questions.
- `/innsikt/forbruk` is deliberately unlinked and internal
  (copilot-intern beslutningslogg.md:81-84, kostnadsinnsikt PRD S5, E6). It stays as is.
- `/innsikt/kostnad` is a permanent redirect to `/admin/kostnad` (next.config.ts:91)
  with inventoried anchors. It must not become a page.
- Tilpasninger `#synkronisering` reads `/adoption/staleness` (repo copies vs source),
  so it belongs to the inventory, not to nav-pilot telemetry.

## Principles

- One topic per page; the overview card states the question it answers.
- Bruk = how many, where and what it costs now. Trender = how it changes.
- Never break links: permanent redirect per moved path (no chains, next.config.ts:81),
  legacy anchor per moved anchor (`src/lib/legacy-anchors.ts`; a legacy key must not
  also be a real id). `src/link-inventory.test.ts` only sees `InsightSection` and
  `LinkableHeading` ids, so pages with local wrappers need their anchors guarded first.
  Some links are invisible to the guard and are checked by hand: month-insights.ts:351,
  praksis/sections/cost-optimization.tsx:183, ordliste/terms.ts:279, cost-anomalies.tsx.
- A new private page goes into `PRIVATE_PAGE_PATHS` (src/lib/private-paths.ts) and matcher (src/proxy.ts),
  `PRIVATE_ROUTES` (scripts/auto-login-ignore-paths.mjs), and app.yaml
  `autoLoginIgnorePaths`. The sitemap lists public pages only.
- Norwegian copy goes through språkvask before merge.

## Phases (one PR each)

1. **Copy and loose ends.** Rename "Teaminnsikt" to "Team"; clearer card texts; overview
   description without a page list; move `report-controls` to `src/components/`
   (five import sites). Branch `innsikt-phase1`.
2. **Guard Trender anchors.** Switch Trender's local `Section` wrapper to `InsightSection`
   (or inventory its anchors) so later moves are caught by CI.
3. **Cost into Bruk.** Move Trender `#modellfamilier` and `#ai-credits-per-bruker` into
   Bruk's cost section; legacy anchors from Trender; Trender links to Bruk. Align PR
   section labels between Bruk and Trender. No new route.
4. **nav-pilot i bruk.** Move `#faktisk-bruk` and the other nav-pilot telemetry from
   Tilpasninger to a protected `/innsikt/nav-pilot`; legacy anchors; check
   insight-strip.tsx:53 and daily-fact.ts:19 copy.
5. **Lokale modeller split.** Results and observed usage to `/innsikt/lokale-modeller/resultater`
   (public: sitemap + app.yaml). Anchor-by-anchor destination table first:
   `#delegeringsnivaer`, `#modell-64-gb`, `#malt-decide`, `#modeller-per-minne` are linked
   from nav-pilot pages and news. The logged-in-only section (page.tsx:215-217) stays
   on whichever page keeps it.

Each PR: `mise run check` in `apps/my-copilot`, link-inventory green, visual check, review.

## Owner decisions

- 2026-10-19: public and private pages stay mixed under `/innsikt/`; lock icon marks private.
- 2026-10-19: nav-pilot telemetry moves to protected `/innsikt/nav-pilot`.
- Forbruk: the 2026-10-19 "merge into Kostnad" answer rested on a wrong premise; the
  logged decision (own internal page, unlinked) stands unless the owner says otherwise.

## UI consistency (review 2026-10-19, code-only)

Separate PR track.

- **U1:** `/priser` inline hex colours (`priser/page.tsx:44-79`) break dark mode; h3 with
  no h2 (`:47`). `lokale-modeller:98` raw `<a>`; use Aksel `Link`. Branch `innsikt-ui-u1`.
- **U2:** one chart skeleton (heights now 300/320/custom); `ErrorState` + Suspense on
  lokale-modeller, priser, reisen; UTC dates via shared helpers (`bruk:59` lacks `timeZone`).
- **U3:** priser, reisen, undersokelser/[id] and the hub use PageHero with ad-hoc widths;
  move to `InsightPage`/`InsightSection`. team/rapport keeps its print shell.
- **U4:** one k-anonymity note wording, one period/month picker (keep query params),
  verify every chart sits in `ChartFigure`.
