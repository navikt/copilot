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

## Det viktigste først: bruk `@nav-pilot`

`@nav-pilot` er allerede satt opp for å gi deg kompakte svar. Den:

- Starter med konklusjonen, ikke forklaringen
- Viser kode direkte uten lang innledning
- Tilbyr «Si 'forklar' for detaljer» når den hopper over begrunnelse
- Spør bare når svaret faktisk endrer implementeringen

Du trenger ikke gjøre noe spesielt. Bare bruk `@nav-pilot` i stedet for standard Copilot Chat, så får du dette automatisk.

## Vil du ha enda kortere svar? Bruk `$terse-mode`

`$`-prefikset er Copilot sin måte å aktivere en [skill](https://code.visualstudio.com/docs/copilot/customization/agent-skills) — tenk på det som en kommando du skriver i chatten. Skriv `$terse-mode` for å skru på ekstra kompakt stil. Tre nivåer:

| Nivå | Hva den gjør | Eksempel |
| ---- | ------------ | -------- |
| **lett** | Fjerner fyllord, beholder fulle setninger | «Komponenten re-rendrer fordi du lager ny objektreferanse. Bruk `useMemo`.» |
| **normal** | Fragmenter og korte synonymer | «Ny objekt-ref hver render → re-render. `useMemo`.» |
| **ultra** | Telegrafisk, forkortelser | «Inline obj-prop → ny ref → re-render. `useMemo`.» |

Slik bruker du det:

```text
@nav-pilot $terse-mode          ← slår på normal-nivå
@nav-pilot $terse-mode ultra    ← for raske iterasjoner
Stopp terse                     ← tilbake til vanlig stil
```

Stilen vedvarer hele sesjonen. Ved sikkerhetsvarsler eller destruktive handlinger bytter den automatisk tilbake til full prosa — du mister aldri viktig informasjon.

## Fire vaner som kutter kostnader

### 1. Vær presis i spørsmålet

Den dyreste token-sløsingen er misforståelser. Fem runder med «mente du A eller B?» koster mer enn ett godt spørsmål.

**Dyrt** (vagt → mange oppfølgingsspørsmål):
> «Lag en tjeneste for søknader»

**Billig** (presist → agenten kan starte med én gang):
> «Lag et Ktor REST-endepunkt som tar imot dagpengesøknader over Kafka fra dp-soknad. Kotlin, Nais, Postgres.»

Jo mer kontekst du gir i første melding, jo færre runder bruker du.

### 2. For store oppgaver: la intervjuet gjøre jobben

`@nav-pilot` starter alltid med en intervjufase der den kartlegger blindsoner — personvern, auth, avhengigheter. For små oppgaver går dette raskt. For store oppgaver (ny tjeneste, stor refaktor) bruker den mer tid på å stille presise spørsmål.

Hvis du vil ha et enda grundigere intervju, kan du be om det eksplisitt med `$nav-deep-interview`. Den kjører en strukturert gjennomgang med impactanalyse og dekker flere områder enn standardintervjuet.

Fem minutter med avklaring slår en bortkastet sesjon.

### 3. Hold sesjoner fokuserte

Copilot bruker [prompt caching](/nyheter/model-pinning-kostnadsoptimalisering) — kontekst fra tidligere i sesjonen koster opptil 90 % mindre enn ny kontekst. Det betyr at du *ikke* trenger å starte ny sesjon bare for å spare penger. Men en fokusert sesjon gir bedre svar fordi modellen slipper å filtrere bort irrelevant historikk.

- Én oppgave per sesjon gir mer presise svar
- Unngå «kan du også...» som tar sesjonen i helt ny retning
- Lang, ufokusert historikk forvirrer — ikke bare koster

### 4. La `@nav-pilot` finne verktøyene

Du trenger ikke huske alle skills. `@nav-pilot` bruker riktig kunnskap basert på konteksten:

- Skriver du Go eller Kotlin? → Sikkerhetsregler (OWASP) er allerede aktive
- Jobber du med Nais-manifest? → Nais-kunnskap aktiveres
- Trenger du auth? → TokenX/Azure AD-kunnskap brukes

Du kan be om en spesifikk skill med `$skill-navn`, men for de fleste oppgaver klarer nav-pilot seg selv.

## Oppsummert

| Tips | For hvem | Hva du gjør |
| ---- | -------- | ----------- |
| Bruk `@nav-pilot` | Alle | Skriv `@nav-pilot` foran spørsmålet — kompakte svar er standard |
| `$terse-mode` | Deg som vil ha kortere svar | Skriv `$terse-mode` i starten av sesjonen |
| Vær presis | Alle | Nevn språk, rammeverk og integrasjoner i første melding |
| `$nav-deep-interview` | Nye tjenester, stor refaktor | Skriv `$nav-deep-interview` for grundigere avklaring |
| Fokuserte sesjoner | Alle | Hold deg til én oppgave per sesjon — bedre svar |

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

RTK rapporterer 60–90 % reduksjon på verktøydata. RTK og `$terse-mode` utfyller hverandre: RTK kutter det agenten *leser*, terse-mode kutter det agenten *skriver*.

### TOON og dynamiske verktøysett

To teknikker for deg som bygger MCP-servere:

- [TOON](https://toonformat.dev/) koder strukturert data med 30–60 % færre tokens. Relevant når serveren returnerer store JSON-objekter.
- **Dynamiske verktøysett** — ikke last alle verktøyskjemaer på forhånd. Bruk et søk → beskriv → utfør-mønster. [Speakeasy rapporterer 100x reduksjon](https://www.speakeasy.com/blog/how-we-reduced-token-usage-by-100x-dynamic-toolsets-v2) med denne tilnærmingen.

## Videre lesing

- [GitHub: Improving Token Efficiency in Agentic Workflows](https://github.blog/ai-and-ml/github-copilot/improving-token-efficiency-in-github-agentic-workflows/)
- [arXiv: Brief is better?](https://arxiv.org/abs/2604.00025) — korte svar kan gi bedre nøyaktighet
- [Matt Pocock: skills](https://github.com/mattpocock/skills) — grill-mønster og andre agent-skills
- [Vantage: Hidden Cost Driver in Agentic Coding](https://www.vantage.sh/blog/agentic-coding-costs) — kostnadsanalyse av agentiske arbeidsflyter
