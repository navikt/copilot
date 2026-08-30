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

Vi bruker rundt 45 000 dollar i måneden på GitHub Copilot for 650 utviklere. Siden 1. juni betaler vi per token, i AI-credits, og de tyngste brukerne har brukt opp creditsene sine før måneden er omme. En modell som kjører på din egen maskin trekker ingen credits. Ingen.

Det er den enkle motivasjonen. Den andre er at vi vil vite hva slike modeller faktisk duger til, mens vi fortsatt kan velge selv. Maskinvaren står allerede på pultene, modellene blir bedre for hvert kvartal, og vi vil heller ha et svar fra våre egne målinger enn fra en leverandørs benchmark.

`nav-pilot alpha local` er nå klar til testing.

## Hva det er

En lokal modell som hovedagenten kan sende avgrensede oppgaver til: døpe om et symbol i mange filer, tre et felt gjennom en mapper og kallstedene. Små oppslag og enkeltkommentarer beholder hovedagenten selv, fordi den går raskere enn å sende dem videre. Den lokale modellen planlegger ikke, fører ingen samtale og velger ikke hva som skal gjøres.

```
nav-pilot alpha local init
```

Det er hele oppsettet. Den laster ned modellen, setter opp miljøet, hever minnegrensen i macOS (den spør om passordet ditt) og starter serveren.

Modellen er `Qwen3.6-35B-A3B-OptiQ-4bit`. Den tar rundt 26 GB på disk med Python-miljøet, holder 21 GB i minnet mens den kjører, og krever en Mac med Apple Silicon og 48 GB minne. `init` hever en minnegrense i macOS underveis og spør om passordet ditt når den gjør det.

## Hva vi har målt, og hvor lite det betyr

146 gyldige målinger, på én maskin, mot ett Kotlin-repo. Kort fortalt: er arbeidet mekanisk og spesifisert på forhånd, gjør modellen det oftest, og det koster ingen credits.

Å legge til et nytt felt i en dataklasse og oppdatere mapperen og alle kallstedene kostet 13 AI-credits med lokal utsending mot 34 uten. Testresultatet var likt. Det tok lengre tid: 156 sekunder mot 100. I en annen kjøring døpte modellen om 46 forekomster i 10 filer helt alene, uten skymodell inne i bildet, og prosjektet kompilerte etterpå.

**Men den besparelsen følger ikke med over til Spring.** Vi kjørte den samme oppgaven mot en Spring-app til slutt, og der kostet lokal utsending mer enn å la være: 15 credits mot 9, og det tok lengre tid. Motsatt fortegn av Kotlin-tallene, med samme modell og samme oppsett. Fire kjøringer per arm, så størrelsen på forskjellen vet vi ikke, men retningen er tydelig nok til å si det høyt: besparelsen er ikke en egenskap ved modellen alene, den henger sammen med kodebasen. Kvaliteten holdt begge steder.

Det nyttigste vi fant, er hvor skillet går: modellen er god til å gjennomføre en beslutning som allerede er tatt, og dårlig til å ta den selv. Ber du den skrive en ny testfil fra bunnen, gjør den ingenting.

**Men dette er lab.** Alle tallene over kommer fra enkeltoppgaver i et rent repo, på én maskin, uten avbrytelser, uten halvferdig arbeid i repoet og uten en kollega som venter. Ingen har brukt dette en hel arbeidsdag på ekte kode. Vi vet ikke hvordan det oppfører seg når du har tre ting i gang samtidig, når repoet er stort og rotete, eller når du har dårlig tid.

Vi har fire kjøringer på Spring og ingenting på frontend. Spring utgjør mesteparten av det som står i produksjon i dag, så det er der vi vet minst og trenger mest.

## En ting vi trodde, og tok feil om

Vi så at hovedagenten aldri sendte visse oppgaver videre, og antok at den var for forsiktig. Så skrev vi om instruksjonen for å oppmuntre den til å sende mer. Den sendte to oppgaver videre, og den ene kom tilbake med en test som ikke lenger virket. De seks kjøringene der den lot være, gikk alle bra.

I dette forsøket hadde hovedagenten rett, og vi rullet endringen tilbake. To utsendinger er et tynt grunnlag for en regel, og det er nettopp poenget: vi vet nok til at dette er verdt å prøve, ikke nok til å overstyre verktøyet.

## Det er derfor vi kjører en alfa

Poenget med alfaen er ikke å bekrefte tallene over, men å finne ut hva som skjer når folk bruker dette på ekte oppgaver, i repoer vi ikke har testet, med arbeidsvaner vi ikke har simulert.

Vi vil særlig vite:

- Om noe henger. Kjør `nav-pilot alpha local status` med en gang det skjer. Hos oss har den skilt «treg» fra «død», men den har bare vært prøvd én gang.
- Om en endring kompilerer, men er feil på en måte du ikke ville ventet av en slurvete kollega.
- Om ventetiden er verdt det i praksis. Lokal kjøring er gratis, men langsommere, og bare du vet om det er en god byttehandel midt i en arbeidsdag.
- Hva du prøvde å bruke det til som vi ikke har tenkt på.

Negative erfaringer er like nyttige som positive. Blir konklusjonen at dette ikke er verdt bryet i praksis, er det et helt greit svar, og bedre å få det nå enn om et år.

## Vil du være med?

Meld deg i #nav-pilot på Slack hvis du har en Mac med 48 GB minne og bruker opp AI-creditsene dine. Vi tar inn én om gangen i starten, så vi rekker å følge opp ordentlig.

Alle målingene og metoden ligger i [navikt/mlx-workspace](https://github.com/navikt/mlx-workspace), inkludert kjøringene som gikk galt og feilene vi gjorde i selve måleoppsettet.
