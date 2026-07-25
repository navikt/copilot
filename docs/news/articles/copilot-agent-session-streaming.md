---
title: "Strømming av Copilot agent-sesjoner i public preview"
date: 2026-07-02
category: copilot
excerpt: "Enterprise-bred tilgang til agent-sesjonsdata — prompter, svar og verktøykall — fra cloud-agenter, CLI, VS Code, Visual Studio og partner-IDE-er, via strømming eller REST API."
url: "https://github.blog/changelog/2026-07-02-copilot-agent-session-streaming-is-now-in-public-preview/"
tags:
  - enterprise-controls
  - governance
  - observability
  - api
---

Strømming av Copilot agent-sesjoner er nå i offentlig forhåndsvisning (public preview). GitHub Enterprise Cloud-kunder med enterprise managed users får enterprise-bred innsyn i AI-bruk på tvers av Copilot-flatene — inkludert prompter, svar og verktøykall (tool calls).

## Hvilke flater dekkes

- Cloud-agenter (github.com og ghe.com)
- GitHub Copilot CLI
- Visual Studio Code og Visual Studio
- Partner-IDE-er (JetBrains, Eclipse)

## To måter å hente data på

- **Strømme-endepunkt:** Strømmer sesjonsdata automatisk til foretrukne event-collectors, SIEM-verktøy eller Microsoft Purview (i public preview).
- **REST API:** Enterprise-eiere kan hente de siste 48 timene med data på forespørsel via `GET /enterprises/{enterprise}/copilot/usage-records`.

## Oppsett

Administratorer aktiverer både «Copilot Usage Records Streaming» og «Copilot Usage Records API» under Copilot-underseksjonen i AI Controls, og velger «Enable everywhere» for hver av dem.

Dette er relevant for Navs arbeid med observabilitet og styring: det gir et konkret grunnlag for revisjonsspor og oppfølging av AI-bruk på tvers av utviklingsverktøyene.

**Kilde:** [Copilot agent session streaming is now in public preview](https://github.blog/changelog/2026-07-02-copilot-agent-session-streaming-is-now-in-public-preview/) (GitHub Changelog, 2. juli 2026)
