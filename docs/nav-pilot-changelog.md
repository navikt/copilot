# Nav-Pilot Changelog

Endringslogg for nav-pilot agent harness — agenter, skills, instruksjoner, prompts og samlinger.

## 2026-06-30

### nav-pilot CLI — robusthet, proxy og credential-varsling
- **Robust ferskhetssjekk & feilcooldown**: Lagt til 1-times cooldown på mislykkede API-søk mot GitHub for å hindre rate-limiting feilsirkler under ustabile nettverk eller offline-tilstand.
- **Proxy- og tokenstøtte**: Lagt til støtte for system-proxy (`http.ProxyFromEnvironment`) og bruk av `GITHUB_TOKEN` for ferskhetssjekk- og oppdateringskall. Økt sjekktimeout fra 2s til 5s for bedriftsnettverk.
- **Installasjons-fallback (rtk_setup)**: Implementert fallback-installasjon fra Brew til `curl` dersom Homebrew feiler. Lagt til hjelpetekster til Stderr ved mislykket hook-initialisering.
- **Feilsikker bakgrunnskloning**: Lagt til `GIT_TERMINAL_PROMPT=0` under kloningskall for å unngå henger i ikke-interaktive bakgrunnsjobber, samt mer presis parsing av git-feilmeldinger for manglende SSH-nøkler eller autentiseringstokens.
- **Atomisk skriving av cache**: Forbedret `WriteCache` til å skrive atomisk via midlertidig fil og rename for å forhindre korrupt JSON ved avbrudd.
- **Sikkerhetskonfigurasjon (.gitignore)**: Git-ignorerer lokal teststatus (`.local/`) for å forhindre innsending av testdata.

## 2026-06-26

