---
title: "Copilot impact dashboard og repo-nivå bruksmetrikker"
date: 2026-07-22
category: copilot
excerpt: "Nytt impact dashboard med adopsjonsfaser og -kohorter, plus GA for repository-nivå bruksmetrikker via REST API."
url: "https://github.blog/changelog/2026-07-22-new-copilot-usage-metrics-impact-dashboard"
tags:
  - metrics
  - enterprise-controls
  - api
---

## Impact dashboard med adopsjonskohorter

GitHub lanserer et nytt Copilot metrics impact dashboard for enterprise-administratorer og organisasjonseiere. Dashbordet grupperer engasjerte brukere etter AI-adopsjonsfase og viser nøkkelmetrikker for hver kohort:

- **Adopsjonsfaser:** Phase 1 (Code-first), Phase 2 (Agent-first) og Phase 3 (Multi-agent/Copilot app), pluss et passivt segment (lisensiert men ikke engasjert).
- **Per-kohort-metrikker:** Gjennomsnittlig antall mergede PR-er per bruker per måned, median merge-hastighet, antall brukere, andel av totalen, og gjennomsnittlig antall kodelinjer per dag.
- **Adopsjonsmultiplikator:** En sammenligning av gjennomstrømning og hastighet mellom passive og engasjerte brukere.
- **Trender:** Diagrammer som viser kohortvekst og PR-gjennomstrømning over seks måneder.

## Repository-nivå bruksmetrikker (GA)

Copilot usage metrics REST API rapporterer nå aktivitet på repository-nivå. To nye endepunkter gir daglig nedbrytning per repository av PR-aktivitet for coding agent og code review:

- `GET /enterprises/{enterprise}/copilot/metrics/reports/repos-1-day?day=YYYY-MM-DD`
- `GET /orgs/{org}/copilot/metrics/reports/repos-1-day?day=YYYY-MM-DD`

Responsen inkluderer PR-er opprettet og merget av coding agent, og PR-er gjennomgått av code review med antall forslag fordelt etter kommentartype.

**Kilder:**

- [New Copilot usage metrics impact dashboard](https://github.blog/changelog/2026-07-22-new-copilot-usage-metrics-impact-dashboard) (GitHub Changelog, 22. juli 2026)
- [Repository-level GitHub Copilot usage metrics generally available](https://github.blog/changelog/2026-07-17-repository-level-github-copilot-usage-metrics-generally-available) (GitHub Changelog, 17. juli 2026)
- [GitHub Copilot app now available in the usage metrics API](https://github.blog/changelog/2026-07-17-github-copilot-app-now-available-in-the-usage-metrics-api) (GitHub Changelog, 17. juli 2026)
