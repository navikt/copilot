---
title: "Personlige Copilot-tilpasninger som følger deg mellom prosjekter"
date: 2026-03-20
category: praksis
excerpt: "Skills og agenter kan lagres i ~/.copilot/ og brukes på tvers av alle repoer. CLI støtter også globale instruksjoner."
tags:
  - customizations
  - skills
  - agents
  - instructions
  - copilot-cli
  - vscode
---

Copilot-tilpasninger trenger ikke leve i repoet. Skills og agenter kan lagres i hjemmemappa di og gjenbrukes på tvers av alle repoer. CLI støtter også personlige instruksjoner.

---

## Hva støttes hvor?

| Tilpasningstype   | Reponivå                          | Personnivå                                     | Org/enterprise         |
| ----------------- | --------------------------------- | ---------------------------------------------- | ---------------------- |
| **Skills**        | `.github/skills/`                 | `~/.copilot/skills/`                           | Kommer snart           |
| **Agenter**       | `.github/agents/`                 | `~/.copilot/agents/`                           | `.github-private` repo |
| **Instruksjoner** | `.github/copilot-instructions.md` | `~/.copilot/copilot-instructions.md` (kun CLI) | Kommer snart           |
| **Prompts**       | `.github/prompts/`                | Ikke støttet                                   | Ikke støttet           |

Skills på personnivå fungerer i VS Code, Copilot CLI og kodingsagenten. Agenter fungerer i VS Code og CLI. Instruksjoner er foreløpig bare støtta i CLI.

---

## Personlige skills

Lag en mappe under `~/.copilot/skills/` med en `SKILL.md`-fil, på samme måte som i `.github/skills/`:

```text
~/.copilot/skills/
└── my-debug-skill/
    ├── SKILL.md
    └── examples/
```

Copilot velger skills ut fra `description`-feltet i frontmatter, uavhengig av hvilket repo du jobber i. Repo-skills og personlige skills fungerer side om side.

VS Code leser også fra `~/.claude/skills/` og `~/.agents/skills/`. Du kan legge til flere stier med innstillinga `chat.agentSkillsLocations`.

**Kilder:**

- [About agent skills](https://docs.github.com/en/copilot/concepts/agents/coding-agent/about-agent-skills) (GitHub Docs)
- [Use Agent Skills in VS Code](https://code.visualstudio.com/docs/copilot/customization/agent-skills) (VS Code Docs, 18. mars 2026)

---

## Personlige agenter

Agenter i `~/.copilot/agents/` er tilgjengelige i alle repoer. I VS Code oppretter du dem via **Configure Custom Agents > Create new custom agent > User profile**. I CLI bruker du `/agent` > **Create new agent** > **User (~/.copilot/agents/)**, eller lager fila direkte.

Ved navnekonflikt vinner personnivået — en agent i `~/.copilot/agents/` overstyrer en med samme navn i `.github/agents/`.

VS Code-agenter støtter handoffs: styrte overganger der du hopper fra en agent til en annen med kontekst og forhåndsutfylt prompt. En personlig planleggingsagent kan sende videre til en implementeringsagent i repoet.

**Kilder:**

- [Creating and using custom agents for GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/use-copilot-cli#use-custom-agents) (GitHub Docs)
- [Custom agents in VS Code](https://code.visualstudio.com/docs/copilot/customization/custom-agents) (VS Code Docs, 18. mars 2026)

---

## Personlige instruksjoner (kun CLI)

Copilot CLI leser instruksjoner fra `~/.copilot/copilot-instructions.md`. Disse slås sammen med repo-instruksjoner når du jobber i et repo.

Med miljøvariabelen `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` kan du peke CLI til flere mapper. Den leter etter `AGENTS.md` og `*.instructions.md`-filer i hver av dem.

VS Code og kodingsagenten på GitHub.com har foreløpig ikke personnivå for instruksjoner — der er de fortsatt repo-spesifikke.

**Kilde:** [Adding custom instructions for GitHub Copilot CLI](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/use-copilot-cli/copilot-cli-custom-instructions) (GitHub Docs)

---

## Hva med prompts?

Prompt-filer (`.prompt.md`) støttes foreløpig bare på reponivå. Det finnes ingen `~/.copilot/prompts/`-ekvivalent. Trenger du gjenbrukbare arbeidsflyter på tvers av repoer, er skills et bedre valg — de er portable og følger den åpne standarden fra [agentskills.io](https://agentskills.io/).

---

## Repo-tilpasninger først

Personlige tilpasninger er nyttige, men vi anbefaler å starte med repo-tilpasninger i `.github/`. Grunnen er enkel: det du legger i repoet, deler du med teamet. Personlige tilpasninger lever bare på maskina di.

Det betyr at:

- Andre i teamet får ikke tilgang til skills og agenter du har i `~/.copilot/`
- Kodingsagenten på GitHub.com ser bare repo-tilpasninger
- Tilpasningene er ikke versjonskontrollerte og blir ikke code-reviewet
- Ved navnekonflikt overstyrer personnivået repoet, noe som kan gi ulik oppførsel for ulike utviklere

Personlige tilpasninger passer for ting som faktisk er personlige: en debug-skill du bruker overalt, en planleggingsagent for din egen arbeidsflyt, eller eksperimentering før du foreslår noe for teamet.

Tommelfingerregel: har flere enn deg nytte av tilpasninga, hører den hjemme i repoet.

---

GitHub har annonsert støtte for deling av agenter og skills på organisasjons- og enterprisenivå, men dette er foreløpig ikke tilgjengelig for alle. Vi følger opp i [issue #123](https://github.com/navikt/copilot/issues/123).

---

## Praktisk: Kom i gang

1. Lag `~/.copilot/skills/` og `~/.copilot/agents/` om de ikke finnes
2. Flytt generelle skills/agenter som ikke er repo-spesifikke dit
3. Sjekk at de dukker opp: i VS Code, åpne **Configure Custom Agents** eller skriv `/skills list` i CLI
4. Bruk `chat.agentSkillsLocations` og `chat.agentFilesLocations` i VS Code for å legge til andre stier

```bash
mkdir -p ~/.copilot/skills ~/.copilot/agents
```

---

## Relevans for Nav

| Trend                                | Hva det betyr for Nav                                                                                              |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------ |
| Personlige skills og agenter         | Utviklere kan ha egne arbeidsflyt-tilpasninger uten å fylle opp team-repoet                                        |
| navikt/copilot-skills som personlige | Våre skills fra [navikt/copilot](https://github.com/navikt/copilot) kan installeres globalt i `~/.copilot/skills/` |
| Org-nivå (kommer)                    | Når organisasjonsnivå blir tilgjengelig, kan Nav distribuere felles agenter og skills sentralt                     |
| CLI med globale instruksjoner        | Utviklere som bruker Copilot CLI kan ha personlige kodestandarder i `~/.copilot/copilot-instructions.md`           |
