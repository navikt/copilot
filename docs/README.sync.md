# Keeping customizations up to date

Denne siden er flyttet til [Synkronisere](https://ki-utvikling.nav.no/nav-pilot/guider/synkronisere) på nettsiden. Overskriftene under
er beholdt så gamle lenker fortsatt virker. De to siste avsnittene finnes
ikke på nettsiden og står her inntil videre.

## Quick reference

Se [Synkronisere](https://ki-utvikling.nav.no/nav-pilot/guider/synkronisere).

## Automated sync (GitHub Actions)

Se [Automatisk sync](https://ki-utvikling.nav.no/nav-pilot/guider/synkronisere#automatisk-sync).

## How detection works

Se [Hvordan nav-pilot finner filer](https://ki-utvikling.nav.no/nav-pilot/guider/synkronisere#hvordan-nav-pilot-finner-filer).

## Files you have edited

Se [Hvordan nav-pilot finner filer](https://ki-utvikling.nav.no/nav-pilot/guider/synkronisere#hvordan-nav-pilot-finner-filer).

## Files that were there before install

Se [Hvordan nav-pilot finner filer](https://ki-utvikling.nav.no/nav-pilot/guider/synkronisere#hvordan-nav-pilot-finner-filer).

## Files the source removed

Se [Hvordan nav-pilot finner filer](https://ki-utvikling.nav.no/nav-pilot/guider/synkronisere#hvordan-nav-pilot-finner-filer).

## Overrides

Se [Tilpasse sync](https://ki-utvikling.nav.no/nav-pilot/guider/synkronisere#tilpasse-sync).

## Suppressing new-item reminders (user scope)

X tolerance

Markdown files (`.md`) are compared with formatting tolerance, so these differences are ignored:
- Line endings: CRLF vs LF
- Trailing whitespace per line
- Consecutive blank lines (collapsed to single blank line)

Your team can therefore run its own formatters, Prettier with different settings for instance, without getting false-positive update PRs. JSON files (`.json`) are still compared byte-for-byte.

## Staleness tracking

The [copilot-adoption](../apps/copilot-adoption/) scanner tracks whether each customization file across all `navikt` repos is in sync with the source. It compares git blob OIDs and stores an `in_sync` boolean per file in BigQuery, which powers the staleness dashboard.
