# 🎯 Agent Skills

Skills er selvstendige mapper med instruksjoner og referansedata som gir Copilot spesialisert Nav-kunnskap.

📖 **Utforsk og installer:** [min-copilot.ansatt.nav.no/verktoy](https://min-copilot.ansatt.nav.no/verktoy)

### Installer

```bash
mkdir -p .github/skills
# Bruk nav-pilot CLI
nav-pilot install <skill-name>
```

## Tilgjengelige skills

<!-- BEGIN GENERATED TABLE -->
| Name | Description | Location |
| ---- | ----------- | -------- |
<!-- | **ai-news-research** | Skriv månedlige oppsummeringer av AI-nyheter for utviklere på norsk med fungerende kildelenker. Bruk for å skrive nyheter, oppsummere AI-trender, lage månedlig oppdatering, eller undersøke hva som er nytt i GitHub Copilot, coding agents, AGENTS.md, skills, memory, agentic workflows eller developer experience. | [`.github/skills/ai-news-research/`](../.github/skills/ai-news-research/SKILL.md) | -->
| **aksel-builder** | Bygg grensesnitt med Aksel Design System (v8+) - Dokumentasjon, setup, tokens, komponenter og layout primitives, ikoner og migrering mellom major-versjoner  | [`.github/skills/aksel-builder/`](../.github/skills/aksel-builder/SKILL.md) |
| **api-design** | REST API-designmønstre, versjonering, feilhåndtering (RFC 7807) og OpenAPI-konvensjoner for Nav-tjenester | [`.github/skills/api-design/`](../.github/skills/api-design/SKILL.md) |
| **conventional-commit** | Generer conventional commit-meldinger med Nav-relevante scopes og breaking change-format | [`.github/skills/conventional-commit/`](../.github/skills/conventional-commit/SKILL.md) |
| **flyway-migration** | Databasemigrasjonsmønstre med Flyway og versjonerte SQL-skript | [`.github/skills/flyway-migration/`](../.github/skills/flyway-migration/SKILL.md) |
| **java-to-kotlin** | Trinnvis Java-til-Kotlin-migrering med rammeverk-bevisste transformasjoner for Spring, Ktor og Nav-mønstre | [`.github/skills/java-to-kotlin/`](../.github/skills/java-to-kotlin/SKILL.md) |
| **kafka** | Rapids & Rivers, eventdrevet arkitektur, Kafka-mønstre og schema-design for Nav-applikasjoner | [`.github/skills/kafka/`](../.github/skills/kafka/SKILL.md) |
| **kotlin-app-config** | Sealed class-konfigurasjon for Kotlin-applikasjoner med miljøspesifikke innstillinger | [`.github/skills/kotlin-app-config/`](../.github/skills/kotlin-app-config/SKILL.md) |
| **ktor-scaffold** | Scaffold eit nytt Ktor-prosjekt med Kotliquery, Flyway, Koin og Nais-konfigurasjon | [`.github/skills/ktor-scaffold/`](../.github/skills/ktor-scaffold/SKILL.md) |
| **nais** | Nais-deployment, GCP-ressurser, pod-lifecycle og feilsøking på plattformen | [`.github/skills/nais/`](../.github/skills/nais/SKILL.md) |
| **nav-architecture-review** | Generer Architecture Decision Records (ADR) med flerperspektiv-review tilpasset Nav | [`.github/skills/nav-architecture-review/`](../.github/skills/nav-architecture-review/SKILL.md) |
| **nav-auth** | Azure AD, TokenX, ID-porten, Maskinporten og JWT-validering for Nav-applikasjoner | [`.github/skills/nav-auth/`](../.github/skills/nav-auth/SKILL.md) |
| **nav-deep-interview** | Strukturert intervju som avdekker blindsoner i Nav-prosjekter — personvern, auth, avhengigheter og observerbarhet | [`.github/skills/nav-deep-interview/`](../.github/skills/nav-deep-interview/SKILL.md) |
| **nav-dekoratoren** | Integrer og konfigurer Nav Dekoratøren – felles header og footer for nav.no-applikasjoner. Bruk når et team skal ta i bruk Dekoratøren, oppdatere konfigurasjon, legge til breadcrumbs/språkvelger/analytics, håndtere samtykke (ekomloven), CSP eller feilsøke integrasjon mot dekoratøren. | [`.github/skills/nav-dekoratoren/`](../.github/skills/nav-dekoratoren/SKILL.md) |
| **nav-plan** | Arkitekturplanlegging med beslutningstrær for auth, kommunikasjon, database og Nais-konfigurasjon | [`.github/skills/nav-plan/`](../.github/skills/nav-plan/SKILL.md) |
| **nav-troubleshoot** | Strukturerte diagnostiske trær for vanlige Nav-plattformproblemer — pod-krasj, auth-feil, Kafka-lag og databaseproblemer | [`.github/skills/nav-troubleshoot/`](../.github/skills/nav-troubleshoot/SKILL.md) |
| **observability-debugging** | Feilsøk produksjonsproblemer med Mimir-metrikker, Loki-logger og Tempo-traces — strukturerte debugging-workflows for Nav-utviklere | [`.github/skills/observability-debugging/`](../.github/skills/observability-debugging/SKILL.md) |
| **observability-setup** | Sett opp Prometheus-metrikker, OpenTelemetry-tracing og health check-endepunkter for Nais-applikasjoner | [`.github/skills/observability-setup/`](../.github/skills/observability-setup/SKILL.md) |
| **playwright-testing** | Generer og kjør Playwright E2E-tester for webapplikasjoner med page objects, auth fixtures og tilgjengelighetstester | [`.github/skills/playwright-testing/`](../.github/skills/playwright-testing/SKILL.md) |
| **postgresql-review** | PostgreSQL query review, optimalisering og beste praksis for Nav-applikasjoner | [`.github/skills/postgresql-review/`](../.github/skills/postgresql-review/SKILL.md) |
| **readme-review** | Strukturell gjennomgang og generering av README-er tilpasset prosjekttype — tjeneste, bibliotek, monorepo eller naisjob | [`.github/skills/readme-review/`](../.github/skills/readme-review/SKILL.md) |
| **rust-development** | Idiomatisk Rust-utvikling med cargo, clippy, error handling, async/tokio, unsafe og testing | [`.github/skills/rust-development/`](../.github/skills/rust-development/SKILL.md) |
| **security-owasp** | OWASP Top 10:2025 kodenivå-mønstre for Kotlin, Go, Java og Node.js — tilgangskontroll, forsyningskjede, injeksjon og feilhåndtering | [`.github/skills/security-owasp/`](../.github/skills/security-owasp/SKILL.md) |
| **security-review** | Bruk før commit, push eller pull request for å sjekke at koden er trygg å merge | [`.github/skills/security-review/`](../.github/skills/security-review/SKILL.md) |
| **spring-boot-scaffold** | Scaffold et nytt Spring Boot Kotlin-prosjekt med Nais-konfigurasjon, Flyway og standard Nav-mønstre | [`.github/skills/spring-boot-scaffold/`](../.github/skills/spring-boot-scaffold/SKILL.md) |
| **terse-mode** | Kompakt output-stil som kutter fyllord og beholder teknisk substans — spar output-tokens uten å miste nøyaktighet. | [`.github/skills/terse-mode/`](../.github/skills/terse-mode/SKILL.md) |
| **threat-model** | STRIDE-A trusselmodellering for Nais-mikrotjenester — dataflyt, tillitsgrenser og risikovurdering | [`.github/skills/threat-model/`](../.github/skills/threat-model/SKILL.md) |
| **tokenx-auth** | Tjeneste-til-tjeneste-autentisering med TokenX token exchange i Nais | [`.github/skills/tokenx-auth/`](../.github/skills/tokenx-auth/SKILL.md) |
| **web-design-reviewer** | Visuell inspeksjon av nettsider for å identifisere og fikse designproblemer. Trigges av forespørsler som "sjekk designet", "gå gjennom UI-en", "fiks layouten", "finn designfeil". Finner problemer med responsivt design, tilgjengelighet, visuell konsistens og layout, og fikser dem i kildekoden. | [`.github/skills/web-design-reviewer/`](../.github/skills/web-design-reviewer/SKILL.md) |
| **workstation-security** | Sikkerhetssjekk for macOS-utviklermaskiner — brannmur, SSH, Git, hemmeligheter, nettverk og Nav-plattformverktøy | [`.github/skills/workstation-security/`](../.github/skills/workstation-security/SKILL.md) |
<!-- END GENERATED TABLE -->

## For bidragsytere

Hver skill ligger i `.github/skills/<name>/`:

```
.github/skills/
└── skill-name/
    ├── SKILL.md              # Instruksjonsfil
    └── references/           # Referansedata (beslutningstrær, maler, sjekklister)
```

Se [Agent Skills-spesifikasjonen](https://agentskills.io/specification) og [AGENTS.md](../AGENTS.md) for retningslinjer.
