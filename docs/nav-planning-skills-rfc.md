# RFC: nav-pilot — Nav's AI Developer Toolkit

**Date:** 2026-04-12
**Status:** Draft
**Author:** AI-assisted research

---

## Summary

**nav-pilot** is Nav's "oh-my-codex" — a cohesive AI developer toolkit that encodes Nav's institutional knowledge as executable workflows. Instead of building a separate CLI harness, nav-pilot delivers a **single entry point** (`@nav-pilot`) backed by planning skills, domain agents, and always-loaded Nav context. It works across VS Code, JetBrains, Copilot CLI, and GitHub.com.

```
# One install → one entry point → full pipeline
@nav-pilot I need to build a new service that processes dagpenger søknader
```

**The moat is institutional knowledge, not orchestration.**

### Architecture: Three Layers

```
┌─────────────────────────────────────────────────────────┐
│  Layer 1: Instructions (always loaded)                  │
│  Nav patterns, decision trees, anti-patterns            │
│  → Every Copilot session is Nav-aware automatically     │
├─────────────────────────────────────────────────────────┤
│  Layer 2: @nav-pilot agent (single entry point)         │
│  Orchestrates the full planning pipeline:               │
│  interview → plan → review → scaffold                   │
│  Also delegates to domain agents (@auth, @nais, @kafka) │
├─────────────────────────────────────────────────────────┤
│  Layer 3: Skills (building blocks)                      │
│  $nav-plan, $nav-deep-interview, $nav-troubleshoot...   │
│  Used by @nav-pilot, or standalone by developers        │
└─────────────────────────────────────────────────────────┘
```

### vs oh-my-codex

| Aspect         | oh-my-codex              | nav-pilot                              |
| -------------- | ------------------------ | -------------------------------------- |
| Install        | `npm install -g`         | One-click from my-copilot or curl      |
| Entry point    | `omx plan`               | `@nav-pilot`                           |
| Works in       | Terminal only            | VS Code, JetBrains, CLI, GitHub.com    |
| Updates        | `npm update`             | Auto-sync workflow (weekly PR)         |
| Knowledge      | Generic coding           | Nav's institutional playbook           |
| Maintenance    | Keep up with CLI changes | Just markdown — GitHub maintains runtime |

---

## Background: Agent Harness Landscape

The "oh-my-\*" tools (oh-my-codex, oh-my-claudecode, oh-my-openagent) are orchestration wrappers around CLI coding agents. They add multi-agent teams, lifecycle hooks, persistent state, and skills-as-markdown.

| Tool              | Stars | Wraps          | Key Innovation                                    |
| ----------------- | ----- | -------------- | ------------------------------------------------- |
| oh-my-codex       | ~21k  | OpenAI Codex   | 30+ skills, tmux teams, HUD, Sisyphus loop        |
| oh-my-claudecode  | ~28k  | Claude Code    | Same author as OMX, model routing, cost tracking  |
| oh-my-openagent   | ~49k  | OpenCode       | Provider-agnostic, 40+ lifecycle hooks             |
| OpenCode          | ~100k | Standalone     | Client-server architecture, LSP, multi-session     |

### Common Architectural Patterns

- **Multi-agent via tmux + git worktrees** — isolated parallel execution
- **Skills as markdown** — reusable agent behaviors in `.md` files
- **Pipeline execution** — clarify → plan → execute → verify
- **Lifecycle hooks** — automated pre/post actions
- **Persistent state** — context across sessions

### Key Insight: Planning Skills Are the Highest-Value Skills

Looking at the source code of OMX's `$deep-interview` (20KB) and `$plan` (19KB):

- `$deep-interview` uses **mathematical ambiguity scoring** — weighted dimensions with threshold gates that block execution until requirements are clear enough
- `$plan` implements a **Planner → Architect → Critic consensus loop** (max 5 iterations) with structured deliberation

These planning skills — not execution skills — are what differentiate a good agent harness from a generic CLI. **Planning is where the magic happens. Execution is just "do the thing."**

---

## What Nav Already Has

