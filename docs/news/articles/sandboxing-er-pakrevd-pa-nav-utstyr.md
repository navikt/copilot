---
title: "Sandboxing og isolasjon er påkrevd på Nav-utstyr"
date: 2026-08-10
author: starefossen
category: praksis
excerpt: "AI-agenter skal alltid kjøre isolert på Nav-utstyr. Bruk cplt som standard, også når agentarbeidet er personlig."
tags:
  - cplt
  - security
  - sandbox
  - coding-agents
---

Når du bruker en AI-agent på Nav-utstyr, skal agenten kjøre i en sandbox eller
tilsvarende isolasjon. Dette er et krav, ikke et valgfritt sikkerhetstiltak.

Kravet gjelder alt agentarbeid på utstyret:

- Nav-relaterte oppgaver
- arbeid i private repoer
- personlig utforsking og sideprosjekter

Det er utstyret og tilgangene som må beskyttes. At oppgaven er personlig, reduserer
ikke risikoen for at agenten får tilgang til filer, hemmeligheter,
tilgangsinformasjon eller interne tjenester på maskinen.

## Bruk cplt

[`cplt`](/cplt) er den anbefalte og enkleste måten å oppfylle kravet på. cplt
starter agenten med isolasjon som operativsystemet håndhever. Dette begrenser
blant annet tilgangen til filer, tilgangsinformasjon og nettverk.

```bash
brew install navikt/tap/cplt
cplt
```

nav-pilot er laget for å brukes sammen med cplt:

```bash
brew install navikt/tap/nav-pilot navikt/tap/cplt
nav-pilot
```

Bruk cplt som standard fremfor å bygge din egen løsning.

## Hvis du ikke bruker cplt

Du må selv sette deg inn i hvordan agentklienten isolerer agenten, aktivere denne
funksjonen og forstå hva den faktisk beskytter. Hvis klienten ikke gir
tilstrekkelig beskyttelse, må du bruke en annen mekanisme, for eksempel en VM
eller container.

Vi går ikke nærmere inn på oppsett av alternative løsninger her. Hovedregelen er
enkel: Ikke kjør en AI-agent med ubegrenset tilgang til Nav-utstyret.

## Hvorfor godkjenning ikke er nok

En agent kan lese innhold den ikke bør stole på, kjøre kommandoer og installere
avhengigheter. Prompt injection og kompromitterte pakker kan påvirke hva agenten
gjør. Bekreftelsesdialoger og gode instruksjoner reduserer risiko, men erstatter
ikke en teknisk sikkerhetsgrense.

Sandboxing begrenser konsekvensene når agenten, et verktøy eller en avhengighet
gjør noe uventet. Derfor skal isolasjonen være på plass før agenten starter.

## Kortversjonen

**På Nav-utstyr skal AI-agenter alltid kjøre isolert. Bruk cplt. Velger du noe
annet, er du ansvarlig for å sikre tilsvarende isolasjon. Kravet gjelder både
Nav-relatert og personlig agentarbeid.**

**Kilder:**

- [cplt-dokumentasjonen](/cplt) (Nav)
- [cplt threat model](https://github.com/navikt/cplt/blob/main/SECURITY.md) (GitHub)
- [Cloud and local sandboxes for GitHub Copilot](https://github.blog/changelog/2026-06-02-cloud-and-local-sandboxes-for-github-copilot-now-in-public-preview) (GitHub, 2. juni 2026)
