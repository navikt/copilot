---
title: "Copilot code review: tilpasning, brannmur og head branch-instruksjoner"
date: 2026-07-17
category: copilot
excerpt: "Code review leser nå instruksjoner fra feature-branchen, støtter REVIEW.md/GEMINI.md/CLAUDE.md, har brannmur og egne runner-innstillinger."
url: "https://github.blog/changelog/2026-07-17-copilot-code-review-customization-and-configurability-improvements"
tags:
  - code-review
  - agentic
  - enterprise-controls
---

Copilot code review får fire forbedringer som gir utviklere og administratorer mer kontroll:

## Instruksjoner fra head branch

Custom instructions leses nå fra *head branch* i stedet for base branch. Det inkluderer `copilot-instructions.md`, `*.instructions.md`, agent skills og `AGENTS.md`. Du kan dermed teste og iterere på instruksjoner i en feature branch uten å måtte merge dem først.

## Utvidet filstøtte

Code review leser nå også `REVIEW.md`, `GEMINI.md` og `CLAUDE.md` fra repositoriet — slik at tilpasningene fungerer uavhengig av hvilken instruksjonsfil-konvensjon teamet bruker.

## Brannmur

Copilot code review kjører nå bak en brannmur, noe som gir administratorer kontroll over nettverkstilgangen under kodevurderingsprosessen.

## Egne runner-innstillinger

Organisasjoner kan nå konfigurere runnere for Copilot code review uavhengig av andre CI-workflows, og definere egne setup steps for kodevurderingens kjøremiljø.

**Kilde:** [Copilot code review: Customization and configurability improvements](https://github.blog/changelog/2026-07-17-copilot-code-review-customization-and-configurability-improvements) (GitHub Changelog, 17. juli 2026)
