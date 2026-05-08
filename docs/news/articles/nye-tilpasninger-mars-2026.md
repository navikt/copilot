---
title: "17 nye Copilot-tilpasninger for Nav-utviklere"
date: 2026-03-17
author: starefosen
category: nav
excerpt: "Tre instruksjoner, fem skills, to agenter, to prompts og seks utvidelser. Dekker kodegjennomgang, Spring Boot, Playwright, PostgreSQL, Docker med Chainguard og WCAG-tilgjengelighet."
tags:
  - customizations
  - instructions
  - skills
  - agents
  - prompts
  - code-review
  - accessibility
  - spring-boot
  - playwright
  - chainguard
  - docker
---

Vi har analysert [github/awesome-copilot](https://github.com/github/awesome-copilot) opp mot hva Nav-utviklere faktisk bruker — Java, Kotlin, Spring Boot, Ktor, Next.js, React, Aksel, PostgreSQL — og laget tilpasninger som dekker hullene. Resultatet er 17 nye filer og fem utvidelser av eksisterende.

## Tre nye instruksjoner

Instruksjoner aktiveres automatisk når du redigerer filer som matcher mønsteret i `applyTo`.

| Instruksjon                                        | Trigger                          | Hva den gjør                                                                              |
| -------------------------------------------------- | -------------------------------- | ----------------------------------------------------------------------------------------- |
| [**accessibility**](/verktoy?item=accessibility)   | `src/**/*.{tsx,jsx}`             | WCAG 2.1 AA, semantisk HTML, Aksel a11y-komponenter, ARIA, tastaturnavigasjon             |
| [**github-actions**](/verktoy?item=github-actions) | `.github/workflows/*.{yml,yaml}` | SHA-pinning av actions, minimale permissions, Nais deploy, caching, reusable workflows    |
| [**docker**](/verktoy?item=docker)                 | `**/Dockerfile`                  | Chainguard base images, multi-stage builds for JVM/Go/Node/Python/nginx, CI-autentisering |

## Fem nye skills

Skills lastes på forespørsel — agenten henter SKILL.md når oppgaven matcher.

| Ferdighet                                                      | Hva den gjør                                                                        |
| -------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| [**playwright-testing**](/verktoy?item=playwright-testing)     | E2E-tester med page objects, auth-fixtures for Azure AD, axe-core for a11y          |
| [**postgresql-review**](/verktoy?item=postgresql-review)       | EXPLAIN ANALYZE, indeksstrategier, JSONB, window functions, anti-patterns           |
| [**spring-boot-scaffold**](/verktoy?item=spring-boot-scaffold) | Komplett prosjektoppsett: `build.gradle.kts`, nais.yaml, Dockerfile, Testcontainers |
| [**api-design**](/verktoy?item=api-design)                     | REST-konvensjoner, ProblemDetail (RFC 7807), paginering, OpenAPI                    |
| [**conventional-commit**](/verktoy?item=conventional-commit)   | Commit-meldinger med `feat`/`fix`/`docs`, Nav-scopes og breaking change-format      |

## Nye agenter

### [`@code-review-agent`](/verktoy?item=code-review-agent)

Gjennomgår kode på tvers av Navs stack — Kotlin/Spring, Kotlin/Ktor, TypeScript/Next.js, Go, Dockerfile og GitHub Actions. Agenten:

- Prioriterer funn med 🔴 Blocker, 🟡 Forslag og 💭 Småplukk
- Kjører `mise check` for lint og typesjekk
- Sjekker sikkerhet, feilhåndtering, testing og Nais-krav
- Delegerer til spesialistagenter (`@security-champion-agent`, `@accessibility-agent`, `@aksel-agent`)
- Rapporterer funn i en tabell — fikser ikke kode selv

Bruk: `@code-review-agent Gå gjennom endringene mine før jeg lager PR`

### [`@accessibility-agent`](/verktoy?item=accessibility-agent)

En WCAG 2.1/2.2-ekspert du kan spørre direkte i chatten. Agenten kan:

- Kjøre axe-core-audit via Playwright
- Gjennomgå WCAG-krav (Oppfattbar, Betjenbar, Forståelig, Robust)
- Foreslå Aksel-komponenter med riktig a11y-oppsett
- Hente designkontekst fra Figma via MCP

Bruk: `@accessibility-agent Sjekk skjemaet i denne filen for UU-feil`

## To nye prompts

| Prompt                                                          | Hva den gjør                                                                                          |
| --------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| [**#spring-boot-endpoint**](/verktoy?item=spring-boot-endpoint) | Scaffolder Controller, Service, Repository, Flyway-migrasjon og integrasjonstest med MockOAuth2Server |
| [**#nextjs-api-route**](/verktoy?item=nextjs-api-route)         | Scaffolder Next.js App Router API-route med validering, feilhåndtering og test                        |

## Utvidelser av eksisterende tilpasninger

Seks filer har fått nytt innhold:

- [**kotlin-spring**](/verktoy?item=kotlin-spring) — ProblemDetail-feilhåndtering, `@ConfigurationProperties`, test slices (`@WebMvcTest`, `@DataJpaTest`)
- [**testing**](/verktoy?item=testing) — Teststrategi-tabell, Playwright E2E-seksjon, axe-core for a11y-testing
- [**nextjs-aksel**](/verktoy?item=nextjs-aksel) — `loading.tsx`, `error.tsx`, Suspense med streaming, Server Actions med `useActionState`
- [**database**](/verktoy?item=database) — PostgreSQL: EXPLAIN ANALYZE, JSONB med GIN-indeks, window functions, store migrasjoner
- [**security-review**](/verktoy?item=security-review) — OWASP Top 10 med kodeeksempler, filopplasting, avhengighetsskanning
- [**docker**](/verktoy?item=docker) — Byttet fra Google distroless til [Chainguard base images](https://sikkerhet.nav.no/docs/verktoy/chainguard-dockerimages) fra Navs private registry. Nye eksempler for JVM, Node.js, Python, Go og nginx. CI-seksjon med `nais/docker-build-push` for autentisering mot registeret

## Installer

Alle tilpasninger er tilgjengelige fra [verktøysiden](https://min-copilot.ansatt.nav.no/verktoy) med ett-klikk-installasjon for VS Code. Du kan også kopiere filene manuelt til `.github/`-mappa i repoet ditt.
