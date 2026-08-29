---
title: "Lokale modeller i nav-pilot — hva vi målte, og hva vi ikke vet"
date: 2026-08-29
author: starefossen
category: praksis
excerpt: "nav-pilot kan nå sende avgrensede oppgaver til en modell som kjører på din egen maskin. Her er hva vi målte over en uke med benchmarking: hva modellen klarer, hva den ikke klarer, og hvorfor vi ennå ikke vet om det sparer penger."
tags:
  - nav-pilot
  - local-models
  - mlx
  - cost
  - alpha
---

Vi bruker rundt 45 000 dollar i måneden på GitHub Copilot for 650 utviklere, og de tyngste brukerne treffer taket på 400 dollar før måneden er omme. Kan en modell som kjører på utviklerens egen maskin ta unna nok av de små oppgavene til at taket rekker lenger?

Det vet vi ikke ennå. Det vi vet, er at modellen løser en avgrenset del av arbeidet godt nok til at det er verdt å prøve. `nav-pilot alpha local` er nå ute til testing.

## Hva du får

En lokal modell som hovedagenten sender avgrensede oppgaver til: les en fil og svar, legg til en kommentar, døp om et symbol, skriv en test. Den planlegger ikke, fører ingen samtale og velger ikke hva som skal gjøres. Hovedagenten kjører fortsatt i skyen.

```
nav-pilot alpha local init      # laster ned og setter opp
nav-pilot alpha local start
nav-pilot alpha local status
```

## Hva vi målte

Vi kjørte elleve realistiske oppgaver mot `navikt/isoppfolgingstilfelle` og de samme elleve mot en Nav-frontend i TypeScript. Åtte av dem kan avgjøres maskinelt, ved å kompilere eller kjøre testsuiten; de tre siste er spørsmål som må leses av et menneske. Tabellen teller de åtte. Ingenting er verifisert ved å ta modellen på ordet.

| | Kotlin | TypeScript |
|---|---|---|
| Median per oppgave | 21 sekunder | 15 sekunder |
| Verifisert riktig | 4 av 8 | 6 av 8 |

Modellen er `Qwen3.6-35B-A3B-OptiQ-4bit`. Den tar 25 GB på disk og rundt 21 GB i minnet mens den kjører, og krever en maskin med 48 GB minne.

## Hva den ikke klarer

Den vanligste feilen er at den ikke gjør noe: den leser filene, forklarer hva som burde endres og stopper der. Det koster deg et minutt.

Den nest vanligste er verre. Omtrent én gang per elleve oppgaver gjentok modellen den samme kommandoen til noe stoppet den. Vi målte serier på 203 og 220 like kall. nav-pilot avbryter nå en tur etter åtte like kall på rad, så du skal få en feilmelding i stedet for en maskin som står og maler.

Vi valgte OptiQ-varianten nettopp derfor: den vanlige 4-bit-varianten løp løpsk i begge de fulle kjøringene våre, OptiQ i ingen av dem.

## Hva vi ikke vet

**På én oppgavetype sparer det penger, og vi har målt hvor mye.** Å tre et nytt felt gjennom en dataklasse, mapperen og alle kallstedene kostet 13 AI-credits med lokal utsending mot 34 uten, som median over 20 kjøringer på samme oppgave og samme commit. Testresultatet var det samme i begge armene. Det tok lengre tid: 156 sekunder mot 100.

Det gjelder den oppgavetypen, ikke alle. To andre oppgaver vi målte sendte hovedagenten aldri videre i det hele tatt — den vurderte at den gjorde jobben raskere selv, og den hadde antakelig rett. Hvor ofte den velger å sende noe videre er tallet som avgjør om dette lønner seg i praksis, og det varierer med oppgaven.

**Ingen har brukt dette en hel arbeidsdag.** Alle tallene over kommer fra kjøringer med elleve oppgaver, ikke fra noen som satt og gjorde jobben sin.

**Spring er ikke testet.** Kotlin-tallene våre er fra Ktor-kode, som er der nye Nav-apper havner. Spring er mesteparten av det som allerede står der ute.

## Vil du være med?

Meld deg hvis du har en Mac med 48 GB minne og bruker opp AI-creditsene dine. Vi vil særlig vite om noe henger i mer enn to minutter, om en løkke slipper forbi vakten på åtte kall, og om en endring kompilerer, men er feil på en måte du ikke ville ventet av en slurvete kollega.

Alle målingene ligger i [navikt/mlx-workspace](https://github.com/navikt/mlx-workspace), inkludert de som gikk galt.
