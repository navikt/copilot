---
title: "Nyheter og trender — April 2026"
date: 2026-04-02
draft: true
category: copilot
excerpt: "Copilot cloud agent støtter forskning og planlegging, Copilot SDK i public preview, organisasjonstilpassede instruksjoner GA, Visual Studio marsoppdatering med custom agents, CLI-bruksmetrikker per bruker, GPT-5.4 mini for Student."
tags:
  - coding-agents
  - copilot-sdk
  - instructions
  - models
  - enterprise
  - metrics
---

<!-- AI-REDAKSJONELT: Denne artikkelen er en oppsummering av de viktigste endringene og trendene — ikke en komplett liste. Prioriter det som er mest relevant for Nav-utviklere. Individuelle nyheter dekkes av egne excerpt-filer i samme mappe. -->

Starten av april 2026 bringer en viktig utvidelse av hva Copilot cloud agent kan gjøre — fra ren kodeproduksjon til forskning og planlegging. Samtidig går Copilot SDK til public preview med fem språk, organisasjonstilpassede instruksjoner når GA, og Visual Studio får custom agents og MCP-styring. Fellesnevneren: GitHub bygger ut plattformen for å gjøre Copilot til et verktøy som tilpasser seg organisasjonen, ikke omvendt.

---

## 1. Copilot cloud agent støtter forskning og planlegging

Copilot cloud agent (tidligere kjent som Copilot coding agent) er ikke lenger begrenset til å lage pull requests. Agenten støtter nå tre nye arbeidsmoduser som gjør den til et bredere verktøy for utviklere.

For det første kan agenten nå jobbe på en branch uten å opprette en PR. Du kan se hele diffen, iterere med agenten, og først opprette PR-en når du er fornøyd. For det andre kan du be om en implementeringsplan — agenten analyserer oppgaven og foreslår en tilnærming som du godkjenner eller gir tilbakemelding på før noen kode skrives. For det tredje kan du starte en «deep research»-sesjon der agenten undersøker kodebasen grundig for å svare på brede spørsmål.

Disse modusene er tilgjengelige via Agents-fanen i repoet og i Copilot Chat. For Business- og Enterprise-brukere må en administrator ha aktivert Copilot cloud agent.

