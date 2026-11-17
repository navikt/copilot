# fleet-gateway

OAuth 2.1 authorization server fronting Microsoft Entra ID for Nav's internal MCP gateway (`nav-fleet`).

Step 1 implements the authorization server only: protected resource metadata, authorization server metadata, CIMD client verification, consent, and Entra authorization-code redemption. Downstream team MCP routing and Texas OBO exchange (`TeamTokenSource`) follow in step 2.

## Endpoints

- `GET /.well-known/oauth-protected-resource` — RFC 9728 Protected Resource Metadata
- `GET /.well-known/oauth-protected-resource/{resource}` — Resource-specific PRM (must match a configured pilot route)
- `GET /.well-known/oauth-authorization-server` — RFC 8414 Authorization Server Metadata
- `GET /authorize` — OAuth 2.1 authorization endpoint (CIMD allowlist, exact `redirect_uri`, PKCE `S256`, RFC 8707 `resource`, RFC 9207 `iss`)
- `POST /consent` — Consent submission with CSRF cookie check
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
| `ALLOWED_REDIRECT_URIS` | Yes | — | Comma-separated allowlist of exact `redirect_uri` values |
| `AZURE_APP_CLIENT_ID` | Yes | — | Nais-injected Entra client ID |
| `AZURE_APP_CLIENT_SECRET` | Yes | — | Nais-injected Entra client secret |
| `AZURE_OPENID_CONFIG_TOKEN_ENDPOINT` | Yes | — | Nais-injected Entra token endpoint |
| `AZURE_APP_WELL_KNOWN_URL` | Conditional | — | Nais-injected OpenID discovery URL (required unless `AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT` is set) |
| `AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT` | Conditional | — | Direct override for Entra authorization endpoint |
| `ENTRA_SCOPE` | No | `openid api://<own client id>/.default` (plus `offline_access` for `refresh`) | Scope requested from Entra |
| `COOKIE_SIGNING_SECRET` | No | Random 32 bytes | HMAC key (>= 32 bytes) for `__Host-fleet_entra_state` cookie |
| `PORT` | No | `8080` | HTTP listen port |
| `LOG_LEVEL` | No | `INFO` | `DEBUG`, `INFO`, `WARN`, or `ERROR` |

| `TEAM_TOKEN_STRATEGY` | No | `obo` | `obo` (Texas OBO) or `refresh` (Entra refresh token held in the session; needs `RT_STORAGE=server`) |
| `SESSION_STORE` | No | `memory` | `memory` (tests/local; lost on restart, not shared across replicas) or `postgres` |
| `DATABASE_URL` | No | — | Postgres URL override. If empty, pgx reads the libpq `PG*` variables that Nais Postgres injects (TLS client certs, PgBouncer transaction mode; simple query protocol, no session state). Migrations run with the unprefixed admin `PG*` credentials; the running store uses the `READWRITE_PG*` runtime credentials |
| `SESSION_MAX_AGE` | No | `12h` | Absolute session lifetime from login; refresh never extends it |
| `RT_STORAGE` | No | `client-encrypted` if `RT_KEYS` set, else none | `client-encrypted`: the Entra refresh token is sealed (AES-256-GCM) inside the gateway refresh token returned to the client; nothing is stored. `server`: refresh token stays in the session store, no client refresh token |
| `RT_KEYS` | With `postgres` or `client-encrypted` | — | `kid:base64key,...` (32-byte keys, newest first encrypts, all decrypt). Also seals every value in the session store |
| `GATEWAY_DISABLED` | No | `false` | Set to `true` to disable all `/mcp/*` routes (`503`, `Retry-After: 300`) |

Routes in `pilot-routes.yaml` (`PILOT_ROUTES_FILE`) support an optional `enabled: false` field to disable a single route (`503`, `Retry-After: 300`) while keeping it validated and registered.

Key rotation: prepend a new `kid:key`, deploy, drop the old entry after `SESSION_MAX_AGE` plus one deploy. Unknown or tampered values fail closed.

Empty `ALLOWED_CLIENT_IDS` or `ALLOWED_REDIRECT_URIS` fails closed at startup.

## Development

```bash
mise run install  # Download Go modules
mise run lint     # Run golangci-lint
mise run test     # Run tests with -race
mise run check    # Run fmt, vet, staticcheck, lint, test
mise run build    # Build binary to bin/fleet-gateway
```

## References

- [MCP gateway plan](../../docs/mcp-gateway-plan.md)
