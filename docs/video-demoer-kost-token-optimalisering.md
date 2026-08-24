# Video-demoer (3–5 min): kost/token-optimalisering for alle Nav-utviklere

Kort serie for alle utviklere i Nav som bruker Copilot i det daglige.

## Mål for serien

- Redusere unødvendig tokenbruk per oppgave.
- Øke andel oppgaver som løses uten unødvendig modell-eskalering.
- Gjøre teamene bedre på konteksthygiene og verktøybruk.

## Målgruppe

- Alle utviklere i Nav som bruker Copilot (uavhengig av team og kodebase).
- Tech leads som følger kosttrend, kvalitet og modellbruk.

## Avgrensing for demoene

- `navikt/copilot` brukes kun som demo-/referanseapplikasjon i videoene.
- Mønstrene i videoene skal være overførbare til andre Nav-repoer.
- Eksempler i monorepoet brukes for å gjøre demoene konkrete og repeterbare.

## Prinsipp: hver video skal fungere alene

- Hver episode starter med kort kontekst (problem, mål og hva du får igjen).
- Hver episode viser ett konkret før/etter-scenario uten krav om å ha sett tidligere episoder.
- Rød tråd beholdes, men alle nøkkelbegreper forklares kort på nytt i hver video.
- Hver episode avsluttes med en selvstendig sjekkliste du kan bruke med en gang.

## Publiseringsplan (6+4)

**Kjerneepisoder (for alle):**
1. Episode 1: Presis prompt på første forsøk
2. Episode 2: En oppgave per tråd
3. Episode 3: Riktig agentvalg
4. Episode 4: Tool-first workflow
5. Episode 5: Kort output uten kvalitetstap
6. Episode 6: Kosteffektiv PR-flyt

**Bonusepisoder (rolle/spesialisert):**
1. Bonus episode A: Tre dyre anti-mønstre
2. Bonus episode B: Mål effekt i statistikk
3. Bonus episode C: Chronicle — forstå og optimaliser context
4. Bonus episode D: Cplt sandbox — kom i gang på 3 minutter
5. Bonus episode E: Fire grep som faktisk senker regningen

## Produksjonsstatus

| Episode | Status | Kommentar |
| --- | --- | --- |
| Episode 1 | Spilt inn | Lås manus, metadata og overlay. |
| Episode 2 | Spilt inn | Lås manus, metadata og overlay. |
| Episode 3 | Spilt inn | Lås manus, metadata og overlay. |
| Episode 4 | Planlagt | Kan fortsatt justeres. |
| Episode 5 | Planlagt | Kan fortsatt justeres. |
| Episode 6 | Planlagt | Kan fortsatt justeres. |
| Bonus A | Planlagt | Kan fortsatt justeres. |
| Bonus B | Planlagt | Kan fortsatt justeres. |
| Bonus C | Planlagt | Fokus: `/context`, `tips`, `cost-tips`, `improve`. |
| Bonus D | Spilt inn | Lås manus, metadata og overlay. |
| Bonus E | Planlagt | Fokus: modellvalg, resonneringsnivå, prompt-cache, mindre arbeid. Erstatter tidligere rtk-utkast. |

**Regel:** Ikke endre innholdet i episoder merket **Spilt inn**. Juster bare status, beskrivelser eller produksjonsnotater ved behov.

## Videobeskrivelser for detaljsider

Korte, actionable artikkelsammendrag for hver episode ligger nederst i denne samme fila, slik at video-plan og detaljtekster holdes samlet og synkronisert.

- **Format:** 2-3 avsnitt per episode, ~150-200 ord
- **Tone:** Konkret problem først, spesifik gevinst, actionable ending
- **Mål:** Hjelp seeren bestemme seg og gi en forhåndsvisning av hva de lærer

## Krav: hver innspilling skal gi reell verdi i repoet

For hver episode skal vi velge en konkret oppgave fra backlog før opptak, og output skal ende i én av disse:

1. Mergebar kodeendring (helst PR samme dag).
2. Mergebar dokumentasjonsendring med tydelig effekt på teampraksis.
3. Verifisert beslutningsunderlag (ADR/plan) som brukes i neste implementeringssteg.

**Stoppregel:** Hvis oppgaven ikke gir en konkret forbedring i `navikt/copilot`, stopper vi opptaket og velger en ny oppgave.

## #meta-spor: større oppgave for serien (video hosting/playback)

Bruk PRD-en `docs/prd-video-hosting-og-visning.md` som kilde for større, ekte arbeidspakker:

1. Del 1: API-kontrakt i `copilot-api` (`/public/v1/videos`, `/play`, `/captions`).
2. Del 2: Metadata-/manifestmodell og validering.
3. Del 3: Signed URL-flyt og caching.
4. Del 4: Frontend shorts-feed på forsiden (`my-copilot`), inkl. autoplay muted, lazy loading og tastaturnavigasjon.
5. Del 5: Teksting, fallback og observability/KPI-events.

**Mål med #meta:** Minst én leveranseklar deloppgave per episodeblokk når vi bruker tokens på større arbeid.

## Mechanical overlay metadata

Bruk dette som frontend-metadata, ikke som hardkodet app-logikk. Den ekstraherte posteren skal være basebildet; overlayene rendres mekanisk i frontend fra feltene under.

**Felles stilregel:** behold terminalbildet og ansiktet, legg overlays i ledig plass, bruk høy kontrast, tydelig typografi og serieidentitet, og unngå å dekke viktig innhold. Hver episode må ha eget fargegrep og eget motiv.

### Felles skjema

```ts
type OverlayKind =
  | "episode-number"
  | "badge"
  | "chip"
  | "rule-pill"
  | "counter"
  | "ladder"
  | "pipeline"
  | "compare-bars"
  | "kpi-grid"
  | "warning-cards"
  | "index-list";

interface OverlayComponent {
  kind: OverlayKind;
  anchor: "top-left" | "top-right" | "center-left" | "center-right" | "bottom-left" | "bottom-right" | "bottom-full";
  labels: string[];
  highlightIndex?: number;
  monospace?: boolean;
}

interface EpisodeOverlayMeta {
  id: string;
  title: string;
  accent: string;
  secondaryAccent?: string;
  motif: string;
  poster: string;
  components: OverlayComponent[];
}
```

### Episode notes

Episode-spesifikk overlay metadata, manus, oppsummering og outro ligger under hver episodeheading lenger nede i dokumentet.

## Frontend-lag

- Baseposter: uendret, full-bleed object-cover.
- Scrim per overlay: liten mørk plate bak hver komponent, ikke heldekkende dim.
- Motivlag: stige, pipeline, bars, index-list som SVG/DOM.
- Tekstlag: chips, badges og serienummer.
- Beskyttede soner: tittel og webcam-inset skal aldri få overlays.

## Overgang fra AI-prompt til mekanisk overlay

1. Behold metadatafeltene, men bytt ut generator-prompt med `components`.
2. La frontend rendere `OverlayComponent[]` direkte.
3. Bruk `accent` og `secondaryAccent` til kantlinjer og utheving, ikke til store fargeflater.
4. Valider at hver episode har minst ett unikt motiv og en unik primæraaccent.

## Episode 1: Presis prompt på første forsøk

**Status:** Spilt inn

**Overlay:** 01 · amber · crosshair + prompt card · Mål / Fil / Begrensning / Output · cost-optimization.tsx · ✓

