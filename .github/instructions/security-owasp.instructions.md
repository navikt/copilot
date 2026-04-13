---
applyTo: "**/*.{kt,go}"
---

# OWASP Top 10:2025 — Code-Level Security

Tactical security patterns for Kotlin and Go code on NAIS. This instruction focuses on **code-level anti-patterns** and fixes — it complements `@security-champion-agent` (architecture-level threat modeling) and the `security-review` skill (scanning tools like trivy, zizmor, govulncheck).

> For automated scanning workflows, run the `security-review` skill before commit.

## A01: Broken Access Control

The most critical category. Every endpoint that returns or modifies a resource must verify the caller owns or is authorized for that resource.

### Kotlin (Ktor)

```kotlin
// ❌ IDOR — trusts user-supplied ID without ownership check
get("/api/vedtak/{id}") {
    val id = call.parameters["id"]!!.toLong()
    val vedtak = vedtakRepository.findById(id)
    call.respond(vedtak)
}

// ✅ Verify ownership before returning resource
get("/api/vedtak/{id}") {
    val bruker = call.hentBruker()
    val id = call.parameters["id"]!!.toLong()
    val vedtak = vedtakRepository.findById(id)
        ?: return@get call.respond(HttpStatusCode.NotFound)
    if (vedtak.brukerId != bruker.id) {
        return@get call.respond(HttpStatusCode.Forbidden)
    }
    call.respond(vedtak.toDTO())
}
```

### Go

```go
// ❌ Handler trusts user-supplied tenant ID
func GetResource(w http.ResponseWriter, r *http.Request) {
    tenantID := r.URL.Query().Get("tenant_id")
    resources := db.FindByTenant(r.Context(), tenantID)
    json.NewEncoder(w).Encode(resources)
}

// ✅ Extract tenant from authenticated token, not query params
func GetResource(w http.ResponseWriter, r *http.Request) {
    claims := auth.ClaimsFromContext(r.Context())
    resources, err := db.FindByTenant(r.Context(), claims.TenantID)
    if err != nil {
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    json.NewEncoder(w).Encode(resources)
}
```

### Patterns

- **Deny by default** — require explicit grants, not explicit denials
- **Resource-level checks** — not just "is authenticated" but "owns this resource"
- **M2M tokens** — validate `azp` claim against `AZURE_APP_PRE_AUTHORIZED_APPS`
- **Horizontal privilege** — user A must never access user B's data via predictable IDs

## A02: Cryptographic Failures

### Kotlin

```kotlin
// ❌ Weak hashing — MD5/SHA-1 are broken for passwords
val hash = MessageDigest.getInstance("MD5").digest(password.toByteArray())

// ❌ Hardcoded encryption key
val secretKey = "my-super-secret-key-12345678"

// ✅ bcrypt for password hashing
import org.mindrot.jbcrypt.BCrypt
val hashed = BCrypt.hashpw(password, BCrypt.gensalt(12))
val valid = BCrypt.checkpw(candidatePassword, hashed)

// ✅ Secrets from Nais environment
val encryptionKey = System.getenv("ENCRYPTION_KEY")
    ?: throw IllegalStateException("ENCRYPTION_KEY not configured")
```

### Go

```go
// ❌ MD5 for integrity — trivially broken
import "crypto/md5"
hash := md5.Sum([]byte(data))

// ❌ Disabling TLS verification
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    },
}

// ✅ SHA-256 for integrity checks
import "crypto/sha256"
hash := sha256.Sum256([]byte(data))

// ✅ bcrypt for passwords
import "golang.org/x/crypto/bcrypt"
hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
err = bcrypt.CompareHashAndPassword(hashed, []byte(candidate))

// ✅ Default TLS config (Go enforces TLS 1.2+ by default)
client := &http.Client{}
```

### Patterns

- **Passwords**: bcrypt (cost ≥ 12) or argon2id — never MD5/SHA-1/SHA-256
- **TLS**: 1.2+ enforced — never set `InsecureSkipVerify: true`
- **Secrets**: always from Nais environment variables or Secret resources — never hardcoded
- **Encryption keys**: use AES-256-GCM, rotate keys periodically

## A03: Injection

### Kotlin (Kotliquery / JdbcTemplate)

