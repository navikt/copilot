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
<!-- | **ai-news-research** | Skriv månedlige oppsummeringer av AI-nyheter for utviklere på norsk med fungerende kildelenker. Bruk for å skrive nyheter, oppsummere AI-trender, lage månedlig oppdatering, eller undersøke hva som er nytt i GitHub Copilot, coding agents, AGENTS.md, skills, memory, agentic workflows eller developer experience. | [`skills/ai-news-research/`](../skills/ai-news-research/SKILL.md) | -->
| **aksel-builder** | Expert builder for the Aksel design system (Nav / @navikt) React components, design tokens, layout primitives, theming (light/dark), icons, CSS, the Tailwind preset, version migrations, and Figma-to-code. Trigger on any frontend UI task that mentions Aksel, Nav/Navikt, "designsystemet", or @navikt/ds-* / @navikt/aksel-* packages — or that asks to add, create, build, or refactor a component (button, input, modal, table, alert, card, form) or layout, or to implement a design from Figma (a pasted figma.com/design/...?node-id link, "implement this design", "build this from Figma", design-to-code). Strong signals "using/with aksel", "@navikt/ds-react", "design system", a pasted figma.com link. If the work is frontend UI and there is any Aksel signal, invoke this skill unless the user explicitly opts out. | [`skills/aksel-builder/`](../skills/aksel-builder/SKILL.md) |
| **aksel-spacing** | Lag responsive layouts med Aksel Design System (v8+) - spacing tokens, layout primitives (Box, HStack, VStack, HGrid, Page, Bleed) og ResponsiveProp | [`skills/aksel-spacing/`](../skills/aksel-spacing/SKILL.md) |
| **api-design** | REST API-designmønstre, versjonering, feilhåndtering (RFC 7807) og OpenAPI-konvensjoner for Nav-tjenester | [`skills/api-design/`](../skills/api-design/SKILL.md) |
| **conventional-commit** | Generer conventional commit-meldinger med Nav-relevante scopes og breaking change-format | [`skills/conventional-commit/`](../skills/conventional-commit/SKILL.md) |
| **flyway-migration** | Databasemigrasjonsmønstre med Flyway og versjonerte SQL-skript | [`skills/flyway-migration/`](../skills/flyway-migration/SKILL.md) |
| **java-to-kotlin** | Trinnvis Java-til-Kotlin-migrering med rammeverk-bevisste transformasjoner for Spring, Ktor og Nav-mønstre | [`skills/java-to-kotlin/`](../skills/java-to-kotlin/SKILL.md) |
| **kafka** | Rapids & Rivers, eventdrevet arkitektur, Kafka-mønstre og schema-design for Nav-applikasjoner | [`skills/kafka/`](../skills/kafka/SKILL.md) |
| **kotlin-app-config** | Sealed class-konfigurasjon for Kotlin-applikasjoner med miljøspesifikke innstillinger | [`skills/kotlin-app-config/`](../skills/kotlin-app-config/SKILL.md) |
| **ktor-scaffold** | Scaffold eit nytt Ktor-prosjekt med Kotliquery, Flyway, Koin og Nais-konfigurasjon | [`skills/ktor-scaffold/`](../skills/ktor-scaffold/SKILL.md) |
| **nais** | Nais-deployment, GCP-ressurser, pod-lifecycle og feilsøking på plattformen | [`skills/nais/`](../skills/nais/SKILL.md) |
| **nav-architecture-review** | Generer Architecture Decision Records (ADR) med flerperspektiv-review tilpasset Nav | [`skills/nav-architecture-review/`](../skills/nav-architecture-review/SKILL.md) |
| **nav-auth** | Azure AD, TokenX, ID-porten, Maskinporten og JWT-validering for Nav-applikasjoner | [`skills/nav-auth/`](../skills/nav-auth/SKILL.md) |
| **nav-deep-interview** | Strukturert intervju som avdekker blindsoner i Nav-prosjekter — personvern, auth, avhengigheter og observerbarhet | [`skills/nav-deep-interview/`](../skills/nav-deep-interview/SKILL.md) |
| **nav-dekoratoren** | Integrer og konfigurer Nav Dekoratøren – felles header og footer for nav.no-applikasjoner. Bruk når et team skal ta i bruk Dekoratøren, oppdatere konfigurasjon, legge til breadcrumbs/språkvelger/analytics, håndtere samtykke (ekomloven), CSP eller feilsøke integrasjon mot dekoratøren. | [`skills/nav-dekoratoren/`](../skills/nav-dekoratoren/SKILL.md) |
| **nav-plan** | Arkitekturplanlegging med beslutningstrær for auth, kommunikasjon, database og Nais-konfigurasjon | [`skills/nav-plan/`](../skills/nav-plan/SKILL.md) |
| **nav-troubleshoot** | Strukturerte diagnostiske trær for vanlige Nav-plattformproblemer — pod-krasj, auth-feil, Kafka-lag og databaseproblemer | [`skills/nav-troubleshoot/`](../skills/nav-troubleshoot/SKILL.md) |
| **observability-debugging** | Feilsøk produksjonsproblemer med Mimir-metrikker, Loki-logger og Tempo-traces — strukturerte debugging-workflows for Nav-utviklere | [`skills/observability-debugging/`](../skills/observability-debugging/SKILL.md) |
| **observability-setup** | Sett opp Prometheus-metrikker, OpenTelemetry-tracing og health check-endepunkter for Nais-applikasjoner | [`skills/observability-setup/`](../skills/observability-setup/SKILL.md) |
| **playwright-testing** | Generer og kjør Playwright E2E-tester for webapplikasjoner med page objects, auth fixtures og tilgjengelighetstester | [`skills/playwright-testing/`](../skills/playwright-testing/SKILL.md) |
| **postgresql-review** | PostgreSQL query review, optimalisering og beste praksis for Nav-applikasjoner | [`skills/postgresql-review/`](../skills/postgresql-review/SKILL.md) |
| **readme-review** | Strukturell gjennomgang og generering av README-er tilpasset prosjekttype — tjeneste, bibliotek, monorepo eller naisjob | [`skills/readme-review/`](../skills/readme-review/SKILL.md) |
| **rust-development** | Idiomatisk Rust-utvikling med cargo, clippy, error handling, async/tokio, unsafe og testing | [`skills/rust-development/`](../skills/rust-development/SKILL.md) |
| **security-owasp** | OWASP Top 10:2025 kodenivå-mønstre for Kotlin, Go, Java og Node.js — tilgangskontroll, forsyningskjede, injeksjon og feilhåndtering | [`skills/security-owasp/`](../skills/security-owasp/SKILL.md) |
| **security-review** | Bruk før commit, push eller pull request for å sjekke at koden er trygg å merge | [`skills/security-review/`](../skills/security-review/SKILL.md) |
| **spring-boot-scaffold** | Scaffold et nytt Spring Boot Kotlin-prosjekt med Nais-konfigurasjon, Flyway og standard Nav-mønstre | [`skills/spring-boot-scaffold/`](../skills/spring-boot-scaffold/SKILL.md) |
| **terse-mode** | Kompakt output-stil som kutter fyllord og beholder teknisk substans — spar output-tokens uten å miste nøyaktighet. | [`skills/terse-mode/`](../skills/terse-mode/SKILL.md) |
| **threat-model** | STRIDE-A trusselmodellering for Nais-mikrotjenester — dataflyt, tillitsgrenser og risikovurdering | [`skills/threat-model/`](../skills/threat-model/SKILL.md) |
| **tokenx-auth** | Tjeneste-til-tjeneste-autentisering med TokenX token exchange i Nais | [`skills/tokenx-auth/`](../skills/tokenx-auth/SKILL.md) |
| **web-design-reviewer** | Visuell inspeksjon av nettsider for å identifisere og fikse designproblemer. Trigges av forespørsler som "sjekk designet", "gå gjennom UI-en", "fiks layouten", "finn designfeil". Finner problemer med responsivt design, tilgjengelighet, visuell konsistens og layout, og fikser dem i kildekoden. | [`skills/web-design-reviewer/`](../skills/web-design-reviewer/SKILL.md) |
| **workstation-security** | Sikkerhetssjekk for macOS-utviklermaskiner — brannmur, SSH, Git, hemmeligheter, nettverk og Nav-plattformverktøy | [`skills/workstation-security/`](../skills/workstation-security/SKILL.md) |
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