**Mål:** Vise hvordan presis første melding kutter antall runder.

**Kan sees alene fordi:** Vi forklarer grunnbegrepet "presis prompt" på 20 sek før demo.

**Script-outline (one-take):**
1. "Hei og velkommen! I dag viser jeg hvordan du skriver en presis prompt som gir bedre svar på første forsøk."
2. Vis oppgaven og hva som skal forbedres i repoet.
3. Kjør dyr variant og vis hvorfor den gir flere runder.
4. Kjør billig variant med presis scope og sammenlign resultat.
5. Oppsummer regelen i én setning og pek på konkret filendring.

**Innhold (3–5 min):**
1. Start med en vag prompt mot en liten endring i `apps/my-copilot/src/app/praksis/`.
2. Vis resultat: flere avklaringsrunder, mer tekst, flere tokens.
3. Kjør samme oppgave med presis prompt (fil, scope, ønsket output, begrensninger).
4. Sammenlign antall runder og kvalitet.

**Demo-kontekst (referanserepo):** Frontend-endring i `my-copilot` er lett å demonstrere visuelt.

**Reell oppgave i repo (velg én før opptak):**
- Forbedre eksisterende praksis-innhold i `apps/my-copilot/src/app/praksis/` med tydeligere handling/eksempel.

**Ta med deg videre:** Bruk en 4-linjers promptmal (mål, fil/område, begrensninger, forventet output).

**Prompt-manus (copy/paste):**

```text
# Dyr variant
Kan du forbedre praksis-siden?
```

```text
# Billig variant
Mål: Oppdater seksjon om kostoptimalisering med informasjon om hvordan AGENTS.md / copilot-instructions.md påvirker tokenkostnad og konkrete anbefalinger for å redusere kost i praksis med å holde innholdet i disse filene minimalt og relevant.
Fil: apps/my-copilot/src/app/praksis/sections/cost-optimization.tsx
Begrensning: Ikke endre andre filer. Bruk eksisterende Nav DS-komponenter.
Output: Vis kun patch + 2 linjer forklaring.
```

**Forventet respons-signal:** Færre avklaringsrunder, én fil endres, kort svar.

**Overgang til neste episode:** Vi tar opp igjen samme arbeidsflyt med `/resume`, ser på endringen vi nettopp gjorde i kostoptimalisering, og starter deretter en ny tråd for neste mål.

---

## Episode 2: En oppgave per tråd (`/clear` og `/compact`)

**Status:** Spilt inn

**Overlay:** 02 · teal · split threads + scissors node · /resume /compact /clear · chronicle search · .../adoption/summary · nytt mål = ny tråd

**Mål:** Lære når man skal starte ny tråd for å unngå dyr kontekst.

**Kan sees alene fordi:** Vi forklarer `/resume`, `/clear` og `/compact` i starten før vi viser feil vs riktig flyt.

**Script-outline (one-take):**
1. "Hei og velkommen! Vi fortsetter der episode 1 slapp, bruker `/resume` for å hente opp tråden, og viser hvorfor én oppgave per tråd gir lavere kost og bedre presisjon."
2. Vis endringen vi nettopp gjorde i praksis-koden for kostoptimalisering.
3. Kjør `/resume` for å hente tilbake historikken fra forrige arbeid.
4. Kjør `/compact` og `/clear` når du ser at neste mål er en ny oppgave.
5. Start ny tråd med ett tydelig mål og sammenlign kvalitet.
6. Avslutt med regelen: nytt mål = ny tråd.

**Innhold (3–5 min):**
1. Vis at episode 1 nettopp endte med en konkret endring i kostoptimaliserings-innholdet.
2. Hent samme tråd tilbake med `/resume` og vis den siste relevante historikken.
3. Demonstrer hvordan kvalitet/kost blir dårligere når du blander flere mål i samme tråd.
4. Kjør samme arbeid med:
   - ny tråd ved nytt mål
   - `/compact` før handoff
   - `/clear` når historikk er irrelevant
5. Vis at Chronicle er for når du faktisk trenger å finne igjen gammel sesjonshistorikk, ikke for å dra med deg alt inn i samme tråd.
6. Oppsummer enkel regel: én tydelig oppgave per tråd.

**Demo-kontekst (referanserepo):** Bytt mellom `docs/`, `apps/copilot-api/` og `apps/my-copilot/`.

**Reell oppgave i repo (velg én før opptak):**
- Isoler én konkret feil i `apps/copilot-api` og lever kun den fiksen i egen tråd.

**Ta med deg videre:** Regel: nytt mål = ny tråd.

**Prompt-manus (copy/paste):**

```text
# Resume først
/resume
Oppsummer kun siste relevante endring i kostoptimaliserings-innholdet.
Output: 1) hvilken fil, 2) hva som ble endret, 3) hva som blir neste naturlige oppgave.
```

```text
/compact
/clear
```

Hvis du trenger historikk:

```text
/chronicle search "adoption summary"
```

Ny tråd:

```text
Ny oppgave:
Oppgave: Finn og forklar årsaken til 500-feil i /api/v1/copilot/adoption/summary.
Scope: Kun apps/copilot-api.
Output: 1) sannsynlig rotårsak, 2) berørte filer, 3) konkret fix-forslag.
```

**Forventet respons-signal:** Assistenten holder seg til én kodeflate og ett mål.

---

## Episode 3: Riktig modus og agentnivå (default ask/execute -> plan -> evt. autopilot)

**Status:** Spilt inn

**Overlay:** 03 · indigo/orange · autonomy ladder + agent stack · ask/execute / plan /autopilot · @research-agent / @nav-pilot / @nav-pilot-opus · DATE/string-mismatch

**Mål:** Vise hvordan du velger modus først (default ask/execute, plan mode, autopilot), og agentnivå etterpå.

**Kan sees alene fordi:** Vi forklarer modekartet først, og viser én konkret flyt fra lav til høy autonomi.

**Modekart for dagens Copilot CLI (vises tidlig i episoden):**
1. **Default ask/execute mode**: Start her. Brukes til vanlig dialog, avklaringer og små oppgaver.
2. **Plan mode**: Bytt med `Shift+Tab` når du vil ha en plan og avklaringer før du gjør endringer.
3. **Autopilot**: Slå på med `/autopilot` når oppgaven er tydelig avgrenset og du vil delegere mer.

**Agentnivå etter modusvalg:**
1. **Chat (uten `@agent`)** i default ask/execute mode for enkle avklaringer og rutineendringer.
2. **`@research-agent`** når du må kartlegge en ukjent kodeflate.
3. **`@nav-pilot`** når du trenger en konkret plan med avgrensning og tradeoffs.
4. **`@nav-pilot-opus`** kun for smal høyrisikovurdering.

**Script-outline (one-take):**
1. "Hei og velkommen! I dag viser jeg forskjellen på default ask/execute mode, plan mode og autopilot i Copilot CLI."
2. Start i default ask/execute mode og vis en kort rutineoppgave.
3. Bytt til plan mode med `Shift+Tab` og vis at du får en tydeligere plan før endringer.
4. Vis `/autopilot`, forklar når det passer, og vis hvordan du går tilbake til manuell styring.
5. Avslutt med regelen: start lavt, øk autonomi ved behov.

