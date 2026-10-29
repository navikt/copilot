---
title: "Nais MCP får sin egen sandkasse i cplt"
date: 2026-10-29
author: starefossen
category: nav-pilot
cli: true
excerpt: "cplt kjører Nais MCP i en egen sandkasse, utenfor sandkassen til agenten. Agenten får aldri tilgang til nøkkelringen. Oppdater cplt og nav-pilot og slå på Nais MCP på nytt."
tags:
  - nav-pilot
  - cplt
  - mcp
  - sikkerhet
---

Gjør dette:

```bash
brew upgrade navikt/tap/nav-pilot navikt/tap/cplt   # skriptinstallasjon: nav-pilot upgrade && cplt update
nav-pilot mcp enable io.github.nais/nais-mcp
nav-pilot mcp list                                  # PATH viser «cplt broker»
```

Start en ny økt etterpå. nav-pilot fra før 24. oktober 2026 avviser Nais MCP-oppføringen i registeret, så oppgrader først.

## Hva er nytt

Fra 24. oktober kjørte cplt Nais MCP utenfor sandkassen til agenten ([forrige artikkel](/nyheter/mcp-i-nav-pilot)). Nå kjører serveren også i en egen sandkasse, som et barn av cplt. Den kan bruke Nais-innloggingen din, men sandkassen til agenten får ikke tilgang til nøkkelringen.

- **macOS:** Serveren kjører i en egen Seatbelt-profil. Den når nettet bare gjennom sin egen proxy, og bare til vertene i serverens oppsett (`egressHosts`). Av systemtjenestene når den bare securityd, som håndterer nøkkelringen, og de grunnleggende tjenestene alle programmer får.
- **Linux:** Serveren kjører i bubblewrap med eget nettverk og en tom hjemmemappe. Den ser bare Secret Service på D-Bus, ikke resten av bussen.
- **Innloggingssjekken** kjører i den samme sandkassen på begge systemer. `nav-pilot mcp enable` spør cplt om du er logget inn, i stedet for å sjekke selv utenfor sandkassen.
- **Bare lesing.** Nais MCP får bare lesende verktøy som standard, og cplt avviser GraphQL-mutasjoner.
- **Logg per kall.** Hvert verktøykall gir én linje i loggfila du gir med `--proxy-log`: server, verktøy, utfall og tid. Argumenter og svar logges ikke.

`nav-pilot doctor` varsler nå også når en lokal MCP-server på maskinen din (for eksempel på `127.0.0.1`) ikke svarer.

## Dette er ikke på plass ennå

- **GitHub MCP** går ennå ikke gjennom den samme brokeren ([cplt#909](https://github.com/navikt/cplt/issues/909)).
- **Figma** med OAuth gjennom cplt er planlagt ([cplt#910](https://github.com/navikt/cplt/issues/910)).
- **Restrisiko:** Serveren kjører kode vi stoler på, men med argumenter agenten velger. Den kan lese det du kan lese og sende det til vertene den har lov til å nå. På Linux kan en server som bruker Secret Service lese alle oppføringer som er låst opp i økten din, ikke bare sine egne. Se [SECURITY.md i cplt](https://github.com/navikt/cplt/blob/main/SECURITY.md).

**Kilder:**

- navikt/cplt: [#862](https://github.com/navikt/cplt/pull/862), [#873](https://github.com/navikt/cplt/pull/873), [#914](https://github.com/navikt/cplt/pull/914), [#915](https://github.com/navikt/cplt/pull/915), [#932](https://github.com/navikt/cplt/pull/932)
- navikt/copilot: [#2053](https://github.com/navikt/copilot/pull/2053), [#2061](https://github.com/navikt/copilot/pull/2061), [#2063](https://github.com/navikt/copilot/pull/2063)
