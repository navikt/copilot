# RFC: nav-pilot, Nav's AI developer toolkit

**Date:** 2026-04-12
**Status:** Draft
**Author:** AI-assisted research

---

## Summary

nav-pilot is Nav's version of oh-my-codex. It turns Nav's institutional knowledge into workflows an agent can run. We do not build a CLI harness. Developers get one entry point, `@nav-pilot`, wherever they already work.

```
# One install, one entry point, full pipeline
@nav-pilot I need to build a new service that processes dagpenger søknader
```

The moat is institutional knowledge, not orchestration.

### Architecture: three layers

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

| Aspect      | oh-my-codex              | nav-pilot                             |
| ----------- | ------------------------ | ------------------------------------- |
| Install     | `npm install -g`         | One-click from my-copilot or curl     |
| Entry point | `omx plan`               | `@nav-pilot`                          |
| Works in    | Terminal only            | VS Code, JetBrains, CLI, GitHub.com   |
| Updates     | `npm update`             | Auto-sync workflow (weekly PR)        |
| Knowledge   | Generic coding           | Nav's institutional playbook          |
| Maintenance | Keep up with CLI changes | Just markdown, GitHub owns the runtime |

---

## Background: agent harnesses

The oh-my-\* tools are orchestration wrappers around CLI coding agents. They add multi-agent teams over tmux and git worktrees, lifecycle hooks, state that survives between sessions, and skills written as markdown files. The pipeline is always some variant of clarify, plan, execute, verify.

| Tool             | Stars | Wraps        | Key innovation                                   |
| ---------------- | ----- | ------------ | ------------------------------------------------ |
| oh-my-codex      | ~21k  | OpenAI Codex | 30+ skills, tmux teams, HUD, Sisyphus loop       |
| oh-my-claudecode | ~28k  | Claude Code  | Same author as OMX, model routing, cost tracking |
| oh-my-openagent  | ~49k  | OpenCode     | Provider-agnostic, 40+ lifecycle hooks           |
| OpenCode         | ~100k | Standalone   | Client-server architecture, LSP, multi-session   |

### The planning skills are the valuable ones

Read the source of OMX's `$deep-interview` (20KB) and `$plan` (19KB) and the pattern is clear. `$deep-interview` scores ambiguity across weighted dimensions and blocks execution until the requirements clear a threshold. `$plan` runs a Planner, Architect and Critic consensus loop, capped at 5 iterations.

That is what separates a good harness from a generic CLI. Execution is just doing the thing. Planning is where the value sits, and planning is exactly where Nav-specific knowledge pays off.

---

## What Nav already has

| Component           | Count | Notes                                                 |
| ------------------- | ----- | ----------------------------------------------------- |
| Agents              | 11    | auth, kafka, nais, security, aksel and more            |
| Skills              | 15    | api-design, flyway, playwright and more                |
| Prompts             | 5     | nais-manifest, kafka-topic and more                    |
| Scoped instructions | 10+   | Kotlin, Next.js, Dockerfile, CI/CD and more            |
| MCP registry        | 1     | Unique to Nav                                          |
| MCP reference       | 1     | Unique to Nav                                          |
| Self-service portal | 1     | my-copilot, unique to Nav                              |
| Sync workflow       | 1     | copilot-customization-sync, works today                |
| Collections concept | 1     | ⚠️ Exists but empty (`README.collections.md`)          |

The content is strong. The paths into it are not. Nothing structures the planning work, so developers spend days on decisions that should take hours. Nothing bundles the pieces, so a new team has to browse 15 skills and guess. Sync works once you are set up, but the first install is manual. The platform troubleshooting knowledge that would save the most time is still in people's heads and in Slack threads. And migrations, the changes most likely to break something, have no structure at all.

---

## What to build

### Part 1: planning skills, Nav's development playbook

Five skills that encode Nav's institutional knowledge as workflows. Together they form a pipeline:

