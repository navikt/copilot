---
title: "Agenter, skills eller instruksjoner? Slik velger du riktig"
date: 2026-05-22
author: starefossen
category: praksis
excerpt: "Copilot har flere typer tilpasninger. Her er når du bruker hva, med beslutningstre og eksempler fra navikt."
tags:
  - agents
  - skills
  - instructions
  - customizations
  - best-practices
---

GitHub Copilot i VS Code har flere tilpasningstyper. De er alle markdown-filer i `.github/` og ser like ut, men løser forskjellige problemer. Her er hvordan du velger riktig type.

## Tilpasningstyper

| Type | Filplassering | Aktivering | Bruk til |
|------|---------------|------------|----------|
| **Instruksjoner** | `.github/instructions/*.instructions.md` | Automatisk (glob-match) | Kodestandarder og regler |
| **Skills** | `.github/skills/*/SKILL.md` | On-demand (`/skill` eller auto) | Workflows og domenekunnskap |
| **Agenter** | `.github/agents/*.agent.md` | Eksplisitt (`@agent`) | Spesialistroller med verktøykontroll |
| **Prompts** | `.github/prompts/*.prompt.md` | Eksplisitt (`/prompt`) | Enkeltoppgaver med kontekst |
| **MCP-servere** | `mcp.json` / VS Code settings | Alltid tilgjengelig | Koble til API-er og databaser |
| **Hooks** | `.github/hooks/` | Automatisk (livssyklus) | Skript ved filendring eller commit |

Vi dekker de tre første, siden instruksjoner, skills og agenter er mest relevante å lage selv.

---

## Instruksjoner er regler som alltid gjelder

Instruksjoner er den enkleste tilpasningen. Du skriver regler i en markdown-fil, angir et glob-mønster, og reglene gjelder automatisk for matchende filer. Du trenger ikke aktivere noe manuelt.

Bruk instruksjoner for:

- Kodestandarder ("bruk Aksel spacing tokens, aldri Tailwind p-/m-")
- Språkkonvensjoner (Go-idiomer, Kotlin-mønstre)
- Sikkerhetsregler (OWASP-sjekker for Kotlin og Go)
- Review-retningslinjer (hva Copilot skal flagge i PR-er)

```yaml
---
applyTo: "**/*.{kt,go}"
---
# OWASP Top 10 — kodenivå
Bruk parameteriserte spørringer. Logg aldri PII...
```

