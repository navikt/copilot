# Nav-Pilot Changelog

Endringslogg for nav-pilot agent harness — agenter, skills, instruksjoner, prompts og samlinger.

---

## 2026-04-14

### Bruker-hjemmappe-installasjon (`--user`)

- Nytt `InstallScope`-konsept (repo vs bruker) — `--user`-flagg installerer agenter og skills til `~/.copilot/`
- Bruker-scope fungerer på tvers av alle repoer uten å modifisere hvert enkelt
- Instruksjoner og prompts støttes kun i repo-scope
- Scope-felt i state-fil for å forhindre kryssforurensning

### TUI-oppgradering

- Erstattet nummererte tekstvalg med TUI-velgere (opp/ned + enter)
- Bruker `charmbracelet/huh` for Select-komponenter
- Interaktiv modus spør om repo- eller bruker-installasjon

### Feilrettinger

- Fikset uendelig «update available»-loop forårsaket av foreldet manifest-versjon
- `cplt`-lansering bruker `-- --agent` passthrough, `copilot` bruker `--agent` direkte
- `--user`-flagg avvises for kommandoer som ikke støtter det
- `--user --target .` oppdages korrekt som ugyldig (mutually exclusive)
- Symlink-beskyttelse i state-skriving dekker nå hele mappekjeden
- Versjon lagres i à-la-carte-installasjoner (`nav-pilot add`)
- Korrupt bruker-state viser advarsel i stedet for å ignoreres stille

### Refaktorering

- `installSingleFile`, `countFileIntegrity`, `shortSHA` ekstrahert som gjenbrukbare hjelpere
- All state-validering går gjennom `InstallScope`
- Deduplisert installasjonslogikk

---

## 2026-04-13

### Nye artefakter

- **threat-model** (skill) — STRIDE-A trusselmodellering for NAIS-mikrotjenester med dataflytdiagram, tillitsgrenser og risikovurdering
- **java-to-kotlin** (skill) — Rammeverk-bevisst Java→Kotlin-migrering (Spring→Ktor, JPA→Kotliquery, JUnit→Kotest)
- **performance** (instruksjon) — Core Web Vitals-mål for Next.js/Aksel-apper med server components, datafetching og bundle-optimalisering
- **security-owasp** (instruksjon) — OWASP Top 10:2025 kodemønstre med ✅/❌-eksempler i både Kotlin og Go

### Integrasjonsaudit

Gjennomført kryssreferanseaudit av alle 4 samlinger. Lagt til `Related`-tabeller i 7 instruksjoner og 1 agent for bedre kobling mellom artefakter:

- `performance` → @aksel-agent, @observability-agent, aksel-spacing, playwright-testing
- `security-owasp` → security-review, @security-champion, @auth-agent, threat-model
- `database` → flyway-migration, @nais-agent, postgresql-review
- `kotlin-ktor` → kotlin-app-config, ktor-scaffold, @auth-agent, @nais-agent, @observability-agent
- `accessibility` → @accessibility-agent, @aksel-agent, playwright-testing
- `nextjs-aksel` → @aksel-agent, @accessibility-agent, performance, aksel-spacing
- `golang` → @nais-agent, @observability-agent, security-owasp, @security-champion
- `security-champion` (agent) → threat-model, security-review, security-owasp

### Forbedrede instruksjoner

- **performance** — utvidet med Core Web Vitals-mål, server components, bundle-optimalisering
- **nextjs-aksel** — utvidet med middleware, streaming, server actions
- **accessibility** — redusert overlapp med Aksel-instruksjoner, fokus på WCAG-regler
- **golang** — utvidet med pgx, sqlc, slog, Chainguard Docker
- **kotlin-ktor** — Spring Boot-deprekering og Ktor-migreringsråd, Koin/Arrow-kt

### @forfatter-integrasjon

- Lagt til språkvask som siste del-steg i nav-pilot Fase 4
- Delegerer til `@forfatter` for klartspråk, anglismer og mikrotekst

### Omdøping

- `go-nais` → `golang` (instruksjon)
- `go-service` → `golang-service` (prompt)

### Copilot CLI-integrasjon

- `nav-pilot` CLI finner nå både `cplt` og `copilot` i PATH
- Interaktiv agentvelger — velg blant installerte agenter
- Starter Copilot CLI med `--agent`-flagg

### Tre innganger til nav-pilot

Dokumentert tre måter å bruke nav-pilot på:
- **Terminal**: `copilot --agent nav-pilot`
- **VS Code / JetBrains**: `@nav-pilot` i chat
- **nav-pilot CLI**: interaktiv modus med agentvelger

### Feilrettinger

- Opprettet manglende `ktor-scaffold/metadata.json`
- Refaktorert `threat-model` SKILL.md fra 613→487 linjer (ekstrahert kodeeksempler til `references/`)
- Rettet metadata-skjema i 3 instruksjoner (`displayName`/`domain`/`tags`/`examples`)
- Rettet Nynorsk→Bokmål i docs-tabeller og metadata
- Rettet ugyldig import-syntaks i performance-instruksjon
- Fjernet ubrukt `launchCopilot()`-funksjon
- Skills lint: 0 feil

### Samlingsoversikt

| Kategori | Antall |
|----------|--------|
| Agenter | 12 |
| Skills | 22 |
| Instruksjoner | 13 |
| Prompts | 7 |
| Samlinger | 4 |
