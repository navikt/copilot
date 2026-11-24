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
- `POST /token` also accepts `grant_type=refresh_token` when `RT_STORAGE=client-encrypted` (single-use rotation; sliding: each refresh moves the idle deadline, `SESSION_ABSOLUTE_MAX_AGE` from the original login is a hard cap)
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
| `AZURE_OPENID_CONFIG_ISSUER` | Yes | — | Nais-injected Entra issuer; the `id_token` `iss` must match it |
| `AZURE_APP_WELL_KNOWN_URL` | Conditional | — | Nais-injected OpenID discovery URL (required unless `AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT` is set) |
| `AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT` | Conditional | — | Direct override for Entra authorization endpoint |
| `ENTRA_SCOPE` | No | `openid api://<own client id>/.default` (plus `offline_access` for `refresh`) | Scope requested from Entra |
| `COOKIE_SIGNING_SECRET` | No | HKDF subkey of the primary `RT_KEYS` key; random only outside Nais | HMAC key (>= 32 bytes) for the `__Host-fleet_entra_state` cookie. On Nais (`NAIS_CLUSTER_NAME` set) startup fails if neither this nor `RT_KEYS` is set |
| `PORT` | No | `8080` | HTTP listen port |
| `UPSTREAM_TIMEOUT` | No | `60s` | Time the MCP server gets to send response headers, and again to finish a non-SSE body. SSE uses `SSE_IDLE_TIMEOUT` and `SSE_MAX_DURATION` instead |
| `MCP_MAX_IN_FLIGHT` | No | `16` | Concurrent buffered `/mcp/*` requests per pod. Over the cap: `503` with `Retry-After: 1` |
| `MCP_MAX_IN_FLIGHT_PER_ROUTE` | No | `8` | Concurrent buffered requests per route, within `MCP_MAX_IN_FLIGHT`, so one slow upstream cannot hold every slot |
| `MCP_MAX_IN_FLIGHT_PER_USER` | No | `4` | Concurrent buffered `/mcp/*` requests per user per pod, within `MCP_MAX_IN_FLIGHT`. Over the cap: `503` with `Retry-After: 1` |
| `MCP_MAX_SSE` | No | `64` | Concurrent SSE response streams per pod, counted apart from `MCP_MAX_IN_FLIGHT` |
| `MCP_MAX_SSE_PER_USER` | No | `4` | Concurrent SSE response streams per user per pod, within `MCP_MAX_SSE`. Over the cap: `503` with `Retry-After: 1` |
| `METRICS_PORT` | No | `9090` | Port for `/metrics`. It has no ingress |
| `LOG_LEVEL` | No | `INFO` | `DEBUG`, `INFO`, `WARN`, or `ERROR` |

