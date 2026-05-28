---
name: security-owasp
description: OWASP Top 10:2025 kodenivå-mønstre for Kotlin, Go, Java og Node.js — tilgangskontroll, forsyningskjede, injeksjon og feilhåndtering
license: MIT
metadata:
  domain: auth
  tags: security owasp kotlin go java nodejs nais supply-chain
---

# OWASP Top 10:2025 — Code-Level Security

Tactical security patterns for Kotlin, Go, Java, and Node.js on NAIS, aligned with the **2025 OWASP Top 10**.

Complements `@security-champion-agent` (architecture-level threat modeling) and the `security-review` skill (scanning tools).

> Full code examples for each category: see `examples.md` in this skill directory.

## A01: Broken Access Control (incl. SSRF)

```kotlin
// ❌ IDOR — trusts user-supplied ID without ownership check
get("/api/vedtak/{id}") {
    val vedtak = vedtakRepository.findById(call.parameters["id"]!!.toLong())
    call.respond(vedtak)
}

// ✅ Verify ownership before returning resource
get("/api/vedtak/{id}") {
    val bruker = call.hentBruker()
    val vedtak = vedtakRepository.findById(call.parameters["id"]!!.toLong())
        ?: return@get call.respond(HttpStatusCode.NotFound)
    if (vedtak.brukerId != bruker.id) return@get call.respond(HttpStatusCode.Forbidden)
    call.respond(vedtak.toDTO())
}
```

```go
// ✅ SSRF prevention — validate outbound URL against allowlist
func fetchExternal(targetURL string) error {
    parsed, err := url.Parse(targetURL)
    if err != nil { return err }
    if !isAllowedHost(parsed.Host) { return fmt.Errorf("host not allowed: %s", parsed.Host) }
    // proceed with request
}
```

- Deny by default — require explicit grants, not explicit denials
- Resource-level checks — not just "is authenticated" but "owns this resource"
- M2M tokens — validate `azp` claim against `AZURE_APP_PRE_AUTHORIZED_APPS`
- SSRF — validate outbound URLs; use Nais `accessPolicy.outbound` as defense-in-depth

## A02: Security Misconfiguration

```kotlin
// ❌ Open CORS
install(CORS) { anyHost() }

// ✅ Restrict to known origins
install(CORS) { allowHost("my-app.intern.nav.no", schemes = listOf("https")) }
```

```go
// ❌ Debug endpoint exposed on public ingress
mux.HandleFunc("/debug/pprof/", pprof.Index)

// ✅ Debug endpoints on separate internal-only port (Nais handles this)
internalMux := http.NewServeMux()
internalMux.HandleFunc("/debug/pprof/", pprof.Index)
go http.ListenAndServe(":9090", internalMux) // not exposed via ingress
```

- CORS restricted to known origins — never `*` or `anyHost()`
- Debug/admin endpoints not on public ingress
- Error responses sanitized — no stack traces, SQL errors, or file paths to client
- Default-deny Nais `accessPolicy` — explicit inbound/outbound only

## A03: Software Supply Chain Failures (NEW in 2025)

```go
// go.sum provides integrity verification — always commit it
// Use govulncheck for known vulnerabilities
// $ govulncheck ./...

// ✅ Pin dependencies to exact versions in go.mod
require (
    golang.org/x/crypto v0.31.0
    github.com/jackc/pgx/v5 v5.7.2
)
```

```kotlin
// build.gradle.kts — use dependency locking and BOM
dependencyLocking { lockAllConfigurations() }
dependencyManagement {
    imports { mavenBom("org.springframework.boot:spring-boot-dependencies:3.4.1") }
}
// Run: ./gradlew dependencies --write-locks
```

```yaml
# ✅ GitHub Actions — pin to full SHA, never @main or floating tags
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
# ❌ Vulnerable to supply chain attack
- uses: actions/checkout@main
```

- Pin all dependencies to exact versions; use lockfiles
- Scan dependencies: `govulncheck ./...`, `trivy repo .`, `./gradlew dependencyCheckAnalyze`
- GitHub Actions pinned to full commit SHA (not tags)
- Generate SBOM for production artifacts when possible
- Prefer well-maintained, first-party packages

## A04: Cryptographic Failures

```go
// ❌ Disabling TLS verification
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    },
}

// ✅ Default TLS config (Go enforces TLS 1.2+ by default)
client := &http.Client{}
```

```kotlin
// ❌ Weak hashing for passwords
val hash = MessageDigest.getInstance("MD5").digest(password.toByteArray())

// ✅ bcrypt for password hashing
val hashed = BCrypt.hashpw(password, BCrypt.gensalt(12))
```

- Passwords: bcrypt (cost ≥ 12) or argon2id — never MD5/SHA-1/SHA-256
- Secrets: always from Nais environment variables or Secret resources — never hardcoded
- TLS 1.2+: never set `InsecureSkipVerify: true`
- Encryption: AES-256-GCM; rotate keys periodically

## A05: Injection

```kotlin
// ❌ SQL injection via string template
session.run(queryOf("SELECT * FROM vedtak WHERE status = '$status'").map { ... }.asList)

// ✅ Parameterized query (Kotliquery)
session.run(queryOf("SELECT * FROM vedtak WHERE status = ?", status).map { ... }.asList)
```