```kotlin
// ❌ SQL injection via string template
fun findByStatus(status: String): List<Vedtak> =
    using(sessionOf(dataSource)) { session ->
        session.run(
            queryOf("SELECT * FROM vedtak WHERE status = '$status'")
                .map { row -> row.toVedtak() }.asList
        )
    }

// ❌ Command injection via ProcessBuilder
fun convert(filename: String): String {
    val process = ProcessBuilder("sh", "-c", "convert $filename output.pdf").start()
    return process.inputStream.bufferedReader().readText()
}

// ✅ Parameterized query — Kotliquery
fun findByStatus(status: String): List<Vedtak> =
    using(sessionOf(dataSource)) { session ->
        session.run(
            queryOf("SELECT * FROM vedtak WHERE status = ?", status)
                .map { row -> row.toVedtak() }.asList
        )
    }

// ✅ Avoid shell — pass arguments directly
fun convert(filename: String): String {
    require(filename.matches(Regex("[a-zA-Z0-9._-]+"))) { "Invalid filename" }
    val process = ProcessBuilder("convert", filename, "output.pdf").start()
    return process.inputStream.bufferedReader().readText()
}
```

### Go (pgx)

```go
// ❌ SQL injection via fmt.Sprintf
func FindByStatus(ctx context.Context, pool *pgxpool.Pool, status string) ([]Vedtak, error) {
    query := fmt.Sprintf("SELECT * FROM vedtak WHERE status = '%s'", status)
    rows, err := pool.Query(ctx, query)
    // ...
}

// ❌ Template injection — using text/template with user input as template
tmpl, _ := template.New("t").Parse(userInput)

// ✅ Parameterized query — pgx
func FindByStatus(ctx context.Context, pool *pgxpool.Pool, status string) ([]Vedtak, error) {
    rows, err := pool.Query(ctx, "SELECT * FROM vedtak WHERE status = $1", status)
    // ...
}

// ✅ Use html/template with user input as data, not template
tmpl, _ := template.New("t").Parse("Hello, {{.Name}}")
tmpl.Execute(w, map[string]string{"Name": userInput})
```

### Patterns

- **SQL**: always parameterized — `?` (Kotlin), `$1` (pgx) — never string concatenation
- **Shell**: avoid `sh -c` — pass arguments as separate strings to `ProcessBuilder` / `exec.Command`
- **Templates**: user input is **data**, never part of the template itself

## A04: Insecure Design

### Kotlin (Ktor)

```kotlin
// ❌ No rate limiting on login endpoint
post("/api/login") {
    val credentials = call.receive<LoginRequest>()
    val user = authService.authenticate(credentials)
    call.respond(user.toToken())
}

// ❌ Negative amount allowed — business logic flaw
fun transfer(from: Account, to: Account, amount: BigDecimal) {
    from.balance -= amount
    to.balance += amount
}

// ✅ Rate limiting middleware
install(RateLimit) {
    register(RateLimitName("auth")) {
        rateLimiter(limit = 10, refillPeriod = 60.seconds)
        requestKey { call.request.origin.remoteAddress }
    }
}
authenticate {
    rateLimit(RateLimitName("auth")) {
        post("/api/login") { /* ... */ }
    }
}

// ✅ Validate business rules at boundaries
fun transfer(from: Account, to: Account, amount: BigDecimal) {
    require(amount > BigDecimal.ZERO) { "Amount must be positive" }
    require(from.balance >= amount) { "Insufficient funds" }
    from.balance -= amount
    to.balance += amount
}
```

### Go

```go
// ❌ No rate limiting
http.HandleFunc("/api/login", handleLogin)

// ✅ Rate limiting with middleware
import "golang.org/x/time/rate"

func rateLimitMiddleware(limiter *rate.Limiter, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow() {
            http.Error(w, "too many requests", http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}

// ✅ Validate business rules
func Transfer(from, to *Account, amount int64) error {
    if amount <= 0 {
        return fmt.Errorf("amount must be positive: %d", amount)
    }
    if from.Balance < amount {
        return fmt.Errorf("insufficient funds")
    }
    from.Balance -= amount
    to.Balance += amount
    return nil
}
```

### Patterns

- **Rate limiting** on authentication, password reset, and OTP endpoints
- **Input validation** at service boundaries — never trust client-side validation alone
- **Idempotency keys** for financial operations to prevent double-processing
- **Race conditions** — use database-level locking for balance checks, not in-memory checks

## A05: Security Misconfiguration

