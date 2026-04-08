---
title: "Nyheter og trender — April 2026"
date: 2026-04-08
draft: true
category: copilot
excerpt: "Copilot SDK i public preview, legacy metrics API nedlagt, organisasjonsstyrt runner, personvernpolicy trer i kraft 24. april, BYOK og lokale modeller i Copilot CLI, Dependabot + AI-agenter, Project Glasswing."
tags:
  - copilot-sdk
  - coding-agents
  - enterprise-controls
  - privacy
  - metrics
  - copilot-cli
  - security
  - models
---

<!-- AI-REDAKSJONELT: Denne artikkelen er en oppsummering av de viktigste endringene og trendene — ikke en komplett liste. Prioriter det som er mest relevant for Nav-utviklere. Mindre oppdateringer samles i «Flere oppdateringer»-seksjonen. Individuelle nyheter dekkes av egne excerpt-filer i samme mappe. -->

April 2026 starter med infrastruktur. GitHub åpner Copilot-motoren som SDK, legger ned det gamle metrics-API-et, og gir organisasjoner bedre kontroll over hvordan coding agent kjører. Senere i måneden trer den kontroversielle personvernpolicyen for treningsdata i kraft.

---

## 1. Copilot SDK i public preview

GitHub Copilot SDK er nå tilgjengelig i public preview — det samme agentmotoren som driver Copilot cloud agent og Copilot CLI, pakket som bibliotek. SDK-et gir verktøyinvokning, streaming, filoperasjoner og multi-turn-sesjoner rett ut av boksen, uten at du trenger å bygge egen AI-orkestrering.

Tilgjengelig i fem språk: Node.js/TypeScript, Python, Go, .NET og Java (nytt). Nøkkelfunksjoner inkluderer custom tools med handlers, finkornet system-prompt-tilpasning (`replace`, `append`, `prepend`, `transform`), OpenTelemetry-integrasjon for distribuert tracing, et tillatelsesrammeverk for sensitive operasjoner, og Bring Your Own Key (BYOK) for OpenAI, Azure AI Foundry eller Anthropic.

SDK-et er tilgjengelig for alle — også brukere uten Copilot-abonnement via BYOK. Hver prompt teller mot premium request-kvoten for Copilot-abonnenter.

