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

> **Testresultat 23. september:** Den første blokkeringsskjermen fant én GPT-6 Sol-kjøring av fem som hoppet over intervjuet og ga en løsningsanbefaling uten spørsmål. GPT-5.6 Sol fulgte fasekravene i fem av fem kjøringer. Opus 5.5 Medium oppga feil linjenumre i to av fem kodegjennomganger. GPT-6 Luna fulgte de avgrensede kravene i ti av ti oppgaver og brukte omtrent 45 prosent færre credits enn GPT-5.6 Luna. Funnene krever oppfølging, men er ikke store nok til å stoppe en kontrollert utrulling med de eldre modellene som fallback.

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

## Vi går videre med en kontrollert utrulling

GPT-6 Sol brøt stopp-og-vent-regelen i én av fem `nav-pilot`-kjøringer. Kontrollmodellen GPT-5.6 Sol fulgte regelen i fem av fem kjøringer. Dette er et negativt signal som vi skal følge, men ikke en bred kvalitetsregresjon: modellen fant personvern, tilgangskontroll og riktig TokenX-mønster i alle fem kjøringer. GPT-6 Sol brukte også færre credits og var raskere i deler av testen.

GPT-6 Luna er fortsatt den mest lovende kandidaten for avgrensede oppgaver. Den besto de samme ti smale kravene som GPT-5.6 Luna og brukte omtrent 45 prosent færre credits, med omtrent lik veggklokketid. Testen dekker ikke generell kodekvalitet eller lange agentoppgaver. Vi ruller derfor ut på `@research` og malpromptene med GPT-5.6 Luna som fallback.

Opus 5.5 Medium var billigere og raskere, men plasserte flere funn på feil linje i to av fem TSX-gjennomganger. High traff de plantede linjene i fem av fem kjøringer. Den kostet 12 prosent mer og var tregere enn Opus 5 High, men nøyaktigheten gjør High til et bedre valg for kodegjennomgang med høy risiko.

Vi går videre med de planlagte modellbyttene og følger feilrate, credit-forbruk og konkrete regresjonsrapporter. Prøv de nye modellene på oppgaver du kjenner godt, og meld fra om feil med prompt, modell, effort-nivå og forventet resultat. Vi beholder GPT-5.6 Luna, GPT-5.6 Sol og Claude Opus 5 som fallback. Vi slår dem ikke av før større tester viser at etterfølgerne ikke gir uønskede bivirkninger.

## Relevans for Nav

| Endring                                                       | Hva det betyr for Nav                                                                              |
| ------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| GPT-6 Luna koster mindre enn GPT-5.6 Luna                     | Lavere tokenpris må bekreftes mot kostnad per ferdig oppgave før vi bytter                         |
| GPT-6 Sol koster halvparten av GPT-5.6 Sol                    | Vi tester sikkerhetsagentens kvalitet før pris får avgjøre modellvalget                            |
| Opus 5.5 bruker færre steg og tokens i Anthropics egne tester | Leverandørpåstanden må bekreftes på våre lange plan- og reviewoppgaver                             |
| Opus 5.5 har alltid aktiv thinking og vannmerker tekst        | Team som integrerer modellen direkte må kontrollere API-endringer og krav til behandling av output |
| GPT-6 Sol brøt fasekravet i én av fem kjøringer              | Vi ruller ut med GPT-5.6 Sol som fallback og følger fasebrudd særskilt                              |
| GPT-6 Luna besto den smale testen med lavere credit-forbruk  | Vi bruker den på avgrensede research- og maloppgaver og følger kostnad per ferdig oppgave           |
| Opus 5.5 Medium oppga feil linjer i to av fem reviews        | Vi bruker High fremfor Medium på oppgaver der presise funn er viktig                                |
| Opus 5.5 High var dyrere og tregere enn Opus 5 High          | Vi ruller ut på den avgrensede høyrisikoagenten og beholder Opus 5 som fallback                     |
| Eldre modeller skal fases ut                                  | GPT-5.6 Luna, GPT-5.6 Sol og Claude Opus 5 slås først av etter en kontrollert overgang              |

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
