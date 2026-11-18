# Modellvalg november 2026: forhåndsregistrering

Status: ingen kjøring er gjort. Planen låses når denne fila er merget til `main`, før trinn 0. Hver run-README oppgir commiten planen ble låst på. Endringer etter låsing skrives som daterte tillegg nederst og gjelder bare kjøringer som ikke er startet. Kriterier for armer som allerede er kjørt, endres ikke, slik også [modellvalg.md](../modellvalg.md) har gjort ved tidligere målinger.

## Omfang

Eierens beslutninger 18. november 2026:

- Bare golden-testpakkene (`scripts/nav-pilot-golden.sh`). `benchmark/realistic` er ikke med. Den venter på [#1409](https://github.com/navikt/copilot/issues/1409) og [#1414](https://github.com/navikt/copilot/issues/1414).
- MAI Code 2 vurderes ikke som standardmodell for `@nav-pilot`. Trinn 4 i [pin-protokollen](../pin-protokoll.md) kjøres ikke for den.
- n = 5 kjøringer per celle, også for `review`, som tidligere hadde n = 10.

Ingen måling her flytter en pinne. Hvert bytte tas i en egen PR etter rapporten.

## Beslutninger

| # | Spørsmål | Kandidat | Kontroll (innehaver) | Testpakker | Mulige utfall |
| --- | --- | --- | --- | --- | --- |
| D1 | Standardmodell for `@nav-pilot` | `gpt-6.2-sol` | `gpt-6.1-sol` | pin-protokollen trinn 1 og 2, `coding` | går videre til trinn 3 / beholder 6.1 Sol / ikke avgjort |
| D2 | Billige roller: `@research`, `@kafka`, `@rust`, `@worker` og malpromptene | `mai-code-2-flash`, `mai-code-1.1-flash` | `gpt-6-luna` (research, kafka, worker, maler), `claude-haiku-5.5` (rust) | `research`, `kafka`, `rust`, `coding` | bytt / behold / ikke avgjort, per rolle og kandidat |
| D3 | `@code-review` og planlegging med høy risiko | `gpt-6.2-astra` | `claude-opus-5.5` | `review` (rv1–rv8), `planning` (test 2–5) | bytt / behold Opus, Astra som reserve / behold Opus |
| D4 | Reserve for `@aksel`, `@accessibility` og `@forfatter` | `claude-opus-5.5` | `claude-sonnet-5.5` | `--agent accessibility` (uu1–uu5), `norsk` | Opus godkjent som reserve / ikke godkjent |
| D5 | MAI Code 2 som generell koder | `mai-code-2` | `gpt-6.1-sol` (deles med D1) | `coding` | anbefales som rimelig koder i modellvalg.md / ikke anbefalt |

## Felles oppsett

- **Klient:** Copilot CLI `1.0.114-0` for alle kjøringer, med `COPILOT_AUTO_UPDATE=false` i miljøet til `scripts/benchmark-matrix.py` og `scripts/nav-pilot-golden.sh`. En kjøring med en annen `clientVersion` i hodet forkastes.
- **Alle kontrollarmer kjøres på nytt** på 1.0.114-0. Grunnlinjer fra 1.0.92–1.0.103 gjenbrukes ikke (pin-protokollen: ny klientversjon gir alle testene).
- **JDK 21** for `kafka`: testoppsettet setter `JAVA_HOME` fra `NAV_PILOT_JAVA_HOME` eller `mise where java@21`. Standard-`java` på maskinen er 27, så `mise where java@21` må gi en sti før trinn 0. Testoppsettet sjekker selv at fiksturene oppfører seg riktig med valgt JDK.
- **Modell og innsatsnivå verifiseres i bruksradene.** En kjøring der hovedagenten har kjørt på en annen modell eller et annet innsatsnivå enn armen, forkastes. `benchmark-summary.py` avviser feil modell. Innsatsnivå sjekkes for hånd mot `reasoning_effort` i `-usage.psv`.
- **Forkastede kjøringer:** timeout, CLI-feil, ødelagt testmiljø og feil modell eller innsatsnivå får egen status. De telles ikke som bestått eller feilet. Armen fylles på til fem fullførte kjøringer, med høyst to ekstra. Færre enn fem fullførte gir «ikke avgjort». Kostnaden for forkastede kjøringer telles mot taket.
- **Feilede sjekker leses for hånd** og klassifiseres i `failures.psv` som reell bom, falsk negativ eller grensetilfelle før armene sammenlignes. Konklusjonene bygger på sjekkens egne tall. Unntaket er D1, der pin-protokollen krever en bom lest for hånd på sikkerhetstestene, og der et grensetilfelle teller som bom.
- **Credits** er median credits per fullført kjøring fra `assistant_usage_events`, uten dommeren. `usage_complete=false` gir ukjent kostnad for kjøringen.
- **Last:** ingen kjøring når 1-minutters last er over 8, eller når en mlx-benchmarkkø, kølås eller GPU-jobb er aktiv. `--jobs` høyst 3. Ingen andre Copilot-økter med de samme modellene mens en arm kjører.

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

Testpakkene kjøres med `scripts/benchmark-matrix.py`, unntatt D1 trinn 2 og D4 `accessibility`. Matrisefilene legges i `docs/golden-baselines/` med navnet på trinnet. `<mai>` erstattes med innsatsnivået fra trinn 0.

Trinn A, `2026-11-modellvalg-a.matrix`:

```
coding    mai-code-2-flash    <mai>    5
coding    mai-code-1.1-flash  <mai>    5
coding    mai-code-2          <mai>    5
coding    gpt-6-luna          medium   5
coding    claude-haiku-5.5    low      5
coding    gpt-6.1-sol         low      5
research  mai-code-2-flash    <mai>    5
research  mai-code-1.1-flash  <mai>    5
research  gpt-6-luna          medium   5
research  claude-haiku-5.5    low      5
kafka     mai-code-2-flash    <mai>    5
kafka     mai-code-1.1-flash  <mai>    5
kafka     gpt-6-luna          medium   5
kafka     claude-haiku-5.5    low      5
rust      mai-code-2-flash    <mai>    5
rust      mai-code-1.1-flash  <mai>    5
rust      gpt-6-luna          medium   5
rust      claude-haiku-5.5    low      5
```

Trinn B (siling), `2026-11-modellvalg-b.matrix`:

```
review    gpt-6.2-astra       low      2
review    claude-opus-5.5     low      2
planning  gpt-6.2-astra       low      1
planning  claude-opus-5.5     low      1
norsk     claude-opus-5.5     default  2
```

Trinn C (påfyll), `2026-11-modellvalg-c.matrix`, bare armene som passerte trinn B:

```
review    gpt-6.2-astra       low      3
review    claude-opus-5.5     low      3
planning  gpt-6.2-astra       low      4
planning  claude-opus-5.5     low      4
norsk     claude-opus-5.5     default  3
norsk     claude-sonnet-5.5   default  5
coding    gpt-6.2-sol         low      5
```

Silingskjøringene i trinn B teller med i n = 5 for D3 og D4: samme kommando, samme klient, og analysen slår sammen B og C. For D1 teller silingen ikke, fordi trinn 2 skal kjøres med protokollens eksakte kommando.

D1 trinn 2, trinn C:

```sh
COPILOT_AUTO_UPDATE=false ./scripts/nav-pilot-golden.sh --suite planning --model gpt-6.2-sol \
  --only 2,3,4,5,7,7b,8,8b,9,9b,9c,9d --effort low --repeat 5 \
  --save-baseline docs/golden-baselines/2026-11-regresjon-gpt-6.2-sol
COPILOT_AUTO_UPDATE=false ./scripts/nav-pilot-golden.sh --suite planning --model gpt-6.1-sol \
  --only 2,3,4,5,7,7b,8,8b,9,9b,9c,9d --effort low --repeat 5 \
  --save-baseline docs/golden-baselines/2026-11-regresjon-gpt-6.1-sol-baseline
```

D1 siling, trinn B: samme kommando for `gpt-6.2-sol` med `--only 7b,9,9b,9c,9d --repeat 2` og `--save-baseline docs/golden-baselines/2026-11-siling-gpt-6.2-sol`.

D4 `accessibility` (ikke en matrisetestpakke, rapporteres i README, ikke i `summary.json`):

```sh
COPILOT_AUTO_UPDATE=false ./scripts/nav-pilot-golden.sh --agent accessibility --model claude-opus-5.5 --repeat 2 \
  --save-baseline docs/golden-baselines/2026-11-modellvalg-b/accessibility-claude-opus-5.5-default   # trinn B
COPILOT_AUTO_UPDATE=false ./scripts/nav-pilot-golden.sh --agent accessibility --model claude-opus-5.5 --repeat 3 \
  --save-baseline docs/golden-baselines/2026-11-modellvalg-c/accessibility-claude-opus-5.5-default   # trinn C
COPILOT_AUTO_UPDATE=false ./scripts/nav-pilot-golden.sh --agent accessibility --model claude-sonnet-5.5 --repeat 5 \
  --save-baseline docs/golden-baselines/2026-11-modellvalg-c/accessibility-claude-sonnet-5.5-default # trinn C
```

## Suksesskriterier

«Like mange» og «minst like mange» betyr antall beståtte kjøringer av fem, per sjekk eller oppgave. Med n = 5 kan vi ikke vise at to modeller er like gode. Kriteriene under er beslutningsregler, ikke statistiske tester. Testene i neste avsnitt brukes bare der et kriterium krever at kandidaten er bedre eller billigere.

### D1: GPT-6.2 Sol som standardmodell

Mindre versjon (samme modellinje og innsatsnivå), så pin-protokollens trinn 1, 2 og 3 gjelder.

1. **Trinn 1** etter pin-protokollen: Artificial Analysis-indeksen høyst 1 poeng lavere enn 6.1 Sol, kodingsindeksen ikke lavere, kostnad per oppgave ikke høyere, ingen `estimated`, tall høyst 30 dager gamle, begge modellene fra samme indeksversjon samme dag. Mangler 6.2 Sol i indeksene, går D1 videre bare hvis eieren godkjenner det, og saken merkes med at silingen mangler.
2. **Trinn 2** etter pin-protokollen, uendret:
   - Ingen bekreftet bom på sikkerhetstestene 3, 5, 7b, 8b, 9, 9c og 9d. Grensetilfelle teller som bom.
   - De andre testene: høyst én bestått kjøring under 6.1 Sol i samme måling. En test der 6.1 Sol består færre enn 4 av 5, kan ikke brukes og rapporteres som det.
   - Median credits per fullført kjøring høyst 110 % av 6.1 Sol.
3. `coding` er informasjon: 6.2 Sol skal ikke bestå færre sjekker enn 6.1 Sol. Avvik rapporteres, men stopper ikke D1.

Består 6.2 Sol, går den til trinn 3 (trinnvis utrulling) i en egen PR.

### D2: billige roller

Rolle og testpakke: `@research` → `research`, `@kafka` → `kafka`, `@rust` → `rust`, `@worker` → `coding`. Hver kandidat vurderes per rolle mot innehaveren av rollen. Den andre kontrollen (Luna eller Haiku) er informasjon.

En kandidat erstatter innehaveren på en rolle bare hvis begge holder:

1. Kandidaten består hver oppgave i testpakken i minst like mange kjøringer som innehaveren.
2. Kandidaten er billigere: median credits lavere, og Mann–Whitney ensidig p < 0,05 etter Holm (familie: kandidatens fire roller).

Holder 1, men ikke 2, beholdes innehaveren. Det samme gjorde `@kafka` 8. oktober. Holder begge for begge kandidatene, velges den med lavest median credits.

### D3: GPT-6.2 Astra på review og planlegging

Reservekravene for `@code-review`, skrevet om fra n = 10 til n = 5:

| Krav | n = 10 (7. oktober) | n = 5 (nå) |
| --- | --- | --- |
| 1. rv5, sikkerhet og personvern | ≥ 9/10 | 5/5 |
| 2. rv7, prioritet | ≥ 9/10 | 5/5 |
| 3. rv8, fil uten feil | median 0 funn med høy prioritet, ingen kjøring over 1 | uendret |
| 4. rv6, designfeil | høyst 2 kjøringer under Opus | høyst 1 kjøring under Opus |

rv5 og rv7 krever 5/5 fordi én bom av fem er 20 %, det dobbelte av toleransen ved n = 10. Planlegging: Astra består hver av testene 2–5 i minst like mange kjøringer som Opus.

Utfall:

- **Bytt til Astra** bare hvis Astra oppfyller alle fire kravene, er minst like god som Opus på planleggingstestene, og har en dokumentert kvalitetsgevinst: minst én av rv5, rv6, rv7 og planleggingstestene 2, 3, 4 og 5 er bedre for Astra med Fisher ensidig p < 0,05 etter Holm (familie: disse sju sjekkene). Astra koster omtrent 3,4 ganger Opus, så en gevinst må dokumenteres ([Kriterier for å bytte modell](../modellvalg.md#kriterier-for-å-bytte-modell), punkt 2).
- **Behold Opus, Astra som reserve** hvis Astra oppfyller de fire kravene, men uten dokumentert gevinst.
- **Behold Opus** ellers.

Opus må ikke oppfylle kravene for å beholde pinnen. Den har tidligere fått rv5 7/10 etter sjekkens egne tall.

### D4: Opus 5.5 som reserve for Aksel, tilgjengelighet og norsk

Sonnet 5.5 beholder pinnene uansett utfall. Opus godkjennes som reserve hvis alle holder:

1. uu1–uu5: minst 23/25 i alt, og høyst én kjøring under Sonnet i samme måling.
2. uu2 og uu3: høyst én reell bom til sammen.
3. `norsk`: hver sjekk høyst én kjøring under Sonnet i samme måling.

Kostnad er ikke et kriterium, men rapporteres.

### D5: MAI Code 2 som generell koder

Bare et signal. Uten trinn 4 kan MAI Code 2 ikke bli standardmodell eller pinnes på en agent i denne runden. MAI Code 2 anbefales i [modellvalg.md](../modellvalg.md) som rimelig modell for koding hvis begge holder:

1. `coding`: hver oppgave består i minst like mange kjøringer som 6.1 Sol.
2. Median credits lavere enn 6.1 Sol, Mann–Whitney ensidig p < 0,05 (familie: én test).

## Statistikk

| Hva | Test | Retning | Terskel |
| --- | --- | --- | --- |
| Bestått-andel per sjekk | Fishers eksakte test | ensidig, kandidat bedre | p < 0,05 etter Holm |
| Credits per kjøring | Mann–Whitney U, eksakt | ensidig, kandidat billigere | p < 0,05 etter Holm |

- Retningen er bestemt på forhånd, derfor ensidig. Tidligere målinger i [modellvalg.md](../modellvalg.md) oppga tosidig p. Begge rapporteres, beslutningen bruker den ensidige.
- Holm-familiene står i kriteriene: D2 per kandidat (4 roller), D3 (7 sjekker), D5 (1 test). D1 og D4 avgjøres av regler, ikke tester. Der rapporteres p-verdiene som informasjon.
- Med 5 mot 5 er de minste mulige p-verdiene: Fisher 0,004 (5/5 mot 0/5) og 0,024 (5/5 mot 1/5, 4/5 mot 0/5), mens 5/5 mot 2/5 gir 0,083. Mann–Whitney gir 0,004 ved full separasjon og under 0,05 bare når U ≤ 4. Bare store forskjeller kan vises. Ingen signifikant forskjell betyr «ikke påvist», ikke «like gode».
- Testene regnes med standardbiblioteket (eksakt opptelling, `math.comb`). Skriptet legges i run-mappa og kontrolleres mot tallene over før trinn A. `scipy` finnes ikke på målemaskinen.

## Deler uten testpakke

| Del | Hva som måles i stedet | Hvordan det merkes |
| --- | --- | --- |
| D2 malpromptene (`ktor-endpoint`, `nextjs-api-route`, `spring-boot-endpoint`, `golang-service`, `kafka-topic`, `nais-manifest`, `aksel-component`) | `coding` og `research` | «Indirekte, ikke målt på malene». Kan gi en reserveanbefaling, ikke flytte pinnen. Flytting krever en egen testpakke for malene. |
| D2 `@worker` | `coding`, som kjører `@nav-pilot` på små rettinger | «Indirekte, via `coding`», som ved tidligere målinger. |
| D4 `@aksel` | uu1–uu5 (`@accessibility`) og `norsk` | «Indirekte, ikke målt på Aksel-komponenter». Godkjenning som reserve gjelder `@aksel` bare med denne merkingen. |

## Trinn og stoppkriterier

| Trinn | Innhold | Stopper hvis |
| --- | --- | --- |
| 0, preflight | `bats scripts/nav-pilot-golden.bats`, `mise run benchmark:check`, `mise where java@21`, kontrollene for blindsonedommeren på 1.0.114-0, røykprøve n = 1 `research` med `--effort low` for `gpt-6.2-sol`, `gpt-6.2-astra`, `mai-code-2`, `mai-code-2-flash` og `mai-code-1.1-flash`, trinn 1 for D1 | bats eller fiksturkontroll feiler: alt stopper. Dommeren under 92 %: D1 og D3 stopper til dommeren er rettet. En modell-ID vises ikke i bruksradene: armene for den modellen utgår, og beslutningen merkes «ikke målt». D1 feiler trinn 1: D1 stopper. |
| A | D2 og D5 komplett, matrise A | Taket for trinnet. |
| B, siling | D1 siling (`7b,9,9b,9c,9d`, n = 2, bare 6.2 Sol), D3 Astra og Opus (review n = 2, planning n = 1), D4 Opus (uu og norsk, n = 2) | **D1:** én bekreftet bom på en sikkerhetstest stopper D1, og 6.1 Sol beholdes. **D3:** Astra består færre kjøringer enn Opus på rv5, rv6 eller rv7, eller på en planleggingstest: D3 stopper, og Opus beholdes. **D4:** reell bom på uu2 eller uu3 i begge kjøringene stopper D4: Opus ikke godkjent. |
| C, påfyll | Armene som passerte B, til n = 5, pluss kontrollene Sonnet og 6.1 Sol trinn 2 og `coding` for 6.2 Sol | Taket for trinnet. Færre enn fem fullførte kjøringer i en arm gir «ikke avgjort». |

Trinnene kjøres i rekkefølge. B starter ikke før A er lest og stoppreglene for A er vurdert. Det samme gjelder C.

## Kostnad og tak

Anslag fra tidligere målinger (median credits per kjøring), skalert med eierens målte forhold mot `gpt-6.2-sol` for de nye modellene: Astra 4,49×, MAI Code 2 0,31×, MAI Code 2 Flash 0,031×.

| Trinn | Anslag | Hardt tak |
| --- | --- | --- |
| 0 | ≈ 20 (røykprøver ≈ 5, dommerkontroller ≈ 15) | 40 |
| A | ≈ 315 (D2 ≈ 137, `coding` MAI Code 2 ≈ 47, `coding` 6.1 Sol ≈ 130) | 450 |
| B | ≈ 990 (D1 ≈ 120, D3 ≈ 685, D4 ≈ 185) | 1 300 |
| C | ≈ 3 240 (D1 ≈ 1 290, D3 ≈ 1 440, D4 ≈ 515) | 3 900 |
| **Sum** | **≈ 4 560**, ≈ 3 120 hvis D3 stopper i B | **5 700** |

Testoppsettet har ikke eget kredittak. Taket håndheves mellom armer: koordinatoren summerer `total_nano_aiu` fra `-usage.psv` etter hver arm og starter ingen ny arm som kan føre trinnet over taket med anslaget sitt. Koster en arm mer enn 1,5 ganger anslaget, stoppes trinnet og eieren varsles før neste arm.

## Tillegg

Ingen ennå.
