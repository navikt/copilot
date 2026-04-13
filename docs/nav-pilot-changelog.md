# Nav-Pilot Changelog

Endringslogg for nav-pilot agent harness — agenter, skills, instruksjoner, prompts og samlinger.

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

### Feilrettinger

- Opprettet manglende `ktor-scaffold/metadata.json`
- Refaktorert `threat-model` SKILL.md fra 613→487 linjer (ekstrahert kodeeksempler til `references/`)
- Skills lint: 0 feil

### Samlingsoversikt

| Kategori | Antall |
|----------|--------|
| Agenter | 12 |
| Skills | 22 |
| Instruksjoner | 12 |
| Prompts | 7 |
| Samlinger | 4 |