| Component          | Count | Status  |
| ------------------ | ----- | ------- |
| Agents             | 11    | ✅ Strong (auth, kafka, nais, security, aksel, etc.) |
| Skills             | 15    | ✅ Strong (api-design, flyway, playwright, etc.)      |
| Prompts            | 5     | ✅ Good (nais-manifest, kafka-topic, etc.)             |
| Scoped instructions| 10+   | ✅ Strong (Kotlin, Next.js, Dockerfile, CI/CD, etc.)  |
| MCP registry       | 1     | ✅ Unique                                              |
| MCP reference      | 1     | ✅ Unique                                              |
| Self-service portal| 1     | ✅ Unique (my-copilot)                                 |
| Sync workflow      | 1     | ✅ Works (copilot-customization-sync)                  |
| Collections concept| 1     | ⚠️ Exists but empty (README.collections.md)           |

### What's Missing

| Gap                              | Impact |
| -------------------------------- | ------ |
| Structured planning workflows    | High — developers spend days on decisions that should take hours |
| Curated bundles (skill packs)    | High — adoption friction is the bottleneck                       |
| Frictionless initial install     | High — sync works, but first install is manual                   |
| Nav-specific troubleshooting     | Medium — tribal knowledge locked in people's heads               |
| Migration planning               | Medium — the most dangerous changes lack structure               |

---

## What to Build

### Part 1: Planning Skills — Nav's Development Playbook

Five skills that encode Nav's institutional knowledge as executable workflows. Together they form a **pipeline**:

```
$nav-deep-interview  →  $nav-plan  →  $nav-architecture-review  →  scaffold/execute  →  $nav-troubleshoot
    (clarify)           (plan)          (validate)                   (build)              (operate)

                                                                    $nav-migrate
                                                                     (evolve)
```

#### Skill 1: `$nav-deep-interview` — Clarification Interview

**Purpose:** Expose Nav-specific blind spots _before_ implementation begins. Like OMX's `$deep-interview`, but tuned to the things Nav developers commonly miss.

**Probes by domain:**

| Domain           | Key Questions                                                                |
| ---------------- | ---------------------------------------------------------------------------- |
| Data & Privacy   | PII categories? Access model (selvbetjening/saksbehandler/system)? GDPR retention? Audit logging? |
| Platform & Auth  | Who initiates requests? Which services does it call? External exposure? Dependency failure strategy? |
| Operations       | How do you know it works in prod? Key business metrics? Alert triggers? On-call ownership? |
| Team & Process   | New vs extend? Dependent teams? Coordinated deployment? Regulatory deadline? |

**Output:** A structured requirements document with clear scope, non-goals, and identified risks.

**Complexity:** Medium. Mostly a well-structured SKILL.md with reference data about Nav's data classification levels and auth mechanisms.

---

#### Skill 2: `$nav-plan` — Architecture Planning

**Purpose:** Turn a vague idea ("I need a new service") into a concrete, Nav-compliant implementation plan by walking through Nav-specific decision points.

**Phase 1 — Intent Clarification:**
- What capability? (business need, not tech)
- Who calls it? (user-facing, service-to-service, batch, event-driven)
- What data? (PII, financial, public)
- Expected load?

**Phase 2 — Architecture Decision Tree:**

| Question            | If...                   | Then...                          |
| ------------------- | ----------------------- | -------------------------------- |
| Who calls it?       | Users via browser       | Next.js + ID-porten              |
| Who calls it?       | Other Nav services      | Ktor/Spring + TokenX             |
| Who calls it?       | External partners       | Ktor/Spring + Maskinporten       |
| Data sensitivity?   | PII (fnr, name)         | Strict accessPolicy, no logging  |
| Communication?      | Sync request/response   | REST API                         |
| Communication?      | Async events            | Kafka + Rapids & Rivers          |
| Database?           | Simple CRUD             | PostgreSQL + Flyway              |
| Database?           | Read-heavy analytics    | BigQuery                         |

**Phase 3 — Generate Plan:**
Concrete deliverables: project structure, Nais manifest, CI/CD workflow, database strategy, auth config, observability, security checklist.

**Phase 4 — Validate:**
Invoke `@security-champion` and `@nais-agent` as critics. Check: does accessPolicy match communication pattern? Is auth correct for caller type? Is observability complete?

**Phase 5 — Scaffold:**
Hand off to `spring-boot-scaffold` or equivalent skill with derived parameters.

**Complexity:** High. Requires reference data (decision trees, Nais manifest templates, access policy examples) bundled as skill assets.

---

#### Skill 3: `$nav-architecture-review` — ADR Generation

**Purpose:** Structured architecture review following Nav's Architecture Advice Process. Three perspectives evaluate the change:

