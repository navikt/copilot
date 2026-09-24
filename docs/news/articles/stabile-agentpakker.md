---
title: "Agentpakker følger stabile releases"
date: 2026-09-24
author: starefossen
category: nav-pilot
excerpt: "Nav-pilot samler standardoppsettet i én agentpakke og lar pakker publisere stabile releases med kontrollerte oppdateringer."
tags:
  - nav-pilot
  - agentpakker
  - releases
  - customizations
---

Nav-pilot har erstattet de fem gamle samlingene med én standardpakke. Samtidig har agentpakker fått stabile releases og støtte for Copilot, opencode og pi.

En agentpakke kan inneholde agenter, skills, instruksjoner, prompts, hooks og extensions. Manifestet sier hvilke klienter pakka støtter, hvilken persona som starter og hvor innholdet ligger.

## Pakker kan følge GitHub Releases

En pakke kan publisere `agentpakke-release.json` i en stabil GitHub Release merket `immutable`. Når releasen oppfyller [release-kontrakten](../../README.agentpakke.md#stabile-releases), kan `nav-pilot install` og `nav-pilot sync` følge den i stedet for standardbranchen. Eksplisitte revisjonsvalg og nedgraderingsvernet gjelder fortsatt.

Payload-baserte pakker som lagres med en lokal pinne kan også spørre ved oppstart. Da kan du velge:

- oppdater nå
- oppdater nå og ta senere releases automatisk ved oppstart i en terminal
- hopp over denne versjonen og spør igjen når en nyere kommer
- behold dagens revisjon og slutt å spørre

Valget lagres per pakke. Tier 1-pakker oppdateres fortsatt gjennom `nav-pilot sync`; de har ikke oppstartsspørsmålet eller et varig oppdateringsvalg.

`nav-pilot list --installed` viser hvilken versjon som er installert, og om pakka følger releases.

## Pinner og rollback

Payload-baserte pakker lagres lokalt med eksakt commit-SHA. `nav-pilot sync --apply` flytter pinnen først etter at den nye revisjonen er verifisert.

Hvis den nye revisjonen gir problemer, kan du gå ett steg tilbake uten nett, så lenge forrige revisjon fortsatt ligger på maskinen og består verifiseringen:

```bash
nav-pilot rollback
```

Rollback gjelder i dag bare pakker som bruker en lokal payload-pinne. Tier 1-pakker, som installerer filer direkte i et scope, støtter ikke denne kommandoen.

## Samme pakke på tre klienter

Agentpakken kan deklarere Copilot, opencode og pi i samme manifest. Hver klient kan ha egen standardmodell og egne primære personaer.

Du kan velge en annen deklarert persona ved start:

```bash
nav-pilot --client opencode --persona nav-pilot-opus
```

Agentfiler som ikke er deklarert som primære personaer, materialiseres som subagenter. En pakke kan også levere bare skills eller instruksjoner uten å finne på en agent.

## Standardpakken er samlet

Standardoppsettet i `navikt/copilot` er nå én pakke: `nav-pilot`. Du velger fortsatt bort enkeltartefakter ved installasjon, men sync, modellvalg og klientstøtte styres samlet.

Dette reduserer forskjellen mellom klientene. Samme pakke beskriver hva Copilot, opencode og pi skal få, mens nav-pilot håndterer klientenes ulike filformater og launch-flagg.

**Kilder:**

- [Kollaps de fem samlingene til én pakke](https://github.com/navikt/copilot/pull/670) (navikt/copilot, 5. september 2026)
- [Agentpakker følger stabile releases](https://github.com/navikt/copilot/pull/780) (navikt/copilot, 11. september 2026)
- [Install og status følger stabile releases](https://github.com/navikt/copilot/pull/785) (navikt/copilot, 11. september 2026)
- [Rull en pinnet agentpakke tilbake uten nett](https://github.com/navikt/copilot/pull/836) (navikt/copilot, 13. september 2026)
- [Pi får agentpakken som de andre klientene](https://github.com/navikt/copilot/pull/812) (navikt/copilot, 13. september 2026)
