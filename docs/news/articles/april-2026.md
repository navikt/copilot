---
title: "Nyheter og trender — April 2026"
date: 2026-05-01
author: starefosen
draft: false
category: oppsummering
excerpt: "Ny kostnadsmodell for Copilot fra 1. juni, EU-dataopphold med EFTA-dekning, Autopilot-modus i VS Code, gh skill CLI, Dependabot kan tildeles AI-agenter, Claude Opus 4.7 og GPT-5.5 GA, agentsesjoner i Issues og Projects."
tags:
  - billing
  - data-residency
  - vscode
  - skills
  - github-cli
  - coding-agents
  - models
  - dependabot
---

April 2026 var måneden GitHub la om kostnadsmodellen for Copilot, åpnet EU-dataopphold med Norge dekket via EFTA, og ga utviklere et helt nytt sett verktøy: Autopilot i VS Code, `gh skill` i terminalen, og to nye toppmodeller. Her er det viktigste for Nav-utviklere.

---

## 1. Ny kostnadsmodell for Copilot fra 1. juni

GitHub legger om hele faktureringsmodellen for Copilot 1. juni 2026. Premium request-enheter (PRU) erstattes av **GitHub AI Credits**, og forbruk beregnes per token (input, output, cache) etter publiserte API-rater for hver modell.

Planpriser endres ikke — Enterprise er fortsatt $39/bruker/mnd — og hver plan får AI Credits tilsvarende prisen. Code completions og Next Edit-forslag forblir inkludert og bruker ikke credits. For Business og Enterprise innføres «pooled usage», så ubrukte credits deles på tvers av organisasjonen.

**To ting Nav må forberede:**

- **Code review begynner å bruke Actions-minutter.** Fra 1. juni faktureres agentbasert code review både i AI Credits og i GitHub Actions-minutter fra eksisterende plan-entitlement for private repoer. Offentlige repoer er ikke berørt. Gjennomgå Actions-budsjett og vurder om dere trenger større runners eller self-hosted.
- **Budsjettkontroller på flere nivåer.** Administratorer får nye kontroller på enterprise-, kostnadssenter- og brukernivå. En «preview bill» kommer i mai for å gi innsyn i forventede kostnader.

Fallback til billigere modeller når kvota er brukt opp forsvinner — det er credits og budsjettregler som styrer bruken.

**Kilder:**

