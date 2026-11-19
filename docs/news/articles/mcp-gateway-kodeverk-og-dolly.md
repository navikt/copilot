---
title: "MCP-gatewayen i pilot med Felles kodeverk og Dolly"
date: 2026-11-19
author: starefossen
category: nav-pilot
cli: true
excerpt: "Navs interne MCP-gateway er i pilot med to skrivebeskyttede servere: Felles kodeverk og Dolly. Du logger inn med Nav-kontoen, og oppsettet virker i Copilot CLI, OpenCode og andre MCP-klienter."
tags:
  - nav-pilot
  - mcp
  - kodeverk
  - dolly
---

Nav har en intern MCP-gateway i pilot. To skrivebeskyttede MCP-servere er klare gjennom den: **Felles kodeverk** (`io.github.navikt/kodeverk`) og **Dolly** (`io.github.navikt/dolly`).

Kom i gang slik:

```bash
nav-pilot mcp enable kodeverk
nav-pilot mcp enable dolly
```

Du finner også oppføringene og oppsettet for andre klienter på [ki-utvikling.nav.no/verktoy](https://ki-utvikling.nav.no/verktoy) og i guiden [Koble til Navs MCP-servere](/nav-pilot/guider/nav-mcp-servere).

## Hva gatewayen gjør

Gatewayen står foran interne MCP-servere på Nais og tar seg av innloggingen med Entra ID. Du logger inn i nettleseren med Nav-kontoen første gang KI-agenten kaller en server, og teamene som lager servere, slipper å bygge OAuth selv.

Oppsettet virker i Copilot CLI, OpenCode og andre MCP-klienter. I pilotperioden er gatewayen bare tilgjengelig internt via naisdevice. Gatewayen revisjonslogger hvert kall med NAVident, rute, MCP-metode og verktøy.

## To nye MCP-servere

Begge serverne støtter bare lesing.

- **Felles kodeverk (`io.github.navikt/kodeverk`).** Slå opp publiserte kodeverk, koder og betydninger i Nav, for eksempel at postnummer `0150` er `OSLO`.
- **Dolly (`io.github.navikt/dolly`).** List opp testgruppene dine i Dolly, se identene til syntetiske testpersoner og sjekk bestillingsstatus per fagsystem, som `PDL`, `TESTNORGE` og `PEN`. Du trenger en gruppe med testpersoner i Dolly fra før. Verktøy for å opprette testdata kommer senere, når grupper har tilgangskontroll.

Takk til **team-rocket** (Felles kodeverk) og **team Dolly** for samarbeidet om tilgang og oppsett.

## Tilby teamets egen MCP-server

Har teamet ditt en tjeneste på Nais som dere vil tilby som MCP-server, står krav, tokenvalidering og fremgangsmåte i [veiledningen for team på ki-utvikling.nav.no/verktoy/fleet-gateway/for-team](https://ki-utvikling.nav.no/verktoy/fleet-gateway/for-team).

## Hvilke systemer ønsker du deg?

Vi vil gjerne høre hvilke interne systemer du ønsker deg som MCP-server neste gang. Begrepskatalogen er allerede etterspurt. Del innspill og ønsker i [#ki-produktutvikling-diskusjon](https://nav-it.slack.com/archives/C0ASJSZK8P8) på Slack.

**Kilder:**

- navikt/copilot: [#2276](https://github.com/navikt/copilot/issues/2276), [#2429](https://github.com/navikt/copilot/issues/2429), [#2457](https://github.com/navikt/copilot/issues/2457)
