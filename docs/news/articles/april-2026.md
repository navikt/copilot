---
title: "Nyheter og trender — April 2026"
date: 2026-04-20
draft: true
category: copilot
excerpt: "gh skill CLI for installasjon og publisering av agent skills, Claude Opus 4.7 GA, Autopilot-modus i VS Code, Copilot SDK i public preview, selektiv utrulling av cloud agent, personvernpolicy trer i kraft 24. april, BYOK og lokale modeller i Copilot CLI, Dependabot + AI-agenter, Project Glasswing, fjernstyr CLI fra nett og mobil, automatisk modellvalg i CLI, Copilot-planendringer og strammere bruksgrenser."
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
  - auto-model
  - pricing
  - rate-limits
  - breaking-change
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

## 17. Automatisk modellvalg i Copilot CLI

Copilot auto model selection er nå generelt tilgjengelig i Copilot CLI for alle planer. I stedet for å velge modell manuelt, kan du nå la Copilot velge den mest effektive modellen for hver forespørsel — dynamisk og transparent.

Auto ruter til modeller som GPT-5.4, GPT-5.3-Codex, Sonnet 4.6 og Haiku 4.5 basert på plan og administratorpolicyer. Modellutvalget vil endre seg over tid etter hvert som nye modeller blir tilgjengelige. Du ser alltid hvilken modell som ble brukt direkte i CLI-en, og kan bytte tilbake til manuelt valg når som helst.

Alle betalende abonnenter får 10 % rabatt på premium request-multiplikatoren når de bruker auto. For eksempel koster en modell med 1× multiplikator bare 0,9 premium requests via auto. Auto er begrenset til modeller med 0×–1× multiplikatorer, noe som også beskytter mot utilsiktet høyt forbruk.

**Kilde:** [GitHub Copilot CLI now supports Copilot auto model selection](https://github.blog/changelog/2026-04-17-github-copilot-cli-now-supports-copilot-auto-model-selection) (GitHub Changelog, 17. april 2026)

---

## 18. Endringer i Copilot-abonnementer for enkeltpersoner

GitHub annonserte 20. april omfattende endringer i Copilot-planene for individuelle brukere. Endringene er begrunnet med kapasitetsbehov og tjenestekvalitet, men har møtt sterk kritikk fra utviklermiljøet.

**Nye registreringer pauset.** Pro-, Pro+- og Student-planene aksepterer ikke nye brukere inntil videre. Copilot Free er fortsatt åpen, og eksisterende brukere kan oppgradere mellom planer. **Strammere bruksgrenser.** Pro+-planen tilbyr mer enn 5× grensene til Pro. Brukere som trenger høyere grenser må oppgradere. VS Code og Copilot CLI viser varsler når du nærmer deg grensen. **Opus fjernet fra Pro.** Opus-modeller er ikke lenger tilgjengelige på Copilot Pro. Opus 4.7 forblir på Pro+, men Opus 4.5 og 4.6 fjernes også fra Pro+ som tidligere varslet.

Brukere som ikke er fornøyde med endringene kan kansellere abonnementet og få refusjon for gjenværende tid frem til 20. mai.

**Kilde:** [Changes to GitHub Copilot plans for individuals](https://github.blog/changelog/2026-04-20-changes-to-github-copilot-plans-for-individuals) (GitHub Changelog, 20. april 2026)

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
| Auto model selection i CLI | Forenkler modellvalg for utviklere — la Copilot optimalisere. 10 % rabatt beskytter premium request-budsjett. Anbefal som standard for team som ikke har spesifikke modellpreferanser. |
| Copilot-planendringer | Nav bruker Enterprise — ikke direkte berørt. Men utviklere med personlige kontoer mister tilgang til Opus på Pro og får strammere grenser. Informer om endringene og at refusjon er mulig til 20. mai. |
