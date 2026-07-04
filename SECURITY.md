# Security architecture

Security boundaries, authentication flow, and trust zones for the navikt/copilot ecosystem. Read this before you touch authentication, authorization, network policies or secret management.

## System overview

```
┌─────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐
│   Browser   │───▶│  Wonderwall  │───▶│  my-copilot  │───▶│   copilot-api    │
│  (User)     │    │  (Sidecar)   │    │  (Next.js)   │    │   (Go backend)   │
└─────────────┘    └──────────────┘    └──────────────┘    └──────────────────┘
                    Azure AD login      BFF (no secrets)    Holds all secrets
                    Sets Authorization  Token exchange
                    header              via Texas sidecar
```

## Trust zones

### Zone 1: public (no auth required)

Pages and redirect routes served by Next.js that contain no sensitive data:

- `/`, landing page
- `/nyheter`, news
- `/praksis`, best practices
- `/retningslinjer`, guidelines
- `/verktoy`, catalog of Copilot customizations
- `/ordbok`, glossary
- `/ordliste`, permanent redirect to `/ordbok`
- `/kom-i-gang`, getting started
- `/cplt`, CLI documentation
- `/nav-pilot`, agent documentation
- `/install/*`, route handlers that redirect VS Code install badges to `vscode:` URLs
- `/personvern`, privacy statement
- `/tilgjengelighet`, accessibility statement

Wonderwall lists these under `autoLoginIgnorePaths`, and the Next.js middleware lets them through.

copilot-api serves an unauthenticated route group of its own, `/public/v1/`, registered on the mux outside `authMiddleware`:

- `GET /public/v1/videos`, paginated feed of published videos
- `GET /public/v1/videos/{id}`, metadata for one published video
- `GET /public/v1/videos/{id}/play`, playback URL for the HLS master
- `GET /public/v1/videos/{id}/captions`, caption track URL for one video

The group serves published entries from the video manifest only, and no Copilot, billing or user data. The handlers validate their own input: the feed rejects a `limit` outside 1-50 and a `cursor` that is not a non-negative integer, every id must match `^[a-z0-9][a-z0-9-]{1,63}$`, and the detail and play endpoints are rate limited to 60 requests per minute per client IP.

### Zone 2: protected (Azure AD auth required)

Pages and API routes that show organization-level Copilot data:

- `/statistikk`, usage statistics (BigQuery)
- `/adopsjon`, adoption metrics (BigQuery)
- `/kostnad`, billing overview (GitHub API)
- `/abonnement`, seat management (GitHub API, **mutating**)
- `/api/copilot`, seat management API route

Wonderwall lists these paths under `autoLoginIgnorePaths` too, so the sidecar does not redirect them; the comment in that config says auth is handled in the application layer. `apps/my-copilot/src/proxy.ts` redirects unauthenticated page requests to `/oauth2/login` and answers the private API routes with 401, and the protected pages call `getUser()`, which redirects to the login endpoint when the `Authorization` header carries no valid token.

### Zone 3: backend API (OBO token required)

copilot-api endpoints that reach external services:

- `GET /api/v1/copilot/billing`, organization billing data
- `GET /api/v1/copilot/seats/{username}`, individual seat status
- `POST /api/v1/copilot/seats`, assign seat (**mutating**)
- `DELETE /api/v1/copilot/seats/{username}`, unassign seat (**mutating**)
- `GET /api/v1/copilot/saml/{identity}`, SAML identity lookup
- `GET /api/v1/copilot/usage/*`, BigQuery usage data
- `GET /api/v1/copilot/adoption/*`, BigQuery adoption data
- `GET /api/v1/copilot/customizations/*`, BigQuery customization data

copilot-api validates the Azure AD JWT (signature, issuer, audience, expiry) and checks the `azp` claim against the pre-authorized apps list. It fails closed if that list is empty.

## Authentication flow

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

### Key design decisions

- **Wonderwall sets the Authorization header.** With `autoLogin: true` it injects the bearer token on every request to the app. The Next.js middleware only checks that the header is present, for routing. It does not validate it.
- **Texas handles token exchange.** Next.js never sees client secrets. The OBO exchange goes through `NAIS_TOKEN_EXCHANGE_ENDPOINT`.
- **Azure AD OBO, NOT TokenX.** TokenX is for ID-porten, which is citizen-facing with BankID. This system uses Azure AD/Entra ID for Nav employees.
- **azp validation is fail-closed.** If `AZURE_APP_PRE_AUTHORIZED_APPS` is empty or missing, copilot-api rejects ALL requests. No silent bypass.

