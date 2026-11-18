# Modellvalg november 2026: regnskap og fremdrift

Plan: [2026-11-modellvalg-plan.md](2026-11-modellvalg-plan.md), låst på `ac35a630` (#2396).
Myk ramme: 5 700 credits. Gren: `benchmark-2026-11-run`.

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

## Trinn A

| Arm | Estimat | Reservert (brukt + estimat) | Agent | Dommer | Probe | Faktisk | Bestått | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| coding mai-code-2-flash low | 5 | 222.72 | 3.58 | 0 | 5.91 | 9.49 | 30/30 sjekker, 15/15 oppgaver | ferdig; **over 1,5 × estimat (7,5) bare pga. proben → stopp etter regel 5** |

Brukt etter arm 1: **227.21**.

## Fremdrift

- Trinn 0 ferdig (utfall i tillegget i planen). D1 trinn 1 ikke fullført: mangler API-nøkkel til Artificial Analysis.
- Trinn A: arm 1 ferdig. **Stoppet etter regel 5** (arm > 1,5 × estimat). Årsak: innloggingsproben (5,91 per kjøring) er ikke med i estimatene. Agentkallene alene (3,58) var under estimatet.
- Venter på eieren: godkjenne at proben legges til estimatet per arm (eller at regel 5 vurderes uten proben). Uten det vil også `coding mai-code-1.1-flash` (20), alle `research`-armene, `kafka mai-code-2-flash` (5) og `rust`-armene med estimat ≤ 11 trolig bryte regel 5.
- Fortsette: legg neste linje fra planens matrise A inn i `2026-11-modellvalg-a.matrix` (`<mai>` = `low`), reserver i tabellen over, kjør `COPILOT_AUTO_UPDATE=false python3 scripts/benchmark-matrix.py docs/golden-baselines/2026-11-modellvalg-a.matrix --jobs 1`, kontroller og regn etter planen.