### Kotlin

```kotlin
// ❌ Wildcard CORS — allows any origin
install(CORS) {
    anyHost()
}

// ❌ Ktor development mode in production
// application.conf: ktor.development = true

// ✅ Restrictive CORS — named origins only
install(CORS) {
    allowHost("my-app.intern.nav.no", schemes = listOf("https"))
    allowHeader(HttpHeaders.Authorization)
    allowHeader(HttpHeaders.ContentType)
}

// ✅ Production-safe error responses
install(StatusPages) {
    exception<Throwable> { call, cause ->
        logger.error(cause) { "Unhandled exception" }
        call.respond(
            HttpStatusCode.InternalServerError,
            mapOf("error" to "Internal server error")
        )
    }
}
```

### Go

```go
// ❌ pprof exposed in production
import _ "net/http/pprof"
http.ListenAndServe(":8080", nil)

// ❌ Verbose error responses leaking internals
http.Error(w, fmt.Sprintf("database error: %v", err), 500)

// ✅ pprof on separate internal port (not exposed via ingress)
go func() {
    debugMux := http.NewServeMux()
    debugMux.HandleFunc("/debug/pprof/", pprof.Index)
    http.ListenAndServe("localhost:6060", debugMux)
}()

// ✅ Sanitized error responses
if err != nil {
    slog.Error("database query failed", "error", err)
    http.Error(w, "internal server error", http.StatusInternalServerError)
    return
}
```

### Patterns

- **CORS**: explicit origins only — never `*` or `anyHost()`
- **Debug endpoints**: pprof/actuator on internal port, never on public ingress
- **Error messages**: log details server-side, return generic messages to clients
- **Production profiles**: disable development mode, verbose errors, debug features

## A06: Vulnerable and Outdated Components

### Kotlin

```bash
# Gradle dependency vulnerability check
./gradlew dependencyCheckAnalyze

# Trivy repo scan (covers all languages)
trivy repo .
```

### Go

```bash
# Go vulnerability check
govulncheck ./...

# Verify go.sum integrity
go mod verify

# Trivy repo scan
trivy repo .
```

### Patterns

- **Automated updates**: Dependabot or Renovate for dependency PRs
- **SBOM**: generate in CI with `trivy repo . --format cyclonedx`
- **Pin versions**: lock files (`go.sum`, `gradle.lockfile`) checked into git

> For full scanning workflow, run the `security-review` skill.

## A07: Identification and Authentication Failures

### Kotlin

```kotlin
// ❌ Missing claim validation — accepts any valid JWT
fun validateToken(token: String): Claims {
    return Jwts.parserBuilder()
        .setSigningKey(key)
        .build()
        .parseClaimsJws(token)
        .body
}

// ✅ Strict claim validation — verify issuer, audience, expiry
fun validateToken(token: String): Claims {
    return Jwts.parserBuilder()
        .setSigningKey(key)
        .requireIssuer(expectedIssuer)
        .requireAudience(expectedAudience)
        .build()
        .parseClaimsJws(token)
        .body
        .also { claims ->
            require(claims.expiration.after(Date())) { "Token expired" }
        }
}
```

### Go

```go
// ❌ JWT validation skips critical claims
func ValidateToken(tokenString string) (*Claims, error) {
    token, err := jwt.Parse(tokenString, keyFunc)
    if err != nil {
        return nil, err
    }
    return token.Claims.(*Claims), nil
}

// ✅ Validate issuer, audience, and expiry
func ValidateToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{},
        keyFunc,
        jwt.WithIssuer(expectedIssuer),
        jwt.WithAudience(expectedAudience),
        jwt.WithExpirationRequired(),
        jwt.WithValidMethods([]string{"RS256"}),
    )
    if err != nil {
        return nil, fmt.Errorf("token validation failed: %w", err)
    }
    return token.Claims.(*Claims), nil
}
```

### Patterns

- **Wonderwall** — delegate user-facing auth to Nais sidecar, don't roll your own
- **JWT validation** — always check `exp`, `iss`, `aud`, and signing algorithm
- **Session management** — invalidate sessions on logout and password change
- **M2M tokens** — validate `azp` against pre-authorized apps list

## A08: Software and Data Integrity Failures

### CI/CD Pipeline

