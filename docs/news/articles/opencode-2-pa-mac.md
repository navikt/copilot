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

Du kan nå bruke OpenCode 2 med nav-pilot på macOS, og nå også på Linux (se under). Bruker du OpenCode 1, trenger du ikke gjøre noe.

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

## Slik kommer du i gang på Linux

1. Oppdater cplt. Kjør `cplt --version`: tallet etter `cplt` må være `2026.10.08-092800` eller høyere.
2. Installer bubblewrap (`bwrap`). Mangler det, nekter cplt å starte OpenCode 2 og sier hvordan du installerer det.
3. Bytter du fra OpenCode 1, kjører du `opencode auth import` én gang.

Er Linux-kjernen eldre enn 6.7, skriver cplt en advarsel: den kan ikke begrense hvilke porter økten når, så bare passordet til OpenCode-tjenesten beskytter den.

Mer om klientene finner du på [Klienter](/nav-pilot/klienter).