```
$nav-deep-interview  →  $nav-plan  →  $nav-architecture-review  →  scaffold/execute  →  $nav-troubleshoot
    (clarify)           (plan)          (validate)                   (build)              (operate)

                                                                    $nav-migrate
                                                                     (evolve)
```

#### Skill 1: `$nav-deep-interview`, the clarification interview

Expose Nav-specific blind spots _before_ implementation starts. Same idea as OMX's `$deep-interview`, tuned to what Nav developers actually miss.

| Domain          | Key questions                                                                                       |
| --------------- | --------------------------------------------------------------------------------------------------- |
| Data & privacy  | PII categories? Access model (selvbetjening/saksbehandler/system)? GDPR retention? Audit logging?     |
| Platform & auth | Who initiates requests? Which services does it call? External exposure? Dependency failure strategy?   |
| Operations      | How do you know it works in prod? Key business metrics? Alert triggers? On-call ownership?            |
| Team & process  | New vs extend? Dependent teams? Coordinated deployment? Regulatory deadline?                          |

Out comes a requirements document with scope, non-goals and the risks the interview surfaced.

---

#### Skill 2: `$nav-plan`, architecture planning

Turn a vague idea ("I need a new service") into a concrete, Nav-compliant plan by walking the Nav-specific decision points.

**Phase 1, intent.** What capability, stated as a business need rather than a technology? Who calls it: users, other services, batch, events? What data, PII or financial or public? What load do you expect?

**Phase 2, architecture decision tree.**

| Question          | If                    | Then                            |
| ----------------- | --------------------- | ------------------------------- |
| Who calls it?     | Users via browser     | Next.js + ID-porten             |
| Who calls it?     | Other Nav services    | Ktor/Spring + TokenX            |
| Who calls it?     | External partners     | Ktor/Spring + Maskinporten      |
| Data sensitivity? | PII (fnr, name)       | Strict accessPolicy, no logging |
| Communication?    | Sync request/response | REST API                        |
| Communication?    | Async events          | Kafka + Rapids & Rivers         |
| Database?         | Simple CRUD           | PostgreSQL + Flyway             |
| Database?         | Read-heavy analytics  | BigQuery                        |

**Phase 3, generate the plan.** Project structure, Nais manifest, CI/CD workflow, database strategy, auth config, observability, security checklist.

**Phase 4, validate.** Invoke `@security-champion` and `@nais-agent` as critics. Does the accessPolicy match the communication pattern? Is the auth right for the caller type? Is observability complete?

**Phase 5, scaffold.** Hand off to `spring-boot-scaffold` or an equivalent skill with the parameters derived above.

---

#### Skill 3: `$nav-architecture-review`, ADR generation

A structured review following Nav's Architecture Advice Process. Three perspectives look at the change:

1. Planner. Does this solve the right problem? Is the scope right-sized?
2. Architect. Does this fit Nav's patterns? Are there simpler alternatives?
3. Security champion. What are the threats? Is the data handled correctly?

They iterate until they agree, at most 3 rounds. The output is an Architecture Decision Record covering context, the decision, the alternatives considered, the Nav-specific consequences (auth impact, Nais config, data classification, observability) and follow-up actions.

---

#### Skill 4: `$nav-troubleshoot`, platform diagnostics

Diagnostic trees for the platform problems that today get answered by asking in a Slack channel.

| Symptom                 | Checks                                                                     |
| ----------------------- | -------------------------------------------------------------------------- |
| Pod won't start         | Status → CrashLoopBackOff/ImagePullBackOff/Pending → logs → manifest        |
| Auth failures (401/403) | Auth mechanism → token issuer → audience → scope → accessPolicy → JWKS      |
| Kafka consumer lag      | Consumer group → poison pills → processing time → offsets → R&R validation  |
| DB connection issues    | Cloud SQL proxy → credentials → pool exhaustion → max_connections → Flyway  |

Every step names what to check, the exact command, how to read the output, and the fix it points to.

---

#### Skill 5: `$nav-migrate`, migration planning

