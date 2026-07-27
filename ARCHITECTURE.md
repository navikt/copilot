# Copilot Ecosystem Architecture

## Overview

This document describes the cross-service architecture of Nav's GitHub Copilot ecosystem. For service-specific details, see:
- [`apps/copilot-api/ARCHITECTURE.md`](apps/copilot-api/ARCHITECTURE.md) — Identity resolution pattern
- [`SECURITY.md`](SECURITY.md) — Trust zones, secret isolation, network policies

## System Diagram

```
                  ┌──────────────────────────────────────────────────────────────────────┐
                  │                        Web Flow (my-copilot)                          │
                  │                                                                       │
┌─────────┐      │  ┌──────────┐   ┌───────────┐   ┌────────┐                           │
│ Browser │─────▶│  │Wonderwall│──▶│my-copilot │──▶│ Texas  │                           │
│         │      │  │(OAuth2)  │   │ (Next.js) │   │(OBO)   │                           │
└─────────┘      │  └──────────┘   └───────────┘   └───┬────┘                           │
                  │                                      │ OBO token                      │
                  └──────────────────────────────────────┼───────────────────────────────┘
                                                         │
                                                         ▼
                  ┌──────────────────────────────────────────────────────────────────────┐
                  │                      copilot-api (Go backend)                         │
                  │                                                                       │
                  │   ┌─────────────┐   ┌────────────────────┐   ┌────────────────────┐  │
                  │   │ Auth MW     │──▶│ Identity Middleware │──▶│ Business Logic     │  │
                  │   │ (JWT+JWKS)  │   │ (Strategy Chain)   │   │ (GitHub/BQ/Budget) │  │
                  │   └─────────────┘   └────────────────────┘   └────────────────────┘  │
                  │                                                                       │
                  └──────────────────────────────────────────────────────────────────────┘
                                                         ▲
                                                         │ M2M token + X-On-Behalf-Of
                  ┌──────────────────────────────────────┼───────────────────────────────┐
                  │                        CLI Flow (copilot-cli)                         │
                  │                                                                       │
┌─────────┐      │  ┌────────────┐   ┌───────────┐   ┌────────┐                         │
│Developer│─────▶│  │ nav-pilot  │──▶│copilot-cli│──▶│ Texas  │                         │
│Terminal │      │  │(GitHub PAT)│   │(Go proxy) │   │(M2M)   │                         │
└─────────┘      │  └────────────┘   └───────────┘   └────────┘                         │
                  │                                                                       │
                  └──────────────────────────────────────────────────────────────────────┘
```

## Authentication Flows

### Web Flow: Browser → my-copilot → copilot-api

```
1. Browser → Wonderwall: User navigates to protected page
2. Wonderwall → Azure AD: Redirects for login (autoLogin: true)
3. Azure AD → Wonderwall: Returns user token
4. Wonderwall → my-copilot: Sets Authorization header
5. my-copilot → Texas: Exchanges user token for OBO token (scoped to copilot-api)
6. my-copilot → copilot-api: Calls API with OBO token
7. copilot-api → Identity Middleware: SAMLIdentityResolver resolves email → GitHub username
```

### CLI Flow: nav-pilot → copilot-cli → copilot-api

```
1. Developer runs `nav-pilot auth login` (GitHub device flow → token stored in OS keychain)
2. nav-pilot → copilot-cli: Sends request with GitHub PAT as Bearer token
3. copilot-cli validates PAT via GitHub API (GET /user, check navikt org membership)
4. copilot-cli → Texas: Gets M2M token scoped to copilot-api audience
5. copilot-cli → copilot-api: M2M token + X-On-Behalf-Of: <github-username>
6. copilot-api → Identity Middleware: OnBehalfOfIdentityResolver trusts header (azp validated)
```

## Identity Resolution Architecture

The central design pattern is **mechanism-agnostic identity resolution** — handlers never know how the caller's GitHub username was determined.

