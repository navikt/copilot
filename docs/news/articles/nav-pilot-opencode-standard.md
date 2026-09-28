---
title: "opencode er standardklient for nye nav-pilot-installasjoner"
date: 2026-09-28
author: starefossen
category: nav-pilot
cli: true
excerpt: "Installerer du nav-pilot for første gang, starter den opencode. Har du brukt nav-pilot før, beholder du klienten du har, og Copilot CLI er fortsatt fullt støttet."
tags:
  - nav-pilot
  - opencode
  - copilot-cli
---

nav-pilot starter nå opencode på nye installasjoner. Har du brukt nav-pilot før, endrer ingenting seg: du beholder klienten du har, og Copilot CLI er fortsatt fullt støttet.

Du velger klient selv når du vil:

```bash
nav-pilot --client opencode           # én økt
nav-pilot config set client opencode  # for godt
nav-pilot config set client copilot   # tilbake til Copilot CLI
```

## Hvorfor opencode

Bare i opencode kan hovedagenten kjøre på en skymodell og sende avgrensede jobber til en lokal modell på maskinen din, underagenten `local-worker`. Stoppet som hindrer hovedagenten i å redigere selv (`local_dispatch`), finnes også bare der. I Copilot CLI kjører en økt enten helt lokalt eller helt i skyen.

Før byttet har vi tettet hullene mellom de to klientene. opencode har nå de samme vaktene som Copilot CLI:

- maskering av hemmeligheter og fødselsnumre i verktøyresultater
- løkkevakt, og merknad om prompt-injeksjon i verktøyresultater
- gates du har installert, også agentpakke-hooks
- `tools:`-begrensningene i agentene
- Navs MCP-register: nav-pilot slår av servere utenfor registeret
- deling til opencode.ai er slått av i hver økt

Hva hver klient kan, og hva som fortsatt mangler i opencode, står på [Klienter](/nav-pilot/klienter).

## Ny installasjon

Første gang du kjører `nav-pilot` i en terminal, spør den hvilken klient du vil bruke, med opencode forhåndsvalgt. Mangler opencode og du har Homebrew, tilbyr nav-pilot å installere den med `brew install anomalyco/tap/opencode`. Uten Homebrew, eller om du sier nei, bruker nav-pilot Copilot CLI og viser hvordan du installerer opencode selv (`curl -fsSL https://opencode.ai/install | bash`) og bytter med `nav-pilot config set client opencode`. nav-pilot lagrer aldri en klient som ikke kan starte.

## Har du brukt nav-pilot før

Du trenger ikke gjøre noe. Klienten din står i `~/.nav-pilot/config.toml`, og en fil uten `client` betyr Copilot CLI. Vil du prøve opencode, bruk kommandoene øverst.

## I CI og uten terminal

Ingenting endres. Uten `config.toml` starter nav-pilot fortsatt Copilot CLI, siden den ikke kan se om du har brukt nav-pilot før. Vil du ha opencode i CI, bruk `--client opencode`.

**Kilder:**

- [opencode is the default client on new installs](https://github.com/navikt/copilot/pull/1159) (navikt/copilot, 28. september 2026)
- [Final parity status on /nav-pilot/klienter](https://github.com/navikt/copilot/pull/1148) (navikt/copilot, 28. september 2026)