Safe migration plans for the kinds of change teams get wrong.

| Type         | Strategy                                                                       |
| ------------ | ------------------------------------------------------------------------------ |
| DB schema    | Expand-contract: add column → dual-write → migrate → switch reads → remove old  |
| API version  | Additive if possible → v2 alongside v1 → notify consumers → monitor → deprecate |
| Kafka schema | Backward compatible? → dual-write topics → migrate consumers → stop old → delete |
| Auth         | From/to? → affected services → gradual rollout → rollback plan                  |

Only `$nav-plan` is high complexity, because it needs the decision trees, manifest templates and access policy examples bundled with it. The other four are medium: a well-structured SKILL.md plus the reference files listed in Part 3.

---

### Part 2: skill packs, curated bundles

Adoption is the bottleneck, not content. Instead of asking a team to browse 15 skills and work out which ones apply, let them pick their stack archetype and get the whole set. This fills in the existing `README.collections.md` concept, which today says "Coming Soon".

| Pack            | Agents                                    | Skills                                                                       | Instructions                        |
| --------------- | ----------------------------------------- | ---------------------------------------------------------------------------- | ----------------------------------- |
| kotlin-backend  | auth, kafka, nais, security-champion      | api-design, flyway, kotlin-app-config, observability, security-review, tokenx | kotlin-ktor, kotlin-spring, testing  |
| nextjs-frontend | accessibility, aksel, forfatter           | aksel-builder, playwright, web-design-reviewer                               | nextjs-aksel, testing, accessibility |
| fullstack       | all of the above + code-review, observability | union of the above                                                       | union of the above                   |
| platform        | nais, observability, security-champion    | observability-setup, workstation-security                                    | github-actions, dockerfile           |

Each pack is a `manifest.json`:

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

Three options, in order of preference. Whichever we pick, installing a pack must also configure the sync workflow so updates keep arriving.

**Option A, the my-copilot web UI. Recommended.** A page where a team picks its stack, previews what it gets, and clicks Install, which opens a PR on their repo with the right files. It fits how Nav does self-service, it runs on portal infrastructure we already have, and it is the only option that can show us adoption numbers.

**Option B, an install script.**

```bash
curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install-pack.sh \
  | bash -s -- kotlin-backend
```

**Option C, a mise task.**

```bash
mise run copilot:install kotlin-backend
```

---

### Part 3: directory structure

Every new skill directory holds a `SKILL.md`, a `metadata.json` and a `references/` folder. The reference files are the part that differs:

```
.github/
├── skills/
│   ├── nav-plan/                        # NEW, architecture planning
│   │   └── references/
│   │       ├── decision-tree.md         # Auth/communication/data decision trees
│   │       ├── nais-templates.md        # Nais manifest templates per archetype
│   │       └── access-policies.md       # Common access policy patterns
│   │
│   ├── nav-deep-interview/              # NEW
│   │   └── references/
│   │       ├── data-classification.md   # Nav's data sensitivity levels
│   │       └── blind-spots.md           # Common Nav-specific oversights
│   │
│   ├── nav-architecture-review/         # NEW
│   │   └── references/
│   │       ├── adr-template.md          # ADR format
│   │       └── nav-principles.md        # Architecture principles
│   │
│   ├── nav-troubleshoot/                # NEW
│   │   └── references/
│   │       └── diagnostic-trees.md      # All diagnostic decision trees
│   │
│   ├── nav-migrate/                     # NEW
│   │   └── references/
│   │       └── migration-patterns.md
│   │
│   └── ... (existing 15 skills)
│
├── collections/                         # Skill packs, one manifest.json each
│   ├── kotlin-backend/
│   ├── nextjs-frontend/
│   ├── fullstack/
│   └── platform/
│
└── ... (existing agents, instructions, prompts)

scripts/
└── install-pack.sh                      # Pack installer script
```

---

## Prioritization

