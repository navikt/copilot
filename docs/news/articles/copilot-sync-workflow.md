---
title: "Automatisk synkronisering av Copilot-tilpasninger"
date: 2026-04-06
category: nav
excerpt: "Ny reusable workflow holder agents, instructions, prompts og skills i repoet ditt oppdatert mot navikt/copilot. Åtte linjer YAML, ingen secrets."
tags:
  - customizations
  - sync
  - github-actions
  - workflow
---

Vi har over 35 tilpasningsfiler i [navikt/copilot](https://github.com/navikt/copilot) — agents, instructions, prompts og skills. Mange team har kopiert deler av disse inn i sine egne repoer. Problemet er at filene oppdateres jevnlig, og da blir kopiene utdaterte.

Nå finnes det en sync-workflow som fikser dette. Den fungerer som Dependabot, men for Copilot-filer i stedet for pakker.

## Kom i gang

Opprett `.github/workflows/copilot-sync.yml` i repoet ditt:

```yaml
name: Copilot Customization Sync
on:
  schedule:
    - cron: '0 7 * * 1'  # Mandager kl. 07:00 UTC
  workflow_dispatch:
jobs:
  sync:
    uses: navikt/copilot/.github/workflows/copilot-customization-sync.yml@main
    permissions:
      contents: write
      pull-requests: write
```

Ferdig. Ingen tokens, ingen secrets, ingen konfigurasjon.

## Hva skjer?

Workflowen kjører ukentlig og gjør tre ting:

1. Finner alle Copilot-filer i repoet ditt
2. Sammenligner SHA-256-hasher mot siste versjon i `navikt/copilot`
3. Åpner en PR hvis noe er utdatert

PR-en havner på branchen `copilot-customization-sync` med tittel som _«chore: sync 3 Copilot customization(s)»_. Du ser diffen og bestemmer selv om du merger.

## Hvilke filer sjekkes?

Workflowen sjekker alle Copilot-filer som allerede finnes i repoet ditt:

- `.github/agents/*.agent.md`
- `.github/instructions/*.instructions.md`
- `.github/prompts/*.prompt.md`
- `.github/skills/*/SKILL.md` og `metadata.json`

`AGENTS.md` og `.github/copilot-instructions.md` synces aldri fordi de alltid er repospesifikke.

## Bare bestemte filer?

Opprett `.github/copilot-sync.json` for å begrense hva som sjekkes:

```json
{
  "files": [
    ".github/agents/nais-platform.agent.md",
    ".github/instructions/kotlin-ktor.instructions.md"
  ]
}
```

Uten denne fila sjekkes alt som finnes i repoet.

## Hva om jeg har gjort lokale endringer?

PR-en viser diffen. Du kan merge selektivt, redigere PR-en, eller lukke den. Workflowen tvinger ingenting — den åpner bare PR-er.

## Oppdateringer kommer automatisk

Workflowen er en [reusable workflow](https://docs.github.com/en/actions/sharing-automations/reusing-workflows). Den refereres med `@main`, så forbedringer vi gjør i `navikt/copilot` treffer alle team ved neste kjøring uten at dere trenger å endre noe.

## Mer informasjon

Se [dokumentasjonen for sync-workflowen](https://github.com/navikt/copilot/blob/main/docs/README.sync.md) for tekniske detaljer og FAQ.
