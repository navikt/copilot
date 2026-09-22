---
title: "GPT-6 Sol, GPT-6 Luna og Claude Opus 5.5 er tilgjengelige"
date: 2026-09-22
category: copilot
excerpt: "Tre nye modeller er slått på i Copilot for Nav. GPT-6 halverer listeprisen i Sol- og Luna-klassene, mens Opus 5.5 retter seg mot lange og krevende agentoppgaver."
tags:
  - models
  - gpt
  - claude
  - coding-agents
---

GPT-6 Sol, GPT-6 Luna og Claude Opus 5.5 er nå slått på i GitHub Copilot for Nav. Utrullingen er gradvis, så modellene kan mangle i modellvelgeren en kort stund.

## Modellene dekker tre ulike behov

| Modell              | Kategori    | Pris per 1M tokens          | Bruk den til                                                                |
| ------------------- | ----------- | --------------------------- | --------------------------------------------------------------------------- |
| **GPT-6 Luna**      | Lightweight | $0.10 input / $0.50 output  | Raske rutineoppgaver, søk, dokumentasjon og faste maler                     |
| **GPT-6 Sol**       | Powerful    | $2.00 input / $10.00 output | Daglig agentisk koding som krever validering i flere steg                   |
| **Claude Opus 5.5** | Powerful    | $4.00 input / $20.00 output | Lange agentoppgaver, store migreringer, revisjoner og høyrisiko planlegging |

Prisene gjelder standard kontekst. GPT-6-modellene har egne, høyere priser over 272K input-tokens.

GPT-6 Sol er et balansert valg for interaktiv og agentisk koding. GPT-6 Luna er familiens raskeste og billigste modell. Sammenlignet med GPT-5.6-modellene halverer Luna inputprisen og reduserer outputprisen fra $1.20 til $0.50. Sol halverer både input- og outputprisen.

Opus 5.5 bruker ifølge tidlig testing færre steg og tokens enn Opus 5, og henter seg raskt inn etter feil i flerstegsoppgaver. Anthropic oppgir 1M kontekstvindu, opptil 128K output-tokens og alltid aktiv adaptiv thinking. Modellen vannmerker tekst den genererer. Vannmerket endrer ikke innholdet og legger ikke til tokens.

## Tidlige reaksjoner: lavere pris er ikke det samme som lavere kostnad

Hacker News-trådene fikk raskt flere hundre kommentarer. Reaksjonene er positive til prisfallet, men langt mer avventende til ytelsespåstandene. Flere peker på at pris per token ikke sier hva en ferdig oppgave koster. En modell som bruker flere reasoning-tokens, fyller kontekstvinduet raskere eller trenger flere forsøk, kan bli dyrere selv med lavere listepris.

De første GPT-6-kommentarene trekker i ulike retninger. Noen ser Luna som billig nok til lange rutinejobber. Andre mener den forrige Luna-modellen var for svak, brukte for mange tokens og mistet kodekvalitet i lange, uovervåkede oppgaver. Sol får ros som mulig implementeringsmodell, men flere vil se egne tester før de bytter fra GPT-5.6 Sol eller Claude. Det er også uklart om lavere API-pris gir høyere kvoter i abonnementene.

Opus 5.5 møter særlig skepsis rundt skrivestil og benchmarks. Flere tidlige brukere mener modellen fortsatt begraver viktige poenger i lange, garderte svar, selv om «Claude-frasene» er mindre tydelige. Andre tviler på at leverandørens Fable-sammenligning sier nok om praktisk bruk. Positive kommentarer peker på lavere pris, færre tokens og de store migreringseksemplene, men disse eksemplene kommer foreløpig fra Anthropic og utvalgte testere.

Reddit ga ingen verifiserbare, indekserte diskusjoner om de nye modellene på lanseringskvelden. Det er for tidlig å kalle fraværet positivt eller negativt. Vi oppdaterer vurderingen når det finnes konkrete erfaringer med kodebaser, agentløp og kostnad per ferdig oppgave.

## Standardmodellene våre er oppdatert

Vi flytter `@research` og de fire enkle malpromptene fra GPT-5.6 Luna til GPT-6 Luna. `@security-champion` flyttes fra GPT-5.6 Sol til GPT-6 Sol. `@nav-pilot-opus` flyttes til Claude Opus 5.5, som er laget for høyrisiko planlegging og kritisk review. Vi har ikke målt de tre agentene mot de nye modellene ennå.

Pinnene gjelder når agenten eller prompten startes direkte. Subagenter arver fortsatt modellen fra foreldreagenten hvis klienten ikke har en egen overstyring.

## Relevans for Nav

| Endring                                                          | Hva det betyr for Nav                                                                              |
| ---------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| GPT-6 Luna koster mindre enn GPT-5.6 Luna                        | Vi kan bruke den til lesing, søk og malbaserte oppgaver med lavere kostnad                         |
| GPT-6 Sol koster halvparten av GPT-5.6 Sol                       | Sikkerhetsagenten får en nyere modell til lavere listepris                                         |
| Opus 5.5 bruker færre steg og tokens enn Opus 5 i tidlig testing | Lange migreringer og kritisk review kan bli raskere og enklere å kontrollere                       |
| Opus 5.5 har alltid aktiv thinking og vannmerker tekst           | Team som integrerer modellen direkte må kontrollere API-endringer og krav til behandling av output |
| Tidlige brukererfaringer spriker                                 | Vi bør måle kostnad per ferdig oppgave, kodekvalitet og antall forsøk før flere agenter flyttes    |

**Kilder:**

- [OpenAI’s GPT-6 Sol and GPT-6 Luna now available](https://github.blog/changelog/2026-09-22-openais-gpt-6-sol-and-gpt-6-luna-now-available/) (GitHub Changelog, 22. september 2026)
- [Claude Opus 5.5 is now available in GitHub Copilot](https://github.blog/changelog/2026-09-22-claude-opus-5-5-is-now-available-in-github-copilot/) (GitHub Changelog, 22. september 2026)
- [Claude Opus 5.5](https://www.anthropic.com/claude-opus-5-5) (Anthropic, 22. september 2026)
- [Claude Opus 5.5 model overview](https://platform.claude.com/docs/en/models/opus-5-5/overview) (Anthropic, 22. september 2026)
- [Models and pricing for GitHub Copilot](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing) (GitHub Docs, lest 22. september 2026)
- [GPT-6 Sol and Luna](https://news.ycombinator.com/item?id=49805509) (Hacker News, lest 22. september 2026)
- [Claude Opus 5.5](https://news.ycombinator.com/item?id=49803892) (Hacker News, lest 22. september 2026)
