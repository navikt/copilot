# Modellvalg november 2026: regnskap og fremdrift

Plan: [2026-11-modellvalg-plan.md](2026-11-modellvalg-plan.md), låst på `ac35a630` (#2396).
Myk ramme: ~~5 700~~ 6 500 credits (eierbeslutning 18. november 2026, se tillegget i planen). Gren: `benchmark-2026-11-run`.

Regel 5 (eierbeslutning 18. november 2026): stopp hvis faktisk > 1,5 × (estimat + 5,91).

Faktisk kostnad per arm = agentkall (`-usage.psv`) + dommer (`-judge-credits.psv`) + innloggingsprobe (målt i trinn 0) per kjøring av `nav-pilot-golden.sh`.

## Trinn 0

| Post | Credits | Merknad |
| --- | --- | --- |
| Innloggingsprobe (måling) | 5.91 | `copilot -p "svar kun med ordet OK"`, standardmodell `gpt-6.1-sol`/`medium`, 23,6k tokens skrevet til cache. Settes til 5.91 per kjøring av testoppsettet. |
| Blindsonedommer, kontroller | 6.70 | 73/75, `pass` |
| Røykprøver, agentkall | 163.74 | 7 modeller, `research` n = 1 (se tillegget i planen) |
| Røykprøver, probe | 41.37 | 7 × 5.91 |
| **Sum trinn 0** | **217.72** | Estimat ≈ 25. Astra (67,3) og Opus (49,3) per kjøring står for mest. |

Brukt etter trinn 0: **217.72**.

## Fremdrift

- Trinn 0 ferdig (utfall i tillegget i planen).
- 18. november 2026, eierbeslutninger (avvik fra planen, se tillegget): proben legges til estimatet i regel 5, rammen er 6 500, og D1 trinn 1 prøves én gang til. Andre forsøk: krav 3 (kostnad per oppgave) holder ikke, så D1 stopper etter planen. Eieren avgjør før trinn B.
- Trinn A kjøres én arm om gangen med `run-arms.sh` (lokal hjelpefil): stopper ved reservasjon over rammen, last > 8, ufullstendige bruksdata, feil modell/nivå/klient, forkastede kjøringer eller regel 5.
- Fortsette for hånd: legg neste linje fra planens matrise A inn i `2026-11-modellvalg-a.matrix` (`<mai>` = `low`), legg inn reservasjonen nederst i tabellen, kjør `COPILOT_AUTO_UPDATE=false python3 scripts/benchmark-matrix.py docs/golden-baselines/2026-11-modellvalg-a.matrix --jobs 1`, og kontroller og regn etter planen.
- Brukt hittil = brukt etter trinn 0 + summen av «Faktisk» i tabellen under.

## Trinn A


| Arm | Estimat | Reservert (brukt + estimat) | Agent | Dommer | Probe | Faktisk | Bestått | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| coding mai-code-2-flash low | 5 | 222.72 | 3.58 | 0 | 5.91 | 9.49 | 30/30 sjekker, 15/15 oppgaver | ferdig; stoppet først på regel 5 (7,5), innenfor etter eierbeslutningen (1,5 × 10,91 = 16,4) |
| coding mai-code-1.1-flash low | 20 | 253.12 | 13.66 | 0 | 5.91 | 19.57 | ko1–ko6 5/5 | ferdig |
| coding mai-code-2 low (avbrutt) | 47 | 299.69 | 3.74 | 0 | 5.91 | 9.65 | – | avbrutt i run 1 da OpenCode-serveren startet på nytt 19:34 UTC. Ingenting lagret. Agentkostnaden er hentet fra `session-store.db` (11 kall, `mai-code-2`/`low`, 19:33:49–19:34:31 UTC). Armen kjøres på nytt fra start |
