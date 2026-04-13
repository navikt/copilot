---
name: threat-model
description: STRIDE-A trusselmodellering for Nais-mikrotjenester — dataflyt, tillitsgrenser og risikovurdering
---

# Threat Model — STRIDE-A Analysis

Systematic threat identification for NAIS microservices using the STRIDE-A methodology. Produces a data flow diagram, structured threats table, prioritized mitigations, and residual risk summary.

## When to Use

- Before launching a new service
- Major architecture changes (new data stores, auth mechanism changes)
- New data flows (especially involving PII)
- External integrations (partner APIs, third-party services)
- API exposure (new public or internal endpoints)
- Regulatory or compliance reviews
- Post-incident analysis to update existing threat models

## Step 1: Define Scope

Start by answering these questions to establish the threat model boundary:

### Service Identity

- **What does the service do?** (one sentence — e.g., "Processes dagpenger applications")
- **What team owns it?**
- **What cluster and namespace does it run in?** (dev-gcp, prod-gcp)

### Data Classification

- **What data does the service process?**
- **PII classification level:**
  - **Strengt fortrolig** — FNR, health data, criminal records
  - **Fortrolig** — name, address, phone, email
  - **Intern** — case IDs, team metadata
  - **Åpen** — public statistics, documentation

### Consumers and Dependencies

- **Who consumes this service?** (end users, other services, external partners)
- **What does this service depend on?** (databases, Kafka topics, upstream APIs)
- **What auth mechanisms are in play?** (ID-porten, Azure AD, TokenX, Maskinporten)

### Deployment Context

- **Nais cluster:** dev-gcp / prod-gcp
- **Ingress type:** intern.nav.no (internal) / nav.no (public) / none
- **Has egress to external services?** Which ones?

## Step 2: Data Flow Diagram (DFD)

Map the system using these element types:

### Element Types

| Symbol | Element | Example |
|--------|---------|---------|
| `[External Entity]` | User or external system | `[Citizen Browser]`, `[Partner API]` |
| `(Process)` | Your service or component | `(dp-soknad)`, `(dp-behandling)` |
| `{Data Store}` | Database, topic, bucket | `{PostgreSQL}`, `{kafka: dp.soknad.v1}`, `{GCS Bucket}` |
| `-->` | Data flow | `[User] --> (API)` |
| `== boundary ==` | Trust boundary | `== Internet/Ingress ==` |

### Example DFD

```
[Citizen Browser]
    |
    | HTTPS (ID-porten login)
    |
== Internet → Ingress (Wonderwall) ===========================
    |
    | Authorization header (JWT)
    |
(dp-soknad-frontend)
    |
    | TokenX token exchange
    |
== Frontend → Backend (TokenX validated) =====================
    |
    | REST/JSON + Bearer token
    |
(dp-soknad-api)
    |
    |--- REST (Azure AD M2M) ---> (dp-behandling)
    |
    |--- Kafka produce ---------> {kafka: dp.soknad.v1}
    |                                  |
    |                                  | Kafka consume
    |                                  v
    |                             (dp-mottak)
    |
    |--- SQL (Nais credentials) -> {PostgreSQL: dp-soknad-db}
    |
    |--- HTTPS (egress) --------> [External: Altinn API]
    |
== Application → Database (mTLS, connection pooling) =========
== Application → Kafka (mTLS, schema registry) ===============
== Application → External (egress policy, HTTPS) =============
```

### Nav-Specific Trust Boundaries

Identify these trust boundaries in every Nav threat model:

| Boundary | Transition | Security Mechanism |
|----------|-----------|-------------------|
| Internet → Ingress | External user to Nais | Wonderwall + ID-porten / Azure AD |
| Ingress → Application | Sidecar to app container | Token validation (JWT claims) |
| Application → Application | Service-to-service | TokenX token exchange / Azure AD M2M |
| Application → Kafka | App to message broker | mTLS (Nais-managed certs), schema validation |
| Application → Database | App to PostgreSQL | Nais-injected credentials, connection pooling |
| Application → External API | App to outside Nais | Egress policy, mutual TLS, API keys |
| GCP → On-prem | Cloud to legacy systems | NAV VPN / Private Service Connect |

## Step 3: STRIDE-A per Element

Analyze each element and data flow against all seven threat categories.

### Threat Categories

#### S — Spoofing (Identity Forgery)

Can an attacker impersonate a legitimate user or service?

