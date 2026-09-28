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

nav-pilot starter nå opencode på nye installasjoner. Har du brukt nav-pilot før, beholder du klienten du har. Copilot CLI er fortsatt fullt støttet.

## Hvorfor opencode

- **Åpen kildekode.** Vi kan lese koden og melde feil der de hører hjemme.
- **Passer med resten.** opencode virker med mange modeller og leverandører, og med agentpakkene, hooks og Navs MCP-register i nav-pilot. Når nav-pilot starter opencode, gjelder maskering av hemmeligheter og fødselsnumre og løkkevakten som i Copilot CLI. Med `--pure` kjører ingen hooks.
- **Lokale modeller.** Bare i opencode kan hovedagenten kjøre på en skymodell og sende avgrensede jobber til en lokal modell på maskinen din (`local-worker`). I Copilot CLI kjører en økt enten helt lokalt eller helt i skyen. Stoppet som griper inn når hovedagenten redigerer for mye selv (`local_dispatch` på `balanced` og `aggressive`), finnes også bare i opencode.

Hva hver klient kan, og hva som fortsatt mangler i opencode, står på [Klienter](/nav-pilot/klienter).

## Hvem det gjelder

- **Ny installasjon:** første gang du kjører `nav-pilot` i en terminal, er opencode forhåndsvalgt. Mangler den og du har Homebrew, tilbyr nav-pilot å installere den med `brew install anomalyco/tap/opencode`. Ellers får du Copilot CLI og beskjed om hvordan du bytter.
- **Har du brukt nav-pilot før,** trenger du ikke gjøre noe. En `~/.nav-pilot/config.toml` uten `client` betyr Copilot CLI.
- **CI og kjøring uten terminal** bruker Copilot CLI når `config.toml` mangler. Vil du ha opencode der, bruk `--client opencode`.

**Kilder:**

- [opencode is the default client on new installs](https://github.com/navikt/copilot/pull/1159) (navikt/copilot, 28. september 2026)
- [Final parity status on /nav-pilot/klienter](https://github.com/navikt/copilot/pull/1148) (navikt/copilot, 28. september 2026)
- [Proposal: make OpenCode the default nav-pilot client](https://github.com/navikt/copilot/issues/1022) (navikt/copilot, 27. september 2026)
