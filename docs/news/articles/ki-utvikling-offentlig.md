---
title: "ki-utvikling.nav.no — nå åpen for alle"
date: 2026-05-05
category: nav
excerpt: "Nyheter, beste praksis, verktøy og ordliste er nå tilgjengelig for alle utviklere — ikke bare Nav-ansatte."
tags:
  - nav-internal
  - public
  - launch
---

Siden lanseringen har Min Copilot vært et internt verktøy bak Navs bedriftsinnlogging. Nå åpner vi dørene. Fra i dag er mesteparten av innholdet tilgjengelig for alle på **ki-utvikling.nav.no** — uten innlogging.

## Hva er åpent?

Alt som handler om å bli bedre med AI-kodeverktøy er nå offentlig:

- **Nyheter** — oppdateringer om modeller, verktøy og funksjoner
- **Beste praksis** — retningslinjer for bevisst og effektiv AI-bruk
- **Verktøy** — MCP-servere, agenter, instruksjoner og skills
- **Ordliste** — begreper og forkortelser i Copilot-økosystemet
- **Nav-pilot** — brukerveiledning for Copilot CLI

## Hva krever fortsatt innlogging?

Data som er spesifikke for Nav krever fortsatt innlogging:

- Bruksstatistikk og adopsjon
- Kostnader og fakturering
- Abonnementshåndtering
- Kalkulator

Nav-ansatte logger inn via den vanlige knappen — Azure AD gir automatisk SSO på Nav-nettet, så det er null ekstra klikk.

## Hvorfor åpne?

Vi deler erfaringene våre fordi:

1. **Gjenbruk** — andre offentlige virksomheter kan bruke det vi har lært
2. **Åpenhet** — Nav skal være åpne om hvordan vi bruker AI-verktøy
3. **Samarbeid** — vi inviterer til bidrag fra utviklermiljøet

## Teknisk

Begge domenene (`ki-utvikling.nav.no` og `min-copilot.ansatt.nav.no`) serveres av samme applikasjon. Appen håndterer autentisering — offentlige sider serveres fritt, mens private sider sender deg til innlogging.

Den gamle adressen `min-copilot.ansatt.nav.no` fungerer som før for Nav-ansatte som har bokmerket den.
