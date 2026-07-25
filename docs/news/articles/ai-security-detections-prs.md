---
title: "AI-sikkerhetsfunn på pull requests og CodeQL 2.26.0 med prompt-injeksjon"
date: 2026-07-14
category: copilot
excerpt: "Code scanning viser nå AI-drevne sikkerhetsfunn direkte på PR-er for språk uten CodeQL-dekning. CodeQL 2.26.0 legger til Kotlin 2.4.0-støtte og deteksjon av prompt-injeksjon."
url: "https://github.blog/changelog/2026-07-14-code-scanning-shows-ai-security-detections-on-pull-requests"
tags:
  - security
  - codeql
  - code-scanning
  - kotlin
---

## AI-drevne sikkerhetsfunn på pull requests

GitHub code scanning viser nå AI-drevne sikkerhetsfunn direkte på pull requests. Funksjonaliteten utvider sårbarhetsdekninger til språk og rammeverk som ikke dekkes av CodeQLs innebygde analyse — slik at blinde flekker i kodebasen reduseres.

Funn vises i PR-visningen som vanlige code scanning-varsler, merket med «AI» slik at du enkelt kan skille dem fra CodeQL-resultater. Funksjonen må aktiveres på enterprise-nivå først, deretter på organisasjon eller repository.

## CodeQL 2.26.0: Kotlin 2.4.0 og prompt-injeksjon

CodeQL 2.26.0 bringer tre viktige oppdateringer:

- **Kotlin 2.4.0-støtte** — CodeQL analyserer nå Kotlin-kode opp til versjon 2.4.0.
- **Prompt-injeksjon for JavaScript/TypeScript** — ny query som oppdager system prompt injection-sårbarheter, svært relevant ettersom flere team bygger AI-integrasjoner.
- **Go `log/slog`-modeller** — `go/log-injection` og `go/clear-text-logging` kan nå oppdage problemer i kode som bruker slog-pakken.

**Kilder:**

- [Code scanning shows AI security detections on pull requests](https://github.blog/changelog/2026-07-14-code-scanning-shows-ai-security-detections-on-pull-requests) (GitHub Changelog, 14. juli 2026)
- [CodeQL 2.26.0 adds Kotlin 2.4.0 support and AI prompt injection detection](https://github.blog/changelog/2026-07-10-codeql-2-26-0-adds-kotlin-2-4-0-support-and-ai-prompt-injection-detection) (GitHub Changelog, 10. juli 2026)
