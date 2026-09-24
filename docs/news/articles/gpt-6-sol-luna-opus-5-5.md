---
title: "GPT-6 Sol er standardmodellen i nav-pilot"
date: 2026-09-22
category: copilot
excerpt: "GPT-6 Sol er standard i Copilot, opencode og pi. Luna og Opus 5.5 er valgt for enkelte agenter ved direkte start."
tags:
  - models
  - gpt
  - claude
  - coding-agents
---

GPT-6 Sol er nå standardmodellen i nav-pilot når du ikke har valgt modell selv. Standarden gjelder Copilot, opencode og pi. En brukerpinne vinner fortsatt.

Ved direkte start bruker `@research` GPT-6 Luna, mens `@nav-pilot-opus` og `@code-review` bruker Claude Opus 5.5. Seks scaffold-prompts er også satt til Luna. I Copilot CLI arver subagenter normalt hovedagentens modell, så delegering bytter ikke automatisk til agentens egen modellpinne.

## Slik posisjonerer leverandørene modellene

| Modell              | Kategori    | Pris per 1M tokens          | Leverandørens tiltenkte bruk                                                |
| ------------------- | ----------- | --------------------------- | --------------------------------------------------------------------------- |
| **GPT-6 Luna**      | Lightweight | $0.10 input / $0.50 output  | Raske rutineoppgaver, søk, dokumentasjon og faste maler                     |
| **GPT-6 Sol**       | Powerful    | $2.00 input / $10.00 output | Daglig agentisk koding som krever validering i flere steg                   |
| **Claude Opus 5.5** | Powerful    | $4.00 input / $20.00 output | Lange agentoppgaver, store migreringer, revisjoner og høyrisiko planlegging |

Prisene gjelder standard kontekst. GPT-6-modellene har egne, høyere priser over 272K input-tokens.

OpenAI beskriver GPT-6 Sol som et balansert valg for interaktiv og agentisk koding, og Luna som familiens raskeste og billigste modell. Sammenlignet med GPT-5.6-modellene halverer Luna inputprisen og reduserer outputprisen fra $1.20 til $0.50. Sol halverer både input- og outputprisen. Vi har ikke verifisert at lavere tokenpris gir lavere kostnad per ferdig oppgave.

Anthropic oppgir at Opus 5.5 bruker færre steg og tokens enn Opus 5, og henter seg raskt inn etter feil i flerstegsoppgaver. Selskapet oppgir også 1M kontekstvindu, opptil 128K output-tokens og alltid aktiv adaptiv thinking. Modellen vannmerker tekst den genererer. Vannmerket endrer ikke innholdet og legger ikke til tokens.

## Tidlige reaksjoner: lavere pris er ikke det samme som lavere kostnad

De lenkede Hacker News- og Reddit-trådene diskuterer kostnad per ferdig oppgave, kodekvalitet, skrivestil og modelltilgang. Dette er enkeltreaksjoner, ikke kontrollerte sammenligninger. Vi bruker dem ikke som dokumentasjon på at én modell er bedre eller billigere i praktisk bruk.

## Slik er utrullingen satt opp

GPT-6 Sol brøt stopp-og-vent-regelen i én av fem `nav-pilot`-kjøringer. Kontrollmodellen GPT-5.6 Sol fulgte regelen i fem av fem. GPT-6 Sol fant likevel personvern, tilgangskontroll og riktig TokenX-mønster i alle fem kjøringer.

Agentpakken bruker GPT-6 Sol som standard i Copilot, opencode og pi. `@security-champion`, `@kafka` og `@rust` bruker også Sol ved direkte start. GPT-5.6 Sol og GPT-5.3-Codex beholdes som fallbacks.

GPT-6 Luna besto de samme ti smale kravene som GPT-5.6 Luna og brukte omtrent 45 prosent færre credits, med omtrent lik veggklokketid. `@research` og seks scaffold-prompts bruker Luna. GPT-5.6 Luna er fallback.

Opus 5.5 Medium var billigere og raskere, men plasserte funn på feil linje i to av fem TSX-gjennomganger. High traff de plantede linjene i fem av fem. `@nav-pilot-opus` og `@code-review` bruker derfor Opus 5.5, med High effort anbefalt for kodegjennomgang. Claude Opus 5 og GPT-5.3-Codex er fallbacks.