```yaml
# ❌ Unpinned action — tag can be compromised
- uses: actions/checkout@v4

# ✅ SHA-pinned with version comment
- uses: actions/checkout@b4ffde65f46336ab88eb53be808477a3936bae11 # v4.1.1
```

### Kotlin

```kotlin
// ❌ Unsafe Jackson polymorphic deserialization — allows arbitrary class instantiation
objectMapper.enableDefaultTyping()

// ✅ Restrict polymorphic types to known classes
objectMapper.activateDefaultTyping(
    BasicPolymorphicTypeValidator.builder()
        .allowIfSubType("no.nav.myapp.dto.")
        .build(),
    ObjectMapper.DefaultTyping.NON_FINAL
)

// ✅ Prefer explicit DTOs — no polymorphism needed
data class VedtakRequest(val type: String, val belop: BigDecimal)
```

### Go

```go
// ❌ Decoding gob from untrusted source — arbitrary types
func HandleUpload(w http.ResponseWriter, r *http.Request) {
    var data interface{}
    gob.NewDecoder(r.Body).Decode(&data)
}

// ✅ Decode into concrete, known types only
func HandleUpload(w http.ResponseWriter, r *http.Request) {
    var req VedtakRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }
}
```

### Patterns

- **GitHub Actions**: pin all actions to full SHA — see `github-actions` instruction
- **Container images**: use signed images from Nav's Chainguard registry
- **Deserialization**: decode into concrete types, never `interface{}` or `Any` from untrusted input
- **SBOM attestation**: attach provenance to container images in CI

## A09: Security Logging and Monitoring Failures

### Kotlin

```kotlin
// ❌ Logging PII — GDPR violation
logger.info("Opprettet vedtak for bruker ${bruker.fnr}, navn: ${bruker.navn}")

// ❌ No audit trail for sensitive operations
fun slettVedtak(id: Long) {
    vedtakRepository.deleteById(id)
}

// ✅ Structured logging without PII — use correlation IDs
import net.logstash.logback.argument.StructuredArguments.kv
logger.info("Vedtak opprettet",
    kv("vedtakId", vedtak.id),
    kv("sakId", sak.id),
    kv("callId", MDC.get("callId"))
)

// ✅ Audit trail for sensitive operations
fun slettVedtak(id: Long, utfortAv: String) {
    logger.info("Vedtak slettet",
        kv("vedtakId", id),
        kv("utfortAv", utfortAv),
        kv("handling", "SLETT_VEDTAK")
    )
    vedtakRepository.deleteById(id)
}
```

### Go

```go
// ❌ Logging PII
slog.Info("created vedtak", "fnr", bruker.Fnr, "name", bruker.Name)

// ❌ No request correlation
slog.Info("processing request")

// ✅ Structured logging without PII
slog.Info("vedtak created",
    "vedtak_id", vedtak.ID,
    "sak_id", sak.ID,
)

// ✅ Request-scoped logger with correlation ID
func withRequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = uuid.NewString()
        }
        ctx := context.WithValue(r.Context(), requestIDKey, requestID)
        logger := slog.With("request_id", requestID)
        ctx = context.WithValue(ctx, loggerKey, logger)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// ✅ Audit trail for sensitive operations
func DeleteVedtak(ctx context.Context, id int64) error {
    logger := loggerFromContext(ctx)
    logger.Info("vedtak deleted",
        "vedtak_id", id,
        "action", "DELETE_VEDTAK",
    )
    return repo.Delete(ctx, id)
}
```

### Patterns

- **Never log PII**: fnr, name, address, phone, email — use opaque IDs (sakId, vedtakId)
- **Correlation IDs**: propagate `X-Request-ID` / `callId` across service boundaries
- **Audit trail**: log who did what, when — for vedtak, utbetaling, access grants
- **Structured logging**: use `kv()` (Kotlin) or `slog` fields (Go) — never string interpolation

## A10: Server-Side Request Forgery (SSRF)

### Kotlin

```kotlin
// ❌ User-controlled URL passed directly to HTTP client
get("/api/fetch") {
    val url = call.request.queryParameters["url"]!!
    val response = httpClient.get(url)
    call.respond(response.bodyAsText())
}

// ✅ Validate against allowlist of known hosts
private val allowedHosts = setOf("api.nav.no", "pdl-api.intern.nav.no")

get("/api/fetch") {
    val url = Url(call.request.queryParameters["url"]!!)
    require(url.host in allowedHosts) { "Host not allowed: ${url.host}" }
    require(url.protocol == URLProtocol.HTTPS) { "HTTPS required" }
    val response = httpClient.get(url)
    call.respond(response.bodyAsText())
}
```

