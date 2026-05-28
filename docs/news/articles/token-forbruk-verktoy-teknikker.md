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
  - instructions
  - skills
  - owasp
---

Med [bruksbasert fakturering](/nyheter/model-pinning-kostnadsoptimalisering) blir token-forbruk direkte synlig på regningen. Den skjulte kostnadsdriveren er input-tokens: hver runde i en agentisk sesjon sender hele konteksten på nytt — systemprompt, instruksjonsfiler, åpne filer, samtalehistorikk. En enkelt flerstegs-sesjon i Copilot CLI kan bruke over 1 million input-tokens. [Vantage sin analyse](https://www.vantage.sh/blog/agentic-coding-costs) viser at kostnader varierer opptil 30x mellom kjøringer på samme oppgave.

Her er verktøy og teknikker som fungerer med GitHub Copilot-økosystemet.

## Konkrete tall fra dette repoet

I dag flyttet vi `security-owasp.instructions.md` (21 KB) fra alltid-aktiv instruks til en on-demand skill. Før ble OWASP-referansen lastet ved hver Go- og Kotlin-redigering, også når oppgaven ikke handlet om sikkerhet. Nå ligger bare en kort instruksjonsstub igjen med kritiske regler og peker til skillen.

| Måling | Før | Etter | Endring |
|--------|-----|-------|---------|
| `security-owasp` per redigering | 21 KB | ~1 KB stub | −95 % |
| Go-kontekst | 42 KB | 22 KB | −20 KB |
| Kotlin-kontekst | 54 KB | 34 KB | −20 KB |

Det viktigste poenget er ikke bare færre tokens. Vi fikk også en tydeligere deling mellom regler som alltid gjelder og referansemateriale som bare trengs av og til.

## Verktøy

### RTK — Rust Token Killer

[RTK](https://github.com/rtk-ai/rtk) er et praktisk verktøy for Nav-team som bruker agenter i terminalen. Det er en Apache 2.0-lisensiert Rust-binær med 55k+ GitHub-stjerner, og det komprimerer kommandoutput før den når LLM-konteksten.

**Konkret besparelse i en 30-minutters sesjon:**

| Operasjon | Uten RTK | Med RTK | Besparelse |
|-----------|----------|---------|------------|
| `cat` / `read` (20x) | 40 000 | 12 000 | −70 % |
| `cargo test` / `npm test` (5x) | 25 000 | 2 500 | −90 % |
| `git diff` (5x) | 10 000 | 2 500 | −75 % |
| **Totalt** | **~118 000** | **~24 000** | **−80 %** |

Installer med én kommando: `brew install rtk`. For Copilot CLI bruker du `rtk init -g --copilot`, som setter opp en `PreToolUse`-hook. Det fungerer godt med `kubectl`, `git`, `go test` og `cargo test`, altså kommandoer mange av oss bruker i NAIS-arbeidsflyter.

RTK jobber på et annet lag enn egne repo-tiltak som `nav-pilot` og instruksjonsopprydding. RTK komprimerer verktøyutdata. Repo-arbeidet vårt kutter alltid-lastet kontekst. Du får best effekt når du gjør begge deler.

### Caveman — idé, ikke verifisert verktøy

Tidligere pekte vi til «Caveman» som et offentlig verktøy for semantisk komprimering. Det ser ikke ut til å finnes som et verifiserbart, offentlig verktøy. Behold derfor dette som en idé, ikke som en anbefalt installasjon.

Poenget står fortsatt: korte, tette instruksjoner gir ofte bedre signal enn lange tekster med mye fyll. Men i praksis er RTK, bedre instruksjonsarkitektur og mindre MCP-responser tryggere tiltak å starte med.

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

Faustregel: Hvis en instruks bare er relevant for 10 % av oppgavene, hører den hjemme i en skill.

#### Slik skriver du effektive instruksjoner og skills

- **Hold instruksjoner lean.** Ta bare med regler som faktisk gjelder per redigering: sjekklister, kritiske regler og korte påpekninger.
- **Flytt referansemateriale til skills.** Kodeeksempler, utdypinger, matriser og lange forklaringer hører hjemme i innhold som lastes på forespørsel.
- **Følg skill-lint-regelen.** Hold `SKILL.md` under 500 linjer. Del opp i `SKILL.md` + referansefiler som `examples.md` når innholdet blir stort.
- **Bruk en fast struktur.** Instruksjonsstub med kritiske regler og peker til skill → skill med konsise mønstre → `examples.md` med full referanse.

Det var akkurat dette vi gjorde med OWASP-innholdet: en kort sikkerhetsinstruks ble stående igjen, mens detaljene flyttet til `security-owasp`-skillen og egne eksempelfiler.

#### OWASP Top 10:2025 bør ligge i skillen, ikke i hot path

OWASP-listen for 2025 gjør dette enda tydeligere. Den inneholder både nye kategorier og flyttede temaer:

- **A03: Software Supply Chain Failures** erstatter «Vulnerable Components»
- **A10: Mishandling of Exceptional Conditions** er ny
- **SSRF** er slått sammen inn i **A01: Broken Access Control**
- **Security Misconfiguration** er flyttet til **A02**

Denne typen oversikter endrer seg. Derfor bør du legge den fulle referansen i en skill som er enkel å oppdatere, mens instruksjonsfila bare minner om de viktigste reglene som alltid gjelder.

#### Instruksjon → skill er et mønster du kan kopiere

Dette er ikke bare et grep for sikkerhetsfiler. Det er et generelt mønster for alle repo med mye kontekst:

1. **Mål hvilke instruksjonsfiler som alltid lastes.** Start med de største filene.
2. **Skill mellom regler og oppslagsverk.** Regler blir i instruksjonen, oppslagsverket flyttes ut.
3. **Lag en liten stub.** Behold 5–10 kritiske regler og en tydelig peker til riktig skill.
4. **Samle eksempler i egne filer.** Da holder du `SKILL.md` kort uten å miste nytteverdien.
5. **Mål på nytt etter flytting.** Se på faktisk kontekststørrelse per interaksjon, ikke bare filstørrelse.

For team som jobber mye i Go, Kotlin, TypeScript eller YAML er dette ofte den enkleste måten å kutte tokens uten å ofre kvalitet.

#### Praktisk anbefaling for Nav-team

Hvis du bare skal prøve ett eksternt verktøy, start med RTK. Det er enkelt å installere, lett å teste i egen terminal og treffer kommandoene vi bruker mest i Nav: `kubectl`, `git`, `go test` og `cargo test`.

Anbefalt oppsett:

1. `brew install rtk`
2. `rtk init -g --copilot`
3. Kjør en vanlig arbeidsøkt og sammenlign output før og etter

RTK er et godt første grep fordi det ikke krever at du skriver om instruksjoner eller skills. Samtidig erstatter det ikke repo-opprydding. Bruk RTK for kommandoutput, og bruk instruksjon → skill-mønsteret for å kutte statisk kontekst.

### 4. Output-kontroll

Sett `"Code only, no explanation"` eller tilsvarende i agentinstruksjoner. Output-tokens koster 2–5x mer enn input. Besparelse: 40–70 %.

### 5. Modell-routing

Bruk billige og raske modeller (Haiku, GPT-5-mini) for enkle oppgaver som klassifisering, filsøk og formatering. Reserver frontier-modeller for kompleks resonnering. Copilot CLI gjør dette allerede med explore-agenter på Haiku.

### 6. Verktøyutdata-filtrering

Returner bare nødvendige felter fra MCP-verktøy. Aldri dump hele objekter. [MindStudio dokumenterer 80–98 % reduksjon](https://www.mindstudio.ai/blog/reduce-token-usage-ai-agents-mcp-optimization) bare ved å begrense responsstørrelse.

## Hva du kan gjøre i dag

1. **Installer RTK** (`brew install rtk && rtk init -g --copilot`) — umiddelbar besparelse på kommandoutput
2. **Mål instruksjonskonteksten din** — finn filer som lastes ofte og er større enn de trenger å være
3. **Flytt oppslagsverk til skills** — behold bare kritiske regler i instruksjonsfila
4. **Sett skill-tak på 500 linjer** — del store skills i `SKILL.md` og referansefiler
5. **Oppdater sikkerhetsreferanser i skillen** når OWASP, NIST eller interne standarder endrer seg
6. **Be om terse output og bruk riktig modell** — ikke bruk dyr modell til enkle oppgaver

## Videre lesing

- [GitHub: Improving Token Efficiency in Agentic Workflows](https://github.blog/ai-and-ml/github-copilot/improving-token-efficiency-in-github-agentic-workflows/) — GitHubs egen tilnærming
- [awesome-llm-token-optimization](https://github.com/pleasedodisturb/awesome-llm-token-optimization) — kuratert samling
- [Morph: 7 Context Compression Methods Compared](https://www.morphllm.com/context-compression) — benchmark av metoder
- [SEP-1576: Mitigating Token Bloat in MCP](https://github.com/modelcontextprotocol/modelcontextprotocol/issues/1576) — spesifikasjonsforslag
- [Vantage: Hidden Cost Driver in Agentic Coding](https://www.vantage.sh/blog/agentic-coding-costs) — kostnadsanalyse
