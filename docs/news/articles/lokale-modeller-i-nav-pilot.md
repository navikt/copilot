---
title: "Lokal modell i nav-pilot: alfa"
date: 2026-08-30
author: starefossen
category: praksis
excerpt: "Dere vil ha flere tokens. Vi har ikke mer budsjett i år. Dette er svaret i mellomtiden: en modell som kjører på din egen maskin, for det mekaniske arbeidet."
tags:
  - nav-pilot
  - local-models
  - mlx
  - cost
  - alpha
---

Dere vil ha flere tokens. Vi har ikke mer budsjett i år.

Dette er svaret i mellomtiden. `nav-pilot alpha local` kjører en modell på din egen maskin. Hovedagenten sender den mekaniske delen av jobben dit, og den delen koster ingenting.

```
nav-pilot alpha local init
```

Det er hele oppsettet. Krever Mac med Apple Silicon og 48 GB minne. Modellen tar 26 GB på disk og 21 GB i minnet mens den kjører. `init` spør om passordet ditt for å heve en minnegrense i macOS.

## Hva det er

Hovedagenten er den samme som før, i skyen. Den sender bort arbeid som allerede er bestemt: døpe om et symbol på tvers av filer, tre et nytt felt gjennom mapperen og kallstedene. Én kjøring døpte om 46 forekomster i 10 filer uten skymodell inne i bildet, og prosjektet kompilerte.

## Hva det ikke er

Ikke en erstatning for skymodellen. Den lokale modellen planlegger ikke, tar ingen beslutninger og fører ingen samtale.

Ikke noe som skriver ny kode. Ber du om en ny testfil fra bunnen, gjør den ingenting. Den gjennomfører en beslutning godt og tar den dårlig.

Ikke raskere. Som regel tregere.

Ikke prøvd av noen som har brukt den en hel arbeidsdag.

## Hva vi vet

183 målinger, én maskin, to Kotlin-repoer.

I Ktor-repoet kostet en oppgave 13 AI-credits med lokal utsending mot 34 uten. Samme testresultat. Det tok 156 sekunder mot 100.

I Spring-repoet snudde det. 16 credits mot 9, altså dyrere lokalt, med samme modell og samme oppsett. Besparelsen henger sammen med kodebasen, ikke bare med modellen. Spring er mesteparten av det vi har i produksjon, så det er der vi vet minst.

Alt dette er lab. Enkeltoppgaver, rent repo, ingen avbrytelser, ingen som venter.

## Hva vi trenger fra deg

- Kjør `nav-pilot alpha local status` med en gang noe henger.
- Si fra hvis en endring kompilerer, men er feil på en måte du ikke ville ventet av en slurvete kollega.
- Si fra om ventetiden er verdt det midt i en arbeidsdag. Bare du vet det.

At det ikke er verdt bryet er et like nyttig svar som det motsatte, og bedre å få nå enn om et år.

Mens dette er alfa måler vi det tettere enn resten av nav-pilot: hvor mange oppgaver hver økt sender til modellen, hvilken modell, oppstartstid, og når serveren henger. Aldri koden din eller det du skriver. `DO_NOT_TRACK=1` skrur det av.

## Bli med

Si fra i #nav-pilot. Vi tar inn én om gangen i starten.

Målinger og metode ligger i [navikt/mlx-workspace](https://github.com/navikt/mlx-workspace), inkludert kjøringene som gikk galt.