**Kilde:** [Copilot SDK in public preview](https://github.blog/changelog/2026-04-02-copilot-sdk-in-public-preview/) (GitHub Changelog, 2. april 2026)

---

## 2. Legacy Copilot Metrics API nedlagt

Det gamle Copilot Metrics API-et ble offisielt nedlagt 2. april 2026, som varslet i januar. Organisasjoner som fortsatt bruker de gamle endepunktene mister nå tilgang til bruksdata. Det nye Usage Metrics API-et leverer data via NDJSON-filer med langt mer detaljert telemetri — per språk, IDE, modell, kodelinje og redigeringsmodus.

Team-nivå-metrikker er ikke lenger tilgjengelig — kun organisasjons- og enterprise-nivå støttes i det nye skjemaet.

**Kilde:** [Closing down notice of legacy Copilot metrics APIs](https://github.blog/changelog/2026-01-29-closing-down-notice-of-legacy-copilot-metrics-apis/) (GitHub Changelog, 29. januar 2026)

---

## 3. Organisasjonsstyrt runner for cloud agent

Inntil nå ble runner-konfigurasjonen for coding agent satt per repository via `copilot-setup-steps.yml`. Nå kan organisasjonsadministratorer sette en standard-runner som brukes automatisk for alle repoer — og valgfritt låse innstillingen slik at individuelle repoer ikke kan overstyre den.

Dette gjør det enklere å rulle ut konsistente defaults (for eksempel større GitHub Actions-runnere for bedre ytelse) og sikre at agenten alltid kjører der organisasjonen vil — for eksempel på self-hosted runners med tilgang til interne ressurser.

**Kilde:** [Organization runner controls for Copilot cloud agent](https://github.blog/changelog/2026-04-03-organization-runner-controls-for-copilot-cloud-agent/) (GitHub Changelog, 3. april 2026)

---

## 4. Personvernpolicy for treningsdata trer i kraft

Den 24. april trer GitHubs oppdaterte personvernpolicy i kraft: interaksjonsdata fra Copilot Free-, Pro- og Pro+-brukere brukes til modelltrening med mindre de aktivt velger bort. Copilot Business og Enterprise er ikke berørt — kontraktsvilkårene beskytter enterprise-data.

Policyen ble kunngjort 25. mars og har møtt sterk kritikk for å være opt-out i stedet for opt-in. Utviklere som bruker personlige kontoer bør sjekke innstillingene under [Settings → Copilot → Privacy](https://github.com/settings/copilot).

**Kilde:** [Updates to GitHub Copilot interaction data usage policy](https://github.blog/news-insights/company-news/updates-to-github-copilot-interaction-data-usage-policy/) (GitHub Blog, 25. mars 2026)

---

## 5. Flere oppdateringer

- **Visual Studio Mars-oppdatering**: custom agents (`.agent.md`), agent skills, `find_symbol`-verktøy, Enterprise MCP governance med allowlist-policyer. [Kilde](https://github.blog/changelog/2026-04-02-github-copilot-in-visual-studio-march-update/)
- **GPT-5.1 Codex avviklet**: alle GPT-5.1-varianter (Codex, Codex-Max, Codex-Mini) er fjernet fra Copilot. Anbefalt erstatning er GPT-5.3-Codex. [Kilde](https://github.blog/changelog/2026-04-03-gpt-5-1-codex-gpt-5-1-codex-max-and-gpt-5-1-codex-mini-deprecated/)
- **Gemma 4 open source**: Google lanserer sin mest avanserte åpne modellfamilie under Apache 2.0 — fire varianter fra 2B til 31B parametere, multimodal (tekst, bilde, video, lyd), opptil 256K kontekst. [Kilde](https://blog.google/innovation-and-ai/technology/developers-tools/gemma-4/)

---

## 6. BYOK og lokale modeller i Copilot CLI

Copilot CLI støtter nå Bring Your Own Key (BYOK) og lokale modeller. Du kan koble til Azure OpenAI, Anthropic eller et hvilket som helst OpenAI-kompatibelt endepunkt — inkludert lokale løsninger som Ollama, vLLM og Foundry Local. Konfigurasjonen skjer gjennom miljøvariabler, og innebygde sub-agenter (explore, task, code-review) arver automatisk leverandørkonfigurasjonen.

En ny offline-modus (`COPILOT_OFFLINE=true`) slår av all telemetri og forhindrer at CLI-en kontakter GitHubs servere. Kombinert med en lokal modell gir dette en fullstendig air-gapped utviklingsopplevelse. GitHub-autentisering er nå valgfritt — du kan starte CLI-en med kun leverandør-credentials. Logger du også inn på GitHub, får du tilgang til funksjoner som `/delegate`, GitHub Code Search og GitHub MCP-serveren i tillegg.

Modellen må støtte tool calling og streaming. For best resultat anbefales minst 128K kontekstvindu.

**Kilde:** [Copilot CLI now supports BYOK and local models](https://github.blog/changelog/2026-04-07-copilot-cli-now-supports-byok-and-local-models) (GitHub Changelog, 7. april 2026)

---

## 7. Dependabot-varsler kan tildeles AI-agenter

Noen avhengighetssårbarheter krever mer enn en versjonsoppdatering — de trenger kodeendringer på tvers av prosjektet. Nå kan du tildele Dependabot-varsler direkte til AI coding agents, inkludert Copilot, Claude og Codex. Agenten analyserer varselet, åpner en draft-PR med foreslått fiks, og forsøker å løse eventuelle testfeil som oppstår.

Du kan tildele flere agenter til samme varsel. Hver agent jobber uavhengig og åpner sin egen PR, slik at du kan sammenligne tilnærminger. Dette er spesielt nyttig for major version-oppgraderinger som introduserer breaking API-endringer, for nedgradering av kompromitterte pakker, og for komplekse oppdateringsscenarier som faller utenfor Dependabots regelbaserte motor.

Funksjonen krever GitHub Code Security og et Copilot-abonnement med tilgang til coding agent.

**Kilde:** [Dependabot alerts are now assignable to AI agents for remediation](https://github.blog/changelog/2026-04-07-dependabot-alerts-are-now-assignable-to-ai-agents-for-remediation) (GitHub Changelog, 7. april 2026)

---

## 8. Project Glasswing — AI-drevet cybersikkerhet

Anthropic avduket Project Glasswing, et samarbeid mellom 12 industripartnere — blant dem AWS, Apple, Google, Microsoft, NVIDIA og Linux Foundation. Initiativet er bygget rundt Claude Mythos Preview, en urelatert frontiermodell som autonomt finner zero-day-sårbarheter i alle store operativsystemer og nettlesere. Modellen har allerede funnet tusenvis av kritiske sårbarheter, inkludert en 27 år gammel feil i OpenBSD og en 16 år gammel feil i FFmpeg som automatiserte tester hadde kjørt forbi fem millioner ganger.

Mythos Preview scorer 83,1 % på CyberGym-benchmarken for sårbarhetsgjenfinning, mot 66,6 % for Opus 4.6. Anthropic forplikter opptil $100M i brukskreditter og $4M i donasjoner til open source-sikkerhetsorganisasjoner. Modellen er ikke generelt tilgjengelig — den deles med partnere og over 40 organisasjoner som bygger eller vedlikeholder kritisk programvareinfrastruktur.

Selv om Mythos Preview ikke er en kodeverktøy-modell, signaliserer den at AI-modeller nå konkurrerer med de beste menneskene på å finne og utnytte sårbarheter — noe som endrer trusselbildet fundamentalt for alle som skriver og vedlikeholder programvare.

**Kilde:** [Project Glasswing](https://anthropic.com/glasswing) (Anthropic, 7. april 2026)

---

## Relevans for Nav

| Trend | Hva det betyr for Nav |
| --- | --- |
| Copilot SDK | Nav kan bygge egne verktøy med Copilots agentmotor — Go SDK er direkte relevant for mcp-onboarding og mcp-registry. Vurder for interne tjenester. |
| Legacy metrics API nedlagt | Navs copilot-metrics-app bruker allerede det nye Usage Metrics API-et — ingen handling nødvendig. Verifiser at ingen andre Nav-verktøy bruker det gamle API-et. |
| Org-runner for cloud agent | Sentralstyrt runner-konfigurasjon. Nav kan sette standard for alle repoer og låse til self-hosted runners ved behov — viktig for compliance og ytelse. |
| Personvernpolicy | Nav bruker Enterprise — ikke berørt. Informer utviklere som bruker personlige Copilot-kontoer om opt-out før 24. april. |
| GPT-5.1 deprecering | Sjekk om noen team har satt GPT-5.1 som foretrukket modell. |
| BYOK og lokale modeller i CLI | Relevant for team med spesielle krav til datatilgang eller som ønsker å bruke egne Azure OpenAI-endepunkter. Offline-modus kan være interessant for sikkerhetssensitive miljøer. |
| Dependabot + AI-agenter | Kan akselerere sikkerhetsoppdateringer i Navs ~500 repoer. Vurder å aktivere for team med mange Dependabot-varsler — spesielt nyttig for breaking changes i major-oppgraderinger. |
| Project Glasswing | Signaliserer at AI-drevet sårbarhetsjakt er her. Nav bør følge med på når slike verktøy blir tilgjengelige for enterprise-kunder, og vurdere implikasjonene for egen sikkerhetspraksis. |
