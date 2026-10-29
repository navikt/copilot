---
title: "Kvalitet foran fart: bruk KI på vedlikeholdet"
date: 2026-10-29
author: starefossen
category: praksis
excerpt: "Kvalitet går foran fart, som før. Trusselbildet har skjerpet seg, så nå ber vi dere bruke KI-verktøyene på koden dere allerede har, før dere lager mer av den. Her er korte prompter dere kan lime rett inn i nav-pilot."
tags:
  - praksis
  - nav-pilot
  - skills
  - sikkerhet
  - vedlikehold
  - cplt
---

Kvalitet har alltid gått foran fart hos oss, og [Dette er retningen](/nyheter/dette-er-retningen) står fast: bruk KI der det gir verdi, og stå inne for det dere leverer.

Nå ber vi dere om én ting til: sett sikkerhetsoppdateringer, herding, refaktorering og modernisering foran ny funksjonalitet en periode. Det er et råd, ikke en regel, og dere bestemmer selv hva dere prioriterer. Mer kode er mer å vedlikeholde, og vedlikeholdet er dessuten arbeid KI gjør godt.

NSM [hevet 27. oktober det nasjonale cyberrisikonivået til høyt](https://nsm.no/aktuelt/nsm-hever-det-nasjonale-cyberrisikonivaet-til-hoyt), det nest høyeste nivået, og ber alle norske virksomheter sette inn tiltak med en gang. KI-modeller har blitt gode til å finne sårbarheter i kode, så det publiseres langt flere enn før, også i komponenter mange av oss bygger på, og med store avhengighetstrær er det vanskelig å vite om en selv er rammet. Angripere utnytter kjente sårbarheter raskere enn tidligere, så det som virket før, virker fortsatt, bare at vi må patche raskere. «Det er legacy» eller «det skal avvikles» er ikke en grunn til å vente, for alt som kjører, er en del av angrepsflaten. Sikkerhetsoppdateringer er derfor det mest presserende av de fire temaene og står først under.

## Slik jobber dere

nav-pilot planlegger først, sikrer med tester og stopper mellom stegene så dere får godkjenne underveis. Det trenger dere ikke sette opp selv. Vi vet at dagene er fulle, så vi ber ikke om mer enn dette:

- **Dere velger oppgavene selv.** Dere kjenner koden og vet hvor etterslepet ligger: avhengigheter, gamle mønstre, hull i tilgangsstyring og logging.
- **Små PR-er gjør det lettere for alle.** Én avhengighet, én modul eller én type endring om gangen.
- **Dere leser diffen før dere merger.** Dere står inne for koden uansett hvem som skrev den, og det stoler vi på.

Promptene under limer dere rett inn i nav-pilot. Skillen som står i prompten, plukker nav-pilot opp selv. Mangler den, kjør `nav-pilot install <skill>`.

## Sikkerhetsoppdateringer

```text
Bruk dependency-upgrade. Rydd opp i etterslepet av Dependabot-PR-er i dette repoet.
Én PR per økosystem.
```

```text
Bruk security-review. Gå gjennom endringene på denne branchen før jeg lager PR.
```

## Refaktorering

```text
Bruk nav-plan. Del opp VedtakService i mindre enheter.
```

```text
Bruk java-to-kotlin. Konverter pakken no.nav.<app>.integrasjon, én klasse om gangen.
```

## Modernisering

```text
Bruk dependency-upgrade. Oppgrader direkte avhengigheter i dette repoet.
Hold tilbake major-hopp og vis meg hva du holdt tilbake.
```

```text
Bruk jackson-3-migration. Migrer denne appen fra Jackson 2 til Jackson 3.
```

## Herding

```text
Bruk threat-model. Lag en trusselmodell for denne tjenesten.
List tillitsgrensene og de viktigste risikoene.
```

```text
Bruk nav-auth. Sjekk at alle endepunkter validerer token med riktig audience og issuer,
og at kall videre bruker token exchange.
```

Funn fra KI er forslag. Sjekk dem før dere retter.

## Flere repoer i samme sesjon

Ligger biblioteket i ett repo og appene i et annet, kan nav-pilot jobbe mot begge i én sesjon. Start i ett repo og pek på det andre:

```sh
cplt config set --local sandbox.repo_dirs ~/src/<annet-repo>
```

Agenten kan da lese, skrive og kjøre kode i det repoet også, på lik linje med repoet dere startet i. Detaljene står i [cplt: global, local eller repo, og flere repoer i samme sesjon](/nyheter/cplt-lokal-config-og-flere-repoer).

```text
Oppgrader <bibliotek> til <versjon> i ~/src/<bibliotek> først, deretter ~/src/<app>.
Én PR per repo.
```

## Hjelp

Spør i [#ki-produktutvikling-diskusjon](https://nav-it.slack.com/archives/C0ASJSZK8P8) hvis noe ikke virker eller dere savner en skill. [Praksis](/praksis) har mer om å forberede oppgaver og gå gjennom resultatet.
