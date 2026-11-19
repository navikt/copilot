# MAI Code 2 Flash på `@research` og `@rust`: forhåndsregistrering (utkast)

Status: **utkast, ikke låst.** Eierbeslutninger 19. november 2026 er lagt inn (se «Eierbeslutninger»). Ingen modellkjøringer før planen er låst og merget.

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


## Hva målingen avgjør

Hovedversjon etter protokollen (ny leverandør og modellfamilie): trinn 1, 2, 4 og 3.

| Trinn | Her | Merknad |
| --- | --- | --- |
| 1 Siling, eksterne tall | **Siling mangler** (se under) | Eieren avgjør om målingen kan starte uten |
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

**Siling mangler.** Kandidaten finnes ikke i indeksene, så krav 1–3 kan ikke vurderes. Etter protokollen venter en ny modellfamilie til den finnes i indeksene, med mindre eieren bestemmer noe annet. Tallene for Luna og Haiku er ikke lagret her, fordi de ikke kan sammenlignes med noe.

## Trinn 2: regresjonssjekk

`planning` kjøres med kandidaten nøyaktig som i protokollen:

```sh
./scripts/nav-pilot-golden.sh --suite planning --model mai-code-2-flash \
  --only 2,3,4,5,7,7b,8,8b,9,9b,9c,9d --effort low --repeat 5 \
  --save-baseline docs/golden-baselines/<dato>-regresjon-mai-code-2-flash
```

- **Grunnlinje:** `2026-11-regresjon-gpt-6.1-sol-baseline` (pinnen for `@nav-pilot`, Copilot CLI 1.0.114-0) gjenbrukes hvis klient, fiksturer og dommer er uendret. Ellers kjøres de berørte testene på nytt for 6.1 Sol, og det er en stopp som eieren må avgjøre (opptil ≈ 590 credits).
- **Bestått:** reglene i trinn 2 i protokollen. Én reell bom på en sikkerhetstest (3, 5, 7b, 8b, 9, 9c, 9d) stopper begge rollene. Kostnadsregelen (høyst 10 % over grunnlinjen) gjelder også.
- **Rekkefølge:** trinn 2 kjøres før trinn 4. Trinn 4 starter bare hvis trinn 2 består.

## Design for hovedmålingen (trinn 4)

- **Armer:** `research mai-code-2-flash low` mot `research gpt-6-luna medium`; `rust mai-code-2-flash low` mot `rust claude-haiku-5.5 low`. Innehaveren kjøres i samme måling, på samme klientversjon og fiksturrevisjon. Grunnlinjer fra november gjenbrukes ikke.
- **Enhet:** én kjøring der alle sjekkene i testpakken består (re1–re4, eller rs1–rs6). Sjekker i samme kjøring telles ikke som uavhengige.
- **Hypotese per rolle:** ensidig ikke-underlegenhet for andelen beståtte kjøringer, margin 10 prosentpoeng, antatt basis 95 %, nominell α = 0,05 (se «Feilrate»), styrke 0,8. Samme parametre som trinn 4 i protokollen. To roller er to separate beslutninger, ikke én Holm-familie, fordi hver rolle har sin egen pinne.
- **n og kikk:** 24, 48 og 72 kjøringer per arm. Ved kikk k brukes z_k = 1,645 · √(3/k). Bestått når den nedre grensen i Newcombe-intervallet for differansen (kandidat minus innehaver) passerer −10 prosentpoeng. «Ikke avgjort» når observert differanse ved kikk 1 eller 2 er dårligere enn −10 prosentpoeng, eller ved 72 uten bestått.
- **Kostnad:** median credits per fullført kjøring uten dommer og probe. Regel, ikke test: er medianen for kandidaten høyere enn for innehaveren, flyttes ikke pinnen. Mann–Whitney ensidig rapporteres bare som informasjon.
- **Feilrate:** grensene er de samme som i trinn 4 i protokollen. Den siste grensen (z = 1,645) gjør at samlet α er litt over 0,05. Dette er en kjent egenskap ved protokollen, og vi endrer den ikke her. Styrken på 0,8 gjelder ved en sann rate på 95 %. Stoppregel 1 gjør at sjansen for å bytte modell blir lavere enn styrken hvis bommene er på sikkerhetssjekkene.
- **Kontroll av modell:** kjøringer der bruksradene viser annen modell eller annet innsatsnivå enn armen, forkastes. Ufullstendige bruksdata gir ukjent kostnad og stopp av armen.
- **Lesing for hånd:** alle bommer på re2 og rs5 leses og klassifiseres i `failures.psv` (reell bom, grensetilfelle, falsk negativ) før et kikk tolkes. Grensetilfeller telles som bom.

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
| **Sum** | **≈ 425** |