| `TEAM_TOKEN_STRATEGY` | No | `obo` | `obo` (Texas OBO) or `refresh` (Entra refresh token held in the session; needs `RT_STORAGE=server`) |
| `SESSION_STORE` | No | `memory` | `memory` (tests/local; lost on restart, not shared across replicas) or `postgres` |
| `DATABASE_URL` | No | — | Postgres URL override. If empty, pgx reads the libpq `PG*` variables that Nais Postgres injects (TLS client certs, PgBouncer transaction mode; simple query protocol, no session state). Migrations run with the unprefixed admin `PG*` credentials; the running store uses the `READWRITE_PG*` runtime credentials |
| `SESSION_MAX_AGE` | No | `12h` | Longest life of one access token session (Entra `expires_in` is capped at it) |
| `SESSION_ABSOLUTE_MAX_AGE` | No | `7d` | Hard cap from the original login (`iat` in the refresh token); refresh never extends it. Must be at least `SESSION_MAX_AGE`. Audit reason `session_max_age_expired`. An external host sets a shorter value (#2363) |
| `SESSION_IDLE_TIMEOUT` | No | `12h` | A refresh token dies this long after it was issued; each successful refresh issues a new one (sliding), never past the hard cap. Audit reason `session_idle_expired`. Only with `RT_STORAGE=client-encrypted`; `RT_STORAGE=server` sessions are capped by `SESSION_ABSOLUTE_MAX_AGE` only |
| `REFRESH_GRACE_WINDOW` | No | `30s` | How long the immediately previous refresh token may be reused. Reuse inside the window returns the same token pair; reuse after it revokes the whole token family |
| `CONSENT_TTL` | No | `720h` | How long a consent is remembered per NAVident, client_id and route. `/revoke` clears it |
| `RT_STORAGE` | No | `client-encrypted` if `RT_KEYS` set, else none | `client-encrypted`: the Entra refresh token is sealed (AES-256-GCM) inside the gateway refresh token returned to the client; nothing is stored. `server`: refresh token stays in the session store, no client refresh token |
| `RT_KEYS` | With `postgres` or `client-encrypted` | — | `kid:base64key,...` (32-byte keys, newest first encrypts, all decrypt). Also seals every value in the session store |
| `GATEWAY_DISABLED` | No | `false` | Set to `true` to disable all `/mcp/*` routes (`503`, `Retry-After: 300`) |

A route path is `/mcp/<namespace>/<name>`, and `<namespace>` must equal the namespace in `target` (for example `/mcp/copilot/onboarding`). The old `/mcp/<name>` paths answer `404` with the new URL in `new_url`; they are not forwarded.

Routes in `pilot-routes.<env>.yaml` (`PILOT_ROUTES_FILE`, required) support an optional `enabled: false` field to disable a single route (`503`, `Retry-After: 300`) while keeping it validated and registered.

`ROUTES_SOURCE` picks where routes come from: `git` (default, the file only) or `fleet-control` (#2641, dev only). With `fleet-control` the gateway never serves routes from the file (#2797): the file only pins the primary resource. Until the first valid snapshot from `FLEET_CONTROL_URL/internal/v1/snapshots/routes` arrives, every `/mcp/<namespace>/<name>` answers 503 and `/ready` fails, so a pod started while fleet-control or Texas is down takes no traffic. It polls every 30 seconds with `If-None-Match`, using an M2M token from Texas (`NAIS_TOKEN_ENDPOINT`, audience `FLEET_CONTROL_AUDIENCE`). A snapshot is validated like the file. If it is empty, invalid or from an older generation, or if fleet-control or Texas fails, a pod that already has a snapshot keeps it, and the gateway counts `fleet_gateway_route_sync_failures_total`.

In route files (`version: 2`) every route lists the tools it allows in `tools: [...]`; the gateway rejects any other `tools/call`, and any ambiguous JSON-RPC body (case-variant or duplicate keys, trailing data), with a JSON-RPC error and does not forward it. Only the operator sets `passAllTools: true` instead, for a route that forwards every call. `version: 1` files are rejected at startup with an error.

On every route, `passAllTools` included, a `GET` or `DELETE` with a body gets `400` and is logged as `deny`/`ambiguous`: MCP defines no body for them, so a `tools/call` sent that way is never forwarded.

The gateway authenticates the user and checks the tool against the route allowlist. It does not decide which users may use a team's server: the OBO token carries only the groups the team's app lists in `azure.application.claims.groups`, and the team's server checks them (same model as Wonderwall). Emergency stop: `nais app set env` with `GATEWAY_DISABLED=true` stops every route in about 35 s (34 s measured in dev, 2026-11-18); see the [runbook](docs/runbook.md).

Key rotation takes two deploys (append the new key last, then promote it) and keeps the old key for 30+ days. See the [runbook](docs/runbook.md). Unknown or tampered values fail closed.

Empty `ALLOWED_CLIENT_IDS` or `ALLOWED_REDIRECT_URIS` fails closed at startup.

Fetched CIMD documents are cached for 5 minutes; if a refresh fails with a transport, non-200 status, or decode error, the last valid document is served for up to 24 hours.

## Memory and limits

Worst case per buffered MCP call, at the size limits:

| Part | MiB |
|---|---|
| Response body, read in 32 KiB chunks (no `io.ReadAll` growth) | 10.03 |
| Request body (1 MiB) through `io.ReadAll`, which allocates about 2.25x the body (measured: 23.5 MB for 10 MiB), plus JSON parsing for tool policy | 4 |
| Connection and transport buffers | 0.1 |
| **Per call** | **~14.1** |

`MCP_MAX_IN_FLIGHT` = 16 calls: 16 x 14.1 = 226 MiB. `MCP_MAX_SSE` = 64 streams at about 48 KiB each (32 KiB read buffer plus connection buffers): 3 MiB. Go runtime, route config and session caches: about 30 MiB. Total: about 259 MiB.

Both clusters therefore run with a 384 Mi limit, a 128 Mi request and `GOMEMLIMIT=320MiB` (83 % of the limit), so the garbage collector works harder before the kernel kills the pod. The figures are arithmetic plus one allocation measurement, not a load test. Raise `MCP_MAX_IN_FLIGHT` or the body limits only together with `memory_limit` and `gomemlimit`; `TestMemoryBudgetFitsManifests` checks the sum against both manifests.

A request takes its in-flight slot after the bearer token is checked and before the body is read, so requests with invalid tokens get `401`, never a slot. A `GET`, or a request whose `Accept` admits `text/event-stream`, also reserves an `MCP_MAX_SSE` slot before the upstream call, so a `503` always means the tool did not run. A JSON reply hands the SSE slot back. An event stream hands back the in-flight slot and may stay open for `SSE_MAX_DURATION`. An event stream the client did not accept gets `502` with outcome `upstream_unexpected_sse`. Over either cap the gateway answers `503` with `Retry-After: 1`, writes an `audit mcp` line with outcome `over_capacity` or `sse_over_capacity`, and counts it in the metrics. A truncated upstream body, shorter than its `Content-Length`, gets `502` with outcome `stream_error`.

`WriteTimeout` is derived in code (`mcpWriteTimeout`): read timeout (10 s) + team-token exchange (10 s) + 2 x `UPSTREAM_TIMEOUT` (headers, then body) + 5 s margin. With the defaults that is 145 s.

## Metrics and alerts

Prometheus scrapes `/metrics` on port 9090. That port has no ingress, so the public host does not serve `/metrics`. No label carries a user identifier.

| Metric | Labels | What it shows |
|---|---|---|
| `fleet_gateway_http_requests_total` | `code`, `reason` | All responses except `/health` and `/ready`. `reason` is empty except on deliberate 503s: `disabled` (kill switch), `cap` (in-flight or SSE limit), `user_cap` (per-user in-flight limit), `user_sse_cap` (per-user SSE limit), `store_full` (store quota reached), `refresh_in_flight` (concurrent refresh of the same token). The 5xx alert counts only `reason=""` |
| `fleet_gateway_mcp_requests_total` | `route`, `outcome`, `decision`, `reason`, `tool` | MCP calls. `reason` is set on `deny` only; `tool` is the called tool if on the route list, else `other`. `outcome` matches the `audit mcp` log line, for example `ok`, `upstream_token_error`, `store_error` or `upstream_502` |
| `fleet_gateway_mcp_request_duration_seconds` | `route` | Time per MCP call, token exchange and upstream included. An SSE stream counts until it ends |
| `fleet_gateway_routes_loaded` | none | Routes in the current table |
| `fleet_gateway_route_sync_failures_total` | none | fleet-control snapshots not applied |
| `fleet_gateway_rate_limited_total` | `oauth_endpoint` | Requests rejected with `429` by the in-memory rate limiter on `authorize`, `token`, `revoke`, or `register` |

Useful panels in Grafana:

- Latency per route: `histogram_quantile(0.95, sum by (le, route) (rate(fleet_gateway_mcp_request_duration_seconds_bucket[5m])))`
- Failures per route: `sum by (route, outcome) (rate(fleet_gateway_mcp_requests_total{outcome!="ok"}[5m]))`

Alerts are in `.nais/app.yaml` and go to `#github-copilot-alerts`: no ready pods, more than 2 restarts in 15 minutes, more than 10 % 5xx (only with over 20 requests in 5 minutes), and more than 5 `upstream_token_error` in 10 minutes. An idle SSE stream that ends in `stream_error` does not trigger them, since its status 200 has already been sent. The alert series start at zero at startup, so the first burst counts. To test in dev, stop the app with `nais app stop fleet-gateway -t copilot -e dev-gcp` (it asks for confirmation in an interactive terminal) and restore it with `nais app set replicas fleet-gateway --min 1 --max 2 -t copilot -e dev-gcp`. `FleetGatewayNoReadyPods` fires after 2 minutes; in the 2026-11-18 drill it reached Slack after about 3.5 minutes (an upper bound). The team's alert channel in Nais Console is also `#github-copilot-alerts`, in both dev and prod (confirmed 2026-11-18). See the [runbook](docs/runbook.md).

## Audit record

Every request to an MCP route logs one JSON line with `msg` `audit mcp` in the OpenID AuthZEN Access Evaluation shape (owner decision 2026-11-18, #2342):

```json
{"msg":"audit mcp",
 "subject":{"type":"user","id":"Z123456","properties":{"client_id":"<oauth client>","actor":{"type":"oauth_client","id":"<oauth client>"},"issuer":"fleet-gateway","assurance":"session"}},
 "action":{"name":"tools/call","properties":{"tool":"search"}},
 "resource":{"type":"mcp_route","id":"copilot/onboarding","properties":{"backend":"prod-gcp:copilot:mcp-onboarding"}},
 "context":{"request_id":"<26 chars>","http_method":"POST","scan_incomplete":false,"header_mismatch":false,"protocol_version":"2026-07-28","mcp_headers":{"method":true,"name":true},"outcome":"ok","duration_ms":12},
 "decision":{"decision":true,"context":{"reason":"listed"}}}
```

`decision.decision` is `true` only when the gateway's own checks let the request through; `context.outcome` says what happened after that (for example `upstream_error`). `decision.context.reason` is the tool policy reason (`listed`, `pass_all`, `not_tools_call`, `not_listed`, `ambiguous`) or, for a request stopped earlier, the outcome (`unauthorized`, `disabled`, `bad_request`, ...). `context.scan_incomplete` is `true` when the body scan stopped before the end. `context.protocol_version` is the `MCP-Protocol-Version` header (capped at 32 bytes, `""` if absent) and `context.mcp_headers` says whether `Mcp-Method` and `Mcp-Name` were sent: evidence of which MCP revision clients use before the gateway enforces 2026-07-28. `subject.id` is the user the call is made for. `subject.properties.actor` is the agent or client acting for them, `issuer` is who vouched for the identity and `assurance` how (#2582). Today the issuer is always the gateway's own session and the actor is always the OAuth client, so `actor.id` equals `client_id`. `client_id` stays for existing queries. Both come from the validated session; the gateway never reads an actor from a request header. A request without a valid token has no `subject`: AuthZEN 1.0 requires `subject.id`, and there is no identity to put there. Tool arguments and tokens are never logged.

### agentgateway field names (transition)

Owner decision 2026-11-24: the record also carries agentgateway's access-log names as flat keys, with the same values. Both sets are written until **2027-01-31**; after that the AuthZEN fields marked below may be removed. Move queries to the new names now.

| AuthZEN field (old) | agentgateway name (new) |
| --- | --- |
| `subject.id` | `jwt.sub` (absent without a session, like `subject`) |
| `action.name` | `mcp.method.name` |
| `action.properties.tool` | `gen_ai.tool.name` (`mcp.tool.name` in agentgateway CEL) |
| `resource.id` | `route` |
| `context.request_id` | `request.id` |
| `context.http_method` | `http.method` |

The names come from agentgateway's access log (`crates/agentgateway/src/telemetry/log.rs`). The other fields have no agentgateway counterpart and stay as they are: `subject.properties.*`, `resource.properties.backend`, `context.outcome`, `context.duration_ms`, `context.scan_incomplete`, `context.header_mismatch`, `context.protocol_version`, `context.mcp_headers` and `decision`. The record has no HTTP status, so there is no `http.status`. Loki's `json` parser turns dots into underscores: query `jwt_sub`, `mcp_method_name`, `gen_ai_tool_name`. See the [runbook](docs/runbook.md) for queries.

The flat fields of earlier versions (`navident`, `route`, `method`, `tool`, `decision_reason`, `outcome`) are gone; no dashboard or alert in this repository read them.

## Security

MCP request metadata headers: `Mcp-Name` and `Mcp-Method` may appear once and hold at most 1024 bytes; `Mcp-Param-*` is capped at 32 headers of 4096 bytes each. A value in any `Mcp-*` header with a byte in 0x80–0xFF (obs-text) is rejected. These requests get `400` before any token exchange. The server sets `MaxHeaderBytes` to 192 KiB, enough for every `Mcp-Param-*` at its cap plus a bearer token. The gateway never decides anything on these headers; the JSON-RPC body is the truth. The `audit mcp` record sets `context.header_mismatch: true` when `Mcp-Method` or (for `tools/call`) `Mcp-Name` disagrees with the body.

The audit record logs `action.name` and `action.properties.tool` as they are when they are short visible ASCII. Anything else (spaces, control characters, invisible characters such as U+200B) is logged Go-quoted and escaped, for example `"whoami "`, so a denied name never reads as a listed tool.

Accepted risk (owner decision 2026-11-17): consent is remembered for 30 days and loopback redirect URIs accept any port. Another process on the developer's machine can start a login as an allowlisted public client and get a token without a click, through browser SSO and the remembered consent. PKCE does not help, because the attacker starts the flow. This is the same threat model as public-client logins in the az and gh CLIs. Mitigations: tokens are scoped to fleet-gateway, access tokens are capped at 12 hours and sessions at 7 days, and tokens can be revoked.

DCR clients must register `grant_types` as `[authorization_code]` (the default) or `[authorization_code, refresh_token]`. Only the latter get a gateway refresh token.

### Rate limits

Each unauthenticated endpoint, `/authorize`, `/token`, `/revoke`, and `/register`, has its own in-memory rate limiter. By default each limiter allows 30 requests per minute per IP via `RATE_LIMIT_REQUESTS`, 300 requests per minute globally via `RATE_LIMIT_GLOBAL_REQUESTS`, and up to 4096 IP buckets via `RATE_LIMIT_MAP_CAP`. Traffic to one endpoint cannot exhaust another endpoint's budget. When the IP table stays full after sweeping expired windows, a new IP evicts the bucket with the earliest reset time instead of failing closed, while the global ceiling still applies.

### Client IP

The rate-limit key is the last valid IP in `X-Forwarded-For`, which the Nais ingress appends, falling back to `RemoteAddr` when absent or unparseable. Earlier client-supplied entries are ignored. This is only safe because the pod is reachable only through the ingress, and is not yet verified on an external host (#2360).

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
