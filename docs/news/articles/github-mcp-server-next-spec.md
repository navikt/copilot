---
title: "GitHub MCP Server støtter neste MCP-spesifikasjon"
date: 2026-07-23
category: copilot
excerpt: "GitHub MCP Server blir stateless 28. juli 2026 — raskere handshakes uten sesjoner, verdier leses fra HTTP-headere, og oppgradert elicitation fungerer for både gamle og nye klienter."
url: "https://github.blog/changelog/2026-07-23-github-mcp-server-supports-the-next-mcp-specification/"
tags:
  - mcp
  - integrations
  - infrastructure
---

GitHub MCP Server støtter nå den neste MCP-spesifikasjonen. Protokollen går over til en stateless arkitektur 28. juli 2026, og GitHubs MCP-server er allerede tilpasset. En stateless kjerne gjør at MCP-deployeringer blir enkle å skalere.

## Hva som endres

- **Sesjoner og initialisering fjernes:** GitHub har fjernet Redis-sesjoner og databaseoperasjoner ved oppstart. Det gir raskere klient–server-oppkobling med parallelle handshakes.
- **Verdier leses fra HTTP-headere:** Deep packet inspection er borte; serveren leser nødvendige verdier direkte fra HTTP-headerne i stedet for å analysere selve forespørselen.
- **Oppgradert elicitation:** Implementasjonen støtter både eldre og nyere klienter gjennom en wrapper i Go SDK-en.

## Trenger du å gjøre noe?

Nei. Alle tier 1-SDK-er har bevart bakoverkompatibilitet og har allerede sluppet beta-støtte, så du trenger ikke gjøre noe for å beholde støtten. MCP-prosjektet har i tillegg innført offisielle conformance-tester for å validere egne implementasjoner.

**Kilde:** [GitHub MCP Server supports the next MCP specification](https://github.blog/changelog/2026-07-23-github-mcp-server-supports-the-next-mcp-specification/) (GitHub Changelog, 23. juli 2026)