**Innhold (3–5 min):**
1. Case: 500-feil i `copilot-api` med DATE/string-mismatch.
2. Default ask/execute mode: avklar problem og scope i chat.
3. Plan mode (`Shift+Tab`): få en tydelig plan før implementering.
4. Agentnivå: `@research-agent` -> `@nav-pilot` -> evt. `@nav-pilot-opus`.
5. Autopilot (`/autopilot`): vis kort av/på og forklar at modusen passer best for tydelig avgrensa oppgaver.

**Demo-kontekst (referanserepo):** Høyrisiko-case i `apps/copilot-api`, lavrisiko-case i `docs/`.

**Reell oppgave i repo (velg én før opptak):**
- Kartlegg berørte filer for én faktisk bugfix, lag plan, og velg deretter riktig agentnivå.

**Ta med deg videre:** Velg modus først (default ask/execute -> plan mode -> evt. `/autopilot`), deretter agentnivå.

**Prompt-manus (CLI, kjørbar):**

```text
# Default ask/execute mode
Mål: Forklar 500-feilen i /api/v1/copilot/adoption/summary.
Kontekst: Feil ved henting av adopsjonsdata med DATE/string-mismatch.
Scope: Kun apps/copilot-api. Ikke foreslå kodeendringer ennå.
Output: 1) sannsynlig rotårsak, 2) 2-5 berørte filer med én linje begrunnelse per fil, 3) hva som må avklares før plan.
```

```text
# Plan mode (Shift+Tab)
Mål: Lag en implementeringsplan før kode for DATE/string-mismatch i /api/v1/copilot/adoption/summary.
Scope: Kun apps/copilot-api. Ingen kodeendringer nå.
Krav: 5 punkter, risiko per punkt, og tydelig rekkefølge for gjennomføring.
Output: nummerert plan.
```

```text
@research-agent Kartlegg filer som påvirker /api/v1/copilot/adoption/summary i apps/copilot-api.
Output: tabell med Fil | Ansvar | Hvorfor relevant.
```

```text
@nav-pilot Med utgangspunkt i kartleggingen: foreslå minste sikre endring for DATE/string-mismatch.
Krav: stabil API-kontrakt, ingen sideeffekter utenfor berørte filer.
Output: endringsplan i 5 punkter + risiko per punkt.
```

```text
# Autopilot (valgfritt i demo)
/autopilot
```

```text
# Stopp/tilbake til manuell styring
/autopilot
```

**Fallback hvis `@...` ikke er tilgjengelig i klienten:** Be om samme flyt i chat: "avklaring i default ask/execute mode, plan i plan mode (`Shift+Tab`), deretter kartlegging og forslag".

**Forventet respons-signal:** Tydelig progresjon fra lav til høy autonomi, uten unødvendig eskalering.

---

## Episode 4: Tool-first workflow (deterministisk først, LLM etterpå)

**Status:** Planlagt

**Overlay:** 04 · cyan · tool pipeline -> AI node · gh pr view 275 / git diff / rg · verktøy først, modell etterpå

**Mål:** Vise hvordan CLI-funn før LLM reduserer tokenforbruk og feil.

**Kan sees alene fordi:** Vi forklarer "tool-first" i én setning og viser full flyt fra null.

**Script-outline (one-take):**
1. "Hei og velkommen! I dag viser jeg tool-first: verktøy først, modell etterpå."
2. Vis bred prompt uten forarbeid som baseline.
3. Kjør `gh`, `git diff` og `rg` for målrettede funn.
4. Gi funnene til modellen og sammenlign med baseline.
5. Avslutt med én konkret forbedring levert i repo/PR.

**Innhold (3–5 min):**
1. Start med å be LLM "tolke alt" uten forarbeid.
2. Kjør så tool-first:
   - `gh` for PR/review-data
   - målrettet filvisning/diff
   - enkel grep/søk
3. Gi funnene til LLM for syntese.
4. Sammenlign svarlengde, presisjon og iterasjoner.

**Demo-kontekst (referanserepo):** PR-flyt i dette repoet med endringer i både docs og app-filer.

**Reell oppgave i repo (velg én før opptak):**
- Kjør tool-first på en aktiv PR og lever en presis, avgrenset forbedring.

**Ta med deg videre:** Kjør verktøy først, be LLM om syntese etterpå.

**Oppsummering:** Tool-first betyr at du samler fakta med verktøy før du spør modellen. Det gir kortere svar og færre gjetninger.

**Outro:** Neste gang du står fast, start med `rg`, `git diff` eller `gh` før du skriver en stor prompt.

**Steg-for-steg manus (CLI):**

1. Kjør verktøy først:
   - `gh pr view 275 --comments`
   - `git --no-pager diff --name-only origin/main...HEAD`
   - `rg -n "scan_date|adoption|customization" apps/copilot-api`
2. Send prompt:

```text
Bruk funnene fra kommandoene over.
Oppgave: Gi topp 3 konkrete problemer med filreferanser.
Output: tabell med kolonnene Problem, Fil, Foreslått tiltak.
```

3. Sammenlign mot bred prompt:

```text
Se på hele repoet og finn hva som er galt i PR-en.
```

**Forventet respons-signal:** Tool-first gir kortere, mer presis respons med færre gjetninger.

---

## Tilleggsepisode (v2): Debugging i rød sone

**Status:** Planlagt

**Overlay:** rød sone · symptom → hypoteser → test → minste sikre endring

**Mål:** Lære når du skal kode og feilsøke selv først, og bruke AI målrettet etterpå.

**Kan sees alene fordi:** Vi viser én konkret feilflyt fra symptom til verifisert løsning.

**Script-outline (one-take):**
1. "Hei og velkommen! I dag feilsøker vi en ekte 500-feil uten å delegere tenkingen for tidlig."
2. Beskriv symptom og lag tre testbare hypoteser.
3. Test hypoteser med kommandoer og vis hva som falsifiseres.
4. Be AI om minste sikre endring basert på funn.
5. Oppsummer hva som var rød sone og hva AI hjalp med.

**Innhold (3–5 min):**
1. Beskriv bug og lag 3 egne hypoteser før AI.
2. Test hypotesene med målrettede kommandoer.
3. Gi AI kun relevant feilkontekst (ikke hele loggen).
4. Verifiser med kort sjekkliste.

**Prompt-manus (copy/paste):**

```text
Jeg feilsøker en 500-feil. Før du foreslår løsning: hjelp meg lage 3 testbare hypoteser.
Kontekst:
- Endepunkt: /api/v1/copilot/adoption/summary
- Feil: schema field scan_date of type DATE is not assignable to struct field scan_date of type string
Output: Hypotese | Hvordan teste | Hva som falsifiserer hypotesen.
```

```text
Basert på testresultatene: foreslå minste sikre kodeendring.
Krav: behold API-kontrakt, begrens til berørte filer.
Output: konkret patch-plan + verifiseringssjekkliste i 4 punkter.
```

**Forventet respons-signal:** Assistenten foreslår hypoteser først, ikke bred "fiks alt"-løsning.

**Oppsummering:** Når feilen er uklar, start med hypoteser og tester før du ber modellen om en løsning.

**Outro:** Feilsøking blir billigere når du avgrenser tidlig og bare sender relevant kontekst videre.

---

## Bonus episode A: Tre dyre anti-mønstre

**Status:** Planlagt

**Overlay:** A · red · warning cards · full log-dump / for bred prompt / feil modellvalg · rg først

**Mål:** Gjøre vanlige kostfeil konkrete og lette å unngå.

**Kan sees alene fordi:** Episoden er en ren anti-mønsterliste med korte "gjør dette i stedet"-eksempler.

