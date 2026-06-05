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

## Publiseringsplan (6+3)

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
3. Bonus episode C: Chronicle — innsikt på tvers av agentsesjoner

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

## Episode 1: Presis prompt på første forsøk

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
Mål: Oppdater seksjon om kostoptimalisering med informasjon om hvordan AGENTS.md / copilot-instructions.md påvirker tokenkostnad og konkrete anbefalinger for å redusere kost i praksis med å holde innholde i disse filene minimalt og relevant.
Fil: apps/my-copilot/src/app/praksis/sections/cost-optimizations.tsx
Begrensning: Ikke endre andre filer. Bruk eksisterende Nav DS-komponenter.
Output: Vis kun patch + 2 linjer forklaring.
```

**Forventet respons-signal:** Færre avklaringsrunder, én fil endres, kort svar.

**Overgang til neste episode:** Vi tar opp igjen samme arbeidsflyt med `/resume`, ser på endringen vi nettopp gjorde i kostoptimalisering, og starter deretter en ny tråd for neste mål.

---

## Episode 2: En oppgave per tråd (`/clear` og `/compact`)

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
copilot chronicle search "adoption summary"
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

---

## Bonus episode A: Tre dyre anti-mønstre

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

## Bonus episode C: Chronicle — innsikt på tvers av agentsesjoner

**Mål:** Vise hvordan `chronicle` gir deg søkbar sesjonshistorikk, standup-oppsummeringer og personlige tips, og hvordan du bruker `/context` og `/instructions` for å forstå og trimme tung kontekst.

**Kan sees alene fordi:** Vi forklarer hva Chronicle er og viser én konkret flyt fra blank start.

**Script-outline (one-take):**
1. "Hei og velkommen! I dag viser jeg hvordan Chronicle gjør agentsesjoner søkbare, og hvordan `/context` og `/instructions` hjelper deg å holde konteksten slank."
2. Vis `copilot chronicle` og de tilgjengelige subcommandene.
3. Kjør `copilot chronicle standup` for å hente en kort oppsummering av siste døgn.
4. Vis `/context` og forklar at system prompt, system tools, MCP tools og meldinger alle spiser av samme budsjett.
5. Kjør `copilot chronicle search` og `copilot chronicle tips` eller `cost-tips` for å finne tidligere arbeid og forbedringsforslag.
6. Avslutt med når du bruker Chronicle i stedet for å lete manuelt i gamle sesjoner, og når du bruker `/instructions` for å trimme tunge repo-instrukser.

**Innhold (3–5 min):**
1. Finn et tidligere agentspor med `copilot chronicle search`.
2. Oppsummer siste dag med `copilot chronicle standup`.
3. Vis `copilot /context` og pek på hvordan system prompt og tool-definisjoner tar plass.
4. Vis `copilot /instructions` for å se hvilke instrukser som lastes inn, og fjern eller juster tunge eller irrelevante filer.
5. Vis personlige tips med `copilot chronicle tips` og kostforslag med `cost-tips`.
6. Vis `copilot chronicle improve` for forslag til forbedringer i `copilot-instructions.md`.
7. Vis `copilot chronicle reindex` når indeksen må bygges opp på nytt.

**Demo-kontekst (referanserepo):** Bruk en nylig Copilot CLI-sesjon i dette repoet og gjenfinn en konkret beslutning eller forbedring derfra.

**Ta med deg videre:** Chronicle er best når du trenger historikk, standup eller tips, mens `/context` og `/instructions` hjelper deg å holde resten av budsjettet under kontroll.

**Kilder:**
- GitHub Docs: [Chronicle — pick a subcommand](https://docs.github.com/en/copilot/concepts/agents/copilot-cli/chronicle)

## Felles mal for hver video

- **Start (20 sek):** Hva du lærer og hvorfor det sparer kost.
- **Demo (2–3 min):** Ett konkret repo-scenario.
- **Sammenligning (45 sek):** "dyr måte" vs "billig måte".
- **Avslutning (30 sek):** 1 regel + 1 handling til neste gang.

## Fast "stå alene"-intro per episode (15–20 sek)

Bruk samme introformat i alle episoder:

1. Problem: "Dette koster unødvendig tid/tokens."
2. Mål: "På 3–5 minutter lærer du én konkret teknikk."
3. Effekt: "Etterpå kan du bruke dette direkte i egen repo."

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
