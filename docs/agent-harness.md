# Navs Agent Harness — Kartlegging og analyse

> Oversikt over hvordan navikt/copilot regulerer AI-kodingsagenter gjennom guides (feedforward) og sensors (feedback), basert på [Martin Fowlers Harness Engineering](https://martinfowler.com/articles/harness-engineering.html).

## Hva er en harness?

En *harness* er summen av mekanismer som styrer en kodingsagent mot ønsket atferd. Fowler deler disse i to akser:

| | **Computational** (deterministisk) | **Inferential** (AI-basert) |
|---|---|---|
| **Guide** (feedforward — styrer *før* handling) | Lintere, typesjekk, codemods, CLI-verktøy | Instruksjonsfiler, agenter, skills, prompts |
| **Sensor** (feedback — korrigerer *etter* handling) | Tester, CI-sjekker, helsesjekk | Code review-agenter, arkitekturreview |

**Computational** = deterministisk, regelbasert, gir samme resultat uavhengig av kontekst.
**Inferential** = AI-basert, tolker intensjon, gir kontekstavhengig veiledning.

En moden harness har dekning i alle fire kvadranter og er koblet sammen i automatiserte feedback-løkker.

### Scope og telleprinsipp

Denne inventaren teller **konkrete artefakter** (filer, verktøy, endepunkter). Når vi skriver "15 instruksjoner" mener vi 15 `.instructions.md`-filer. Orkestrering (nav-pilot, CI-workflows som distribusjon) er listet separat fordi det er infrastruktur som *binder* kvadrantene sammen.

## Relasjon til bevisst AI-bruk (#187)

Grønn/rød sone-rammeverket (`deliberate-ai-use.instructions.md`) er et **governance-lag** som regulerer *utviklerens kompetanse*, ikke koden direkte. Det opererer over Fowlers harness-modell:

- **Grønn sone** = feedforward guide for når AI-delegering er trygt
- **Rød sone** = feedforward guide for når manuell koding bygger kritisk kompetanse
- **Generer-så-forstå** = menneskelig refleksjonspraksis (ikke en teknisk sensor, men en organisatorisk feedback-mekanisme)

Dette er utenfor Fowlers snevre scope (som handler om å regulere agenter), men det er en forutsetning for at harnessen gir kompetanseverdi og ikke bare kodekvalitet.

---

## Inventar: Navs harness-komponenter

### Computational Guides (5 kategorier)

Deterministiske verktøy og konfigurasjoner som begrenser agentens handlingsrom *før* den handler — uavhengig av LLM-tolkning.

| Komponent | Hva den gjør | Livssyklus | Automatisert? |
|-----------|-------------|------------|---------------|
| `.mise.toml` + `hack/`-skript | Orkestrerer `mise check` (fmt, lint, typecheck), `mise test`, `mise build` | Pre-commit → CI | Automatisert |
| `.nais/`-manifester | Kubernetes-konfigurasjon: ressursgrenser, helsesjekk, auth, tilgangspolicyer | Pre-deploy | Halvautomatisk (via CI) |
| Dockerfile-standarder | Chainguard-baseimages, multi-stage builds | Pre-build | Manuell |
| `ratchet:pin` / `ratchet:lint` | Pinner GitHub Actions til SHA (supply chain-sikkerhet) | Pre-commit | Manuell |
| EditorConfig / Prettier / ESLint-konfig | Formatering og stilregler som håndheves deterministisk | In-session + CI | Automatisert |

### Inferential Guides (52 artefakter)

AI-basert veiledning som former LLM-ens atferd gjennom naturlig språk. Disse er inferential fordi de *tolkes* av modellen, ikke håndheves mekanisk.

| Komponent | Antall | Livssyklus | Automatisert? |
|-----------|--------|------------|---------------|
| `.github/copilot-instructions.md` | 1 global | Pre-session (Copilot leser automatisk) | Automatisk |
| `.github/instructions/*.instructions.md` | 15 filer | In-session (pattern-matchet til filtype) | Automatisk |
| `.github/prompts/*.prompt.md` | 7 maler | In-session (bruker velger) | Manuell |
| `AGENTS.md` | 1 global | Pre-session (Copilot leser automatisk) | Automatisk |
| `.github/agents/*.agent.md` | 12 agenter | In-session (bruker nevner `@agent`) | Manuell |
| `skills/*/SKILL.md` | 23 skills | In-session (agent delegerer) | Halvautomatisk |

**Totalt:** 1 + 15 + 7 + 1 + 12 + 23 = **59 inferential guide-artefakter**

**Styrke:** 15:1-ratio mellom veiledende innhold og ren kodegenerering. Agentene forklarer *hvorfor*, ikke bare *hva*.

### Computational Sensors (9 artefakter)

Deterministiske sjekker som gir feedback *etter* handling — signalerer pass/fail uten AI-tolkning.

| Sensor | Hva den fanger | Livssyklus | Automatisert? |
|--------|---------------|------------|---------------|
| `mise check` (per app) | Formatering, lint, typesjekk, tester | Post-code / CI | Automatisert i CI, manuell in-session |
| CI-workflows (5 app-spesifikke) | Build-feil, testfeil, formatavvik som PR-blokkering | Post-commit (PR) | Automatisert |
| `mise:skills:lint` | Token-budsjett og strukturvalidering av skills | Pre-publish | Manuell |
| `mise:collections:lint` | Konsistens mellom manifest og filer | Pre-publish | Manuell |
| `docs:check` | Generert dokumentasjon matcher kilde | Pre-commit | Manuell |
| `scripts/sync-skills-dirs.sh` | Drift mellom `.github/skills/` og `skills/` | Pre-commit | Manuell |
| Trivy / zizmor | CVE-skanning og GitHub Actions-sikkerhet | Pre-commit | Manuell |
| Nais-helsesjekker | `.isalive`, `.isready`, `/metrics` | Post-deploy | Automatisert (plattformen) |
| `copilot-customization-sync.yml` | Driftdeteksjon i downstream-repoer → PR | Ukentlig (schedule) | Automatisert |

### Inferential Sensors (8 artefakter)

AI-baserte feedback-mekanismer som vurderer kvalitet kontekstuelt.

| Sensor | Domene | Livssyklus | Automatisert? |
|--------|--------|------------|---------------|
| `@code-review-agent` | Feil, sikkerhet, Nav-konvensjoner | In-session | Manuell (bruker ber) |
| `@security-champion-agent` | Trusselmodellering, GDPR, hemmeligheter | In-session | Manuell (delegert fra code-review) |
| `@accessibility-agent` | WCAG 2.1/2.2, Aksel-mønstre, tastaturnavigasjon | In-session | Manuell (delegert) |
| `@observability-agent` | Metrikk-instrumentering, tracing, varsling | In-session | Manuell (delegert) |
| threat-model skill | STRIDE-A trusselanalyse | In-session (on-demand) | Manuell |
| nav-deep-interview skill | Avdekker blinde flekker i prosjekter | In-session (on-demand) | Manuell |
| nav-architecture-review skill | Arkitektur-ADR med flerperspektiv | In-session (on-demand) | Manuell |
| security-review skill | Kodesjekk før commit/push/PR | In-session (on-demand) | Manuell |

**Merk:** Menneskelig code review i PR-prosessen er også en feedback-mekanisme, men ligger utenfor denne tekniske inventaren.

### Orkestrering (6 komponenter)

Verktøy som binder systemet sammen.

| Komponent | Funksjon |
|-----------|----------|
| `nav-pilot` CLI | Installerer og oppdaterer customization-filer i downstream-repoer |
| `copilot-customization-sync.yml` | Ukentlig workflow som oppdager drift og lager PR-er |
| `publish-skills.yaml` | Publiserer skills til GitHub Copilot Marketplace ved push |
| `copilot-metrics` (Naisjob) | Daglig innsamling av Copilot-bruksmetrikk til BigQuery |
| `copilot-adoption` (Naisjob) | Ukentlig skanning av 700+ repoer for customization-adopsjon |
| `my-copilot` (portal) | Selvbetjening, oppdagelse, statistikk, abonnement |

---

## Livssyklusposisjon

Hvor i utviklerflyten harnessen griper inn:

| Stadium | Computational | Inferential |
|---------|--------------|-------------|
| **Pre-session** | `mise.toml`-orkestrering, nav-pilot-installer, EditorConfig | AGENTS.md, `copilot-instructions.md` (automatisk lest av Copilot) |
| **In-session** | Prettier/ESLint auto-fix | 15 instructions (pattern-matchet), 12 agenter + 23 skills on-demand, 7 prompt-maler |
| **Pre-commit** | `ratchet:pin`, `docs:check`, `skills:lint` (manuelt) | `@code-review-agent` / `@security-champion-agent` (manuelt) |
| **CI (post-commit)** | `mise check` + per-app workflows (format, lint, test, build) → PR-blokkering | Ingen agentbaserte sensors; menneskelig review er utenfor teknisk harness |
| **Post-deploy** | Nais helsesjekker, readiness-prober | Ingen agentbasert feedback |
| **Asynkront** | Copilot-metrikkinnsamling (daglig), adopsjonsskanning (ukentlig), sync-driftdeteksjon | Ingen automatisk agentdrevet analyse |

---

## Koblingsstatus

### Tett koblet (aktive feedback-løkker)

- ✅ `mise check` → CI → PR-blokkering → utvikler fikser (computational guide → computational sensor → handling)
- ✅ Instructions + agents → Copilot genererer med constraints (inferential guide → kode)
- ✅ Nais helsesjekk → auto-restart ved feil (computational sensor → plattformhandling)
- ✅ `publish-skills.yaml` → Marketplace (inferential guide-distribusjon)
- ✅ `copilot-customization-sync` → PR ved drift (computational sensor → oppdatering)

### Løst koblet (manuell aktivering)

- ⚠️ `@code-review` + spesialistagenter: Bruker må eksplisitt nevne agenten
- ⚠️ Trivy/zizmor-skanninger: Utvikler må kjøre manuelt
- ⚠️ Skills (nav-plan, threat-model, nav-deep-interview): On-demand, aldri automatisk utløst
- ⚠️ `mise check` in-session: Agenter *refererer* til den men kjører den ikke alltid selv

### Ikke koblet (gap / fremtidig)

- 🔴 Ingen inferential sensors i CI — code-review-agenten kjører aldri automatisk på PR
- 🔴 Ingen post-deploy inferential sensors — AI-analyse av produksjonslogger/hendelser
- 🔴 Ingen drift-sensorer — dead code detection, test mutation quality, runtime SLO-feedback
- 🔴 Ingen lukking av feedback-løkke mellom metrikkinnsamling og guide/sensor-tuning

---

## Identifiserte gap

### Gap 1: Ingen inferential sensors i CI/PR-løpet

**Status quo:** `@code-review-agent` og `@security-champion-agent` finnes, men kjører kun når en utvikler manuelt ber om det i en session. Ingen automatisk AI-basert review på PR-er.

**Konsekvens:** Kodekvalitetssjekker som krever kontekstuell vurdering (arkitekturbrudd, sikkerhetsimplikasjoner, aksessibilitetsproblemer) fanges bare opp hvis utvikleren husker å spørre.

**Mulig tiltak:** CI-workflow som trigger `@code-review-agent` på PR-diff. Krever at agenten kan kjøre headless (uten VS Code).

### Gap 2: `mise check` kjører ikke *inne i* agent-sessions

**Status quo:** Alle agenter og instruksjoner *refererer* til `mise check` ("kjør etter endringer"), men agenten gjør det ikke alltid selv som selvkorrigering. Det er post-hoc feedback, ikke in-loop.

**Konsekvens:** Agenter kan generere kode med lint-feil som først fanges i CI (sent i løkken).

**Mulig tiltak:** Agenter som Copilot CLI kjører allerede `mise check` via sin tool-loop. Problemet er primært at instructions *oppfordrer* men ikke *krever*. Fowlers anbefaling: gjør computational sensors til obligatoriske gates i agentens tool-execution.

### Gap 3: Ingen drift-sensorer

**Status quo:** Ingen kontinuerlig overvåking av:
- Dead code-akkumulering
- Test mutation quality (er testene meningsfulle?)
- Arkitektur-fitness (overholder koden definerte arkitekturregler over tid?)
- Runtime SLO-brudd → agentforslag

**Konsekvens:** Harness-en fanger problemer ved *endring* (CI-sensors), men ikke problemer som *akkumuleres* over tid.

**Mulig tiltak:** Periodiske Naisjobs som kjører statisk analyse og rapporterer drift. Lav prioritet — dette er avansert harness-modenhet.

### Gap 4: Ingen lukket feedback-løkke mellom metrikk og guide-tuning

**Status quo:** BigQuery samler inn bruksdata (DAU, språk, funksjoner) og adopsjonsskanneren kartlegger customization-bruk. Men ingen mekanisme bruker denne dataen til å *justere* guides/sensors automatisk.

**Konsekvens:** Vi vet *at* ting brukes, men ikke *om de hjelper*. Fowler: "If sensors never fire, is that high quality or inadequate detection?"

**Mulig tiltak:** Kvartalsvis manuell gjennomgang av metrikkdata → juster instruksjoner/skills. Automatisering her er prematur.

---

## Modenhetsvurdering

| Kvadrant | Modenhet | Kommentar |
|----------|----------|-----------|
| Computational Guides | ⭐⭐⭐⭐ | Sterk dekning — lint, typesjekk, CI-pipeline på plass |
| Inferential Guides | ⭐⭐⭐⭐⭐ | Svært sterk — 59 artefakter, bredt domenedekning |
| Computational Sensors | ⭐⭐⭐⭐ | God CI, men mange manuelle steg som kunne automatiseres |
| Inferential Sensors | ⭐⭐ | Finnes, men alle er manuelt aktiverte — ingen i CI/post-deploy |

**Overordnet:** Harnessen er sterk på feedforward (guides) men svak på automatisert feedback (sensors i loop). Fowlers modell tilsier at neste modenhetssteg er å koble inferential sensors tettere til CI og post-deploy.

---

## Referanser

- [Martin Fowler: Harness Engineering](https://martinfowler.com/articles/harness-engineering.html)
- [Anthropic: Effective Harnesses for Long-Running Agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)
- [OpenAI: Harness Engineering](https://openai.com/index/harness-engineering/)
- [Stripe: Minions — one-shot end-to-end coding agents](https://stripe.dev/blog/minions-stripes-one-shot-end-to-end-coding-agents)
- [navikt/copilot#213](https://github.com/navikt/copilot/issues/213) — Tracking issue
- [navikt/copilot#209](https://github.com/navikt/copilot/issues/209) — Effektdokumentasjon
