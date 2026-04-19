---
name: security-review
description: Bruk før commit, push eller pull request for å sjekke at koden er trygg å merge
license: MIT
metadata:
  domain: auth
  tags: security pre-commit vulnerability-scanning code-review
---

# Security Review Skill

This skill provides pre-commit and pre-PR security checks for Nav applications. Covers secret scanning, vulnerability scanning, and Nav-specific requirements.

For architecture questions, threat modeling, or compliance decisions, use `@security-champion` instead.

## Automated Scans

Run with `run_in_terminal`:

```bash
# Scan repo for known vulnerabilities and secrets
trivy repo .

# Scan Docker image for HIGH/CRITICAL CVEs
trivy image <image-name> --severity HIGH,CRITICAL

# Scan GitHub Actions workflows for insecure patterns
zizmor .github/workflows/

# Quick search for secrets in git history
git log -p --all -S 'password' -- '*.kt' '*.ts' | head -100
git log -p --all -S 'secret' -- '*.kt' '*.ts' | head -100
```

## Parameterized SQL (Never Concatenate)

```kotlin
// ✅ Correct – parameterized query
fun findBruker(fnr: String): Bruker? =
    jdbcTemplate.queryForObject(
        "SELECT * FROM bruker WHERE fnr = ?",
        brukerRowMapper,
        fnr
    )

// ❌ Wrong – SQL injection risk
fun findBrukerUnsafe(fnr: String): Bruker? =
    jdbcTemplate.queryForObject(
        "SELECT * FROM bruker WHERE fnr = '$fnr'",
        brukerRowMapper
    )
```

## No PII in Logs

```kotlin
// ✅ Correct – log correlation ID, not PII
log.info("Behandler sak for bruker", kv("sakId", sak.id), kv("tema", sak.tema))

// ❌ Wrong – never log FNR, name, or other PII
log.info("Behandler sak for bruker ${bruker.fnr}")  // GDPR violation
log.info("Navn: ${bruker.navn}")                      // GDPR violation
```

## Secrets from Environment, Never Hardcoded

```kotlin
// ✅ Correct – read from environment (Nais injects via Secret)
val dbPassword = System.getenv("DB_PASSWORD")
    ?: throw IllegalStateException("DB_PASSWORD mangler")

// ❌ Wrong – hardcoded secret
val dbPassword = "supersecret123"
```

## Network Policy (Nais)

Only expose what must be exposed:

```yaml
spec:
  accessPolicy:
    inbound:
      rules:
        - application: frontend-app      # only explicitly named callers
    outbound:
      rules:
        - application: pdl-api
          namespace: pdl
          cluster: prod-gcp
      external:
        - host: api.external-service.no  # only if strictly necessary
```

## OWASP Top 10 Checks

### A01: Broken Access Control

```kotlin
// ✅ Correct — check that user has access to the resource
@GetMapping("/api/vedtak/{id}")
fun getVedtak(@PathVariable id: UUID): ResponseEntity<VedtakDTO> {
    val bruker = hentInnloggetBruker()
    val vedtak = vedtakService.findById(id)
    if (vedtak.brukerId != bruker.id) {
        return ResponseEntity.status(HttpStatus.FORBIDDEN).build()
    }
    return ResponseEntity.ok(vedtak.toDTO())
}

// ❌ Wrong — no access control (IDOR)
@GetMapping("/api/vedtak/{id}")
fun getVedtak(@PathVariable id: UUID) = vedtakService.findById(id)
```

### A03: Injection

```kotlin
// ✅ Correct — parameterized query
jdbcTemplate.query("SELECT * FROM bruker WHERE fnr = ?", mapper, fnr)

// ❌ Wrong — string concatenation
jdbcTemplate.query("SELECT * FROM bruker WHERE fnr = '$fnr'", mapper)
```

### A05: Security Misconfiguration

```kotlin
// ✅ Correct — CORS only for known domains
@Bean
fun corsFilter() = CorsFilter(CorsConfiguration().apply {
    allowedOrigins = listOf("https://my-app.intern.nav.no")
    allowedMethods = listOf("GET", "POST")
    allowedHeaders = listOf("Authorization", "Content-Type")
})

// ❌ Wrong — open CORS
allowedOrigins = listOf("*")
```