1. **Planner** — Does this solve the right problem? Is scope right-sized?
2. **Architect** — Does this fit Nav's patterns? Are there simpler alternatives?
3. **Security Champion** — What are the threat vectors? Is data handling correct?

**Output:** An Architecture Decision Record (ADR) with:
- Context, decision, considered alternatives
- Nav-specific considerations (auth impact, Nais config, data classification, observability)
- Follow-up action items

**Validation loop:** Iterate perspectives until consensus. Max 3 iterations.

**Complexity:** Medium. Well-structured SKILL.md with ADR template and Nav architecture principles as reference.

---

#### Skill 4: `$nav-troubleshoot` — Platform Diagnostics

**Purpose:** Structured diagnostic trees for common Nav platform issues. Replaces "ask the Slack channel" with guided troubleshooting.

**Diagnostic trees:**

| Symptom                | Checks                                                              |
| ---------------------- | ------------------------------------------------------------------- |
| Pod won't start        | Status → CrashLoopBackOff/ImagePullBackOff/Pending → logs → manifest |
| Auth failures (401/403)| Auth mechanism → token issuer → audience → scope → accessPolicy → JWKS |
| Kafka consumer lag     | Consumer group → poison pills → processing time → offsets → R&R validation |
| DB connection issues   | Cloud SQL proxy → credentials → pool exhaustion → max_connections → Flyway |

For each: **what to check → exact command → what output means → suggested fix**.

**Complexity:** Medium. Mostly a well-organized SKILL.md with diagnostic decision trees and example commands.

---

#### Skill 5: `$nav-migrate` — Migration Planning

**Purpose:** Safe migration plans for the types of changes that teams get wrong.

**Migration types:**

| Type          | Strategy                                                             |
| ------------- | -------------------------------------------------------------------- |
| DB schema     | Expand-contract: add column → dual-write → migrate → switch reads → remove old |
| API version   | Additive if possible → v2 alongside v1 → notify consumers → monitor → deprecate |
| Kafka schema  | Backward compatible? → dual-write topics → migrate consumers → stop old → delete |
| Auth          | From/to? → affected services → gradual rollout → rollback plan      |

**Complexity:** Medium. Decision trees and checklists as reference data.

---

### Part 2: Skill Packs — Curated Bundles

**Purpose:** Solve the adoption problem. Instead of "browse 15 skills and figure out which ones you need", teams pick their stack archetype and get a complete, curated package.

This builds on the existing `README.collections.md` concept (currently "Coming Soon").

#### Proposed Packs

| Pack              | Agents                                          | Skills                                                        | Instructions                        |
| ----------------- | ----------------------------------------------- | ------------------------------------------------------------- | ----------------------------------- |
| **kotlin-backend**| auth, kafka, nais, security-champion            | api-design, flyway, kotlin-app-config, observability, security-review, tokenx | kotlin-ktor, kotlin-spring, testing |
| **nextjs-frontend**| accessibility, aksel, forfatter                | aksel-spacing, playwright, web-design-reviewer                | nextjs-aksel, testing, accessibility |
| **fullstack**     | All of above + code-review, observability       | Union of above                                                | Union of above                       |
| **platform**      | nais, observability, security-champion          | observability-setup, workstation-security                     | github-actions, dockerfile           |

#### Pack Manifest (`manifest.json`)

```json
{
  "name": "kotlin-backend",
  "description": "Agents, skills og instruksjoner for Kotlin/Ktor-team på Nais",
  "version": "2026.04",
  "agents": ["auth", "kafka", "nais", "security-champion"],
  "skills": ["api-design", "flyway-migration", "kotlin-app-config", "observability-setup", "security-review", "tokenx-auth"],
  "instructions": ["kotlin-ktor", "kotlin-spring", "testing"],
  "prompts": ["spring-boot-endpoint"],
  "planning_skills": ["nav-plan", "nav-deep-interview", "nav-architecture-review"]
}
```

#### Distribution

Three options, in order of preference:

**Option A: my-copilot web UI (recommended)**
Add a page to the my-copilot portal where teams select their stack, preview what they'll get, and click "Install" — which opens a PR on their repo with the right files and configures the sync workflow.

- Fits Nav's self-service culture
- Leverages existing my-copilot infrastructure
- Can show adoption metrics

