---
name: security-owasp
description: OWASP Top 10:2025 kodenivå-mønstre for Kotlin og Go — injeksjon, tilgangskontroll, kryptografi og logging
license: MIT
metadata:
  domain: auth
  tags: security owasp kotlin go nais
---

# OWASP Top 10:2025 — Code-Level Security

Tactical security patterns for Kotlin and Go code on NAIS. Focuses on **code-level anti-patterns** and fixes.

Complements `@security-champion-agent` (architecture-level threat modeling) and the `security-review` skill (scanning tools).

> Detailed code examples for each category: see `examples.md` in this skill directory.

## Critical Patterns

### A01: Broken Access Control

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

- Deny by default — require explicit grants
- Resource-level checks — not just "is authenticated" but "owns this resource"
- M2M tokens — validate `azp` claim against `AZURE_APP_PRE_AUTHORIZED_APPS`

### A02: Cryptographic Failures

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

- Passwords: bcrypt (cost ≥ 12) or argon2id — never MD5/SHA-1
- Secrets: always from Nais environment variables — never hardcoded
- TLS: never set `InsecureSkipVerify: true`

### A03: Injection

```kotlin
// ❌ SQL injection via string template
session.run(queryOf("SELECT * FROM vedtak WHERE status = '$status'").map { ... }.asList)

// ✅ Parameterized query
session.run(queryOf("SELECT * FROM vedtak WHERE status = ?", status).map { ... }.asList)
```

```go
// ❌ Command injection
exec.Command("sh", "-c", fmt.Sprintf("process %s", userInput)).Run()

// ✅ Pass arguments as separate params (no shell)
exec.Command("process", userInput).Run()
```

### A05: Security Misconfiguration

```kotlin
// ❌ Open CORS
install(CORS) { anyHost() }

// ✅ Restrict to known origins
install(CORS) { allowHost("my-app.intern.nav.no", schemes = listOf("https")) }
```

### A09: Logging — No PII

```kotlin
// ✅ Structured logging with correlation ID, no PII
log.info("Vedtak opprettet", kv("vedtakId", vedtak.id), kv("sakId", sak.id))

// ❌ PII in logs — GDPR violation
log.info("Vedtak for bruker ${bruker.fnr}")
```

### A10: SSRF

```go
// ✅ Validate outbound URL against allowlist
func fetchExternal(targetURL string) error {
    parsed, err := url.Parse(targetURL)
    if err != nil { return err }
    if !isAllowedHost(parsed.Host) { return fmt.Errorf("host not allowed: %s", parsed.Host) }
    // ... proceed with request
}
```

## Quick Reference Checklist

- [ ] **A01** — Resource-level access checks on every endpoint (not just auth)
- [ ] **A01** — M2M tokens validate `azp` against pre-authorized apps
- [ ] **A02** — bcrypt/argon2id for passwords, never MD5/SHA-1
- [ ] **A02** — TLS 1.2+ enforced, no `InsecureSkipVerify`
- [ ] **A02** — Secrets from environment/Nais, never hardcoded
- [ ] **A03** — All SQL queries parameterized (`?` / `$1`)
- [ ] **A03** — No shell execution with user input
- [ ] **A04** — Rate limiting on auth and sensitive endpoints
- [ ] **A05** — CORS restricted to known origins
- [ ] **A05** — Error responses sanitized (no stack traces to client)
- [ ] **A06** — Dependencies scanned (`govulncheck`, `trivy repo .`)
- [ ] **A07** — JWT validates `exp`, `iss`, `aud`, and algorithm
- [ ] **A08** — GitHub Actions pinned to full SHA
- [ ] **A09** — No PII in logs (fnr, name, address)
- [ ] **A09** — Audit trail for sensitive operations
- [ ] **A10** — Outbound URLs validated against allowlist

## Related

| Resource | Use For |
|----------|---------|
| `security-review` skill | Pre-commit scanning (trivy, zizmor, govulncheck) |
| `@security-champion-agent` | Threat modeling, compliance, Nav security architecture |
| `@auth-agent` | JWT validation, TokenX, ID-porten implementation |
| `threat-model` skill | STRIDE-A analysis for new services |
| [sikkerhet.nav.no](https://sikkerhet.nav.no) | Nav Golden Path |

## Boundaries

### ✅ Always

- Parameterized queries for all SQL
- Resource-level access checks on every data-returning endpoint
- Structured logging without PII
- SHA-pinned GitHub Actions

### ⚠️ Ask First

- Custom cryptographic implementations
- Disabling security features for testing
- Changing authentication or authorization logic

### 🚫 Never

- String-concatenated SQL queries
- `InsecureSkipVerify: true` in production
- PII in log statements (fnr, name, address)
- Wildcard CORS (`*` / `anyHost()`)
- Hardcoded secrets or encryption keys