Instruksjoner er passive. De påvirker AI-ens svar uten at du trenger å tenke på dem. [VS Code-dokumentasjonen](https://code.visualstudio.com/docs/copilot/customization/custom-instructions) anbefaler å starte her: «Start with a single `.github/copilot-instructions.md` file for project-wide coding standards. Add `.instructions.md` files when you need different rules for different file types.»

**Viktig:** Instruksjoner gjelder *ikke* for inline-forslag mens du skriver, kun for chat, agenter og code review.

---

## Skills gir kunnskap og workflows on-demand

Skills er mapper med en `SKILL.md`-fil som inneholder instruksjoner, og valgfritt skript, maler og referansemateriale. De lastes bare når oppgaven matcher, så de bruker ikke opp kontekstvinduet.

Skills er en [åpen standard](https://agentskills.io) som fungerer i VS Code, Copilot CLI og kodingsagenten, så en skill du lager i repoet virker uansett hvor du jobber.

Bruk skills for:

- Scaffolding (`/ktor-scaffold`, `/spring-boot-scaffold`)
- Diagnostikk (`/nav-troubleshoot`, `/observability-debugging`)
- Domenekunnskap (`/api-design`, `/flyway-migration`)
- Sikkerhetsprosedyrer (`/security-review`)

```
my-skill/
├── SKILL.md          # Påkrevd: metadata + instruksjoner
├── scripts/          # Valgfritt: kjørbar kode
├── references/       # Valgfritt: referansemateriale
└── assets/           # Valgfritt: maler og ressurser
```

### Gode råd for skills

[Agent Skills-spesifikasjonen](https://agentskills.io/skill-creation/best-practices) gir konkrete råd:

- **Skriv det agenten ikke vet.** Fokuser på prosjektspesifikke konvensjoner, API-detaljer og kjente fallgruver. Du trenger ikke forklare hva en database er.
- **Hold det under 500 linjer.** Flytt detaljert referansemateriale til `references/`-mappen og fortell agenten *når* den skal laste det.
- **Inkluder en gotchas-seksjon.** Feil agenten gjør uten å bli fortalt er ofte det mest verdifulle innholdet.
- **Test med ekte oppgaver.** Kjør skillen mot reelle oppgaver, les agenttracene, og oppdater basert på hva som fungerer.

---

## Agenter er spesialister med egne verktøy

Agenter er den mest avanserte tilpasningen. De definerer en persona med egne verktøy, modellvalg og handoffs til andre agenter. [VS Code-dokumentasjonen](https://code.visualstudio.com/docs/copilot/customization/custom-agents) beskriver dem slik: «Custom agents give the AI a specific persona and constrained set of tools for a particular role.»

Bruk agenter når du trenger:

- **Verktøybegrensning**, for eksempel en planleggingsagent som kun kan lese, ikke redigere
- **Modellvalg**, som Opus for arkitektur og Codex for implementering
- **MCP-verktøy** for tilgang til Figma, GitHub API eller databaser
- **Handoffs** for sekvensielle workflows (Plan → Implementer → Review)

```yaml
---
name: aksel-agent
model: Claude Sonnet 4.6
tools:
  - com.figma/figma-mcp/get_design_context
  - com.figma/figma-mcp/get_variable_defs
  - search
  - edit
---
```

Den viktigste forskjellen: agenter styrer *hvilke verktøy* som er tilgjengelige. En agent uten verktøybegrensning er i praksis bare en skill med ekstra overhead.

---

## Beslutningstre

```
Skal reglene gjelde automatisk for en filtype?
  → Ja → Instruksjon (.instructions.md)

Trenger du verktøybegrensning, modellvalg eller MCP?
  → Ja → Agent (.agent.md)

Er det domenekunnskap, workflow eller prosedyre?
  → Ja → Skill (SKILL.md)

Er det en enkel engangsoppgave med forhåndsdefinert kontekst?
  → Ja → Prompt (.prompt.md)
```

---

## Vanlige feil

| Feil | Problem | Bedre løsning |
|------|---------|---------------|
| Agent uten verktøyrestriksjon | Gir bare kunnskap, ingen faktisk begrensning | Skill, som er mer portabel og lastes on-demand |
| Skill for noe som alltid skal gjelde | Utviklere glemmer å aktivere den | Instruksjon, som ikke trenger å aktiveres |
| Instruksjon med 500 linjer workflow | For mye kontekst i hvert svar | Skill, som lastes kun ved behov |
| Samme innhold i agent og skill | Dobbeltvedlikehold, drift | Velg én. Agenten kan referere til `/skill-name` |

---

## Hva vi har gjort i navikt

- **Lagt til `code-review.instructions.md`** med review-regler for sikkerhet, NAIS-konfig, GitHub Actions og testdekning. Gjelder automatisk.
- **Identifisert duplikater.** Flere agenter og skills har overlappende innhold. Se [issue #252](https://github.com/navikt/copilot/issues/252) for oppryddingsplan.
- **Beholdt agenter med reell verktøykontroll.** `@aksel-agent` (Figma MCP), `@nav-pilot` (Opus + orkestrator), `@security-champion` (Opus + rådgiver).

---

## Kom i gang

VS Code har innebygde kommandoer for å generere tilpasninger:

- `/create-instruction` lager en instruksjon for kodestandarder
- `/create-skill` lager en skill for workflows
- `/create-agent` lager en agent med verktøykontroll
- `/create-prompt` lager en prompt for enkeltoppgaver

Start med instruksjoner. Legg til skills for workflows teamet gjentar. Bruk agenter bare når du trenger verktøykontroll eller modellvalg.

---

## Kilder

- [VS Code: Customization concepts](https://code.visualstudio.com/docs/copilot/concepts/customization), offisiell oversikt over alle tilpasningstyper
- [VS Code: Custom instructions](https://code.visualstudio.com/docs/copilot/customization/custom-instructions), instruksjoner og glob-mønstre
- [VS Code: Agent skills](https://code.visualstudio.com/docs/copilot/customization/agent-skills), skills-format og bruk
- [VS Code: Custom agents](https://code.visualstudio.com/docs/copilot/customization/custom-agents), agenter, verktøy og handoffs
- [Agent Skills specification](https://agentskills.io), åpen standard for skills (Anthropic)
- [Best practices for skill creators](https://agentskills.io/skill-creation/best-practices), hvordan skrive gode skills
- [GitHub Blog: How to write a great agents.md](https://github.blog/ai-and-ml/github-copilot/how-to-write-a-great-agents-md-lessons-from-over-2500-repositories/), analyse av 2 500+ repoer
