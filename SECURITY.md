# Security Architecture

This document describes the security boundaries, authentication flow, and trust zones for the navikt/copilot ecosystem. Read this before modifying authentication, authorization, network policies, or secret management.

## System Overview

```
┌─────────────┐    ┌──────────────┐    ┌─────────────┐    ┌──────────────────┐
│   Browser   │───▶│  Wonderwall  │───▶│  my-copilot  │───▶│   copilot-api    │
│  (User)     │    │  (Sidecar)   │    │  (Next.js)   │    │   (Go backend)   │
└─────────────┘    └──────────────┘    └──────────────┘    └──────────────────┘
                    Azure AD login      BFF (no secrets)    Holds all secrets:
                    Sets Authorization  Token exchange       • GitHub App key
                    header              via Texas sidecar    • BigQuery access
                                                            • Azure AD validation
```

## Trust Zones

### Zone 1: Public (no auth required)

Pages served by Next.js that contain no sensitive data:

- `/` — Landing page
- `/nyheter` — News
- `/praksis` — Best practices
- `/retningslinjer` — Guidelines
- `/cplt` — CLI documentation
- `/nav-pilot` — Agent documentation

**Enforcement:** Wonderwall `autoLoginIgnorePaths` + Next.js middleware passes through.

### Zone 2: Protected (Azure AD auth required)

Pages and API routes that show organization-level Copilot data:

- `/statistikk` — Usage statistics (BigQuery)
- `/adopsjon` — Adoption metrics (BigQuery)
- `/kostnad` — Billing overview (GitHub API)
- `/abonnement` — Seat management (GitHub API — **mutating**)
- `/kalkulator` — Cost calculator (GitHub API)
- `/api/copilot` — Seat management API route

**Enforcement:** Wonderwall auto-login redirect → Azure AD → Authorization header → Texas introspection → OBO token exchange → copilot-api JWT validation.

### Zone 3: Backend API (OBO token required)

copilot-api endpoints that access external services:

- `GET /api/v1/copilot/billing` — Organization billing data
- `GET /api/v1/copilot/seats/{username}` — Individual seat status
- `POST /api/v1/copilot/seats` — Assign seat (**mutating**)
- `DELETE /api/v1/copilot/seats/{username}` — Unassign seat (**mutating**)
- `GET /api/v1/copilot/saml/{identity}` — SAML identity lookup
- `GET /api/v1/copilot/usage/*` — BigQuery usage data
- `GET /api/v1/copilot/adoption/*` — BigQuery adoption data
- `GET /api/v1/copilot/customizations/*` — BigQuery customization data

**Enforcement:** Azure AD JWT validation (signature, issuer, audience, expiry) + `azp` claim validation against pre-authorized apps list. Fails closed if pre-authorized apps list is empty.

## Authentication Flow

```
1. Browser → Wonderwall: User navigates to protected page
2. Wonderwall → Azure AD: Redirects for login (autoLogin: true)
3. Azure AD → Wonderwall: Returns token after authentication
4. Wonderwall → Next.js: Sets Authorization: Bearer <token> header
5. Next.js → Texas sidecar: Introspects token (validates user session)
6. Next.js → Texas sidecar: Exchanges token (OBO) for copilot-api audience
7. Next.js → copilot-api: Calls API with OBO Bearer token
8. copilot-api: Validates JWT signature via JWKS, checks iss/aud/exp/azp
```

### Key Design Decisions

- **Wonderwall sets Authorization header** — with `autoLogin: true`, Wonderwall injects the bearer token on every request to the app. The Next.js middleware checks header presence for routing (not validation).
- **Texas sidecar handles token exchange** — Next.js never sees client secrets. OBO exchange happens via `NAIS_TOKEN_EXCHANGE_ENDPOINT`.
- **Azure AD OBO, NOT TokenX** — TokenX is for ID-porten (citizen-facing with BankID). This system uses Azure AD/Entra ID for Nav employees.
- **azp validation is fail-closed** — If `AZURE_APP_PRE_AUTHORIZED_APPS` is empty or missing, copilot-api rejects ALL requests. No silent bypass.

## Secret Isolation

| Secret | Location | Access |
|--------|----------|--------|
| GitHub App ID | copilot-api pod (via Nais Secret) | copilot-api only |
| GitHub App Private Key | copilot-api pod (via Nais Secret) | copilot-api only |
| GitHub Installation ID | copilot-api pod (via Nais Secret) | copilot-api only |
| BigQuery credentials | copilot-api pod (via GCP Workload Identity) | copilot-api only |
| Azure AD client config | Both pods (injected by Nais) | Auto-managed |

All external service credentials (GitHub App, BigQuery) live exclusively in the `copilot-api` pod. `my-copilot` holds no GitHub App credentials and reaches GitHub and BigQuery only through `copilot-api` via Azure AD OBO tokens.

## Network Policy

### copilot-api (`apps/copilot-api/.nais/app.yaml`)

