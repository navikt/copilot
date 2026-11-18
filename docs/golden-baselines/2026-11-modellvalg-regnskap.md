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
- **Trinn A ferdig 18. november 2026 kl. 23:00, brukt 626,60.** Alle 18 armene har fullstendige bruksdata, riktig modell, nivå og klient og ingen forkastede kjøringer. Ingen arm brøt regel 5. Feilede sjekker er klassifisert i `2026-11-modellvalg-a/failures.psv`. I `coding gpt-6.1-sol` kommer «1 kall fra andre økter» fra testoppsettets egen innloggingsprobe, som også bruker `gpt-6.1-sol`. Det er ikke en annen økt.
- Neste: B og C som én måling med n = 5 per arm (se tillegget fra kvelden 18. november i planen). Armfila står under «Trinn B+C».

## Trinn A


| Arm | Estimat | Reservert (brukt + estimat) | Agent | Dommer | Probe | Faktisk | Bestått | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| coding mai-code-2-flash low | 5 | 222.72 | 3.58 | 0 | 5.91 | 9.49 | 30/30 sjekker, 15/15 oppgaver | ferdig; stoppet først på regel 5 (7,5), innenfor etter eierbeslutningen (1,5 × 10,91 = 16,4) |
| coding mai-code-1.1-flash low | 20 | 253.12 | 13.66 | 0 | 5.91 | 19.57 | ko1–ko6 5/5 | ferdig |
| coding mai-code-2 low (avbrutt) | 47 | 299.69 | 3.74 | 0 | 5.91 | 9.65 | – | avbrutt i run 1 da OpenCode-serveren startet på nytt 19:34 UTC. Ingenting lagret. Agentkostnaden er hentet fra `session-store.db` (11 kall, `mai-code-2`/`low`, 19:33:49–19:34:31 UTC). Armen kjøres på nytt fra start |
| coding mai-code-2 low | 47 | 309.34 | 27.73 | 0.00 | 5.91 | 33.64 | ko1 5/5 ko2 5/5 ko3 5/5 ko4 5/5 ko5 5/5 ko6 5/5 | ferdig |
| coding gpt-6-luna medium | 9 | 304.98 | 9.94 | 0.00 | 5.91 | 15.85 | ko1 5/5 ko2 5/5 ko3 5/5 ko4 5/5 ko5 2/5 ko6 2/5 | ferdig |
| coding claude-haiku-5.5 low | 8 | 319.83 | 9.03 | 0.00 | 5.91 | 14.94 | ko1 5/5 ko2 5/5 ko3 5/5 ko4 5/5 ko5 5/5 ko6 5/5 | ferdig |
| coding gpt-6.1-sol low | 130 | 456.77 | 148.52 | 0.00 | 5.91 | 154.43 | ko1 5/5 ko2 5/5 ko3 5/5 ko4 5/5 ko5 5/5 ko6 5/5 | ferdig; 1 kall fra andre økter på samme modell i vinduet |
| research mai-code-2-flash low | 2 | 483.2 | 1.23 | 0.00 | 5.91 | 7.14 | re1 5/5 re2 5/5 re3 5/5 re4 5/5 | ferdig |
| research mai-code-1.1-flash low | 10 | 498.34 | 7.47 | 0.00 | 5.91 | 13.38 | re1 5/5 re2 5/5 re3 5/5 re4 5/5 | ferdig |
| research gpt-6-luna medium | 5 | 506.72 | 5.31 | 0.00 | 5.91 | 11.22 | re1 5/5 re2 5/5 re3 5/5 re4 5/5 | ferdig |
| research claude-haiku-5.5 low | 5 | 517.94 | 6.33 | 0.00 | 5.91 | 12.24 | re1 5/5 re2 5/5 re3 5/5 re4 5/5 | ferdig |
| kafka mai-code-2-flash low | 5 | 530.18 | 3.34 | 0.00 | 5.91 | 9.25 | kf1 4/5 kf2 5/5 kf3 5/5 kf4 5/5 | ferdig |
| kafka mai-code-1.1-flash low | 20 | 554.43 | 15.75 | 0.00 | 5.91 | 21.66 | kf1 4/5 kf2 5/5 kf3 5/5 kf4 5/5 | ferdig |
| kafka gpt-6-luna medium | 10 | 566.09 | 12.23 | 0.00 | 5.91 | 18.14 | kf1 5/5 kf2 5/5 kf3 5/5 kf4 5/5 | ferdig |
| kafka claude-haiku-5.5 low | 9 | 583.23 | 9.48 | 0.00 | 5.91 | 15.39 | kf1 5/5 kf2 5/5 kf3 5/5 kf4 5/5 | ferdig |
| rust mai-code-2-flash low | 3 | 592.62 | 1.81 | 0.00 | 5.91 | 7.72 | rs1 5/5 rs2 5/5 rs3 5/5 rs4 5/5 | ferdig |
| rust mai-code-1.1-flash low | 14 | 611.34 | 5.81 | 0.00 | 5.91 | 11.72 | rs1 5/5 rs2 5/5 rs3 5/5 rs4 5/5 | ferdig |
| rust gpt-6-luna medium | 7 | 616.06 | 6.87 | 0.00 | 5.91 | 12.78 | rs1 5/5 rs2 5/5 rs3 5/5 rs4 5/5 | ferdig |
| rust claude-haiku-5.5 low | 5 | 626.84 | 4.76 | 0.00 | 5.91 | 10.67 | rs1 5/5 rs2 5/5 rs3 5/5 rs4 5/5 | ferdig |