**Script-outline (one-take):**
1. "Hei og velkommen! I dag viser jeg tre vanlige anti-mønstre som gjør Copilot dyrere enn nødvendig."
2. Vis anti-mønster 1 og forbedret variant.
3. Vis anti-mønster 2 og forbedret variant.
4. Vis anti-mønster 3 og forbedret variant med `rg` først.
5. Avslutt med en enkel sjekkliste før du sender neste prompt.

**Innhold (3–5 min):**
1. Full log-dump i prompt.
2. For bred prompt ("fiks alt i repoet").
3. Feil modellvalg/eskalering.

**Demoformat:** 40–50 sek per anti-mønster + kort "gjør dette i stedet".

**Demo-kontekst (referanserepo):** Bruk utdrag fra tidligere arbeid i `nav-pilot/docs` + `praksis`-sider.

**Ta med deg videre:** Unngå disse tre anti-mønstrene før du ber om mer avansert optimalisering.

**Oppsummering:** De dyreste feilene er ofte de enkleste å unngå: for bred scope, for mye støy og feil modellvalg.

**Outro:** Start smalt, bruk `rg` først, og be om mer bare når det faktisk trengs.

**Prompt-manus (inkluder grep-instruksjon):**

```text
Jeg viser et anti-mønster. Lag en bedre prompt med samme mål, men mindre scope.
Krav: Be agenten bruke `rg` først for å begrense filer før videre analyse.
Output: Før/etter i maks 6 linjer.
```

**Eksempel "etter"-prompt som skal vises i video:**

```text
Bruk `rg -n "nav-pilot|cost|token" docs apps/my-copilot/src/app/praksis` først.
Les kun filer som matcher.
Deretter: foreslå én konkret forbedring med patch-format.
```

**Forventet respons-signal:** Assistenten starter med målrettet søk i stedet for bred repo-lesing.

---

## Episode 5: Kort output uten kvalitetstap (`terse-mode`)

**Status:** Planlagt

**Overlay:** 05 · lime · before/after density bars · @terse-mode · full/kort · maks 5 punkter · 1 linje

**Mål:** Lære når kort output gir bedre flyt.

**Kan sees alene fordi:** Vi sammenligner samme oppgave med og uten `terse-mode` fra blank start.

**Script-outline (one-take):**
1. "Hei og velkommen! I dag viser jeg når kort output gir raskere flyt uten å miste kvalitet."
2. Kjør samme oppgave uten `terse-mode`.
3. Kjør samme oppgave med `@terse-mode`.
4. Sammenlign informasjonstetthet og handlingsevne.
5. Avslutt med regel: kort som standard, utvid ved risiko.

**Innhold (3–5 min):**
1. Samme oppgave med og uten `terse-mode`.
2. Vis at strukturert, kort output er nok for rutineoppgaver.
3. Vis når man bør be om mer detalj (sikkerhet/tradeoffs).

**Demo-kontekst (referanserepo):** Små docs- og konfigoppgaver i `.github/agents/` og `docs/`.

**Reell oppgave i repo (velg én før opptak):**
- Kutt unødvendig tekst i én konkret fil uten å miste teknisk innhold.

**Ta med deg videre:** Kort output som standard, utvid bare når risikoen krever det.

**Oppsummering:** `terse-mode` er nyttig når oppgaven er liten og tydelig. Da får du det viktigste raskere.

**Outro:** Bruk kort format som standard, og be først om mer detalj når du faktisk trenger det.

**Prompt-manus (copy/paste):**

```text
# Uten terse-mode
Gi en full gjennomgang av hva som bør forbedres i docs/video-demoer-kost-token-optimalisering.md.
```

```text
@terse-mode
Gå gjennom docs/video-demoer-kost-token-optimalisering.md.
Output: maks 5 konkrete forbedringer, én linje per punkt.
```

**Forventet respons-signal:** Samme kjerneinnhold, men kortere output.

---

## Bonus episode B: Mål effekt i statistikk (ikke bare "følelse")

**Status:** Planlagt

**Overlay:** B · blue-teal · dashboard · kost/uke · andel eskalerte · runder per oppgave

**Mål:** Knytte arbeidsmåte til målbar kostutvikling.

**Kan sees alene fordi:** Vi forklarer KPI-ene i episoden og trenger ikke historikk fra andre videoer.

**Script-outline (one-take):**
1. "Hei og velkommen! I dag viser jeg hvordan vi måler om token-optimalisering faktisk virker."
2. Vis hvor tallene hentes i løsningen.
3. Definer tre KPI-er med terskler og tiltak.
4. Vis ukentlig 15-min gjennomgangsformat.
5. Avslutt med én konkret vane teamet endrer neste uke.

**Innhold (3–5 min):**
1. Vis hvor teamet kan følge utvikling i `my-copilot` (kost/statistikk-visning).
2. Definer 2–3 enkle team-KPI-er:
   - kost per uke
   - andel eskalerte oppgaver
   - antall runder per oppgavetype
3. Forklar ukentlig justeringssløyfe.

**Demo-kontekst (referanserepo):** `apps/my-copilot/src/app/statistikk/` og praksissider.

**Ta med deg videre:** Mål ukentlig, juster én vane av gangen.

**Oppsummering:** Målene gjør at teamet ser om små grep faktisk virker. Da blir kostoptimalisering konkret, ikke bare følelse.

**Outro:** Ta én KPI og én vane inn i neste uke, og juster på nytt etterpå.

**Prompt-manus (copy/paste):**

```text
Lag 3 KPI-er for kost/token-optimalisering i teamet.
Krav: hver KPI må ha definisjon, datakilde og ukentlig tiltak ved avvik.
Output: tabell.
```

```text
Basert på KPI-tabellen: lag en ukentlig 15-min sjekkliste i 5 steg.
Krav: Hvert steg må peke til én KPI og ett konkret tiltak ved avvik.
Output: nummerert liste med maks 1 linje per steg.
```

**Forventet respons-signal:** KPI-er blir konkrete nok til å brukes direkte i teammøte.

---

## Episode 6: Kosteffektiv PR-flyt

**Status:** Planlagt

**Overlay:** 06 · green · PR pipeline · commit / PR / review / fix · 1 nå / 2 senere / 3 avvist · små PR-er, tydelig scope

**Mål:** Vise hvordan små, tydelige PR-er senker review-kost.

**Kan sees alene fordi:** Vi viser hele mini-flyten (commit, PR, review-fiks) i én episode.

**Script-outline (one-take):**
1. "Hei og velkommen! I dag viser jeg en kosteffektiv PR-flyt fra endring til review-fiks."
2. Del opp endringene i logiske commits.
3. Skriv kort PR-body med tydelig scope.
4. Hent kommentarer og prioriter hva som må fikses nå.
5. Avslutt med ferdig avgrenset leveranse uten overfiksing.

**Innhold (3–5 min):**
1. Del opp endringer i logiske commits.
2. Skriv kort PR-body med tydelig scope.
3. Hent review-kommentarer målrettet og fiks kun relevante funn.
4. Sammenlign med "stor PR + bred prompt".

**Demo-kontekst (referanserepo):** Bruk faktisk flyt med docs-endringer + review-fiks i denne monorepoen.

**Reell oppgave i repo (velg én før opptak):**
- Lever en liten PR med tydelig scope og konkret oppfølging av review-kommentarer.

