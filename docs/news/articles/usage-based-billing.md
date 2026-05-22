---
title: "Copilot går over til bruksbasert fakturering 1. juni"
date: 2026-04-27
author: starefosen
category: copilot
excerpt: "Premium requests erstattes av GitHub AI Credits basert på tokenforbruk — planpriser forblir uendret."
url: "https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/"
tags:
  - billing
  - enterprise-controls
  - breaking-change
---

Fra 1. juni 2026 erstattes premium request-enheter (PRU) med **GitHub AI Credits**. Forbruket beregnes per token — input, output og cached input — etter publiserte API-rater for hver modell. 1 AI Credit = $0.01.

## Hva endrer seg

| | Før (PRU) | Etter (AI Credits) |
| --- | --- | --- |
| Enhet | Premium request (flat) | Tokens × modellpris |
| Inkludert | Fast antall PRU | 1 900 credits/bruker (Business) |
| Overskudd | Fallback til billig modell | Blokkeres eller koster tillegg |
| Pooling | Per bruker | Per organisasjon |

Planprisene er uendret — Business koster $19/bruker/mnd, Enterprise $39/bruker/mnd. Code completions og Next Edit er fortsatt gratis.

## Tre token-typer

Hver interaksjon består av:

1. **Input tokens** — det du sender (prompt, kontekst, filer)
2. **Output tokens** — det modellen genererer
3. **Cached input tokens** — kontekst som modellen gjenbruker fra sesjonen

Cached input koster **90 % mindre** enn vanlig input. Eksempel:

| Modell | Input/1M | Cached input/1M | Output/1M |
| --- | --- | --- | --- |
| GPT-5.3-Codex | $1.75 | $0.175 | $14.00 |
| Claude Sonnet 4.6 | $3.00 | $0.30 | $15.00 |
| Claude Opus 4.6 | $5.00 | $0.50 | $25.00 |

Anthropic-modeller har i tillegg en **cache write**-kostnad (25 % over vanlig input) første gang konteksten skrives.

## Token cache — hva vi vet

- Cachen gjelder **innenfor sesjonen** — lukker du sesjonen, betaler du full pris igjen
- Auto model selection velger modell langs «naturlige cache-grenser» for å unngå ekstra cachekostnader
- Modellbytte midt i sesjonen invaliderer cachen
- GitHub dokumenterer **ingen cache-TTL** — i praksis varer cachen så lenge sesjonen er aktiv
- Ut fra provider-atferd (Anthropic: 5 min default TTL, OpenAI: varierer) kan cachen falle bort i inaktive sesjoner

## Promokreditter juni–august

Eksisterende Business-kunder får 3 000 credits/bruker/mnd (i stedet for 1 900) i overgangsperioden juni–august 2026.

## Kilder

- [GitHub Copilot is moving to usage-based billing](https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/) (GitHub Blog, april 2026)
- [Models and pricing for GitHub Copilot](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing) (GitHub Docs)
- [Usage-based billing for organizations and enterprises](https://docs.github.com/en/copilot/concepts/billing/usage-based-billing-for-organizations-and-enterprises) (GitHub Docs)
- [About Copilot auto model selection](https://docs.github.com/en/copilot/concepts/auto-model-selection) (GitHub Docs)
