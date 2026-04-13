---
title: "Nyheter og trender — April 2026"
date: 2026-04-13
draft: true
category: copilot
excerpt: "Autopilot-modus i VS Code, Copilot SDK i public preview, PR-merge-metrikker for code review, organisasjonsstyrt runner, personvernpolicy trer i kraft 24. april, BYOK og lokale modeller i Copilot CLI, Dependabot + AI-agenter, Project Glasswing, fjernstyr CLI fra nett og mobil."
tags:
  - copilot-sdk
  - coding-agents
  - enterprise-controls
  - privacy
  - metrics
  - copilot-cli
  - security
  - remote
  - mobile
  - models
  - vscode
  - autopilot
---

<!-- AI-REDAKSJONELT: Denne artikkelen er en oppsummering av de viktigste endringene og trendene — ikke en komplett liste. Prioriter det som er mest relevant for Nav-utviklere. Mindre oppdateringer samles i «Flere oppdateringer»-seksjonen. Individuelle nyheter dekkes av egne excerpt-filer i samme mappe. -->

April 2026 starter med infrastruktur. GitHub åpner Copilot-motoren som SDK og gir organisasjoner bedre kontroll over hvordan coding agent kjører. VS Code får Autopilot-modus for helt autonome agentsesjoner. Senere i måneden trer den kontroversielle personvernpolicyen for treningsdata i kraft.

---

## 1. Copilot SDK i public preview

GitHub Copilot SDK er nå tilgjengelig i public preview — det samme agentmotoren som driver Copilot cloud agent og Copilot CLI, pakket som bibliotek. SDK-et gir verktøyinvokning, streaming, filoperasjoner og multi-turn-sesjoner rett ut av boksen, uten at du trenger å bygge egen AI-orkestrering.

Tilgjengelig i fem språk: Node.js/TypeScript, Python, Go, .NET og Java (nytt). Nøkkelfunksjoner inkluderer custom tools med handlers, finkornet system-prompt-tilpasning (`replace`, `append`, `prepend`, `transform`), OpenTelemetry-integrasjon for distribuert tracing, et tillatelsesrammeverk for sensitive operasjoner, og Bring Your Own Key (BYOK) for OpenAI, Azure AI Foundry eller Anthropic.

SDK-et er tilgjengelig for alle — også brukere uten Copilot-abonnement via BYOK. Hver prompt teller mot premium request-kvoten for Copilot-abonnenter.

