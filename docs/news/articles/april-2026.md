---
title: "Nyheter og trender — April 2026"
date: 2026-04-03
draft: true
category: copilot
excerpt: "Cloud agent utvides til forskning og planlegging, Copilot SDK i public preview, organisasjonsstyring av runner og brannmur, signerte commits, merge-konfliktløsning, Slack-integrasjon for issues, agent-aktivitet i Projects, GitHub Mobile-oppdateringer, Visual Studio mars-oppdatering, CLI-metrikker per bruker, modelldeprecering."
tags:
  - coding-agents
  - copilot-sdk
  - enterprise
  - models
  - integrations
  - mobile
---

<!-- AI-REDAKSJONELT: Denne artikkelen er en oppsummering av de viktigste endringene og trendene — ikke en komplett liste. Prioriter det som er mest relevant for Nav-utviklere. Individuelle nyheter dekkes av egne excerpt-filer i samme mappe. -->

Starten av april 2026 markerer en tydelig retning: Copilot cloud agent (tidligere «coding agent») er ikke lenger bare et PR-verktøy. Den kan nå forske i kodebasen, lage implementeringsplaner og kode uten å åpne en pull request. Samtidig får organisasjoner betydelig bedre kontrollmuligheter med sentralisert styring av runnere, brannmur og instruksjoner. Copilot SDK åpner for å bygge egne agenter med samme runtime som GitHub bruker selv.

---

## 1. Cloud agent: forskning, planer og koding uten PR

Den største funksjonelle endringen i denne perioden er at Copilot cloud agent ikke lenger er begrenset til å åpne pull requests. Tre nye moduser utvider bruksområdet:

**Kode uten PR**: Agenten kan nå generere kode på en branch uten å opprette en PR. Du kan se hele diffen, iterere med Copilot, og først opprette PR-en når du er klar. Alternativt kan du be om en PR eksplisitt i prompten.

**Implementeringsplaner**: Be agenten om å lage en plan først. Copilot presenterer forslaget før den skriver en eneste linje kode. Du kan gi tilbakemelding og justere planen, og agenten bruker den godkjente planen som grunnlag for implementeringen.

**Deep research**: Start en forskningsøkt der Copilot undersøker kodebasen din grundig og svarer på spørsmål som krever dyp forståelse av repo-kontekst. Kan også startes direkte fra en Copilot Chat-samtale.

Disse modusene er tilgjengelige via Agents-fanen i repoet og i Copilot Chat.

