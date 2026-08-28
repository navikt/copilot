# Copilot backend API architecture

`copilot-api` is the Go service behind `my-copilot`, the Next.js portal at
min-copilot.ansatt.nav.no. It holds the GitHub App and BigQuery credentials; my-copilot
holds neither.

This document covers the shape of the system and the reasoning behind it. The endpoint
list, the full config table and the error-type catalogue live in
[`apps/copilot-api/README.md`](./apps/copilot-api/README.md). Trust zones, input
validation and audit logging live in [SECURITY.md](./SECURITY.md).

## Request path

```
Browser
  │  HTTPS
  ▼
Wonderwall (Azure AD OAuth2 proxy)
  │  sets session cookie, injects Authorization: Bearer <Azure AD token>
  ▼
my-copilot (Next.js BFF)
  │  server-side rendering, presentation logic, client-specific transforms
  ▼
Texas sidecar
  │  introspects the user token, then exchanges it for an OBO token
  ▼
copilot-api (Go, internal only)
  │
  ├──▶ GitHub API
  ├──▶ BigQuery (GCP)
  └──▶ MCP Registry
```

The BFF never validates a token itself and never holds a client secret. It asks Texas
to introspect the incoming token, asks Texas again for an On-Behalf-Of token minted for
the `copilot-api` audience, and forwards that. `copilot-api` then validates the OBO
token from scratch. Nothing the BFF asserts about the user is trusted.

## Token validation

Every request to `/api/v1/` must clear five checks before a handler runs.

1. Signature, against the JWKS at `AZURE_OPENID_CONFIG_JWKS_URI` (cached, refreshed when the
   TTL lapses or an unknown `kid` shows up)
2. Issuer, must match `AZURE_OPENID_CONFIG_ISSUER`
3. Audience, must be `AZURE_APP_CLIENT_ID` (the `aud` claim may be a string or an array)
4. Authorized party, `azp` must appear in `AZURE_APP_PRE_AUTHORIZED_APPS`, which in
   practice means my-copilot and nothing else
5. Expiry

From the surviving token the service takes `preferred_username` (email), `NAVident`
(employee ID), `name`, `groups` and `azp`. `azp` is kept for the audit trail so a
mutation can be traced back to the calling application as well as the person.

The `azp` check fails closed. An empty or missing pre-authorized apps list rejects
everything rather than letting anything through.

## API design

### Resource-oriented, not page-oriented

```
GET  /api/v1/copilot/usage/summary      ← Aggregate metrics
GET  /api/v1/copilot/usage/trends       ← Time-series
GET  /api/v1/copilot/seats/{username}   ← Single resource
POST /api/v1/copilot/seats              ← Create
```

Endpoints name resources, not screens. There is deliberately no
`GET /api/v1/dashboard-data` and no `GET /api/v1/overview-stats`. A page that needs
three things makes three calls, and a new page needs no new backend endpoint.

### Canonical DTOs

The backend returns one stable documented shape per resource. The frontend reshapes it
for whatever the chart library wants. Chart-shaped JSON never leaks into the API.

```go
// Backend DTO (canonical, stable)
type UsageSummary struct {
    TotalAcceptances  int64  `json:"total_acceptances"`
    TotalGenerations  int64  `json:"total_generations"`
    AcceptanceRate    int    `json:"acceptance_rate"`
    DateRange         string `json:"date_range"`
}
```

### Cache strategy

| Data type | Backend cache | BFF cache | Why |
|-----------|---------------|-----------|-----|
| Seat status | 60s + invalidation | Optional | Mutates often |
| Billing | 1h TTL | 1h stale | Friendly to the GitHub rate limit |
| BigQuery dashboards | 1h TTL | 1h stale | Queries are expensive |
| Seat mutations | None | None | Always fresh |
| `/metrics` | Background (5min) | No | Prometheus scrape |

The backend owns the TTL and says so in a `Cache-Control` header. The BFF may layer its
own caching on top, but it does not get to decide how stale org-wide data is allowed
to be.

### Errors

Client-facing errors are RFC 7807 Problem Details, served as `application/problem+json`.

