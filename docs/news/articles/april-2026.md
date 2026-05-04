---
title: "Nyheter og trender — April 2026"
date: 2026-04-17
draft: true
category: copilot
excerpt: "gh skill CLI for installasjon og publisering av agent skills, Claude Opus 4.7 GA, Autopilot-modus i VS Code, Copilot SDK i public preview, selektiv utrulling av cloud agent, personvernpolicy trer i kraft 24. april, BYOK og lokale modeller i Copilot CLI, Dependabot + AI-agenter, Project Glasswing, fjernstyr CLI fra nett og mobil, BYOK i VS Code for Business/Enterprise, agentsesjoner i Issues og Projects, Copilot Chat med PR-kontekst, Copilot for Jira med custom agents, GPT-5.5 GA, bruksbasert fakturering fra 1. juni, code review bruker Actions-minutter, cloud agent 20 % raskere, VS Code 1.118 med Agents-app og token-effektivitet, Visual Studio april-oppdatering."
tags:
  - skills
  - github-cli
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
  - code-review
  - agentic-workflows
  - jira
  - billing
  - gpt
  - performance
  - actions
  - visual-studio
---

<!-- AI-REDAKSJONELT: Denne artikkelen er en oppsummering av de viktigste endringene og trendene — ikke en komplett liste. Prioriter det som er mest relevant for Nav-utviklere. Mindre oppdateringer samles i «Flere oppdateringer»-seksjonen. Individuelle nyheter dekkes av egne excerpt-filer i samme mappe. -->

April 2026 starter med infrastruktur og ender med økosystem. GitHub åpner Copilot-motoren som SDK, gir organisasjoner finkornet kontroll over coding agent, og lanserer `gh skill` for å installere og publisere agent skills rett fra terminalen. VS Code får Autopilot-modus, Claude Opus 4.7 blir tilgjengelig i Copilot, og den kontroversielle personvernpolicyen for treningsdata trer i kraft 24. april.

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

## 7. Flere oppdateringer (tidlig april)

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

## 12. Modellvalg for tredjeparts coding agents

Claude og Codex coding agents på github.com støtter nå modellvalg. Når du starter en oppgave, velger du modell på samme måte som for Copilot cloud agent. Claude-brukere kan velge mellom Sonnet 4.5/4.6 og Opus 4.5/4.6, mens Codex-brukere kan velge mellom GPT-5.2-Codex, GPT-5.3-Codex og GPT-5.4.

Tilgang til tredjeparts-agenter følger med eksisterende Copilot-abonnement. For Business og Enterprise må admin aktivere policyen for Anthropic Claude eller OpenAI Codex.

