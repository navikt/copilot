# Nav-Pilot Changelog

Endringslogg for nav-pilot agent harness — agenter, skills, instruksjoner, prompts og samlinger.

---

## 2026-04-20

### Språkstrategi — engelsk for maskininstruksjoner, norsk for brukersynlig output

Forskning (Multi-IF-benchmark) viser at norske instruksjoner gir 5–15 % lavere etterlevelse i LLM-er, og forverres per samtalesvng. nav-pilot hadde inkonsekvent språkblanding — det verste alternativet.

Refaktorert `nav-pilot.agent.md` med hybridstrategi:

- **Engelsk** (maskininstruksjoner): Fasemaskin-tabell, blindsoner, arketyper, beslutningstrær, review-perspektiver, leveransesjekkliste, vanlige mønstre, feilsøking, boundaries
- **Norsk** (brukersynlig output): Fasehoder, tilstandsfot, sjekkpunkt-mal, delegeringsmal, «Slik bruker du meg»-eksempler, @forfatter-delegering
- Eksplisitt språkdirektiv lagt til: «Respond to users in Norwegian. All internal instructions in this file are in English for optimal adherence.»
- Formalisert språkpolicy i AGENTS.md under «Customization Language»

Refs: #179

### Fasepersistens — nav-pilot husker hvem den er

Nav-pilot mistet fasebevissthet og persona under lange samtaler fordi instruksjonene ble erklært én gang og deretter begravd av konteksthistorikk. Omskrevet kjernemekanismen:

- **Operasjonsløkke** — erstatter engangs `<response_format>` med en 5-stegs løkke som kjøres på hvert svar: bestem fase → faseoverskrift → kun fase-tillatt arbeid → sjekkpunkt ved overgang → tilstandsfot
- **Tilstandsfot** — kompakt one-liner på slutten av hvert svar som sporer gjeldende fase, ferdige faser, nøkkelbeslutninger og åpne spørsmål. Fungerer som minneoppfrisking uten token-oppblåsing
- **Fasemaskin-tabell** — eksplisitte inn-/ut-kriterier per fase slik at modellen har et oppslagsverk for hva som er tillatt
- **Tilbakerullingsregel** — ny informasjon som konflikter med tidligere beslutninger tvinger eksplisitt retur til tidligste berørte fase
- **Utvidet Fase 3 (Review)** — fra 9 linjer til fullstendig 4-perspektiv-review med 16 konkrete spørsmål og strukturert output-mal med dom (Godkjent / Godkjent med endringer / Tilbake til Fase 2)
- **Delegeringskontrakt** — «deleger kun delproblemet, aldri hele samtalen. Gjenoppta alltid kontroll med oppsummering.» Forhindrer at spesialistagenter overtar
- **Nummererte blindsoner** — 10 punkt med krav om dekningsrapport i Fase 1-sjekkpunkt
- **Fasedisiplin i Boundaries** — nye ✅ Always og 🚫 Never-regler for faseoverskrift, tilstandsfot og fase-hopping

### Installasjonsskript — immunisert mot releasekaperng

Skills-release `v0.1.0` kapret GitHubs «Latest»-flagg og brakk `install.sh` (404 på nav-pilot-binærer):

- **Installasjonsskript** — byttet fra `/releases/latest` API til å filtrere `/releases` etter `nav-pilot/`-tag-prefiks. Nå immun mot andre release-strømmer i monorepoet
- **Skills-workflow** — lagt til `--latest=false` på `gh release edit` slik at skills-releaser aldri stjeler Latest-flagget
- **GitHub** — manuelt gjenopprettet nav-pilot-release som Latest

### Adopsjonssiden — 4 nye kategorier og verktøysammenligning

Surfacet 4 manglende skannerkategorier og lagt til verktøysammenligningsgraf:

- **BQ-views** — 4 nye kolonner i `v_adoption_summary`, `v_team_adoption`; 2 nye UNION ALL-seksjoner i `v_customization_details`
- **Nye kategorier**: copilot_setup_steps, agentic_workflows, agents_skills, nav_pilot_state
- **Gruppert CustomizationTypeChart** — delt i Copilot/Agentic/nav-pilot-seksjoner med filtrering av tomme grupper
- **Ny ToolComparisonChart** — Copilot vs Cursor vs Claude vs Windsurf sammenligning
- **TopCustomizationsChart** — 2 nye kategorier med automatisk filtrering av tomme kort

---

## 2026-04-17

### Eksport til OpenCode (`nav-pilot export opencode`)

- Ny `export`-kommando som transformerer `.github/`-artefakter til `.opencode/`-format for [OpenCode](https://github.com/anomalyco/opencode) og [oh-my-openagent](https://github.com/code-yeongyu/oh-my-openagent)
- Skills kopieres 1:1 (OpenCode støtter `name`, `description`, `license`, `metadata` nativt)
- Prompts → commands (fjerner `name` fra frontmatter, OpenCode utleder fra filnavn)
- Agenter → agents (erstatter frontmatter med `description` + `mode: subagent`, dropper VS Code-spesifikke `tools`)
- Instruksjoner + `copilot-instructions.md` → samlet `AGENTS.md` med seksjonsoverskrifter
- Støtter `--user` for global installasjon til `~/.config/opencode/`
- Gjenbruker eksisterende flagg: `--dry-run`, `--force`, `--target`, `--ref`, `--source`
- Blokkerer skriving til eksisterende `.opencode/` med mindre `--force` brukes
- YAML-safe sitering av beskrivelser med spesialtegn (`:`, `#`, etc.)

---

## 2026-04-14

### Bruker-hjemmappe-installasjon (`--user`)

- Nytt `InstallScope`-konsept (repo vs bruker) — `--user`-flagg installerer agenter, skills og instruksjoner til `~/.copilot/`
- Bruker-scope fungerer på tvers av alle repoer uten å modifisere hvert enkelt
- Instruksjoner installeres til `~/.copilot/.github/instructions/` og krever `COPILOT_CUSTOM_INSTRUCTIONS_DIRS` (kun Copilot CLI)
- nav-pilot setter env-variabelen automatisk ved lansering av cplt i interaktiv modus
- Ny `nav-pilot env`-kommando for shell-profilintegrasjon: `eval "$(nav-pilot env)"`
- Prompts støttes kun i repo-scope
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