```json
{
  "type": "https://copilot-api.nav.no/errors/unauthorized",
  "title": "Unauthorized",
  "status": 401,
  "detail": "Invalid or expired token"
}
```

## Security boundaries

### What the backend MUST NOT trust

- The user identity the BFF claims, without validating the token itself
- Client-provided pagination parameters (validate ranges)
- Date ranges for BigQuery (enforce max 365 days)

## Metrics collection

`/metrics` used to be served by my-copilot, which called `getCopilotBilling()` inline on
every Prometheus scrape. That made scrape latency a function of GitHub's mood, and a
GitHub outage turned into a gap in the metrics.

Now a background job in `copilot-api` polls GitHub every 5 minutes and writes into an
in-memory struct. The scrape reads that struct and returns in <1ms, and cannot fail
because GitHub is down.
`github_metrics_last_success_timestamp` carries the freshness of the data, so a stale
collector is visible in Prometheus rather than silently serving old numbers.

## Deployment

Both apps deploy to NAIS in `dev-gcp` and `prod-gcp`.

```yaml
# copilot-api
azure:
  application:
    enabled: true
    tenant: nav.no

accessPolicy:
  inbound:
    rules:
      - application: my-copilot
        namespace: copilot
  outbound:
    external:
      - host: api.github.com
      - host: bigquery.googleapis.com
```

```yaml
# my-copilot
accessPolicy:
  outbound:
    rules:
      - application: copilot-api
        namespace: copilot
```

`copilot-api` has no ingress. The inbound rule is the only way in. `/health` and
`/ready` are unauthenticated so the Kubernetes probes can reach them, and so are
`/metrics` and the `/public/v1/` routes.

Against GitHub the service authenticates as a GitHub App. It signs a short-lived JWT
with the app private key, trades it for an installation token, and reuses that token
until it nears expiry. Calls respect the GitHub rate limits.

Azure config (`AZURE_APP_CLIENT_ID`, `AZURE_OPENID_CONFIG_ISSUER`,
`AZURE_OPENID_CONFIG_JWKS_URI`, `AZURE_APP_PRE_AUTHORIZED_APPS`) and
`GCP_TEAM_PROJECT_ID` are injected by NAIS. The GitHub App credentials
(`GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY`, `GITHUB_APP_INSTALLATION_ID`) come from a
NAIS secret. The rest of the variables and their defaults are tabulated in
[`apps/copilot-api/README.md`](./apps/copilot-api/README.md#configuration).

my-copilot needs `NAIS_TOKEN_EXCHANGE_ENDPOINT` and
`NAIS_TOKEN_INTROSPECTION_ENDPOINT` for the Texas sidecar, and `COPILOT_API_URL`, which
is the internal address `http://copilot-api`.

## Testing

Unit tests cover token validation, the GitHub client, the BigQuery client and error
handling. Integration tests cover the auth flow end to end with test tokens, GitHub API
mocking and query correctness. Golden tests compare output from the old path
(my-copilot calling GitHub directly) against the new one to prove the data survived the
move unchanged. Load tests cover seat management under load, cache invalidation and the
resilience of the background collector.

## Observability

Logs go to Loki as structured JSON from `slog`. Requests to `/api/v1/` are logged;
errors carry stack traces. Prometheus scrapes the seat gauges and the freshness
timestamp from `/metrics`, plus the auto-instrumented HTTP request metrics. Traces go to
Tempo through OpenTelemetry auto-instrumentation, and span the whole chain from BFF to
backend to GitHub or BigQuery.

## Status

The backend is deployed with all endpoints, and every main flow in the frontend goes
through it via OBO token exchange: `/metrics` on the background-collection pattern,
BigQuery reads for usage and adoption, GitHub billing, and seat assignment and
unassignment with audit logging. The one thing still to move is the admin-only debug
endpoint for raw `repo_scan` BigQuery queries, which is low priority.

## References

- [Texas Sidecar Documentation](https://doc.nais.io/auth/explanations/README/)
- [Azure AD On-Behalf-Of Flow](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-on-behalf-of-flow)
- [RFC 7807 Problem Details](https://www.rfc-editor.org/rfc/rfc7807)
- [GitHub Apps Authentication](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/about-authentication-with-a-github-app)