```go
// ❌ Command injection via shell
exec.Command("sh", "-c", fmt.Sprintf("process %s", userInput)).Run()

// ✅ Pass arguments directly (no shell interpretation)
exec.Command("process", userInput).Run()
```

- All SQL queries parameterized (`?` / `$1`) — never string concatenation
- No shell execution with user-controlled input
- Validate/sanitize all external input at service boundary

## A09: Security Logging and Alerting Failures

```kotlin
// ✅ Structured logging with correlation ID, no PII
log.info("Vedtak opprettet", kv("vedtakId", vedtak.id), kv("sakId", sak.id))

// ❌ PII in logs — GDPR violation
log.info("Vedtak for bruker ${bruker.fnr}")
```

- No PII in logs (fnr, name, address, tokens)
- Audit trail for sensitive operations (vedtak, utbetaling, tilgang)
- Correlation IDs propagated across services (OpenTelemetry trace context)
- Alerting on anomalous patterns (auth failures, rate spikes)

## A10: Mishandling of Exceptional Conditions (NEW in 2025)

```go
// ❌ Panic leaks to caller, crashes service
func processRequest(data []byte) Result {
    var req Request
    json.Unmarshal(data, &req) // ignores error, req may be zero-value
    return handle(req)
}

// ✅ Handle errors explicitly, fail safely
func processRequest(data []byte) (Result, error) {
    var req Request
    if err := json.Unmarshal(data, &req); err != nil {
        return Result{}, fmt.Errorf("invalid request payload: %w", err)
    }
    return handle(req)
}
```

```kotlin
// ❌ Swallowing exceptions silently
fun process(data: String): Result {
    try { return parse(data) }
    catch (e: Exception) { return Result.empty() } // silent failure, no logging
}

// ✅ Log, wrap, and surface errors appropriately
fun process(data: String): Result {
    return try { parse(data) }
    catch (e: Exception) {
        log.error("Parsing failed", kv("error", e.message))
        throw ServiceException("Could not process input", e)
    }
}
```

- Always handle errors — never ignore returned errors in Go
- Recover from panics at HTTP handler boundaries (middleware)
- Fail securely: deny access by default when state is uncertain
- Sanitize error messages: internal details stay in logs, not in responses
- Centralized error handling via middleware/exception mappers

## Quick Reference Checklist

- [ ] **A01** — Resource-level access checks on every endpoint (not just auth)
- [ ] **A01** — M2M tokens validate `azp` against pre-authorized apps
- [ ] **A01** — Outbound URLs validated; Nais egress policy configured
- [ ] **A02** — CORS restricted to known origins
- [ ] **A02** — Debug endpoints not on public ingress
- [ ] **A02** — Error responses sanitized (no stack traces to client)
- [ ] **A03** — Dependencies pinned to exact versions with lockfiles
- [ ] **A03** — `govulncheck` / `trivy repo .` pass without HIGH/CRITICAL
- [ ] **A03** — GitHub Actions pinned to full SHA
- [ ] **A04** — bcrypt/argon2id for passwords, never MD5/SHA-1
- [ ] **A04** — TLS 1.2+ enforced, no `InsecureSkipVerify`
- [ ] **A04** — Secrets from environment/Nais, never hardcoded
- [ ] **A05** — All SQL queries parameterized (`?` / `$1`)
- [ ] **A05** — No shell execution with user input
- [ ] **A07** — JWT validates `exp`, `iss`, `aud`, and algorithm
- [ ] **A08** — Deserialization into concrete types only
- [ ] **A09** — No PII in logs (fnr, name, address)
- [ ] **A09** — Audit trail for sensitive operations
- [ ] **A10** — All errors handled (no ignored returns in Go)
- [ ] **A10** — Panic recovery in HTTP handlers
- [ ] **A10** — Error messages sanitized before client responses

## Related

| Resource | Use For |
|----------|---------|
| `security-review` skill | Pre-commit scanning (trivy, zizmor, govulncheck) |
| `@security-champion-agent` | Threat modeling, compliance, Nav security architecture |
| `@auth-agent` | JWT validation, TokenX, ID-porten implementation |
| `threat-model` skill | STRIDE-A analysis for new services |
| [OWASP Top 10:2025](https://owasp.org/Top10/2025/) | Official category descriptions |
| [OWASP Go SCP](https://owasp.org/www-project-go-secure-coding-practices/) | Go-specific secure coding guide |
| [OWASP CI/CD Top 10](https://owasp.org/www-project-top-10-ci-cd-security-risks/) | Pipeline security risks |
| [sikkerhet.nav.no](https://sikkerhet.nav.no) | Nav Golden Path |

## Boundaries

### ✅ Always

- Parameterized queries for all SQL
- Resource-level access checks on every data-returning endpoint
- Structured logging without PII
- SHA-pinned GitHub Actions
- Explicit error handling (no ignored errors)
- Dependencies scanned before release

### ⚠️ Ask First

- Custom cryptographic implementations
- Disabling security features for testing
- Changing authentication or authorization logic
- Adding new outbound external hosts

### 🚫 Never

- String-concatenated SQL queries
- `InsecureSkipVerify: true` in production
- PII in log statements (fnr, name, address)
- Wildcard CORS (`*` / `anyHost()`)
- Hardcoded secrets or encryption keys
- Floating tags (`@main`, `@v3`) for GitHub Actions
- Silently swallowing errors without logging
