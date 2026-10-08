---
title: "OpenCode 2 fungerer nå med nav-pilot på Mac"
date: 2026-10-08
author: starefossen
category: nav-pilot
cli: true
excerpt: "Du kan nå bruke OpenCode 2 med nav-pilot på macOS. Oppdater cplt først."
tags:
  - nav-pilot
  - opencode
---

Du kan nå bruke OpenCode 2 med nav-pilot på macOS.

## Slik kommer du i gang

1. Oppdater cplt til versjonen fra 8. oktober 2026 eller nyere:

   ```sh
   brew upgrade cplt
   ```

   Eldre versjoner av cplt nekter å starte OpenCode 2.

2. Bytter du fra OpenCode 1, kjører du denne kommandoen én gang for å ta med innloggingen:

   ```sh
   opencode auth import
   ```

## Linux og OpenCode 1

På Linux må du bli på OpenCode 1 inntil videre. Støtte for OpenCode 2 kommer senere.

Bruker du OpenCode 1, trenger du ikke gjøre noe.

Mer om klientene finner du på [Klienter](/nav-pilot/klienter).