```
                    ┌─────────────────────────────────┐
                    │   IdentityResolverChain          │
                    │   (first-match-wins)             │
                    └─────────┬───────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              │                               │
              ▼                               ▼
┌──────────────────────────┐   ┌──────────────────────────┐
│ OnBehalfOfIdentityResolver│   │ SAMLIdentityResolver      │
│ Condition: azp ∈ trusted  │   │ Condition: user has email │
│ Resolution: X-On-Behalf-Of│   │ Resolution: SCIM lookup   │
│ (format-validated)        │   │ (GitHub GraphQL)          │
└──────────────────────────┘   └──────────────────────────┘
              │                               │
              └───────────────┬───────────────┘
                              ▼
                    ┌─────────────────────────┐
                    │ ResolvedIdentity         │
                    │ { GitHubUsername, Source }│
                    └─────────────────────────┘
                              │
                              ▼
                    ┌─────────────────────────┐
                    │ requireOwnership(w,r,u)  │
                    │ (mechanism-agnostic)     │
                    └─────────────────────────┘
```

**Adding a new auth mechanism** (e.g., GitHub Actions OIDC, service accounts):
1. Implement `IdentityResolver` interface (CanResolve + Resolve)
2. Register in chain in `main.go`
3. Zero handler changes needed

See [`apps/copilot-api/ARCHITECTURE.md`](apps/copilot-api/ARCHITECTURE.md) for full implementation details.

## Service Responsibilities

| Service | Role | Secrets Held | Auth Mechanism |
|---------|------|-------------|----------------|
| **my-copilot** | Web BFF — SSR, presentation logic | None (Texas handles tokens) | Azure AD OBO via Texas |
| **copilot-cli** | CLI gateway — validates GitHub PAT, proxies to copilot-api | None (Texas handles tokens) | GitHub PAT → M2M via Texas |
| **copilot-api** | Backend — business logic, data access | GitHub App key, BigQuery | Azure AD JWT (OBO or M2M) |
| **nav-pilot** | Developer CLI — auth, usage display | GitHub PAT (OS keychain) | GitHub device flow |
| **copilot-metrics** | Naisjob — daily BigQuery ETL | BigQuery (Workload Identity) | None (internal job) |

## Defense-in-Depth Layers

```
Layer 1: Network       — NAIS accessPolicy (only authorized services can reach copilot-api)
Layer 2: Auth          — JWT signature, issuer, audience, expiry validated
Layer 3: Authorization — azp claim checked against pre-authorized apps list
Layer 4: Identity      — Resolved GitHub username via appropriate mechanism
Layer 5: Ownership     — requireOwnership() compares resolved identity with requested resource
Layer 6: Input         — Format validation on all external inputs (usernames, headers, body)
```

## Data Flow

### Per-User Data (requires ownership check)

```
/usage/user/{username}   — BigQuery personal metrics
/seats/{username}        — Copilot seat status
POST /seats              — Self-service seat assignment
DELETE /seats/{username}  — Self-service seat removal
/budget                  — Personal budget allocation
```

All wrapped with `requireResolvedIdentity` at the router level + `requireOwnership` in handlers.

### Aggregate Data (org-level, no ownership check)

```
/usage/metrics           — Org-wide daily metrics
/adoption/*              — Team/language adoption
/billing                 — Enterprise billing summary
/billing/premium         — Premium request usage
```

Available to any authenticated Nav employee (valid Azure AD token with matching azp).

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| copilot-api holds all secrets | Single audit point; BFF/CLI never touch GitHub App credentials |
| Texas sidecar for token exchange | No client secrets in application code; platform-managed rotation |
| X-On-Behalf-Of (not token forwarding) | M2M tokens have no user claims; header is explicit and auditable |
| Format validation on trusted headers | Defense-in-depth even against compromised intermediaries |
| Chain pattern (not if/else) | Open/Closed principle — new mechanisms don't modify existing code |
| Double middleware on per-user routes | Belt-and-suspenders: structural enforcement + handler-level check |
| GitHub PAT for CLI (not Azure AD) | Developers already have GitHub tokens; avoid Azure device flow UX issues |