### Refaktorering og struktur
- **Rotmappe-migrering**: Flyttet alle customization-artefakter (agents, skills, instructions, prompts) til prosjektets rotmappe for ryddigere struktur (#330).

### nav-pilot CLI — UX, robusthet og auto-oppdatering
- **nav-pilot doctor**: Erstattet den gamle `status`-kommandoen med en ny, handlingsrettet `doctor`-kommando som kjører systemhelsesjekk og gir proaktive, kopierbare løsninger på manglende kontekst, feil i konfigurasjon eller cplt sandbox-tilganger (#308, #231).
- **Sandbox-konfigurasjon**: Implementert konfigurasjon for `cplt` sandbox og synlighet i den interaktive oppsettveiviseren for å enklere sette riktig prosjektmodus (#309).
- **Auto-oppdatering og varsler**: CLI-en tilbyr nå en interaktiv oppgradering for utdaterte nav-pilot-installasjoner, samt støtte for `auto_update`-konfigurasjon (7-dagers terskel).
- **UX-løft**: Lagt til animerte spinnere under nettverkskall, `did-you-mean`-forslag ved skrivefeil i kommandoer/flagg, og tydeligere exit-koder dokumentert i `--help` (#331).
- **Konflikthåndtering i sync**: `sync --dry-run` evaluerer nå konflikter for å automatisk rydde opp dem som allerede er løst manuelt.
- **Kloning fra tilpasset `--source`**: Fikset en bug der `sync --source` feilet. CLI-en fanger nå opp og formaterer `git clone` feilmeldinger (stderr) slik at nettverks- og referansefeil blir tydelige.
- **Sikkerhetskontekst (Sandbox)**: Dokumentert `cplt` sandbox-restriksjoner eksplisitt i `nav-pilot.agent.md` og globale `AGENTS.md` for å forhindre filtilgang utenfor gjeldende workspace (#326).

### Standardisering av språk og innhold
- **Språkstandardisering**: Body-tekst i instruksjoner og skills er harmonisert til engelsk, mens metadata i YAML frontmatter forblir på norsk (#179).
- **Tilgjengelighet slanket**: Trimmet `accessibility.instructions.md` kraftig for å unngå dobbeltoppføring. Dype WCAG-remedieringer og ARIA-eksempler er samlet i `@accessibility`-agenten (#167).
- **Konsistente agentnavn**: Navngivning av flere agenter er strømlinjeformet (f.eks. ble `auth-agent` til `@auth` og `code-review-agent` til `@code-review`), inkludert manifest-oppdateringer og oppdaterte prompt-eksempler.

### Telemetri og test
- **Separasjon av bakgrunnssynk**: Telemetri skiller nå `auto_sync` fra manuelle `sync`-kall for å gi mer nøyaktig bruksstatistikk.
- **Test-robusthet (Bats)**: Bypasset macOS `noexec`-restriksjoner på `/tmp` ved å peke Bats tmp-katalog til workspace-mappen.
- Diverse opprydding etter grundige kodegjennomganger (Adversarial Review og Opus).

---

## 2026-06-09

### nav-pilot design — canonical spec og delegasjonsklarhet

- La til `docs/nav-pilot-design.md` som canonical design/spec for nav-pilot
- Festet at `@nav-pilot` er koordinator, mens spesialistagenter er leaf-only
- La inn matrise som skiller Copilot-CLI-tips fra nav-pilot-praksis
- Oppdaterte referanser fra README og agentprompt til å peke på design-docen

### nav-pilot CLI — `export opencode` token-optimalisering

`export opencode` genererte tidligere én AGENTS.md med alle instruksjoner (~4 600 linjer) som ble lastet inn av OpenCode på hvert prompt.

Ny oppførsel: instruksjoner med spesifikk `applyTo`-pattern (`.go`, `.kt`, `.tsx`, osv.) eksporteres som individuelle filer til `.opencode/instructions/<name>.md` og refereres lazily fra AGENTS.md. Globale instruksjoner (uten pattern eller `applyTo: "**"`) forblir inline i AGENTS.md.

Resultat: AGENTS.md er nå ~300 linjer i stedet for ~4 600 — ca. 85 % tokenreduksjon per prompt. Språk- og rammeverk-spesifikk kontekst lastes kun når relevant.

---

## 2026-06-05

### nav-pilot og my-copilot — sync, launch og hash-anchor

### nav-pilot CLI — launch og sync

- Launch sender ikke lenger tvungne `--mode plan` / `--effort high`; agent-default og bruker-overstyring gjelder
- `sync` viser konfliktfiler tydelig i output/JSON når de blir hoppet over
- `sync --apply` rydder `conflict`-status når filer faktisk matcher source
- Forbedret auto-sync feedback per scope (repo/user)
- Egen source-resolve-strategi for sync + utvidet testdekning

### my-copilot — navigasjon og prising

- La til hash-anchor scrolling ved hard reload (`HashAnchorScroll` i root layout)
- Robust håndtering av ugyldig URL-fragment (fallback når `decodeURIComponent` feiler)
- Synket model-pricing-data (inkludert oppdatert dato og modelliste)

---

## 2026-06-04

### nav-pilot web docs — README-audit og riktig integrasjon

Fjernet README-embedding fra `/nav-pilot/docs` og gjorde i stedet en målrettet innholdsjustering i web docs:

- La til lenke til primær dokumentasjon: `https://ki-utvikling.nav.no/nav-pilot`
- La til lenke til changelog i ressursseksjonen
- Beholdt docs-siden som kuratert dokumentasjon i stedet for å rendere README rått
- Fjernet duplikatinnhold i leseflyten:
  - «Første kommandoer» ble erstattet med pekere til «CLI-referanse»
  - «Livssyklus» ble fjernet fra TOC og erstattet med kort krysslenke til relevante seksjoner

### README — slanket for web docs-først

`docs/README.nav-pilot.md` er redusert til en kort inngangsside:

- kort beskrivelse + lenke til online docs
- minimale komme-i-gang-kommandoer
- korte bidragsyter-pekere

### my-copilot — nav-pilot README inn i web docs

Denne tilnærmingen ble testet og deretter erstattet samme dag med kuratert docs-side (se «README-audit og riktig integrasjon» over).

- Rå README-embedding i docs-side er fjernet

### nav-pilot — ekstra kosttiltak fra oppdatert research

La inn flere håndhevbare tiltak som ikke var eksplisitt dekket tidligere:

- **Ask-before-Agent gate**: små fakta-/syntaksoppgaver skal løses i Ask/chat før Agent Mode vurderes
- **Cache-hygiene**: unngå bytte av instruksjoner/verktøy midt i tråd; start ny tråd for stabil cache
- **Fasebudsjett**: grovt tokenbudsjett per fase i full-tier oppgaver
- **Governance hooks**: følg Opus-eskaleringer, Agent Mode-andel og kosttrend per oppgavetype

### Dokumentasjonsstruktur for kosteffektiv Copilot-bruk

Dokumentasjonen ble tydelig delt i fire lag for mindre sprik mellom policy og formidling:

- `.github/agents/nav-pilot.agent.md` er styrende policy (fasit)
- `docs/README.nav-pilot.md` er operativ playbook for bruk
- `docs/nav-pilot-changelog.md` er sporbar endringslogg
- `apps/my-copilot/src/app/praksis/sections/cost-optimization.tsx` er pedagogisk praksis-side

### nav-pilot — routing-policy for lavere tokenkost

La til en eksplisitt routing-policy i `nav-pilot.agent.md` for å redusere unødvendig kontekst og modellkost:

- Bruk `@research-agent` først til kartlegging og faktainnhenting
- Hold `@nav-pilot` til orkestrering, syntese og fasekontroll
- Eskaler kun smale høyrisiko-delproblemer til `@nav-pilot-opus`
- Deleger domenespørsmål til spesialistagenter i stedet for å laste alt i én kontekst

### nav-pilot — operasjonelle kostnadsvern på routing

For å dekke hele research-bildet (7 tiltak) ble policyen skjerpet med håndhevbare regler:

- **Model-gate for Opus**: Krever irreversibel/høyrisiko-beslutning + uløst tradeoff + smalt delproblem før eskalering
- **Eksplisitt «never escalate»** for rutineoppgaver (boilerplate, enkel wiring, lint/test-tolkning)
- **Konteksthygiene**: én oppgave per tråd, bruk `/compact` ved handoff, `/clear` ved problembytte
- **Tool-first** som standard: deterministiske kommandoer før bred LLM-tolkning
- **MCP/tool-pruning**: hold aktive verktøysett smale for aktuell oppgave
- **Output-disiplin**: kort output som standard, utvid bare ved reelle tradeoffs/sikkerhetsbehov

---

## 2026-06-03

### nav-pilot — sterkere fasedisiplin og rød-sone-håndhevelse

Analyse av agent-interaksjoner viste at nav-pilot for ofte hoppet over fasestopp og leverte kode uten å deklarere rød sone. Omskrevet fasemaskinen og rød/grønn-sone-systemet med 8 konkrete forbedringer. Fil: 492 → 336 linjer (−32 %).

**Fasedisiplin:**

- **PHASE INTEGRITY** — ny seksjon øverst i filen som eksplisitt forbyr fase N+1-innhold i samme svar som fase N-utput. `Phase gates override concise-by-default.`
- **Scope-klassifisering** — erstatter vage small/medium/large med eksplisitt tre-nivå-tabell (trivial/compressed/full) med entydige kriterier per nivå. Default til Full ved tvil, PII, auth, ny API-kontrakt eller nytt dataflyt
- **Kontekst-anker** — etter 5+ svar begynner nav-pilot med én linje som oppsummerer fase, nøkkelbeslutninger og åpne spørsmål. Kompenserer for LLM-konteksttap i lange samtaler
- **FORBIDDEN-regel** — eksplisitt klausulen «generating Phase N+1 content in the same response as Phase N output» fjernet tvetydighet

**Rød/grønn sone:**

- **🔴 Rød-sone-deklarasjon som punkt #10** — obligatorisk i alle Fase 2-planer. Inkluderer begrunnelse per element, ikke bare en liste. Grønn-sone-elementer er «les gjennom før merge», ikke «trygt»
- **Explain-back-regel** — etter at utvikleren implementerer rød-sone-kode ber nav-pilot dem forklare den tilbake. Mer effektivt enn stub-blokkering alene (basert på Anthropic-studie 2026)
- **Blindsoner #1/#2 alltid-obligatorisk** — personvern og tilgangskontroll merket ⚠️ uavhengig av scope-tier når endringen berører brukerdata eller nye endepunkter

**Filstruktur:**

- Fjernet «Slik bruker du meg»-seksjon (25 linjer, lav atferdsverdi)
- Kondensert HikariCP-kodeblokk og Nais YAML-eksempler til kompakte tabeller/bullets
- Forkortet Opus-eskaleringseksjon til kjernetriggere
- Vedlikeholder `<operating_loop>` XML-tag og 6 høykonsekvens-mønster inline

---

## 2026-05-28

### `$terse-mode` — native output-komprimering

Ny skill som kutter output-tokens med ~65 % uten å miste teknisk substans. Inspirert av Caveman (66k ⭐) men native i nav-pilot — ingen tredjepartsavhengighet.

- **Tre nivåer**: lett (profesjonell kort), normal (fragmenter), ultra (telegrafisk)
- **Auto-clarity**: slår seg av for sikkerhetsvarsler og destruktive handlinger
- **Persistens**: anti-drift-instruksjon hindrer modellen i å falle tilbake til verbose
- **Norsk ordliste**: dropper fyllord som «Selvfølgelig», «La meg», «Absolutt»
- Aktivér med `$terse-mode` i Copilot Chat

Tilgjengelig i alle 5 samlinger (kotlin-backend, frontend, nextjs-frontend, fullstack, platform).

### `$security-owasp` — OWASP 2025 med Java og Node.js

Oppdatert sikkerhetsskill med OWASP Top 10 2025, utvidet fra kun Go/Kotlin til også Java og Node.js/Next.js. Flyttet fra always-on instruksjon (21 KB per interaksjon) til on-demand skill.

### nav-pilot oppførsel — kortere svar og smartere kontekst

- **Concise by default**: nav-pilot gir nå korte, handlingsrettede svar som standard. Si «forklar» for detaljer.
- **Infer-and-confirm**: Infererer kontekst fra repo-filer i stedet for å stille mange spørsmål. Stiller maks 2–3 spørsmål ved store/uklare oppgaver.
- **Skill-routing**: Anvender automatisk riktig Nav-kunnskap (auth, Nais, Kafka, sikkerhet) basert på kontekst — brukeren trenger ikke huske skill-navn.

---

## 2026-05-19

### Agenter vs skills — deprecation og erstatning

Deprecerte 5 agenter som manglet verktøytilgang (ga kun råd, kunne ikke gjøre endringer). Erstattet med tilsvarende skills som fungerer som kunnskapspakker inne i agenter som *har* verktøy.

Refs: #255

### Bevisst AI-bruk — kompetansebevaringsrammeverk

Ny instruksjon (`deliberate-ai-use.instructions.md`) basert på Anthropic-, MIT- og Nav-forskning. Klassifiserer oppgaver i grønn sone (AI-egnet) og rød sone (lær manuelt først). Inkluderer «generer-så-forstå»-mønster.

Refs: #187

---

## 2026-05-14

### `nav-pilot init` — scaffold repo-lokal Copilot-konfig

Ny kommando som genererer `AGENTS.md`, `.github/copilot-instructions.md` og `.github/copilot-review-instructions.md` tilpasset repoet ditt.

### Code review-instruksjoner

Ny `code-review.instructions.md` som gir Copilot Code Review kontekst om Nav-konvensjoner (sikkerhet, Nais, auth, infrastruktur).

---

## 2026-05-07

### nav-pilot CLI forenklet til 4 kommandoer

Breaking change: CLI-en ble forenklet fra mange subcommands til `install`, `update`, `init` og `ignore`. Synk skjer nå automatisk ved install/update.

### `--sync`-flagg og default all-scopes

`nav-pilot install` synkroniserer nå alle scopes (agents, skills, instructions, prompts) som standard. Bruk `--sync=false` for å hoppe over.

---

## 2026-04-28

### `$readme-review` skill

Ny skill for strukturell gjennomgang og generering av README-er tilpasset prosjekttype (tjeneste, bibliotek, monorepo, naisjob).

### Norsk tekstkvalitets-instruksjon

Ny `norwegian-text.instructions.md` som aktiveres for alle `.md`-filer. Sikrer klart språk, riktige fagtermer og konsistent norsk.

### AI Credits-kalkulator

Ny side på ki-utvikling.nav.no som estimerer månedlig Copilot-kostnad basert på modellvalg og bruksmønster.

---

## 2026-04-22

### `nav-pilot ignore` — undertrykk påminnelser

Ny kommando for å undertrykke «nye elementer tilgjengelig»-påminnelser for spesifikke filer eller scopes.

### `/fleet` og Git worktrees-artikkel

Dokumentasjon om hvordan bruke Copilot `/fleet` med Git worktrees for parallell utvikling.

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