### Go

```go
// ❌ User-provided URL fetched without validation
func ProxyHandler(w http.ResponseWriter, r *http.Request) {
    targetURL := r.URL.Query().Get("url")
    resp, _ := http.Get(targetURL)
    io.Copy(w, resp.Body)
}

// ✅ Validate against allowlist before fetching
var allowedHosts = map[string]bool{
    "api.nav.no":              true,
    "pdl-api.intern.nav.no":   true,
}

func ProxyHandler(w http.ResponseWriter, r *http.Request) {
    targetURL, err := url.Parse(r.URL.Query().Get("url"))
    if err != nil || !allowedHosts[targetURL.Hostname()] {
        http.Error(w, "host not allowed", http.StatusForbidden)
        return
    }
    if targetURL.Scheme != "https" {
        http.Error(w, "HTTPS required", http.StatusBadRequest)
        return
    }
    resp, err := http.Get(targetURL.String())
    if err != nil {
        http.Error(w, "fetch failed", http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()
    io.Copy(w, resp.Body)
}
```

### Patterns

- **URL allowlists** — only permit known, trusted hosts
- **HTTPS enforcement** — reject plain HTTP outbound requests
- **Nais egress policy** — use `accessPolicy.outbound.external` as defense-in-depth
- **Block internal ranges** — reject `127.0.0.1`, `10.x`, `169.254.169.254` (metadata API)

## Quick Reference Checklist

- [ ] **A01** — Resource-level access checks on every endpoint (not just auth)
- [ ] **A01** — M2M tokens validate `azp` against pre-authorized apps
- [ ] **A02** — bcrypt/argon2id for passwords, never MD5/SHA-1
- [ ] **A02** — TLS 1.2+ enforced, no `InsecureSkipVerify`
- [ ] **A02** — Secrets from environment/Nais, never hardcoded
- [ ] **A03** — All SQL queries parameterized (`?` / `$1`)
- [ ] **A03** — No shell execution with user input
- [ ] **A04** — Rate limiting on auth and sensitive endpoints
- [ ] **A04** — Business rules validated server-side
- [ ] **A05** — CORS restricted to known origins
- [ ] **A05** — Debug endpoints not on public ingress
- [ ] **A05** — Error responses sanitized (no stack traces to client)
- [ ] **A06** — Dependencies scanned (`govulncheck`, `trivy repo .`)
- [ ] **A07** — JWT validates `exp`, `iss`, `aud`, and algorithm
- [ ] **A07** — Wonderwall for user-facing authentication
- [ ] **A08** — GitHub Actions pinned to full SHA
- [ ] **A08** — Deserialization into concrete types only
- [ ] **A09** — No PII in logs (fnr, name, address)
- [ ] **A09** — Audit trail for sensitive operations (vedtak, utbetaling)
- [ ] **A09** — Correlation IDs propagated across services
- [ ] **A10** — Outbound URLs validated against allowlist
- [ ] **A10** — Nais egress policy configured for defense-in-depth

## Related

| Resource | Use For |
|----------|---------|
| `security-review` skill | Pre-commit scanning (trivy, zizmor, govulncheck) |
| `@security-champion-agent` | Threat modeling, compliance, Nav security architecture |
| `@auth-agent` | JWT validation, TokenX, ID-porten implementation |
| `threat-model` skill | STRIDE-A analysis for new services |
| [sikkerhet.nav.no](https://sikkerhet.nav.no) | Nav Golden Path, authoritative security guidance |

## Boundaries

### ✅ Always

- Parameterized queries for all SQL
- Resource-level access checks on every data-returning endpoint
- Structured logging without PII
- Validate all external input at service boundaries
- SHA-pinned GitHub Actions

### ⚠️ Ask First

- Custom cryptographic implementations
- Disabling security features for testing
- Adding new outbound external hosts
- Changing authentication or authorization logic

### 🚫 Never

- String-concatenated SQL queries
- `InsecureSkipVerify: true` in production
- PII in log statements (fnr, name, address)
- Wildcard CORS (`*` / `anyHost()`)
- Hardcoded secrets or encryption keys
- `encoding/gob` or polymorphic deserialization from untrusted input
