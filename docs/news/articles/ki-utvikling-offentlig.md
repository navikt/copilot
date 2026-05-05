---
title: "ki-utvikling.nav.no — nå åpen for alle"
date: 2026-05-05
category: nav
excerpt: "Nyheter, god praksis, verktøy og retningslinjer er nå tilgjengelig for alle utviklere — ikke bare Nav-ansatte."
tags:
  - nav-internal
  - public
  - launch
---

Fra i dag er mesteparten av innholdet på **ki-utvikling.nav.no** tilgjengelig for alle, uten innlogging.

Oh-My-Nav har til nå vært et internt verktøy. Kildekoden har ligget åpent, men den kjørende versjonen krevde Nav-innlogging. Etter hvert som vi har bygd ut nyheter, god praksis og verktøyoversikter, har flere påpekt at innholdet er nyttig for alle som jobber med AI-kodeverktøy — ikke bare Nav-ansatte.

## Hva er åpent?

Alt som handler om å bli bedre med AI-kodeverktøy:

- **Nyheter**: oppdateringer om modeller, verktøy og funksjoner
- **God praksis**: tre nivåer (Grunnmur → Arbeidsmodus → Ekspertnivå) med konkrete råd
- **Retningslinjer**: hva som er tillatt, inkludert agent-modus, MCP-servere, coding agent og bevisst AI-bruk
- **Kom i gang**: installasjonsveiledning med `brew install copilot-cli`, `nav-pilot` og `cplt`
- **Verktøy**: MCP-servere, agenter, instruksjoner og skills
- **Modeller og kostnader**: tilgjengelige modeller, premium requests og multiplikatorer
- **Nav-pilot**: brukerveiledning for Nav sin egen utvikleragent

## Hva er fortsatt internt?

Bruksstatistikk, adopsjonsoversikter, kostnader og abonnementshåndtering krever fortsatt Nav-innlogging på min-copilot.ansatt.nav.no. Disse tallene er bare relevante for oss, men vi skriver gjerne artikler som refererer til dem — følg med når vi snakker om adopsjon eller kostnadsutvikling.

## Nytt innhold

Samtidig med lanseringa har vi oppdatert mye:

**Retningslinjer** dekker nå agent-modus, MCP-servere fra Nav-godkjent registry, coding agent for feature branches, og tilpassede instruksjoner via `.github/copilot-instructions.md`. Vi har også lagt til et rammeverk for bevisst AI-bruk med rød og grønn sone, basert på forskning om kompetansebevaring.

**Kom i gang** er skrevet om med Mac først:

1. `brew install copilot-cli` — GitHub Copilot i terminalen
2. `brew install navikt/tap/nav-pilot` og `brew install navikt/tap/cplt` — Nav-verktøy
3. Kjør `nav-pilot` for interaktiv oppsettveiviser

Andre installasjonsmetoder (Linux, manuell, IntelliJ, VS Code) ligger i sammenleggbare seksjoner.

**Modeller og kostnader** er oppdatert med gjeldende modellutvalg. Modellene som er inkludert (GPT-5 mini, GPT-4.1, GPT-4o) bruker ingen premium requests. GitHub går over til bruksbasert prising, og vi forventer omtrent 3× kostnadsøkning.

## Hvorfor dele dette?

Vi åpner av samme grunn som alltid: offentlig finansierte løsninger bør være offentlig tilgjengelige. Koden til [Nais](https://nais.io), [Aksel](https://aksel.nav.no) og tusenvis av andre repoer ligger åpent på GitHub. Retningslinjene i [navikt/offentlig](https://github.com/navikt/offentlig) beskriver filosofien: koden vi skriver implementerer lovene Stortinget vedtar — da bør den være like tilgjengelig som lovene selv.

Konkret håper vi at:

- andre offentlige virksomheter som ruller ut AI-kodeverktøy kan lære av erfaringene våre — hva som fungerer, og hvilke praksiser som hjelper utviklere bruke verktøyene ansvarlig
- utviklermiljøet utenfor Nav kan bidra med perspektiver vi ikke har tenkt på
- åpenhet om hvordan vi bruker AI-verktøy gir innsyn i en teknologi mange lurer på

Vi vil også trekke fram Oslo kommune, som har delt sine erfaringer åpent under [Din brolagte sti for kunstig intelligens](https://ki.utvikler.oslo.kommune.no/).