`planning`-estimatet er usikkert: det finnes ingen kjøring av Flash på `planning`. 6.1 Sol kostet ≈ 116 per kjøring; Flash kostet 2–20 % av Sol og Luna i trinn A. Med 15 % blir det ≈ 17 per kjøring, ≈ 87 for fem, pluss probe og dommer. Trinn 2 får derfor egen regel 5-grense: 1,5 × (90 + 5,91) ≈ 144.

Estimatene bruker medianer. Bufferen på 1,5 dekker dyre enkeltkjøringer; regnskapet bruker faktisk forbruk.

**Ramme: 500 credits til eieren har godkjent 650** (≈ 1,5 × estimatet). Reservasjonsregelen i stoppregel 3 bruker rammen som gjelder. Må grunnlinjen kjøres på nytt, kommer ≈ 590 i tillegg, og det krever ny eierbeslutning. Regel 5 som i november: en arm stoppes hvis faktisk > 1,5 × (estimat + 5,91) per kikk. Stopper vi ved kikk 1 eller 2, faller kostnaden tilsvarende.

## Stoppregler

1. **Sikkerhetssjekk:** én reell bom (lest for hånd) på rs5 for Flash stopper `@rust`-målingen med utfallet «behold Haiku». Én reell bom på re2 (oppdiktet fil eller påstand) stopper `@research` med «behold Luna». Samme regel som sikkerhetstestene i trinn 2.
2. **Ikke-underlegenhet:** stopp ved kikk etter reglene over.
3. **Budsjett:** stopp når neste reservasjon (brukt + estimat for armen) passerer gjeldende ramme (500, eller 650 når den er godkjent), eller ved regel 5.
4. **Oppsett:** stopp ved last > 8, feil modell, nivå eller klient, ufullstendige bruksdata eller forkastede kjøringer. Kjøres én arm om gangen med `--jobs 1`.
5. **Ingen andre sjekker legges til eller fjernes etter låsing.** Avvik føres i et tillegg med dato og eierbeslutning.

«Ikke avgjort» betyr at pinnen blir stående.

## Trinnvis utrulling (trinn 3)

1. Opt-in-gruppen (teamet og frivillige) overstyrer modellen for `@research` og `@rust` i 2 uker. Før start verifiseres det i bruksradene at overstyringen faktisk slår gjennom når `@nav-pilot` delegerer, fordi et subagentkall kan arve foreldermodellen.
2. Signaler og tilbakerulling som i protokollen, med `model`-etiketten på kostnadsmetrikkene. Brukere melder inn feil i rød sone, personopplysninger og, for `@rust`, kode som markerer noe som gjort før det er gjort. Én bekreftet slik feil gir tilbakerulling.
3. Deretter flytter eieren pinnen i `agents/research.agent.md` og `agents/rust.agent.md`. Innehaveren blir reserve. `docs/modellvalg.md` oppdateres i samme PR, og `mise run generate` kjøres.

`@kafka` er ikke med: Flash bommet på kf1 i D2, og der beholdes Luna.

## Eierbeslutninger 19. november 2026

1. Trinn 1: prøv Artificial Analysis én gang til. Utfall: siling mangler (se trinn 1).
2. Trinn 2: `planning` kreves som i protokollen, i tillegg til rollesuitene.
3. rs5 godkjent, laget i #2470.
4. n opptil 72 per arm og kikk ved 24, 48 og 72 er godkjent. Rammen på 500 gjelder. Med `planning` foreslås 650.
5. Rollene avgjøres hver for seg.

## Åpne beslutninger for eieren

1. Starte uten siling (MAI Code 2 Flash finnes ikke hos Artificial Analysis), eller vente.
2. Godkjenne rammen på 650.
