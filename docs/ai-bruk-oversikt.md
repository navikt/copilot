# AI-bruk i Nav: Oversikt og bevisste grenser

> Kartlegging av hvordan vi bruker AI-verktøy i utviklingsarbeidet, og hvor vi bevisst *ikke* bruker dem.

## Agenter — domeneeksperter for utviklere

| Agent | Formål | Domene | Sone |
|-------|--------|--------|------|
| `@nav-pilot` | Planlegg, arkitekturer og bygg Nav-applikasjoner | Generalist (Nais, auth, Kafka, sikkerhet) | 🟢 |
| `@code-review-agent` | Finn feil, sikkerhetsproblemer, konvensjonsbrudd | Kvalitetssikring | 🟢 |
| `@security-champion-agent` | Trusselmodellering, GDPR, compliance | Sikkerhet | 🟢/🔴 |
| `@accessibility-agent` | WCAG 2.1/2.2, universell utforming, UU-testing | Tilgjengelighet | 🟢 |
| `@observability-agent` | Prometheus, OpenTelemetry, Grafana, varsling | Observerbarhet | 🟢 |
| `@nais-agent` | Deployment, GCP-ressurser, plattformfeilsøking | Infrastruktur | 🟢 |
| `@auth-agent` | Azure AD, TokenX, ID-porten, Maskinporten, JWT | Autentisering | 🔴 |
| `@kafka-agent` | Rapids & Rivers, eventdrevet arkitektur, schema | Meldingssystemer | 🟢/🔴 |
| `@aksel-agent` | Aksel Design System — komponenter, tokens, layout | Frontend | 🟢 |
| `@forfatter` | Norsk klarspråk, AI-markører, fagtermer, mikrotekst | Språk/innhold | 🟢 |
| `@research-agent` | Utforsker kodebaser, samler kontekst | Forberedelse | 🟢 |
| `@rust-agent` | Idiomatisk Rust med cargo, clippy, async/tokio | Språkspesifikt | 🟢 |

**Sone-forklaring:**
- 🟢 = Grønn sone — AI-delegering er trygt, boilerplate/kjent teknologi
- 🔴 = Rød sone — kjernelogikk/sikkerhet, forstå grundig før du aksepterer
- 🟢/🔴 = Blandingsdomene — avhenger av oppgavens kompleksitet

---

## Skills — kunnskapspakker brukt av agenter og utviklere

| Skill | Formål | Kategori |
|-------|--------|----------|
| nav-plan | Arkitekturplanlegging med beslutningstrær | Planlegging |
| nav-deep-interview | Avdekke blinde flekker før implementering | Planlegging |
| nav-architecture-review | ADR-generering med flerperspektiv | Review |
| security-review | Sikkerhetssjekk før commit/push/PR | Review |
| threat-model | STRIDE-A trusselmodellering | Review |
| postgresql-review | Query-optimalisering og beste praksis | Review |
| readme-review | README-strukturvurdering | Review |
| web-design-reviewer | Visuell inspeksjon og designfiks | Review |
| observability-setup | Prometheus + OTel + health checks | Oppsett |
| spring-boot-scaffold | Spring Boot Kotlin-prosjekt med Nais | Scaffolding |
| ktor-scaffold | Ktor-prosjekt med Kotliquery, Flyway, Koin | Scaffolding |
| kotlin-app-config | Sealed class-konfigurasjon | Scaffolding |
| flyway-migration | Databasemigrasjoner med Flyway | Scaffolding |
| api-design | REST API-mønstre, RFC 7807, OpenAPI | Scaffolding |
| tokenx-auth | TokenX token exchange-implementasjon | Scaffolding |
| aksel-spacing | Responsive layouts med spacing tokens | Frontend |
| playwright-testing | E2E-tester med page objects og a11y | Testing |
| conventional-commit | Commit-meldinger med Nav-scopes | Workflow |
| ai-news-research | Månedlige AI-nyhetsoppsummeringer | Innhold |
| java-to-kotlin | Trinnvis migrasjonsguide | Migrering |
| rust-development | Idiomatisk Rust-utvikling | Språkspesifikt |
| nav-troubleshoot | Diagnostiske trær for plattformproblemer | Feilsøking |
| workstation-security | Sikkerhetssjekk for utviklermaskiner | Drift |

---

## Instruksjoner — automatiske regler per filtype