**Nav-specific threats:**
- Stolen or leaked JWT tokens used to access APIs
- Missing `azp` (authorized party) validation on M2M tokens
- ID-porten session fixation or token replay
- Forged `sub` claim in test environments leaking to prod
- Missing `iss` and `aud` validation

**Detection patterns:**

```kotlin
// ✅ Correct — validate azp against pre-authorized apps
fun validateAzp(token: JWTClaimsSet) {
    val azp = token.getStringClaim("azp")
    val preAuthorized = System.getenv("AZURE_APP_PRE_AUTHORIZED_APPS")
    require(azp in parsePreAuthorizedApps(preAuthorized)) {
        "Unauthorized client: $azp"
    }
}

// ❌ Vulnerable — only checks signature, not authorized party
fun validateToken(token: JWTClaimsSet) {
    require(token.expirationTime.after(Date())) { "Token expired" }
    // Missing: azp, iss, aud validation
}
```

#### T — Tampering (Data Modification)

Can an attacker modify data in transit or at rest?

**Nav-specific threats:**
- Unsigned Kafka messages allowing message injection
- Unvalidated request bodies (missing schema validation)
- Missing HMAC on webhook payloads
- SQL injection through unparameterized queries
- Tampered idempotency keys causing duplicate processing

**Detection patterns:**

```kotlin
// ✅ Correct — validate and sanitize input
data class SoknadRequest(
    @field:Pattern(regexp = "^[0-9]{11}$") val fnr: String,
    @field:Size(max = 500) val beskrivelse: String,
    @field:NotNull val soknadsdato: LocalDate,
)

// ❌ Vulnerable — raw Map, no validation
@PostMapping("/api/soknad")
fun create(@RequestBody body: Map<String, Any>): ResponseEntity<*> {
    repository.save(body) // no validation, no type safety
}
```

#### R — Repudiation (Deniability)

Can an actor deny performing an action?

**Nav-specific threats:**
- Missing audit logs for vedtak (legally required)
- No correlation IDs across service calls (cannot trace actions)
- No user action trails for saksbehandling
- Overwritten audit entries in mutable logs
- Missing timestamps or actor identity in log entries

**Detection patterns:**

```kotlin
// ✅ Correct — structured audit log with actor, action, resource
logger.info(
    "Vedtak fattet",
    kv("action", "vedtak.opprettet"),
    kv("actor", saksbehandler.navIdent),
    kv("vedtakId", vedtak.id),
    kv("sakId", sak.id),
    kv("correlationId", MDC.get("x-correlation-id")),
    // Never log PII — fnr, name, address
)

// ❌ Insufficient — no actor, no correlation, PII leaked
logger.info("Vedtak opprettet for bruker ${bruker.fnr}")
```

#### I — Information Disclosure (Data Leaks)

Can an attacker access data they should not see?

**Nav-specific threats:**
- PII in logs (FNR, name, address) — GDPR violation
- Overly broad API responses returning more fields than needed
- Kafka topic access too permissive (team-wide instead of app-specific)
- Stack traces in error responses exposing internal details
- PII in Prometheus metric labels
- Unencrypted data in GCS buckets

**Detection patterns:**

```kotlin
// ✅ Correct — return only what the consumer needs
data class VedtakResponse(
    val vedtakId: UUID,
    val status: String,
    val dato: LocalDate,
    // No FNR, no internal IDs, no sensitive details
)

// ❌ Vulnerable — returns entire entity including PII
@GetMapping("/api/vedtak/{id}")
fun getVedtak(@PathVariable id: UUID) = vedtakRepository.findById(id)
```

```yaml
# ✅ Correct — app-specific Kafka ACL
spec:
  kafka:
    pool: nav-prod
    streams: true
    topics:
      - topic: dp.soknad.v1
        access: readwrite  # only this app

# ❌ Vulnerable — overly broad topic access
```

#### D — Denial of Service (Availability)

Can an attacker degrade or disable the service?

**Nav-specific threats:**
- Missing rate limiting on public-facing endpoints
- No circuit breakers for downstream service calls
- Unbounded database queries (missing LIMIT/pagination)
- Kafka consumer lag causing cascading delays
- Large payload attacks (unbounded request body size)
- Resource exhaustion from missing Nais resource limits

**Detection patterns:**

```yaml
# ✅ Correct — Nais resource limits and liveness probes
spec:
  resources:
    limits:
      memory: 512Mi
    requests:
      cpu: 50m
      memory: 256Mi
  liveness:
    path: /isalive
    initialDelay: 10
    timeout: 1
    periodSeconds: 5
  readiness:
    path: /isready
    initialDelay: 10
    timeout: 1
```

