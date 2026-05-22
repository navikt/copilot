---
title: "Agenter, skills eller instruksjoner? Slik velger du riktig"
date: 2026-05-22
author: starefosen
category: praksis
excerpt: "Copilot har seks typer tilpasninger. Her er de tre viktigste for Nav-utviklere — og når du bruker hva."
tags:
  - agents
  - skills
  - instructions
  - customizations
  - best-practices
---

GitHub Copilot har seks typer tilpasninger. Denne artikkelen dekker de tre som er mest relevante for Nav-utviklere å lage selv: instruksjoner, skills og agenter. (De øvrige — prompts, MCP-servere og hooks — dekkes separat.)

---

## Oversikt

| Type | Filplassering | Aktivering | Bruk til |
|------|---------------|------------|----------|
| **Instruksjoner** | `.github/instructions/*.instructions.md` | Automatisk (glob-match) | Kodestandarder og regler |
| **Skills** | `.github/skills/*/SKILL.md` | On-demand (`/skill` eller auto) | Gjenbrukbare workflows og domenekunnskap |
| **Agenter** | `.github/agents/*.agent.md` | Eksplisitt (`@agent`) | Spesialistroller med verktøykontroll |
| **Prompts** | `.github/prompts/*.prompt.md` | Eksplisitt (`/prompt`) | Engångsoppgaver med forhåndsdefinert kontekst |
| **MCP-servere** | `mcp.json` / settings | Alltid tilgjengelig | Koble til eksterne API-er og verktøy |
| **Hooks** | `.github/hooks/` / settings | Automatisk (livssyklus) | Kjør skript ved filendring eller commit |

---

## Instruksjoner — alltid-på regler

Instruksjoner gjelder automatisk basert på filtype. Du trenger ikke huske å aktivere dem.

**Bruk instruksjoner for:**
- Kodestandarder ("bruk Aksel spacing tokens, aldri Tailwind p-/m-")
- Språkkonvensjoner (Go idiomer, Kotlin-mønstre)
- Sikkerhetsregler (OWASP-mønstre for spesifikke filtyper)
- Code review-retningslinjer

**Eksempel:** `security-owasp.instructions.md` med `applyTo: "**/*.{kt,go}"` sørger for at sikkerhetsregler alltid gjelder når du jobber med Kotlin eller Go — uten at du trenger å tenke på det.

```yaml
---
applyTo: "**/*.{kt,go}"
---
# OWASP Top 10 — kodenivå
Bruk parameteriserte spørringer. Logg aldri PII...
```

---

## Skills — gjenbrukbare oppskrifter

Skills er kunnskapspakker som lastes inn ved behov. De kan inneholde instruksjoner, skript, maler og eksempler — og fungerer i VS Code, Copilot CLI og kodingsagenten.

**Bruk skills for:**
- Scaffolding-workflows (`/ktor-scaffold`, `/spring-boot-scaffold`)
- Diagnostikk og feilsøking (`/nav-troubleshoot`, `/observability-debugging`)
- Domenekunnskap med eksempler (`/api-design`, `/flyway-migration`)
- Sikkerhetssjekker (`/security-review`)
- Engangsprosedyrer ("generer ADR", "lag trusselmodell")

**Eksempel:** `/security-review` gir deg en komplett sjekkliste med kommandoer du kan kjøre:

```markdown
---
name: security-review
description: Bruk før commit for å sjekke at koden er trygg å merge
---
# Security Review Skill
## Scan repo
trivy repo .
zizmor .github/workflows/
```

**Nøkkelforskjell fra instruksjoner:** Skills lastes kun når de trengs (sparer kontekstvindu). De kan inneholde filer og skript, ikke bare tekst.

---

## Agenter — spesialister med verktøykontroll

Agenter er spesialister med egne verktøy, modellvalg og handoffs mellom roller.

**Bruk agenter når du trenger:**
- Verktøybegrensninger (en planleggingsagent som kun kan lese, ikke redigere)
- Spesifikt modellvalg (Opus for arkitektur, Codex for koding)
- MCP-verktøy (Figma, GitHub API)
- Handoffs mellom roller (Plan → Implementer → Review)
- En vedvarende persona i samtalen

**Eksempel:** `@aksel-agent` har tilgang til Figma MCP-verktøy for å hente designtokens direkte:

```yaml
---
name: aksel-agent
model: Claude Sonnet 4.6
tools:
  - com.figma/figma-mcp/get_design_context
  - com.figma/figma-mcp/get_variable_defs
---
```

**Nøkkelforskjell fra skills:** Agenter kontrollerer *hvilke* verktøy som er tilgjengelige. Skills gir kunnskap, agenter gir verktøytilgang.

---

## Beslutningstre

```
Trenger det å gjelde automatisk for en filtype?
  → Ja → Instruksjon

Trenger det verktøybegrensning, modellvalg eller MCP?
  → Ja → Agent

Er det domenekunnskap, workflow eller prosedyre?
  → Ja → Skill
```

---

## Vanlige feil

| Feil | Bedre løsning |
|------|---------------|
| Agent som kun leverer kunnskap (ingen verktøyrestriksjon) | Skill — mer portabel, lastes on-demand |
| Skill som alltid skal gjelde | Instruksjon — trenger ikke aktiveres manuelt |
| Instruksjon med kompleks workflow og skript | Skill — kan ha filer og ressurser |
| Samme innhold i agent OG skill | Velg én. Agent refererer til skill med `/skill-name` |

---

## Hva vi har gjort

Vi har gjort følgende i navikt/copilot:

- **Lagt til `code-review.instructions.md`** — generelle review-instruksjoner som gjelder automatisk under Copilot code review (sikkerhet, NAIS-konfig, GitHub Actions, testdekning).
- **Identifisert duplikater** — `@rust-agent` og `/rust-development` har samme innhold. Se [issue #252](https://github.com/navikt/copilot/issues/252) for oppryddingsplan.
- **Beholdt agenter som faktisk trenger verktøykontroll** — `@aksel-agent` (Figma MCP), `@nav-pilot` (Opus + orkestrator), `@security-champion` (Opus + rådgiver).

---

## Lag dine egne

VS Code har innebygde kommandoer for å generere tilpasninger:

- `/create-instruction` — lag en instruksjon
- `/create-skill` — lag en skill
- `/create-agent` — lag en agent

Start med instruksjoner for kodestandarder. Legg til skills for workflows teamet gjentar ofte. Bruk agenter bare når du trenger verktøykontroll.
