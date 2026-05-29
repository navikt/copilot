---
title: "Slik holder du token-forbruket nede"
date: 2026-05-28
author: starefossen
category: praksis
excerpt: "Praktiske tips for å få raskere og billigere Copilot-svar — fra innebygd $terse-mode til smarte vaner som sparer tokens."
tags:
  - token-optimization
  - cost-optimization
  - tools
  - agents
  - nav-pilot
---

Med [bruksbasert fakturering](/nyheter/model-pinning-kostnadsoptimalisering) betaler du per token. Lange sesjoner med mye frem og tilbake koster mer. Den gode nyheten: du kan få kortere, mer presise svar uten å miste kvalitet.

En [2026-studie](https://arxiv.org/abs/2604.00025) fant at modeller ble mer treffsikre når de ble tvunget til å svare kort. Kortere svar er altså ikke bare billigere — de kan også være bedre.

## Det viktigste først: bruk `nav-pilot` CLI

`nav-pilot` er kommandolinjeverktøyet som holder Copilot-oppsettet ditt oppdatert og starter Copilot med riktig konfigurasjon. Den enkleste måten å starte en AI-sesjon:

```bash
nav-pilot --sync    # synkroniserer, deretter starter Copilot med @nav-pilot-agenten
```

Eventuelt steg for steg:

```bash
nav-pilot install kotlin-backend   # installer collection for teamet ditt
nav-pilot sync                     # sjekk for oppdateringer
nav-pilot                          # interaktiv meny — install, sync, eller start Copilot
```

Når du starter Copilot via `nav-pilot`, får du automatisk:

- Konklusjonen først, ikke forklaringen
- Kode direkte uten lang innledning
- «Si 'forklar' for detaljer» når begrunnelse hoppes over
- Spørsmål bare når svaret faktisk endrer implementeringen
- Nav-konvensjoner (Aksel, Nais, auth) uten at du trenger å be om det

I VS Code Chat kan du skrive `@nav-pilot` for å få det samme.

## Vil du ha enda kortere svar? Bruk `terse-mode`

Skills er tilleggskunnskap nav-pilot kan bruke. Du aktiverer dem ved å skrive navnet i meldingen — enten med `$`-prefiks (`$terse-mode`) eller uten (`bruk terse-mode`). `$` er vår visuelle konvensjon, ikke påkrevd syntax. Les mer om [skills i VS Code-dokumentasjonen](https://code.visualstudio.com/docs/copilot/customization/agent-skills).

Skriv `terse-mode` for å skru på ekstra kompakt stil. Tre nivåer:

| Nivå | Hva den gjør | Eksempel |
| ---- | ------------ | -------- |
| **lett** | Fjerner fyllord, beholder fulle setninger | «Komponenten re-rendrer fordi du lager ny objektreferanse. Bruk `useMemo`.» |
| **normal** | Fragmenter og korte synonymer | «Ny objekt-ref hver render → re-render. `useMemo`.» |
| **ultra** | Telegrafisk, forkortelser | «Inline obj-prop → ny ref → re-render. `useMemo`.» |

Slik bruker du det:

```text
terse-mode                ← slår på normal-nivå
terse-mode ultra          ← for raske iterasjoner
Stopp terse               ← tilbake til vanlig stil
```

Stilen vedvarer hele sesjonen. Ved sikkerhetsvarsler eller destruktive handlinger bytter den automatisk tilbake til full prosa — du mister aldri viktig informasjon.

## Fem vaner som kutter kostnader

### 1. Vær presis i spørsmålet

Den dyreste token-sløsingen er misforståelser. Fem runder med «mente du A eller B?» koster mer enn ett godt spørsmål.

**Dyrt** (vagt → mange oppfølgingsspørsmål):
> «Lag en tjeneste for søknader»

**Billig** (presist → agenten kan starte med én gang):
> «Lag et Ktor REST-endepunkt som tar imot dagpengesøknader over Kafka fra dp-soknad. Kotlin, Nais, Postgres.»

Jo mer kontekst du gir i første melding, jo færre runder bruker du.

### 2. For store oppgaver: la intervjuet gjøre jobben

For små oppgaver gjør nav-pilot jobben direkte — ingen spørsmål. For medium/store oppgaver går den gjennom en kort intervjufase der den sjekker blindsoner som personvern, auth og avhengigheter.

Hvis du vil ha et enda grundigere intervju, be om det med `nav-deep-interview`. Den kjører en strukturert gjennomgang med impactanalyse.

Fem minutter med avklaring slår en bortkastet sesjon.

### 3. Hold sesjoner fokuserte

Copilot bruker [prompt caching](/nyheter/model-pinning-kostnadsoptimalisering) — kontekst fra tidligere i sesjonen koster opptil 90 % mindre enn ny kontekst. Det betyr at du *ikke* trenger å starte ny sesjon bare for å spare penger. Men en fokusert sesjon gir bedre svar fordi modellen slipper å filtrere bort irrelevant historikk.

- Én oppgave per sesjon gir mer presise svar
- Unngå «kan du også...» som tar sesjonen i helt ny retning
- Lang, ufokusert historikk forvirrer — ikke bare koster

### 4. La nav-pilot finne verktøyene

Du trenger ikke huske alle skills. Nav-pilot bruker riktig kunnskap basert på konteksten:

- Skriver du Kotlin eller TypeScript? → Sikkerhetsregler (OWASP) er allerede aktive
- Jobber du med Nais-manifest? → Nais-kunnskap aktiveres
- Trenger du auth? → TokenX/Azure AD-kunnskap brukes

Du kan be om en spesifikk skill med navn (f.eks. `bruk terse-mode`), men for de fleste oppgaver klarer nav-pilot seg selv.

### 5. Hjelp agenten med å lese mindre

I terminalen er den største token-lekkasjen ofte verktøyoutput — testlogger, stacktraces, store diffs. Noen vaner som hjelper:

- La agenten lese filer selv i stedet for å lime inn hele filer
- Ved testfeil: gi den relevante feilmeldingen, ikke hele build-loggen
- Be agenten kjøre målrettede tester (`./gradlew test --tests *MinTest`) før full pipeline
- Pek på branch eller fil for diff i stedet for å lime inn manuelt

## Gode første meldinger

Eksempler som gir presise svar uten mange oppfølgingsrunder:

```text
Legg til et nytt REST-endepunkt /vedtak/{id} i Ktor-appen.
Valider tilgang med TokenX. Hent vedtak fra Postgres via kotliquery.
Les eksisterende endepunkter i src/main/kotlin/routes/ for stil.

Finn ut hvorfor /soknader-siden i Next.js-appen re-rendrer mye.
Sjekk komponenten i src/app/soknader/page.tsx. Foreslå minimal fix.

Podden dp-soknad krasjer i dev med OOMKilled. Se på .nais/dev.yaml
og foreslå justeringer. Ikke endre prod-konfig.

Migrer denne Java-klassen til idiomatic Kotlin. Behold samme
offentlige API. Bruk sealed class for feilhåndtering.
```

## Oppsummert

| Tips | For hvem | Hva du gjør |
| ---- | -------- | ----------- |
| Start via `nav-pilot` | Alle | `nav-pilot --sync` synkroniserer og starter Copilot med riktig oppsett |
| Vær presis | Alle | Nevn språk, rammeverk og integrasjoner i første melding |
| Fokuserte sesjoner | Alle | Én oppgave per sesjon — start ny når du bytter problem |
| La agenten lese | Alle i terminalen | Ikke lim inn store filer/logger — la agenten lese selv |
| `terse-mode` | Deg som vil ha kortere svar | Skriv `terse-mode` i starten av sesjonen |
| `nav-deep-interview` | Nye tjenester, stor refaktor | Skriv `nav-deep-interview` for grundigere avklaring |

---

## Under panseret

Resten er for deg som vedlikeholder instruksjoner, bygger MCP-servere, eller vil forstå *hvorfor* tipsene over fungerer.

### Hvorfor kortere kontekst gir bedre svar

Hver gang du sender en melding, pakker Copilot med alt: systemprompt, instruksjonsfiler, åpne filer, verktøydefinisjoner og hele samtalehistorikken. En flerstegssesjon i Copilot CLI kan bruke over 1 million input-tokens. [Vantage sin analyse](https://www.vantage.sh/blog/agentic-coding-costs) viser at kostnader varierer opptil 30x mellom kjøringer på samme oppgave.

De to største besparelsene:
1. **Kortere sesjoner** (færre runder = mindre historikk å sende)
2. **Mindre instruksjonslast** (smalere `applyTo`-mønstre = færre filer i konteksten)

### Hva vi gjorde i dette repoet

Vi hadde en OWASP-sikkerhetsinstruks på 21 KB som ble lastet ved *hver eneste* Go- og Kotlin-redigering — også når oppgaven ikke handlet om sikkerhet. Vi flyttet innholdet til en on-demand skill og beholdt bare en kort stub (1 KB) med de mest kritiske reglene.

| Måling | Før | Etter |
| ------ | --- | ----- |
| OWASP per redigering | 21 KB | ~1 KB |
| Go-kontekst totalt | 42 KB | 22 KB |
| Kotlin-kontekst totalt | 54 KB | 34 KB |

**Tommelfingerregel:** Hvis en instruks bare er relevant for 10 % av oppgavene, hører den hjemme i en skill — ikke i en alltid-aktiv fil.

### Instruksjonsarkitektur

Det viktigste feltet i en Copilot-instruksjon er `applyTo`-globen:

- `applyTo: "**"` → lastes alltid (bruk sparsomt)
- `applyTo: "**/*.go"` → lastes bare for Go-filer
- `applyTo: "**/db/migration/**/*.sql"` → lastes bare ved databasemigrering

Andre tips for vedlikeholdere:
- Komprimer innhold — instruksjoner leses av modellen, ikke mennesker
- Hold filer stabile — prompt caching fungerer best når innholdet sjelden endres
- Bruk konkrete dropp-lister («dropp artikler og høflighetsfraser») fremfor vagt «vær kort»

### RTK — komprimerer terminaloutput

[RTK](https://github.com/rtk-ai/rtk) komprimerer kommandoutput (testresultater, diff, kubectl) før den når kontekstvinduet. Nyttig hvis du bruker Copilot CLI mye.

```bash
brew install rtk
rtk init -g --copilot
```

RTK rapporterer 60–90 % reduksjon på verktøydata. RTK og terse-mode utfyller hverandre: RTK kutter det agenten *leser*, terse-mode kutter det agenten *skriver*.

**Merk:** RTK prosesserer terminaloutput lokalt. Sjekk at verktøyet er godkjent for ditt team før du bruker det med output som kan inneholde sensitive data.

### TOON og dynamiske verktøysett

To teknikker for deg som bygger MCP-servere:

- [TOON](https://toonformat.dev/) koder strukturert data med 30–60 % færre tokens. Relevant når serveren returnerer store JSON-objekter.
- **Dynamiske verktøysett** — ikke last alle verktøyskjemaer på forhånd. Bruk et søk → beskriv → utfør-mønster. [Speakeasy rapporterer 100x reduksjon](https://www.speakeasy.com/blog/how-we-reduced-token-usage-by-100x-dynamic-toolsets-v2) med denne tilnærmingen.

## Videre lesing

- [GitHub: Improving Token Efficiency in Agentic Workflows](https://github.blog/ai-and-ml/github-copilot/improving-token-efficiency-in-github-agentic-workflows/)
- [arXiv: Brief is better?](https://arxiv.org/abs/2604.00025) — korte svar kan gi bedre nøyaktighet
- [Matt Pocock: skills](https://github.com/mattpocock/skills) — grill-mønster og andre agent-skills
- [Vantage: Hidden Cost Driver in Agentic Coding](https://www.vantage.sh/blog/agentic-coding-costs) — kostnadsanalyse av agentiske arbeidsflyter