**Ta med deg videre:** Små PR-er + tydelig scope gir lavere kost og raskere review.

**Oppsummering:** En liten PR med tydelig scope er lettere å reviewe, lettere å fikse og billigere å jobbe med.

**Outro:** Del opp endringer, skriv kort scope og fiks bare det review faktisk peker på.

**Prompt-manus (copy/paste):**

```text
Skriv PR-body for endringer i:
- docs/video-demoer-kost-token-optimalisering.md
- .github/agents/nav-pilot.agent.md
Krav: kort scope, hva som er endret, hva som ikke er endret, hvordan verifisere.
Format: markdown, maks 12 linjer.
```

```text
Hent review-kommentarer for PR #275.
Klassifiser: 1) må fikses nå, 2) kan tas senere, 3) avvises med begrunnelse.
Krav: Kun kommentarer som gjelder filene i denne PR-en. Ikke foreslå nye features.
Output: tabell med kolonnene Kommentar | Klasse | Begrunnelse | Foreslått handling.
```

**Forventet respons-signal:** Presis prioritering, ingen overfiksing utenfor scope.

---

## Bonus episode C: Chronicle — forstå og optimaliser context

**Status:** Planlagt

**Overlay:** C · violet/indigo · searchable session index · /context · tips · cost-tips · improve

**Mål:** Vise hvordan du bruker `/context` og Chronicle til å forstå hva som bruker budsjettet, og hvordan du optimaliserer Copilot-bruken over tid.

**Kan sees alene fordi:** Vi forklarer context-budsjett, Chronicle-tipsene og én konkret forbedringsflyt før demoen starter.

**Narrativ tråd fra episode 1 og 2:** Episode 1 viser presis prompt. Episode 2 viser når du skal starte ny tråd. Bonus C viser hvordan du holder resten av budsjettet nede med `/context`, tips og cost-tips.

**Script-outline (one-take):**
1. "Hvis Copilot føles tungt, er det ofte context-budsjettet som er problemet. Her ser vi hva som bruker plass, og hvordan Chronicle hjelper oss å bruke mindre."
2. Kjør `/context` og forklar kort hvilke deler som deler samme budsjett.
3. Kjør `/chronicle tips` og vis et konkret tips som forbedrer arbeidsflyten.
4. Kjør `/chronicle cost-tips` og vis ett råd som kan senke tokenbruk.
5. Kjør `/chronicle improve` og vis hvordan repo-instruksjoner kan bli bedre.
6. Pek kort på `/chronicle search` og `/chronicle standup` som støttefunksjoner, ikke hovedpoeng.

**Innhold (3–5 min):**
1. Vis hvordan `/context` gjør deg bevisst på hva som faktisk ligger i budsjettet.
2. Vis `/chronicle tips` som en måte å få konkrete forbedringer på arbeidsflyten.
3. Vis `/chronicle cost-tips` for å finne et lite, konkret sted å spare tokens.
4. Vis `/chronicle improve` for forslag til bedre repo-instrukser.
5. Avslutt med regelen: mindre context gir bedre svar, og Chronicle hjelper deg å holde kursen over tid.

**Demo-kontekst (referanserepo):** Bruk en nylig sesjon i dette repoet der du faktisk kan peke på et konkret context-problem eller et valg som kan forbedres med Chronicle-tips.

**Ta med deg videre:** `/context` viser hva som bruker budsjettet. Chronicle viser hva du kan gjøre bedre.

**Oppsummering:** `/context` viser hva som tar plass, mens `tips`, `cost-tips` og `improve` viser hva du kan forbedre.

**Outro:** Bruk mindre context, og la Chronicle hjelpe deg å finne ett konkret neste grep.

**Prompt-manus (copy/paste):**

Vis context:
```text
/context
```

Tips:
```text
/chronicle tips
```

Kosttips:
```text
/chronicle cost-tips
```

Forbedringsforslag:
```text
/chronicle improve
```

Valgfri støtte:
```text
/chronicle search "cost optimization"
```

Bygg indeks på nytt:
```text
/chronicle reindex
```

**Forventet respons-signal:** `/context` viser hva som tar plass, `tips` gir et konkret arbeidsflytråd, og `cost-tips` gir ett råd du faktisk kan bruke.