**Kilde:** [Model selection for Claude and Codex agents on github.com](https://github.blog/changelog/2026-04-14-model-selection-for-claude-and-codex-agents-on-github-com) (GitHub Changelog, 14. april 2026)

---

## 13. Selektiv utrulling av cloud agent med custom properties

Enterprise-administratorer kan nå aktivere Copilot cloud agent per organisasjon, enten enkeltvis eller via custom properties. Tidligere var valget alt eller ingenting — nå kan du pilotere med utvalgte team og gradvis utvide tilgangen.

Tre nye API-endepunkter (PUT, POST, DELETE) styrer policyen programmatisk. Det samme valget er tilgjengelig i AI Controls-sida i innstillingene.

Merk: Custom properties evalueres kun på tidspunktet du konfigurerer. Endringer i properties senere aktiverer eller deaktiverer ikke cloud agent automatisk.

**Kilde:** [Enable Copilot cloud agent via custom properties](https://github.blog/changelog/2026-04-15-enable-copilot-cloud-agent-via-custom-properties) (GitHub Changelog, 15. april 2026)

---

## 14. `gh skill` — agent skills fra terminalen

GitHub CLI v2.90.0 introduserer `gh skill`, en ny kommando for å installere, oppdatere, publisere og søke etter agent skills. Skills følger den åpne [agentskills.io-spesifikasjonen](https://agentskills.io/specification) og fungerer på tvers av hosters: Copilot, Claude Code, Cursor, Codex og Gemini CLI.

Installasjon er én kommando:

```bash
gh skill install github/awesome-copilot documentation-writer
```

Skills installeres automatisk i riktig mappe for din agent host. Du kan også pinne til en spesifikk versjon eller commit:

```bash
gh skill install github/awesome-copilot documentation-writer --pin v1.2.0
```

`gh skill update` sjekker alle installerte skills mot upstream og tilbyr oppdatering. Provenance-metadata (repo, ref, tree SHA) skrives rett i SKILL.md-frontmatter, slik at sporbarhet følger med fila uansett hvor den havner.

For de som vedlikeholder skills-repoer: `gh skill publish` validerer mot agentskills.io-spesifikasjonen og sjekker om tag protection, secret scanning og code scanning er aktivert. Kommandoen kan også aktivere immutable releases, slik at publiserte versjoner ikke kan endres i ettertid.

**Kilde:** [Manage agent skills with GitHub CLI](https://github.blog/changelog/2026-04-16-manage-agent-skills-with-github-cli) (GitHub Changelog, 16. april 2026)

---

## 15. Claude Opus 4.7 tilgjengelig i Copilot

Anthropics nyeste Opus-modell ruller nå ut i Copilot. I GitHubs tidlige testing gir Opus 4.7 bedre ytelse på flertrinnsoppgaver og mer pålitelig agentisk utførelse enn forgjengeren. Modellen viser også framgang på langvarig resonnering og komplekse verktøyavhengige arbeidsflyter.

Opus 4.7 erstatter Opus 4.5 og 4.6 i modellvelgeren for Pro+-brukere i løpet av de kommende ukene. Business- og Enterprise-administratorer må aktivere modellpolicyen.

Modellen lanseres med en 7,5× premium request-multiplikator som kampanjepris fram til 30. april.

Tilgjengelig i VS Code, Visual Studio, Copilot CLI, cloud agent, github.com, GitHub Mobile, JetBrains, Xcode og Eclipse.

**Kilde:** [Claude Opus 4.7 is generally available](https://github.blog/changelog/2026-04-16-claude-opus-4-7-is-generally-available) (GitHub Changelog, 16. april 2026)

---

## 16. Flere oppdateringer (mid-april)

- **Modellvalg for tredjeparts-agenter**: Se seksjon 12. [Kilde](https://github.blog/changelog/2026-04-14-model-selection-for-claude-and-codex-agents-on-github-com)
- **OIDC for Dependabot og code scanning**: Dependabot og code scanning støtter nå OIDC-tokens for autentisering mot private registre — erstatter langlevde secrets. [Kilde](https://github.blog/changelog/2026-04-14-oidc-support-for-dependabot-and-code-scanning)
- **Rule insights dashboard**: Nytt visuelt dashboard for repository rulesets — se trender i blokkerte pushes, bypass-aktivitet og regelbrudd over tid. [Kilde](https://github.blog/changelog/2026-04-16-rule-insights-dashboard-and-unified-filter-bar)
- **CodeQL 2.25.2**: Kotlin 2.3.20-støtte og andre oppdateringer. [Kilde](https://github.blog/changelog/2026-04-15-codeql-2-25-2-adds-kotlin-2-3-20-support-and-other-updates)

---

## 17. BYOK for Copilot Business og Enterprise i VS Code

VS Code 1.117 (22. april) introduserer Bring Your Own Key (BYOK) for Copilot Business- og Enterprise-brukere. Team som trenger spesifikke modeller for compliance, ytelse eller kostnadsårsaker kan nå koble til egne API-nøkler fra leverandører som OpenRouter, Ollama, Google og OpenAI — og bruke disse modellene direkte i VS Code-chatten.

Funksjonen er aktivert som standard. Administratorer kan deaktivere den med policyen «Bring Your Own Language Model Key» i Copilots policy-innstillinger på GitHub.com. Organisasjonsmedlemmer kan legge til modeller fra innebygde leverandører eller installere language model provider-utvidelser.

Andre nyheter i 1.117 inkluderer inkrementell chat-rendering (eksperimentell) som streamer innhold blokk-for-blokk med valgfri animasjon, VS Code Agents-appen (Insiders) med sub-sesjoner og inline diff-rendering, og forbedret terminalstøtte for agent-CLI-er som Copilot CLI, Claude Code og Gemini CLI.

**Kilde:** [Visual Studio Code 1.117 Release Notes](https://code.visualstudio.com/updates/v1_117) (VS Code, 22. april 2026)

---

## 18. Agentsesjoner synlige i Issues og Projects

Cloud agent-sesjoner er nå synlige direkte i GitHub Issues og Projects. En ny «session pill» på issues viser aktive og fullførte agentsesjoner, og du kan åpne en sesjon i sidepanelet for å se fremdrift, gjennomgå logger eller gi agenten retning — uten å forlate issue-visningen.

I Projects er «Show agent sessions» nå aktivert som standard for både nye og eksisterende visninger. Du kan klikke på en agentsesjon i prosjekttavlen for å åpne den i sidepanelet, se detaljer og styre agenten direkte.

Dette gjør det lettere å holde oversikt over agentaktivitet i planleggingskonteksten der arbeidet allerede spores.

**Kilde:** [View and manage agent sessions from issues and projects](https://github.blog/changelog/2026-04-23-view-and-manage-agent-sessions-from-issues-and-projects) (GitHub Changelog, 23. april 2026)

---

## 19. Copilot Chat med rikere PR-kontekst

Copilot Chat på github.com har fått tre nye evner for pull requests. Når en PR gis som kontekst, inkluderer chatten nå kommentarer, filendringer, commits og reviews — ikke bare koden. Du kan be Copilot om å reviewe en PR og få en strukturert gjennomgang, eller be om en oppsummering for å raskt forstå hva endringene gjør.

Funksjonene virker både i on-page-chat (Copilot-knappen på en diff) og i den immersive chatten på github.com/copilot. Foreslåtte prompts er oppdatert for å guide deg til relevant funksjonalitet, for eksempel «Help review this pull request».

**Kilde:** [Copilot Chat improvements for pull requests](https://github.blog/changelog/2026-04-23-copilot-chat-improvements-for-pull-requests) (GitHub Changelog, 23. april 2026)

---

## 20. Strukturert feilsøking med stack traces

Copilot Chat på github.com gjenkjenner nå stack traces mer pålitelig og gir en strukturert rotårsaksanalyse. Når du limer inn en stack trace, svarer Copilot med hva som feilet og hvor, hvilken antakelse som ble brutt, den mest sannsynlige rotårsaken med kodebevis, et konfidensnivå og foreslått fiks, og neste steg for verifisering.

Legg ved relevant repo- eller filkontekst for best resultat. Har du et reproduksjonssteg som trigger feilen, gir det enda raskere analyse. Tilgjengelig for alle som bruker Copilot på github.com.

**Kilde:** [Better debugging with GitHub Copilot on the web](https://github.blog/changelog/2026-04-23-better-debugging-with-github-copilot-on-the-web) (GitHub Changelog, 23. april 2026)

---

## 21. Copilot for Jira: custom agents og mer

Copilot cloud agent for Jira har fått flere tilpasninger. Du kan nå spesifisere en custom agent fra GitHub-repoet rett i Jira-ticketen, slik at agenten bruker teamets egne instruksjoner og verktøy. Agenten leser også Atlassian custom fields (som akseptansekriterier) og følger forgreningsregler definert i ticketen.

Nye instruksjoner på Atlassian space-nivå lar deg sette standardverdier for target-repo, forgreningsregler, foretrukken modell og agent — slik at konfigurasjonen ikke gjentas for hver ticket. Når agenten åpner en draft-PR og ber om review, postes en kommentar i Jira-issuet så du vet at den er klar.

**Kilde:** [GitHub Copilot for Jira: Our latest enhancements](https://github.blog/changelog/2026-04-22-github-copilot-for-jira-our-latest-enhancements) (GitHub Changelog, 22. april 2026)

---

## 22. Flere oppdateringer (sen april)

- **Pause på Copilot Business-registreringer**: GitHub stanser midlertidig nye selvbetjente Copilot Business-registreringer for organisasjoner på Free og Team. Eksisterende kunder er ikke berørt. [Kilde](https://github.blog/changelog/2026-04-22-pausing-new-self-serve-signups-for-github-copilot-business)
- **Cloud agent-felt i usage metrics**: Nytt `used_copilot_cloud_agent`-felt i Copilot Usage Metrics API speiler `used_copilot_coding_agent` under nytt navn. Det gamle feltet fases ut 1. august 2026. [Kilde](https://github.blog/changelog/2026-04-23-copilot-cloud-agent-fields-added-to-usage-metrics)
- **Endring i nedlastings-URL-er for metrikk-rapporter**: Fra 20. mai migreres nedlastings-URL-er til `copilot-reports.github.com`. Oppdater brannmur-/proxy-allowlister. [Kilde](https://github.blog/changelog/2026-04-22-upcoming-change-to-copilot-usage-metrics-report-download-urls)

---

## 23. GPT-5.5 tilgjengelig i Copilot

OpenAIs nyeste GPT-modell er nå tilgjengelig i GitHub Copilot. I GitHubs tidlige testing leverer GPT-5.5 sterkest ytelse på komplekse, flerstegs agentoppgaver og løser kodeutfordringer som tidligere GPT-modeller ikke klarte.

Modellen lanseres med en 7,5× premium request-multiplikator som kampanjepris. GPT-5.5 er tilgjengelig for Pro+-, Business- og Enterprise-brukere i VS Code, Visual Studio, Copilot CLI, cloud agent, github.com, GitHub Mobile, JetBrains, Xcode og Eclipse. Business- og Enterprise-administratorer må aktivere GPT-5.5-policyen i Copilot-innstillingene.

**Kilde:** [GPT-5.5 is generally available for GitHub Copilot](https://github.blog/changelog/2026-04-24-gpt-5-5-is-generally-available-for-github-copilot) (GitHub Changelog, 24. april 2026)

---

## 24. Copilot går over til bruksbasert fakturering

GitHub kunngjør at alle Copilot-planer går over til bruksbasert fakturering 1. juni 2026. Premium request-enheter (PRU-er) erstattes av GitHub AI Credits. Forbruk beregnes basert på tokenbruk — input, output og cachede tokens — etter publiserte API-rater for hver modell.

Planpriser endres ikke: Pro er $10/mnd, Pro+ er $39/mnd, Business er $19/bruker/mnd og Enterprise er $39/bruker/mnd. Hver plan inkluderer AI Credits tilsvarende prisen. Code completions og Next Edit-forslag forblir inkludert og forbruker ikke credits. Fallback til billigere modeller ved uttømt kvote fjernes — i stedet styrer tilgjengelige credits og admin-budsjettkontroller bruken.

For Business og Enterprise innføres «pooled usage» — ubrukte credits deles på tvers av organisasjonen. Eksisterende kunder får kampanjekreditter i juni, juli og august (Business: $30/mnd, Enterprise: $70/mnd). Administratorer får nye budsjettkontroller på enterprise-, kostnadssenter- og brukernivå. En «preview bill»-opplevelse lanseres i mai for å gi innsyn i forventede kostnader.

**Kilde:** [GitHub Copilot is moving to usage-based billing](https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/) (GitHub Blog, 27. april 2026)

---

## 25. Code review bruker Actions-minutter fra 1. juni

I forbindelse med overgangen til bruksbasert fakturering kunngjør GitHub at Copilots agentbaserte kodegjennomgang vil forbruke GitHub Actions-minutter fra 1. juni. Hvert review faktureres på to måter: AI Credits (som all annen Copilot-bruk) og Actions-minutter fra eksisterende planentitlement for private repoer.

Offentlige repoer er ikke berørt — Actions-minutter forblir gratis der. Organisasjoner bør gjennomgå sitt Actions-minutterforbruk og budsjettgrenser, og vurdere om de vil bruke større GitHub-hosted runners eller self-hosted runners (som faktureres annerledes). Ingen ekstra konfigurasjon er nødvendig for de som allerede har GitHub-hosted runners aktivert.

**Kilde:** [GitHub Copilot code review will start consuming GitHub Actions minutes on June 1, 2026](https://github.blog/changelog/2026-04-27-github-copilot-code-review-will-start-consuming-github-actions-minutes-on-june-1-2026) (GitHub Changelog, 27. april 2026)

---

## 26. Cloud agent starter 20 % raskere

Copilot cloud agent starter nå over 20 % raskere takket være forhåndsbygde runner-miljøer med GitHub Actions custom images. Når du tildeler en issue til Copilot, starter en oppgave fra Agents-fanen, eller nevner `@copilot` i en PR, spinnes et skymiljø opp for å gjøre jobben. Ved å forhåndsbygge det miljøet med en custom Actions-image er oppstartskostnadene kuttet betydelig.

Dette bygger på 50 %-forbedringen fra mars og fortsetter å korte ned tilbakemeldingsløkken for cloud agent-brukere. Samlet er oppstartstiden nå over 60 % raskere enn ved starten av 2026.

**Kilde:** [Copilot cloud agent starts 20% faster with Actions custom images](https://github.blog/changelog/2026-04-27-copilot-cloud-agent-starts-20-faster-with-actions-custom-images) (GitHub Changelog, 27. april 2026)

---

## 27. VS Code 1.118 — Agents-app, semantisk søk og token-effektivitet

VS Code 1.118 (29. april) er en stor release for Copilot med fokus på effektivitet og nye agentopplevelser.

**VS Code Agents-appen** (Insiders) er en dedikert følgeapp for parallelle agentsesjoner. Du kan starte den fra tittellinjen i VS Code, dele autentisering og innstillinger, og bruke den fra nettleseren via Dev Tunnels. Claude agent er nå tilgjengelig i Agents-appen sammen med Copilot CLI og cloud agent.

**Semantisk indeksering** er nå tilgjengelig i alle arbeidsrom — ikke bare GitHub/ADO-repoer. Det gir agenten bedre kontekst ved å søke etter mening, ikke bare eksakte strenger.

**Token-effektivitet** er et gjennomgående tema. Et nytt «tool search»-verktøy holder bare ~30 kjernevektøy i konteksten og laster resten on-demand, med opptil 20 % token-besparelse. Agentic search tool og agentic execution tool er spesialiserte sub-agenter drevet av mindre modeller som håndterer kodesøk og terminalkommandoer til lavere kostnad. WebSockets for OpenAI-modeller gir 12 % raskere respons. Prompt caching er forbedret slik at over 93 % av hver forespørsel gjenbrukes fra cache i lengre sesjoner.

Andre nyheter: **Chronicle** (eksperimentell) indekserer chathistorikken i lokal SQLite for standup-rapporter og brukstips; **dedicated context for skills** isolerer skill-kjøring i en subagent; og en ny enterprise-policy (`ChatApprovedAccountOrganizations`) krever godkjent organisasjonsmedlemskap før AI-funksjoner aktiveres.

**Kilde:** [Visual Studio Code 1.118 Release Notes](https://code.visualstudio.com/updates/v1_118) (VS Code, 29. april 2026)

---

## 28. Visual Studio april-oppdatering

Visual Studio 2026 april-oppdateringen fokuserer på agentiske arbeidsflyter. **Cloud agent-integrasjon** lar deg starte nye cloud agent-sesjoner direkte fra IDE-en — velg «Cloud» fra agentvelgeren, beskriv oppgaven, og agenten oppretter issue og PR på remote-infrastruktur mens du jobber videre.

**Debugger-agent** er en ny arbeidsflyt for agentic «issue to resolution». Start fra en GitHub- eller Azure DevOps-issue, og agenten reproduserer feilen, instrumenterer koden, diagnostiserer og foreslår en målrettet fiks gjennom faktisk kjøring — validert mot live kjøretidsatferd.

Custom agents støtter nå brukerdefinisjoner lagret i `%USERPROFILE%/.github/agents/`, slik at personlige agenter følger deg på tvers av prosjekter. Agent skills oppdages fra flere steder inkludert `.claude/skills/` og `.agents/skills/` i tillegg til `.github/skills/`.

**Kilde:** [GitHub Copilot in Visual Studio — April update](https://github.blog/changelog/2026-04-30-github-copilot-in-visual-studio-april-update) (GitHub Changelog, 30. april 2026)

---

## Relevans for Nav

| Trend | Hva det betyr for Nav |
| --- | --- |
| `gh skill` CLI | Alle Nav-utviklere har `gh` installert. Vi har lagt til `gh skill install` på verktøysida og gjort alle 22 skills agentskills.io-kompatible. Vurder `gh skill publish` med immutable releases når vi flytter til `skills/`. |
| Claude Opus 4.7 | Ny toppmodell for agentiske oppgaver. Enterprise-admin må aktivere policyen. Kampanjepris (7,5×) til 30. april — test på komplekse oppgaver mens prisen er lav. |
| VS Code Autopilot | Agenter uten manuell godkjenning passer for rutineoppgaver. Test på avgrensede oppgaver med tydelige grenser via hooks og instruksjoner. |
| Copilot SDK | Nav kan bygge egne verktøy med Copilots agentmotor. Go SDK er direkte relevant for mcp-onboarding og mcp-registry. |
| Selektiv cloud agent-utrulling | Nav kan pilotere coding agent med utvalgte team via custom properties og gradvis utvide tilgangen. |
| PR-merge-metrikker | copilot-metrics kan hente nye felter for å måle om Copilot-review gir raskere merge. Konkrete tall til DORA-arbeid. |
| Org-runner for cloud agent | Sentralstyrt runner-konfigurasjon. Nav kan sette standard for alle repoer og låse til self-hosted runners. |
| Personvernpolicy | Nav bruker Enterprise — ikke berørt. Informer utviklere med personlige Copilot-kontoer om opt-out før 24. april. |
| BYOK og lokale modeller i CLI | Relevant for team med spesielle datatilgangskrav eller som vil bruke egne Azure OpenAI-endepunkter. |
| Dependabot + AI-agenter | Kan akselerere sikkerhetsoppdateringer i Navs ~500 repoer. Nyttig for breaking changes i major-oppgraderinger. |
| Project Glasswing | AI-drevet sårbarhetsjakt er her. Følg med på tilgjengeliggjøring for enterprise. |
| Fjernstyr CLI fra nett/mobil | Start lange CLI-sesjoner og følg med fra mobilen. Krever at admin aktiverer CLI- og remote-policyer. |
| OIDC for Dependabot | Erstatter langlevde secrets med korte OIDC-tokens for avhengighetsoppdateringer. |
| BYOK i VS Code for Business/Enterprise | Nav kan la team bruke egne Azure OpenAI-endepunkter direkte i VS Code — relevant for team med spesielle modellkrav. Admin styrer med policy. |
| Agentsesjoner i Issues/Projects | Gjør det lettere å spore agentaktivitet i planleggingsverktøy Nav allerede bruker. Aktivert som standard — ingen konfigurasjon nødvendig. |
| Copilot Chat med PR-kontekst | Strukturert review og oppsummering rett i github.com. Kan supplere code review-prosessen uten å forlate nettleseren. |
| Copilot for Jira | Relevant for Nav-team som bruker Jira med GitHub. Custom agents og space-instruksjoner gir konsistent agentoppførsel på tvers av tickets. |
| Metrikk-URL-endring | copilot-metrics-appen bør oppdateres til `copilot-reports.github.com` før 20. mai. `used_copilot_cloud_agent`-feltet bør tas i bruk før august. |
| GPT-5.5 GA | Ny toppmodell for agentoppgaver. Enterprise-admin må aktivere policyen. 7,5× multiplikator gjør den dyr — test på komplekse oppgaver der billigere modeller feiler. |
| Bruksbasert fakturering | Nav bruker Enterprise — pool-baserte credits erstatter PRU-er fra 1. juni. Administratorer bør sette budsjettkontroller og følge med på «preview bill» i mai. |
| Code review + Actions-minutter | Nav må budsjettere Actions-minutter for agentbasert code review i private repoer. Gjennomgå eksisterende minutt-entitlement og budsjettkontroller. |
| Cloud agent 20 % raskere | Samlet 60 %+ raskere oppstart i 2026. Kortere feedback-loop gjør cloud agent mer praktisk for daglig bruk. |
| VS Code 1.118 token-effektivitet | Lavere token-forbruk betyr lavere kostnad under ny bruksbasert modell. Oppdater VS Code for å dra nytte av automatiske besparelser. |
| VS Code Agents-appen | Parallelle agentsesjoner og web-tilgang via Dev Tunnels. Relevant for utviklere som jobber med flere oppgaver samtidig. |
| Visual Studio debugger-agent | Relevant for .NET-team i Nav som bruker Visual Studio. Automatisert feilsøking fra issue til fiks. |
