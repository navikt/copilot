---
title: "MCP-servere nå tilgjengelig fra verktøysiden"
date: 2026-03-10
author: starefosen
category: nav
excerpt: "Verktøysiden viser nå MCP-servere fra Navs MCP-register — med installasjonsinstruksjoner, verktøyliste og direkte CLI-kommandoer for VS Code."
tags:
  - mcp
  - mcp-registry
  - developer-tools
  - nav-internal
---

Verktøysiden på my-copilot har fått en ny seksjon: **MCP-servere**. Serverne hentes dynamisk fra [Navs MCP-register](https://github.com/navikt/copilot/tree/main/apps/mcp-registry) og vises sammen med agenter, instruksjoner og skills.

## Seks servere i registeret

| Server                   | Hva den gjør                                                           |
| ------------------------ | ---------------------------------------------------------------------- |
| **GitHub MCP**           | Tilgang til repos, issues og PRs via GitHub API                        |
| **Nav MCP Onboarding**   | Utforsk Copilot-tilpasninger, sjekk agent-readiness, generer AGENTS.md |
| **Figma MCP**            | Hent designkontekst fra Figma inn i koden                              |
| **Next.js DevTools MCP** | Diagnostikk, dokumentasjon og feilmeldinger fra Next.js dev-server     |
| **Svelte MCP**           | Søk i Svelte 5- og SvelteKit-dokumentasjon                             |
| **Playwright MCP**       | Browser-automatisering for testing og debugging                        |

## Installasjon rett fra verktøysiden

Klikk på en MCP-server for å se detaljer — inkludert verktøy, tags og installasjonsinstruksjoner. For servere med npm-pakker får du en ferdig `code --add-mcp`-kommando du kan kopiere rett inn i terminalen. HTTP-baserte servere viser også `gh copilot mcp add`-kommandoen.

Playwright MCP er forhåndskonfigurert med Nav-spesifikke sikkerhetsregler: isolert browser, blokkerte Nav-domener og trace-logging.

## Teknisk

MCP-registeret følger [MCP Registry v0.1-spesifikasjonen](https://modelcontextprotocol.io) og er tilgjengelig som REST API på `/v0.1/servers`. Verktøysiden henter data via dette APIet og presenterer serverne i samme katalog som øvrige Copilot-tilpasninger.
