---
title: "Test GPT-6 Sol, GPT-6 Luna og Claude Opus 5.5 før modellbyttet"
date: 2026-09-22
category: copilot
excerpt: "Tre nye modeller er tilgjengelige for utprøving. Vi utsetter endringen av standardmodellene mens vi undersøker tidlige regresjonsrapporter."
tags:
  - models
  - gpt
  - claude
  - coding-agents
---

GPT-6 Sol, GPT-6 Luna og Claude Opus 5.5 er nå slått på i GitHub Copilot for Nav. Vi anbefaler at de fleste prøver modellene på egne oppgaver nå. Utrullingen er gradvis, så modellene kan mangle i modellvelgeren en kort stund.

> **Oppdatert 23. september:** Vi utsetter endringen av standardmodellene mens vi undersøker tidlige regresjonsrapporter. Én GPT-6 Sol-bruker på Hacker News gikk tilbake til GPT-5.6 Sol etter vesentlig dårligere resultater. En Opus 5.5-bruker fant fire feil linjenumre og to overdrevne funn i en kodegjennomgang på Medium effort. Dette er enkelterfaringer, ikke dokumentasjon på en generell regresjon, men de er konkrete nok til at vi tester før vi bytter.

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

Hacker News-trådene fikk raskt flere hundre kommentarer. Reaksjonene er positive til prisfallet, men langt mer avventende til ytelsespåstandene. Flere peker på at pris per token ikke sier hva en ferdig oppgave koster. En modell som bruker flere reasoning-tokens, fyller kontekstvinduet raskere eller trenger flere forsøk, kan bli dyrere selv med lavere listepris.

De første GPT-6-kommentarene trekker i ulike retninger. Noen ser Luna som billig nok til lange rutinejobber. Andre mener den forrige Luna-modellen var for svak, brukte for mange tokens og mistet kodekvalitet i lange, uovervåkede oppgaver. Sol får ros som mulig implementeringsmodell, men flere vil se egne tester før de bytter fra GPT-5.6 Sol eller Claude. Det er også uklart om lavere API-pris gir høyere kvoter i abonnementene.

Opus 5.5 møter særlig skepsis rundt skrivestil og benchmarks. Flere tidlige brukere mener modellen fortsatt begraver viktige poenger i lange, garderte svar, selv om «Claude-frasene» er mindre tydelige. Andre tviler på at leverandørens Fable-sammenligning sier nok om praktisk bruk. Positive kommentarer peker på lavere pris, færre tokens og de store migreringseksemplene, men disse eksemplene kommer foreløpig fra Anthropic og utvalgte testere.

Reddit ga ingen verifiserbare, indekserte diskusjoner om de nye modellene på lanseringskvelden. Det finnes nå diskusjoner om ujevn utrulling, modellvalg og kostnad, men få dokumenterte sammenligninger fra reelle kodebaser. Flere innlegg gjentar leverandørenes priser og benchmarks uten egne målinger. Vi bruker derfor ikke Reddit-reaksjonene som grunnlag for modellbyttet.

## Standardmodellbyttet venter på testresultater

Vi beholder dagens modellvalg mens vi tester GPT-6 Sol mot GPT-5.6 Sol og følger utrullingen av de andre modellene. Bruk de nye modellene på oppgavene du vanligvis løser, og meld fra hvis du finner blokkeringer eller tydelige regresjoner. Vi følger særlig med på kodekvalitet, instruksjonsfølging, sikkerhetsvurderinger, tokenforbruk og antall forsøk per ferdig oppgave.

Hvis testene ikke finner blokkeringer, flytter vi `@research` og de enkle malpromptene til GPT-6 Luna, `@security-champion` til GPT-6 Sol og `@nav-pilot-opus` til Claude Opus 5.5. Deretter slår vi av GPT-5.6 Luna, GPT-5.6 Sol og Claude Opus 5 i modellpolicyen. De eldre modellene forblir tilgjengelige som fallback til vi har kontrollert at overgangen ikke gir uønskede bivirkninger.

## Relevans for Nav

| Endring                                                       | Hva det betyr for Nav                                                                              |
| ------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| GPT-6 Luna koster mindre enn GPT-5.6 Luna                     | Lavere tokenpris må bekreftes mot kostnad per ferdig oppgave før vi bytter                         |
| GPT-6 Sol koster halvparten av GPT-5.6 Sol                    | Vi tester sikkerhetsagentens kvalitet før pris får avgjøre modellvalget                            |
| Opus 5.5 bruker færre steg og tokens i Anthropics egne tester | Leverandørpåstanden må bekreftes på våre lange plan- og reviewoppgaver                             |
| Opus 5.5 har alltid aktiv thinking og vannmerker tekst        | Team som integrerer modellen direkte må kontrollere API-endringer og krav til behandling av output |
| Tidlige brukererfaringer spriker                              | Vi anbefaler at de fleste prøver modellene nå og melder fra om blokkeringer                        |
| Standardmodellbyttet er utsatt                                | Vi tester konkrete regresjonsrapporter før vi endrer modellvalget                                  |
| Eldre modeller skal fases ut                                  | GPT-5.6 Luna, GPT-5.6 Sol og Claude Opus 5 slås av etter at vi har kontrollert overgangen          |

**Kilder:**

- [OpenAI’s GPT-6 Sol and GPT-6 Luna now available](https://github.blog/changelog/2026-09-22-openais-gpt-6-sol-and-gpt-6-luna-now-available/) (GitHub Changelog, 22. september 2026)
- [Claude Opus 5.5 is now available in GitHub Copilot](https://github.blog/changelog/2026-09-22-claude-opus-5-5-is-now-available-in-github-copilot/) (GitHub Changelog, 22. september 2026)
- [Claude Opus 5.5](https://www.anthropic.com/claude-opus-5-5) (Anthropic, 22. september 2026)
- [Claude Opus 5.5 model overview](https://platform.claude.com/docs/en/models/opus-5-5/overview) (Anthropic, 22. september 2026)
- [Models and pricing for GitHub Copilot](https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing) (GitHub Docs, lest 22. september 2026)
- [GPT-6 Sol and Luna](https://news.ycombinator.com/item?id=49805509) (Hacker News, lest 22. september 2026)
- [Claude Opus 5.5](https://news.ycombinator.com/item?id=49803892) (Hacker News, lest 22. september 2026)
- [GPT-6 Sol: bruker gikk tilbake til GPT-5.6 Sol](https://news.ycombinator.com/item?id=49811448) (Hacker News, lest 23. september 2026)
- [Opus 5.5: feil linjenumre i kodegjennomgang](https://news.ycombinator.com/item?id=49810898) (Hacker News, lest 23. september 2026)
- [GPT-6 Sol og Luna mangler i Codex-utvidelsen](https://www.reddit.com/r/OpenAI/comments/1wnwm69/gpt_6_sol_luna_not_available_on_codex_extension/) (Reddit, lest 23. september 2026)
- [Diskusjon om kostnad per ferdig oppgave](https://www.reddit.com/r/OpenaiCodex/comments/1wnwivc/stop_comparing_6sol_to_opus_55_the_correct/) (Reddit, lest 23. september 2026)