### A07: Cross-Site Scripting (XSS)

```tsx
// ✅ Correct — React escapes automatically
<BodyShort>{bruker.navn}</BodyShort>

// ❌ Wrong — raw HTML injection
<div dangerouslySetInnerHTML={{ __html: userInput }} />
```

### A08: Insecure Deserialization

```kotlin
// ✅ Correct — validate input after deserialization
@PostMapping("/api/vedtak")
fun create(@RequestBody @Valid request: CreateVedtakRequest): ResponseEntity<VedtakDTO>

// ✅ Limit Jackson to known types
objectMapper.apply {
    activateDefaultTyping(
        polymorphicTypeValidator,
        ObjectMapper.DefaultTyping.NON_FINAL
    )
}
```

### A09: Logging & Monitoring

```kotlin
// ✅ Correct — structured logging with correlation ID, no PII
log.info("Vedtak opprettet", kv("vedtakId", vedtak.id), kv("sakId", sak.id))

// ❌ Wrong — PII in logs
log.info("Vedtak for bruker ${bruker.fnr} opprettet")
```

## File Upload Security

```kotlin
// ✅ Correct — validate file type, size, and magic bytes
fun validateUpload(file: MultipartFile) {
    require(file.size <= 10 * 1024 * 1024) { "File too large (max 10 MB)" }
    require(file.contentType in ALLOWED_TYPES) { "Invalid file type" }

    val bytes = file.bytes.take(8).toByteArray()
    require(verifyMagicBytes(bytes, file.contentType!!)) { "File content does not match type" }
}

private val ALLOWED_TYPES = setOf("application/pdf", "image/png", "image/jpeg")
```

## Dependency Management

```kotlin
// build.gradle.kts — pin versions, use BOM
dependencyManagement {
    imports {
        mavenBom("org.springframework.boot:spring-boot-dependencies:3.4.1")
    }
}

// Check vulnerable dependencies
// ./gradlew dependencyCheckAnalyze
// trivy repo .
```

## Expanded Checklist

- [ ] SQL queries are parameterized (no string concatenation)
- [ ] No PII in logs (fnr, name, address)
- [ ] Secrets only from environment/secrets
- [ ] Nais accessPolicy is explicit (no open inbound)
- [ ] CORS is restricted to known domains
- [ ] Input is validated and sanitized
- [ ] Access control checks ownership (not just auth)
- [ ] File upload validates type, size, and content
- [ ] Dependencies are up to date and vulnerability-scanned
- [ ] No `dangerouslySetInnerHTML` without sanitization

## Dependency Management

```bash
# Kotlin – check for outdated/vulnerable dependencies
./gradlew dependencyUpdates
./gradlew dependencyCheckAnalyze   # OWASP check

# Node/TypeScript
npm audit
npm audit fix
```

## Security Checklist

- [ ] No secrets, tokens, or API keys hardcoded in source
- [ ] No PII (FNR, name, address) in log statements
- [ ] All SQL queries use parameterized statements
- [ ] Nais `accessPolicy` limits inbound/outbound to only what is needed
- [ ] Token validation on all protected endpoints (see `@security-champion`)
- [ ] M2M tokens validate `azp` against `AZURE_APP_PRE_AUTHORIZED_APPS`
- [ ] Auth code matches `.nais/` accessPolicy inbound rules (no dead code or missing rules)
- [ ] `trivy repo .` passes without HIGH/CRITICAL findings
- [ ] `zizmor` passes on all GitHub Actions workflows
- [ ] Git history clean of committed secrets (`git log` scan above)
- [ ] HTTPS enforced – no plain HTTP calls to external services
- [ ] Dependencies up to date (`dependencyUpdates` / `npm audit`)

## Related

| Resource | Use For |
|----------|---------|
| `@security-champion` | Threat modeling, compliance questions, Nav security architecture |
| `@auth-agent` | JWT validation, TokenX, ID-porten, Maskinporten |
| `@nais-agent` | Nais manifest, accessPolicy, secrets setup |
| [sikkerhet.nav.no](https://sikkerhet.nav.no) | Nav Golden Path, authoritative security guidance |
