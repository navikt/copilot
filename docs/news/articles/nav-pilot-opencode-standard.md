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

```bash
nav-pilot --client opencode           # én økt
nav-pilot config set client opencode  # for godt
nav-pilot config set client copilot   # tilbake til Copilot CLI
```

nav-pilot starter nå opencode på nye installasjoner. Har du brukt nav-pilot før, endrer ingenting seg: du beholder klienten du har, og Copilot CLI er fortsatt fullt støttet.

## Hvorfor opencode

- **Åpen kildekode.** Vi kan lese koden, melde feil og rette dem selv.
- **Passer med resten.** opencode virker med mange modeller og leverandører, og med agentpakkene, hooks og MCP-reglene i nav-pilot. Maskering av hemmeligheter og fødselsnumre, løkkevakt og Navs MCP-register gjelder i opencode som i Copilot CLI.
- **Lokale modeller.** Bare i opencode kan hovedagenten kjøre på en skymodell og sende avgrensede jobber til en lokal modell på maskinen din (`local-worker`). I Copilot CLI kjører en økt enten helt lokalt eller helt i skyen. Stoppet som hindrer hovedagenten i å redigere selv (`local_dispatch`), finnes også bare i opencode.

Hva hver klient kan, og hva som fortsatt mangler i opencode, står på [Klienter](/nav-pilot/klienter).

## Hvem det gjelder

- **Ny installasjon:** Første gang du kjører `nav-pilot` i en terminal, er opencode forhåndsvalgt. Mangler opencode og du har Homebrew, tilbyr nav-pilot å installere den. Ellers bruker nav-pilot Copilot CLI og viser hvordan du bytter senere.
- **Har du brukt nav-pilot før,** trenger du ikke gjøre noe. En `~/.nav-pilot/config.toml` uten `client` betyr Copilot CLI.
- **CI og kjøring uten terminal** bruker fortsatt Copilot CLI. Vil du ha opencode der, bruk `--client opencode`.

**Kilder:**

- [opencode is the default client on new installs](https://github.com/navikt/copilot/pull/1159) (navikt/copilot, 28. september 2026)
- [Final parity status on /nav-pilot/klienter](https://github.com/navikt/copilot/pull/1148) (navikt/copilot, 28. september 2026)