### Trinn A, foreløpig utfall (sjekkens egne tall)

Median credits per kjøring uten dommer og probe. Mann–Whitney er ensidig, kandidaten billigere. Holm er regnet over de åtte D2-hypotesene.

| Rolle | Innehaver | MAI Code 2 Flash | MAI Code 1.1 Flash |
| --- | --- | --- | --- |
| `@research` | Luna 1,08, alle 5/5 | 0,22, alle 5/5, p 0,004 (Holm 0,032) → **videre til beslutningsmåling** | 1,49, p 1,0 → behold |
| `@kafka` | Luna 2,29, alle 5/5 | 0,67, p 0,004, men kf-idem 4/5 < 5/5 → behold | 3,12, kf-idem 4/5 → behold |
| `@rust` | Haiku 0,93, alle 5/5 | 0,33, alle 5/5, p 0,004 (Holm 0,032) → **videre til beslutningsmåling** | 1,14, p 0,996 → behold |
| `@worker` (`coding`) | Luna 1,99, Go to filer 2/5 | 0,71, alle 5/5, p 0,004 (Holm 0,032) → **anbefaling, utforskende** | 2,76, p 1,0 → behold |

D5: MAI Code 2 i `coding`: alle oppgaver 5/5 mot 6.1 Sol 5/5, median 5,11 mot 29,27, p 0,004 → **anbefales** som rimelig koder.


## Trinn B+C (n = 5 per arm)

Armfil (kjøres i rekkefølge med `run-arms.sh`): D1 6.2 Sol (580), D1 6.1 Sol (580), review Opus (293), review Astra (1 000), planning Opus (254), planning Astra (575), norsk Opus (175), norsk Sonnet (93), accessibility Opus (285), accessibility Sonnet (144), coding 6.2 Sol (130). Estimatene er summen av B og C, og prognosen med probe er ≈ 4 175.

| Arm | Estimat | Reservert (brukt + estimat) | Agent | Dommer | Probe | Faktisk | Bestått | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-11-regresjon-gpt-6.2-sol | 580 | 1212.51 | 482.58 | 2.88 | 5.91 | 491.38 | 2 5/5 3 5/5 4 5/5 5 5/5 7b 5/5 7 5/5 8b 4/5 8 3/5 9b 5/5 9c 5/5 9d 1/5 9 5/5 | ferdig |
| 2026-11-regresjon-gpt-6.1-sol-baseline | 580 | 1703.89 | 580.89 | 3.05 | 5.91 | 589.85 | 2 5/5 3 5/5 4 5/5 5 5/5 7b 5/5 7 5/5 8b 5/5 8 1/5 9b 5/5 9c 5/5 9d 5/5 9 4/5 | ferdig; 1 kall fra andre økter på samme modell i vinduet |
| review claude-opus-5.5 low | 293 | 2006.74 | 318.26 | 0.00 | 5.91 | 324.17 | rv1 5/5 rv2 5/5 rv3 4/5 rv4 4/5 rv5 4/5 rv6 4/5 rv7 5/5 rv8 5/5 | ferdig |
| review gpt-6.2-astra low | 1000 | 3037.91 | 660.98 | 0.00 | 5.91 | 666.89 | rv1 5/5 rv2 5/5 rv3 1/5 rv4 0/5 rv5 4/5 rv6 4/5 rv7 2/5 rv8 5/5 | ferdig |
| planning claude-opus-5.5 low | 254 | 2958.8 | 288.50 | 0.75 | 5.91 | 295.16 | 2 5/5 3 5/5 4 0/5 5 5/5 | ferdig |
| planning gpt-6.2-astra low | 575 | 3574.96 | 549.36 | 0.85 | 5.91 | 556.12 | 2 5/5 3 5/5 4 5/5 5 5/5 | ferdig |
| norsk claude-opus-5.5 default | 175 | 3731.08 | 235.62 | 0.00 | 5.91 | 241.53 | no1 5/5 no2 5/5 no3 5/5 no4 5/5 | ferdig |
| norsk claude-sonnet-5.5 default | 93 | 3890.61 | 115.76 | 0.00 | 5.91 | 121.67 | no1 5/5 no2 5/5 no3 5/5 no4 5/5 | ferdig |
| accessibility-claude-opus-5.5-default | 285 | 4204.28 | 434.55 | 0.00 | 5.91 | 440.46 | uu1 5/5 uu2 5/5 uu3 0/5 uu4 5/5 uu5 5/5 | ferdig |

**Stopp 19. november 2026 kl. 00:5x, regel 5:** accessibility Opus kostet 440,46, over grensen 1,5 × (285 + 5,91) = 436,4 (agent 434,55, median 82,2 per kjøring mot planens ≈ 57). Brukt: **4 353,83**. Gjenstår: accessibility Sonnet (144) og coding 6.2 Sol (130), som venter på eieren. I D1 6.1 Sol kommer «1 kall fra andre økter» fra testoppsettets innloggingsprobe (`gpt-6.1-sol`).

