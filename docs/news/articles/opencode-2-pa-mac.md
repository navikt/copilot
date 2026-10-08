---
title: "OpenCode 2 fungerer nå med nav-pilot på Mac"
date: 2026-10-08
author: starefossen
category: nav-pilot
cli: true
excerpt: "Vil du bruke OpenCode 2 på macOS, må du oppdatere cplt først. Bruker du OpenCode 1, trenger du ikke gjøre noe."
tags:
  - nav-pilot
  - opencode
---

Du kan nå bruke OpenCode 2 med nav-pilot på macOS. Bruker du OpenCode 1, trenger du ikke gjøre noe. Fra 8. oktober 2026 virker OpenCode 2 også på Linux, med cplt fra samme dag eller nyere og bubblewrap installert.

## Slik kommer du i gang på Mac

1. Oppdater cplt til versjonen fra 8. oktober 2026 eller nyere:

   ```sh
   brew upgrade cplt
   ```

   Eldre versjoner av cplt nekter å starte OpenCode 2.

2. Bytter du fra OpenCode 1, kjører du denne kommandoen én gang for å ta med innloggingen:

   ```sh
   opencode auth import
   ```

## Linux

På Linux trenger du cplt der `cplt --version` viser `2026.10.08-092800` eller høyere, og bubblewrap (`bwrap`). Mangler bubblewrap, nekter cplt å starte og sier hvordan du installerer det. Med Linux-kjerne eldre enn 6.7 får du en advarsel om svakere isolasjon. Bytter du fra OpenCode 1, kjører du `opencode auth import` én gang.

Mer om klientene finner du på [Klienter](/nav-pilot/klienter).
