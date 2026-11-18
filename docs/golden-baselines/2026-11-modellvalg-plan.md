# Modellvalg november 2026: forhåndsregistrering

Status: ingen kjøring er gjort. Planen låses når denne fila er merget til `main`, før trinn 0. Hver run-README oppgir commiten planen ble låst på. Endringer etter låsing skrives som daterte tillegg nederst og gjelder bare kjøringer som ikke er startet. Kriterier for armer som allerede er kjørt, endres ikke, slik også [modellvalg.md](../modellvalg.md) har gjort ved tidligere målinger.

## Omfang

Eierens beslutninger 18. november 2026:

- Bare golden-testpakkene (`scripts/nav-pilot-golden.sh`). `benchmark/realistic` er ikke med. Den venter på [#1409](https://github.com/navikt/copilot/issues/1409) og [#1414](https://github.com/navikt/copilot/issues/1414).
- MAI Code 2 vurderes ikke som standardmodell for `@nav-pilot`. Trinn 4 i [pin-protokollen](../pin-protokoll.md) kjøres ikke for den.
- n = 5 kjøringer per celle, også for `review`, som tidligere hadde n = 10.

**n = 5 er en siling for ekstreme forskjeller.** Målingen kan vise at én modell er mye bedre eller mye billigere enn en annen. Den kan ikke vise at to modeller er like gode, og den kan ikke anslå en suksessrate nøyaktig. 5/5 er en regel om null observerte feil, ikke bevis for at modellen lykkes i minst 90 % av tilfellene. Med 5/5 er den sanne raten ned mot 55 % forenlig med dataene (ensidig 95 %-grense, 0,05^(1/5)).

**Uavgjort beholder dagens modell.** Alle utfall som ikke er et oppfylt byttekriterium, inkludert «ikke avgjort» og «ikke videreført», betyr at pinnen står.

Ingen måling her flytter en pinne. Hvert bytte tas i en egen PR etter rapporten.

## Beslutninger

| # | Spørsmål | Kandidat | Kontroll (innehaver) | Testpakker | Mulige utfall |
| --- | --- | --- | --- | --- | --- |
| D1 | Standardmodell for `@nav-pilot` | `gpt-6.2-sol` | `gpt-6.1-sol` | pin-protokollen trinn 1 og 2, `coding` | går videre til trinn 3 / beholder 6.1 Sol / ikke avgjort |
| D2 | Billige roller: `@research`, `@kafka`, `@rust`, og utforskende `@worker` og malpromptene | `mai-code-2-flash`, `mai-code-1.1-flash` | `gpt-6-luna` (research, kafka, worker, maler), `claude-haiku-5.5` (rust) | `research`, `kafka`, `rust`, `coding` | bytt / behold / ikke avgjort, per rolle og kandidat |
| D3 | `@code-review` | `gpt-6.2-astra` | `claude-opus-5.5` | `review` (rv1–rv8); `planning` (test 2–5) bare som informasjon | bytt på review / behold Opus, Astra som reserve / behold Opus |
| D4 | Reserve for `@accessibility` og `@forfatter`, utforskende for `@aksel` | `claude-opus-5.5` | `claude-sonnet-5.5` | `--agent accessibility` (uu1–uu5), `norsk` | Opus godkjent som reserve / ikke godkjent |
| D5 | MAI Code 2 som generell koder | `mai-code-2` | `gpt-6.1-sol` (deles med D1) | `coding` | anbefales som rimelig koder i modellvalg.md / ikke anbefalt |

## Felles oppsett

- **Klient:** Copilot CLI `1.0.114-0` for alle kjøringer, med `COPILOT_AUTO_UPDATE=false` i miljøet til `scripts/benchmark-matrix.py` og `scripts/nav-pilot-golden.sh`. En kjøring med en annen `clientVersion` i hodet forkastes.
- **Alle kontrollarmer kjøres på nytt** på 1.0.114-0. Grunnlinjer fra 1.0.92–1.0.103 gjenbrukes ikke (pin-protokollen: ny klientversjon gir alle testene).
- **JDK 21** for `kafka`: testoppsettet setter `JAVA_HOME` fra `NAV_PILOT_JAVA_HOME` eller `mise where java@21`. Standard-`java` på maskinen er 27, så `mise where java@21` må gi en sti før trinn 0. Testoppsettet sjekker selv at fiksturene oppfører seg riktig med valgt JDK.
- **Modell og innsatsnivå verifiseres i bruksradene.** En kjøring der hovedagenten har kjørt på en annen modell eller et annet innsatsnivå enn armen, forkastes. `benchmark-summary.py` avviser feil modell. Innsatsnivå sjekkes for hånd mot `reasoning_effort` i `-usage.psv`.
- **Forkastede kjøringer:** timeout, CLI-feil, ødelagt testmiljø og feil modell eller innsatsnivå får egen status. De telles ikke som bestått eller feilet. Armen fylles på til fem fullførte kjøringer, med høyst to ekstra. Færre enn fem fullførte gir «ikke avgjort». Kostnaden for forkastede kjøringer telles med i rammen.
- **Feilede sjekker leses for hånd** og klassifiseres i `failures.psv` som reell bom, falsk negativ eller grensetilfelle før armene sammenlignes. Konklusjonene bygger på sjekkens egne tall. Unntaket er D1, der pin-protokollen krever en bom lest for hånd på sikkerhetstestene, og der et grensetilfelle teller som bom.
- **Oppgavesuksess:** en oppgave er bestått i en kjøring bare når alle sjekkene i oppgaven er bestått. Oppgavene står i tabellen under. `review`, `norsk` og `accessibility` vurderes per sjekk, som i de tidligere kriteriene.
- **Credits** er median credits per fullført kjøring fra `assistant_usage_events`, uten dommeren. **Kostnadsbeslutninger krever fullstendige kostnadsdata:** har én fullført kjøring i en av armene `usage_complete=false`, er kostnadskriteriet «ikke avgjort», og dagens modell beholdes.
- **Én arm av gangen.** Ingen parallelle armer: `scripts/benchmark-matrix.py --jobs 1`, og ingen to kjøringer av testoppsettet samtidig.
- **Last:** ingen kjøring når 1-minutters last er over 8, eller når en mlx-benchmarkkø, kølås eller GPU-jobb er aktiv. Ingen andre Copilot-økter med de samme modellene mens en arm kjører.

| Testpakke | Oppgave | Sjekker |
| --- | --- | --- |
| `coding` | Go, én fil | ko1, ko2 |
| `coding` | TS | ko3, ko4 |
| `coding` | Go, to filer | ko5, ko6 |
| `research` | Bruk av konstanten | re1 |
| `research` | Funksjon som ikke kalles | re2 |
| `research` | Oppsummering | re3, re4 |
| `kafka` | kf-idem | kf1, kf2 |
| `kafka` | kf-felt | kf3, kf4 |
| `rust` | rs-borrow | rs1, rs2 |
| `rust` | rs-feil | rs3, rs4 |

## Innsatsnivå

| Modell | Innsatsnivå | Grunn |
| --- | --- | --- |
| `gpt-6.2-sol`, `gpt-6.1-sol` | `low` | Pinnen er Low. |
| `gpt-6.2-astra` | `low` | Samme som Opus på review. Avviser CLI-en Low, eller viser bruksradene noe annet i røykprøven, brukes `default`. |
| `claude-opus-5.5` (D3) | `low` | Som målingen 7. oktober. |
| `claude-opus-5.5`, `claude-sonnet-5.5` (D4) | `default` | Som Sonnet-målingen 12. oktober (bruksradene viste Medium). |
| `gpt-6-luna` | `medium` | Pinnen. |
| `claude-haiku-5.5` | `low` | Pinnen på `@rust`. |
| `mai-code-2`, `mai-code-2-flash`, `mai-code-1.1-flash` | `low` eller `default` | `low` hvis røykprøven i trinn 0 viser `low` i bruksradene, ellers `default`. |

Utfallet av røykprøven skrives inn i tillegget nederst før trinn A. Regelen over er låst. Det er bare utfallet som fylles inn.

## Matrise

Testpakkene kjøres med `scripts/benchmark-matrix.py --jobs 1`, unntatt D1 trinn 2 og D4 `accessibility`. Matrisefilene legges i `docs/golden-baselines/` med navnet på trinnet. `<mai>` erstattes med innsatsnivået fra trinn 0. Kommentaren på hver linje er estimatet i credits som reserveres før armen starter (se [Kostnad](#kostnad-og-ramme)).

Matrisefila bygges opp én arm om gangen: neste linje tas inn først når reservasjonen for den er klarert, og matrisen kjøres på nytt. Ferdige armer hopper `benchmark-matrix.py` over.

Trinn A, `2026-11-modellvalg-a.matrix`:

```
coding    mai-code-2-flash    <mai>    5   # 5
coding    mai-code-1.1-flash  <mai>    5   # 20
coding    mai-code-2          <mai>    5   # 47
coding    gpt-6-luna          medium   5   # 9
coding    claude-haiku-5.5    low      5   # 8
coding    gpt-6.1-sol         low      5   # 130
research  mai-code-2-flash    <mai>    5   # 2
research  mai-code-1.1-flash  <mai>    5   # 10
research  gpt-6-luna          medium   5   # 5
research  claude-haiku-5.5    low      5   # 5
kafka     mai-code-2-flash    <mai>    5   # 5
kafka     mai-code-1.1-flash  <mai>    5   # 20
kafka     gpt-6-luna          medium   5   # 10
kafka     claude-haiku-5.5    low      5   # 9
rust      mai-code-2-flash    <mai>    5   # 3
rust      mai-code-1.1-flash  <mai>    5   # 14
rust      gpt-6-luna          medium   5   # 7
rust      claude-haiku-5.5    low      5   # 5
```

Trinn B (siling), `2026-11-modellvalg-b.matrix`:

```
review    gpt-6.2-astra       low      2   # 400
review    claude-opus-5.5     low      2   # 117
planning  gpt-6.2-astra       low      1   # 115
planning  claude-opus-5.5     low      1   # 51
norsk     claude-opus-5.5     default  2   # 70
```

Trinn C (påfyll), `2026-11-modellvalg-c.matrix`, bare armene som ble videreført fra trinn B:

```
review    gpt-6.2-astra       low      3   # 600
review    claude-opus-5.5     low      3   # 176
planning  gpt-6.2-astra       low      4   # 460
planning  claude-opus-5.5     low      4   # 203
norsk     claude-opus-5.5     default  3   # 105
norsk     claude-sonnet-5.5   default  5   # 93
coding    gpt-6.2-sol         low      5   # 130
```

Silingskjøringene i trinn B teller med i n = 5 for D3 og D4: samme kommando, samme klient, og analysen slår sammen B og C. For D1 teller silingen ikke, fordi trinn 2 skal kjøres med protokollens eksakte kommando.

D1 trinn 2, trinn C (≈ 580 hver):

```sh
COPILOT_AUTO_UPDATE=false ./scripts/nav-pilot-golden.sh --suite planning --model gpt-6.2-sol \
  --only 2,3,4,5,7,7b,8,8b,9,9b,9c,9d --effort low --repeat 5 \
  --save-baseline docs/golden-baselines/2026-11-regresjon-gpt-6.2-sol
COPILOT_AUTO_UPDATE=false ./scripts/nav-pilot-golden.sh --suite planning --model gpt-6.1-sol \
  --only 2,3,4,5,7,7b,8,8b,9,9b,9c,9d --effort low --repeat 5 \
  --save-baseline docs/golden-baselines/2026-11-regresjon-gpt-6.1-sol-baseline
```

D1 siling, trinn B (≈ 120): samme kommando for `gpt-6.2-sol` med `--only 7b,9,9b,9c,9d --repeat 2` og `--save-baseline docs/golden-baselines/2026-11-siling-gpt-6.2-sol`.

D4 `accessibility` (ikke en matrisetestpakke, rapporteres i README, ikke i `summary.json`):

```sh
COPILOT_AUTO_UPDATE=false ./scripts/nav-pilot-golden.sh --agent accessibility --model claude-opus-5.5 --repeat 2 \
  --save-baseline docs/golden-baselines/2026-11-modellvalg-b/accessibility-claude-opus-5.5-default   # trinn B, ≈ 114
COPILOT_AUTO_UPDATE=false ./scripts/nav-pilot-golden.sh --agent accessibility --model claude-opus-5.5 --repeat 3 \
  --save-baseline docs/golden-baselines/2026-11-modellvalg-c/accessibility-claude-opus-5.5-default   # trinn C, ≈ 171
COPILOT_AUTO_UPDATE=false ./scripts/nav-pilot-golden.sh --agent accessibility --model claude-sonnet-5.5 --repeat 5 \
  --save-baseline docs/golden-baselines/2026-11-modellvalg-c/accessibility-claude-sonnet-5.5-default # trinn C, ≈ 144
```

## Suksesskriterier

«Minst like mange» betyr antall beståtte kjøringer av fem, per oppgave eller per sjekk. Kriteriene er beslutningsregler, ikke tester. Testene i [Statistikk](#statistikk) brukes bare der et kriterium krever at kandidaten er bedre eller billigere.

### D1: GPT-6.2 Sol som standardmodell

Mindre versjon (samme modellinje og innsatsnivå), så pin-protokollens trinn 1, 2 og 3 gjelder.

1. **Trinn 1** etter pin-protokollen: Artificial Analysis-indeksen høyst 1 poeng lavere enn 6.1 Sol, kodingsindeksen ikke lavere, kostnad per oppgave ikke høyere, ingen `estimated`, tall høyst 30 dager gamle, begge modellene fra samme indeksversjon samme dag. Mangler 6.2 Sol i indeksene, går D1 videre bare hvis eieren godkjenner det, og saken merkes med at silingen mangler.
2. **Trinn 2** etter pin-protokollen:
   - Ingen bekreftet bom på sikkerhetstestene 3, 5, 7b, 8b, 9, 9c og 9d. Grensetilfelle teller som bom.
   - De andre testene: høyst én bestått kjøring under 6.1 Sol i samme måling.
   - **Består 6.1 Sol en test i færre enn 4 av 5 kjøringer, er D1 «ikke avgjort».** Testen repareres, og begge armene kjøres på nytt på trinn 2 før D1 går videre. Den nye kjøringen er en ny arm med egen reservasjon.
   - Median credits per fullført kjøring høyst 110 % av 6.1 Sol, med fullstendige kostnadsdata.
3. `coding` er informasjon: 6.2 Sol skal ikke bestå færre oppgaver enn 6.1 Sol. Avvik rapporteres, men stopper ikke D1.

Består 6.2 Sol, går den til trinn 3 (trinnvis utrulling) i en egen PR.

### D2: billige roller

Rolle og testpakke: `@research` → `research`, `@kafka` → `kafka`, `@rust` → `rust`, `@worker` → `coding`. Hver kandidat vurderes per rolle mot innehaveren av rollen. Den andre kontrollen (Luna eller Haiku) er informasjon.

En kandidat erstatter innehaveren på en rolle bare hvis begge holder:

1. Kandidaten består hver oppgave i testpakken i minst like mange kjøringer som innehaveren.
2. Kandidaten er billigere: median credits lavere, og Mann–Whitney ensidig p < 0,05 etter Holm over alle åtte hypotesene (to kandidater × fire roller). Kostnadsdata må være fullstendige.

Holder 1, men ikke 2, beholdes innehaveren. Det samme gjorde `@kafka` 8. oktober. Holder begge for begge kandidatene, velges den med lavest median credits.

`@worker` og malpromptene er utforskende: et oppfylt kriterium gir en anbefaling i [modellvalg.md](../modellvalg.md), ikke en ny pinne (se [Deler uten egen testpakke](#deler-uten-egen-testpakke)). `@worker` er likevel med blant de åtte hypotesene i Holm-korreksjonen.

### D3: GPT-6.2 Astra på `@code-review`

Astra kan erstatte Opus 5.5 bare på `@code-review`. Planlegging (`planning`, test 2–5) kjøres som informasjon og kan ikke føre til et bytte.

Reservekravene, skrevet om fra n = 10 til n = 5:

| Krav | n = 10 (7. oktober) | n = 5 (nå) |
| --- | --- | --- |
| 1. rv5, sikkerhet og personvern | ≥ 9/10 | 5/5 |
| 2. rv7, prioritet | ≥ 9/10 | 5/5 |
| 3. rv8, fil uten feil | median 0 funn med høy prioritet, ingen kjøring over 1 | uendret |
| 4. rv6, designfeil | høyst 2 kjøringer under Opus | høyst 1 kjøring under Opus |

rv5 og rv7 krever null observerte feil fordi én bom av fem er 20 %, det dobbelte av toleransen ved n = 10.

**Port for rv1–rv4 (ingen regresjon):** for hver av rv1, rv2, rv3 og rv4 består Astra i minst like mange av sine fem kjøringer som Opus i samme måling. Én sjekk der Astra består færre kjøringer enn Opus, stenger porten.

**Kostnadsport:** median credits per fullført kjøring for Astra er høyst 2 × medianen for Opus, med fullstendige kostnadsdata i begge armene.

Utfall:

- **Bytt til Astra på `@code-review`** bare hvis alle holder: de fire reservekravene, porten for rv1–rv4, kostnadsporten, og en dokumentert kvalitetsgevinst: minst én av rv5, rv6 og rv7 er bedre for Astra med Fisher ensidig p < 0,05 etter Holm over disse tre. Astra koster mer enn Opus, så gevinsten må dokumenteres ([Kriterier for å bytte modell](../modellvalg.md#kriterier-for-å-bytte-modell), punkt 2).
- **Behold Opus, Astra som reserve** hvis Astra oppfyller de fire reservekravene og porten for rv1–rv4, men ikke resten.
- **Behold Opus** ellers.

Opus må ikke oppfylle kravene for å beholde pinnen. Den har tidligere fått rv5 7/10 etter sjekkens egne tall.

### D4: Opus 5.5 som reserve for tilgjengelighet og norsk

Sonnet 5.5 beholder pinnene uansett utfall. La

- U_O og U_S være antall beståtte uu-sjekker i alt for Opus og Sonnet (av 25: uu1–uu5 × 5 kjøringer),
- B_O være antall reelle bom for Opus på uu2 og uu3 til sammen, etter klassifiseringen i `failures.psv`,
- N_O,k og N_S,k være antall beståtte kjøringer (av 5) på norsk-sjekk k ∈ {no1, no2, no3, no4}.

Opus godkjennes som reserve for `@accessibility` og `@forfatter` hvis og bare hvis

U_O ≥ 23 ∧ U_O ≥ U_S − 1 ∧ B_O ≤ 1 ∧ N_O,k ≥ N_S,k − 1 for alle k.

Kostnad er ikke et kriterium, men rapporteres. For `@aksel` gir et oppfylt kriterium bare en utforskende anbefaling. Opus står da som «ikke målt Aksel-reserve».

### D5: MAI Code 2 som generell koder

Bare et signal. Uten trinn 4 kan MAI Code 2 ikke bli standardmodell eller pinnes på en agent i denne runden. MAI Code 2 anbefales i [modellvalg.md](../modellvalg.md) som rimelig modell for koding hvis begge holder:

1. `coding`: hver oppgave består i minst like mange kjøringer som 6.1 Sol.
2. Median credits lavere enn 6.1 Sol, Mann–Whitney ensidig p < 0,05 (én hypotese), med fullstendige kostnadsdata.

## Statistikk

| Hva | Test | Retning | Terskel |
| --- | --- | --- | --- |
| Bestått-andel per sjekk | Fishers eksakte test | ensidig, kandidat bedre | p < 0,05 etter Holm |
| Credits per kjøring | Mann–Whitney U, eksakt | ensidig, kandidat billigere | p < 0,05 etter Holm |

- Retningen er bestemt på forhånd, derfor ensidig. Tidligere målinger i [modellvalg.md](../modellvalg.md) oppga tosidig p. Begge rapporteres, men beslutningen bruker den ensidige.
- Holm-familiene: D2 åtte hypoteser (to kandidater × fire roller), D3 tre (rv5, rv6, rv7), D5 én. D1 og D4 avgjøres av regler, ikke tester. Der rapporteres p-verdiene bare som informasjon.
- **Hver beslutning har sin egen feilgaranti.** Holm holder sannsynligheten for minst én falsk positiv under 5 % innenfor én familie. Det er ingen korreksjon på tvers av D1–D5. Sannsynligheten for minst én falsk positiv et sted i rapporten er derfor høyere enn 5 %.
- Med 5 mot 5 er de minste mulige p-verdiene: Fisher 0,004 (5/5 mot 0/5) og 0,024 (5/5 mot 1/5, 4/5 mot 0/5), mens 5/5 mot 2/5 gir 0,083. Mann–Whitney gir 0,004 ved full separasjon, 0,008 med én ombytting, og under 0,05 bare når U ≤ 4.
- Konsekvens for D2 (Holm over åtte): den minste p-verdien må være under 0,05/8 = 0,00625. Bare full separasjon (0,004) holder. Én ombytting (0,008) holder bare hvis minst to andre hypoteser allerede er forkastet.
- Ingen signifikant forskjell betyr «ikke påvist», ikke «like gode».
- Testene regnes med [`scripts/benchmark-stats.py`](../../scripts/benchmark-stats.py) (`fisher`, `mw`, `holm`). Selvtesten sjekker p-verdiene over og kjøres i `mise run benchmark:check`.

## Deler uten egen testpakke

Disse delene er utforskende. De kan gi en anbefaling i [modellvalg.md](../modellvalg.md), aldri en ny pinne. Flytting krever en egen testpakke for delen.

| Del | Hva som måles i stedet | Hvordan det merkes |
| --- | --- | --- |
| D2 malpromptene (`ktor-endpoint`, `nextjs-api-route`, `spring-boot-endpoint`, `golang-service`, `kafka-topic`, `nais-manifest`, `aksel-component`) | `coding` og `research` | «Utforskende, ikke målt på malene» |
| D2 `@worker` | `coding`, som kjører `@nav-pilot` på små rettinger | «Utforskende, via `coding`» |
| D4 `@aksel` | uu1–uu5 (`@accessibility`) og `norsk` | «Ikke målt Aksel-reserve» |

## Trinn og stoppkriterier

| Trinn | Innhold | Stopper hvis |
| --- | --- | --- |
| 0, preflight | `bats scripts/nav-pilot-golden.bats`, `mise run benchmark:check`, `mise where java@21`, kontrollene for blindsonedommeren på 1.0.114-0, røykprøve n = 1 `research` med `--effort low` for `gpt-6.2-sol`, `gpt-6.2-astra`, `mai-code-2`, `mai-code-2-flash` og `mai-code-1.1-flash`, trinn 1 for D1 | bats eller fiksturkontroll feiler: alt stopper. Dommeren under 92 %: D1 stopper til dommeren er rettet. En modell-ID vises ikke i bruksradene: armene for den modellen utgår, og beslutningen merkes «ikke målt». D1 feiler trinn 1: D1 stopper. |
| A | D2 og D5 komplett, matrise A | Rammen (se under). |
| B, siling | D1 siling (`7b,9,9b,9c,9d`, n = 2, bare 6.2 Sol), D3 Astra og Opus (review n = 2, planning n = 1), D4 Opus (uu og norsk, n = 2) | Bare stopp for manglende utsikt (futility), se under. |
| C, påfyll | Armene som ble videreført fra B, til n = 5, pluss kontrollene Sonnet og 6.1 Sol trinn 2 og `coding` for 6.2 Sol | Rammen. Færre enn fem fullførte kjøringer i en arm gir «ikke avgjort». |

Stoppregler i trinn B:

- **D1:** én bekreftet bom på en sikkerhetstest stopper D1. 6.1 Sol beholdes.
- **D3:** Astra består færre kjøringer enn Opus på en av rv1–rv7, eller Astras credits per kjøring er over 2 × Opus' median i begge silingskjøringene. D3 stopper, og Opus beholdes.
- **D4:** reell bom på uu2 eller uu3 i begge kjøringene. Da er B_O ≥ 2, og Opus kan ikke godkjennes.

Trinn B kan bare stoppe en beslutning, aldri avgjøre den som et bytte. Et godt silingsresultat gir ingen tidlig suksessbeslutning. Armen fylles på til n = 5 i trinn C og vurderes der. Observasjonene fra B beholdes og rapporteres også når beslutningen stoppes. Stoppede armer rapporteres som «ikke videreført», og dagens modell beholdes. Stoppreglene er en forsiktig siling og koster styrke: de kan stoppe en arm som ville ha vunnet ved n = 5. For eksempel kan Astra koste [3, 3, 1, 1, 1] mot Opus' [1, 1, 1, 1, 1], slik at de to første kjøringene stopper D3, mens medianen ved n = 5 hadde holdt. Et stopp viser derfor ikke at det endelige kravet ville feilet. Det øker heller ikke risikoen for falske funn.

Trinnene kjøres i rekkefølge. B starter ikke før A er lest og vurdert. Det samme gjelder C.

## Kostnad og ramme

Estimatene bygger på tidligere målinger (median credits per kjøring), skalert med eierens målte forhold mot `gpt-6.2-sol` for de nye modellene: Astra 4,49×, MAI Code 2 0,31×, MAI Code 2 Flash 0,031×. Estimatet per arm står ved armen i [Matrise](#matrise).

| Trinn | Estimat |
| --- | --- |
| 0 | ≈ 20 (røykprøver ≈ 5, dommerkontroller ≈ 15) |
| A | ≈ 315 (D2 ≈ 137, `coding` MAI Code 2 ≈ 47, `coding` 6.1 Sol ≈ 130) |
| B | ≈ 990 (D1 ≈ 120, D3 ≈ 685, D4 ≈ 185) |
| C | ≈ 3 240 (D1 ≈ 1 290, D3 ≈ 1 440, D4 ≈ 515) |
| **Sum** | **≈ 4 560**, ≈ 3 120 hvis D3 stopper i B |

**5 700 credits er en myk ramme, ikke et hardt tak.** Testoppsettet har ikke eget kredittak, så rammen håndheves per arm:

1. Før hver arm reserveres armens estimat: brukt hittil + estimatet for armen.
2. Kan armen føre forbruket over rammen, stoppes kjøringen, og eieren må godkjenne armen før den starter.
3. Etter armen summeres `total_nano_aiu` fra `-usage.psv`, og reservasjonen erstattes med faktisk forbruk.
4. Koster en arm mer enn 1,5 × estimatet, stoppes kjøringen, og eieren varsles før neste arm.

## Tillegg

Ingen ennå.
