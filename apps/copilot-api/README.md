# Copilot API

Internal backend API for Nav's GitHub Copilot ecosystem. This service provides authenticated access to GitHub API, BigQuery metrics, and related Copilot functionality.

## Architecture

```
Browser → Wonderwall → my-copilot (BFF) → Texas (OBO) → copilot-api → GitHub/BigQuery
```

**Key principles:**

- **Internal-only API** — only accessible from `my-copilot` within NAIS network
- **Azure AD On-Behalf-Of (OBO)** authentication via Texas sidecar
- **Resource-oriented REST API** with canonical DTOs
- **RFC 7807 Problem Details** for errors
- **Background metrics collection** for `/metrics` endpoint
- **Explicit cache control** with backend-owned TTLs

## API Endpoints

### Public (no auth)

- `GET /health` — Health check
- `GET /ready` — Readiness check
- `GET /metrics` — Prometheus metrics (cached, background-collected)

### Protected (requires Azure AD OBO token)

#### Usage Metrics

- `GET /api/v1/copilot/usage/metrics` — Daily usage metrics
- `GET /api/v1/copilot/usage/summary` — Not implemented yet
- `GET /api/v1/copilot/usage/trends` — Not implemented yet
- `GET /api/v1/copilot/usage/features` — Not implemented yet
- `GET /api/v1/copilot/usage/languages` — Not implemented yet
- `GET /api/v1/copilot/usage/editors` — Not implemented yet
- `GET /api/v1/copilot/usage/models` — Not implemented yet

#### Billing

- `GET /api/v1/copilot/billing` — Enterprise billing overview
- `GET /api/v1/copilot/billing/premium` — Premium request usage

#### Adoption

- `GET /api/v1/copilot/adoption/summary` — Adoption overview
- `GET /api/v1/copilot/adoption/teams` — Team-level adoption
- `GET /api/v1/copilot/adoption/languages` — Language-specific adoption
- `GET /api/v1/copilot/adoption/staleness` — Last activity per repository

#### Customizations

- `GET /api/v1/copilot/customizations/details` — Customization details
- `GET /api/v1/copilot/customizations/usage` — Customization usage

#### Seats

- `GET /api/v1/copilot/seats/{username}` — User seat status
- `POST /api/v1/copilot/seats` — Assign seat to user
- `DELETE /api/v1/copilot/seats/{username}` — Remove user seat
- `GET /api/v1/copilot/saml/{identity}` — Resolve GitHub username from SAML identity
- `GET /api/v1/copilot/repo-contributors` — Repository file contributors

#### MCP

- `GET /api/v1/mcp/servers` — Not implemented yet

## Authentication

API uses **Azure AD On-Behalf-Of (OBO)** tokens obtained via Texas sidecar. The BFF (`my-copilot`) exchanges user tokens for OBO tokens targeting this API.

**Token validation:**

1. **Signature** — verified against Azure AD JWKS
2. **Issuer** — `https://login.microsoftonline.com/{tenant}/v2.0`
3. **Audience** — `AZURE_APP_CLIENT_ID` (supports both string and array `aud` claims)
4. **Authorized Party (azp)** — must match pre-authorized client (my-copilot)
5. **Expiry** — token must not be expired

**User claims extracted:**

- `preferred_username` — Email address
- `NAVident` — Nav employee ID
- `name` — Display name
- `groups` — Azure AD group memberships
- `azp` — Calling application client ID

## Configuration

| Variable | Description | Default |
|---|---|---|
| `PORT` | Server port | `8080` |
| `LOG_LEVEL` | Log level (DEBUG, INFO, WARN, ERROR) | `INFO` |
| `LOGGED_ENDPOINTS` | Comma-separated paths to log | `/api/v1/` |
| `AZURE_APP_CLIENT_ID` | Expected audience in tokens | (injected by NAIS) |
| `AZURE_OPENID_CONFIG_ISSUER` | Expected issuer | (injected by NAIS) |
| `AZURE_OPENID_CONFIG_JWKS_URI` | JWKS endpoint | (injected by NAIS) |
| `AZURE_APP_PRE_AUTHORIZED_APPS` | Allowed client IDs (JSON) | (injected by NAIS) |
| `GITHUB_ORG` | GitHub organization | `navikt` |
| `GITHUB_APP_ID` | GitHub App ID | (secret) |
| `GITHUB_APP_PRIVATE_KEY` | GitHub App private key | (secret) |
| `GITHUB_APP_INSTALLATION_ID` | GitHub App installation ID | (secret) |
| `GCP_TEAM_PROJECT_ID` | GCP project ID | (injected by NAIS) |
| `COPILOT_METRICS_DATASET` | BigQuery metrics dataset | `copilot_metrics` |
| `COPILOT_METRICS_TABLE` | BigQuery metrics table | `usage_metrics` |
| `COPILOT_ADOPTION_DATASET` | BigQuery adoption dataset | `copilot_adoption` |

## Development

```bash
# Install dependencies
cd apps/copilot-api
go mod download

# Run locally (dev mode, no auth)
mise dev

# Run tests
mise test

# Run all checks (fmt, vet, staticcheck, lint, test)
mise check

# Build
mise build
```

## Deployment

Deployed via NAIS to `dev-gcp` and `prod-gcp`:

- **Image:** `ghcr.io/navikt/copilot-api`
- **Access:** Internal-only (my-copilot inbound rule)
- **Observability:** OpenTelemetry auto-instrumentation, Prometheus metrics, Loki logs

## Testing

```bash
# Unit tests
mise test

# With coverage
mise test:coverage

# Lint
mise lint

# All checks
mise check
```

## Security

- **Zero trust** — validates all tokens, doesn't trust BFF blindly
- **Least privilege** — only `my-copilot` can call API (azp validation)
- **Audit logging** — all mutations logged with user identity
- **Secrets rotation** — GitHub App credentials stored in NAIS secrets
- **Rate limiting** — GitHub API calls respect rate limits with exponential backoff

## Cache Strategy

| Data Type | Backend Cache | BFF Cache |
|---|---|---|
| Seat status | 60s TTL + invalidation | Optional |
| Billing | 1h TTL | Yes (1h stale) |
| BigQuery dashboards | 1h TTL | Yes (1h stale) |
| Seat mutations | None | None |
| `/metrics` | Background refresh (5min) | No |

## Error Handling

API returns **RFC 7807 Problem Details** for all errors:

```json
{
  "type": "https://copilot-api.nav.no/errors/unauthorized",
  "title": "Unauthorized",
  "status": 401,
  "detail": "Invalid or expired token"
}
```

Common error types:

- `unauthorized` — Missing or invalid token
- `forbidden` — Valid token but insufficient permissions
- `not_found` — Resource not found
- `invalid_request` — Malformed request
- `github_error` — GitHub API error
- `bigquery_error` — BigQuery error

## Monitoring

Prometheus metrics:

- `copilot_seats_total` — Total Copilot seats
- `copilot_seats_active` — Active seats this cycle
- `copilot_seats_inactive` — Inactive seats this cycle
- `copilot_seats_pending` — Pending invitation
- `copilot_seats_cancelling` — Pending cancellation
- `github_metrics_last_success_timestamp` — Last successful collection

## License

MIT
