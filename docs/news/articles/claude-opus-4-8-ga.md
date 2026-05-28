---
title: "Claude Opus 4.8 er generelt tilgjengelig i GitHub Copilot"
date: 2026-05-28
category: copilot
excerpt: "Opus 4.8 er et tydelig steg fremover for kodeforståelse, fikser tool-calling-svakheter og gir mer pålitelig agentisk arbeid."
url: "https://github.blog/changelog/2026-05-28-claude-opus-4-8-is-generally-available-for-github-copilot/"
tags:
  - models
  - claude
  - copilot
---

Claude Opus 4.8 er nå tilgjengelig som modellvalg i GitHub Copilot. Det er en direkte oppgradering fra Opus 4.7 som fikser kjente regresjoner.

## Viktige forbedringer

- **Bedre feildeteksjon** — Tredjeparts-benchmarks rapporterer opptil 4× færre uoppdagede feil sammenlignet med 4.7. GitHub beskriver det som «a clear step forward in code understanding».
- **Pålitelig tool-calling** — 4.8 fullfører flerstegs agentisk arbeid mer konsekvent enn 4.7
- **Mindre verbose** — Bedre kontroll over output-lengde, spesielt i agentiske arbeidsflyter
- **Samme pris** — $5/$25 per million tokens (input/output), ingen prisendring

## Hva vi har gjort

nav-pilot og security-champion er oppgradert fra Opus 4.6 til 4.8. Du får bedre arkitekturforslag, mer pålitelig kodegjennomgang og færre avbrutte agentsesjoner.

## Premium-modell

Opus 4.8 bruker premium-multiplikator mot Copilot-kvoten (se [GitHub sin modelltabell](https://docs.github.com/en/copilot/about-github-copilot/github-copilot-plans-and-billing) for gjeldende sats). Fra 1. juni gjelder bruksbasert fakturering der du betaler per token i stedet for en fast kvote.

## Anbefaling

Bruk Opus 4.8 for:
- Kompleks arkitekturplanlegging (`@nav-pilot`)
- Sikkerhetsgjennomgang (`@security-champion`)
- Flerstegs refaktorering

For enklere oppgaver (autocomplete, enkle spørsmål) er GPT-5.4 mini eller Haiku 4.5 mer kostnadseffektive.