| Priority | Deliverable                      | Why                                                  |
| -------- | -------------------------------- | ---------------------------------------------------- |
| P0       | `$nav-plan`                      | Architecture decisions are the number one struggle    |
| P0       | `$nav-deep-interview`            | Prevents the most common planning failures            |
| P0       | Skill pack manifests + installer | Makes adoption frictionless                           |
| P1       | `$nav-architecture-review`       | Encodes the Architecture Advice Process               |
| P1       | `$nav-troubleshoot`              | Cuts time to resolution for platform issues           |
| P1       | my-copilot install page          | Self-service pack installation from the web UI        |
| P2       | `$nav-migrate`                   | Keeps the most dangerous changes from going wrong     |
| P2       | Staleness dashboard              | Shows which teams are running outdated customizations |

---

## What not to build

We are deliberately not building a CLI or a runtime. Skills run natively in Copilot CLI, VS Code and JetBrains, GitHub maintains that runtime, and everything we ship stays markdown.

Three more things we are leaving alone. Multi-agent orchestration is being commoditized, and Copilot CLI is absorbing the pattern. A HUD or dashboard is no differentiator when the Copilot CLI UI is improving this fast. Model routing is GitHub's problem, since Nav uses GitHub Copilot.

---

## Open questions

1. Which planning skill do we prototype first? `$nav-plan` is the recommendation. It has the most impact and we can iterate on it.
2. Should the packs live in `.github/collections/` or `.github/skill-packs/`? `README.collections.md` already says "collections", so aligning on that is the obvious move, but it is worth a decision rather than a drift.
3. Should planning skills invoke existing agents as critics? `$nav-plan` phase 4 calls `@security-champion` and `@nais-agent`, which creates agent-to-skill dependencies.
4. How sophisticated should ambiguity scoring be? OMX's deep-interview scores weighted dimensions mathematically. Overkill for Nav, or the thing that stops developers from skipping the interview?
5. What is the scope of the my-copilot install page? Only generate a PR, or also preview pack contents and show adoption metrics per team?

---

## Appendix A: Nav architecture patterns

These are concrete patterns observed across real navikt repositories. This is the reference data the planning skills will bundle, the institutional knowledge that makes Nav's skills impossible to replicate generically.

Repos analyzed: `dp-behandling`, `tiltakspenger-saksbehandling-api`, `helse-spesialist`, `dinesykmeldte-backend`, `familie-ba-sak`, `familie-tilbake`, `sykepengesoknad-frontend`, `pensjonskalkulator-frontend`, `nav-dekoratoren`, `amt-deltakelser`, `toi-rapids-and-rivers`, `arbeidsoppfolging-adr` and others.

---

### A.1 Auth decision tree

```
WHO CALLS YOUR SERVICE?
│
├─ Citizens (BankID/MinID login)
│  → ID-porten + Wonderwall sidecar
│  → Optional: TokenX for downstream calls on behalf of user
│  Nais: idporten.enabled: true, idporten.sidecar.enabled: true
│
├─ Internal Nav services (with user context)
│  → TokenX (on-behalf-of token exchange)
│  Nais: tokenx.enabled: true
│
├─ Internal Nav services (no user context, batch or cron)
│  → Azure AD / Entra ID (client_credentials)
│  Nais: azure.application.enabled: true
│
└─ External partners / government APIs
   → Maskinporten (JWT bearer grant)
   Nais: maskinporten.enabled: true, maskinporten.scopes: [...]
```

The first three branches validate with `@navikt/oasis` on Node.js or `token-support` on the JVM. Maskinporten uses `token-support`.

| Language    | Library                                    | Repo                 |
| ----------- | ------------------------------------------ | -------------------- |
| Node.js     | `@navikt/oasis`                            | navikt/oasis         |
| Spring Boot | `no.nav.security:token-validation-spring`  | navikt/token-support |
| Ktor        | `no.nav.security:token-validation-ktor-v3` | navikt/token-support |