**Option B: Install script**
```bash
curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install-pack.sh \
  | bash -s -- kotlin-backend
```

**Option C: mise task**
```bash
mise run copilot:install kotlin-backend
```

All options should configure the sync workflow so future updates are automatic.

---

### Part 3: Directory Structure

```
.github/
├── skills/
│   ├── nav-plan/                    # NEW — Architecture planning
│   │   ├── SKILL.md
│   │   ├── metadata.json
│   │   └── references/
│   │       ├── decision-tree.md     # Auth/communication/data decision trees
│   │       ├── nais-templates.md    # Nais manifest templates per archetype
│   │       └── access-policies.md   # Common access policy patterns
│   │
│   ├── nav-deep-interview/          # NEW — Clarification interview
│   │   ├── SKILL.md
│   │   ├── metadata.json
│   │   └── references/
│   │       ├── data-classification.md   # Nav's data sensitivity levels
│   │       └── blind-spots.md           # Common Nav-specific oversights
│   │
│   ├── nav-architecture-review/     # NEW — ADR generation
│   │   ├── SKILL.md
│   │   ├── metadata.json
│   │   └── references/
│   │       ├── adr-template.md      # ADR format
│   │       └── nav-principles.md    # Architecture principles
│   │
│   ├── nav-troubleshoot/            # NEW — Platform diagnostics
│   │   ├── SKILL.md
│   │   ├── metadata.json
│   │   └── references/
│   │       └── diagnostic-trees.md  # All diagnostic decision trees
│   │
│   ├── nav-migrate/                 # NEW — Migration planning
│   │   ├── SKILL.md
│   │   ├── metadata.json
│   │   └── references/
│   │       └── migration-patterns.md
│   │
│   └── ... (existing 15 skills)
│
├── collections/                     # Skill packs (rename from proposed skill-packs)
│   ├── kotlin-backend/
│   │   └── manifest.json
│   ├── nextjs-frontend/
│   │   └── manifest.json
│   ├── fullstack/
│   │   └── manifest.json
│   └── platform/
│       └── manifest.json
│
└── ... (existing agents, instructions, prompts)

scripts/
└── install-pack.sh                  # Pack installer script
```

---

## Prioritization

| Priority | Deliverable                    | Why                                              |
| -------- | ------------------------------ | ------------------------------------------------ |
| **P0**   | `$nav-plan`                    | #1 thing developers struggle with — architecture decisions |
| **P0**   | `$nav-deep-interview`          | Prevents the most common planning failures        |
| **P0**   | Skill pack manifests + installer | Makes adoption frictionless                      |
| **P1**   | `$nav-architecture-review`     | Encodes Architecture Advice Process               |
| **P1**   | `$nav-troubleshoot`            | Reduces time-to-resolution for platform issues    |
| **P1**   | my-copilot install page        | Self-service pack installation via web UI         |
| **P2**   | `$nav-migrate`                 | Prevents the most dangerous changes from going wrong |
| **P2**   | Staleness dashboard            | Track which teams have outdated customizations    |

---

## What NOT to Build

| Don't Build           | Why                                                              |
| --------------------- | ---------------------------------------------------------------- |
| Separate CLI binary   | Skills work natively in Copilot CLI / VS Code / JetBrains       |
| Multi-agent orchestration | Being commoditized — Copilot CLI absorbs this pattern         |
| HUD/dashboard         | Not a differentiator, Copilot CLI UI improving rapidly           |
| Model routing         | Nav uses GitHub Copilot — model selection is GitHub's problem    |
| New agent runtime     | Leverage Copilot CLI as the base, focus on Nav-specific content  |

---

## Open Questions

1. **Which planning skill to prototype first?** Recommendation: `$nav-plan` — it's the most impactful and can be iterated.

2. **Should skill packs live in `.github/collections/` or `.github/skill-packs/`?** The existing `README.collections.md` uses "collections" terminology — should we align?

3. **Should planning skills invoke existing agents as critics?** e.g., `$nav-plan` Phase 4 invokes `@security-champion` and `@nais-agent`. This creates agent-to-skill dependencies.

4. **How sophisticated should ambiguity scoring be?** OMX's deep-interview has mathematical scoring with weighted dimensions. Is that overkill for Nav, or does it prevent developers from skipping the interview?

5. **my-copilot install page scope?** Should it just generate a PR, or also preview what each pack contains and show adoption metrics per team?

