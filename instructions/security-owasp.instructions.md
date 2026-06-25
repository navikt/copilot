---
applyTo: "**/*.{kt,go,java,ts,tsx}"
---

# Security Essentials

For detailed OWASP Top 10:2025 code-level patterns (Kotlin, Go, Java, Node.js), invoke the `$security-owasp` skill.

## Critical Rules (always apply)

- **Parameterized queries only** — never concatenate user input into SQL/commands
- **No PII in logs** — no FNR, name, address, or tokens in log statements
- **Secrets from environment** — never hardcode tokens, passwords, or keys
- **Verify resource ownership** — not just "is authenticated" but "owns this resource"
- **Validate `azp` for M2M** — check against `AZURE_APP_PRE_AUTHORIZED_APPS`
- **TLS 1.2+** — never set `InsecureSkipVerify: true` or disable certificate validation

For scanning workflows (trivy, zizmor, govulncheck), use the `$security-review` skill.
For architecture-level threat modeling, use `@security-champion`.
