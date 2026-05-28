---
title: "Copilot Memory får bedre slettekontroll, repo-avslåing og CLI-støtte"
date: 2026-05-26
category: copilot
excerpt: "Copilot Memory støtter nå repo-nivå av/på, tydeligere scope ved lagring og /memory-kommando i CLI."
url: "https://github.blog/changelog/2026-05-26-copilot-memory-has-more-controls-for-deletion-scope-and-the-copilot-cli"
tags:
  - copilot-memory
  - copilot-cli
  - enterprise-controls
---

GitHub har oppdatert Copilot Memory med tre forbedringer som gir bedre kontroll over hva Copilot husker.

## Hva er nytt

### Repo-nivå av/på

Repo-administratorer kan nå slå av memory per repo under **Settings → Copilot → Memory**. Nyttig for repos med sensitiv kode der du ikke vil at Copilot lagrer fakta.

### `/memory`-kommando i CLI

Copilot CLI har fått nye kommandoer:

- `/memory show` — vis lagrede fakta for gjeldende kontekst
- `/memory off` — slå av memory for resten av sesjonen

### Tydeligere scope ved sletting

Slettedialogene viser nå eksplisitt om du sletter en personlig, repo- eller organisasjons-memory.

## Hva dette betyr for Nav

**Ingen org-level memory ennå.** Instruksjonsfiler (`.github/copilot-instructions.md`, agents, skills) er fortsatt den eneste måten å dele kunnskap på tvers av et team. Memory er personlig eller repo-nivå.

**28-dagers utløp.** Fakta som ikke brukes på 28 dager forsvinner automatisk. Memory er ikke en erstatning for dokumentasjon.

**Memory vs instruksjoner.** Når både memory og instruksjonsfiler dekker samme tema, avgjør modellen selv hva den vektlegger. Det finnes ingen eksplisitt prioritering.

## Hva du bør gjøre

- **Sensitiv kode?** Vurder å slå av memory i repoet.
- **Feil fakta lagret?** Bruk `/memory show` og slett via [github.com/settings/copilot/memory](https://github.com/settings/copilot/memory).
- **Team-kunnskap?** Bruk instruksjonsfiler og skills, ikke memory. De er versjonskontrollerte og delte.