---

## Appendix A: Nav Architecture Patterns (from navikt repo analysis)

This appendix documents concrete patterns observed across real navikt repositories. This is the **reference data** that planning skills will bundle — the institutional knowledge that makes Nav's skills impossible to replicate generically.

Repos analyzed: `dp-behandling`, `tiltakspenger-saksbehandling-api`, `helse-spesialist`, `dinesykmeldte-backend`, `familie-ba-sak`, `familie-tilbake`, `sykepengesoknad-frontend`, `pensjonskalkulator-frontend`, `nav-dekoratoren`, `amt-deltakelser`, `toi-rapids-and-rivers`, `arbeidsoppfolging-adr`, and others.

---

### A.1 Auth Decision Tree

```
WHO CALLS YOUR SERVICE?
│
├─ Citizens (BankID/MinID login)
│  → ID-porten + Wonderwall sidecar
│  → Optional: TokenX for downstream calls on behalf of user
│  Nais: idporten.enabled: true, idporten.sidecar.enabled: true
│  Library: @navikt/oasis (Node.js) or token-support (JVM)
│
├─ Internal Nav services (with user context)
│  → TokenX (on-behalf-of token exchange)
│  Nais: tokenx.enabled: true
│  Library: @navikt/oasis or token-support
│
├─ Internal Nav services (no user context — batch, cron)
│  → Azure AD / Entra ID (client_credentials)
│  Nais: azure.application.enabled: true
│  Library: @navikt/oasis or token-support
│
└─ External partners / government APIs
   → Maskinporten (JWT bearer grant)
   Nais: maskinporten.enabled: true, maskinporten.scopes: [...]
   Library: token-support
```

**Token validation libraries:**

| Language    | Library                              | Repo                   |
| ----------- | ------------------------------------ | ---------------------- |
| Node.js     | `@navikt/oasis`                      | navikt/oasis           |
| Spring Boot | `no.nav.security:token-validation-spring` | navikt/token-support |
| Ktor        | `no.nav.security:token-validation-ktor-v3` | navikt/token-support |

**Common auth mistakes:**
- Using Azure `client_credentials` when user context is needed (breaks audit trail)
- Not setting `accessPolicy.inbound` (service unreachable — network policy blocks all)
- Forgetting `idporten.sidecar.enabled: true` for Node.js apps
- Reusing tokens across multiple downstream calls instead of per-target OBO exchange

---

### A.2 Nais Manifest Patterns

**Resource sizing (from real manifests):**

| Service Type         | CPU Request | Memory Request | Memory Limit | Replicas |
| -------------------- | ----------- | -------------- | ------------ | -------- |
| Rapids listener       | 12m         | 360Mi          | 512Mi        | min: 2   |
| Standard web service  | 25m         | 1024Mi         | 1024Mi       | min: 2, max: 4 |
| Frontend (Next.js)    | 50m         | 256Mi          | 512Mi        | min: 2, max: 5 |

**Ingress conventions:**
- Dev internal: `https://{app}.intern.dev.nav.no`
- Prod internal: `https://{app}.intern.nav.no`
- Prod public: `https://{app}.nav.no`

**Environment variable patterns:**
```yaml
env:
  - name: JDK_JAVA_OPTIONS
    value: -XX:+UseParallelGC -XX:ActiveProcessorCount=4
  - name: KAFKA_RAPID_TOPIC
    value: team{navn}.rapid.v1
  - name: KAFKA_CONSUMER_GROUP_ID
    value: {app-name}-v1
```

**Observability (always enabled):**
```yaml
observability:
  autoInstrumentation:
    enabled: true
    runtime: java  # or nodejs
  logging:
    destinations:
      - id: loki
      - id: elastic
prometheus:
  enabled: true
  path: /metrics
```

**accessPolicy pattern:**
```yaml
accessPolicy:
  inbound:
    rules:
      - application: caller-app
        namespace: caller-team
  outbound:
    rules:
      - application: downstream-app
        namespace: downstream-team
    external:
      - host: external-api.nav.no
```

---

### A.3 Kotlin/Ktor Application Patterns

**Bootstrapping — two main patterns:**

