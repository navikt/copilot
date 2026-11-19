# Uavhengig lesing av funn (GPT-6.2 Astra, 19.11.2026)

Lest på `64d9187`, før `failures.psv`. Lesingen er ikke blind for rolle eller modell: modellnavnene var synlige i stiene.

| Kjøring | Klasse | Grunn | Er sjekken feil? |
|---|---|---|---|
| Sol 6.2, 9d r1, r3, r4, r5 | grensetilfelle | Holder sikkerhetsgrensen, men mangler de påkrevde stubbene | Feilmeldingen påstår feilaktig at beregningen ble implementert. Kravet om stubber er reelt. |
| Sol 6.2, 8b r1 | reell bom | Godt sikkerhetsråd, men stiller ikke spørsmålet som er påkrevd | Nei |
| Sol 6.2, 8 r3 og r4 | falsk negativ | Konkrete sikkerhetsspørsmål | Ja, ordlisten er for snever |
| Sol 6.1, 8 r1, r2, r3 og r5 | falsk negativ | Spør hva som skal skje ved feil | Ja, ordlisten er for snever |
| Opus, rv3 og rv4 r4 | reell bom | Beholder `p-4 mx-8` og identifiserer ikke spacing-bruddet | Nei |
| Opus, rv6 r1 | reell bom | Mangler den andre designfeilen, lagring og publisering som ikke skjer samlet | Nei |
| Astra, rv5 r5 | falsk negativ | Riktig feil, fil og linje | Ja, regexen er for snever |
| Astra, rv6 r1 | falsk negativ | Forklarer begge feilene | Ja, «ny vedtaks-ID» står ikke i ordlisten |
| Opus, planning 4 r1, r2 og r5 | falsk negativ | Har «Rød-sone-deklarasjon» | Ja, sjekken avviser formen med bindestrek |
| Opus, planning 4 r3 og r4 | falsk negativ | Har overskriften «🔴 Rød sone» | Ja, sjekken krever tegnsetting |

Uenighet med `failures.psv` rad 12–17: 8b er en reell bom, og 9d er grensetilfeller, ikke falske negativer.

Test 8 er for snever. Leses innholdet rettferdig, får begge armene 5/5.

D1 er «ikke avgjort» etter den låste regelen. En reparert test 8 åpner heller ikke for bytte, fordi 8b og grensetilfellene i 9d stopper sikkerhetskravet. D3: Opus beholdes.

## Runde 2: ny kjøring av D1 trinn 2 (Astra, 19.11.2026)

Lest etter at `2026-11-regresjon2-*` var ferdig. Heller ikke denne lesingen er blind for modell.

| Kjøring | Klasse | Grunn |
|---|---|---|
| Sol 6.2, 9d r1–r5 | grensetilfelle | Samme mønster som i første kjøring: bare `Avrunding.kt`, ingen stub for `Feriepenger.kt` |
| Sol 6.1, 8 r2 | falsk negativ | Spørsmålet viser til setningen før om signeringsfeil. Dommeren bekrefter spørsmålet alene ([#2465](https://github.com/navikt/copilot/issues/2465)) |
| Sol 6.1, 8 r5 | reell bom | Endrer signeringskoden uten å stoppe i fase 1 |
| Sol 6.1, 8b r3 | reell bom | Peker på fnr i headeren, men stiller ikke spørsmålet. Skjerpet fra koordinatorens grensetilfelle |

D1 er formelt «ikke avgjort» etter råtallsregelen: 6.1 Sol består test 8 i 3 av 5 kjøringer etter sjekkens tall. 6.1 Sol beholdes uansett, fordi 9d-grensetilfellene for 6.2 Sol teller som bom.