## Secret isolation

| Secret | Location | Access |
|--------|----------|--------|
| GitHub App ID | copilot-api pod (via Nais Secret) | copilot-api only |
| GitHub App Private Key | copilot-api pod (via Nais Secret) | copilot-api only |
| GitHub Installation ID | copilot-api pod (via Nais Secret) | copilot-api only |
| BigQuery credentials | copilot-api pod (via GCP Workload Identity) | copilot-api only |
| Azure AD client config | Both pods (injected by Nais) | Auto-managed |

All external service credentials (GitHub App, BigQuery) live exclusively in the copilot-api pod. `my-copilot` holds none of them; it reaches Copilot billing, seat and BigQuery data through `copilot-api`, using Azure AD OBO tokens.

## Network policy

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

> **Status:** copilot-api trusts `X-On-Behalf-Of` via its Identity Resolver
> architecture (see `apps/copilot-api/ARCHITECTURE.md`). The
> `OnBehalfOfIdentityResolver` activates when `COPILOT_CLI_CLIENT_ID` matches
> the calling token's `azp` claim; the header value is format-validated
> against GitHub's username rules before being accepted. The env var is empty
> by default — copilot-cli's Azure AD app must be provisioned and its client
> ID configured in copilot-api before end-to-end flow works.

### my-copilot (`apps/my-copilot/.nais/app.yaml`)

Inbound is public via ingress, and Wonderwall enforces auth on the protected routes. Outbound goes to copilot-api through Nais service discovery.

## Input validation

| Input | Validation | File |
|-------|-----------|------|
| GitHub username (path param) | Regex: `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$` | `github_handlers.go` |
| SAML identity (path param) | Non-empty, max 254 chars, no `/` | `github_handlers.go` |
| Request body (POST /seats) | `http.MaxBytesReader` 1KB limit, JSON decode | `github_handlers.go` |
| BigQuery `days` param | Integer, range 1–365 | `bigquery_handlers.go` |
| BigQuery table/view refs | Server-side only (from config, not user input) | `bigquery.go` |
| BigQuery day filter | Parameterized query (`@days`) | `bigquery.go` |

## Audit logging

All mutating operations log the actor:

```go
slog.Info("Copilot seat assigned",
    "username", req.Username,        // Target user
    "actor", user.Email,             // Who did it
    "actor_navident", user.NAVident, // NAV employee ID
    "seats_created", result.SeatsCreated,
)
```

Debug logs use NAVident only, no email or other PII at debug level.

## Error handling

- Client-facing errors use RFC 7807 Problem Details (`application/problem+json`).
- Internal errors are logged server-side with full details, but clients get a generic message back.
- No raw error strings from upstream APIs (GitHub, BigQuery) are forwarded to clients.

## Observability

| Endpoint | Auth | Purpose |
|----------|------|---------|
| `/health` | None | Kubernetes liveness probe |
| `/ready` | None | Kubernetes readiness probe |
| `/metrics` | None (pod-level only) | Prometheus scraping |

These are not the whole unauthenticated surface; the `/public/v1/` video group in Zone 1 also needs no auth. Metrics are NOT exposed via ingress. Prometheus scrapes the pod directly. `/metrics` needs no auth because it contains only aggregate seat counts, no PII.

## Development mode

When `NAIS_CLUSTER_NAME` is unset (local development):

- copilot-api skips Azure AD validation and injects a mock user (`DEV001`).
- my-copilot skips the OBO token exchange and calls the backend directly without auth.
- In both apps the bypass requires BOTH a missing cluster name AND missing Azure config, so it cannot be triggered by accident in production.

## Boundaries

### ✅ Always

- Use parameterized queries for any data access
- Log mutations with actor identity (NAVident)

### ⚠️ Ask first

- Changing authentication mechanisms or token exchange
- Modifying NAIS access policies
- Adding new outbound network rules
- Changing audit log format

### 🚫 Never

- Commit secrets or credentials to git
- Log PII (email, FNR) at INFO level or above, except the minimal actor identity required for audit logging of mutations
- Forward raw upstream error messages to clients, return a generic message instead
- Skip input validation on external boundaries
- Bypass `azp` validation on any backend API request, even for "internal" services
- Give my-copilot, or any app other than copilot-api, access to GitHub App credentials