Fem kjøringer per testarm i Copilot CLI er en liten blokkeringsskjerm, ikke dokumentasjon på generell forbedring eller på ytelsen i opencode og pi. Kafka- og Rust-agentene samt `kafka-topic` og `nais-manifest` ble ikke målt direkte. Meld regresjoner med prompt, modell, effort-nivå og forventet resultat.

## Relevans for Nav

| Endring                                                       | Hva det betyr for Nav                                                                                             |
| ------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| GPT-6 Luna koster mindre enn GPT-5.6 Luna                     | Vi bruker den på avgrensede oppgaver og måler fortsatt kostnad per ferdig oppgave                                 |
| GPT-6 Sol koster halvparten av GPT-5.6 Sol                    | Den er standard i nav-pilot, med GPT-5.6 Sol og GPT-5.3-Codex som fallbacks                                       |
| Opus 5.5 bruker færre steg og tokens i Anthropics egne tester | Leverandørpåstanden må bekreftes på våre lange plan- og reviewoppgaver                                            |
| Opus 5.5 har alltid aktiv thinking og vannmerker tekst        | Team som integrerer modellen direkte må kontrollere API-endringer og krav til behandling av output                |
| GPT-6 Sol brøt fasekravet i én av fem kjøringer               | Den er rullet ut med GPT-5.6 Sol som fallback, og vi følger fasebrudd særskilt                                    |
| GPT-6 Luna besto den smale testen med lavere credit-forbruk   | Vi bruker den på avgrensede research- og maloppgaver og følger kostnad per ferdig oppgave                         |
| Opus 5.5 Medium oppga feil linjer i to av fem reviews         | Velg High når klienten støtter det. Agentfila håndhever ikke effort-nivået, og linjene må kontrolleres mot diffen |
| Opus 5.5 High var dyrere og tregere enn Opus 5 High           | Den brukes på den avgrensede høyrisikoagenten, med Opus 5 som fallback                                            |
| Eldre modeller skal fases ut                                  | GPT-5.6 Luna, GPT-5.6 Sol og Claude Opus 5 slås først av etter en kontrollert overgang                            |

**Kilder:**

- [OpenAI’s GPT-6 Sol and GPT-6 Luna now available](https://github.blog/changelog/2026-09-22-openais-gpt-6-sol-and-gpt-6-luna-now-available/) (GitHub Changelog, 22. september 2026)
- [Claude Opus 5.5 is now available in GitHub Copilot](https://github.blog/changelog/2026-09-22-claude-opus-5-5-is-now-available-in-github-copilot/) (GitHub Changelog, 22. september 2026)
- [Claude Opus 5.5](https://www.anthropic.com/claude-opus-5-5) (Anthropic, 22. september 2026)
- [Claude Opus 5.5 model overview](https://platform.claude.com/docs/en/models/opus-5-5/overview) (Anthropic, 22. september 2026)
- [Models and pricing for GitHub Copilot](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing) (GitHub Docs, lest 22. september 2026)
- [Navs modellvalg og blokkeringsskjerm](../../modellvalg.md#blokkeringsskjerm-for-gpt-6-og-opus-55) (målt 23. september 2026)
- [GPT-6 Sol and Luna](https://news.ycombinator.com/item?id=49805509) (Hacker News, lest 22. september 2026)
- [Claude Opus 5.5](https://news.ycombinator.com/item?id=49803892) (Hacker News, lest 22. september 2026)
- [GPT-6 Sol: bruker gikk tilbake til GPT-5.6 Sol](https://news.ycombinator.com/item?id=49811448) (Hacker News, lest 23. september 2026)
- [Opus 5.5: feil linjenumre i kodegjennomgang](https://news.ycombinator.com/item?id=49810898) (Hacker News, lest 23. september 2026)
- [GPT-6 Sol og Luna mangler i Codex-utvidelsen](https://www.reddit.com/r/OpenAI/comments/1wnwm69/gpt_6_sol_luna_not_available_on_codex_extension/) (Reddit, lest 23. september 2026)
- [Diskusjon om kostnad per ferdig oppgave](https://www.reddit.com/r/OpenaiCodex/comments/1wnwivc/stop_comparing_6sol_to_opus_55_the_correct/) (Reddit, lest 23. september 2026)

_Oppdatert 24. september 2026 etter at modellvalgene ble rullet ut i nav-pilot._
