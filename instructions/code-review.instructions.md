---
applyTo: "**"
---

# Code Review Instructions

General review instructions for GitHub Copilot code review. Flag for human reviewer — do not block.

## General Checks

- **Scope**: Reasonable scope, no unrelated changes bundled in
- **Branch name**: Expected prefix `feature/`, `fix/`, `chore/`, `docs/`, `refactor/`

## Security-Critical Changes (always flag)

### Secrets and Credentials

- Hardcoded tokens, passwords, API keys, or credentials
- Environment variables containing `SECRET`, `TOKEN`, `CREDENTIAL`, `PASSWORD`
- Vault or Azure Key Vault references added/changed
- `.env` files committed (should be in `.gitignore`)

### Sensitive Data in Output

- FNR (fødselsnummer), aktørId, or other PII in log statements
- Token values, request bodies, or headers logged
- Validation annotations exposing user input (`${validatedValue}` in `@Pattern`/`@Size`)
- Exception messages containing sensitive context

### Authentication and Authorization

- Token handling (OIDC, SAML, JWT, Azure AD, TokenX, ID-porten)
- Access control annotations (`@BeskyttetRessurs`, `@ProtectedWithClaims`)
- ABAC/policy evaluation logic
- New or modified API endpoints without auth middleware
- CORS configuration changes (especially `allowedOrigins: ["*"]`)

## Infrastructure Changes (flag for review)

### NAIS Configuration

Changes to `nais*.yaml` / `.nais/*.yaml`:

- `accessPolicy` — inbound/outbound rules changed
- `env` — new secrets, scopes, or credentials
- `azure.application` / `tokenx` / `idporten` configuration
- Resource limits (CPU, memory) significantly reduced
- Replica count changes
- New `envFrom` secret references

### GitHub Actions

Changes to `.github/workflows/`:

- Deployment targets or environments changed
- Test steps removed or weakened
- Unpinned action versions (use SHA, not `@main` or `@v3`)
- Secrets used in workflow steps
- `pull_request_target` trigger (security risk)
- New permissions granted to workflow

## Code Quality (flag if concerning)

### Test Coverage

- Test files removed or test assertions deleted
- `@Disabled`, `skipTests`, `skip()` added without explanation
- Coverage-reducing changes to critical paths
- Deployment guards or approval gates weakened

### Integration Points

- New external service clients (REST, gRPC, SOAP)
- Kafka topic, serialization, or error handling changes
- Database migration files (schema changes, data loss risk)
- Retry logic or circuit breaker configuration changes

### Error Handling

- Catch-all exception handlers swallowing errors silently
- Missing error propagation in async/background tasks
- Panic/fatal in library code (should return errors)
