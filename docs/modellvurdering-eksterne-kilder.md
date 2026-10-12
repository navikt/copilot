# Eksterne modellmålinger og pinneprotokollen

Skrevet 12. oktober 2026 som grunnlag for den trinnvise [pinneprotokollen](pin-protokoll.md). Der notatet og protokollen er uenige, gjelder protokollen. Alle kilder er lest 12. oktober 2026 med mindre annet står. «Uverifisert» betyr at påstanden bare kommer fra sekundærkilder eller at vi ikke fikk lest primærkilden.

## Kort fortalt

Eksterne indekser er gratis for oss og dekker mange modeller og innsatsnivåer. De sier lite om hvordan `@nav-pilot` oppfører seg med våre instruksjoner, røde soner, personvern og norsk. De passer derfor som **silingsport før vi bruker credits**, og for å velge når to kandidater står likt i våre egne målinger. De kan aldri brukes alene til å godkjenne et bytte.

## Kildene

**Artificial Analysis (AA).** Intelligence Index v4.3.2 vekter agentoppgaver med 30 %, koding med 20 % (Terminal-Bench 4.0 og SciCode), generelle oppgaver (kunnskap, dokumenter, lang kontekst) med 30 % og naturvitenskapelig resonnering med 20 %. Antall gjentak per oppgave er 1, 3 eller 5 alt etter test. Bare Elo-delene har 95 %-intervall. Selve indeksen har ingen intervaller. [https://artificialanalysis.ai/methodology/intelligence-benchmarking] Kostnaden regnes ut fra tokentellingene som leverandørens API oppgir, med målt cache-andel. Tid per oppgave er for noen tester beregnet fra tokenbruk og skrivehastighet, ikke målt fra start til slutt (samme kilde). Gratis-API-et krever kildehenvisning og tillater 1 000 kall i døgnet [https://artificialanalysis.ai/api-reference]. Hvor ofte indeksen oppdateres står ikke oppgitt. Versjonsnummeret endres når sammensetningen endres, så tall fra ulike versjoner kan ikke sammenlignes.

**Vilkårene til Artificial Analysis.** Nettstedets vilkår (Website Terms of Use, versjon 1.0, revidert 28. april 2024) sier i §2.2(d): «except as expressly stated herein, no part of the Site may be copied, reproduced, distributed, republished, downloaded, displayed, posted or transmitted in any form or by any means». §3.3(b)(vi) forbyr å «use software or automated agents or scripts … to strip, scrape, or mine data from» nettstedet [https://artificialanalysis.ai/docs/legal/Terms-of-Use.pdf]. Vi henter derfor tall bare fra gratis-API-et, med kildehenvisning, og publiserer bare utfallet av silingen, indeksversjonen, datoen og en lenke. Øyeblikksbilder med tallene lagres i copilot-intern, ikke her. Vilkårene for dataplattformen (Data Platform Terms, https://artificialanalysiscdn.com/legal/ProDataPlatformTerms.pdf) er ikke lest ennå og må leses før vi tar i bruk API-et.

**slopalytics.com** viser AA-data (bunnteksten lenker til artificialanalysis.ai). Den har grafer for kostnad mot intelligens og tid per oppgave mot intelligens, en «Pareto line» som viser varianter ingen andre slår på både pris og score, og et valg mellom logaritmisk og lineær akse. Hver modell har én variant per innsatsnivå (none/low/medium/high/xhigh/max). Varianter merket `estimated` har beregnede verdier. Lisens er ikke oppgitt. Vi bruker siden til å se på dataene, ikke som kilde vi siterer.

**SWE-bench Verified og Pro.** Begge måler om agenten løser ekte GitHub-saker, kontrollert av skjulte tester. Verified har et «bash only»-brett med den samme enkle agenten (mini-SWE-agent) for alle modeller [https://www.swebench.com/], og koden er MIT-lisensiert [https://github.com/SWE-bench/SWE-bench]. Pro henter oppgavene fra GPL-lisensierte kodebaser og private kodebaser for å redusere lekkasje til treningsdata, og har en skjult delmengde [https://scale.com/leaderboard/swe_bench_pro_public]. Vi har ikke verifisert om noen av dem oppgir intervaller eller antall gjentak. Verified bygger på offentlige Python-repoer fra før 2024 og er sannsynligvis lekket til treningsdata.

**Aider polyglot.** Består av 225 Exercism-oppgaver i seks språk. Modellen får to forsøk (`pass_rate_2`), og tabellen viser kostnad og innsatsnivå (`reasoning_effort`). Tabellen har ingen intervaller. Siste dato i datafilen er 3. oktober 2025, så brettet er i praksis foreldet [https://aider.chat/docs/leaderboards/; https://raw.githubusercontent.com/Aider-AI/aider/main/aider/website/_data/polyglot_leaderboard.yml]. Kildekoden er Apache 2.0 (uverifisert for selve dataene).

**METR.** «Tidshorisont» er lengden på en oppgave, målt i hvor lang tid et menneske bruker, der modellen lykkes i 50 % av forsøkene. Den finnes ved logistisk regresjon og oppgis med bootstrap-intervall [https://arxiv.org/abs/2503.14499]. Intervallene er brede, for eksempel 1 t 49 min til 20 t 25 min for en toppmodell. Kritikere mener at 80 %-horisonten og ekstrapolasjonen er svakt fundert [https://www.greaterwrong.com/posts/sBEzomgnYJmYHki9T/]. Det er uverifisert at hver oppgave kjøres om lag åtte ganger. Metoden er nyttig som mal, ikke som silingstall.

**Epoch AI Benchmarking Hub.** Kjører mange tester selv, med flere gjentak (for eksempel 8 på MATH Level 5 og 16 på GPQA Diamond). Figurene viser ±1 standardfeil. Data kan lastes ned som CSV under CC BY, og siden var oppdatert 12. oktober 2026 [https://epoch.ai/benchmarks]. Hubben dokumenterer også at leverandørens egne tall avviker fra egne målinger: Claude 3.5 Sonnet ble oppgitt til 65 % på GPQA Diamond, mens Epoch målte 55 ± 3 % (samme kilde).

**LMArena.** Bygger på parvise stemmer fra brukere, med Bradley-Terry-modell og 95 %-intervall. Rangering gis bare når intervallene ikke overlapper. Det finnes også et brett med stilkontroll for lengde og formatering. Koden (Arena-Rank) er Apache 2.0 og bruker intervaller beregnet i lukket form [https://news.lmarena.ai/arena-rank]. Toppen av rangeringen kan endres ved å fjerne en håndfull stemmer [https://arxiv.org/pdf/2508.11847]. Brettet måler hva brukere liker, ikke om oppgaven blir løst, og er lite relevant for oss.

**Terminal-Bench 4.0.** Måler agentoppgaver i terminal. Brettet viser andelen løste oppgaver med 95 %-intervall, for eksempel «64,8 % ± 3,1 %», i tillegg til kostnad i USD og tokens. Hver oppføring er et par av agent og modell [https://www.tbench.ai/leaderboard]. Antall forsøk per oppføring og hvordan de fordeler seg på oppgaver og gjentak, er ikke verifisert. Siden ber om at testdataene aldri havner i treningsdata og har en GUID som kan brukes til å oppdage lekkasje.

**Gjentatte kjøringer og intervaller.** τ-bench innfører **pass^k**: sannsynligheten for at alle k forsøk på samme oppgave lykkes. Målet faller kraftig med k selv når pass@1 ser bra ut [https://arxiv.org/abs/2406.12045]. Miller (2024) anbefaler standardfeil gruppert per oppgave, parede sammenligninger på samme oppgaver og gjentak for å redusere variansen innenfor hver oppgave [https://arxiv.org/abs/2411.00640].

## (a) Slik kan eksterne tall brukes i den trinnvise protokollen

**Silingsport, gratis og først.** En kandidat går videre til betalt måling bare hvis:

1. **AA Intelligence Index**, på **samme innsatsnivå** som pinnen bruker (i dag Low), er minst like høy som for den pinnede modellen, med en toleranse på −1 poeng fordi indeksen ikke har intervaller.
2. **AA-kodingsindeksen** ikke er lavere. Bare hvis AA mangler kandidaten, bruker vi Terminal-Bench 4.0 med samme agent, og godtar et resultat innenfor den pinnede modellens 95 %-intervall.
3. **Kostnad per oppgave** (fra AA-API-et) ikke er høyere.
4. Kandidaten **ligger på eller nær Pareto-linjen** for kostnad mot intelligens. Dette er en anbefaling, ikke et krav.

**Innsatsnivå.** Sammenlign alltid samme nivå mot samme nivå. Hvis kandidaten bare består på et høyere nivå, er forslaget «kandidat på high», og kostnaden vurderes på det nivået. Oppføringer merket `estimated` teller ikke.

**Foreldede tall.** Bruk bare tall fra samme indeksversjon og høyst 30 dager gamle. Aider-brettet (siste data 3. oktober 2025) brukes ikke. Lagre et øyeblikksbilde med dato og versjon i copilot-intern.

**Modeller som mangler i indeksene.** Siling er da «ikke mulig». Mindre versjoner kan likevel gå til regresjonssjekken hvis leverandøren dokumenterer forbedring, men da merkes saken med at silingen mangler. Nye modellfamilier uten eksterne tall må vente til de finnes i indeksene.

**Forhåndsantakelse og stopperegler.** Stoppegrensene i den sekvensielle t7b-målingen fastsettes på forhånd og påvirkes ikke av indeksen. Vi bør ikke bruke formelle Bayesianske priorer laget fra indeksdata. Sammenhengen mellom indeksen og t7b er ikke målt.

**Når to står likt.** Står to kandidater likt i våre egne målinger, velger vi den med lavest kostnad per oppgave og deretter høyest kodingsindeks.

**Kan aldri avgjøres med eksterne tall alene:** rød sone og t7b, personvern, norsk språk, rød sone (9, 9c), fasedisiplin, at agenten følger våre skills og verktøy, og oppførsel i `cplt`. Eksterne tall kan bare stoppe en kandidat, ikke godkjenne den.

**Trinnvis utrulling.** Praksisen vi fant kommer fra blogger, ikke standarder: først skyggetrafikk, deretter 1 → 5 → 20 → 50 → 100 % av brukerne, med fast fordeling per bruker og ikke per forespørsel. Tilbakerulling skjer ved å endre et flagg. Et eksempel på utløser er kvalitet som er mer enn 5 % dårligere, eller en feilrate over 1,5 × grunnlinjen [https://tianpan.co/blog/2026-04-20-canary-deploys-llm-model-upgrades; https://hidekazu-konishi.com/entry/safe_foundation_model_rollout_on_aws.html]. Med vår telemetri bør vi følge med på:

- credits per økt
- fullførte økter og avbrutte økter
- verktøyfeil
- omstarter og regenerering
- økter som stopper i Fase 1 uten videre arbeid
- svarlengde
- avslag
- treff i rød sone og blokkeringer fra guards

Fordelingen bør være per bruker, med en opt-in-gruppe først.

## (b) Hva vi bør ta inn i egen metode og presentasjon

- **pass^k ved siden av andel løste oppgaver.** For n = 5 per oppgave kan vi rapportere pass^k for k < n. Det fanger opp ustabilitet som «4 av 5» skjuler.
- **Intervaller på `/modeller`.** Vis Wilson-intervall for hver andel og gruppér standardfeilen per oppgave slik Miller anbefaler. Sammenligner vi mot en gjenbrukt grunnlinje, må vi oppgi klientversjonen den ble målt med.
- **Kostnad og tid per løst oppgave**, ikke bare per forsøk, slik Terminal-Bench gjør.
- **Innsatslinjer og Pareto-linje på log-skala.** Vi har innsatslinjer allerede. En Pareto-linje og log-akse, slik slopalytics viser dem, er enkle å legge til.
- **Testdata med GUID** i golden-fixtures, slik at vi kan oppdage om de lekker til treningsdata.

## (c) Risiko

- **Ulik agent.** Indeksene bruker sine egne agenter (AA bruker Stirrup, SWE-bench bruker mini-SWE-agent), ikke `cplt` og nav-pilot. Samme modell kan oppføre seg annerledes hos oss.
- **Leverandørens egne tall.** Epoch viser avvik på 10 prosentpoeng. Bruk bare uavhengige målinger.
- **Lekkasje til treningsdata.** Verified er sannsynligvis lekket, og Aider er offentlig og gammel. Pro, Terminal-Bench og AA er nyere, men ikke immune.
- **Indeksen endres.** En ny AA-versjon kan snu rekkefølgen mellom modeller. Lagre versjonen.
- **Ingen intervaller i AA.** En forskjell på 1–2 poeng kan være støy.
- **Gjenbrukt grunnlinje.** Klientversjonen eller fixture-revisjonen kan ha endret seg siden grunnlinjen ble målt. Det må sjekkes før sammenligning.

## Oppsummering

| Kilde | Oppgavetype | Gjentak / usikkerhet | Kostnad | Innsatsnivå | Lekkasje / foreldelse | Oppdatering | Gjenbruk |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AA Intelligence Index v4.3.2 | Blanding: agent, koding, kunnskap | 1–5 gjentak per test; intervall bare for Elo | USD per oppgave fra API-tokens | Egen variant per nivå | Versjoneres; noen tester er private | Løpende (ikke oppgitt) | Kildehenvisning, gratis-API |
| slopalytics | Visualisering av AA | Ingen intervaller | Kostnad per oppgave og per full kjøring | Linjer per nivå; `estimated` merket | Arver fra AA | Følger AA (uverifisert) | Ikke oppgitt |
| SWE-bench Verified | Ekte Python-saker | Ikke verifisert | Delvis | Ikke verifisert | Sannsynlig lekket | Løpende | MIT (kode) |
| SWE-bench Pro | Saker fra GPL- og private repoer | Ikke verifisert | Ikke verifisert | Ikke verifisert | Redusert ved valg av lisenser og skjult sett | Løpende | Ikke verifisert |
| Aider polyglot | 225 Exercism-oppgaver | 2 forsøk; ingen intervall | USD per kjøring | `reasoning_effort` oppgis | Foreldet (data fra 3. oktober 2025) | Stanset | Apache 2.0 (kode) |
| METR | Lange programvareoppgaver | Om lag 8 gjentak (uverifisert); bootstrap-intervall | Ikke sentralt | Ikke verifisert | Egne oppgaver | Per modell | Ikke verifisert |
| Epoch Hub | Mange tester | 8–16 gjentak; ±1 standardfeil | Ikke verifisert | Ikke verifisert | Avdekker avvik i leverandørtall | Daglig (12. oktober 2026) | CC BY, CSV |
| LMArena | Parvise brukerstemmer | Bradley-Terry, 95 %-intervall | Nei | Nei | Kan manipuleres | Løpende | Apache 2.0 (kode) |
| Terminal-Bench 4.0 | Agentoppgaver i terminal | Antall forsøk uverifisert; 95 %-intervall | USD, tokens | Par av agent og modell | GUID mot lekkasje | Løpende | Ikke verifisert |
| τ-bench / Miller | Metode | pass^k; gruppert standardfeil | – | – | – | – | arXiv |
