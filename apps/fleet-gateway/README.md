# fleet-gateway

OAuth 2.1 authorization server fronting Microsoft Entra ID for Nav's internal MCP gateway.

The gateway is live in prod (2026-11-17) for a controlled pilot. It runs the authorization server (protected resource metadata, authorization server metadata, CIMD and DCR client registration, consent, Entra authorization-code redemption and refresh) and routes calls to team MCP servers with Texas OBO. See the [plan and decisions log](https://github.com/navikt/copilot/issues/2351) for phases and status.

## Endpoints

- `GET /.well-known/oauth-protected-resource` — RFC 9728 Protected Resource Metadata
- `GET /.well-known/oauth-protected-resource/{resource}` — Resource-specific PRM (must match a configured pilot route)
- `GET /.well-known/oauth-authorization-server` — RFC 8414 Authorization Server Metadata
- `POST /register` — RFC 7591 dynamic client registration, only when `DCR_ENABLED=true`
- `GET /authorize` — OAuth 2.1 authorization endpoint (CIMD allowlist, exact `redirect_uri` except the port of http loopback IP literals per RFC 8252 §7.3, PKCE `S256`, RFC 8707 `resource`, RFC 9207 `iss`)
- `POST /consent` — Consent submission with CSRF cookie check. The page is shown after the Entra login, and only when the user has no remembered consent for this client and route
- `GET /oauth2/callback` — Entra ID reply URL, verified against signed `__Host-fleet_entra_state` cookie
- `POST /token` — Single-use, 60-second authorization code exchange with PKCE verification
- `POST /token` also accepts `grant_type=refresh_token` when `RT_STORAGE=client-encrypted` (single-use rotation, absolute `SESSION_MAX_AGE` cap)
- `POST /revoke` — RFC 7009 revocation of an access token or refresh token
- `GET /health` — Liveness probe
- `GET /ready` — Readiness probe

## Configuration

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `BASE_URL` | Yes | — | External gateway origin without trailing slash |
| `ALLOWED_CLIENT_IDS` | Yes | — | Comma-separated allowlist of CIMD `client_id` URLs |
| `ALLOWED_REDIRECT_URIS` | Yes | — | Comma-separated allowlist of `redirect_uri` values. Exact match, except that `http://127.0.0.1` and `http://[::1]` entries match on any port (RFC 8252 §7.3); `localhost` never does |
| `DCR_ENABLED` | No | `false` | Serve RFC 7591 dynamic client registration at `POST /register` and advertise `registration_endpoint`. Only http loopback redirect URIs and `token_endpoint_auth_method: none`; registered clients are shown as unverified on the consent page |
| `DCR_CLIENT_TTL` | No | `720h` | Lifetime of an unused registered client. Each use extends it, up to `DCR_CLIENT_MAX_AGE` |
| `DCR_CLIENT_MAX_AGE` | No | `4320h` | Maximum lifetime of a registered client, used or not |
| `DCR_CLIENT_CAP` | No | `5000` | Maximum number of live registered clients. When full, a new registration evicts the client with the earliest expiry. Expiry slides forward on each successful token exchange (at most once per `DCR_CLIENT_TTL`/30, never past the max age), so this approximates least recently used |
| `DCR_RATE_LIMIT_REQUESTS` | No | `10` | Registrations per IP per hour, shared by all replicas through the session store |
| `DCR_RATE_LIMIT_GLOBAL_REQUESTS` | No | `100` | Registrations per hour across all IPs and replicas. Requests the per-IP limit rejects do not count |
| `AZURE_APP_CLIENT_ID` | Yes | — | Nais-injected Entra client ID |
| `AZURE_APP_CLIENT_SECRET` | Yes | — | Nais-injected Entra client secret |
| `AZURE_OPENID_CONFIG_TOKEN_ENDPOINT` | Yes | — | Nais-injected Entra token endpoint |
| `AZURE_APP_WELL_KNOWN_URL` | Conditional | — | Nais-injected OpenID discovery URL (required unless `AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT` is set) |
| `AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT` | Conditional | — | Direct override for Entra authorization endpoint |
| `ENTRA_SCOPE` | No | `openid api://<own client id>/.default` (plus `offline_access` for `refresh`) | Scope requested from Entra |
| `COOKIE_SIGNING_SECRET` | No | HKDF subkey of the primary `RT_KEYS` key; random only outside Nais | HMAC key (>= 32 bytes) for the `__Host-fleet_entra_state` cookie. On Nais (`NAIS_CLUSTER_NAME` set) startup fails if neither this nor `RT_KEYS` is set |
| `PORT` | No | `8080` | HTTP listen port |
| `LOG_LEVEL` | No | `INFO` | `DEBUG`, `INFO`, `WARN`, or `ERROR` |

| `TEAM_TOKEN_STRATEGY` | No | `obo` | `obo` (Texas OBO) or `refresh` (Entra refresh token held in the session; needs `RT_STORAGE=server`) |
| `SESSION_STORE` | No | `memory` | `memory` (tests/local; lost on restart, not shared across replicas) or `postgres` |
| `DATABASE_URL` | No | — | Postgres URL override. If empty, pgx reads the libpq `PG*` variables that Nais Postgres injects (TLS client certs, PgBouncer transaction mode; simple query protocol, no session state). Migrations run with the unprefixed admin `PG*` credentials; the running store uses the `READWRITE_PG*` runtime credentials |
| `SESSION_MAX_AGE` | No | `12h` | Absolute session lifetime from login; refresh never extends it |
| `REFRESH_GRACE_WINDOW` | No | `30s` | How long the immediately previous refresh token may be reused. Reuse inside the window returns the same token pair; reuse after it revokes the whole token family |
| `CONSENT_TTL` | No | `720h` | How long a consent is remembered per NAVident, client_id and route. `/revoke` clears it |
| `RT_STORAGE` | No | `client-encrypted` if `RT_KEYS` set, else none | `client-encrypted`: the Entra refresh token is sealed (AES-256-GCM) inside the gateway refresh token returned to the client; nothing is stored. `server`: refresh token stays in the session store, no client refresh token |
| `RT_KEYS` | With `postgres` or `client-encrypted` | — | `kid:base64key,...` (32-byte keys, newest first encrypts, all decrypt). Also seals every value in the session store |
| `GATEWAY_DISABLED` | No | `false` | Set to `true` to disable all `/mcp/*` routes (`503`, `Retry-After: 300`) |

A route path is `/mcp/<namespace>/<name>`, and `<namespace>` must equal the namespace in `target` (for example `/mcp/copilot/onboarding`). The old `/mcp/<name>` paths answer `404` with the new URL in `new_url`; they are not forwarded.

Routes in `pilot-routes.yaml` (`PILOT_ROUTES_FILE`) support an optional `enabled: false` field to disable a single route (`503`, `Retry-After: 300`) while keeping it validated and registered.

In `version: 2` route files every route lists the tools it allows in `tools: [...]`; the gateway rejects any other `tools/call`, and any ambiguous JSON-RPC body (case-variant or duplicate keys, trailing data), with a JSON-RPC error and does not forward it. Only the operator sets `passAllTools: true` instead, for a route that forwards every call. `version: 1` files load unchanged and pass every call.

On every route, `passAllTools` included, a `GET` or `DELETE` with a body gets `400` and is logged as `deny`/`ambiguous`: MCP defines no body for them, so a `tools/call` sent that way is never forwarded.

Key rotation: prepend a new `kid:key`, deploy, drop the old entry after `SESSION_MAX_AGE` plus one deploy. Unknown or tampered values fail closed.

Empty `ALLOWED_CLIENT_IDS` or `ALLOWED_REDIRECT_URIS` fails closed at startup.

## Security

MCP request metadata headers: `Mcp-Name` and `Mcp-Method` may appear once and hold at most 1024 bytes; `Mcp-Param-*` is capped at 32 headers of 4096 bytes each. A value in any `Mcp-*` header with a byte in 0x80–0xFF (obs-text) is rejected. These requests get `400` before any token exchange. The server sets `MaxHeaderBytes` to 192 KiB, enough for every `Mcp-Param-*` at its cap plus a bearer token. The gateway never decides anything on these headers; the JSON-RPC body is the truth. The `audit mcp` line sets `header_mismatch: true` when `Mcp-Method` or (for `tools/call`) `Mcp-Name` disagrees with the body.

The audit line logs `method` and `tool` as they are when they are short visible ASCII. Anything else (spaces, control characters, invisible characters such as U+200B) is logged Go-quoted and escaped, for example `"whoami "`, so a denied name never reads as a listed tool.

Accepted risk (owner decision 2026-11-17): consent is remembered for 30 days and loopback redirect URIs accept any port. Another process on the developer's machine can start a login as an allowlisted public client and get a token without a click, through browser SSO and the remembered consent. PKCE does not help, because the attacker starts the flow. This is the same threat model as public-client logins in the az and gh CLIs. Mitigations: tokens are scoped to fleet-gateway, sessions are capped at 12 hours, and tokens can be revoked.

DCR clients must register `grant_types` as `[authorization_code]` (the default) or `[authorization_code, refresh_token]`. Only the latter get a gateway refresh token.

## Development

```bash
mise run install  # Download Go modules
mise run lint     # Run golangci-lint
mise run test     # Run tests with -race
mise run check    # Run fmt, vet, staticcheck, lint, test
mise run build    # Build binary to bin/fleet-gateway
```

## References

- [MCP gateway plan](https://github.com/navikt/copilot/issues/2351)