Two auth mistakes worth calling out here, beyond the ones in A.8: Node.js apps forget `idporten.sidecar.enabled: true`, and services reuse one token across several downstream calls instead of doing a per-target OBO exchange.

---

### A.2 Nais manifest patterns

Resource sizing, taken from real manifests:

| Service type         | CPU request | Memory request | Memory limit | Replicas       |
| -------------------- | ----------- | -------------- | ------------ | -------------- |
| Rapids listener      | 12m         | 360Mi          | 512Mi        | min: 2         |
| Standard web service | 25m         | 1024Mi         | 1024Mi       | min: 2, max: 4 |
| Frontend (Next.js)   | 50m         | 256Mi          | 512Mi        | min: 2, max: 5 |

Ingress follows a fixed shape. Dev internal is `https://{app}.intern.dev.nav.no`, prod internal is `https://{app}.intern.nav.no`, and anything public is `https://{app}.nav.no`.

Environment variables:

```yaml
env:
  - name: JDK_JAVA_OPTIONS
    value: -XX:+UseParallelGC -XX:ActiveProcessorCount=4
  - name: KAFKA_RAPID_TOPIC
    value: team{navn}.rapid.v1
  - name: KAFKA_CONSUMER_GROUP_ID
    value: {app-name}-v1
```

Observability, always enabled:

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

accessPolicy:

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

### A.3 Kotlin/Ktor application patterns

Bootstrapping comes in two shapes. Event-driven services use RapidApplication with Ktor inside it:

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

API services embed Ktor directly and run background jobs beside it:

```kotlin
fun main() {
    val server = embeddedServer(Netty, port = 8080) {
        ktorSetup(applicationContext)
    }
    server.start(wait = true)
}
```

Configuration uses the Konfig library with cluster detection:

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

Errors go through StatusPages with typed exceptions:

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

The dependency list is remarkably consistent across repos:

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

### A.4 Database patterns

Tables lean on VARCHAR primary keys and JSONB for anything that changes shape:

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

HikariCP, sized for a container rather than a server:

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

Cloud SQL in Nais differs between dev and prod mainly in tier, availability and audit flags:

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

One database mistake that A.8 does not cover: missing indexes on foreign key columns, which shows up later as slow joins.

---

### A.5 Kafka and Rapids & Rivers patterns

A topic:

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

Naming is `{team}.rapid.v1` for the Rapids bus and `privat-{team}-{domain}` for domain topics.

A river event handler:

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

Bootstrapping the handlers:

```kotlin
fun main() {
    RapidApplication.create(System.getenv()).apply {
        MyEventHandler(this)
        AnotherHandler(this)
    }.start()
}
```

---

### A.6 Frontend patterns (Next.js)

Auth is Wonderwall plus Oasis:

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

The BFF proxy exchanges the ID-porten token for a backend token, then proxies:

```typescript
import { proxyApiRouteRequest } from '@navikt/next-api-proxy'
import { requestOboToken } from '@navikt/oasis'

const tokenX = await requestOboToken(idportenToken, backendClientId)
await proxyApiRouteRequest({ ...opts, bearerToken: tokenX.token })
```

The Nais manifest for a frontend:

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

The same dependencies show up everywhere:

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

Three generations coexist right now. Pages Router is the legacy, App Router is what new projects use, and the newest work is Vite monorepos.

---

### A.7 CI/CD patterns

The standard workflow is one line long:

```
push to main → build + test → docker image → deploy dev → deploy prod
```

`nais/docker-build-push@v0` builds and pushes to GAR, `nais/deploy/actions/deploy@v2` deploys to the cluster. The image is tagged with `github.sha`, so the same image reaches every environment, and environment differences live in `.nais/vars-{env}.yaml`. Dev and prod deploy at the same time, since both depend only on the build and not on each other.

Monorepos add `dorny/paths-filter`, a matrix strategy and one reusable workflow per module.

---

### A.8 Common anti-patterns

