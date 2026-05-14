# Copilot Backend API Architecture

## Overview

This document describes the architecture for extracting backend functionality from `my-copilot` (Next.js) into a dedicated `copilot-api` (Go) service.

## Current Implementation Status

### ✅ Phase 1: Backend API Skeleton (Complete)
- Created `apps/copilot-api/` Go application
- Implemented Azure AD On-Behalf-Of (OBO) token validation
- JWKS caching with automatic refresh
- Health endpoints (`/health`, `/ready`)
- RFC 7807 Problem Details error handling
- OpenTelemetry auto-instrumentation ready
- NAIS configuration (dev + prod)
- Comprehensive unit tests (100% pass rate)

### ✅ Phase 2: /metrics Endpoint (Complete)
- GitHub App authentication with JWT + installation tokens
- Background metrics collector (5min interval)
- Cached Prometheus metrics endpoint
- Metrics freshness tracking (`github_metrics_last_success_timestamp`)
- my-copilot NAIS config updated with outbound rule

### 🔄 Phase 3: BigQuery Operations (Next)
- Set up BigQuery client in Go
- Migrate 6 query types to backend
- Create canonical DTOs
- Implement caching strategy
- Update my-copilot to proxy to backend

## Architecture Diagram

```
┌─────────────┐
│   Browser   │
└──────┬──────┘
       │
       │ HTTPS + Azure AD token
       ▼
┌─────────────────────────────────────────────────┐
│  Wonderwall (Azure AD OAuth2 Proxy)             │
└──────┬──────────────────────────────────────────┘
       │
       │ Authenticated request
       ▼
┌─────────────────────────────────────────────────┐
│  my-copilot (Next.js BFF)                       │
│  - Server-side rendering                        │
│  - Token exchange via Texas sidecar             │
│  - Presentation logic                           │
│  - Client-specific data transformation          │
└──────┬──────────────────────────────────────────┘
       │
       │ OBO token (via Texas)
       ▼
┌─────────────────────────────────────────────────┐
│  Texas Sidecar                                  │
│  - Token introspection                          │
│  - Azure AD OBO exchange                        │
└──────┬──────────────────────────────────────────┘
       │
       │ OBO token
       ▼
┌─────────────────────────────────────────────────┐
│  copilot-api (Go Backend) - INTERNAL ONLY       │
│  ┌───────────────────────────────────────────┐  │
│  │ Auth Middleware                           │  │
│  │ - Validate OBO token signature (JWKS)    │  │
│  │ - Verify issuer, audience, expiry        │  │
│  │ - Check azp (authorized party)           │  │
│  │ - Extract user claims (email, NAVident)  │  │
│  └───────────────────────────────────────────┘  │
│                                                  │
│  ┌───────────────────────────────────────────┐  │
│  │ Business Logic                            │  │
│  │ - BigQuery aggregations                   │  │
│  │ - GitHub API operations                   │  │
│  │ - Seat management with audit logging     │  │
│  │ - Cache management                        │  │
│  └───────────────────────────────────────────┘  │
│                                                  │
│  ┌───────────────────────────────────────────┐  │
│  │ Background Jobs                           │  │
│  │ - Metrics collector (5min interval)      │  │
│  │ - Future: cache warming, cleanup         │  │
│  └───────────────────────────────────────────┘  │
└──────┬────────────┬────────────┬────────────────┘
       │            │            │
       │            │            │
       ▼            ▼            ▼
   ┌────────┐  ┌─────────┐  ┌─────────┐
   │ GitHub │  │BigQuery │  │   MCP   │
   │  API   │  │   GCP   │  │Registry │
   └────────┘  └─────────┘  └─────────┘
```

## Authentication Flow

### 1. User Login (Wonderwall)
```
Browser → Wonderwall → Azure AD → Wonderwall → Browser
                                     (sets cookie)
```

### 2. BFF Request
```
Browser → my-copilot → Texas introspection → User claims
              ↓
         Render page or API response
```

### 3. Backend API Call (OBO)
```
my-copilot → Texas OBO exchange → copilot-api
                                      ↓
                          Validate token + Extract user
                                      ↓
                          Execute business logic
                                      ↓
                          Return canonical DTO
```

## Token Validation (copilot-api)

**Required checks:**
1. **Signature** — JWKS from `AZURE_OPENID_CONFIG_JWKS_URI`
2. **Issuer** — Must match `AZURE_OPENID_CONFIG_ISSUER`
3. **Audience** — Must be `AZURE_APP_CLIENT_ID`
4. **Authorized Party (azp)** — Must be in `AZURE_APP_PRE_AUTHORIZED_APPS` (my-copilot client ID)
5. **Expiry** — Token not expired

**Extracted claims:**
- `preferred_username` → Email
- `NAVident` → Employee ID
- `name` → Display name
- `groups` → Azure AD groups
- `azp` → Calling app (audit trail)

## API Design Principles

### 1. Resource-Oriented
```
GET  /api/v1/copilot/usage/summary      ← Aggregate metrics
GET  /api/v1/copilot/usage/trends       ← Time-series
GET  /api/v1/copilot/seats/{username}   ← Single resource
POST /api/v1/copilot/seats              ← Create
```

Not page-oriented:
```
❌ GET /api/v1/dashboard-data
❌ GET /api/v1/overview-stats
```

### 2. Canonical DTOs
Backend returns stable, documented DTOs. Frontend transforms for UI needs:

```go
// Backend DTO (canonical, stable)
type UsageSummary struct {
    TotalAcceptances  int64  `json:"total_acceptances"`
    TotalGenerations  int64  `json:"total_generations"`
    AcceptanceRate    int    `json:"acceptance_rate"`
    DateRange         string `json:"date_range"`
}

// Frontend transforms to chart-specific shapes
```