**Kilde:** [Research, plan, and code with Copilot cloud agent](https://github.blog/changelog/2026-04-01-research-plan-and-code-with-copilot-cloud-agent) (GitHub Changelog, 1. april 2026)

---

## 2. Copilot SDK i public preview

GitHub Copilot SDK er nå tilgjengelig i public preview — en oppgradering fra den tidligere technical preview. SDK-en gir utviklere byggeklossene for å bygge inn Copilots agentiske kapabiliteter direkte i egne applikasjoner, workflows og plattformtjenester.

SDK-en eksponerer den samme produksjonstestede agent-runtimen som driver Copilot cloud agent og Copilot CLI. I stedet for å bygge eget AI-orkestreringsplattform får du verktøykall, streaming, filoperasjoner og flertrinnsesjoner ut av boksen. Nytt i public preview er støtte for fem språk: Node.js/TypeScript, Python, Go, .NET og Java (via Maven).

Blant nøkkelfunksjonene er custom tools og agents, finkornet system prompt-tilpasning med `replace`, `append`, `prepend` og `transform`-callbacks, blob attachments for bilder, innebygd OpenTelemetry-støtte, og et permission-rammeverk. Bring Your Own Key (BYOK) lar enterprise-kunder bruke egne API-nøkler for OpenAI, Azure AI Foundry eller Anthropic.

**Kilde:** [Copilot SDK in public preview](https://github.blog/changelog/2026-04-02-copilot-sdk-in-public-preview) (GitHub Changelog, 2. april 2026)

---

## 3. Organisasjonstilpassede instruksjoner er GA

Organisasjonstilpassede instruksjoner for GitHub Copilot, først introdusert i april 2025, er nå generelt tilgjengelige. Med denne funksjonen kan Copilot Business- og Enterprise-administratorer sette standardinstruksjoner som styrer Copilots oppførsel på tvers av alle repoer i organisasjonen.

Instruksjonene gjelder i Copilot Chat på github.com, Copilot code review og Copilot cloud agent. For eksempel kan en organisasjon instruere Copilot til å alltid bruke bestemte kodekonvensjoner, sikkerhetspolicyer, eller referere til intern dokumentasjon. Konfigureres under organisasjonens innstillinger → Copilot → Custom instructions.

**Kilde:** [Copilot organization custom instructions are generally available](https://github.blog/changelog/2026-04-02-copilot-organization-custom-instructions-are-generally-available) (GitHub Changelog, 2. april 2026)

---

## 4. GitHub Copilot i Visual Studio — marsoppdatering

Marsoppdateringen av Visual Studio 2026 bringer en stor utvidelse av Copilots utvidbarhet, med custom agents, agent skills og nye verktøy for debugging og sikkerhet.

Custom agents kan nå defineres som `.agent.md`-filer i repoet — med full tilgang til workspace-kontekst, verktøy, foretrukket modell og MCP-tilkoblinger. Enterprise MCP-styring betyr at MCP-serverbruk nå respekterer allowlist-policyer satt gjennom GitHub. Agent skills — gjenbrukbare instruksjonssett — oppdages og brukes automatisk av Copilot. Et nytt `find_symbol`-verktøy gir agenter språkbevisst symbolnavigasjon med støtte for C++, C#, Razor, TypeScript og LSP-baserte språk.

På debugging-fronten kan du nå profilere tester med Copilot, PerfTips integrerer med Profiler Agent, og smarte Watch-forslag gir kontekstbevisste uttrykk under debugging. Copilot kan også fikse NuGet-sårbarheter direkte fra Solution Explorer.

**Kilde:** [GitHub Copilot in Visual Studio — March update](https://github.blog/changelog/2026-04-02-github-copilot-in-visual-studio-march-update) (GitHub Changelog, 2. april 2026)

---

## 5. CLI-bruksmetrikker per bruker i organisasjonsrapporter

GitHub kompletterer CLI-metrikkdekningen med per-bruker-nedbrytninger i organisasjonsrapporter. Etter enterprise-nivå, bruker-nivå og organisasjonsnivå CLI-metrikker kan organisasjonsadministratorer nå se individuell CLI-aktivitet i 1-dagers og 28-dagersrapporter.

Metrikkene inkluderer om brukeren har CLI-aktivitet (`used_cli`), antall sesjoner og forespørsler per bruker, total tokenbruk med gjennomsnitt per forespørsel, og siste kjente CLI-versjon per bruker. Det siste er nyttig for å planlegge oppgraderinger og sikre at teamene bruker støttede versjoner.

**Kilde:** [Copilot usage metrics now includes per-user GitHub Copilot CLI activity in organization reports](https://github.blog/changelog/2026-04-02-copilot-usage-metrics-now-includes-per-user-github-copilot-cli-activity-in-organization-reports) (GitHub Changelog, 2. april 2026)

---

## 6. GPT-5.4 mini tilgjengelig for Copilot Student

GPT-5.4 mini er nå tilgjengelig for Copilot Student-planen via auto-modellvalg. Modellen er tilgjengelig i Copilot Chat på VS Code, Visual Studio, JetBrains, Xcode og Eclipse. GPT-5.4 mini gir studenter tilgang til en nyere modell uten å bruke premium request-kvote.

**Kilde:** [GPT-5.4 mini is now available in Copilot Student auto model selection](https://github.blog/changelog/2026-04-01-gpt-5-4-mini-is-now-available-in-copilot-student-auto-model-selection) (GitHub Changelog, 1. april 2026)

---

## Relevans for Nav

| Trend | Hva det betyr for Nav |
| ----- | --------------------- |
| Cloud agent forskning/planlegging | Utviklere kan bruke agenten til å utforske kodebase og lage planer før koding. Nyttig for onboarding og komplekse oppgaver i store repoer. |
| Copilot SDK public preview | Nav kan bygge interne verktøy og plattformtjenester som bruker Copilots agent-runtime. Go- og TypeScript-SDK-ene er relevante for eksisterende tech stack. OpenTelemetry-støtte passer med Navs observability-oppsett. |
| Org custom instructions GA | Nav kan sette standardinstruksjoner for alle ~500 utviklere — f.eks. Aksel spacing tokens, Nais-konvensjoner og sikkerhetskrav. Bør vurderes for bred utrulling. |
| Visual Studio marsoppdatering | Begrenset relevans for Nav (primært VS Code og JetBrains), men viser retningen for agent skills og MCP-styring som også kommer til andre IDE-er. |
| CLI-metrikker per bruker | Gir innsikt i hvilke utviklere som bruker Copilot CLI og versjonsdistribusjon. Relevant for å planlegge opplæring og oppgraderinger. |
| GPT-5.4 mini for Student | Begrenset direkte relevans for Nav, men viser at modellporteføljen fortsetter å utvides nedover i prisklassen. |