1. **RapidApplication + Ktor** (event-driven services):
```kotlin
fun main() {
    ApplicationBuilder(Configuration.config).start()
}

internal class ApplicationBuilder(config: Map<String, String>) :
    RapidsConnection.StatusListener {
    private val rapidsConnection = RapidApplication.create(
        env = config,
        builder = {
            withKtor { preStopHook, rapid ->
                naisApp(
                    meterRegistry = meterRegistry,
                    aliveCheck = rapid::isReady,
                    readyCheck = rapid::isReady,
                ) {
                    myApi(...)
                }
            }
        },
    ) { _, rapidsConnection ->
        MyEventHandler(rapidsConnection)
    }
}
```

2. **Embedded Ktor + background jobs** (API services):
```kotlin
fun main() {
    val server = embeddedServer(Netty, port = 8080) {
        ktorSetup(applicationContext)
    }
    server.start(wait = true)
}
```

**Configuration — Konfig library with environment detection:**
```kotlin
object Configuration {
    private val defaultProperties = ConfigurationMap(mapOf(...))

    val properties = ConfigurationProperties.systemProperties() overriding
        EnvironmentVariables() overriding defaultProperties

    fun config() = when (System.getenv("NAIS_CLUSTER_NAME")) {
        "dev-gcp" -> systemProperties() overriding EnvironmentVariables overriding devProperties overriding defaultProperties
        "prod-gcp" -> systemProperties() overriding EnvironmentVariables overriding prodProperties overriding defaultProperties
        else -> systemProperties() overriding EnvironmentVariables overriding localProperties overriding defaultProperties
    }
}
```

**Error handling — StatusPages with typed exceptions:**
```kotlin
fun Application.configureExceptions() {
    install(StatusPages) {
        exception<Throwable> { call, cause ->
            when (cause) {
                is TilgangException -> call.respond(HttpStatusCode.Forbidden, cause.toErrorJson())
                is IkkeFunnetException -> call.respond(HttpStatusCode.NotFound, ikkeFunnet())
                is ContentTransformationException -> call.respond(HttpStatusCode.BadRequest, ugyldigRequest())
                else -> call.respond(HttpStatusCode.InternalServerError, serverfeil())
            }
        }
    }
}
```

**Common dependencies (from real build.gradle.kts):**
```gradle
// Database
implementation("com.zaxxer:HikariCP:7.0.2")
implementation("org.postgresql:postgresql:42.7.10")
implementation("com.github.seratch:kotliquery:1.9.1")
implementation("org.flywaydb:flyway-database-postgresql:12.3.0")

// Logging
implementation("io.github.oshai:kotlin-logging-jvm:8.0.01")
implementation("ch.qos.logback:logback-classic:1.5.32")
implementation("net.logstash.logback:logstash-logback-encoder:9.0")

// Config
implementation("com.natpryce:konfig:1.6.10.0")

// Testing
testImplementation("io.kotest:kotest-assertions-core")
testImplementation("io.mockk:mockk")
testImplementation("org.testcontainers:postgresql")
testImplementation("com.github.navikt.mock-oauth2-server:mock-oauth2-server")
```

---

### A.4 Database Patterns

**Table design — VARCHAR primary keys, JSONB for flexible data:**
```sql
CREATE TABLE sykmelding (
    sykmelding_id VARCHAR PRIMARY KEY NOT NULL,
    pasient_fnr VARCHAR NOT NULL,
    orgnummer VARCHAR NOT NULL,
    sykmelding JSONB NOT NULL,
    lest BOOLEAN NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    latest_tom DATE NOT NULL
);
```

**HikariCP — right-sized for containers:**
```kotlin
HikariConfig().apply {
    maximumPoolSize = 5              // Small pool for K8s containers
    minimumIdle = 3
    isAutoCommit = false
    transactionIsolation = "TRANSACTION_READ_COMMITTED"
    connectionTimeout = 10_000       // 10s
    idleTimeout = 600_000            // 10 min
    maxLifetime = 1_800_000          // 30 min
}
```

**Cloud SQL in Nais — dev vs prod:**
```yaml
# Dev
gcp:
  sqlInstances:
    - type: POSTGRES_17
      tier: db-f1-micro              # Smallest tier
      highAvailability: false
      databases:
        - name: my-app
          envVarPrefix: DB

# Prod
gcp:
  sqlInstances:
    - type: POSTGRES_17
      tier: db-custom-4-3840         # 4 vCPU, 3.8GB RAM
      highAvailability: true
      autoBackupHour: 2
      databases:
        - name: my-app
          envVarPrefix: DB
      flags:
        - name: cloudsql.enable_pgaudit
          value: "on"
```

