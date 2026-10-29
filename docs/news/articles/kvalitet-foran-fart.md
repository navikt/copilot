---
title: "Kvalitet foran fart: bruk KI på vedlikeholdet"
date: 2026-10-29
author: starefossen
category: praksis
excerpt: "Kvalitet har alltid gått foran fart. Nå ber vi dere i tillegg bruke KI-verktøyene til å gjøre koden bedre før dere bruker dem til å lage mer av den. Her er ferdige prompter for refaktorering, modernisering, sikkerhetsoppdateringer og herding."
tags:
  - praksis
  - nav-pilot
  - skills
  - sikkerhet
  - vedlikehold
  - cplt
---

KI-verktøyene gjør det lett å skrive mye kode raskt. Vi har alltid ment at kvalitet er viktigere enn fart, og det gjelder fortsatt. Det som er nytt, er at vi ber dere bruke verktøyene mer på å gjøre koden dere har bedre, og mindre på å lage mer av den.

Dette er et råd, ikke en regel. Teamene bestemmer selv hva de prioriterer.

Det henger sammen med det vi skrev i [Dette er retningen](/nyheter/dette-er-retningen): målet er ikke mest mulig KI-bruk, men å bruke KI der det gir verdi, og vi må fortsatt forstå og stå inne for det vi leverer.

## Hvorfor

Kode som blir skrevet raskere, må fortsatt leses, testes, driftes og forstås av noen i teamet. Mer kode betyr mer å vedlikeholde. Høyere tempo hjelper lite hvis kvaliteten faller.

Vedlikehold er samtidig arbeid KI-verktøyene er godt egnet til. Å oppgradere avhengigheter, flytte kode til nyere API-er, rydde i gamle mønstre og gå gjennom sikkerhet er ofte kjedelig, tidkrevende og lett å utsette. Det er mye av det samme hver gang, og resultatet kan sjekkes med bygg og tester.

Skillen `deliberate-ai-use` plasserer «refaktorering med kjent mål» i grønn sone, altså oppgaver der KI gir mest verdi uten at dere mister forståelsen. Sikkerhetskritisk kode og forretningslogikk står i rød sone: der skal dere selv forstå og eie hvert valg.

## Hva vi ber teamene om

- Fortsett å velge kvalitet foran fart, også når KI gjør det fristende å gå raskere.
- Gi refaktorering, modernisering, sikkerhetsoppdateringer og herding høyere prioritet enn ny funksjonalitet en periode.
- La KI gjøre det kjedelige grovarbeidet, og bruk tiden dere sparer på å gå gjennom resultatet.

## Slik jobber du

1. **Planlegg først.** Be om en plan før du lar KI skrive kode. `nav-plan` hjelper med arkitekturvalg for auth, kommunikasjon, database og Nais, og har egne steg for modernisering og refaktorering.
2. **Små PR-er.** Én avhengighet, én modul eller én type endring om gangen. Små differ er raske å lese.
3. **Tester før refaktorering.** Mangler testene, er det den første oppgaven. Uten tester vet dere ikke om oppførselen er den samme etterpå.
4. **Les diffen.** Du er ansvarlig for koden som merges, uansett hvem som skrev den.
5. **Kjør `security-review` før du committer.**

Skills er tilleggskunnskap nav-pilot kan bruke. Du aktiverer en skill ved å skrive navnet i meldingen, for eksempel «bruk dependency-upgrade». Mangler skillen i repoet, installerer du den med `nav-pilot install <skill>`. Alle skills finner du på [Verktøy](/verktoy).

## Flere repoer i samme sesjon

Mange team har koden fordelt på flere repoer: et felles bibliotek her, tjenestene som bruker det der. nav-pilot med cplt kan jobbe mot flere av dem i én sesjon. Start i ett repo og navngi de andre:

```sh
cplt config set --local sandbox.repo_dirs ~/src/<annet-repo>
```

Stien må være absolutt eller starte med `~/`. Nøkkelen kan bare settes med `--local`, ikke globalt og ikke i repoets `.cplt.toml`, og gjennom nav-pilot er `--local` den eneste veien. Det virker både for repoer som ligger side om side og for repoer som ligger inni prosjektkatalogen.

**Et repo utenfor prosjektkatalogen gir agenten lese-, skrive- og kjøretilgang der**, på lik linje med repoet du startet i. Du sier altså ja til enda et tre der agenten kan bygge og kjøre. Til gjengjeld får agenten gh-scope mot repoet, så PR-er dit virker, og endringene der rapporteres for seg etter sesjonen. Skal repoet bare redigeres, bruk `allow.write` i stedet. Det gir ikke bygg og ikke gh.

