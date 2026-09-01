---
title: "Nav-pilot lander på bakken"
date: 2026-08-30
featured: true
author: starefossen
category: praksis
excerpt: "En AI-modell på din egen maskin tar de mekaniske oppgavene. Den bruker ingen kreditter, men den er tregere, og den tar ingen avgjørelser."
tags:
  - nav-pilot
  - local-models
  - mlx
  - cost
  - alpha
---

For å få Nav sitt KI-token-budsjett til å strekke lenger, og for at de ivrigste brukerne ikke skal gå tom før måneden er omme, lanserer vi nå `nav-pilot` på bakken.

`nav-pilot alpha local` lar deg kjøre en KI-modell på din egen maskin basert på Qwen-familien etter en lang prosess med testing KI-labben vår. Hovedagenten sender konkrete oppgaver som en del av arbeidet, for at de skal utføres lokalt.

Vi kaller den bakkemodellen. Hovedagenten blir i skya og bestemmer, bakkemodellen utfører. I logger og konfigurasjon heter den `local-worker`.

```
nav-pilot alpha local init
```

Det er hele oppsettet. Krever Mac med Apple Silicon og 48 GB minne. Modellen tar 26 GB på disk og 21 GB i minnet mens den kjører. `init` spør om passordet ditt for å heve en minnegrense i macOS.

## Hva det er

Bakkemodellen får oppgaver der beslutningen allerede er tatt: rename av et symbol på tvers av filer, eller et nytt felt som skal gjennom en mapper og alle call sites.

Vi har også kjørt den alene, uten hovedagent over seg. Den klarte en rename av 46 references i 10 filer i begge forsøkene, og prosjektet kompilerte alle gangene.

## Hva det ikke er

Ikke en erstatning. Bakkemodellen planlegger ikke, tar ingen avgjørelser, og du kan ikke diskutere med den.

Ikke noe som skriver ny kode. Ber du om en ny testfil fra bunnen, gjør den ingenting. Den er god til å gjennomføre en beslutning og dårlig til å ta den.

Ikke raskere. Som regel tregere.

Og ingen har brukt den en hel arbeidsdag ennå.

## Hva vi vet

Først prøvde vi åtte modeller og bygg på det samme oppgavesettet: elleve oppgaver med oppslag, redigering og endringer over flere filer.

- **Qwen3.6-35B-A3B, OptiQ 4-bit.** Den vi valgte. Løste flest oppgaver, og gikk aldri i loop.
- **Qwen3.6-35B-A3B, vanlig 4-bit.** Samme modell, annet bygg. Gikk i tool-loop og brukte 220 kall på én oppgave.
- **KAT-Coder V2.5.** Løste like mange, men trengte flere kall på å komme dit.
- **Qwen3.6-27B.** For treg. Median 113 sekunder mot 18.
- **Qwen3.8-27B i 4-, 6- og 8-bit.** Ikke standard, men 4-bit og 8-bit kan nå velges. Se under.
- **Granite 4.1 8B.** For liten. Løste 1 av 8.

### Oppdatering 1. september: Qwen 3.8 kan velges

Vi skrev først at Qwen3.8 ikke ble valgt fordi den gikk i loop. Etter å ha kjørt testene på nytt
holder ikke den beskrivelsen, og den nye er mer interessant.

Fem kjøringer av det samme oppgavesettet på samme maskin: Qwen3.8-27B 4-bit løste **1, 5, 5, 6 og
7 av 8**. I snitt 4,8 mot standardmodellens 3,4 — rundt 41 % flere oppgaver. Standarden løser 3–4
av 8 og er omtrent sju ganger raskere.

**Qwen3.8 er altså ikke en svakere modell. Den er den sterkere, og den mer upålitelige.** Den
timet ut 11 ganger over de fem kjøringene der standarden timet ut 2, og spennet fra 1 til 7 er på
identisk oppsett. Valget står ikke mellom bedre og dårligere, men mellom sterkere og ujevn, eller
svakere og forutsigbar. Leser du gjennom det modellen lager, er 3.8 verdt å prøve. Vil du stole på
den uten å se etter, behold standarden. Derfor kan du velge den, men den er ikke standard:

```bash
nav-pilot models
nav-pilot config set model mlx-community/Qwen3.8-27B-4bit
nav-pilot alpha local init
```

Det vi selv lærte av dette er verdt mer enn modellvalget: **én kjøring er ikke en måling.** Alle
konklusjonene som snudde, snudde fordi det fantes en kjøring til — ikke fordi vi tenkte oss om en
gang til. Tabellen over var bygget på enkeltkjøringer. Denne konklusjonen snudde to ganger: først
fra «3.8 går i loop» til «3.8 er uforutsigbar», og så ved fem kjøringer til «3.8 er den sterkeste
modellen vi har». Vi krever nå minst fem kjøringer før et tall får styre en anbefaling.

Deretter 200 kjøringer på én maskin med modellen vi valgte, fordelt på to klienter, seks oppgavetyper, tre refactor-strategier og tre kodebaser: en Ktor-app, en Spring-app og en frontend.

I Ktor-repoet kostet oppgaven 13 AI-credits med lokal utsending, mot 34 uten. Testene ga samme resultat. Til gjengjeld tok det 156 sekunder mot 100.

I Spring-repoet snudde det: 16 credits mot 9. Der ble det dyrere å kjøre lokalt, med samme modell og samme oppsett.

Vi trodde en stund det var kodebasen som avgjorde. Det er det ikke. Det som avgjør, er hvor mange steg skymodellen trenger når den gjør jobben alene:

![Jo flere steg skymodellen trenger alene, jo mer sparer du på å sende arbeidet til bakkemodellen. 19 steg sparer 61 prosent, 13 steg sparer 47 prosent, 5 steg sparer 20 prosent, og på 2 steg koster utsendingen 79 prosent mer enn den sparer.](/images/nav-pilot-step-count.svg)

| Skymodellen alene | Med bakkemodellen |
|---|---|
| 19 steg | sparer 61 % |
| 13 steg | sparer 47 % |
| 5 steg | sparer 20 % |
| 2 steg | **koster 79 % mer** |

Jo mer skymodellen må rote seg fram, jo mer sparer du på å sende det mekaniske ned på bakken. Går oppgaven unna på to steg, koster utsendingen mer enn den sparer.

Frontend-tallet, det på 13 steg, er det eneste vi målte etter at vi hadde skrevet ned hva vi trodde ville skje. Vi bommet ikke.

Alt dette er målt i lab: enkeltoppgaver, rent repo, ingen avbrytelser, ingen som venter på deg.

## Hva vi trenger fra deg

- Kjør `nav-pilot alpha local status` med en gang noe henger.
- Si fra hvis en endring kompilerer, men er feil på en måte selv en slurvete kollega ikke ville levert.
- Si fra om ventetiden er verdt det midt i arbeidsdagen. Det er det bare du som vet.

«Ikke verdt bryet» er et like nyttig svar som det motsatte, og bedre å få nå enn om et år.

Så lenge dette er alfa, måler vi tettere enn i resten av nav-pilot: hvor mange oppgaver hver økt sender til bakkemodellen, hvilken modell som kjører, oppstartstid og når serveren henger. Vi samler aldri inn koden din eller det du skriver. `DO_NOT_TRACK=1` skrur alt av.

## Bli med

Meld deg i #nav-pilot. Vi tar inn én og én i starten.

Hele rapporten, med metode og alle tallene: [local-inference-findings.md](https://github.com/navikt/mlx-workspace/blob/main/reports/local-inference-findings.md). Hvorfor akkurat denne modellen, og hva vi forkastet: [alpha-model-decision.md](https://github.com/navikt/mlx-workspace/blob/main/reports/alpha-model-decision.md). Rådataene ligger i [navikt/mlx-workspace](https://github.com/navikt/mlx-workspace), også kjøringene som gikk galt.

> **Rettelse 31. august:** Saken oppga først at 8-bit gikk i timeout på 8 av 11 oppgaver, og forklarte deretter tallet med en chat-mal vi skrev selv, men verken tallet eller den forklaringen kan vi stå inne for.