**Kilde:** [Research, plan, and code with Copilot cloud agent](https://github.blog/changelog/2026-04-01-research-plan-and-code-with-copilot-cloud-agent) (GitHub Changelog, 1. april 2026)

---

## 2. Copilot SDK i public preview

GitHub har publisert Copilot SDK i public preview — det samme agent-runtimet som driver cloud agent og Copilot CLI, eksponert som bibliotek. SDK-et er tilgjengelig for Node.js/TypeScript, Python, Go, .NET og Java.

Nøkkelfunksjoner inkluderer: egendefinerte verktøy og agenter, finkornet tilpasning av systemprompt (replace, append, prepend, transform), streaming av responser, blob-vedlegg (bilder, skjermbilder), innebygd OpenTelemetry-sporing, et tillatelsesrammeverk for sensitive operasjoner, og Bring Your Own Key (BYOK) for OpenAI, Azure AI Foundry og Anthropic.

SDK-et er tilgjengelig for alle — også uten Copilot-abonnement — men hver prompt teller mot premium request-kvoten for abonnenter.

**Kilde:** [Copilot SDK in public preview](https://github.blog/changelog/2026-04-02-copilot-sdk-in-public-preview) (GitHub Changelog, 2. april 2026)

---

## 3. Organisasjonsstyring av cloud agent

Tre nye kontrollflater gir organisasjonsadministratorer langt bedre styringsmuligheter:

**Runner-kontroll**: Admins kan nå sette en standard-runner for cloud agent på tvers av alle repoer, og eventuelt låse innstillingen slik at enkeltrepoer ikke kan overstyre. Dette gjør det mulig å rulle ut større GitHub-hosted runners eller self-hosted runners konsistent.

**Brannmur-kontroll**: Agent-brannmuren — som kontrollerer cloud agents internettilgang — kan nå styres på organisasjonsnivå. Admins kan slå brannmuren av/på, konfigurere en organisasjonsdekkende allowlist (f.eks. for et internt pakkeregister), og bestemme om repoer kan legge til egne allowlist-oppføringer.

**Custom instructions GA**: Organisasjonstilpassede instruksjoner, først introdusert i april 2025, er nå generelt tilgjengelig. Administratorer kan sette standardinstruksjoner som styrer Copilots oppførsel i Chat, code review og cloud agent på tvers av alle repoer.

**Kilder:**

- [Organization runner controls for Copilot cloud agent](https://github.blog/changelog/2026-04-03-organization-runner-controls-for-copilot-cloud-agent) (GitHub Changelog, 3. april 2026)
- [Organization firewall settings for Copilot cloud agent](https://github.blog/changelog/2026-04-03-organization-firewall-settings-for-copilot-cloud-agent) (GitHub Changelog, 3. april 2026)
- [Copilot organization custom instructions are generally available](https://github.blog/changelog/2026-04-02-copilot-organization-custom-instructions-are-generally-available) (GitHub Changelog, 2. april 2026)

---

## 4. Cloud agent signerer commits og løser merge-konflikter

To praktiske forbedringer for cloud agent:

**Signerte commits**: Cloud agent signerer nå alle sine commits. De vises som «Verified» på GitHub, noe som gir trygghet for at committen faktisk ble laget av agenten og ikke er manipulert. Viktigere: dette betyr at cloud agent nå fungerer i repoer med branch protection-regelen «Require signed commits» — som tidligere blokkerte agenten helt.

**Merge-konfliktløsning**: Du kan nå be `@copilot` om å løse merge-konflikter direkte på en PR med en kommentar som `@copilot Merge in main and resolve the conflicts`. Agenten jobber i sitt eget utviklingsmiljø, gjør endringene, kjører bygg og tester, og pusher resultatet.

**Kilder:**

- [Copilot cloud agent signs its commits](https://github.blog/changelog/2026-04-03-copilot-cloud-agent-signs-its-commits) (GitHub Changelog, 3. april 2026)
- [Ask @copilot to resolve merge conflicts on pull requests](https://github.blog/changelog/2026-03-26-ask-copilot-to-resolve-merge-conflicts-on-pull-requests) (GitHub Changelog, 26. mars 2026)

---

## 5. Opprett issues fra Slack med Copilot

GitHub-appen for Slack kan nå opprette issues direkte med naturlig språk. Nevn `@GitHub` i en kanal, beskriv arbeidet, og appen lager strukturerte issues med titler, beskrivelser, assignees, labels og milestones. Den støtter også sub-issues med hierarki.

Du kan iterere i en Slack-tråd med `@GitHub` for å finjustere issue-detaljer før opprettelse, og du kan sette standard-repoer per kanal med `@GitHub settings`.

**Kilde:** [Create issues from Slack with Copilot](https://github.blog/changelog/2026-03-30-create-issues-from-slack-with-copilot) (GitHub Changelog, 30. mars 2026)

---

## 6. Agent-aktivitet synlig i Issues og Projects

Når en coding agent (Copilot, Claude, Codex) er tilordnet et issue, vises agent-sesjonen nå direkte under assignee i sidebaren med live status: «queued», «working», «waiting for review» eller «completed». Du kan klikke for å hoppe rett til sesjonsloggene.

Agent-sesjoner er nå også synlige i project table- og board-visninger, slik at du kan se hvilke issues som har aktive agent-sesjoner og hvordan arbeidet progredierer. Aktiveres via View-menyen med «Show agent sessions».

**Kilde:** [Agent activity in GitHub Issues and Projects](https://github.blog/changelog/2026-03-26-agent-activity-in-github-issues-and-projects) (GitHub Changelog, 26. mars 2026)

---

## 7. GitHub Mobile-oppdateringer for agenter

Mobilappen har fått to oppdateringer som gjør agent-arbeid lettere på farten:

**Ny Copilot-fane**: På Android har Copilot flyttet til navigasjonsmenyen. Den nye Home-opplevelsen gir bedre oversikt over agent-sesjoner og chathistorikk. Du kan nå se fulle sesjonslogger nativt, opprette PR-er fra fullførte sesjoner, og stoppe kjørende sesjoner — alt direkte fra appen.

**Enklere agent-tilordning**: Ny «Assign an Agent»-meny i issue overflow-menyen gjør det raskere å delegere arbeid. Du kan legge til egne instruksjoner og velge et annet repo for mer kontroll over oppgavedelegeringen.

**Kilder:**

- [GitHub Mobile: Stay in flow with a refreshed Copilot tab and native session logs](https://github.blog/changelog/2026-04-01-github-mobile-stay-in-flow-with-a-refreshed-copilot-tab-and-native-session-logs) (GitHub Changelog, 1. april 2026)
- [GitHub Mobile: Faster, more flexible agent assignment from issues](https://github.blog/changelog/2026-04-01-github-mobile-faster-more-flexible-agent-assignment-from-issues) (GitHub Changelog, 1. april 2026)

---

## 8. Copilot i Visual Studio — mars-oppdatering

Mars-oppdateringen for Visual Studio 2026 bringer flere viktige funksjoner:

- **Custom agents**: Definer spesialiserte Copilot-agenter som `.agent.md`-filer i repoet, med full tilgang til workspace, kodeforståelse, verktøy og MCP-tilkoblinger.
- **Enterprise MCP governance**: MCP-serverbruk respekterer nå allowlist-policyer satt via GitHub. Admins kan spesifisere hvilke MCP-servere som er tillatt.
- **Agent skills**: Gjenbrukbare instruksjonssett som lærer agenter spesifikke oppgaver. Copilot oppdager og anvender dem automatisk.
- **`find_symbol`-verktøy**: Nytt verktøy som gir agenter språkbevisst symbolnavigasjon — finn alle referanser, typemetadata, deklarasjoner og scope. Støtter C++, C#, Razor, TypeScript og LSP-språk.
- **Profileringsintegrasjon**: Ny «Profile with Copilot»-kommando i Test Explorer og PerfTips drevet av live profiling under debugging.
- **Sikkerhetsfiksing**: Copilot kan nå fikse NuGet-pakkesårbarheter direkte fra Solution Explorer.

**Kilde:** [GitHub Copilot in Visual Studio — March update](https://github.blog/changelog/2026-04-02-github-copilot-in-visual-studio-march-update) (GitHub Changelog, 2. april 2026)

---

## 9. CLI-bruksmetrikker per bruker i organisasjonsrapporter

Organisasjonsadministratorer kan nå se individuell CLI-bruk i 1-dagers og 28-dagers rapporter. Metrikkene inkluderer: om brukeren har CLI-aktivitet, antall sesjoner og forespørsler per bruker, totalt tokenforbruk med gjennomsnitt per forespørsel, og siste kjente CLI-versjon per bruker.

Dette kompletterer dekningsbildet etter at enterprise-, bruker- og organisasjonsnivå CLI-metrikker allerede er på plass.

**Kilde:** [Copilot usage metrics now includes per-user GitHub Copilot CLI activity in organization reports](https://github.blog/changelog/2026-04-02-copilot-usage-metrics-now-includes-per-user-github-copilot-cli-activity-in-organization-reports) (GitHub Changelog, 2. april 2026)

---

## 10. Modelloppdateringer

Tre depreceringer og én ny modell:

- **GPT-5.1-Codex-familien deprecated** (1. april): GPT-5.1-Codex, GPT-5.1-Codex-Max og GPT-5.1-Codex-Mini er fjernet. Anbefalt alternativ: GPT-5.3-Codex.
- **Gemini 3 Pro deprecated** (26. mars): Fjernet fra alle Copilot-opplevelser. Anbefalt alternativ: Gemini 3.1 Pro.
- **Claude Sonnet 4 fases ut 1. mai**: Varsel om kommende deprecering. Anbefalt alternativ: Claude Sonnet 4.6.
- **GPT-5.4 mini for Student**: GPT-5.4 mini er nå GA i auto-modellvalg for Copilot Student-planen.

**Kilder:**

- [GPT-5.1 Codex, GPT-5.1-Codex-Max, and GPT-5.1-Codex-Mini deprecated](https://github.blog/changelog/2026-04-03-gpt-5-1-codex-gpt-5-1-codex-max-and-gpt-5-1-codex-mini-deprecated) (GitHub Changelog, 3. april 2026)
- [Gemini 3 Pro deprecated](https://github.blog/changelog/2026-03-26-gemini-3-pro-deprecated) (GitHub Changelog, 26. mars 2026)
- [Upcoming deprecation of Claude Sonnet 4 in GitHub Copilot](https://github.blog/changelog/2026-03-31-upcoming-deprecation-of-claude-sonnet-4-in-github-copilot) (GitHub Changelog, 31. mars 2026)
- [GPT-5.4 mini is now available in Copilot Student auto model selection](https://github.blog/changelog/2026-04-01-gpt-5-4-mini-is-now-available-in-copilot-student-auto-model-selection) (GitHub Changelog, 1. april 2026)

---

## Relevans for Nav

| Trend | Hva det betyr for Nav |
| ----- | --------------------- |
| Cloud agent forskning og planer | Kan brukes til kodebase-analyse og planlegging av implementeringer — nyttig for store refaktoreringer eller onboarding i nye repoer. |
| Copilot SDK | Relevant for team som vil bygge egne agenter eller integrere Copilot i interne verktøy. Go og TypeScript SDK matcher Navs stack. |
| Org runner-kontroll | Nav kan sette standard-runner for cloud agent på tvers av alle repoer og låse innstillingen — viktig for sikkerhet og ytelse. |
| Org brannmur-kontroll | Sentralisert allowlist for interne pakkeregistre (f.eks. npm/Maven) og kontroll over agentens internettilgang. Kritisk for datasikkerhet. |
| Custom instructions GA | Nav kan sette organisasjonsdekkende instruksjoner for Copilot som håndhever Aksel-spacing, Nais-mønstre og kodekonvensjoner automatisk. |
| Signerte commits | Cloud agent fungerer nå i repoer med «Require signed commits» — fjerner en blokkering for team med strenge branch protection-regler. |
| Merge-konfliktløsning | Praktisk for PR-er som blir liggende — `@copilot` kan løse konflikter og kjøre tester uten manuell innsats. |
| Slack-integrasjon | Team som bruker Slack kan opprette issues direkte fra samtaler — reduserer kontekstbytte. |
| Agent-aktivitet i Projects | Bedre oversikt i prosjektstyring — se agentenes status i board/table-visninger uten å åpne hver issue. |
| CLI-metrikker per bruker | Gir innsikt i hvilke utviklere som bruker Copilot CLI og kan informere utrullingsstrategien for ~500 utviklere. |
| Modelldeprecering | Sjekk om team bruker Claude Sonnet 4 (fases ut 1. mai) eller GPT-5.1-Codex (allerede fjernet). Oppdater policyer til Claude Sonnet 4.6/GPT-5.3-Codex. |