### 3. Cache Strategy

| Data Type | Backend Cache | BFF Cache | Notes |
|-----------|---------------|-----------|-------|
| Seat status | 60s + invalidation | Optional | Frequent mutations |
| Billing | 1h TTL | 1h stale | GitHub rate limit friendly |
| BigQuery dashboards | 1h TTL | 1h stale | Expensive queries |
| Seat mutations | None | None | Always fresh |
| `/metrics` | Background (5min) | No | Prometheus scrape |

Backend sets `Cache-Control` headers. BFF can layer additional caching.

### 4. Error Handling (RFC 7807)

```json
{
  "type": "https://copilot-api.nav.no/errors/unauthorized",
  "title": "Unauthorized",
  "status": 401,
  "detail": "Invalid or expired token"
}
```

## Security Boundaries

### Defense in Depth
1. **Network isolation** — copilot-api only accessible from my-copilot (NAIS `accessPolicy.inbound.rules`)
2. **Token validation** — Backend independently validates all tokens (zero trust)
3. **Authorized party check** — `azp` claim ensures only my-copilot can call API
4. **Audit logging** — All mutations logged with user identity
5. **Rate limiting** — GitHub API calls respect rate limits
6. **Secrets rotation** — GitHub App credentials stored in NAIS secrets

### What Backend MUST NOT Trust
- ❌ BFF-provided user identity without token validation
- ❌ Client-provided pagination parameters (validate ranges)
- ❌ Date ranges for BigQuery (enforce max 365 days)

## Metrics Architecture

### Problem: Prometheus Scraping Synchronously Calling GitHub
**Old (my-copilot):**
```
Prometheus scrape → /metrics → getCopilotBilling() → GitHub API (slow, fragile)
```

**New (copilot-api):**
```
Background job (5min) → GitHub API → Update in-memory metrics
                                              ↓
Prometheus scrape → /metrics → Return cached metrics (fast, reliable)
```

**Benefits:**
- Prometheus scrapes are <1ms (just reads memory)
- GitHub API failures don't fail scrapes
- `github_metrics_last_success_timestamp` tracks data freshness

## Deployment

### NAIS Configuration

**copilot-api:**
```yaml
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

**my-copilot:**
```yaml
accessPolicy:
  outbound:
    rules:
      - application: copilot-api
        namespace: copilot
```

### Environment Variables

**copilot-api needs:**
- `AZURE_APP_CLIENT_ID` (injected by NAIS)
- `AZURE_OPENID_CONFIG_ISSUER` (injected)
- `AZURE_OPENID_CONFIG_JWKS_URI` (injected)
- `AZURE_APP_PRE_AUTHORIZED_APPS` (injected)
- `GITHUB_APP_ID` (secret)
- `GITHUB_APP_PRIVATE_KEY` (secret)
- `GITHUB_APP_INSTALLATION_ID` (secret)
- `GCP_TEAM_PROJECT_ID` (injected)

**my-copilot needs:**
- `NAIS_TOKEN_EXCHANGE_ENDPOINT` (Texas sidecar)
- `COPILOT_API_URL` (internal: `http://copilot-api`)

## Migration Strategy (Strangler Pattern)

1. ✅ **Backend skeleton** — Deploy internal API with placeholder endpoints
2. ✅ **Migrate /metrics** — Move to background collection pattern
3. 🔄 **Migrate BigQuery reads** — Keep writes in my-copilot, reads from backend
4. ⏳ **Migrate billing** — GitHub billing API to backend
5. ⏳ **Migrate seat management** — Last due to audit requirements
6. ⏳ **Cleanup my-copilot** — Remove Octokit, BigQuery client, secrets

**Each phase:**
- Deploy backend endpoint
- Update my-copilot to call it
- Compare outputs (golden tests)
- Monitor for 24h in dev
- Deploy to prod
- Delete old code

## Testing Strategy

### Unit Tests (Go)
- Token validation logic
- GitHub API client
- BigQuery client
- Error handling

### Integration Tests
- End-to-end auth flow with test tokens
- GitHub API mocking
- BigQuery query correctness

### Golden Tests
- Compare old (my-copilot direct) vs new (via backend) outputs
- Ensure data transformation is identical

### Load Tests
- Seat management under load
- Cache invalidation behavior
- Background collector resilience

## Observability

### Logs (Loki)
- Structured JSON logs (slog)
- Request/response logging for `/api/v1/`
- Error logs with stack traces

### Metrics (Prometheus)
- GitHub billing metrics (seats, active, inactive)
- Metrics freshness timestamp
- HTTP request metrics (auto-instrumented)

### Traces (Tempo)
- OpenTelemetry auto-instrumentation
- Distributed tracing across BFF → Backend → GitHub/BigQuery

## Open Questions / TODOs

1. **BigQuery connection pooling** — Single global client or per-request?
2. **Cache invalidation propagation** — How does backend tell BFF to invalidate?
3. **GraphQL for SAML lookup** — Need GitHub GraphQL client in Go
4. **Rate limit handling** — Retry with exponential backoff? Queue?
5. **Audit log storage** — Structured logs sufficient or dedicated table?

## References

- [Texas Sidecar Documentation](https://doc.nais.io/auth/explanations/README/)
- [Azure AD On-Behalf-Of Flow](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-on-behalf-of-flow)
- [RFC 7807 Problem Details](https://www.rfc-editor.org/rfc/rfc7807)
- [GitHub Apps Authentication](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/about-authentication-with-a-github-app)
