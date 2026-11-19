---
title: "MCP-gatewayen i pilot med Felles kodeverk og Dolly"
date: 2026-11-19
author: starefossen
category: nav-pilot
cli: true
excerpt: "Navs interne MCP-gateway er i pilot med to servere som bare leser: Felles kodeverk og Dolly. Du logger inn med Nav-kontoen, og oppsettet virker i Copilot CLI, OpenCode og andre MCP-klienter."
tags:
  - nav-pilot
  - mcp
  - kodeverk
  - dolly
---

KI-agenter kan nå hente data fra Navs egne systemer, ikke bare fra koden. Den nye MCP-gatewayen gjør det mulig. Den er i pilot, og de to første tjenestene er klare: Felles kodeverk og Dolly.

Gatewayen er fellesinngangen for MCP-servere i Nav. Brukeren logger inn med Nav-kontoen, og hvert kall logges med hvem som spurte. Teamene bestemmer selv hva agentene får se, og trenger ikke bygge innlogging selv. Det legger grunnen for agenter som jobber med Navs egne data på en trygg måte, og flere tjenester kan kobles på etter hvert.

![Copilot CLI eller OpenCode på utviklermaskinen logger inn og holder tokenene selv, og sender kallet til fleet-gateway med innlogging via Entra. Gatewayen henter et OBO-token fra Texas og kaller teamets MCP-server, som sjekker tokenet.](/images/mcp-gateway-kall.svg)

- **Felles kodeverk** er Navs felles lister over koder som systemene bruker, for eksempel postnummer, land, valuta og ytelsestyper. Med serveren `io.github.navikt/kodeverk` kan agenten slå opp hva en kode betyr, for eksempel at postnummer `0150` er OSLO.
- **Dolly** er Navs verktøy for å lage syntetiske testpersoner i testmiljøene. Med serveren `io.github.navikt/dolly` kan agenten finne testpersonene dine og se om en bestilling gikk gjennom i hvert system.

Begge serverne leser bare.

Kom i gang slik:

```bash
nav-pilot mcp enable kodeverk
nav-pilot mcp enable dolly
```

Oppføringene og oppsettet for andre klienter står på [ki-utvikling.nav.no/verktoy](https://ki-utvikling.nav.no/verktoy) og i guiden [Koble til Navs MCP-servere](/nav-pilot/guider/nav-mcp-servere).

## Hva gatewayen gjør

Gatewayen står foran interne MCP-servere på Nais og tar seg av innloggingen med Entra ID. Du logger inn i nettleseren med Nav-kontoen første gang KI-agenten kaller en server. Teamene som lager servere, slipper å bygge OAuth selv.

Oppsettet virker i Copilot CLI, OpenCode og andre MCP-klienter. I pilotperioden er gatewayen bare tilgjengelig internt via naisdevice. Gatewayen revisjonslogger hvert kall med NAVident, rute, MCP-metode og verktøy.

## Dette kan serverne


- **Felles kodeverk (`io.github.navikt/kodeverk`).** Slå opp publiserte kodeverk, koder og betydninger i Nav, for eksempel at postnummer `0150` er `OSLO`.
- **Dolly (`io.github.navikt/dolly`).** List opp testgruppene dine i Dolly, se identene til syntetiske testpersoner og sjekk bestillingsstatus per fagsystem, som `PDL`, `TESTNORGE` og `PEN`. Du trenger en gruppe med testpersoner i Dolly fra før. Verktøy for å opprette testdata kommer senere, når grupper har tilgangskontroll.

Takk til team-rocket (Felles kodeverk) og team Dolly for samarbeidet om tilgang og oppsett.

## Tilby teamets egen MCP-server

Har teamet ditt en tjeneste på Nais som dere vil tilby som MCP-server, står krav, tokenvalidering og fremgangsmåte i [veiledningen for team på ki-utvikling.nav.no/verktoy/fleet-gateway/for-team](https://ki-utvikling.nav.no/verktoy/fleet-gateway/for-team).

## Hvilke systemer ønsker du deg?

Begrepskatalogen er allerede etterspurt. Del ønsker om andre interne systemer i [#ki-produktutvikling-diskusjon](https://nav-it.slack.com/archives/C0ASJSZK8P8) på Slack.

**Kilder:**

- navikt/copilot: [#2276](https://github.com/navikt/copilot/issues/2276), [#2429](https://github.com/navikt/copilot/issues/2429), [#2457](https://github.com/navikt/copilot/issues/2457)
