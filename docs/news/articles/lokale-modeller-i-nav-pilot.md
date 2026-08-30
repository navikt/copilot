---
title: "Lokale modeller i nav-pilot: hvorfor vi prøver, og hva vi trenger hjelp til"
date: 2026-08-30
author: starefossen
category: praksis
excerpt: "nav-pilot kan nå sende avgrensede oppgaver til en modell som kjører på din egen maskin. Alt vi vet så langt kommer fra lab, ikke fra folk som gjør jobben sin. Det er derfor vi kjører en alfa."
tags:
  - nav-pilot
  - local-models
  - mlx
  - cost
  - alpha
---

Vi bruker rundt 45 000 dollar i måneden på GitHub Copilot for 650 utviklere. Siden 1. juni betaler vi per token, i AI-credits, og de tyngste brukerne er tomme før måneden er det. En modell som kjører på din egen maskin trekker ingen credits. Ingen.

Det er den enkle motivasjonen. Den andre er at vi vil vite hva slike modeller faktisk duger til, mens vi fortsatt kan velge selv. Maskinvaren står allerede på pultene, modellene blir bedre for hvert kvartal, og vi vil heller ha et svar fra våre egne målinger enn fra en leverandørs benchmark.

`nav-pilot alpha local` er nå ute til testing.

## Hva det er

En lokal modell som hovedagenten kan sende avgrensede oppgaver til: slå opp noe i koden, legge til en kommentar, døpe om et symbol, tre et felt gjennom en mapper. Den planlegger ikke, fører ingen samtale og velger ikke hva som skal gjøres. Hovedagenten kjører fortsatt i skyen og bestemmer fortsatt alt.

```
nav-pilot alpha local init      # laster ned og setter opp
nav-pilot alpha local start
nav-pilot alpha local status
```

Modellen er `Qwen3.6-35B-A3B-OptiQ-4bit`. Den tar 23 GB på disk, rundt 21 GB i minnet mens den kjører, og krever en Mac med 48 GB.

## Hva vi har målt, og hvor lite det betyr

146 verifiserte målinger, på én maskin, mot ett Kotlin-repo. Kort fortalt: er arbeidet mekanisk og spesifisert på forhånd, gjør modellen det, og det koster ingenting.

Å tre et nytt felt gjennom en dataklasse, mapperen og alle kallstedene kostet 13 AI-credits med lokal utsending mot 34 uten. Testresultatet var likt. Det tok lengre tid: 156 sekunder mot 100. I en annen kjøring døpte modellen om 46 forekomster i 10 filer helt alene, uten skymodell inne i bildet, og prosjektet kompilerte etterpå.

Det nyttigste vi fant, er hvor skillet går: modellen utfører en avgjørelse godt og tar en avgjørelse dårlig. Ber du den skrive en ny testfil fra bunnen, gjør den ingenting.

**Men dette er lab.** Alle tallene over kommer fra enkeltoppgaver i et rent repo, på én maskin, uten avbrytelser, uten halvferdig arbeid liggende og uten en kollega som venter. Ingen har brukt dette en hel arbeidsdag på ekte kode. Vi vet ikke hvordan det oppfører seg når du har tre ting i gang samtidig, når repoet er stort og rotete, eller når du har dårlig tid.

Vi har heller ikke tall som betyr noe for Spring, som er mesteparten av det som står i produksjon i dag, eller for frontend.

## En ting vi trodde, og tok feil om

Vi så at hovedagenten aldri sendte visse oppgaver videre, og antok at den var for forsiktig. Så skrev vi om instruksjonen for å oppmuntre den til å sende mer. Den begynte å sende, og halvparten av de utsendte oppgavene ble feil.

Hovedagenten hadde rett. Den vet bedre enn oss når den lokale modellen bør få en oppgave, og vi har rullet endringen tilbake. Vi nevner det fordi det sier noe om hvor mye vi vet: nok til at det er verdt å prøve, ikke nok til å overstyre verktøyet.

## Det er derfor vi kjører en alfa

Poenget med alfaen er ikke å bekrefte tallene over, men å finne ut hva som skjer når folk bruker dette på ekte oppgaver, i repoer vi ikke har testet, med arbeidsvaner vi ikke har simulert.

Vi vil særlig vite:

- Om noe henger. Kjør `nav-pilot alpha local status` med en gang det skjer. Den kommandoen fant en hengt server riktig på første forsøk hos oss, og er det raskeste vi har for å skille «treg» fra «død».
- Om en endring kompilerer, men er feil på en måte du ikke ville ventet av en slurvete kollega.
- Om ventetiden er verdt det i praksis. Lokalt er gratis, men det er langsommere, og bare du vet om det er en god byttehandel midt i en arbeidsdag.
- Hva du prøvde å bruke det til som vi ikke har tenkt på.

Negative erfaringer er like nyttige som positive. Blir konklusjonen at dette ikke er verdt bryet i praksis, er det et helt greit svar, og bedre å få det nå enn om et år.

## Vil du være med?

Meld deg hvis du har en Mac med 48 GB minne og bruker opp AI-creditsene dine.

Alle målingene og metoden ligger i [navikt/mlx-workspace](https://github.com/navikt/mlx-workspace), inkludert kjøringene som gikk galt og feilene vi gjorde i selve måleoppsettet.
