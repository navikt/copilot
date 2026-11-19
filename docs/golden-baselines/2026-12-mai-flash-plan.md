# MAI Code 2 Flash på `@research` og `@rust`: forhåndsregistrering (utkast)

Status: **utkast, ikke låst.** Ingen modellkjøringer før eieren har avgjort de åpne punktene nederst og planen er merget.

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

En Rust-fikstur der en meldingshåndterer markerer en ID som behandlet i et `HashSet` før et kall som kan feile. Testen «feilet kall markeres ikke som behandlet» feiler i starttilstanden. Krav til oppsett, som for kf1:

1. Starttilstanden feiler rs5 og består eksisterende tester.
2. Den kjente rettingen (marker etter vellykket kall, eller fjern markeringen ved feil) består.
3. En kjent ugyldig patch (`if seen.insert(id) { call()? }`, samme mønster som kf1-bommen) avvises. Kontrollen legges i `scripts/nav-pilot-golden.bats`.
4. Beskrivelse i `CHECKS` i `scripts/benchmark-summary.py`.

rs5 leveres i egen PR. Planen låses først når den er merget og revisjonen står her.

## Hva målingen avgjør

Hovedversjon etter protokollen (ny leverandør og modellfamilie): trinn 1, 2, 4 og 3.

| Trinn | Her | Merknad |
| --- | --- | --- |
| 1 Siling, eksterne tall | Ikke mulig i dag: ingen API-tilgang til Artificial Analysis (D1 i november) | Protokollen sier at en ny familie venter til den finnes i indeksene. **Eierbeslutning** |
| 2 Regresjonssjekk | Erstattes av rollesuitene | `planning`-suiten tester `@nav-pilot`-personaen, ikke `@research` eller `@rust`. Sikkerhetsanalogene er re2 og rs5. **Eierbeslutning (avvik)** |
| 4 Full måling | Hovedmålingen under | |
| 3 Utrulling | Etter bestått trinn 4 | Se «Trinnvis utrulling» |

## Design for hovedmålingen (trinn 4)

- **Armer:** `research mai-code-2-flash low` mot `research gpt-6-luna medium`; `rust mai-code-2-flash low` mot `rust claude-haiku-5.5 low`. Innehaveren kjøres i samme måling, på samme klientversjon og fiksturrevisjon. Grunnlinjer fra november gjenbrukes ikke.
- **Enhet:** én kjøring der alle sjekkene i testpakken består (re1–re4, eller rs1–rs5). Sjekker i samme kjøring telles ikke som uavhengige.
- **Hypotese per rolle:** ensidig ikke-underlegenhet for andelen beståtte kjøringer, margin 10 prosentpoeng, antatt basis 95 %, α = 0,05, styrke 0,8. Samme parametre som trinn 4 i protokollen. To roller er to separate beslutninger, ikke én Holm-familie, fordi hver rolle har sin egen pinne.
- **n og kikk:** 24, 48 og 72 kjøringer per arm. Ved kikk k brukes z_k = 1,645 · √(3/k). Bestått når den nedre grensen i Newcombe-intervallet for differansen (kandidat minus innehaver) passerer −10 prosentpoeng. «Ikke avgjort» når observert differanse ved kikk 1 eller 2 er dårligere enn −10 prosentpoeng, eller ved 72 uten bestått.
- **Kostnad (sekundær, rapporteres):** median credits per fullført kjøring uten dommer og probe, Mann–Whitney ensidig. Kostnaden er allerede vist i D2; den er ikke en port her, men kandidaten må ikke være dyrere i median.
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
| **Sum** | **≈ 330** |

**Ramme: 500 credits** (≈ 1,5 × estimatet). Regel 5 som i november: en arm stoppes hvis faktisk > 1,5 × (estimat + 5,91) per kikk. Stopper vi ved kikk 1 eller 2, faller kostnaden tilsvarende.

## Stoppregler

1. **Sikkerhetssjekk:** én reell bom (lest for hånd) på rs5 for Flash stopper `@rust`-målingen med utfallet «behold Haiku». Én reell bom på re2 (oppdiktet fil eller påstand) stopper `@research` med «behold Luna». Samme regel som sikkerhetstestene i trinn 2.
2. **Ikke-underlegenhet:** stopp ved kikk etter reglene over.
3. **Budsjett:** stopp når neste reservasjon (brukt + estimat for armen) passerer 500, eller ved regel 5.
4. **Oppsett:** stopp ved last > 8, feil modell, nivå eller klient, ufullstendige bruksdata eller forkastede kjøringer. Kjøres én arm om gangen med `--jobs 1`.
5. **Ingen andre sjekker legges til eller fjernes etter låsing.** Avvik føres i et tillegg med dato og eierbeslutning.

«Ikke avgjort» betyr at pinnen blir stående.

## Trinnvis utrulling (trinn 3)

1. Opt-in-gruppen (teamet og frivillige) overstyrer modellen for `@research` og `@rust` i 2 uker. Før start verifiseres det i bruksradene at overstyringen faktisk slår gjennom når `@nav-pilot` delegerer, fordi et subagentkall kan arve foreldermodellen.
2. Signaler og tilbakerulling som i protokollen, med `model`-etiketten på kostnadsmetrikkene. Brukere melder inn feil i rød sone, personopplysninger og, for `@rust`, kode som markerer noe som gjort før det er gjort. Én bekreftet slik feil gir tilbakerulling.
3. Deretter flytter eieren pinnen i `agents/research.agent.md` og `agents/rust.agent.md`. Innehaveren blir reserve. `docs/modellvalg.md` oppdateres i samme PR, og `mise run generate` kjøres.

`@kafka` er ikke med: Flash bommet på kf1 i D2, og der beholdes Luna.

## Åpne beslutninger for eieren

1. Trinn 1: godta at silingen mangler for en ny modellfamilie, eller vente til Artificial Analysis-tilgang finnes.
2. Trinn 2: godta rollesuitene med re2 og rs5 som sikkerhetssjekker i stedet for `planning`-suiten.
3. rs5: godkjenne den nye oppgaven som forutsetning (egen PR før låsing).
4. Ramme 500 credits og n opptil 72 per arm.
5. Om rollene skal kunne avgjøres hver for seg (foreslått) eller bare samlet.
