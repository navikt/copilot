---
title: "Agentstyring i GitHub Issues — godkjenninger, konfidens og begrunnelser"
date: 2026-07-23
category: copilot
excerpt: "Issues viser nå hvorfor agenten gjør endringer og lar deg godkjenne dem før de trer i kraft. Tre nye funksjoner: godkjenninger, konfidensnivåer og begrunnelser."
url: "https://github.blog/changelog/2026-07-23-agent-automation-controls-in-github-issues-in-public-preview"
tags:
  - agentic
  - issues
  - automations
  - governance
---

Agent-automatiseringer som merker, tilordner og lukker issues får nå innebygd styring direkte i GitHub Issues. Tre nye funksjoner er i public preview:

## Godkjenninger (Approvals)

Du kan sette opp automatiseringen til å *foreslå* endringer i stedet for å gjennomføre dem direkte. Forslagene venter i et panel på issuet, og du godkjenner eller avslår enkeltvis eller samlet.

## Konfidensnivåer

Agenten klassifiserer hver handling som høy, middels eller lav konfidens. Høy-konfidens-endringer gjennomføres automatisk. Middels og lav holdes som forslag til manuell gjennomgang.

## Begrunnelser (Rationale)

Alle handlinger — automatiske eller ventende — loggføres med begrunnelse. Du får et revisjonsspor over hva som ble endret og hvorfor, og kan se resonnementet bak hvert forslag før du tar en beslutning.

Bruk `has:suggestions` i issue-søk for å finne issues med ventende forslag. Repository-admins kan konfigurere terskelverdier for konfidens per repository.

**Kilde:** [Agent automation controls in GitHub Issues in public preview](https://github.blog/changelog/2026-07-23-agent-automation-controls-in-github-issues-in-public-preview) (GitHub Changelog, 23. juli 2026)
