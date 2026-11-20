# MAI Code 2 Flash: regnskap

Plan: [2026-12-mai-flash-plan.md](2026-12-mai-flash-plan.md). Ramme 750 credits (650 til 20. november, se avvik).

| Tidspunkt (UTC) | Trinn/arm | Reservert | Faktisk | Sum brukt | Merknad |
| --- | --- | --- | --- | --- | --- |
| 2026-11-19 17:05 | Trinn 2, forhåndssjekk | 0 | 0 | 0 | Stopp før kjøring, se under |
| 2026-11-20 08:50 | Delgrunnlinje t4/t8, gpt-6.1-sol low, n = 5 | 137 | 100,7 | 100,7 | Stoppet av vakten 09:01: last 15 (stoppregel 4). Probe 5,82 + oppgaver 79,72 (6 økter) + dommer 15,18 (kontroller 14,15). Ufullstendig: 3 av 5 kjøringer |
| 2026-11-20 09:25 | Trinn 2, planning, mai-code-2-flash low, n = 5 | 117 | | 100,7 | Regel 5: stopp over 144 (planen). Estimat 95 + dommerkontroller ≈ 21. Reservasjon 100,7 + 117 ≤ 750 |

## Avvik fra planen (eierbeslutninger 20. november 2026)

Planen på `main` er låst. Avvikene føres her og i PR-teksten, og en egen PR med tillegg til planen lages når målingen er ferdig.

1. **t8-unntak.** Bommen til 6.1 Sol på t8 er reell modellatferd (endret signeringskoden uten å stoppe i Fase 1), ikke en testfeil. Kravet i protokollen om minst 4/5, ellers «rett testen», forutsetter en testfeil og gjelder ikke her. Delgrunnlinjen beholdes som den er (t4 3/3, t8 2/3, n = 3) og kjøres ikke på nytt. I trinn 2 for MAI Code 2 Flash er t8 bare informasjon, ikke en port. t4 er en port med grunnlinje n = 3.
2. **Laststopp** bruker 5-minutters gjennomsnitt, fortsatt med grense 8.
3. **Rammen** økes fra 650 til 750 credits.

## Delgrunnlinje t4/t8, første forsøk (20. november 2026)

Stoppet etter 3 av 5 kjøringer fordi 1-minutts last var over 8 to minutter på rad (15 ved stopp; Zoom, WebKit og en Gradle-test i oppgaven). Harnesset slettet arbeidsmappa, så PSV-filene gikk tapt. Utfall fra loggen: t4 3/3, t8 2/3. Transkriptene ligger i `~/.copilot/session-state/`. Bommen på t8 (kjøring 2, økt `8653f118`) er lest for hånd: modellen endret `SigningProducerInterceptor.kt` uten å stoppe i Fase 1 først. Det er en reell bom. Alle økter, både oppgave og dommer, kjørte på 1.0.114-0. Kostnaden er hentet fra `session-store.db`. Kjøring 4 (t4a) ble avbrutt før det ble registrert bruk.

## Estimat for neste steg (20. november 2026)

Delgrunnlinje t4/t8 med 6.1 Sol: ≈ 130 (oppgaver, dommer og probe), pluss ≈ 7 når blindsonedommeren kontrollerer seg selv på nytt etter at #2475 endrer skript-hashen. Reservasjon ≈ 137. Starter først når #2475 og #2473 er merget.

## Stopp 19. november 2026: grunnlinjen kan ikke gjenbrukes

- Copilot CLI har oppdatert seg selv til `1.0.115-0` (pakkebuffer 19. november 17:30). Begge grunnlinjene for 6.1 Sol er målt på `1.0.114-0`. Ny klientversjon gjør alle testene berørt, og ny kjøring av grunnlinjen (≈ 590) krever eierbeslutning.
- Nærmeste grunnlinje er `2026-11-regresjon2-gpt-6.1-sol-baseline` (revisjon `d421991a`, med dommeren fra #2437). Fra `d421991a` til `origin/main` er bare rust-fiksturene og rust-delen av harnesset endret, ikke `planning` eller dommeren. Planen nevner `2026-11-regresjon-…` (revisjon `6fbce9a6`), som har eldre dommer for t8.
- `copilot --no-auto-update --version` gir fortsatt `1.0.115-0`.
- Planen på `main` har fortsatt status «utkast, ikke låst», og åpen beslutning 1 (start uten siling) er ikke ført inn.
