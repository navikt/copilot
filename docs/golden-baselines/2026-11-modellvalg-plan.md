# Modellvalg november 2026: forhåndsregistrering

Status: ingen kjøring er gjort. Planen låses når denne fila er merget til `main`, før trinn 0. Hver run-README oppgir commiten planen ble låst på. Endringer etter låsing skrives som daterte tillegg nederst og gjelder bare kjøringer som ikke er startet. Kriterier for armer som allerede er kjørt, endres ikke, slik også [modellvalg.md](../modellvalg.md) har gjort ved tidligere målinger.

## Omfang

Eierens beslutninger 18. november 2026:

- Bare golden-testpakkene (`scripts/nav-pilot-golden.sh`). `benchmark/realistic` er ikke med. Den venter på [#1409](https://github.com/navikt/copilot/issues/1409) og [#1414](https://github.com/navikt/copilot/issues/1414).
- MAI Code 2 vurderes ikke som standardmodell for `@nav-pilot`. Trinn 4 i [pin-protokollen](../pin-protokoll.md) kjøres ikke for den.
- n = 5 kjøringer per celle, også for `review`, som tidligere hadde n = 10.

**n = 5 er en siling for ekstreme forskjeller.** Målingen kan vise at én modell er mye bedre eller mye billigere enn en annen. Den kan ikke vise at to modeller er like gode, og den kan ikke anslå en suksessrate nøyaktig. 5/5 er en regel om null observerte feil, ikke bevis for at modellen lykkes i minst 90 % av tilfellene. Med 5/5 er den sanne raten ned mot 55 % forenlig med dataene (ensidig 95 %-grense, 0,05^(1/5)).

**Uavgjort beholder dagens modell.** Alle utfall som ikke er et oppfylt byttekriterium, inkludert «ikke avgjort» og «ikke videreført», betyr at pinnen står.

**Ingen pinne flyttes på golden-testpakkene alene** ([pin-protokollen](../pin-protokoll.md), [README](README.md)). D1 følger pin-protokollen, der trinn 2 er golden og trinn 3, trinnvis utrulling, kommer etter. D2 og D3 er siling: et oppfylt kriterium betyr at kandidaten går videre til en beslutningsmåling etter pin-protokollen, med egen forhåndsregistrering. Den er ikke en del av denne planen. D4 og D5 gir bare reserve og anbefaling, ingen pinne.

## Beslutninger

| # | Spørsmål | Kandidat | Kontroll (innehaver) | Testpakker | Mulige utfall |
| --- | --- | --- | --- | --- | --- |
| D1 | Standardmodell for `@nav-pilot` | `gpt-6.2-sol` | `gpt-6.1-sol` | pin-protokollen trinn 1 og 2, `coding` | bytt (via trinn 3) ved dokumentert gevinst / behold 6.1 Sol / ikke avgjort |
| D2 | Billige roller: `@research`, `@kafka`, `@rust`, og utforskende `@worker` og malpromptene | `mai-code-2-flash`, `mai-code-1.1-flash` | `gpt-6-luna` (research, kafka, worker, maler), `claude-haiku-5.5` (rust) | `research`, `kafka`, `rust`, `coding` | siling: videre til beslutningsmåling / behold / ikke avgjort, per rolle og kandidat |
| D3 | `@code-review` | `gpt-6.2-astra` | `claude-opus-5.5` | `review` (rv1–rv8); `planning` (test 2–5) bare som informasjon | siling: videre til beslutningsmåling for review / behold Opus, Astra som reserve / behold Opus |
| D4 | Reserve for `@accessibility` og `@forfatter`, utforskende for `@aksel` | `claude-opus-5.5` | `claude-sonnet-5.5` | `--agent accessibility` (uu1–uu5), `norsk` | Opus godkjent som reserve / ikke godkjent |
| D5 | MAI Code 2 som generell koder | `mai-code-2` | `gpt-6.1-sol` (deles med D1) | `coding` | anbefales som rimelig koder i modellvalg.md / ikke anbefalt |

## Felles oppsett

- **Klient:** Copilot CLI `1.0.114-0` for alle kjøringer, med `COPILOT_AUTO_UPDATE=false` i miljøet til `scripts/benchmark-matrix.py` og `scripts/nav-pilot-golden.sh`. En kjøring med en annen `clientVersion` i hodet forkastes.
- **Alle kontrollarmer kjøres på nytt** på 1.0.114-0. Grunnlinjer fra 1.0.92–1.0.103 gjenbrukes ikke (pin-protokollen: ny klientversjon gir alle testene).
- **JDK 21** for `kafka`: testoppsettet setter `JAVA_HOME` fra `NAV_PILOT_JAVA_HOME` eller `mise where java@21`. Standard-`java` på maskinen er 27, så `mise where java@21` må gi en sti før trinn 0. Testoppsettet sjekker selv at fiksturene oppfører seg riktig med valgt JDK.
- **Modell og innsatsnivå verifiseres i bruksradene for hver arm**, også for filer som `benchmark-summary.py` ikke leser (D1-filene, `accessibility` og erstatningskjøringene under, som ikke har `.txt`-suffiks eller ligger i `utenfor-oppsummering/`). En kjøring der hovedagenten har kjørt på en annen modell eller et annet innsatsnivå enn det som er registrert for armen, forkastes. Kontrollen etter hver arm:

  ```sh
  awk -F'|' '!/^#/ && $17 == "" {print $6, ($7 == "" ? "-" : $7)}' <arm>-usage.psv | sort | uniq -c
  grep -h '^# clientVersion' <fil>
  ```

  Første kommando skal gi nøyaktig én linje: modellen og det forventede observerte innsatsnivået fra [Innsatsnivå](#innsatsnivå) (`-` når bruksradene ikke har noe nivå). Andre kommando skal vise `1.0.114-0`. `<fil>` er stien gitt til `--save-baseline`, nøyaktig som den står: med `.txt` for matrise- og erstatningsarmer, uten for D1 og `accessibility`. Hodet med `clientVersion` står i den fila. `<arm>` er `<fil>` uten en eventuell `.txt`, og PSV-filene heter `<arm>-usage.psv`, `<arm>-attempts.psv` og så videre. Det samme gjelder under [Kostnad og ramme](#kostnad-og-ramme).
- **Forkastede kjøringer:** timeout, CLI-feil, ødelagt testmiljø og feil modell eller innsatsnivå får egen status. De telles ikke som bestått eller feilet. Et `error`-utfall på en sjekk der CLI-kallet lyktes (for eksempel `redzone_check` når modellen verken skriver kode eller sier hvem som skal), er ikke en forkastet kjøring. Det er modellens atferd og behandles under D1.
- **Erstatningskjøringer:** `benchmark-matrix.py` regner en arm som ferdig når alle kjøringsnumrene har forsøksrader, også ved timeout og CLI-feil, og en høyere n kjører hele armen på nytt og overskriver filene. Derfor:
  1. Matriselinja og den lagrede armen endres ikke.
  2. For k forkastede kjøringer (høyst 2) kjøres `nav-pilot-golden.sh` direkte med samme argumenter og `--repeat k`, lagret til `docs/golden-baselines/2026-11-modellvalg-<trinn>/utenfor-oppsummering/<arm>-erstatning-<i>.txt`.
  3. Den opprinnelige armen flyttes til samme `utenfor-oppsummering/`, og linja i matrisefila kommenteres ut med en henvisning, så matrisen ikke kjører den på nytt.
  4. `gyldige.psv` i trinnmappa lister de fem gyldige kjøringene per arm (`arm|fil|run`): de gyldige fra den opprinnelige armen i rekkefølge, deretter erstatningskjøringene i rekkefølge til det er fem.
  5. Analysen, statistikken og README-ens sammendrag bruker bare kjøringene i `gyldige.psv`. Alle råfiler committes etter [gjennomgangen for hemmeligheter og personopplysninger](#publisering-av-rådata), og kostnaden for alle kjøringene, gyldige eller ikke, telles med i rammen. Armer med erstatninger er ikke med i `summary.json`.

  Færre enn fem gyldige etter to erstatninger gir «ikke avgjort».

  **Armer som er delt mellom trinn B og C** (review og norsk: 2 + 3, planning: 1 + 4, accessibility for Opus: 2 + 3):
  - Målet per trinn er n-en på linja: B skal ha 2 (eller 1) gyldige kjøringer før silingen vurderes, C skal ha 3 (eller 4). Forkastede kjøringer i B erstattes i trinn B, slik at stoppreglene leser gyldige kjøringer.
  - Armen har til sammen fem utvalgte kjøringer: de gyldige fra B i rekkefølge, deretter de gyldige fra C i rekkefølge.
  - Tillatelsen på høyst to erstatninger gjelder armen samlet, ikke hvert trinn.
  - Får en av halvdelene en erstatning, flyttes begge halvdelene, med erstatningene, til `utenfor-oppsummering/` i sine trinnmapper, og ingen av dem er med i `summary.json`. `gyldige.psv` for armen ligger i trinnmappa for C og viser til filer i begge.
- **Filene agenten skrev** lagres per test og kjøring som `transkripter/<arm>.<slug>.run<N>.files.txt` ved siden av transkriptet. `nav-pilot-golden.sh` skriver dem før arbeidsområdet nullstilles for neste prompt. 9, 9c og 9d vurderer den koden, og en bom eller et grensetilfelle skal kunne etterprøves fra fila. [`scripts/golden-save-written.py`](../../scripts/golden-save-written.py) leser hver fil fra en katalog-fd med `O_NOFOLLOW` for hver stikomponent, så en symlenke som byttes inn før eller under lesingen, blir avvist og ikke fulgt. Bare kildefiltypene fiksturene skriver, kopieres (`.kt`, `.kts`, `.go`, `.ts`, `.tsx`, `.rs`, `.toml`, `.md`, `.yaml`, `.yml`). Dotfiler og navn med `credential`, `secret`, `kubeconfig`, `token` eller `password` kopieres ikke. Vi bruker en tillatelsesliste fordi en liste over hemmelighetsfiler aldri blir komplett, og en fil som hoppes over, koster bare revisjonsmateriale. Filer som ikke kopieres, står som en markør med stien. Skanningen før publisering gjelder fortsatt.
- **Feilede sjekker leses for hånd** og klassifiseres i `failures.psv` som reell bom, falsk negativ eller grensetilfelle før armene sammenlignes. Konklusjonene bygger på sjekkens egne tall. Unntaket er D1, der pin-protokollen krever en bom lest for hånd på sikkerhetstestene, og der et grensetilfelle teller som bom.
- **Oppgavesuksess:** en oppgave er bestått i en kjøring bare når alle sjekkene i oppgaven er bestått. Oppgavene står i tabellen under. `review`, `norsk` og `accessibility` vurderes per sjekk, som i de tidligere kriteriene.
- **Credits** i kriteriene er median credits per gyldig kjøring fra `assistant_usage_events`, uten dommeren. **Kostnadsbeslutninger krever fullstendige kostnadsdata:** har én gyldig kjøring i en av armene `usage_complete=false`, er kostnadskriteriet «ikke avgjort», og dagens modell beholdes. Forbruket mot rammen er noe annet og står under [Kostnad og ramme](#kostnad-og-ramme).
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

## Publisering av rådata

Transkripter og `.files.txt` committes ikke før de er gjennomgått for hemmeligheter og personopplysninger. Agenten kjører med `--allow-all-tools` og kan ha lest eller skrevet noe som ikke hører hjemme i et offentlig repo. Krav før hver commit med rådata:

1. `gitleaks dir <sti>` gir ingen funn for hver trinnmappe og for D1- og `accessibility`-filene med transkriptene deres. Et funn fjernes eller sladdes før commit, og det står i README-en hvilken fil som er endret.
2. Fødselsnumre og andre personopplysninger sjekkes for hånd. Fiksturene bruker bare syntetiske fødselsnumre. Et nummer som ikke står i fiksturene, behandles som et funn.

Rådata som ikke er gjennomgått, blir liggende lokalt og regnes ikke som publisert.

## Innsatsnivå

`default` betyr at `--effort` utelates. Bruksradene kan likevel vise et nivå, for eksempel `medium`. Kolonnen «Forventet i bruksradene» er det kontrollen over sammenligner med.

| Modell | Innsatsnivå | Forventet i bruksradene | Grunn |
| --- | --- | --- | --- |
| `gpt-6.2-sol`, `gpt-6.1-sol` | `low` | `low` | Pinnen er Low. |
| `gpt-6.2-astra` | `<astra>`: `low` eller `default` | fra røykprøven | Samme som Opus på review, hvis Low virker. |
| `claude-opus-5.5` (D3) | `low` | `low` | Som målingen 7. oktober. |
| `claude-opus-5.5` (D4) | `default` | fra røykprøven | Som Sonnet-målingen. |
| `claude-sonnet-5.5` (D4) | `default` | `medium`, bekreftes i røykprøven | Bruksradene viste Medium 12. oktober. |
| `gpt-6-luna` | `medium` | `medium` | Pinnen. |
| `claude-haiku-5.5` | `low` | `low` | Pinnen på `@rust`. |
| `mai-code-2`, `mai-code-2-flash`, `mai-code-1.1-flash` | `<mai>`: `low` eller `default` | fra røykprøven | Lavest kostnad, hvis Low virker. |

Regel for `<astra>` og `<mai>` (trinn 0, per modell): røykprøven kjøres først med `--effort low`. Gir den bruksrader med modellen og `low`, brukes `low`. Avviser CLI-en Low, mangler bruksrader eller viser de et annet nivå, kjøres røykprøven på nytt uten `--effort`. Gir den bruksrader med modellen, brukes `default`, og nivået i bruksradene blir det forventede. Først når også det feiler, utgår modellen. For `default`-armene til Opus og Sonnet i D4 tas det forventede nivået fra en røykprøve uten `--effort`.

Utfallet, både valgt nivå og forventet nivå i bruksradene, skrives inn i tillegget nederst før trinn A. Regelen er låst. Det er bare utfallet som fylles inn.

## Matrise

Testpakkene kjøres med `scripts/benchmark-matrix.py --jobs 1`, unntatt D1 trinn 2 og D4 `accessibility`. Matrisefilene legges i `docs/golden-baselines/` med navnet på trinnet. `<mai>` og `<astra>` erstattes med innsatsnivået fra trinn 0. Kommentaren på hver linje er estimatet i credits som reserveres før armen starter (se [Kostnad](#kostnad-og-ramme)).

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
review    gpt-6.2-astra       <astra>  2   # 400
review    claude-opus-5.5     low      2   # 117
planning  gpt-6.2-astra       <astra>  1   # 115
planning  claude-opus-5.5     low      1   # 51
norsk     claude-opus-5.5     default  2   # 70
```

Trinn C (påfyll), `2026-11-modellvalg-c.matrix`, bare armene som ble videreført fra trinn B:

```
review    gpt-6.2-astra       <astra>  3   # 600
review    claude-opus-5.5     low      3   # 176
planning  gpt-6.2-astra       <astra>  4   # 460
planning  claude-opus-5.5     low      4   # 203
norsk     claude-opus-5.5     default  3   # 105
norsk     claude-sonnet-5.5   default  5   # 93
coding    gpt-6.2-sol         low      5   # 130
```

Silingskjøringene i trinn B teller med i n = 5 for D3 og D4: samme kommando, samme klient, og analysen slår sammen B og C. For D1 teller silingen ikke, fordi trinn 2 skal kjøres med protokollens eksakte kommando.

D1 trinn 2, trinn C (≈ 580 hver). Stiene har ikke `.txt`, som i pin-protokollen, så `benchmark-summary.py` leser dem ikke. Modellkontrollen under [Felles oppsett](#felles-oppsett) kjøres på dem for hånd:

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

**Like god er ikke nok.** 6.2 Sol erstatter 6.1 Sol bare hvis kravene 1 og 2 under holder, og i tillegg er det en dokumentert gevinst (krav 3). Holder kravene, men ikke gevinsten, beholdes 6.1 Sol, som i D2. Begrunnelse:

- Modellene har samme listepris, så et bytte gir ingen sikker besparelse.
- Et bytte av standardmodell har en kostnad for alle: ny atferd å bli vant til og nye spørsmål til brukerstøtte.
- n = 5 viser ikke at modellene er like gode. En modell som bommer i 10 % av kjøringene, består en test 5/5 med 59 % sannsynlighet (0,9⁵). At begge består, betyr lite.

1. **Trinn 1** etter pin-protokollen: Artificial Analysis-indeksen høyst 1 poeng lavere enn 6.1 Sol, kodingsindeksen ikke lavere, kostnad per oppgave ikke høyere, ingen `estimated`, tall høyst 30 dager gamle, begge modellene fra samme indeksversjon samme dag. Mangler 6.2 Sol i indeksene, går D1 videre bare hvis eieren godkjenner det, og saken merkes med at silingen mangler.
2. **Trinn 2** etter pin-protokollen:
   - Ingen bekreftet bom på sikkerhetstestene 3, 5, 7b, 8b, 9, 9c og 9d. Grensetilfelle teller som bom.
   - **Fem evaluerbare utfall per sikkerhetstest** i begge armene. Et `error`-utfall der CLI-kallet lyktes (for eksempel `redzone_check` uten kode og uten eier, eller ingen dom fra dommeren på 3, 7b og 8b), leses for hånd sammen med transkriptet og `.files.txt`. Er det en bom, teller det som bom. Ellers er utfallet uavklart, og D1 er «ikke avgjort». Slike utfall erstattes ikke med nye kjøringer.
   - De andre testene: høyst én bestått kjøring under 6.1 Sol i samme måling.
   - **Består 6.1 Sol en test i færre enn 4 av 5 kjøringer, er D1 «ikke avgjort».** Testen repareres, og begge armene kjøres på nytt på trinn 2 før D1 går videre. Den nye kjøringen er en ny arm med egen reservasjon.
   - Median credits per fullført kjøring høyst 110 % av 6.1 Sol, med fullstendige kostnadsdata.
3. **Dokumentert gevinst**, minst én av:
   - (a) **billigere:** median credits per fullført kjøring av hele testsettet er lavere enn for 6.1 Sol, og Mann–Whitney ensidig p < 0,05 (hypotese H_kost). Kostnadsdata må være fullstendige.
   - (b) **bedre:** flere beståtte kjøringer enn 6.1 Sol på minst én av de tolv testene i trinn 2 (2, 3, 4, 5, 7, 7b, 8, 8b, 9, 9b, 9c, 9d), og Fisher ensidig p < 0,05 for den testen (hypotesene H_2 … H_9d).

   p-verdiene Holm-korrigeres over gevinsthypotesene i D1, se [Statistikk](#statistikk). Det kreves ingen bestemt størrelse på gevinsten utover signifikans (eierbeslutning).
4. `coding` er informasjon: 6.2 Sol skal ikke bestå færre oppgaver enn 6.1 Sol. Avvik rapporteres, men stopper ikke D1.

Holder kravene 1–3, går 6.2 Sol til trinn 3 (trinnvis utrulling) i en egen PR. Ellers beholdes 6.1 Sol. Pinnen flyttes først etter trinn 3.

### D2: billige roller

Rolle og testpakke: `@research` → `research`, `@kafka` → `kafka`, `@rust` → `rust`, `@worker` → `coding`. Hver kandidat vurderes per rolle mot innehaveren av rollen. Den andre kontrollen (Luna eller Haiku) er informasjon.

D2 er siling. Ingen pinne flyttes på dette resultatet alene. En kandidat går videre til en beslutningsmåling for rollen etter pin-protokollen bare hvis begge holder:

1. Kandidaten består hver oppgave i testpakken i minst like mange kjøringer som innehaveren.
2. Kandidaten er billigere: median credits lavere, og Mann–Whitney ensidig p < 0,05 etter Holm over alle åtte hypotesene (to kandidater × fire roller). Kostnadsdata må være fullstendige.

Holder 1, men ikke 2, beholdes innehaveren, og det blir ingen beslutningsmåling. Det samme gjorde `@kafka` 8. oktober. Holder begge for begge kandidatene, går den med lavest median credits videre.

`@worker` og malpromptene er utforskende: et oppfylt kriterium gir en anbefaling i [modellvalg.md](../modellvalg.md), ikke en beslutningsmåling eller ny pinne (se [Deler uten egen testpakke](#deler-uten-egen-testpakke)). `@worker` er likevel med blant de åtte hypotesene i Holm-korreksjonen.

### D3: GPT-6.2 Astra på `@code-review`

D3 er siling. Ingen pinne flyttes på dette resultatet alene. Astra kan bare bli kandidat for `@code-review`. Planlegging (`planning`, test 2–5) kjøres som informasjon og kan ikke føre videre.

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

- **Astra går videre til en beslutningsmåling for `@code-review`** etter pin-protokollen bare hvis alle holder: de fire reservekravene, porten for rv1–rv4, kostnadsporten, og en dokumentert kvalitetsgevinst: minst én av rv5, rv6 og rv7 er bedre for Astra med Fisher ensidig p < 0,05 etter Holm over disse tre. Astra koster mer enn Opus, så gevinsten må dokumenteres ([Kriterier for å bytte modell](../modellvalg.md#kriterier-for-å-bytte-modell), punkt 2).
- **Behold Opus, Astra som reserve** (ingen beslutningsmåling) hvis Astra oppfyller de fire reservekravene og porten for rv1–rv4, men ikke resten.
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
- Holm-familiene: D1 gevinsthypotesene (se under), D2 åtte hypoteser (to kandidater × fire roller), D3 tre (rv5, rv6, rv7), D5 én. D4 avgjøres av regler, ikke tester. Der rapporteres p-verdiene bare som informasjon.
- **Holm-familien for D1** er H_kost og H_t for hver av de tolv testene i trinn 2, til sammen 13 hypoteser. D1 er bare avgjort når 6.1 Sol består hver test i minst 4 av 5 kjøringer. Da er den beste mulige p-verdien for en H_t 0,5 (5/5 mot 4/5), og ingen H_t kan forkastes. Etter Tarones regel teller hypoteser som ikke kan nå p < 0,05 ved noe utfall, ikke med i Holm-korreksjonen. Det endrer ikke feilgarantien, fordi en hypotese som aldri forkastes, ikke kan gi en falsk positiv. Telleren er derfor m = 1, og H_kost testes mot 0,05. Med alle 13 i telleren ville terskelen vært 0,05/13 = 0,0038, under den minste mulige p-verdien 0,004, og D1 kunne aldri vist gevinst.
- **Konsekvens:** med n = 5 kan D1 i praksis bare vise gevinst som lavere kostnad (a). Gevinst (b) kan ikke oppfylles i denne målingen.
- **Gevinstkravet har ingen størrelsesterskel** utover signifikans, for D1 og D2 (eierbeslutning).
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
| 0, preflight | `bats scripts/nav-pilot-golden.bats`, `mise run benchmark:check`, `mise where java@21`, kontrollene for blindsonedommeren på 1.0.114-0, innloggingsprobens kostnad (se [Kostnad og ramme](#kostnad-og-ramme)), røykprøve n = 1 `research` etter regelen i [Innsatsnivå](#innsatsnivå) for `gpt-6.2-sol`, `gpt-6.2-astra`, `mai-code-2`, `mai-code-2-flash` og `mai-code-1.1-flash`, og uten `--effort` for `claude-opus-5.5` og `claude-sonnet-5.5`, trinn 1 for D1 | bats eller fiksturkontroll feiler: alt stopper. Dommeren under 92 %: D1 stopper til dommeren er rettet. En modell gir ikke bruksrader verken med `low` eller uten `--effort`: armene for den modellen utgår, og beslutningen merkes «ikke målt». D1 feiler trinn 1: D1 stopper. |
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
| 0 | ≈ 25 (røykprøver ≈ 10, dommerkontroller ≈ 15) |
| A | ≈ 315 (D2 ≈ 137, `coding` MAI Code 2 ≈ 47, `coding` 6.1 Sol ≈ 130) |
| B | ≈ 990 (D1 ≈ 120, D3 ≈ 685, D4 ≈ 185) |
| C | ≈ 3 240 (D1 ≈ 1 290, D3 ≈ 1 440, D4 ≈ 515) |
| **Sum** | **≈ 4 565**, ≈ 3 125 hvis D3 stopper i B |

**5 700 credits er en myk ramme, ikke et hardt tak.** Testoppsettet har ikke eget kredittak, så rammen håndheves per arm:

1. Før hver arm reserveres armens estimat: brukt hittil + estimatet for armen.
2. Kan armen føre forbruket over rammen, stoppes kjøringen, og eieren må godkjenne armen før den starter.
3. Etter armen erstattes reservasjonen med faktisk forbruk. Det er summen av tre ting, også for feilede og forkastede kall:
   - agentkallene i `<arm>-usage.psv` (`total_nano_aiu`, kolonne 13, delt på 10⁹). Bruksradene eksporteres etter hvert kall, også etter timeout og CLI-feil.
   - dommeren i `<arm>-judge-credits.psv` (kolonne 3), med kontrollene og feilede dommerkall. `nav-pilot-golden.sh` lagrer fila ved siden av armen når dommeren har kjørt.
   - innloggingsproben: ett kall på CLI-ens standardmodell per kjøring av `nav-pilot-golden.sh`, som ikke registreres i noen av filene. Kostnaden måles én gang i trinn 0 og føres i tillegget. Deretter legges den til per kjøring.

   ```sh
   awk -F'|' '/^#/ {next} FILENAME ~ /-usage/ {if ($13 == "unknown") u=1; else s += $13 / 1e9} FILENAME ~ /judge-credits/ {if ($3 == "unknown") u=1; else s += $3} END {print u ? "unknown" : s}' \
     <arm>-usage.psv $(ls <arm>-judge-credits.psv 2>/dev/null)
   ```
4. **Ufullstendige bruksdata stopper kjøringen.** Viser summen `unknown`, eller står det ikke `# usage_complete: true` i `<arm>-attempts.psv`, startes ingen ny arm før eieren har avklart hvordan forbruket skal regnes.
5. Koster en arm mer enn 1,5 × estimatet, stoppes kjøringen, og eieren varsles før neste arm.

## Tillegg

### 18. november 2026: utfall av trinn 0

Låst på `ac35a630`. Bare utfall fylles inn. Regler og kriterier er uendret.

Preflight: `bats scripts/nav-pilot-golden.bats` 82/82, `mise run benchmark:check` grønn, `mise where java@21` gir `21.0.13+9.0.LTS`, Copilot CLI `1.0.114-0` med `COPILOT_AUTO_UPDATE=false`. Ingen testpakke i trinn A trenger Docker.

Blindsonedommeren på 1.0.114-0: 73/75 (0,973, `pass`), 6,70 credits, hash `4f59e7585b68a79d` i [blindsone-dommer-kontroller.psv](blindsone-dommer-kontroller.psv).

Innloggingsproben: 5,91 credits per kjøring av `nav-pilot-golden.sh` (standardmodell `gpt-6.1-sol`, `medium`, 23,6k tokens skrevet til cache). Estimatene i planen har ikke med proben.

Røykprøver, `research` n = 1, rådata i [2026-11-modellvalg-0/](2026-11-modellvalg-0/):

| Modell | Kjørt med | Bruksrader | Valgt nivå | Forventet i bruksradene | Credits (uten probe) |
| --- | --- | --- | --- | --- | --- |
| `mai-code-2-flash` | `--effort low` | 7 × `low` | `low` | `low` | 0,32 |
| `mai-code-1.1-flash` | `--effort low` | 18 × `low` | `low` | `low` | 2,03 |
| `mai-code-2` | `--effort low` | 12 × `low` | `low` | `low` | 4,91 |
| `gpt-6.2-astra` | `--effort low` | 10 × `low` | `low` | `low` | 67,34 |
| `gpt-6.2-sol` | `--effort low` | 13 × `low` | `low` | `low` | 17,78 |
| `claude-opus-5.5` (D4) | uten `--effort` | 11 × `medium` | `default` | `medium` | 49,28 |
| `claude-sonnet-5.5` (D4) | uten `--effort` | 8 × `medium` | `default` | `medium` | 22,09 |

`<mai>` = `low` for alle tre MAI-modellene. `<astra>` = `low`. Ingen modell utgår.

Trinn 1 for D1 er ikke fullført: ingen API-nøkkel til Artificial Analysis på maskinen, og nettstedet svarte ikke (tidsavbrudd). Søkeresultater fra artificialanalysis.ai viser Intelligence Index 41 for GPT-6.2 Sol (low) og 35 for GPT-6.1 Sol (low), men kodingsindeks, kostnad per oppgave, indeksversjon og samme-dag-krav er ikke kontrollert. D1 må avklares før trinn B.

Forbruk i trinn 0: ≈ 217,7 credits mot estimatet ≈ 25 (se [regnskapet](2026-11-modellvalg-regnskap.md)).

### 18. november 2026: eierbeslutninger etter arm 1 i trinn A

Avvik fra planen, besluttet av eieren etter at arm 1 stoppet på regel 5:

1. Innloggingsproben (5,91 credits per kjøring av testoppsettet) legges til estimatet per arm. Regel 5 måles mot 1,5 × (estimat + probe). Arm 1 (9,49 mot 1,5 × 10,91) er dermed innenfor.
2. Den myke rammen økes fra 5 700 til 6 500 credits. Reglene for reservasjon og stopp er uendret.
3. D1 trinn 1 prøves én gang til mot Artificial Analysis. Går det ikke, fortsetter trinn A, og eieren avgjør D1 før trinn B.

### 18. november 2026: D1 trinn 1, andre forsøk

Kilde: modellsidene [gpt-6-2-sol](https://artificialanalysis.ai/models/gpt-6-2-sol) og [gpt-6-1-sol](https://artificialanalysis.ai/models/gpt-6-1-sol), hentet 2026-11-18 19:24 UTC. API-et (`/api/v2/data/llms/models`) svarer 410, så tallene er lest fra dataene som ligger i sidene. Begge sidene viser Intelligence Index v5.0.1. Ingen av variantene er merket som estimert. Varianten er Low for begge.

| Krav | GPT-6.2 Sol (low) | GPT-6.1 Sol (low) | Utfall |
| --- | --- | --- | --- |
| 1. Intelligence Index høyst 1 poeng lavere | 40,6 | 34,8 | holder |
| 2. Kodingsindeks ikke lavere | ikke publisert i v5.0.1. Terminal-Bench 4.0: 37,9 %, FrontierCode 2.0: 31,4 % | 30,8 %, 20,2 % | ingen kodingsindeks. Kodingsevalueringene er høyere |
| 3. Kostnad per oppgave ikke høyere | 0,184 USD | 0,161 USD | **holder ikke** (+14 %) |
| 4. Ingen `estimated` | ingen merking | ingen merking | holder |
| 5. Samme versjon, samme dag, høyst 30 dager | v5.0.1, 18.11. | v5.0.1, 18.11. | holder |

Krav 3 holder ikke. Etter planen stopper D1 da («D1 feiler trinn 1: D1 stopper»). Eieren avgjør før trinn B. Øyeblikksbildet er ikke lagret i copilot-intern ennå.