## Network Topology

```yaml
# copilot-api inbound
- application: my-copilot     # Web BFF
- application: copilot-cli    # CLI gateway

# copilot-cli inbound
# None — only reachable via .intern.nav.no ingress (requires naisdevice)

# copilot-cli outbound
- application: copilot-api    # Backend API
- host: api.github.com        # PAT validation + org membership check

# copilot-api outbound
- host: api.github.com        # GitHub REST + GraphQL
- host: bigquery.googleapis.com
- host: storage.googleapis.com
- host: login.microsoftonline.com  # JWKS
```

## Implementation Status

### ✅ Complete
- Go backend service with 25+ API endpoints
- Azure AD OBO + M2M token validation
- Identity resolution facade (Strategy + Chain + Middleware)
- JWKS caching with automatic refresh
- Health and readiness endpoints
- RFC 7807 Problem Details error handling
- OpenTelemetry auto-instrumentation
- NAIS deployment (dev + prod)
- GitHub App authentication with JWT + installation tokens
- Background metrics collector (5min interval)
- BigQuery operations with in-memory caching (1h TTL)
- GitHub API operations (billing, premium usage, seat management, SAML lookup)
- copilot-cli service with GitHub PAT validation and M2M proxy
- nav-pilot CLI auth (device flow, keychain, usage command)
- Budget API integration
- Video content delivery (public API)

### ⏳ In Progress
- copilot-cli end-to-end testing in dev environment

## Token Validation (copilot-api)

**Required checks:**
1. **Signature** — JWKS from `AZURE_OPENID_CONFIG_JWKS_URI`
2. **Issuer** — Must match `AZURE_OPENID_CONFIG_ISSUER`
3. **Audience** — Must be `AZURE_APP_CLIENT_ID`
4. **Authorized Party (azp)** — Must be in `AZURE_APP_PRE_AUTHORIZED_APPS`
5. **Expiry** — Token not expired

**Extracted claims:**
- `preferred_username` → Email (used by SAML resolver)
- `NAVident` → Employee ID (audit logging)
- `name` → Display name
- `groups` → Azure AD groups
- `azp` → Calling app client ID (drives resolver selection)

## API Design Principles

### 1. Resource-Oriented
```
GET  /api/v1/copilot/usage/user/{username}  ← Per-user metrics
GET  /api/v1/copilot/usage/metrics          ← Org aggregate
POST /api/v1/copilot/seats                  ← Self-service assign
```

### 2. Canonical DTOs
Backend returns stable, documented DTOs. Frontends transform for UI needs.

### 3. Cache Strategy

| Data Type | Backend Cache | Response Cache-Control |
|-----------|---------------|----------------------|
| Per-user metrics | None | 5min private |
| Seat status | None | 5min private |
| Billing | 1h TTL | 15min private |
| BigQuery dashboards | 1h TTL | 15min public |
| Mutations | None | no-store |
| `/metrics` | Background (5min) | N/A (Prometheus) |

### 4. Error Handling (RFC 7807)

```json
{
  "type": "https://copilot-api.nav.no/errors/unauthorized",
  "title": "Unauthorized",
  "status": 401,
  "detail": "Caller identity could not be determined"
}
```

## Observability

| Layer | Tool | What |
|-------|------|------|
| Logs | Loki | Structured JSON (slog), request/response, audit trail |
| Metrics | Prometheus/Mimir | Seat counts, request latency, data freshness |
| Traces | Tempo | Distributed tracing across BFF → Backend → GitHub/BQ |

## References

- [Texas Sidecar Documentation](https://doc.nais.io/auth/explanations/README/)
- [Azure AD On-Behalf-Of Flow](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-on-behalf-of-flow)
- [RFC 7807 Problem Details](https://www.rfc-editor.org/rfc/rfc7807)
- [GitHub Apps Authentication](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/about-authentication-with-a-github-app)
- [GitHub Device Flow](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow)