**Common DB mistakes:**
- Forgetting `envVarPrefix` on database config (no connection string injected)
- Using default HikariCP pool size of 10 (OOM in small containers)
- Changing `type: POSTGRES_XX` without following upgrade procedure (data loss)
- Missing indexes on foreign key columns (slow joins)

---

### A.5 Kafka / Rapids & Rivers Patterns

**Topic definition:**
```yaml
apiVersion: kafka.nais.io/v1
kind: Topic
metadata:
  name: rapid-1
  namespace: my-team
spec:
  pool: nav-dev           # nav-dev or nav-prod
  config:
    cleanupPolicy: delete
    partitions: 1          # Dev: 1, Prod: 6+
    replication: 3
    retentionHours: 336    # 14 days
  acl:
    - team: my-team
      application: my-app
      access: readwrite
```

**Topic naming:** `{team}.rapid.v1` for Rapids bus, `privat-{team}-{domain}` for domain topics.

**River event handler:**
```kotlin
internal class MyEventHandler(rapidsConnection: RapidsConnection) :
    River.PacketListener {
    init {
        River(rapidsConnection).apply {
            precondition {
                it.requireValue("@event_name", "MyEvent")
                it.requireAny("kode", allowedCodes)
            }
            validate {
                it.requireKey("@id", "@opprettet")
                it.requireKey("ident", "behandlingId")
            }
        }.register(this)
    }

    override fun onPacket(
        packet: JsonMessage,
        context: MessageContext,
        metadata: MessageMetadata,
        meterRegistry: MeterRegistry,
    ) {
        withLoggingContext("behandlingId" to packet["behandlingId"].asText()) {
            packet["@event_name"] = "behov"
            packet["@behov"] = listOf("MyBehov")
            context.publish(packet.toJson())
        }
    }
}
```

**Bootstrap:**
```kotlin
fun main() {
    RapidApplication.create(System.getenv()).apply {
        MyEventHandler(this)
        AnotherHandler(this)
    }.start()
}
```

---

### A.6 Frontend Patterns (Next.js)

**Auth — Wonderwall + Oasis:**
```typescript
import { getToken, validateIdportenToken, requestTokenxOboToken } from '@navikt/oasis'

async function beskyttetSide(req: GetServerSidePropsContext['req']) {
    const token = getToken(req)
    if (!token) return { redirect: { destination: '/oauth2/login?redirect=' + req.url } }

    const validation = await validateIdportenToken(token)
    if (!validation.ok) return wonderwallRedirect

    // Exchange for backend token (OBO)
    const obo = await requestTokenxOboToken(token, 'prod:my-team:backend-api')
    // Call backend with obo.token
}
```

**BFF proxy pattern:**
```typescript
import { proxyApiRouteRequest } from '@navikt/next-api-proxy'
import { requestOboToken } from '@navikt/oasis'

// Exchange ID-porten token for backend token, then proxy
const tokenX = await requestOboToken(idportenToken, backendClientId)
await proxyApiRouteRequest({ ...opts, bearerToken: tokenX.token })
```

**Nais manifest for frontends:**
```yaml
spec:
  port: 3000
  idporten:
    enabled: true
    sidecar:
      enabled: true
      level: Level4
  tokenx:
    enabled: true
  observability:
    autoInstrumentation:
      enabled: true
      runtime: nodejs
```

**Common dependencies:**
```json
{
  "@navikt/ds-react": "^7.40.0",
  "@navikt/aksel-icons": "^7.40.0",
  "@navikt/ds-tailwind": "^7.40.0",
  "@navikt/oasis": "^4.x",
  "@navikt/nav-dekoratoren-moduler": "^3.4.0",
  "@navikt/next-api-proxy": "^4.1.x",
  "@tanstack/react-query": "^5.90.0"
}
```

**Current state:** Pages Router (legacy) → App Router (new projects) → Vite monorepos (latest).

---

### A.7 CI/CD Patterns

**Standard workflow structure:**
```
push to main → build + test → docker image → deploy dev → deploy prod
```

**Key actions:**
- `nais/docker-build-push@v0` — builds and pushes to GAR
- `nais/deploy/actions/deploy@v2` — deploys to Nais cluster
- Image tagged with `github.sha` — same image to all environments
- Environment-specific config via `.nais/vars-{env}.yaml`