| Anti-pattern                                        | Impact                                      | Fix                            |
| --------------------------------------------------- | ------------------------------------------- | ------------------------------ |
| Using Azure `client_credentials` with user context   | Breaks audit trail, no `sub` claim           | Use TokenX OBO                 |
| Not setting `accessPolicy.inbound`                   | Service unreachable, network policy blocks   | List the callers explicitly    |
| Default HikariCP pool size (10)                      | OOM in containers with 512Mi memory          | Reduce to 3 to 5               |
| Changing `POSTGRES_XX` version in Nais               | Data loss, it triggers a new instance        | Follow the upgrade procedure   |
| Forgetting `envVarPrefix` on Cloud SQL               | App can't connect, no env vars injected      | Add `envVarPrefix: DB`         |
| Same path for liveness and readiness                 | Can't tell startup problems from runtime ones | Separate the probes            |
| Outdated FSS rules in accessPolicy                   | Unnecessary access grants after GCP migration | Remove the stale rules         |
| Logging PII (fnr, names)                             | GDPR violation                               | Use `sikkerlogg` for sensitive data |
| Missing `CONCURRENTLY` on large table indexes        | Table locks during migration                 | Use `CREATE INDEX CONCURRENTLY` |

---

### A.9 Shared platform libraries

| Library or operator              | Purpose                            | Used by                                     |
| -------------------------------- | ---------------------------------- | ------------------------------------------- |
| Wonderwall (nais)                | OIDC sidecar for frontends         | All citizen-facing apps                     |
| Tokendings (nais)                | TokenX token exchange service      | All service-to-service with user context    |
| Azurerator (nais)                | Azure AD app registration operator | All apps with Azure AD                      |
| Kafkarator (nais)                | Kafka topic and user management    | All Kafka users                             |
| Naiserator (nais)                | Turns YAML into Kubernetes resources | All Nais apps                             |
| token-support (navikt)           | JVM token validation framework     | All Kotlin/Java backends                    |
| `@navikt/oasis`                  | Node.js token validation and exchange | All Next.js frontends                    |
| rapids-and-rivers (navikt)       | Kafka event bus framework          | Event-driven services                       |
| `@navikt/ds-react` (Aksel)       | Design system components           | All frontends                               |
| `@navikt/nav-dekoratoren-moduler`| Header and footer decorator        | All citizen-facing frontends                |

---

### A.10 ADR practice

Teams keep their own ADR repos with date-based filenames, for example `navikt/arbeidsoppfolging-adr`. What gets written down: major platform migrations such as Arena to GCP, service integration patterns (sync or async), Kafka topic ownership, data ownership changes, and auth mechanism choices.

The structure is stable across teams. Participants and stakeholders, the problem statement (problemstilling), the alternatives with pros and cons, the decision and why, then consequences and follow-ups.

---

## References

- [oh-my-codex](https://github.com/Yeachan-Heo/oh-my-codex), TypeScript and Rust
- [oh-my-claudecode](https://github.com/Yeachan-Heo/oh-my-claudecode), TypeScript
- [oh-my-openagent](https://github.com/code-yeongyu/oh-my-openagent), TypeScript
- [OpenCode](https://github.com/anomalyco/opencode), TypeScript
- [Agent Skills Specification](https://agentskills.io/specification)
- [Nav Architecture Advice Process](https://sikkerhet.nav.no/), internal
- [OMX deep-interview source](https://github.com/Yeachan-Heo/oh-my-codex/blob/main/skills/deep-interview/SKILL.md), 20KB structured interview
- [OMX plan source](https://github.com/Yeachan-Heo/oh-my-codex/blob/main/skills/plan/SKILL.md), 19KB consensus planning
- [nais/doc](https://github.com/nais/doc), official Nais platform documentation
- [navikt/token-support](https://github.com/navikt/token-support), JVM token validation framework
- [navikt/oasis](https://github.com/navikt/oasis), Node.js token validation and exchange
- [navikt/arbeidsoppfolging-adr](https://github.com/navikt/arbeidsoppfolging-adr), example team ADR repo