- [GitHub Copilot is moving to usage-based billing](https://github.blog/news-insights/company-news/github-copilot-is-moving-to-usage-based-billing/) (GitHub Blog, 27. april 2026)
- [Copilot code review will start consuming GitHub Actions minutes on June 1](https://github.blog/changelog/2026-04-27-github-copilot-code-review-will-start-consuming-github-actions-minutes-on-june-1-2026) (GitHub Changelog, 27. april 2026)

---

## 2. EU-dataopphold med Norge dekket via EFTA

Copilot støtter nå dataoppholdskrav for EU, og fra **1. mai 2026** er EFTA-land — inkludert Norge — eksplisitt dekket. All inferens og tilknyttede data forblir innenfor regionen, bygget på Microsofts EU Data Boundary.

Alle GA-funksjoner er støttet: agent mode, inline-forslag, chat, cloud agent, code review, PR-oppsummeringer og Copilot CLI. Modellene som er tilgjengelige ved lansering er GPT-5.4, Claude Sonnet 4.6 og Claude Opus 4.6. Gemini-modeller mangler fordi GCP ikke har dataresidente endepunkter ennå.

Data-residente forespørsler koster **10 % mer** i modellmultiplikator. Policyen aktiveres av Enterprise- eller organisasjonsadministrator — den er av som standard.

**Kilde:** [Data residency (US + EU) and FedRAMP-authorized models now available](https://github.blog/changelog/2026-04-13-copilot-data-residency-in-us-eu-and-fedramp-compliance-now-available/) (GitHub Changelog, 13. april 2026)

---

## 3. Autopilot-modus i VS Code

VS Code 1.115 introduserer **Autopilot** (public preview): agenten godkjenner egne handlinger, prøver på nytt ved feil og jobber til oppgaven er ferdig — uten at du trykker «godkjenn» underveis. Tillatelsesnivået settes per sesjon: Default, Bypass Approvals eller Autopilot.

Dette er en stor endring i hvordan agenter brukes til daglig. Sett tydelige grenser med hooks og instruksjoner før dere lar Autopilot kjøre løst — særlig på operasjoner som rører ekstern infrastruktur.

Andre nyttige nyheter fra mars-releasene (1.111–1.115):

- **Konfigurerbar tenke-innsats** for resonneringsmodeller (Claude Sonnet 4.6, GPT-5.4) direkte fra modellvelgeren
- **Sandbox for MCP-servere** lokalt på macOS og Linux
- **Monorepo-oppdagelse** av instruksjoner, agenter, skills og hooks oppover mappetreet
- **Agent-spesifikke hooks** via YAML-frontmatter i `.agent.md`

**Kilde:** [GitHub Copilot in Visual Studio Code, March Releases](https://github.blog/changelog/2026-04-08-github-copilot-in-visual-studio-code-march-releases/) (GitHub Changelog, 8. april 2026)

---

## 4. `gh skill` — agent skills fra terminalen

GitHub CLI v2.90.0 introduserer `gh skill` for å installere, oppdatere, publisere og søke etter agent skills. Kommandoen følger den åpne [agentskills.io-spesifikasjonen](https://agentskills.io/specification) og fungerer på tvers av Copilot, Claude Code, Cursor, Codex og Gemini CLI.

```bash
gh skill install github/awesome-copilot documentation-writer
gh skill install github/awesome-copilot documentation-writer --pin v1.2.0
gh skill update
```

Skills installeres i riktig mappe for din host. Provenance-metadata (repo, ref, tree SHA) skrives rett i SKILL.md-frontmatter, så sporbarhet følger fila.

For Nav er dette direkte relevant: alle utviklere har `gh` installert, og vi har 22 skills som allerede er agentskills.io-kompatible. `gh skill publish` med immutable releases er en god kandidat når vi flytter skills inn i `skills/`.

**Kilde:** [Manage agent skills with GitHub CLI](https://github.blog/changelog/2026-04-16-manage-agent-skills-with-github-cli) (GitHub Changelog, 16. april 2026)

---

## 5. Dependabot-varsler kan tildeles AI-agenter

Noen avhengighetssårbarheter krever mer enn en versjonsbump — de trenger kodeendringer på tvers av prosjektet. Nå kan du tildele Dependabot-varsler direkte til AI coding agents (Copilot, Claude, Codex). Agenten analyserer varselet, åpner en draft-PR og prøver å løse testfeil som oppstår.

Du kan tildele flere agenter til samme varsel — hver jobber uavhengig og åpner sin egen PR, så du kan sammenligne tilnærminger. Spesielt nyttig for major version-oppgraderinger med breaking changes, nedgradering av kompromitterte pakker, og komplekse scenarier som faller utenfor Dependabots regelmotor.

For Navs ~500 repoer kan dette akselerere sikkerhetsoppdateringer betraktelig. Krever GitHub Code Security og Copilot-abonnement med tilgang til coding agent.

**Kilde:** [Dependabot alerts are now assignable to AI agents for remediation](https://github.blog/changelog/2026-04-07-dependabot-alerts-are-now-assignable-to-ai-agents-for-remediation) (GitHub Changelog, 7. april 2026)

---

## 6. Nye modeller: Claude Opus 4.7 og GPT-5.5

To nye toppmodeller ble GA i Copilot i april:

- **Claude Opus 4.7** (16. april) — bedre på flertrinnsoppgaver og pålitelig agentisk utførelse enn forgjengeren. Erstatter Opus 4.5 og 4.6 i modellvelgeren.
- **GPT-5.5** (24. april) — sterkest ytelse på komplekse, flerstegs agentoppgaver i GitHubs egen testing.

Begge har **7,5× premium request-multiplikator**. Tilgjengelig i VS Code, Visual Studio, Copilot CLI, cloud agent, github.com, GitHub Mobile, JetBrains, Xcode og Eclipse. Enterprise-administrator må aktivere modellpolicyen.

Den høye prisen gjør at de bør spares til oppgaver der billigere modeller faktisk feiler. Test før du bytter standardmodell for hele teamet.

**Kilder:**

- [Claude Opus 4.7 is generally available](https://github.blog/changelog/2026-04-16-claude-opus-4-7-is-generally-available) (16. april 2026)
- [GPT-5.5 is generally available for GitHub Copilot](https://github.blog/changelog/2026-04-24-gpt-5-5-is-generally-available-for-github-copilot) (24. april 2026)

---

## 7. Agentsesjoner synlige i Issues og Projects

Cloud agent-sesjoner er nå synlige direkte i GitHub Issues og Projects. En «session pill» på issues viser aktive og fullførte agentsesjoner, og du kan åpne en sesjon i sidepanelet for å se fremdrift, gjennomgå logger eller gi agenten retning — uten å forlate issue-visningen.

I Projects er «Show agent sessions» aktivert som standard for både nye og eksisterende visninger. Klikk på en agentsesjon i prosjekttavlen for å åpne den i sidepanelet.

Dette gjør det lettere å holde oversikt over agentaktivitet i planleggingskonteksten der arbeidet allerede spores. Ingen konfigurasjon nødvendig.

**Kilde:** [View and manage agent sessions from issues and projects](https://github.blog/changelog/2026-04-23-view-and-manage-agent-sessions-from-issues-and-projects) (GitHub Changelog, 23. april 2026)

---

## Også verdt å vite

Korte oppdateringer som er nyttige å kjenne til, men som ikke krever handling fra de fleste:

- **Copilot SDK i public preview** — samme agentmotor som driver cloud agent og CLI, pakket som bibliotek for Node.js, Python, Go, .NET og Java. Relevant hvis Nav vil bygge egne agentverktøy. [Kilde](https://github.blog/changelog/2026-04-02-copilot-sdk-in-public-preview/)
- **Selektiv cloud agent-utrulling** — Enterprise-admin kan aktivere coding agent per organisasjon eller via custom properties. Bra for piloter. [Kilde](https://github.blog/changelog/2026-04-15-enable-copilot-cloud-agent-via-custom-properties)
- **Org-runner for cloud agent** — sentralstyrt runner-konfigurasjon med valgfri lås mot overstyring per repo. [Kilde](https://github.blog/changelog/2026-04-03-organization-runner-controls-for-copilot-cloud-agent/)
- **Cloud agent 20 % raskere oppstart** — bygger på 50 %-forbedringen fra mars. Samlet over 60 % raskere enn ved starten av 2026. [Kilde](https://github.blog/changelog/2026-04-27-copilot-cloud-agent-starts-20-faster-with-actions-custom-images)
- **PR-merge-metrikker** — Usage Metrics API har fått `pull_requests.total_merged_reviewed_by_copilot` og `median_minutes_to_merge_copilot_reviewed`. Nyttig for DORA-arbeid. [Kilde](https://github.blog/changelog/2026-04-08-copilot-reviewed-pull-request-merge-metrics-now-in-the-usage-metrics-api/)
- **Fjernstyr CLI fra nett og mobil** — `copilot --remote` lar deg følge med på en kjørende CLI-sesjon fra GitHub-nettsida eller mobilappen. [Kilde](https://github.blog/changelog/2026-04-13-remote-control-cli-sessions-on-web-and-mobile-in-public-preview)
- **BYOK og lokale modeller i Copilot CLI** — koble til Azure OpenAI, Anthropic eller lokale endepunkter (Ollama, vLLM). For team med særskilte krav og avklart governance. [Kilde](https://github.blog/changelog/2026-04-07-copilot-cli-now-supports-byok-and-local-models)
- **Personvernpolicy for treningsdata trådte i kraft 24. april** — gjelder kun Free, Pro og Pro+. Nav bruker Enterprise og er ikke berørt, men informer kollegaer som har personlig Copilot-konto om opt-out. [Kilde](https://github.blog/news-insights/company-news/updates-to-github-copilot-interaction-data-usage-policy/)
- **Metrikk-URL-endring 20. mai** — copilot-metrics må oppdateres til `copilot-reports.github.com`. [Kilde](https://github.blog/changelog/2026-04-22-upcoming-change-to-copilot-usage-metrics-report-download-urls)

---

## Relevans for Nav

| Trend | Hva det betyr for Nav |
| --- | --- |
| Ny kostnadsmodell 1. juni | Pool-baserte AI Credits erstatter PRU. Administratorer bør sette budsjettkontroller og følge med på «preview bill» i mai. |
| Code review bruker Actions-minutter | Budsjetter Actions-minutter for agentbasert code review i private repoer. Vurder runner-størrelse. |
| Data residency (EU/EFTA) | Norge er eksplisitt dekket fra 1. mai. Vurder kost/nytte mot 10 % prisøkning. |
| VS Code Autopilot | Test på avgrensede oppgaver med tydelige grenser via hooks og instruksjoner. |
| `gh skill` CLI | Alle Nav-utviklere har `gh`. Skills i `skills/` er klare for `gh skill publish` med immutable releases. |
| Dependabot + AI-agenter | Kan akselerere sikkerhetsoppdateringer i ~500 repoer, særlig major-oppgraderinger. |
| Opus 4.7 og GPT-5.5 | Spar til komplekse oppgaver der billigere modeller feiler. 7,5× multiplikator. |
| Agentsesjoner i Issues/Projects | Aktivert som standard. Gjør agentaktivitet synlig der arbeidet spores. |
