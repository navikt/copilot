---
title: "Parallell utvikling med /fleet og Git worktrees"
date: 2026-04-21
author: starefossen
category: praksis
excerpt: "Slik bruker du Copilot CLI sin /fleet-kommando og Git worktrees til å kjøre flere AI-agenter samtidig — uten filkonflikter og med full isolasjon."
tags:
  - copilot-cli
  - fleet
  - git-worktrees
  - parallell-utvikling
---

Copilot CLI kan kjøre flere agenter samtidig. `/fleet` bryter ned oppgaver i parallelle deloppgaver innenfor én sesjon. Git worktrees gir fullstendig isolasjon mellom separate sesjoner — hvert arbeidsområde har sin egen branch og sine egne filer. Sammen gir de to teknikkene rask utvikling uten venting.

## /fleet — parallelle agenter i terminalen

`/fleet` er en innebygd kommando i Copilot CLI som fungerer som en prosjektleder for AI-agenter. I stedet for å jobbe sekvensielt, gjør den følgende:

1. **Deler opp** oppgaven din i uavhengige deloppgaver
2. **Analyserer avhengigheter** — hva kan kjøres samtidig, hva må vente
3. **Starter sub-agenter** i parallell, hver med eget kontekstvindu
4. **Samler resultatene** når alle agentene er ferdige

### Eksempel

```
/fleet Oppdater autentisering, skriv tester, og oppdater dokumentasjonen

  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
  │ Agent 1      │   │ Agent 2      │   │ Agent 3      │
  │ Refaktorer   │   │ Skriv tester │   │ Oppdater     │
  │ auth-modul   │   │ for auth     │   │ docs/auth/   │
  └──────┬───────┘   └──────┬───────┘   └──────┬───────┘
         │                  │                   │
         └──────────────────┼───────────────────┘
                            ▼
                   Orkestrator setter sammen
```

Konkrete deloppgaver gir best resultat. Vage instruksjoner som «fiks koden» paralleliseres dårlig — spesifikke filer og mål gir reell samtidighet.

### Når /fleet er nyttig

| Scenario | Eksempel |
|---|---|
| Dokumentasjon i flere filer | `/fleet Lag docs for API: auth.md, endpoints.md, errors.md` |
| Refaktorering + tester | `/fleet Flytt utils til egen pakke og oppdater tester` |
| Flere uavhengige bugfikser | `/fleet Fiks feil #42 i auth og #43 i API-ruten` |

## Git worktrees — fullstendig isolasjon

`/fleet` deler kontekst men jobber i samme filsystem. For full isolasjon — der to agenter kan bygge, teste og endre uavhengig — trenger du Git worktrees.

En worktree er en ekstra arbeidskopi av repoet ditt, koblet til en annen branch, men med delt Git-objektdatabase. Ingen ekstra full kloning — historikk og referanser deles, men arbeidskopifilene finnes i hver worktree.

### Oppsett

```bash
# Opprett worktrees for to parallelle oppgaver
git worktree add -b feature/auth-refaktor ../mitt-repo-auth
git worktree add -b bugfix/api-feil ../mitt-repo-api

# Terminal 1 — auth-arbeid
cd ../mitt-repo-auth
copilot

# Terminal 2 — API-fiks
cd ../mitt-repo-api
copilot
```

Hver Copilot-sesjon jobber i sitt eget arbeidsområde. Ingen direkte filkollisjoner mellom sesjoner, ingen `git stash`, ingen tapt kontekst.

### Rydde opp

```bash
# Fjern worktreeet når du er ferdig (må være clean, ellers bruk --force)
git worktree remove ../mitt-repo-auth
git worktree remove ../mitt-repo-api
```

## Praktiske erfaringer

Worktrees og AI-agenter sammen gir merkbart bedre flyt:

- **Du venter aldri på agenten** — start en ny oppgave i en ny worktree mens den forrige kjører. Utviklere som bruker dette mønsteret rapporterer [merkbart høyere gjennomstrømming](https://easyappdev.com/blog/git-worktrees-ai-coding) (anekdotisk).
- **Ingen konteksttap** — branch-bytte kan gjøre agentens kontekst inkonsistent. Separate worktrees bevarer tilstanden for hver sesjon.
- **Trygt å eksperimentere** — en agent som gjør feil i sin worktree påvirker ikke de andre. Kast worktreen og start på nytt.
- **Naturlig code review-flyt** — hver worktree produserer sin egen branch som blir en egen PR. Lettere å gjennomgå og merge enn én gigantisk endring.

## Anbefalt arbeidsflyt for Nav-utviklere

For de fleste daglige oppgaver i ett repo:

```bash
# Bruk /fleet direkte — enklest
copilot
> /fleet Implementer endpoint, skriv tester, oppdater OpenAPI-spec
```

For større oppgaver der agentene trenger å bygge og teste uavhengig:

```bash
# Opprett worktree per oppgave
git worktree add -b feature/oppgave-a ../oppgave-a
git worktree add -b feature/oppgave-b ../oppgave-b

# Kjør Copilot i hver
cd ../oppgave-a && copilot
cd ../oppgave-b && copilot

# Merge tilbake fra hoved-worktreen
cd ../mitt-repo   # tilbake til hoved-worktree med main
git merge feature/oppgave-a
git merge feature/oppgave-b
git worktree remove ../oppgave-a
git worktree remove ../oppgave-b
```

Med sandboxing via [`cplt`](https://min-copilot.ansatt.nav.no/nyheter/cplt-sandbox-copilot-cli):

```bash
cd ../oppgave-a && cplt -- -p "implementer autentisering"
cd ../oppgave-b && cplt -- -p "fiks API-validering"
```

## Verdt å vite

- **Premium requests**: `/fleet` kan øke forbruket av premium requests fordi sub-agentene gjør egne modellkall. Følg med på [forbruket ditt](https://github.com/settings/copilot) om du kjører mange samtidige oppgaver.
- **VS Code-integrasjon**: VS Code kan starte og overvåke Copilot CLI-sesjoner direkte fra Chat-panelet, inkludert fleet-oppgaver. Se [VS Code-dokumentasjonen](https://code.visualstudio.com/docs/copilot/agents/copilot-cli).

## Kilder

### Offisiell dokumentasjon

- [Running tasks in parallel with /fleet — GitHub Docs](https://docs.github.com/en/copilot/concepts/agents/copilot-cli/fleet)
- [Copilot CLI in VS Code — Visual Studio Code Docs](https://code.visualstudio.com/docs/copilot/agents/copilot-cli)
- [git-worktree — Git Reference](https://git-scm.com/docs/git-worktree)
- [Run Multiple Agents at Once with Fleet — GitHub Blog](https://github.blog/ai-and-ml/github-copilot/run-multiple-agents-at-once-with-fleet-in-copilot-cli/)

### Artikler og erfaringer

- [Workspace vs Worktree Isolation in Copilot CLI — Ken Muse](https://www.kenmuse.com/blog/workspace-vs-worktree-isolation-in-copilot-cli/)
- [Git Worktree: The Infrastructure That Unlocks Agentic Development — htek.dev](https://htek.dev/articles/git-worktree-unlocks-agentic-development/)
- [Git Worktrees for AI Coding: Run Multiple Agents in Parallel — EasyAppDev](https://easyappdev.com/blog/git-worktrees-ai-coding)
