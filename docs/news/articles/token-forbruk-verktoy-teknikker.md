---
title: "Slik holder du token-forbruket nede"
date: 2026-05-28
author: starefossen
category: praksis
excerpt: "Praktiske verktøy og teknikker for å redusere token-forbruk i agentiske arbeidsflyter — fra native $terse-mode og RTK til kontekstkomprimering og smartere instruksjonsarkitektur."
tags:
  - token-optimization
  - cost-optimization
  - tools
  - agents
  - context-engineering
  - instructions
  - skills
  - owasp
---

Med [bruksbasert fakturering](/nyheter/model-pinning-kostnadsoptimalisering) blir token-forbruk direkte synlig på regningen. Den skjulte kostnadsdriveren er ofte input-tokens: hver runde i en agentisk sesjon sender hele konteksten på nytt — systemprompt, instruksjonsfiler, åpne filer og samtalehistorikk. En enkelt flerstegs-sesjon i Copilot CLI kan bruke over 1 million input-tokens. [Vantage sin analyse](https://www.vantage.sh/blog/agentic-coding-costs) viser at kostnader varierer opptil 30x mellom kjøringer på samme oppgave.

Nytt funn fra forskningen gjør dette ekstra interessant: en [2026-studie på arXiv](https://arxiv.org/abs/2604.00025) fant at store modeller ble **26 prosentpoeng mer treffsikre** på enkelte resonneringsoppgaver når de ble tvunget til å svare kort. For mye forklaring gir flere feil. Terse betyr derfor ikke nødvendigvis dårligere svar.

Her er verktøy og teknikker som fungerer med GitHub Copilot-økosystemet i dag.

## Konkrete tall fra dette repoet

Vi flyttet `security-owasp.instructions.md` fra alltid-aktiv instruks til en on-demand skill. Før ble OWASP-referansen lastet ved hver Go- og Kotlin-redigering, også når oppgaven ikke handlet om sikkerhet. Nå ligger bare en kort instruksjonsstub igjen med kritiske regler og peker til skillen.

| Måling | Før | Etter | Endring |
| ------ | --- | ----- | ------- |
| `security-owasp` per redigering | 21 KB | ~1 KB stub | −95 % |
| Go-kontekst | 42 KB | 22 KB | −20 KB |
| Kotlin-kontekst | 54 KB | 34 KB | −20 KB |

Dette er fortsatt det mest lønnsomme grepet vi har gjort: flytt referansemateriale ut av hot path. Instruksjonsfiler er i hovedsak maskinlest innmat. Det betyr at vi trygt kan komprimere dem hardere enn vi ville gjort med menneskevennlig dokumentasjon, så lenge reglene fortsatt er presise.

## Verktøy

### `$terse-mode` — native tershet i nav-pilot

Den viktigste nyheten er at nav-pilot nå har en innebygd [`$terse-mode`](https://github.com/navikt/copilot/blob/main/.github/skills/terse-mode/SKILL.md)-skill. Du trenger ikke installere noe. Skriv bare `$terse-mode` i en Copilot-sesjon.

Den bygger på de samme idéene som Caveman, men er skrevet for norsk arbeidsflyt og følger nav-pilot-konvensjonene våre:

- **lett** — korte, profesjonelle svar med hele setninger
- **normal** — fragmenter og mindre fyllord. God standard for hverdagsarbeid
- **ultra** — telegrafisk stil for raske iterasjoner

Det viktigste er ikke bare at svarene blir kortere. Skillen har en eksplisitt vedvaringsregel som hindrer modellen i å drive tilbake til lange svar etter noen runder. Den har også **auto-klarhet**: ved sikkerhetsvarsler, destruktive operasjoner eller tvetydige steg bytter den automatisk tilbake til full prosa.

Praktisk bruk:

```text
$terse-mode
Aktiver terse-mode ultra
Stopp terse
```

Dette er førstevalget vårt for output-komprimering i Nav-miljøet: native, norsk, ingen tredjepartsavhengighet.

### RTK — komprimerer verktøyutdata

[RTK](https://github.com/rtk-ai/rtk) (55k+ stjerner) er et praktisk verktøy for team som bruker agenter i terminalen. Det komprimerer kommandoutput før den når LLM-konteksten. Det treffer et annet problem enn `$terse-mode`: ikke agentens prosa, men verktøyutdata.

**Konkret besparelse i en 30-minutters sesjon:**

| Operasjon | Uten RTK | Med RTK | Besparelse |
| --------- | -------- | ------- | ---------- |
| `cat` / `read` (20x) | 40 000 | 12 000 | −70 % |
| `cargo test` / `npm test` (5x) | 25 000 | 2 500 | −90 % |
| `git diff` (5x) | 10 000 | 2 500 | −75 % |
| **Totalt** | **~118 000** | **~24 000** | **−80 %** |

Installer med én kommando: `brew install rtk`. For Copilot CLI bruker du `rtk init -g --copilot`, som setter opp en `PreToolUse`-hook. Det fungerer godt med `kubectl`, `git`, `go test` og `cargo test`, altså kommandoer mange av oss bruker i Nais-arbeidsflyter.

RTK og `$terse-mode` er komplementære: RTK kutter verktøyutdata, `$terse-mode` kutter agentprosa.

### Caveman — ekstern output-komprimering

[Caveman](https://github.com/JuliusBrussee/caveman) (54k+ stjerner) er fortsatt relevant. Det er et populært oppsett for output-komprimering og fungerer også med Copilot. Forskjellen nå er at du ikke trenger Caveman for å få samme stilgrep i nav-pilot.

**Når Caveman fortsatt er nyttig:**

- du vil ha samme terse-stil på tvers av flere assistenter utenfor nav-pilot
- du vil eksperimentere med `caveman-compress` for å korte ned instruksjonsfiler
- du eier repoets `copilot-instructions.md` og vil la et eksternt verktøy skrive dit

For Nav-team er anbefalingen enklere enn før: start med `$terse-mode`. Vurder Caveman hvis du trenger et agent-uavhengig oppsett.

### TOON — mindre tokens for strukturerte data

[TOON](https://toonformat.dev/) ([spec](https://github.com/toon-format/spec)) er et serialiseringsformat designet for LLM-kontekster. Det koder JSON-lignende data med 30–60 % færre tokens, uten tap av informasjon.

**Eksempel:**

```json
{ "users": [{ "id": 1, "name": "Ada" }, { "id": 2, "name": "Linus" }] }
```

```text
users[2]{id,name}:
  1,Ada
  2,Linus
```

Dette er mest relevant for MCP-servere og verktøy som sender mye strukturert data. Hvis du eier formatet selv, er TOON ofte et bedre grep enn å be modellen være «kort».

### Copilot Memory — innebygd kontekstbevaring

GitHub Copilot har [innebygd memory](/nyheter/copilot-memory) som lagrer fakta og preferanser på tvers av sesjoner. For lengre økter bruker Copilot CLI også checkpointing, som komprimerer eldre kontekst når vinduet fylles opp.

Det løser ikke alt, men det reduserer behovet for å gjenta samme bakgrunnsinformasjon i hver nye sesjon.

## Teknikker

### 1. Avklaring før implementering

En stor del av token-sløsing kommer ikke fra lange svar, men fra lange misforståelser. Matt Pocock sitt [skills-repo](https://github.com/mattpocock/skills) har passert 109k stjerner og populariserte **grill**-mønsteret: la agenten kjøre et sokratisk, nesten adversarialt intervju før implementering starter.

Poenget er enkelt: bruk noen få presise spørsmål først, så slipper du ti runder med «mente du A eller B?». I vårt økosystem dekker [`$nav-deep-interview`](https://github.com/navikt/copilot/blob/main/.github/skills/nav-deep-interview/SKILL.md) mye av det samme behovet. Den avdekker blindsoner rundt personvern, auth, avhengigheter og observerbarhet før koden skrives.

Hvis oppgaven er uklar, er dette ofte den billigste token-optimaliseringen du kan gjøre.

### 2. Kontekstkomprimering i Copilot CLI

Copilot CLI har innebygd kontekstkomprimering som trigges automatisk når kontekstvinduet fylles opp. Den oppsummerer eldre kontekst via checkpointing. Besparelsen er stor, men du får best effekt hvis samtalen er strukturert.

Tips: bruk tydelige overskrifter, eksplisitte tilstandsrapporter og korte beslutningsnotater underveis. Slike signaler overlever komprimering bedre enn løs prat.

### 3. Instruksjonsarkitektur

Her ligger de største input-gevinstene i repo vi kontrollerer selv.

Det viktigste feltet i en Copilot-instruksjon er ofte ikke teksten, men `applyTo`-globen. Den er Copilot sitt sterkeste lazy-loading-grep. En fil med `applyTo: "**"` er alltid på. En fil med et smalt mønster lastes bare når den faktisk trengs.

Det vi ser nå, er et tydelig mønster:

- **Komprimer maskininnhold hardt.** Instruksjonsfiler leses primært av modellen, ikke av mennesker
- **Hold stabile filer stabile.** Prompt caching treffer best når de sjelden endres og starter med det samme innholdet hver gang
- **Bruk konkrete dropp-lister.** Ord som «dropp artikler, høflighetsfraser og hedging» virker bedre enn vage instrukser som «vær kort»
- **Flytt oppslagsverk til skills.** Det ga oss 42 KB → 22 KB i Go-kontekst og 54 KB → 34 KB i Kotlin-kontekst

Faustregel: Hvis en instruks bare er relevant for 10 % av oppgavene, hører den hjemme i en skill.

### 4. Dynamiske verktøysett i MCP

[Speakeasy rapporterer 100x reduksjon](https://www.speakeasy.com/blog/how-we-reduced-token-usage-by-100x-dynamic-toolsets-v2) ved å gå fra å laste alle verktøyskjemaer på forhånd til et søk → beskriv → utfør-mønster. 400 verktøy gikk fra 410 000 tokens til 8 000–31 000.

Relevant for team som bruker mange MCP-servere: bruk [Atlassian mcp-compressor](https://github.com/atlassian/mcp-compressor) for skjemakomprimering eller implementer progressiv avsløring i egne servere.

### 5. Output-kontroll med eksplisitte regler

Ikke skriv bare «be concise». Det er for vagt. Gi modellen konkrete stilregler eller bruk en skill som allerede gjør det.

Tre nivåer som fungerer i praksis:

- **lett** for vanlig samarbeid og PR-gjennomgang
- **normal** for daglig terminalarbeid
- **ultra** for raske iterasjoner når du kjenner domenet godt

Det er akkurat derfor `$terse-mode` fungerer bedre enn en løs énlinjer i prompten: den spesifiserer hva som skal bort, når full prosa må tilbake og at stilen skal vedvare.

### 6. Modell-routing

Bruk billige og raske modeller for enkle oppgaver som klassifisering, filsøk og formatering. Reserver frontier-modeller for kompleks resonnering. Copilot CLI gjør dette allerede med explore-agenter på Haiku.

### 7. Filtrer verktøyresponsene

Returner bare feltene du trenger fra MCP-verktøy. Ikke dump hele objekter hvis brukeren bare trenger `name`, `id` og `status`.

Hvis du kontrollerer output-formatet, har du ofte mer å hente på responsfiltrering eller TOON enn på å be modellen oppsummere dataene etterpå.

## Hva du kan gjøre i dag

1. **Slå på `$terse-mode`** i neste Copilot-sesjon. Start med `lett`, bytt til `ultra` når du vil ha høy fart
2. **Installer RTK** med `brew install rtk && rtk init -g --copilot`
3. **Bruk `$nav-deep-interview`** før større endringer. Færre misforståelser gir kortere sesjoner
4. **Se på `applyTo`-mønstrene dine** og finn instruksjoner som lastes oftere enn de burde
5. **Flytt referansestoff til skills** og behold bare kritiske regler i instruksjonsfila
6. **Gjør instruksjoner mer konkrete** med dropp-lister og korte, stabile regler i toppen av filen
7. **Komprimer strukturerte data** med TOON eller smalere responser fra MCP-verktøy

## Videre lesing

- [GitHub: Improving Token Efficiency in Agentic Workflows](https://github.blog/ai-and-ml/github-copilot/improving-token-efficiency-in-github-agentic-workflows/) — GitHub sin egen tilnærming
- [arXiv: Brief is better?](https://arxiv.org/abs/2604.00025) — studie om hvorfor korte svar kan gi bedre nøyaktighet
- [Matt Pocock: skills](https://github.com/mattpocock/skills) — grill-mønster, TDD og andre agent-skills
- [awesome-llm-token-optimization](https://github.com/pleasedodisturb/awesome-llm-token-optimization) — kuratert samling
- [Vantage: Hidden Cost Driver in Agentic Coding](https://www.vantage.sh/blog/agentic-coding-costs) — kostnadsanalyse
