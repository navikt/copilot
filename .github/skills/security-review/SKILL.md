---
name: security-review
description: Use when about to commit, push, or open a pull request and want to check if code is safe to merge
---

# Security Review Skill

> 🔒 **security-review skill active** — running Nav pre-commit/pre-PR security checks.

## When to Use This Skill

ACTIVATE when the user:
- Mentions "commit", "push", "open PR", or "pull request"
- Asks to "review security", "check security", or "security review"
- Asks if code is "safe to merge" or "ready to deploy"
- Asks about secrets, PII, SQL injection, or Nais access policy
- Uses words like "trivy", "zizmor", or "vulnerability scan"

Do NOT activate for: architecture questions, threat modeling, compliance decisions → use `@security-champion` instead.

---

Run security checks before committing or opening a pull request. Covers secret scanning, vulnerability scanning, and Nav-specific requirements.

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
