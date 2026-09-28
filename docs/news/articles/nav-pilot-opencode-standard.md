---
title: "opencode er standardklient for nye nav-pilot-installasjoner"
date: 2026-09-28
author: starefossen
category: nav-pilot
cli: true
excerpt: "Installerer du nav-pilot for første gang, starter den opencode. Har du brukt nav-pilot før, beholder du Copilot CLI."
tags:
  - nav-pilot
  - opencode
  - copilot-cli
---

nav-pilot starter nå opencode på nye installasjoner. Har du brukt nav-pilot før, endrer ingenting seg: du beholder Copilot CLI, som fortsatt er fullt støttet.

## Hvorfor opencode

Bare i opencode kan hovedagenten kjøre på en skymodell og sende avgrensede jobber til en lokal modell på maskinen din. Stoppet som hindrer hovedagenten i å redigere selv (`local_dispatch`), finnes også bare der. I Copilot CLI kjører en økt enten helt lokalt eller helt i skyen.

Før byttet har vi tettet hullene mellom de to klientene. I opencode har du nå de samme vaktene som i Copilot CLI:

- maskering av hemmeligheter og fødselsnumre i verktøyresultater
- løkkevakten og merknaden om instruksjoner i verktøyresultater
- gates du har installert, også agentpakke-hooks
- `tools:`-begrensningene i agentene
- Navs MCP-register, som nav-pilot håndhever selv
- deling til opencode.ai er slått av i hver økt

Hva hver klient kan, og hva som fortsatt mangler i opencode, står på [Klienter](/nav-pilot/klienter).

## Ny installasjon

Første gang du kjører `nav-pilot` i en terminal, spør den hvilken klient du vil bruke, med opencode forhåndsvalgt. Mangler opencode, og du har Homebrew, tilbyr nav-pilot å installere den. Uten Homebrew, eller om du sier nei, bruker nav-pilot Copilot CLI og forteller hvordan du bytter senere. nav-pilot lagrer aldri en klient som ikke kan starte.

## Har du brukt nav-pilot før

Du trenger ikke gjøre noe. Klienten din står i `~/.nav-pilot/config.toml`, og en fil uten `client` betyr Copilot CLI. Vil du prøve opencode:

```bash
nav-pilot --client opencode          # én økt
nav-pilot config set client opencode # for godt
```

## I CI og uten terminal

I CI og andre kjøringer uten terminal endres ingenting. Uten `config.toml` starter nav-pilot fortsatt Copilot CLI, siden den ikke kan se om du har brukt nav-pilot før. Vil du ha opencode i CI, bruk `--client opencode`.
