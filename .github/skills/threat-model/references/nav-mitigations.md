# Nav Platform Mitigations Reference

Map threats to concrete Nav platform mitigations.

## Authentication

```yaml
# Nais manifest — enable Wonderwall sidecar for ID-porten
spec:
  idporten:
    enabled: true
    sidecar:
      enabled: true
      level: Level4   # requires BankID
```

```kotlin
// Validate token claims in application code
fun validateToken(claims: JWTClaimsSet) {
    require(claims.issuer == expectedIssuer) { "Invalid issuer" }
    require(expectedAudience in claims.audience) { "Invalid audience" }
    require(claims.expirationTime.after(Date())) { "Token expired" }
    validateAzp(claims) // for M2M tokens
}
```

## Authorization

```kotlin
// Resource-level access control — always verify ownership
fun authorizeAccess(resource: Vedtak, principal: NavPrincipal): Boolean {
    return when (principal) {
        is Borger -> resource.brukerId == principal.fnr
        is Saksbehandler -> principal.hasAccessToEnhet(resource.enhet)
        is SystemBruker -> principal.appName in allowedApps
    }
}
```

## Network (Zero-Trust)

```yaml
# Nais accessPolicy — explicit allow-list
spec:
  accessPolicy:
    inbound:
      rules:
        - application: dp-soknad-frontend
          namespace: teamdagpenger
    outbound:
      rules:
        - application: dp-behandling
          namespace: teamdagpenger
      external:
        - host: api.altinn.no
```

## Data Protection

```kotlin
// Input validation — never trust external input
data class SoknadRequest(
    @field:Pattern(regexp = "^[0-9]{11}$", message = "Invalid FNR format")
    val fnr: String,

    @field:Size(min = 1, max = 2000, message = "Description too long")
    val beskrivelse: String,

    @field:PastOrPresent(message = "Date cannot be in the future")
    val soknadsdato: LocalDate,
)

// Output encoding — return only what is needed
fun toPublicResponse(vedtak: Vedtak) = VedtakResponse(
    id = vedtak.id,
    status = vedtak.status,
    dato = vedtak.dato,
    // Intentionally omit: fnr, internal IDs, metadata
)
```

## Observability

```kotlin
// Structured audit logging — no PII
fun auditLog(action: String, actor: String, resourceId: String) {
    logger.info(
        "Audit event",
        kv("action", action),
        kv("actor", actor),
        kv("resourceId", resourceId),
        kv("timestamp", Instant.now().toString()),
        kv("correlationId", MDC.get("x-correlation-id")),
    )
}
```

```yaml
# Nais observability configuration
spec:
  observability:
    autoInstrumentation:
      enabled: true
      runtime: java   # or nodejs
    logging:
      destinations:
        - id: loki
```

## Resilience

```kotlin
// Circuit breaker for downstream calls
val circuitBreaker = CircuitBreaker.of("dp-behandling") {
    failureRateThreshold(50f)
    waitDurationInOpenState(Duration.ofSeconds(30))
    slidingWindowSize(10)
}

// Rate limiting
val rateLimiter = RateLimiter.of("public-api") {
    limitForPeriod(100)
    limitRefreshPeriod(Duration.ofMinutes(1))
    timeoutDuration(Duration.ofMillis(500))
}
```

```go
// Go — HTTP client with timeout and retry
client := &http.Client{
    Timeout: 10 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```
