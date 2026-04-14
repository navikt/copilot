---
title: "Nav-pilot: 4 nye artefakter og kryssreferanseaudit"
date: 2026-04-13
draft: false
category: nav-pilot
excerpt: "Trusselmodellering, OWASP Top 10:2025, Java→Kotlin-migrering og ytelsesoptimalisering. Pluss kryssreferanseaudit av alle samlinger."
tags:
  - nav-pilot
  - skills
  - instructions
  - security
  - performance
---

Nav-pilot agent harness har fått fire nye artefakter basert på analyse av 500+ navikt-repoer og ekstern forskning fra awesome-copilot, awesome-claude-code og kotlin-agent-skills.

## Nye artefakter

### threat-model (skill)

STRIDE-A trusselmodellering tilpasset NAIS-mikrotjenester. Genererer dataflytdiagram med Nav-spesifikke tillitsgrenser (Wonderwall, NAIS accessPolicy, GCP IAM), identifiserer trusler per komponent, og produserer risikovurdering med tiltak.

Bruk: `@nav-pilot trusselmodeller dp-soknad`

### java-to-kotlin (skill)

Rammeverk-bevisst migrering fra Java til Kotlin. Dekker ikke bare språkkonvertering, men også Spring Boot→Ktor, JPA→Kotliquery, JUnit→Kotest, og Lombok→data classes. Inkluderer git-historikkbevaring og batcharbeidsflyt for store kodebaser.

Bruk: `@nav-pilot konverter UserService.java til Kotlin`

### performance (instruksjon)

Core Web Vitals-mål for Next.js/Aksel-apper. Aktiveres automatisk på `src/**/*.{tsx,ts}`-filer. Dekker server components, datafetching med Suspense, bilde-/fontoptimalisering, bundle-analyse og anti-mønstre.

### security-owasp (instruksjon)

OWASP Top 10:2025 kodemønstre med ✅/❌-eksempler i både Kotlin og Go. Aktiveres automatisk på `**/*.{kt,go}`-filer. Dekker injeksjon, autentisering, tilgangskontroll, kryptografi, logging og alle andre OWASP-kategorier.

## Kryssreferanseaudit

Gjennomførte en integrasjonsaudit av alle 4 samlinger. Hovedfunn: instruksjoner var isolerte — de refererte ikke til relaterte agenter, skills eller andre instruksjoner. Fikset ved å legge til `Related`-tabeller i 7 instruksjoner og 1 agent.

## Tracking issues

18 åpne enhancement-issues for videre arbeid, inkludert:

- [#169](https://github.com/navikt/copilot/issues/169) — Spring Boot-deprekering og Ktor-migrering
- [#174](https://github.com/navikt/copilot/issues/174) — Wonderwall auth proxy-instruksjon
- [#173](https://github.com/navikt/copilot/issues/173) — Arrow-kt og Koin-mønstre
- [#162](https://github.com/navikt/copilot/issues/162) — Unleash feature flag-mønstre

Se [nav-pilot-changelog.md](../nav-pilot-changelog.md) for komplett endringslogg.
