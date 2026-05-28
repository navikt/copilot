---
title: "Slik holder du token-forbruket nede"
date: 2026-05-28
author: starefossen
category: praksis
excerpt: "Praktiske verktøy og teknikker for å redusere token-forbruk i agentiske arbeidsflyter — fra RTK og Caveman til kontekstkomprimering og instruksjonsarkitektur."
tags:
  - token-optimization
  - cost-optimization
  - tools
  - agents
  - context-engineering
---

Med [bruksbasert fakturering](/nyheter/model-pinning-kostnadsoptimalisering) blir token-forbruk direkte synlig på regningen. Den skjulte kostnadsdriveren er input-tokens: hver runde i en agentisk sesjon sender hele konteksten på nytt — systemprompt, instruksjonsfiler, åpne filer, samtalehistorikk. En enkelt flerstegs-sesjon i Copilot CLI kan bruke over 1 million input-tokens. [Vantage sin analyse](https://www.vantage.sh/blog/agentic-coding-costs) viser at kostnader varierer opptil 30x mellom kjøringer på samme oppgave.

Her er verktøy og teknikker som fungerer med GitHub Copilot-økosystemet.

## Verktøy

### RTK — Rust Token Killer

[RTK](https://github.com/rtk-ai/rtk) er en CLI-proxy som filtrerer og komprimerer kommandoutput før den når LLM-konteksten. Én Rust-binær, 100+ støttede kommandoer, <10 ms overhead.

**Konkret besparelse i en 30-minutters sesjon:**

| Operasjon | Uten RTK | Med RTK | Besparelse |
|-----------|----------|---------|------------|
| `cat` / `read` (20x) | 40 000 | 12 000 | −70 % |
| `cargo test` / `npm test` (5x) | 25 000 | 2 500 | −90 % |
| `git diff` (5x) | 10 000 | 2 500 | −75 % |
| **Totalt** | **~118 000** | **~24 000** | **−80 %** |

Installasjon: `brew install rtk`, deretter `rtk init -g` for Copilot CLI eller `rtk init -g --codex` for Codex CLI.

RTK støtter Copilot CLI, Claude Code, Codex, Cursor, Windsurf og Cline. Det fungerer ved fire strategier: smart filtrering (fjerner støy), gruppering (samler lignende elementer), trunkering (beholder relevant kontekst) og deduplisering (kollapser gjentatte linjer).

### Caveman — semantisk komprimering

[Caveman](https://github.com/wilpel/caveman-compression) stripper alt LLM-en uansett kan rekonstruere — artikler, bindeord, fyllord — og beholder bare tett semantisk innhold. LLM-er er flinke til å gjenoppbygge grammatikk fra fakta.

**Eksempel:**
- Før: "In order to optimize the database query performance, we should consider implementing an index on the most frequently accessed columns."
- Etter: "Need fast queries. Check which columns used most. Add index those."

Typisk besparelse: 50–75 % på instruksjoner og kontekstfiler. En [studie fra 2026](https://pyshine.com/Caveman-Cut-75-LLM-Output-Tokens/) viste at komprimerte prompts faktisk ga 26 prosentpoeng høyere nøyaktighet — mindre støy betyr bedre fokus.

Caveman kan brukes som skill eller middleware for å komprimere AGENTS.md, instruksjonsfiler og prosjektnotater før de lastes inn i hver sesjon.

### TOON — Token-Oriented Object Notation

[TOON](https://toonformat.dev/) ([spec](https://github.com/toon-format/spec)) er et serialiseringsformat designet for LLM-kontekster. Det koder JSON-datamodellen med 30–60 % færre tokens, uten tap av informasjon.

**Eksempel:**
```json
{ "users": [{ "id": 1, "name": "Ada" }, { "id": 2, "name": "Linus" }] }
```
```
users[2]{id,name}:
  1,Ada
  2,Linus
```

Potensielt nyttig for MCP-servere som returnerer strukturerte data. Implementasjoner finnes for TypeScript, Python, Go og Rust. Vi har [notert dette for videre testing](https://github.com/navikt/copilot/issues/258) med Copilot-arbeidsflyter.

### Copilot Memory — innebygd kontekstbevaring

GitHub Copilot har [innebygd memory](/nyheter/copilot-memory) som lagrer fakta og preferanser på tvers av sesjoner. Dette er den enkleste formen for kontekstbevaring — det krever ingen oppsett og fungerer automatisk i VS Code og Copilot CLI.

For lengre sesjoner bruker Copilot CLI automatisk checkpointing: den oppsummerer konteksten ved jevne intervaller slik at sesjonen kan fortsette uten å miste tråden.

## Teknikker

### 1. Kontekstkomprimering (innebygd i Copilot CLI)

Copilot CLI har innebygd kontekstkomprimering som trigges automatisk når kontekstvinduet fylles opp. Den oppsummerer eldre kontekst til en kompakt form via checkpointing. Besparelse: 65–90 % per syklus.

Tips: Strukturer arbeidet med tydelige overskrifter og eksplisitte tilstandsrapporter — dette overlever komprimering bedre enn implisitt kontekst.

### 2. Dynamiske verktøysett (MCP)

[Speakeasy rapporterer 100x reduksjon](https://www.speakeasy.com/blog/how-we-reduced-token-usage-by-100x-dynamic-toolsets-v2) ved å gå fra å laste alle verktøyskjemaer på forhånd til et søk → beskriv → utfør-mønster. 400 verktøy gikk fra 410 000 tokens til 8 000–31 000.

Relevant for team som bruker mange MCP-servere: bruk [Atlassian mcp-compressor](https://github.com/atlassian/mcp-compressor) for skjemakomprimering (44–97 % reduksjon) eller implementer progressiv avsløring i egne servere.

### 3. Instruksjonsarkitektur (Copilot-spesifikk)

Flytt sjelden brukt innhold fra alltid-aktive instruksjonsfiler (`.github/instructions/`) til **on-demand skills** (`.github/skills/`) som bare lastes ved behov. Instruksjonsfiler med `applyTo: "**"` lastes ved *hver* interaksjon. Filer med filtype-spesifikke patterns lastes ved redigering av matchende filer.

Eksempel fra dette repoet: En Kotlin-redigering laster 54 KB med instruksjoner — inkludert 21 KB OWASP-referansemateriale som sjelden trengs per oppgave.

Faustregel: Hvis en instruks bare er relevant for 10 % av oppgavene, hører den hjemme i en skill.

### 4. Output-kontroll

Sett `"Code only, no explanation"` eller tilsvarende i agentinstruksjoner. Output-tokens koster 2–5x mer enn input. Besparelse: 40–70 %.

### 5. Modell-routing

Bruk billige/raske modeller (Haiku, GPT-5-mini) for enkle oppgaver (klassifisering, filsøk, formatering). Reserver frontier-modeller for kompleks resonnering. Copilot CLI gjør dette allerede med explore-agenter på Haiku.

### 6. Verktøyutdata-filtrering

Returner bare nødvendige felter fra MCP-verktøy. Aldri dump hele objekter. [MindStudio dokumenterer 80–98 % reduksjon](https://www.mindstudio.ai/blog/reduce-token-usage-ai-agents-mcp-optimization) bare ved å begrense responsstørrelse.

## Hva du kan gjøre i dag

1. **Installer RTK** (`brew install rtk && rtk init -g`) — umiddelbar 80 % besparelse på kommandoutput
2. **Vurder Caveman-komprimering** på store instruksjonsfiler og AGENTS.md
3. **Hold instruksjonsfiler fokuserte** — én fil per bekymring, aldri dupliser innhold
4. **Be om terse output** i agentinstruksjoner
5. **Bruk riktig modell for oppgaven** — ikke bruk Opus til grep

## Videre lesing

- [GitHub: Improving Token Efficiency in Agentic Workflows](https://github.blog/ai-and-ml/github-copilot/improving-token-efficiency-in-github-agentic-workflows/) — GitHubs egen tilnærming
- [awesome-llm-token-optimization](https://github.com/pleasedodisturb/awesome-llm-token-optimization) — kuratert samling
- [Morph: 7 Context Compression Methods Compared](https://www.morphllm.com/context-compression) — benchmark av metoder
- [SEP-1576: Mitigating Token Bloat in MCP](https://github.com/modelcontextprotocol/modelcontextprotocol/issues/1576) — spesifikasjonsforslag
- [Vantage: Hidden Cost Driver in Agentic Coding](https://www.vantage.sh/blog/agentic-coding-costs) — kostnadsanalyse
