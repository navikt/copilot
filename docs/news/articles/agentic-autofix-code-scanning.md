---
title: "Agentisk autofix for code scanning-varsler i public preview"
date: 2026-07-10
category: copilot
excerpt: "En AI-agent utforsker koden, foreslår og validerer rettelser, og åpner PR-er for code scanning-varsler fra CodeQL og tredjepartsverktøy."
url: "https://github.blog/changelog/2026-07-10-agentic-autofix-for-code-scanning-alerts-in-public-preview/"
tags:
  - security
  - code-scanning
  - agentic
  - cloud-agent
---

Agentisk autofix for code scanning-varsler er nå i offentlig forhåndsvisning (public preview). Funksjonen retter automatisk varsler fra CodeQL og tredjepartsverktøy: agenten utforsker relevante filer, genererer et forslag til rettelse, validerer at rettelsen fungerer ved å kjøre CodeQL på nytt, itererer om nødvendig, og åpner en utkast-PR til gjennomgang. Generering av en rettelse tar typisk 2–4 minutter.

Der `/security-review` og de AI-drevne sikkerhetsdeteksjonene finner og rapporterer sårbarheter, tar dette steget for seg *remediering* — altså selve rettingen av varslene som allerede er funnet.

## Slik utløses den

- Tildel enkeltvarsler direkte til Copilot.
- Velg flere varsler samlet i repositoryets sikkerhetslister eller i kampanjer.
- Bruk REST API for å tildele varsler til Copilot-agenten.

## Krav og forbruk

Funksjonen krever GitHub Code Security- eller Advanced Security-lisens, samt en Copilot-lisens med cloud-agent aktivert. Aktivering gjøres av organisasjons- eller repository-admin, og organisasjons- og enterprise-admins kan slå funksjonen av via innstillinger eller policy.

Under forhåndsvisningen forbruker autofix AI-kreditter (kun når rettelser faktisk kjøres) og trekker på GitHub Actions-minutter. Forbruket er foreløpig ikke skilt ut separat fra annen Copilot-aktivitet.

**Kilde:** [Agentic autofix for code scanning alerts in public preview](https://github.blog/changelog/2026-07-10-agentic-autofix-for-code-scanning-alerts-in-public-preview/) (GitHub Changelog, 10. juli 2026)