**Kilde:** [Copilot SDK in public preview](https://github.blog/changelog/2026-04-02-copilot-sdk-in-public-preview/) (GitHub Changelog, 2. april 2026)

---

## 2. Organisasjonsstyrt runner for cloud agent

Inntil nå ble runner-konfigurasjonen for coding agent satt per repository via `copilot-setup-steps.yml`. Nå kan organisasjonsadministratorer sette en standard-runner som brukes automatisk for alle repoer — og valgfritt låse innstillingen slik at individuelle repoer ikke kan overstyre den.

Dette gjør det enklere å rulle ut konsistente defaults (for eksempel større GitHub Actions-runnere for bedre ytelse) og sikre at agenten alltid kjører der organisasjonen vil — for eksempel på self-hosted runners med tilgang til interne ressurser.

**Kilde:** [Organization runner controls for Copilot cloud agent](https://github.blog/changelog/2026-04-03-organization-runner-controls-for-copilot-cloud-agent/) (GitHub Changelog, 3. april 2026)

---

## 3. Personvernpolicy for treningsdata trer i kraft

Den 24. april trer GitHubs oppdaterte personvernpolicy i kraft: interaksjonsdata fra Copilot Free-, Pro- og Pro+-brukere brukes til modelltrening med mindre de aktivt velger bort. Copilot Business og Enterprise er ikke berørt — kontraktsvilkårene beskytter enterprise-data.

Policyen ble kunngjort 25. mars og har møtt sterk kritikk for å være opt-out i stedet for opt-in. Utviklere som bruker personlige kontoer bør sjekke innstillingene under [Settings → Copilot → Privacy](https://github.com/settings/copilot).

**Kilde:** [Updates to GitHub Copilot interaction data usage policy](https://github.blog/news-insights/company-news/updates-to-github-copilot-interaction-data-usage-policy/) (GitHub Blog, 25. mars 2026)

---

## 4. VS Code mars-releaser — Autopilot og mer

VS Code gikk over til ukentlige stabile releaser i mars. Changelog-posten dekker versjon 1.111 til 1.115 og er den største VS Code-oppdateringen for Copilot på lenge.

**Autopilot-modus** (public preview) lar agenter kjøre helt autonomt. Agenten godkjenner egne handlinger, prøver på nytt ved feil, og jobber til oppgaven er ferdig — uten at du trenger å trykke «godkjenn» underveis. Tillatelses-nivået settes per sesjon: Default, Bypass Approvals eller Autopilot.

Andre viktige nyheter:

- **Konfigurerbar tenke-innsats**: Styr hvor grundig resonneringsmodeller (Claude Sonnet 4.6, GPT-5.4) tenker — direkte fra modellvelgeren. Innstillingen huskes mellom samtaler.
- **Nestede sub-agenter**: Sub-agenter kan nå starte andre sub-agenter, noe som gjør komplekse flertrinnsoppgaver lettere å dekomponere.
- **Bilder og video i chat**: Legg ved skjermbilder eller videoer. Agenter kan returnere opptak av endringer som du ser i en karusell.
- **Nettleser-debugging**: Sett breakpoints, step through-kode og inspiser variabler i den integrerte nettleseren.
- **Session forking**: Fork en sesjon i Copilot CLI eller Claude agent for å utforske alternative tilnærminger uten å miste originalsamtalen.
- **Samlet tilpasningseditor**: Instruksjoner, agenter, skills og plugins styres fra ett sted.
- **Sandbox for MCP-servere**: Lokale MCP-servere kan kjøre i en begrenset sandbox (macOS og Linux).
- **Monorepo-oppdagelse**: VS Code finner nå instruksjoner, agenter, skills og hooks fra mapper oppover til roten av repoet.
- **Agent-spesifikke hooks**: Knytt pre- og post-prosesseringslogikk til bestemte custom agents via YAML-frontmatter i `.agent.md`-filer.

**Kilde:** [GitHub Copilot in Visual Studio Code, March Releases](https://github.blog/changelog/2026-04-08-github-copilot-in-visual-studio-code-march-releases/) (GitHub Changelog, 8. april 2026)

---

## 5. PR-merge-metrikker for Copilot code review

Usage Metrics API-et har fått to nye felter som måler effekten av Copilots kodegjennomgang:

- `pull_requests.total_merged_reviewed_by_copilot` — antall mergede PR-er som Copilot har reviewet
- `pull_requests.median_minutes_to_merge_copilot_reviewed` — median tid fra PR-opprettelse til merge for Copilot-reviewede PR-er

Dataene er tilgjengelige per dag og i 28-dagers rullerende vindu, på enterprise- og organisasjonsnivå. Kombinert med eksisterende metrikker for Copilot-forfattede PR-er gir dette et komplett bilde av Copilots bidrag — fra koding til review til merge.

**Kilde:** [Copilot-reviewed pull request merge metrics now in the usage metrics API](https://github.blog/changelog/2026-04-08-copilot-reviewed-pull-request-merge-metrics-now-in-the-usage-metrics-api/) (GitHub Changelog, 8. april 2026)

---

## 6. Copilot i sikkerhetsvurderinger

Organisasjonsadministratorer og sikkerhetsansvarlige kan nå starte en Copilot-sesjon direkte fra resultatene av secret risk assessment og Code Security risk assessment. Copilot gir kontekstuelle forklaringer og veiledet utbedring — AI-drevet sikkerhetsstøtte rett i vurderingsarbeidsflyten.

**Kilde:** [Ask Copilot in security assessments now available](https://github.blog/changelog/2026-04-09-ask-copilot-in-security-assessments-now-available) (GitHub Changelog, 9. april 2026)

---

## 7. Flere oppdateringer

- **Visual Studio mars-oppdatering**: Custom agents (`.agent.md`), agent skills, `find_symbol`-verktøy, Enterprise MCP governance med allowlist-policyer. [Kilde](https://github.blog/changelog/2026-04-02-github-copilot-in-visual-studio-march-update/)
- **GPT-5.1 Codex avviklet**: Alle GPT-5.1-varianter (Codex, Codex-Max, Codex-Mini) er fjernet fra Copilot. Anbefalt erstatning er GPT-5.3-Codex. [Kilde](https://github.blog/changelog/2026-04-03-gpt-5-1-codex-gpt-5-1-codex-max-and-gpt-5-1-codex-mini-deprecated/)
- **Gemma 4 open source**: Google lanserer sin mest avanserte åpne modellfamilie under Apache 2.0 — fire varianter fra 2B til 31B parametere, multimodal (tekst, bilde, video, lyd), opptil 256K kontekst. [Kilde](https://blog.google/innovation-and-ai/technology/developers-tools/gemma-4/)

---

## 8. BYOK og lokale modeller i Copilot CLI

Copilot CLI støtter nå Bring Your Own Key (BYOK) og lokale modeller. Du kan koble til Azure OpenAI, Anthropic eller et hvilket som helst OpenAI-kompatibelt endepunkt — inkludert lokale løsninger som Ollama, vLLM og Foundry Local. Konfigurasjonen skjer gjennom miljøvariabler, og innebygde sub-agenter (explore, task, code-review) arver automatisk leverandørkonfigurasjonen.

En ny offline-modus (`COPILOT_OFFLINE=true`) slår av all telemetri og forhindrer at CLI-en kontakter GitHubs servere. Kombinert med en lokal modell gir dette en fullstendig air-gapped utviklingsopplevelse. GitHub-autentisering er nå valgfritt — du kan starte CLI-en med kun leverandør-credentials. Logger du også inn på GitHub, får du tilgang til funksjoner som `/delegate`, GitHub Code Search og GitHub MCP-serveren i tillegg.

Modellen må støtte tool calling og streaming. For best resultat anbefales minst 128K kontekstvindu.

**Kilde:** [Copilot CLI now supports BYOK and local models](https://github.blog/changelog/2026-04-07-copilot-cli-now-supports-byok-and-local-models) (GitHub Changelog, 7. april 2026)

---

## 9. Dependabot-varsler kan tildeles AI-agenter

Noen avhengighetssårbarheter krever mer enn en versjonsoppdatering — de trenger kodeendringer på tvers av prosjektet. Nå kan du tildele Dependabot-varsler direkte til AI coding agents, inkludert Copilot, Claude og Codex. Agenten analyserer varselet, åpner en draft-PR med foreslått fiks, og forsøker å løse eventuelle testfeil som oppstår.

Du kan tildele flere agenter til samme varsel. Hver agent jobber uavhengig og åpner sin egen PR, slik at du kan sammenligne tilnærminger. Dette er spesielt nyttig for major version-oppgraderinger som introduserer breaking API-endringer, for nedgradering av kompromitterte pakker, og for komplekse oppdateringsscenarier som faller utenfor Dependabots regelbaserte motor.

Funksjonen krever GitHub Code Security og et Copilot-abonnement med tilgang til coding agent.

**Kilde:** [Dependabot alerts are now assignable to AI agents for remediation](https://github.blog/changelog/2026-04-07-dependabot-alerts-are-now-assignable-to-ai-agents-for-remediation) (GitHub Changelog, 7. april 2026)

---

## 10. Project Glasswing — AI-drevet cybersikkerhet

Anthropic avduket Project Glasswing, et samarbeid mellom 12 industripartnere — blant dem AWS, Apple, Google, Microsoft, NVIDIA og Linux Foundation. Initiativet er bygget rundt Claude Mythos Preview, en urelatert frontiermodell som autonomt finner zero-day-sårbarheter i alle store operativsystemer og nettlesere. Modellen har allerede funnet tusenvis av kritiske sårbarheter, inkludert en 27 år gammel feil i OpenBSD og en 16 år gammel feil i FFmpeg som automatiserte tester hadde kjørt forbi fem millioner ganger.

Mythos Preview scorer 83,1 % på CyberGym-benchmarken for sårbarhetsgjenfinning, mot 66,6 % for Opus 4.6. Anthropic forplikter opptil $100M i brukskreditter og $4M i donasjoner til open source-sikkerhetsorganisasjoner. Modellen er ikke generelt tilgjengelig — den deles med partnere og over 40 organisasjoner som bygger eller vedlikeholder kritisk programvareinfrastruktur.

Selv om Mythos Preview ikke er en kodeverktøy-modell, signaliserer den at AI-modeller nå konkurrerer med de beste menneskene på å finne og utnytte sårbarheter — noe som endrer trusselbildet fundamentalt for alle som skriver og vedlikeholder programvare.

**Kilde:** [Project Glasswing](https://anthropic.com/glasswing) (Anthropic, 7. april 2026)

---

## 11. Fjernstyr CLI-sesjoner fra nett og mobil

Copilot CLI er ikke lenger en ren lokal opplevelse. Med `copilot --remote` kan du nå overvåke og styre en kjørende CLI-sesjon direkte fra GitHub på nett eller i GitHub Mobile-appene. CLI-en streamer sesjonsaktiviteten til GitHub i sanntid og viser en lenke og QR-kode du kan åpne fra en annen enhet.

Fjernsesjoner støtter alle funksjoner du forventer fra CLI-en: du kan sende meldinger midt i en sesjon, gjennomgå og endre planer, bytte mellom plan-, interaktiv- og autopilot-modus, godkjenne eller avvise tilgangsforespørsler, og svare på spørsmål fra Copilot. Aktiviteten holdes synkront mellom CLI-en og GitHub — handlinger du gjør ett sted reflekteres i det andre. Hver fjernsesjon er privat og kun synlig for brukeren som startet den.

For Copilot Business- og Enterprise-brukere må en administrator aktivere remote control- og CLI-policyer før funksjonen kan brukes.

**Kilde:** [Remote control CLI sessions on web and mobile in public preview](https://github.blog/changelog/2026-04-13-remote-control-cli-sessions-on-web-and-mobile-in-public-preview) (GitHub Changelog, 13. april 2026)

---

## Relevans for Nav

| Trend | Hva det betyr for Nav |
| --- | --- |
| VS Code Autopilot | Agenter som kjører uten manuell godkjenning passer godt for rutineoppgaver. Nav-team bør teste Autopilot-modus på avgrensede oppgaver og sette tydelige grenser via hooks og instruksjoner. |
| Copilot SDK | Nav kan bygge egne verktøy med Copilots agentmotor — Go SDK er direkte relevant for mcp-onboarding og mcp-registry. Vurder for interne tjenester. |
| PR-merge-metrikker | Navs copilot-metrics-app kan hente nye felter for å måle om Copilot-review faktisk gir raskere merge. Gir konkrete tall til DORA-arbeid. |
| Org-runner for cloud agent | Sentralstyrt runner-konfigurasjon. Nav kan sette standard for alle repoer og låse til self-hosted runners ved behov — viktig for compliance og ytelse. |
| Personvernpolicy | Nav bruker Enterprise — ikke berørt. Informer utviklere som bruker personlige Copilot-kontoer om opt-out før 24. april. |
| Copilot i sikkerhetsvurderinger | Nyttig for Navs sikkerhetsansvarlige — kontekstuell AI-støtte rett i risikovurderingene kan akselerere utbedringsarbeid. |
| GPT-5.1 deprecering | Sjekk om noen team har satt GPT-5.1 som foretrukket modell. |
| BYOK og lokale modeller i CLI | Relevant for team med spesielle krav til datatilgang eller som ønsker å bruke egne Azure OpenAI-endepunkter. Offline-modus kan være interessant for sikkerhetssensitive miljøer. |
| Dependabot + AI-agenter | Kan akselerere sikkerhetsoppdateringer i Navs ~500 repoer. Vurder å aktivere for team med mange Dependabot-varsler — spesielt nyttig for breaking changes i major-oppgraderinger. |
| Project Glasswing | Signaliserer at AI-drevet sårbarhetsjakt er her. Nav bør følge med på når slike verktøy blir tilgjengelige for enterprise-kunder, og vurdere implikasjonene for egen sikkerhetspraksis. |
| Fjernstyr CLI fra nett/mobil | Utviklere kan starte lange CLI-sesjoner og følge med fra mobilen eller en annen maskin. Nyttig for oppgaver som tar tid — for eksempel store refaktoreringer eller migreringer. Krever at admin aktiverer CLI- og remote-policyer for Business/Enterprise. |