```yaml
accessPolicy:
  inbound:
    rules:
      - application: my-copilot    # ONLY my-copilot can reach copilot-api
        namespace: copilot
      - application: copilot-cli   # CLI gateway for nav-pilot (see below)
        namespace: copilot
  outbound:
    external:
      - host: api.github.com               # GitHub REST + GraphQL API
      - host: bigquery.googleapis.com       # BigQuery data access
      - host: storage.googleapis.com        # BigQuery storage API
      - host: login.microsoftonline.com     # Azure AD JWKS endpoint
```

### copilot-cli (`apps/copilot-cli/.nais/app.yaml`)

New service (see [issue #337](https://github.com/navikt/copilot/issues/337)) that
lets `nav-pilot` fetch personal Copilot usage data from the terminal, without
routing through the my-copilot web BFF.

```
nav-pilot ──(GitHub token)──▶ copilot-cli ──(M2M token via Texas)──▶ copilot-api
```

1. The developer's GitHub token (from `nav-pilot auth login`, device flow) is
   sent to copilot-cli as a Bearer token — copilot-cli never issues or stores
   GitHub credentials itself.
2. copilot-cli validates the token via `GET api.github.com/user` and checks
   `navikt` org membership via `GET /orgs/navikt/members/{user}` (cached 5 min).
   Fails closed: any GitHub API error rejects the request rather than trusting
   a stale cache entry.
3. copilot-cli exchanges its own workload identity for an M2M access token via
   the Texas sidecar (`NAIS_TOKEN_ENDPOINT`), scoped to the copilot-api audience.
4. copilot-cli calls copilot-api with the M2M token and an
   `X-On-Behalf-Of: <github-username>` header identifying the verified user.

- Inbound: none (`accessPolicy.inbound.rules: []`) — only reachable via its
  `.intern.nav.no` ingress, which requires naisdevice.
- Outbound: copilot-api (service discovery) + `api.github.com` / `github.com`.

> **Status:** copilot-api does not yet trust the `X-On-Behalf-Of` header — that
> change (validating the M2M token's `azp` claim matches copilot-cli's client
> ID before honoring the header) is tracked as a follow-up in issue #337 and
> must land before copilot-cli's proxy is usable end-to-end.

### my-copilot (`apps/my-copilot/.nais/app.yaml`)

- Inbound: Public via ingress (Wonderwall enforces auth on protected routes)
- Outbound: copilot-api (via Nais service discovery)

## Input Validation

| Input | Validation | File |
|-------|-----------|------|
| GitHub username (path param) | Regex: `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$` | `github_handlers.go` |
| SAML identity (path param) | Non-empty, max 254 chars, no `/` | `github_handlers.go` |
| Request body (POST /seats) | `http.MaxBytesReader` 1KB limit, JSON decode | `github_handlers.go` |
| BigQuery `days` param | Integer, range 1–365 | `bigquery_handlers.go` |
| BigQuery table/view refs | Server-side only (from config, not user input) | `bigquery.go` |
| BigQuery day filter | Parameterized query (`@days`) | `bigquery.go` |

## Audit Logging

All mutating operations log the actor:

```go
slog.Info("Copilot seat assigned",
    "username", req.Username,        // Target user
    "actor", user.Email,             // Who did it
    "actor_navident", user.NAVident, // NAV employee ID
    "seats_created", result.SeatsCreated,
)
```

Debug logs use NAVident only (no email/PII at debug level).

## Error Handling

- **Client-facing errors** use RFC 7807 Problem Details (`application/problem+json`)
- **Internal errors** are logged server-side with full details but return generic messages to clients
- **No raw error strings** are forwarded to clients from upstream APIs (GitHub, BigQuery)

## Observability

| Endpoint | Auth | Purpose |
|----------|------|---------|
| `/health` | None | Kubernetes liveness probe |
| `/ready` | None | Kubernetes readiness probe |
| `/metrics` | None (pod-level only) | Prometheus scraping |

Metrics are NOT exposed via ingress — Prometheus scrapes the pod directly. The `/metrics` endpoint does not require auth because it contains only aggregate seat counts (no PII).

## Development Mode

When `NAIS_CLUSTER_NAME` is unset (local development):

- **copilot-api**: Skips Azure AD validation, injects mock user (`DEV001`)
- **my-copilot**: Skips OBO token exchange, calls backend directly without auth
- **Both**: Development bypass requires BOTH missing cluster name AND missing Azure config — cannot be accidentally triggered in production

## Boundaries

### ✅ Always

- Validate `azp` claim on all backend API requests
- Use parameterized queries for any data access
- Log mutations with actor identity (NAVident)
- Return generic error messages to clients
- Keep GitHub App credentials in copilot-api only

### ⚠️ Ask First

- Changing authentication mechanisms or token exchange
- Modifying NAIS access policies
- Adding new outbound network rules
- Changing audit log format

### 🚫 Never

- Commit secrets or credentials to git
- Log PII (email, FNR) at INFO level or above, except the minimal actor identity required for audit logging of mutations
- Forward raw upstream error messages to clients
- Skip input validation on external boundaries
- Bypass `azp` validation (even for "internal" services)
- Give my-copilot direct access to GitHub App credentials