**Kilder:**
- GitHub Changelog: [Gain insights across your agent sessions with /chronicle](https://github.blog/changelog/2026-06-02-gain-insights-across-your-agent-sessions-with-chronicle/)
- GitHub Docs: [About GitHub Copilot CLI session data](https://docs.github.com/en/copilot/concepts/agents/copilot-cli/chronicle)

## Bonus D: Cplt sandbox — kom i gang på 3 minutter

**Status:** Spilt inn

**Overlay:** D · emerald/cyan · bootstrap flow + init command · clone / init / test / publish · nav-copilot-cplt-init.mp4

**Mål:** Vise hvordan du setter opp og tester en cplt-sandbox raskt fra null.

**Kan sees alene fordi:** Vi viser hele oppstartsflyten i én kort demo.

**Script-outline (one-take):**
1. "Hei og velkommen! I dag setter vi opp cplt-sandbox fra null og får en første kjørbar flyt på plass."
2. Klon eller åpne sandboxen og vis startpunktet.
3. Kjør init/sett opp nødvendige filer.
4. Verifiser med en kort testkjøring.
5. Avslutt med hva som må til for å dele eller publisere videre.

**Innhold (3–5 min):**
1. Vis startpunktet og hva som mangler.
2. Sett opp sandboxen steg for steg.
3. Verifiser at initiell flyt virker.
4. Vis hva som er klart til neste steg.

**Demo-kontekst (referanserepo):** `cplt`-sandbox og init-flyten.

**Ta med deg videre:** En liten, repeterbar bootstrap-flyt er nok for å komme i gang raskt.

**Oppsummering:** Sandboxen skal gjøre første steg enkelt, ikke perfekt.

**Outro:** Neste gang starter du fra samme oppskrift og bygger videre derfra.

**Prompt-manus (copy/paste):**

```text
Lag en kort sjekkliste for å komme i gang med cplt-sandbox.
Krav: 5 steg, maks 1 linje per steg.
Output: nummerert liste.
```

**Forventet respons-signal:** En enkel, kjørbar oppstartssekvens med tydelig første steg.

---

## Bonus episode E: Fire grep som faktisk senker regningen

**Status:** Planlagt

**Overlay:** E · orange/amber · lever-meter · modell / resonneringsnivå / cache / mindre arbeid · kr per oppgave

**Mål:** Vise de fire grepene som har dokumentert effekt på tokenkostnad — og lære seeren å måle effekt på fakturert kostnad per fullført oppgave i stedet for på et verktøys eget sparetall.

**Kan sees alene fordi:** Vi starter med hvorfor «spart tokens» og «lavere regning» ikke er samme sak, og går rett på fire grep man kan gjøre samme dag.

**Bakgrunn (til deg som spiller inn):** Denne episoden erstatter et tidligere utkast som handlet om `rtk` og CLI-output-komprimering med en påstand om 60–90 % besparelse. Den påstanden er verktøyets egen selvrapportering. Den eneste store offentlige kontrollerte studien ([JetBrains, 425 kjøringer over 86 oppgaver](https://blog.jetbrains.com/ai/2026/07/rtk-claude-code-token-savings/)) fant 7,6 % *høyere* kostnad per oppgave ved lavt resonneringsnivå (p = 0,004) og ingen forskjell ved høyt nivå (+0,1 %, p = 0,99), uten kvalitetsforskjell. Ikke bruk tall fra verktøydashbord i videoen.

**Oppsett før demo:** Ingen installasjon. Du trenger tilgang til modellvalg, resonneringsnivå og en kostnadsvisning (Spend Meter i VS Code eller statistikk i my-copilot).

**Script-outline (one-take):**
1. "Hei og velkommen! I dag ser vi på hva som faktisk senker AI-regningen — og hvorfor det vanligste rådet ikke gjør det."
2. Vis forskjellen på «tokens spart» og «kroner fakturert»: output-tokens er en liten del av en sesjon, og det meste av input er cachede gjenlesninger til rundt en tidel av prisen.
3. Grep 1 — modellvalg: samme oppgave på to modeller, vis pris per token og resultat.
4. Grep 2 — resonneringsnivå: kjør en rutineoppgave på lavt nivå og vis at kvaliteten holder.
5. Grep 3 — hold prompt-cachen intakt: vis hva som bryter cachen, og hva det koster.
6. Grep 4 — gjør mindre arbeid: målrettet test i stedet for full pipeline, én oppgave per sesjon.
7. Avslutt med målereglen: sammenlign fakturert kostnad per fullført oppgave, aldri et verktøys eget sparetall.

**Innhold (3–5 min):**
1. **Rammen (40 sek):** Hvorfor «spart output» ≈ 0 kroner. Output-tokens er en liten andel av en agent-sesjon, cache-treffraten er høy, og de fleste tellere priser fjernede tokens til full input-pris.
2. **Grep 1 — modellvalg (60 sek):** Den største spaken, fordi den endrer pris per token og ikke bare antall tokens. Kjør samme oppgave på en rask og en tung modell, og vis at rutineoppgaven ikke ble bedre av den dyre modellen.
3. **Grep 2 — resonneringsnivå (45 sek):** Leverandørens egen innstilling for resonneringsnivå er et reelt kostnadsvalg. Vis en rutineoppgave på lavt nivå med samme resultat, og si tydelig når du *bør* skru opp igjen.
4. **Grep 3 — prompt-cache (45 sek):** Vis hva som bryter cachen (endre systemprompt/instruksjonsfiler midt i sesjonen, injisere ny kontekst tidlig i prompten) og hva som bevarer den. Poeng: et tiltak som «sparer tokens» men bryter cachen kan gi *høyere* regning.
5. **Grep 4 — mindre arbeid (45 sek):** `./gradlew test --tests *MinTest` i stedet for full pipeline, én oppgave per sesjon, la agenten lese selv i stedet for å lime inn. Færre runder slår kortere tekst.
6. **Måleregel (30 sek):** Kjør samme oppgavesett før og etter, og sammenlign fakturert kostnad per *fullført* oppgave. Verktøyets eget sparetall teller ikke som bevis.

**Demo-kontekst (referanserepo):** Bruk dette monorepoet. Velg én reell oppgave (f.eks. en liten endring i `apps/copilot-metrics`) og kjør den to ganger med ulikt modell-/resonneringsvalg.

**📄 Referanse:** Se referanseseksjonen nederst i denne fila for kilder, tall og opptaks-sjekkliste.

**Reell oppgave i repo (velg én før opptak):**
- Kjør samme lille endring to ganger med ulik modell og ulikt resonneringsnivå, og noter fakturert kostnad per fullført oppgave for begge.

**Ta med deg videre:** Velg riktig modell og riktig resonneringsnivå, ikke komprimer tekst. Mål i kroner per fullført oppgave.

**Oppsummering:** De fire spakene med dokumentert effekt er modellvalg, resonneringsnivå, intakt prompt-cache og mindre arbeid.

**Outro:** Neste gang du vurderer et token-sparende verktøy: be om fakturert kostnad per fullført oppgave, ikke et skjermbilde av verktøyets eget dashbord.

**Prompt-manus (copy/paste):**

```text
Gjør denne endringen. Bruk en rask modell og lavt resonneringsnivå.
Oppgave: legg til en manglende feilmelding i <fil>.
Output: bare diffen.
```

```text
Samme oppgave, men bruk høyt resonneringsnivå.
Sammenlign resultatet med forrige kjøring.
```

```text
Kjør bare testene som dekker <fil>, ikke hele pipelinen.
Rapporter bare feilende tester.
```

**Demo-senario for videoen:**

Sekvens 1 (0:00–0:50): Rammen
- "Alle vil spare tokens. Men det er kroner du blir fakturert for."
- Vis fordelingen: input dominerer, det meste er cachet, output er en liten andel.
- "Derfor er det å komprimere tekst en liten spak — og noen ganger negativ, hvis den bryter cachen."

Sekvens 2 (0:50–1:50): Modellvalg
- Samme oppgave, to modeller, side ved side
- Vis pris per token for begge og resultatet av begge
- Overlay: "Største spaken: pris per token"

Sekvens 3 (1:50–2:35): Resonneringsnivå
- Rutineoppgave på lavt nivå, samme resultat
- Nevn når du skal skru opp igjen (arkitektur, feilsøking i ukjent kode)

Sekvens 4 (2:35–3:20): Prompt-cache
- Vis en sesjon der cachen holder, og en der den brytes
- Overlay: "Cachet input ≈ 1/10 pris"

Sekvens 5 (3:20–4:00): Mindre arbeid
- Målrettet test vs full pipeline
- Én oppgave per sesjon

Sekvens 6 (4:00–4:20): Måleregel og sjekkliste
- "Her er hva du gjør neste gang:"
  - Velg modell etter oppgave, ikke etter vane
  - Sett resonneringsnivå bevisst
  - Ikke bryt cachen midt i en sesjon
  - Mål i kroner per fullført oppgave

**Forventet respons-signal:** Seeren kan navngi de fire spakene, og vet at et verktøys eget sparetall ikke er bevis for lavere regning.

**Kilder:**
- JetBrains: [Measuring rtk's token savings in Claude Code](https://blog.jetbrains.com/ai/2026/07/rtk-claude-code-token-savings/) — 425 kjøringer, 86 oppgaver, forhåndsregistrerte endepunkter
- [jayn caveman-replay](https://jayn.app/caveman) ([data](https://github.com/jaynapp/jayn-caveman)) — 0,6 % av en faktisk regning på 755 dollar
- Brandon Barker: [Fewer tokens, bigger bill](https://brandonbarker.me/writing/headroom-fewer-tokens-bigger-bill) — 39 % færre tokens, men cachen brutt 123 ganger
- [Token savings: wrong number](https://zernie.com/blog/token-savings-wrong-number/) — ca. −6 % målt mot −65 % annonsert
- Nav: [Slik holder du token-forbruket nede](news/articles/token-forbruk-verktoy-teknikker.md)

**Overlay metadata (OverlayComponent format):**

```ts
{
  id: "bonus-e-kostnadsspaker",
  title: "Fire grep som faktisk senker regningen",
  accent: "#ff8c42", // orange
  secondaryAccent: "#ffc857", // amber
  motif: "lever-meter", // fire spaker med ulik lengde
  poster: "bonus-e-kostnadsspaker-poster.png",
  components: [
    {
      kind: "episode-number",
      anchor: "top-left",
      labels: ["E"]
    },
    {
      kind: "ladder",
      anchor: "center-left",
      labels: ["Modellvalg", "Resonneringsnivå", "Prompt-cache", "Mindre arbeid"],
      highlightIndex: 0
    },
    {
      kind: "rule-pill",
      anchor: "bottom-full",
      labels: ["Mål i kroner per fullført oppgave"]
    },
    {
      kind: "chip",
      anchor: "top-right",
      labels: ["ikke verktøyets eget sparetall"],
      monospace: true
    }
  ]
}
```

**Frontend rendering hints:**
- Lever-meter motif: fire vannrette spaker med synkende lengde, øverste tydelig lengst
- Ladder: vertikal liste med de fire spakene, øverste uthevet
- Rule-pill: én bred pille nederst med målereglen
- Chip: liten monospace-tekst øverst til høyre som advarsel mot selvrapporterte tall

---



- **Start (20 sek):** Hva du lærer og hvorfor det sparer kost.
- **Demo (2–3 min):** Ett konkret repo-scenario.
- **Sammenligning (45 sek):** "dyr måte" vs "billig måte".
- **Avslutning (30 sek):** 1 regel + 1 handling til neste gang.

## Fast vignettintro per episode (10–15 sek)

Bruk samme velkomst i alle episoder:

> Hei, og velkommen til Copilot-tipsene. På noen få minutter ser vi på ett konkret grep som gir bedre svar, ryddigere context og lavere tokenkost. La oss sette i gang.

Kort variant:

> Hei, og velkommen tilbake. I dag tar vi ett konkret grep for bedre Copilot-bruk og lavere kost.

Mal med plassholdere:

> Hei, og velkommen til Copilot-tipsene. I dag ser vi på {tema} – {nytteverdi} på under {varighet} minutter.

## Fast oppsummering per episode (10–15 sek)

> Kort oppsummert: {hva vi lærte}. Det viktige er {én regel eller ett grep}. Bruk dette neste gang du jobber i Copilot.

## Fast outro per episode (10–15 sek)

> Det var dagens tips. Prøv dette i neste økt, og ta med deg regelen: {én kort regel}. Vi sees i neste episode.

## Produksjonsnotater

- Format: én kontinuerlig opptakstakning per episode (ingen klipp internt).
- Format: vertikal video (9:16) med terminal som hovedflate og webkamera i hjørnet.
- Hold webkamera lite og stabilt (ca. 15–20 % av bildeflaten), ikke dekk terminalutskrift.
- Bruk stor terminalfont og høy kontrast slik at tekst er lesbar på mobil.
- Bruk samme oppgave i før/etter-demo der det er mulig.
- Hold skjermbildet fokusert: én terminal + én filvisning om gangen.
- Vis alltid hvilke filer i monorepoet som berøres.
- Unngå lange forklaringer; vis handling og resultat.
- Ha en fast åpningsreplikk per episode som setter kontekst raskt i one-take.
- Legg inn 1–2 sek stille pause mellom segmenter i samme opptak for enklere undertekster.

## Videobeskrivelser for detaljsider

### Episode 1: Presis prompt på første forsøk

Hver gang du skriver en dårlig prompt, må du stille oppfølgingsspørsmål for å fikse den. Det dobler eller tredobler tokenbruken for det samme resultatet. I denne videoen lærer du tre enkle triks som gjør prompts dine presise fra start: legg til kontekst om kodebasen, spesifiser eksakt hva du vil ha, og gi ett eksempel på riktig format.

Disse tre elementene tar 30 sekunder ekstra å skrive, men sparer deg for 5-10 runder med oppklaringer. Det betyr færre tokens, raskere svar, og færre misforståelser. Etter denne videoen skal du kunne skrive en prompt som fungerer første gang—og du sjekker selv at den er klar før du sender den.

### Episode 2: En oppgave per tråd

Hvis du hopper mellom mange ulike oppgaver i samme Copilot-sessjon, blir konteksten rotete. LLM-en må huske alle detaljer fra oppgave A mens du spør om oppgave B—og det varer lenge og koster mye. I denne videoen viser jeg to enkle knapper som fikser det: `/clear` starter en ny, ren sessjon, og `/compact` klemmer sammen gamle meldinger når en session blir lang.

Resultatet: Bedre fokus, raskere svar, og 30-40% færre tokens per oppgave. Du lærer når du skal bruke hver av dem, og hvordan du merker at konteksten begynner å bli for tung. Prøv `/clear` neste gang du bytter fra en task til en helt annen.

### Episode 3: Riktig modus og agentnivå

Du har flere måter å bruke Copilot på: rask spørsmål-svar, planlegging, eller delegering til spesialister. De koster veldig forskjellig—men de fleste velger alltid det dyreste alternativet. I denne videoen lærer du en enkel regel: bruk Ask for småspørsmål, Plan for arkitekturbeslutninger, og spesialistagenter for dypt arbeid. Da betaler du bare for det du trenger.

Når du matcher riktig tool til oppgaven, sparer du 80% tokens på rutinejobber—uten å miste noen kvalitet. Vi viser konkrete eksempler: et designspørsmål (Ask), en API-arkitektur (Plan), og en sikkerhetskontroll (spesialist). Etter denne videoen sjekker du automatisk hvilket modusvalg som passer før du starter.

### Episode 4: Tool-first workflow

Når du spør en LLM "hva inneholder denne filen?", bruker du 200+ tokens for noe som git/grep gjør på 2 millisekund. Det er som å bruke en luftkisser for å hamre inn en spiker. I denne videoen lærer du å tenke verktøy først: bruk grep, git, gh CLI og andre deterministiske verktøy for å hente fakta, og bruk LLM-en bare for å resonnere over dem.

Resultatet: 10x raskere svar, 90% færre tokens, og færre feil—fordi du gir LLM-en faktisk korrekt data i stedet for å be den gjette. Vi viser eksempler med git-historikk, filsøk, og GitHub-API-kall. Test denne regelen på din neste "søkoppgave" og merk hvor mye raskere du blir.

### Episode 5: Kort output uten kvalitetstap

LLM-er gir deg lange, detaljerte svar som regel. Det er fint når du trenger full forklaring, men når du bare vil ha koden eller punktene, er halvparten av outputen bortkastet. I denne videoen introduserer vi `/terse`-modus: den samme svarene, men uten fyllord, unødvendige forklaringer og repetisjon. Du får kortere output med samme kjerneinnhold. Hvor mye det utgjør varierer med oppgaven, så mål det på dine egne oppgaver framfor å stole på et fasttall.

/terse er perfekt når du allerede skjønner domenet og bare vil ha resultatet raskt. Vi viser før/etter-eksempler med kodegenering, planlegging og debugging. Etter denne videoen bruker du `/terse` som standard, og velger full-mode bare når du trenger læring, ikke speed.

### Episode 6: Kosteffektiv PR-flyt

Når du reviewer mange PRs, kjører du gjerne kodegjennomgang som separate Copilot-sesjoner. Det betyr hver review starter fra topp—full kontekst-overhead for hver PR. I denne videoen lærer du å samle reviews: batch 5-6 PRs i én sessjon, bruk `/diff` for å laste inn filendringer uten å paste dem manuelt, og la Copilot sammenligne på tvers av filene.

Resultatet: 70% færre tokens per review, raskere feedback, og færre misforståelser mellom reviewerne. Vi viser konkret workflow: last diff, angi fokusregler ("les for sikkerhet"), få kommentarer på alle PR-er samtidig. Når du har 10+ PRs i backlog, er denne teknikken en game-changer.

### Bonus episode A: Tre dyre anti-mønstre

Det finnes tre skjulte måter utviklere bruker 200-300% ekstra tokens uten å vite det. De er så vanlige at du sannsynligvis gjør minst en hver dag. I denne videoen avslører vi dem: gjenta samme kontekst mange ganger, bruk LLM for søk i stedet for grep, og fortsett lange sesjoner når du burde startet på nytt.

Hver anti-mønster har en enlinjenregel som fikser det. Etter denne videoen kjenner du igjen anti-mønstre før de sliter ressursene dine, og du vet nøyaktig hva du skal gjøre i stedet. Dette er gjenkjenning som blir til forebyggelse.

### Bonus episode B: Mål effekt i statistikk

Du merker at du sparer tokens når du bruker disse trikksene, men hvor mye egentlig? I stedet for å gjette, kan du måle det. I denne videoen lærer du å spore Copilot-bruk fra statistikk-dashbordet, sammenligne før/etter workflow-endringer, og finne hvilken teknikk som gir best return on time.

Data slår "følelse". Vi viser hvordan du setter opp måling, hva du skal se etter (tokens per oppgave, gjennomsnittlig sessionslengde, modellbruk), og hvordan du deler resultater med teamet. Etter denne videoen baserer du optimiseringer på data, ikke antagelser.

### Bonus episode C: Chronicle — forstå og optimaliser context

Context er skjult inne i LLM-en—du kan ikke se hva som blir sendt, og derfor er det vanskelig å optimalisere. Chronicle-verktøyet viser eksakt hvilke filer, meldinger og avhengigheter som er lastet inn, hvor mange tokens hver del bruker, og hvor bottleneckene er. Det er som røntgen for Copilot-sesjoner dine.

Med Chronicle kan du måle hvor mye hver fil/agent/instruksjon koster, og målrette optimiseringen der det faktisk gjør nytte. Vi viser eksempler med monorepo-kontekst, agent-delegering og instruksjonsdrift. Etter denne videoen bruker du Chronicle før store sesjonssett for å finne hidden waste.

### Bonus episode D: Cplt sandbox — kom i gang på 3 minutter

Å sette opp Copilot lokalt kan virke komplisert: installation, konfiguration, testing. I denne videoen viser vi Cplt sandbox—et pre-konfigurert testmiljø som gir deg en arbeidende Copilot-setup på 3 minutter, med alt fra integrasjoner til eksempler allerede på plass.

Bruk sandboxen til å eksperimentere med nye workflows, teste optimaliseringsteknikker, eller lære agentdelegeringen uten å påvirke din normale setup. Det er som en sikker øvingsgrunn for Copilot-bruken din. Etter denne videoen kan du starte eksperimenter med det samme.

### Bonus episode E: Fire grep som faktisk senker regningen

De fleste råd om tokensparing handler om å komprimere tekst. Problemet er at tekst er den minste posten på regningen: i en agent-sesjon er input dominerende, og det meste av input er cachede gjenlesninger som faktureres til en brøkdel av full pris. Verktøy som komprimerer output kan derfor vise imponerende tall i sitt eget dashbord uten at regningen flytter seg — og et tiltak som bryter prompt-cachen kan gjøre den dyrere.

I denne videoen ser vi på de fire grepene som faktisk har målbar effekt: modellvalg (den største spaken, fordi den endrer pris per token), bevisst resonneringsnivå, å holde prompt-cachen intakt, og å gjøre mindre arbeid i stedet for å skrive kortere. Vi kjører den samme oppgaven i dette repoet med ulike valg og sammenligner.

Til slutt får du målereglen som gjør deg immun mot markedsføring: sammenlign fakturert kostnad per *fullført* oppgave før og etter, aldri et verktøys eget sparetall. Etter denne videoen vet du hvilke fire knapper du skal skru på, og hvordan du sjekker at det virket.

### Kostnadsspaker — kilder og opptaksnotater

#### Hvorfor output-komprimering er en liten spak

Output-tokens utgjør en liten andel av tokenbruken i en agent-sesjon, og cache-treffraten på input er høy. Tellere i komprimeringsverktøy overvurderer typisk effekten på tre måter: de regner hele den rå verktøyoutputen som «spart» selv om agenten uansett kutter store resultater, de priser fjernede tokens til full input-pris i stedet for cache-pris, og de fanger bare den delen av verktøyresultatene som faktisk går gjennom hooken.

#### Kilder du kan vise på skjerm

- JetBrains: [Measuring rtk's token savings in Claude Code](https://blog.jetbrains.com/ai/2026/07/rtk-claude-code-token-savings/) — 425 kjøringer, 86 oppgaver, forhåndsregistrerte endepunkter. Resultat: +7,6 % kostnad per oppgave ved lavt resonneringsnivå (p = 0,004), +0,1 % (p = 0,99) ved høyt, ingen kvalitetsforskjell.
- [jayn caveman-replay](https://jayn.app/caveman), [data](https://github.com/jaynapp/jayn-caveman) — 0,6 % spart av en faktisk regning på 755 dollar.
- Brandon Barker: [Fewer tokens, bigger bill](https://brandonbarker.me/writing/headroom-fewer-tokens-bigger-bill) — 39 % færre tokens, men prompt-cachen brutt 123 ganger.
- [Token savings: wrong number](https://zernie.com/blog/token-savings-wrong-number/) — ca. −6 % målt mot −65 % annonsert.

#### Demo 1: modellvalg

Kjør samme lille oppgave to ganger med ulik modell. Vis pris per token for begge før du starter, og vis at resultatet på en rutineoppgave er likeverdig. Dette er den sterkeste demoen fordi den endrer prisen, ikke bare mengden.

#### Demo 2: resonneringsnivå

Kjør en rutineoppgave på lavt resonneringsnivå og vis at kvaliteten holder. Si tydelig når du bør skru opp igjen: arkitekturvalg, feilsøking i ukjent kode, sikkerhetsvurderinger.

#### Demo 3: prompt-cache

Vis to sesjoner: én der instruksjoner og systemprompt ligger i ro, og én der du endrer kontekst tidlig i prompten midt i sesjonen. Poenget er at cachet input koster rundt en tidel, og at et «sparetiltak» som bryter cachen kan gi høyere regning.

#### Demo 4: mindre arbeid

Kjør en målrettet test i stedet for full pipeline, og la agenten lese filer selv i stedet for at du limer inn. Færre runder slår kortere tekst.

#### Måleregel

Kjør et fast sett oppgaver før og etter endringen, og sammenlign fakturert kostnad per fullført oppgave. Ta med kvalitet: en «besparelse» som gir flere runder er ingen besparelse.

#### Opptaksregel

- Ikke vis tall fra et verktøys eget sparedashbord som bevis
- Bruk fakturerte tall (Spend Meter i VS Code eller statistikk i my-copilot)
- Velg en oppgave som er liten nok til å kjøres flere ganger på opptak
- Vær tydelig på hva som *ikke* er testet: én studie, ett verktøy, to resonneringsnivåer

#### Nevn gjerne, men uten anbefaling

`rtk` finnes fortsatt og kan installeres med `brew install rtk`. Den gjør terminaloutput lettere å lese. Hvis du nevner den i videoen, si samtidig at den offentlige kontrollerte målingen ikke gjenskapte den annonserte besparelsen — og ikke bruk `rtk gain` som bevis for noe.
