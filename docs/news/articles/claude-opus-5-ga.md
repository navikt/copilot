---
title: "Claude Opus 5 er tilgjengelig i GitHub Copilot"
date: 2026-07-24
category: copilot
excerpt: "Anthropics nye toppmodell Claude Opus 5 er generelt tilgjengelig (GA) i Copilot samme dag som lanseringen — med justerbar innsatskontroll (effort) og near-frontier ytelse til samme pris som Opus 4.8."
url: "https://github.blog/changelog/2026-07-24-claude-opus-5-is-now-available-in-github-copilot/"
tags:
  - models
  - claude
  - coding-agents
---

Anthropic lanserte Claude Opus 5 (`claude-opus-5`) fredag 24. juli 2026, og modellen ble generelt tilgjengelig (GA) i GitHub Copilot samme dag. Opus 5 er Anthropics nye toppmodell og posisjoneres som «near-frontier» — tett opp mot de sterkeste modellene på markedet, men til en vesentlig lavere kostnad per oppgave.

## Nøkkelfakta

| Egenskap | Detalj |
| --- | --- |
| **Kategori** | Powerful |
| **Status** | GA |
| **Modell-ID** | `claude-opus-5` |
| **Input-pris** | $5 per 1M tokens (samme som Opus 4.8) |
| **Output-pris** | $25 per 1M tokens (samme som Opus 4.8) |
| **Fast mode** | 2x grunnpris, ~2,5x raskere |
| **Tilgjengelig for** | Pro+, Max, Business, Enterprise (ikke Pro/Free) |
| **Standard for Business/Enterprise** | Av — må aktiveres av administrator |

## Justerbar innsatskontroll (effort)

Hovednyheten er justerbar innsatskontroll (effort control). Du kan styre hvor hardt modellen jobber — fra lave nivåer for enkle oppgaver til høyere nivåer som `xhigh` og `max` for det tyngste arbeidet. I praksis lar dette deg bytte mellom kostnad og kapasitet på samme modell: lavere innsats gir raskere og rimeligere svar, høyere innsats gir mer resonnering når oppgaven krever det.

Dette gjør Opus-klassen mer håndterbar kostnadsmessig enn tidligere, fordi du slipper å betale for maksimal resonnering på oppgaver som ikke trenger det.

## Ytelse

Anthropic oppgir at Opus 5 på max effort ligger innenfor 0,5 % av Fable 5s toppnivå på CursorBench 3.2 — til rundt halve kostnaden per oppgave — og at den overgår Fable 5 på OSWorld 2.0 til om lag en tredjedel av kostnaden. Sammenlignet med Opus 4.8 mer enn dobler den resultatet på Frontier-Bench v0.1 og oppnår rundt 3x på ARC-AGI 3. På Zapier AutomationBench ligger den rundt 1,5x over nest beste modell.

## Tilgjengelighet i Copilot

Opus 5 rulles ut gradvis og er tilgjengelig i VS Code, Visual Studio, Copilot CLI, GitHub Copilot cloud agent, Copilot-appen, github.com, GitHub Mobile, JetBrains, Xcode og Eclipse. Modellen faktureres til leverandørens API-listepris under bruksbasert fakturering (usage-based billing).

Copilot Business- og Enterprise-administratorer må aktivere Claude Opus 5-policyen i Copilot-innstillingene før utviklere kan velge modellen. Opus 4.8 er fortsatt tilgjengelig — lanseringen medfører ingen avvikling.

Merk: Opus 5 har innebygde ekstra sikkerhetstiltak mot høyrisiko cyber-innhold, som i noen tilfeller kan blokkere sikkerhetsrelaterte forespørsler.

## Hva dette betyr for Nav

For arbeid der vi tidligere har anbefalt Claude Opus 4.6/4.8 — dyp risikovurdering og sikkerhetskritisk kode der resonnering og nyanserte vurderinger teller mest — er Opus 5 det nye førstevalget når du trenger det ypperste. Opus 4.6/4.8 er fortsatt tilgjengelig for team som foretrekker dem.

Den justerbare innsatskontrollen er særlig relevant for oss: den gjør Opus-klassens prising mer forsvarlig i daglig bruk, siden du kan skru ned innsatsen på rutineoppgaver og bare bruke max effort der det faktisk gir verdi. Husk at Opus 5 kun er tilgjengelig for Pro+, Max, Business og Enterprise, og at utrullingen skjer gradvis.

**Kilder:** [Claude Opus 5](https://www.anthropic.com/news/claude-opus-5) (Anthropic, 24. juli 2026); [Claude Opus 5 is now available in GitHub Copilot](https://github.blog/changelog/2026-07-24-claude-opus-5-is-now-available-in-github-copilot/) (GitHub Changelog, 24. juli 2026)
