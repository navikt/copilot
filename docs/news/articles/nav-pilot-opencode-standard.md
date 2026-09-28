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

nav-pilot starter nå opencode på nye installasjoner. Har du brukt nav-pilot før, skjer det ingenting med deg: du beholder Copilot CLI, og Copilot CLI er fortsatt fullt støttet.

## Hvorfor opencode

Bare i opencode kan hovedagenten kjøre på en skymodell og sende avgrensede jobber til en lokal modell på maskinen din. Stoppet som hindrer hovedagenten i å gjøre jobben selv (`local_dispatch`), finnes også bare der. I Copilot CLI kjører en økt enten helt lokalt eller helt i skyen.

Før byttet har vi tettet hullene mellom de to klientene. I opencode har du nå de samme vaktene som i Copilot CLI:

- maskering av hemmeligheter og fødselsnumre i verktøyresultater
- løkkevakten og merknaden om instruksjoner i verktøyresultater
- gatene du har installert, også agentpakke-hooks
- `tools:`-begrensningene i agentene
- Navs MCP-register, som nav-pilot håndhever selv
- deling til opencode.ai er slått av i hver økt

Hva hver klient kan, og hva som fortsatt mangler i opencode, står på [Klienter](/nav-pilot/klienter).

## Ny installasjon

Første gang du kjører `nav-pilot`, spør den hvilken klient du vil bruke, og opencode er valgt. Mangler opencode, tilbyr nav-pilot å installere den med Homebrew. Uten Homebrew, eller om du sier nei, bruker nav-pilot Copilot CLI og forteller hvordan du bytter senere. nav-pilot lagrer aldri en klient som ikke kan starte.

## Har du brukt nav-pilot før

Du trenger ikke gjøre noe. Klienten din står i `~/.nav-pilot/config.toml`, og en fil uten `client` betyr Copilot CLI. Vil du prøve opencode:

```bash
nav-pilot --client opencode          # én økt
nav-pilot config set client opencode # for godt
```

## I CI

Uten `config.toml` starter nav-pilot opencode når den er installert, og ellers Copilot CLI, med én linje på stderr om hvorfor. Vil du være sikker på klienten, bruk `--client copilot` eller `--client opencode`.
