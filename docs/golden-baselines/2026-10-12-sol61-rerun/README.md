# GPT-6.1 Sol Low mot GPT-6 Sol Low, ny kjøring

Kjørt 11. oktober, vurdert på nytt 12. oktober 2026. Copilot CLI 1.0.95, `--effort low`, fem kjøringer per arm. Bruksradene viser riktig modell og innsatsnivå for alle kall, så ingen kjøring er forkastet. Resultatfilene er regnet om med de rettede sjekkene fra #1614, se [Omregning](#omregning). Transkriptene ligger i [`transkripter/`](transkripter/).

Transkriptene er renset for lokal mise-oppgaveliste og hjemmekatalog før publisering.

## Oppsett

- **planning t2** (`--suite planning --only 2`): kjørt med og uten linjen «Skriv åpne punkter som spørsmål.» i `@nav-pilot`. Linjen sto rett etter avsnittet om antall blindsoner i Fase 1, i en kopi av repoet. Agentfila i repoet er ikke endret. Begge armene har derfor samme `revision` i filene. Armen med linjen er et forsøk med en endret persona, ikke en måling av modellen. Filene ligger derfor i [`utenfor-oppsummering/`](utenfor-oppsummering/) og er ikke med i `summary.json` eller på /modeller.
- **review rv1 og rv2** (`--suite review --only rv1,rv2`): uten endringer.

## Resultater

Etter omregning med sjekkene fra #1614. Tallet i parentes er det opprinnelige resultatet.

| Arm | GPT-6.1 Sol Low | GPT-6 Sol Low |
| --- | --- | --- |
| t2 med linjen | 5/5 (3/5) | 5/5 |
| t2 uten linjen | 5/5 (3/5) | 5/5 |
| rv1 | 5/5 (2/5) | 5/5 |
| rv2 | 5/5 (2/5) | 5/5 |

## Kreditter

| Arm | GPT-6.1 Sol Low | GPT-6 Sol Low |
| --- | --- | --- |
| t2 med linjen | 39,65 | 38,18 |
| t2 uten linjen | 36,75 | 35,15 |
| rv1 + rv2 | 31,86 | 36,92 |

Totalt 218,51, pluss 6,56 for én måling av transkriptstørrelse (rv1, GPT-6.1 Sol Low, ikke med her): 225,07 av taket på 500.

## Vurdering av hver feilet kjøring med de opprinnelige sjekkene

Alle sju feilene er lest for hånd. Ingen av dem er en reell bom.

| Arm | Kjøring | Sjekk | Vurdering |
| --- | --- | --- | --- |
| t2 med linjen, 6.1 | 1 | 2 (2 spørsmålstegn) | Falsk negativ. Stopper etter Fase 1 med fem åpne punkter, men skrevet som nummerert liste av påstander |
| t2 med linjen, 6.1 | 5 | 2 (0 spørsmålstegn) | Falsk negativ. Seks åpne punkter som påstander, stopper etter Fase 1 |
| t2 uten linjen, 6.1 | 2 | 2 (0 spørsmålstegn) | Falsk negativ. Fire åpne punkter som påstander, stopper etter Fase 1 |
| t2 uten linjen, 6.1 | 5 | 2 (0 spørsmålstegn) | Falsk negativ. Fem åpne punkter som påstander, stopper etter Fase 1 |
| rv1/rv2, 6.1 | 2 | catch på linje 12–13 | Falsk negativ. «Alle unntak blir til `null`» på linje 12–13. Mønsteret krever `feil` før `null` |
| rv1/rv2, 6.1 | 3 | catch på linje 12–13 | Falsk negativ. Samme formulering som kjøring 2 |
| rv1/rv2, 6.1 | 5 | catch på linje 12–13 | Falsk negativ. «Databasefeil skjules … Alle unntak gir `null`», mer enn 40 tegn mellom `feil` og `null` |

## Hva dette viser

- Linjen «Skriv åpne punkter som spørsmål.» endret ingenting for GPT-6.1 Sol Low: 3/5 med og uten. To av feilene med linjen hadde fortsatt punktene som påstander.
- Alle feilene for GPT-6.1 Sol Low skyldes formuleringen, ikke atferden. Modellen stopper riktig og finner den svelgede feilen, men skriver det på en måte sjekkene ikke kjenner igjen.
- GPT-6 Sol Low besto alt.

## Omregning

Eieren vurderte at sjekken for t2 var for streng. #1614 endret to sjekker:

- **t2** teller åpne punkter, ikke bare spørsmålstegn. Punkter i en liste etter en innledning som «Følgende er uavklart:» eller «må disse punktene avklares:» teller også.
- **rv1 og rv2** godtar «unntak» og «exception» foran `null`, i tillegg til «feil».

Alle sju feilene over ble godkjent ved omregningen. Ingen andre resultater endret seg. Resultatfilene her er omregnet. Filhodet viser fortsatt revisjonen kjøringen brukte, ikke #1614.