```kotlin
// ✅ Correct — bounded query with pagination
fun findByIdent(ident: String, page: Int, size: Int = 50): List<Vedtak> {
    require(size <= 100) { "Page size too large" }
    return jdbcTemplate.query(
        "SELECT * FROM vedtak WHERE ident = ? ORDER BY dato DESC LIMIT ? OFFSET ?",
        vedtakMapper, ident, size, page * size
    )
}

// ❌ Vulnerable — unbounded query
fun findByIdent(ident: String) = jdbcTemplate.query(
    "SELECT * FROM vedtak WHERE ident = ?", vedtakMapper, ident
)
```

#### E — Elevation of Privilege (Unauthorized Access)

Can an attacker gain access they should not have?

**Nav-specific threats:**
- IDOR — accessing another user's vedtak by guessing ID
- Missing resource-level access checks (only checks authentication, not authorization)
- Admin/saksbehandler endpoints without RBAC
- Horizontal privilege escalation between NAV offices
- Service account with overly broad GCP IAM roles

**Detection patterns:**

```kotlin
// ✅ Correct — resource-level ownership check
@GetMapping("/api/vedtak/{id}")
fun getVedtak(@PathVariable id: UUID): ResponseEntity<VedtakDTO> {
    val bruker = hentInnloggetBruker()
    val vedtak = vedtakService.findById(id)
        ?: return ResponseEntity.notFound().build()
    if (vedtak.brukerId != bruker.id) {
        return ResponseEntity.status(HttpStatus.FORBIDDEN).build()
    }
    return ResponseEntity.ok(vedtak.toDTO())
}

// ❌ Vulnerable — IDOR, no ownership check
@GetMapping("/api/vedtak/{id}")
fun getVedtak(@PathVariable id: UUID) =
    ResponseEntity.ok(vedtakService.findById(id))
```

#### A — Abuse (Business Logic Exploitation)

Can an attacker misuse legitimate functionality?

**Nav-specific threats:**
- Duplicate søknad submissions (missing idempotency)
- Bypassing validation flows by calling downstream APIs directly
- Automated scraping of public-facing APIs
- Manipulating sequential workflow steps (skipping required stages)
- Mass data harvesting through enumeration attacks

**Detection patterns:**

```kotlin
// ✅ Correct — idempotency key prevents duplicates
@PostMapping("/api/soknad")
fun submitSoknad(
    @RequestHeader("Idempotency-Key") idempotencyKey: String,
    @RequestBody request: SoknadRequest,
): ResponseEntity<SoknadResponse> {
    val existing = soknadService.findByIdempotencyKey(idempotencyKey)
    if (existing != null) {
        return ResponseEntity.ok(existing.toResponse())
    }
    val soknad = soknadService.create(request, idempotencyKey)
    return ResponseEntity.status(HttpStatus.CREATED).body(soknad.toResponse())
}

// ❌ Vulnerable — no idempotency, allows duplicate submissions
@PostMapping("/api/soknad")
fun submitSoknad(@RequestBody request: SoknadRequest) =
    ResponseEntity.ok(soknadService.create(request))
```

## Step 4: Risk Assessment

Rate each identified threat using severity levels:

| Severity | Description | Criteria |
|----------|-------------|----------|
| **Critical** | Immediate exploitation risk | PII breach, auth bypass, data corruption at scale |
| **High** | Significant impact if exploited | IDOR, missing access control, unvalidated input on sensitive endpoints |
| **Medium** | Moderate impact, requires conditions | Missing rate limiting, verbose error messages, broad Kafka ACLs |
| **Low** | Minimal impact or unlikely | Missing HSTS headers, informational log leakage |

### Threats Table Template

Document every identified threat in this format:

```markdown
| ID | Element | STRIDE | Threat | Severity | Mitigation | Status |
|----|---------|--------|--------|----------|------------|--------|
| T1 | API Gateway | S | Forged JWT bypasses auth | Critical | Validate `iss`, `aud`, `exp`, `azp` claims | ☐ |
| T2 | dp-soknad-api | T | Unvalidated request body | High | Add `@Valid` + request DTO with constraints | ☐ |
| T3 | Kafka producer | T | Unsigned messages | Medium | Enable schema registry validation | ☐ |
| T4 | dp-soknad-api | R | No audit trail for vedtak | High | Add structured audit logging with actor + correlationId | ☐ |
| T5 | API response | I | PII in error responses | High | Use ProblemDetail, strip stack traces in prod | ☐ |
| T6 | PostgreSQL | I | Overly broad query results | Medium | Return DTOs with only required fields | ☐ |
| T7 | Public endpoint | D | No rate limiting | Medium | Add rate limiter (token bucket, 100 req/min) | ☐ |
| T8 | GET /vedtak/{id} | E | IDOR — no ownership check | Critical | Add resource-level access control | ☐ |
| T9 | POST /soknad | A | Duplicate submissions | Medium | Implement idempotency key pattern | ☐ |
```

**Status legend:** ☐ Open, ☑ Mitigated, ◐ In Progress, ⊘ Accepted Risk

## Step 5: Mitigations

Map threats to concrete Nav platform mitigations. Six areas to cover:

1. **Authentication** — Wonderwall sidecar + JWT claim validation (`iss`, `aud`, `exp`, `azp`)
2. **Authorization** — Resource-level ownership checks using sealed class principals (Borger/Saksbehandler/SystemBruker)
3. **Network (Zero-Trust)** — Nais `accessPolicy` inbound/outbound allow-lists, egress restrictions
4. **Data Protection** — Input validation (Bean Validation / Zod), output DTOs (omit internal fields, PII)
5. **Observability** — Structured audit logging without PII, correlation IDs, Nais auto-instrumentation
6. **Resilience** — Circuit breakers, rate limiting, HTTP client timeouts, Kafka consumer health

See [references/nav-mitigations.md](references/nav-mitigations.md) for code examples of each mitigation area.

## Output Format

The completed threat model should include these four deliverables:

### 1. Data Flow Diagram

Text-based DFD showing all elements, data flows, and trust boundaries (see Step 2).

### 2. Threats Table

Complete table with all identified threats across STRIDE-A categories (see Step 4).

### 3. Priority Mitigations

Ordered list of mitigations, grouped by priority:

```markdown
### P0 — Fix Immediately
- [ ] T1: Validate JWT claims (iss, aud, azp) on all protected endpoints
- [ ] T8: Add resource-level ownership check on GET /vedtak/{id}

### P1 — Fix Before Launch
- [ ] T2: Add request validation DTOs with Bean Validation
- [ ] T4: Implement structured audit logging for vedtak operations
- [ ] T5: Strip stack traces from error responses in prod

### P2 — Fix Soon
- [ ] T7: Add rate limiting on public endpoints
- [ ] T9: Implement idempotency key pattern for POST /soknad
- [ ] T3: Enable Kafka schema registry validation
```

### 4. Residual Risk Summary

Document risks that are accepted, transferred, or cannot be fully mitigated:

```markdown
| Risk | Severity | Rationale | Owner | Review Date |
|------|----------|-----------|-------|-------------|
| Kafka message replay | Low | mTLS + consumer idempotency makes replay difficult | Team Dagpenger | 2025-Q3 |
| GCS bucket misconfiguration | Medium | Nais manages IAM; manual audit quarterly | Platform team | 2025-Q2 |
```

## Related

| Resource | Use For |
|----------|---------|
| `@security-champion-agent` | Security architecture, compliance, Nav security culture |
| `security-review` skill | Pre-commit scanning (trivy, zizmor, secrets) |
| `@auth-agent` | JWT validation, TokenX, ID-porten implementation |
| `@nais-agent` | accessPolicy, network policy, secrets management |
| `nav-architecture-review` skill | Architecture Decision Records with security perspective |
| [sikkerhet.nav.no](https://sikkerhet.nav.no) | Nav Golden Path, authoritative security guidance |

## Boundaries

### ✅ Always

- Cover all seven STRIDE-A categories for every element
- Include Nav-specific trust boundaries in the DFD
- Output a structured threats table with severity and mitigation
- Classify data by PII sensitivity level
- Produce actionable, prioritized mitigations

### ⚠️ Ask First

- Modifying existing threat models created by other teams
- Changing risk ratings on previously accepted risks
- Recommending architecture changes beyond security scope

### 🚫 Never

- Skip data flow analysis — always draw the DFD first
- Ignore PII classification — every data element must be classified
- Approve a threat model without mitigations for High/Critical threats
- Log or include PII (FNR, names) in threat model examples
- Assume network trust — Nais is zero-trust by default