**Monorepo pattern:** `dorny/paths-filter` → matrix strategy → reusable workflow per module.

**Parallel deploy:** Dev and prod can deploy simultaneously (both depend only on build, not each other).

---

### A.8 Common Anti-Patterns

| Anti-Pattern | Impact | Fix |
| ------------ | ------ | --- |
| Using Azure `client_credentials` with user context | Breaks audit trail, no `sub` claim | Use TokenX OBO |
| Not setting `accessPolicy.inbound` | Service unreachable (network policy blocks) | Explicitly list callers |
| Default HikariCP pool size (10) | OOM in containers with 512Mi memory | Reduce to 3–5 |
| Changing `POSTGRES_XX` version in Nais | Data loss — triggers new instance | Follow upgrade procedure |
| Forgetting `envVarPrefix` on Cloud SQL | App can't connect (no env vars injected) | Add `envVarPrefix: DB` |
| Same path for liveness/readiness | Can't distinguish startup from runtime issues | Separate probes |
| Outdated FSS rules in accessPolicy | Unnecessary access grants after GCP migration | Remove stale rules |
| Logging PII (fnr, names) | GDPR violation | Use `sikkerlogg` for sensitive data |
| Missing `CONCURRENTLY` on large table indexes | Table locks during migration | Use `CREATE INDEX CONCURRENTLY` |

---

### A.9 Shared Platform Libraries

| Library/Operator | Purpose | Used By |
| ---------------- | ------- | ------- |
| **Wonderwall** (nais) | OIDC sidecar for frontends | All citizen-facing apps |
| **Tokendings** (nais) | TokenX token exchange service | All service-to-service with user context |
| **Azurerator** (nais) | Azure AD app registration operator | All apps with Azure AD |
| **Kafkarator** (nais) | Kafka topic/user management | All Kafka users |
| **Naiserator** (nais) | YAML → Kubernetes resources | All Nais apps |
| **token-support** (navikt) | JVM token validation framework | All Kotlin/Java backends |
| **@navikt/oasis** | Node.js token validation + exchange | All Next.js frontends |
| **rapids-and-rivers** (navikt) | Kafka event bus framework | Event-driven services |
| **@navikt/ds-react** (Aksel) | Design system components | All frontends |
| **@navikt/nav-dekoratoren-moduler** | Header/footer decorator | All citizen-facing frontends |

---

### A.10 ADR Practice

**Format:** Team-specific ADR repos (e.g., `navikt/arbeidsoppfolging-adr`) with date-based filenames.

**What gets documented:**
- Major platform migrations (Arena → GCP)
- Service integration patterns (sync vs async)
- Kafka topic ownership decisions
- Data ownership changes
- Auth mechanism choices

**Structure:**
1. Participants/stakeholders
2. Problem statement (Problemstilling)
3. Solution alternatives (evaluated with pros/cons)
4. Decision and rationale
5. Consequences and follow-ups

---

## References

- [oh-my-codex](https://github.com/Yeachan-Heo/oh-my-codex) — 21k stars, TypeScript+Rust
- [oh-my-claudecode](https://github.com/Yeachan-Heo/oh-my-claudecode) — 28k stars, TypeScript
- [oh-my-openagent](https://github.com/code-yeongyu/oh-my-openagent) — 49k stars, TypeScript
- [OpenCode](https://github.com/anomalyco/opencode) — 100k stars, TypeScript
- [Agent Skills Specification](https://agentskills.io/specification)
- [Nav Architecture Advice Process](https://sikkerhet.nav.no/) — internal
- [OMX deep-interview source](https://github.com/Yeachan-Heo/oh-my-codex/blob/main/skills/deep-interview/SKILL.md) — 20KB structured interview
- [OMX plan source](https://github.com/Yeachan-Heo/oh-my-codex/blob/main/skills/plan/SKILL.md) — 19KB consensus planning
- [nais/doc](https://github.com/nais/doc) — Official Nais platform documentation
- [navikt/token-support](https://github.com/navikt/token-support) — JVM token validation framework
- [navikt/oasis](https://github.com/navikt/oasis) — Node.js token validation and exchange
- [navikt/arbeidsoppfolging-adr](https://github.com/navikt/arbeidsoppfolging-adr) — Example team ADR repo