Les oppstartsoppsummeringen. Den lister repoene og sier hvilke som er nye kjørbare trær. Detaljene står i [cplt: global, local eller repo, og flere repoer i samme sesjon](/nyheter/cplt-lokal-config-og-flere-repoer).

```text
Oppgrader <bibliotek> til <versjon> i ~/src/<bibliotek> først. Når bygget er grønt,
oppdater appene i ~/src/<app-a> og ~/src/<app-b> til den nye versjonen.
Kjør testene i hvert repo og lag én PR per repo.
```

## Refaktorering

Start med å forstå koden og sikre den med tester. Deretter kan du rydde.

```text
Les modulen src/main/kotlin/no/nav/<app>/vedtak og forklar hva den gjør.
Skriv karakteriseringstester som låser dagens oppførsel før vi endrer noe.
Ikke endre produksjonskoden ennå.
```

```text
Bruk nav-plan. Lag en plan for å dele opp VedtakService i mindre enheter.
Planen skal ha små steg der hvert steg kan bli én PR med grønne tester.
List opp hva som kan gå galt.
```

```text
Bruk java-to-kotlin. Konverter én klasse om gangen i pakken
no.nav.<app>.integrasjon, behold git-historikken, og kjør testene etter hver fil.
```

## Modernisering

Gamle versjoner av rammeverk og biblioteker blir dyrere å oppgradere jo lenger dere venter.

```text
Bruk dependency-upgrade. Oppgrader direkte avhengigheter i dette repoet.
Hold tilbake major-hopp og vis meg hva du holdt tilbake og hvorfor.
```

```text
Bruk jackson-3-migration. Migrer denne Ktor-appen fra Jackson 2 til Jackson 3.
Kjør OpenRewrite-passet først, rydd deretter i Kotlin-koden, og stopp
hvis testene feiler.
```

```text
Bruk aksel-builder. Finn komponenter i denne Next.js-appen som bruker
utdaterte Aksel-API-er eller egne stiler der Aksel har en komponent.
Foreslå en rekkefølge for migreringen, én side om gangen.
```

Spring Boot 3 til 4 og andre rammeverksmigreringer krever kodeendringer. Da passer `dependency-upgrade` dårlig. Bruk `nav-plan` til å lage en plan først.

## Sikkerhetsoppdateringer

Å oppdatere sårbare avhengigheter er den enkleste sikkerhetsgevinsten. Et etterslep med Dependabot-PR-er kan ofte ryddes i én omgang.

```text
Bruk dependency-upgrade. Rydd opp i etterslepet av Dependabot-PR-er
i dette repoet. Én PR per økosystem, med bygg og tester grønne.
```

```text
Bruk security-review. Gå gjennom endringene på denne grenen før jeg lager PR.
Sjekk parameterisert SQL, personopplysninger i logger, hemmeligheter i koden
og accessPolicy i Nais-manifestet.
```

## Herding

Herding betyr å tette hull før noen finner dem: tilgangsstyring, nettverksregler, logging og overvåking.

```text
Bruk threat-model. Lag en STRIDE-A-trusselmodell for denne tjenesten.
Den kjører i prod-gcp, mottar kall med TokenX fra frontend og leser fra Kafka
og Postgres. List tillitsgrensene og de viktigste risikoene.
```

```text
Vurder dette repoet med appsec-advisory. Skill kodefunn fra antatt eksponering,
oppgi hva som ikke er undersøkt, og foreslå trygge tester. Ikke kjør pentest.
```

```text
Bruk nav-auth og tokenx-auth. Sjekk at alle endepunkter i denne Ktor-appen
validerer token med riktig audience og issuer, og at kall videre bruker
token exchange i stedet for å sende brukerens token videre.
```

```text
Bruk observability-setup. Sjekk at appen har health-endepunkter, Prometheus-metrikker,
strukturert logging og varsler i .nais/alert.yml. Vis hva som mangler.
```

```text
Bruk postgresql-review. Gå gjennom spørringene i repository-laget.
Finn manglende indekser, N+1-spørringer og migreringer som låser store tabeller.
```

Funn fra en KI-gjennomgang er forslag. Sjekk dem før dere retter, og noter funn dere avviser, så dere ikke vurderer dem på nytt neste gang.

## Hjelp

- [Praksis](/praksis) har veiledning om å forberede oppgaver og gå gjennom det KI-en har gjort.
- [nav-pilot](/nav-pilot) forklarer hvordan du kommer i gang.
- Spør i [#ki-produktutvikling-diskusjon](https://nav-it.slack.com/archives/C0ASJSZK8P8) på Slack hvis noe ikke virker, eller hvis dere savner en skill.
