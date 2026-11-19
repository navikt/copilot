# MAI Code 2 Flash: regnskap

Plan: [2026-12-mai-flash-plan.md](2026-12-mai-flash-plan.md). Ramme 650 credits.

| Tidspunkt (UTC) | Trinn/arm | Reservert | Faktisk | Sum brukt | Merknad |
| --- | --- | --- | --- | --- | --- |
| 2026-11-19 17:05 | Trinn 2, forhåndssjekk | 0 | 0 | 0 | Stopp før kjøring, se under |

## Estimat for neste steg (20. november 2026)

Delgrunnlinje t4/t8 med 6.1 Sol: ≈ 130 (oppgaver, dommer og probe), pluss ≈ 7 når blindsonedommeren kontrollerer seg selv på nytt etter at #2475 endrer skript-hashen. Reservasjon ≈ 137. Starter først når #2475 og #2473 er merget.

## Stopp 19. november 2026: grunnlinjen kan ikke gjenbrukes

- Copilot CLI har oppdatert seg selv til `1.0.115-0` (pakkebuffer 19. november 17:30). Begge grunnlinjene for 6.1 Sol er målt på `1.0.114-0`. Ny klientversjon gjør alle testene berørt, og ny kjøring av grunnlinjen (≈ 590) krever eierbeslutning.
- Nærmeste grunnlinje er `2026-11-regresjon2-gpt-6.1-sol-baseline` (revisjon `d421991a`, med dommeren fra #2437). Fra `d421991a` til `origin/main` er bare rust-fiksturene og rust-delen av harnesset endret, ikke `planning` eller dommeren. Planen nevner `2026-11-regresjon-…` (revisjon `6fbce9a6`), som har eldre dommer for t8.
- `copilot --no-auto-update --version` gir fortsatt `1.0.115-0`.
- Planen på `main` har fortsatt status «utkast, ikke låst», og åpen beslutning 1 (start uten siling) er ikke ført inn.