| Instruksjon | Gjelder for | Hva den gjør |
|-------------|------------|--------------|
| golang | `*.go` | Go-idiomer, error wrapping, slog-logging |
| kotlin-spring | `*.kt` | Spring Boot-mønstre, dependency injection |
| kotlin-ktor | `*.kt` | Ktor-routing, ApplicationBuilder |
| security-owasp | `*.kt, *.go` | OWASP-sjekkliste, inputvalidering |
| nextjs-aksel | `src/**/*.{tsx,ts}` | Aksel spacing, responsive props, Box/VStack |
| performance | `src/**/*.{tsx,ts}` | Lazy loading, memo, bundle-størrelse |
| accessibility | `src/**/*.{tsx,jsx}` | ARIA, semantisk HTML, tastaturnavigasjon |
| testing | `*.test.*` | Teststruktur, Arrange-Act-Assert |
| testing-typescript | `*.test.{ts,tsx}` | Vitest-mønstre, mock-strategier |
| testing-kotlin | `*.test.{kt,kts}` | JUnit 5, testcontainers |
| norwegian-text | `*.md` | Klarspråk, AI-markører, terminologi |
| github-actions | `*.yml/*.yaml` | Workflow-sikkerhet, SHA-pinning |
| docker | `Dockerfile` | Multi-stage, Chainguard, .dockerignore |
| database | `**/db/migration/**/*.sql` | Flyway-konvensjoner, idempotens |
| deliberate-ai-use | Alle filer | Grønn/rød sone, generer-så-forstå |

---

## Hvor vi bevisst *ikke* bruker AI

Områder der vi har valgt å ikke delegere til AI, med begrunnelse:

| Område | Begrunnelse | Alternativ |
|--------|-------------|-----------|
| **Debugging av produksjonsfeil** | Feilsøking er den sterkeste læringsmekanismen (Anthropic 2026). Å delegere debugging fjerner den kognitive prosessen som bygger dyp forståelse av systemet. | Manuell debugging, eventuelt AI for å *forklare* feilmeldinger etter at utvikleren har prøvd selv (tre-forsøks-regelen). |
| **Sikkerhetskritiske beslutninger** | Auth-flyt, tilgangskontroll og inputvalidering krever at utvikleren forstår konsekvensene fullstendig. AI kan foreslå, men mennesker må verifisere og forstå. | `@security-champion-agent` brukes som *sensor* (review), ikke som *generator* av sikkerhetskode. |
| **Arkitekturbeslutninger** | Systemdesign, datamodeller og API-kontrakter har langvarige konsekvenser. AI mangler kontekst om organisasjonens begrensninger, teamstruktur og langsiktig retning. | `nav-plan` og `nav-architecture-review` brukes for å *utforske* alternativer og *challenge* beslutninger, men mennesker tar den endelige avgjørelsen. |
| **Ny teknologi (første gangs bruk)** | Å lære et nytt rammeverk eller språk via AI-generert kode bygger ikke forståelse. Utvikleren bør kode manuelt først, deretter bruke AI for repetitive mønstre. | Tre-forsøks-regelen: prøv selv tre ganger, deretter bruk AI. |
| **Personopplysninger og GDPR-vurderinger** | Juridiske vurderinger om databehandling krever presisjon og ansvarlighet som AI ikke kan garantere. | Mennesker gjør DPIA og personvernvurderinger. AI kan hjelpe med å *identifisere* potensielle personopplysninger i kode. |
| **Produksjonsinfrastruktur (uten review)** | Nais-manifest og GCP-konfig kan genereres av AI, men deployes aldri uten menneskelig review. Ett feilkonfigurert felt kan eksponere data eller ta ned tjenester. | `@nais-agent` genererer config, men all infrastruktur går gjennom PR-review + CI-validering. |

---

## Kobling til harness-rammeverket

Tabellen over «bevisste grenser» er i Fowlers termer en **feedforward guide** — den styrer utvikleren *før* AI-bruk, ikke etter. Harnessen mangler tilsvarende **sensors** som automatisk oppdager når utviklere delegerer i rød sone. Dette er identifisert som et gap i [agent-harness.md](agent-harness.md).

Utviklerundersøkelsen (59 % bekymret for kompetansetap) validerer at disse grensene er riktig kalibrert — utviklerne selv ønsker mer bevisst bruk, ikke mindre AI.
