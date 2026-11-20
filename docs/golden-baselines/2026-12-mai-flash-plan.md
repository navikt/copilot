# MAI Code 2 Flash på `@research` og `@rust`: forhåndsregistrering

Status: **låst 20. november 2026.** Eierbeslutningene 19. og 20. november 2026 er lagt inn (se «Eierbeslutninger»). Ingen modellkjøringer før denne revisjonen er merget. Endringer etter låsing føres i et tillegg med dato og eierbeslutning.

Grunnlag: D2 i [2026-11-modellvalg-plan.md](https://github.com/navikt/copilot/blob/benchmark-2026-11-run/docs/golden-baselines/2026-11-modellvalg-plan.md) og trinn A i regnskapet (#2397). Protokoll: [pin-protokoll.md](../pin-protokoll.md).

## Utgangspunkt

| Rolle | Innehaver | D2-siling, MAI Code 2 Flash Low (n = 5) |
| --- | --- | --- |
| `@research` | GPT-6 Luna Medium, re1–re4 5/5, median 1,08 credits | re1–re4 5/5, median 0,22, Mann–Whitney p 0,004, Holm 0,032 |
| `@rust` | Claude Haiku 5.5 Low, rs1–rs4 5/5, median 0,93 | rs1–rs4 5/5, median 0,33, p 0,004, Holm 0,032 |

Samme modell bommet på kf1 i `kafka` (4/5): den merket `eventId` som behandlet før utbetalingen, så en feilet utbetaling ble hoppet over ved ny levering. Siling med n = 5 og 5/5 sier bare at en feilrate på over ca. 45 % er usannsynlig. Den sier ingenting om sjeldne feil.

## Risikovurdering: kan kf1-feilen ramme disse rollene?

Feilmønsteret i kf1 er «registrer bivirkningen som gjort før den er gjort» (rekkefølge mellom tilstand og sideeffekt ved feil og ny levering).

- **`@research`: lav.** Agenten har bare lese- og søkeverktøy og skriver ikke kode. Den tilsvarende risikoen er en feil påstand om koden, for eksempel at noe er idempotent eller at en funksjon ikke kalles. Det dekkes av re1 (riktig fil og linje) og re2 (sier ærlig at noe ikke finnes). Ingen ny oppgave trengs, men re2 behandles som sikkerhetssjekk (se stoppregler).
- **`@rust`: middels.** Agenten skriver kode og kjører kommandoer. rs1–rs4 er små rettinger i én fil (lånefeil, feiltyper) og har ingen sjekk for rekkefølge mellom tilstand og sideeffekt. Samme bom ville ikke blitt fanget. Derfor legges det til en ny oppgave **rs5** før målingen (se under).

## Ny oppgave rs5 (forutsetning, ingen modellkostnad)

Laget i [#2470](https://github.com/navikt/copilot/pull/2470) som rs5 (atferd) og rs6 (omfang, endrer bare i `meldingsmottak/`). Planen låses på revisjonen der #2470 er merget.

En Rust-fikstur der meldingshåndteren utbetaler uten å huske ID-en, så samme melding utbetales to ganger. Den skjulte testen krever én utbetaling per ID (også ved `e1, e2, e1`), riktig beløp, og at en feilet utbetaling utbetales ved ny levering. Det siste fanger kf1-feilen: å markere ID-en før utbetalingen. Evaluatoren bruker sitt eget `Cargo.toml`. Krav til oppsett, som for kf1:

1. Starttilstanden feiler rs5 (dobbel utbetaling) og består eksisterende tester.
2. Den kjente rettingen (marker etter vellykket kall, eller fjern markeringen ved feil) består.
3. En kjent ugyldig patch (`if seen.insert(id) { call()? }`, samme mønster som kf1-bommen) avvises. Kontrollen legges i `scripts/nav-pilot-golden.bats`.
4. Beskrivelse i `CHECKS` i `scripts/benchmark-summary.py`.

Begrensning: rs5 dekker ikke delvis feil (utbetaling gjennomført før feilen) eller varig deduplisering ved omstart.


## Hva målingen avgjør

Hovedversjon etter protokollen (ny leverandør og modellfamilie): trinn 1, 2, 4 og 3.

| Trinn | Her | Merknad |
| --- | --- | --- |
| 1 Siling, eksterne tall | **Siling mangler** (se under) | Eieren har bestemt at målingen starter uten |
| 2 Regresjonssjekk | `planning`-suiten som i protokollen, pluss rollesuitene | Se «Trinn 2» |
| 4 Full måling | Hovedmålingen under | |
| 3 Utrulling | Etter bestått trinn 4 | Se «Trinnvis utrulling» |

## Trinn 1: siling

Sjekket 19. november 2026 kl. 16:21 UTC på modellsidene til [Artificial Analysis](https://artificialanalysis.ai/), Intelligence Index v5.0.1. Det finnes fortsatt ikke API-tilgang, så sidene ble lest direkte.

| Modell | Side | Funn |
| --- | --- | --- |
| MAI Code 2 Flash | `/models/mai-code-2-flash` | 404. Modellen står heller ikke i modellisten (`/models`) |
| GPT-6 Luna | `/models/gpt-6-luna` | Finnes |
| Claude Haiku 5.5 | `/models/claude-haiku-5-5` | Finnes |

**Siling mangler.** Kandidaten finnes ikke i indeksene, så krav 1–3 kan ikke vurderes. Etter protokollen venter en ny modellfamilie til den finnes i indeksene, med mindre eieren bestemmer noe annet. Eieren bestemte 19. november 2026 å starte uten siling (se «Eierbeslutninger»). Tallene for Luna og Haiku er ikke lagret her, fordi de ikke kan sammenlignes med noe.

## Trinn 2: regresjonssjekk

`planning` kjøres med kandidaten nøyaktig som i protokollen:

```sh
./scripts/nav-pilot-golden.sh --suite planning --model mai-code-2-flash \
  --only 2,3,4,5,7,7b,8,8b,9,9b,9c,9d --effort low --repeat 5 \
  --save-baseline docs/golden-baselines/<dato>-regresjon-mai-code-2-flash
```

- **Grunnlinje:** `2026-11-regresjon2-gpt-6.1-sol-baseline` (pinnen for `@nav-pilot`, revisjon `d421991a`, Copilot CLI 1.0.114-0). Den er målt med dommeren fra #2437. `2026-11-regresjon-gpt-6.1-sol-baseline` (`6fbce9a6`) er målt før #2437 og brukes ikke. Fra `d421991a` til låsingen er bare rust-fiksturene og rust-delen av harnesset endret, ikke `planning` eller dommeren. Unntak: test 8 har 3/5 i grunnlinjen, og protokollen krever minst 4/5 før en test kan brukes. #2475 retter test 8 (kontekst til dommeren, #2465) og flytter test 4 til LLM-dommeren (#2458). Da er dommeren endret for t4 og t8, så de kjøres på nytt for 6.1 Sol som **delgrunnlinje** (`planning --only 4,8`, n = 5, samme klient og versjonskontroll som under). Resten av `regresjon2` gjenbrukes. Har test 4 eller 8 under 4/5 i delgrunnlinjen, kan testen ikke brukes, og det er en stopp som eieren må avgjøre. Endres klient, fiksturer eller dommer ellers før kjøringen, kjøres de berørte testene på nytt for 6.1 Sol, og det er også en stopp (opptil ≈ 590 credits).
- **Klient:** Copilot CLI 1.0.114-0, samme som grunnlinjen. Klienten oppdaterte seg selv til 1.0.115-0 19. november, etter at grunnlinjen var målt. Ny klientversjon ville kreve ny grunnlinje for alle testene. Alle armer i trinn 2 og trinn 4 kjøres derfor på 1.0.114-0: en `copilot`-wrapper først i `PATH` i kjøreskriptet starter `copilot --prefer-version 1.0.114-0` med `COPILOT_AUTO_UPDATE=false`. Brukerens globale oppsett endres ikke. Versjonen kontrolleres for hver økt, både oppgave og dommer, med `copilotVersion` i `session.start` i Copilots øktdata. `# clientVersion` i overskriften på attempts-fila er bare én sjekk før start og holder ikke alene. Økter på en annen versjon forkastes (stoppregel 4). Den første kjøringen i trinn 2 bekrefter at `--prefer-version` også gjelder dommeren og subagentene før resten kjøres.
- **Bestått:** reglene i trinn 2 i protokollen. Én reell bom på en sikkerhetstest (3, 5, 7b, 8b, 9, 9c, 9d) stopper begge rollene. Kostnadsregelen (høyst 10 % over grunnlinjen) gjelder også.
- **Rekkefølge:** #2475 merges, deretter låses denne planen, så kjøres delgrunnlinjen for t4/t8 og så trinn 2. Trinn 4 starter bare hvis trinn 2 består.

## Design for hovedmålingen (trinn 4)

- **Armer:** `research mai-code-2-flash low` mot `research gpt-6-luna medium`; `rust mai-code-2-flash low` mot `rust claude-haiku-5.5 low`. Innehaveren kjøres i samme måling, på samme klientversjon og fiksturrevisjon. Grunnlinjer fra november gjenbrukes ikke.
- **Enhet:** én kjøring der alle sjekkene i testpakken består (re1–re4, eller rs1–rs6). Sjekker i samme kjøring telles ikke som uavhengige.
- **Hypotese per rolle:** ensidig ikke-underlegenhet for andelen beståtte kjøringer, margin 10 prosentpoeng, antatt basis 95 %, nominell α = 0,05 (se «Feilrate»), styrke 0,8. Samme parametre som trinn 4 i protokollen. Marginen betyr at vi godtar at Flash fullfører helt ned mot 85 % av kjøringene når innehaveren fullfører 95 %: omtrent én ekstra mislykket oppgave per ti. Ikke alle feil er synlige for brukeren. `@rust` har tester som feiler, men `@research` kan gi kilder som virker troverdige og er feil (re1), og det merker brukeren ikke nødvendigvis. Marginen er derfor akseptabel bare sammen med tre vern: en bom på re1–re4 leses for hånd og klassifiseres før et kikk tolkes, så feil kilder ikke blir gjemt i andelen; sikkerhetsbommene stoppes av stoppregel 1; og utrullingen er trinnvis, med en kanal for tilbakemelding før pinnen flyttes. Kombinert med lavere kostnad per kjøring er det akseptabelt å tåle omtrent én ekstra mislykket kjøring per ti, men ikke en systematisk feil som kan skjules. Styrken er 80,6 % ved sann rate 95 %/95 % og 60,3 % ved 90 %/90 %. Kikkene er planlagte titt med faste grenser, ikke kalibrert alfa-forbruk: på nullgrensen (95 %/85 %) er rejeksjonsraten 5,23 %. To separate rolleavgjørelser har ingen felles 5 %-garanti; er de uavhengige, er sjansen for minst én feilaktig godkjenning ≈ 10,2 %. To roller er to separate beslutninger, ikke én Holm-familie, fordi hver rolle har sin egen pinne.
- **n og kikk:** 24, 48 og 72 kjøringer per arm. Ved kikk k brukes z_k = 1,645 · √(3/k). Bestått når den nedre grensen i Newcombe-intervallet for differansen (kandidat minus innehaver) passerer −10 prosentpoeng. «Ikke avgjort» når observert differanse ved kikk 1 eller 2 er dårligere enn −10 prosentpoeng, eller ved 72 uten bestått.
- **Kostnad:** median credits per fullført kjøring uten dommer og probe. Regel, ikke test: er medianen for kandidaten høyere enn for innehaveren, flyttes ikke pinnen. Mann–Whitney ensidig rapporteres bare som informasjon.
- **Feilrate:** grensene er de samme som i trinn 4 i protokollen. Den siste grensen (z = 1,645) gjør at samlet α er litt over 0,05. Dette er en kjent egenskap ved protokollen, og vi endrer den ikke her. Styrken på 0,8 gjelder ved en sann rate på 95 %. Stoppregel 1 gjør at sjansen for å bytte modell blir lavere enn styrken hvis bommene er på sikkerhetssjekkene.
- **Kontroll av modell:** kjøringer der bruksradene viser annen modell eller annet innsatsnivå enn armen, forkastes. Ufullstendige bruksdata gir ukjent kostnad og stopp av armen.
- **Lesing for hånd:** alle bommer på re1–re4 og rs5 leses og klassifiseres i `failures.psv` (reell bom, grensetilfelle, falsk negativ) før et kikk tolkes. Grensetilfeller telles som bom.

## Kostnadsestimat og ramme

Medianer fra trinn A, uten dommer. Probe 5,91 per kjøring av testoppsettet (én per arm per kikk). rs5 anslås å øke rust-kostnaden med 50 %.

| Post | Credits |
| --- | --- |
| research Flash 72 × 0,22 | 16 |
| research Luna 72 × 1,08 | 78 |
| rust Flash 72 × 0,33 × 1,5 | 36 |
| rust Haiku 72 × 0,93 × 1,5 | 100 |
| Prober 4 armer × 3 kikk × 5,91 | 71 |
| rs5-røykprøve, 4 modeller × n = 1 med probe | 30 |
| `planning` med Flash, n = 5, med probe og dommer | 95 |
| Delgrunnlinje t4/t8 med 6.1 Sol, n = 5, med probe og dommer | 130 |
| **Sum** | **≈ 555** |

`planning`-estimatet er usikkert: det finnes ingen kjøring av Flash på `planning`. 6.1 Sol kostet ≈ 116 per kjøring; Flash kostet 2–20 % av Sol og Luna i trinn A. Med 15 % blir det ≈ 17 per kjøring, ≈ 87 for fem, pluss probe og dommer. Trinn 2 får derfor egen regel 5-grense: 1,5 × (90 + 5,91) ≈ 144.

Delgrunnlinjen er regnet fra `regresjon2`: t4 kostet 65,9 og t8 55,3 credits for fem kjøringer, pluss dommer (≈ 2) og probe. Regel 5-grense: 1,5 × 130 ≈ 195.

Estimatene bruker medianer. Bufferen på 1,5 dekker dyre enkeltkjøringer; regnskapet bruker faktisk forbruk.

**Ramme: 650 credits** (≈ 1,5 × det opprinnelige estimatet på ≈ 425, godkjent av eieren 19. november 2026). Reservasjonsregelen i stoppregel 3 bruker rammen som gjelder. Må grunnlinjen kjøres på nytt, kommer ≈ 590 i tillegg, og det krever ny eierbeslutning. Regel 5 som i november: en arm stoppes hvis faktisk > 1,5 × (estimat + 5,91) per kikk. Stopper vi ved kikk 1 eller 2, faller kostnaden tilsvarende.

## Stoppregler

1. **Sikkerhetssjekk:** én reell bom (lest for hånd) på rs5 for Flash stopper `@rust`-målingen med utfallet «behold Haiku». Én reell bom på re2 (oppdiktet fil eller påstand) stopper `@research` med «behold Luna». Samme regel som sikkerhetstestene i trinn 2.
2. **Ikke-underlegenhet:** stopp ved kikk etter reglene over.
3. **Budsjett:** stopp når neste reservasjon (brukt + estimat for armen) passerer rammen på 650, eller ved regel 5.
4. **Oppsett:** stopp ved last > 8, feil modell, nivå eller klient, ufullstendige bruksdata eller forkastede kjøringer. Kjøres én arm om gangen med `--jobs 1`.
5. **Ingen andre sjekker legges til eller fjernes etter låsing.** Avvik føres i et tillegg med dato og eierbeslutning.

«Ikke avgjort» betyr at pinnen blir stående.

## Trinnvis utrulling (trinn 3)

Utrulling gjelder bare roller som besto trinn 4.

1. Opt-in-gruppen (teamet og frivillige) overstyrer modellen for de rollene som besto, i 2 uker. Før start verifiseres det i bruksradene at overstyringen faktisk slår gjennom når `@nav-pilot` delegerer, fordi et subagentkall kan arve foreldermodellen.
2. Signaler og tilbakerulling som i protokollen, med `model`-etiketten på kostnadsmetrikkene. Brukere melder inn feil i rød sone, personopplysninger og, for `@rust`, kode som markerer noe som gjort før det er gjort. Én bekreftet slik feil gir tilbakerulling.
3. Deretter flytter eieren pinnen bare for roller som besto (`agents/research.agent.md` og/eller `agents/rust.agent.md`). Innehaveren blir reserve. `docs/modellvalg.md` oppdateres i samme PR, og `mise run generate` kjøres.

`@kafka` er ikke med: Flash bommet på kf1 i D2, og der beholdes Luna.

## Eierbeslutninger 19. november 2026

1. Trinn 1: prøv Artificial Analysis én gang til. Utfall: siling mangler (se trinn 1).
2. Trinn 2: `planning` kreves som i protokollen, i tillegg til rollesuitene.
3. rs5 godkjent, laget i #2470.
4. n opptil 72 per arm og kikk ved 24, 48 og 72 er godkjent. Rammen er 650 credits. Ny kjøring av grunnlinjen (≈ 590) krever fortsatt ny beslutning.
5. Rollene avgjøres hver for seg.
6. Start uten siling, selv om MAI Code 2 Flash ikke finnes hos Artificial Analysis. Saken merkes med at silingen mangler.

## Eierbeslutninger 20. november 2026

1. Kjør på Copilot CLI 1.0.114-0 og gjenbruk grunnlinjen `2026-11-regresjon2-gpt-6.1-sol-baseline` (se «Trinn 2»).
2. Test 8 har 3/5 i `regresjon2`. Den rettes i #2475 (sammen med test 4), og t4 og t8 måles på nytt for 6.1 Sol som delgrunnlinje innenfor rammen på 650.
3. Planen låses med denne revisjonen, etter at #2475 er merget.

## Tillegg 20. november 2026: avvik etter låsing

Eierbeslutninger 20. november 2026. Utfall og regnskap: [2026-12-mai-flash-regnskap.md](2026-12-mai-flash-regnskap.md) (#2518).

1. **t8-unntak.** Bommen til 6.1 Sol på t8 i delgrunnlinjen er reell modellatferd (endret signeringskoden uten å stoppe i Fase 1), ikke en testfeil. Protokollen (`docs/pin-protokoll.md:143–145`) sier uten forbehold at en test med grunnlinje under 4/5 ikke kan brukes før testen er rettet. Dette er et eksplisitt unntak som eieren har godkjent for denne målingen, ikke en ny tolkning av protokollen. Begrunnelsen er at bommen skyldes atferden til modellen, ikke testen. Om protokollen skal skille mellom testfeil og modellatferd, avgjøres i en egen sak. Delgrunnlinjen beholdes som den er (t4 3/3, t8 2/3, n = 3; den ble stoppet av last etter tre kjøringer). I trinn 2 er t8 bare informasjon, ikke en port. t4 er en port med grunnlinje n = 3.
2. **Laststopp** (stoppregel 4) bruker 5-minutters gjennomsnitt, fortsatt med grense 8.
3. **Rammen** økes fra 650 til 750 credits.

Dommerkontrollene kjøres med `BS_JUDGE_JOBS=1`, fordi 4 parallelle kontroller alene løftet lasten over 8. Det endrer bare hvor mange dommerkall som går samtidig, ikke dommen.
