# mcp-dolly

MCP server for Dolly, Nav's test data tool (`dolly-backend`, namespace `dolly`, `dev-gcp`). It runs in namespace `copilot` behind `fleet-gateway` and has no ingress.

`POST /api/v1/gruppe/{id}/bestilling/fraidenter` must never be used.

## Tools

Read tools call `dolly-backend` with `GET` only. `pageSize` defaults to 20, max 50.

- `dolly_current_user`: `GET /api/v1/bruker/current`
- `dolly_list_groups`: the caller's groups (`GET /api/v1/bruker/current`, then `GET /api/v1/gruppe?brukerId=…&pageNo=&pageSize=`)
- `dolly_get_group`: `GET /api/v1/gruppe/{gruppeId}/page/{pageNo}?pageSize=`
- `dolly_search`: `kind: gruppe` calls `GET /api/v1/gruppe/soekGruppe?fragment=`, `kind: bestilling` calls `GET /api/v1/bestilling/soekBestilling?fragment=` (fragment 1 to 100 characters)
- `dolly_get_order`: `GET /api/v1/bestilling/{bestillingId}`
- `dolly_list_group_orders`: `GET /api/v1/bestilling/gruppe/{gruppeId}?page=&pageSize=`

Write tool (on the prod gateway route behind the server-enforced two-step confirmation below; ROS in copilot-intern#146):

- `create_order`: two-step. The first call returns a preview and a single-use `confirmationToken` (5 min, HMAC-bound to the user, the exact parameters and the issuing process, per-pod seen-set) and orders nothing; only a second call with the token orders and charges quota. `POST /api/v1/gruppe/{gruppeId}/bestilling` (`RsDollyBestillingRequest`). Clients that declare form elicitation (MCP 2026-07-28) never see the token: the first call returns `input_required` with an `elicitation/create` form, the token travels in `requestState`, and the order is sent only when the retry carries `accept` with `confirm: true`. Those clients get no `confirmationToken` in `tools/list`, and a `confirmationToken` argument from them is refused. Clients without form elicitation get the two-step flow only when `MCP_DOLLY_CREATE_ORDER_TWO_STEP=true` (dev); otherwise (prod, default) `create_order` refuses them without issuing a token and names the supported clients. An unknown value fails startup, and startup logs the mode. Verifies group ownership (`opprettetAv.brukerId` against `/api/v1/bruker/current`), caps `antall` at 1 to 5, restricts `environments` to a subset of `q1`, `q2`, `q4`, enforces a per-user quota of 20 persons per rolling 24h in memory per pod keyed by hashed `NAVident`, and writes one audit log line per order without person data.

  State store: `MCP_DOLLY_STORE=memory` (default) keeps used tokens and the rolling 24 h quota in memory per pod. `MCP_DOLLY_STORE=postgres` with `MCP_DOLLY_DATABASE_URL` (or the `PGURL` Nais Postgres injects; Nais runs it in both clusters, see `.nais/postgres.yaml`) stores them in Postgres and counts the quota per UTC day; tables are created at first use under an advisory lock, expired tokens and quota rows older than yesterday are deleted every minute. Both modes refuse a confirmation when the user already has 20 unexpired used tokens. If the database cannot answer within 2 s, `create_order` refuses and orders nothing; read tools are unaffected. Postgres only shares state between replicas once the HMAC keys are shared too (#2584).

  The preview carries a `defaults` line (team Dolly, 2026-11-20): persons get FNR, name, gender, place of birth, date of birth, marital status, residential address, Norwegian citizenship and `personstatus=bosatt`; age 30-60 and even gender split unless set; children 0-17, parents 60-80; spouse within ±3 years; every 20th couple (5%) same-sex. Only `antall`, `environments`, `minAlder`, `maxAlder` and `kjoenn` can be set here; anything else needs the Dolly UI.

Upstream bodies are capped at 1 MiB and tool results at 128 KiB. Logs never contain response bodies, idents or tokens.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `DOLLY_URL` | `https://dolly-backend.ekstern.dev.nav.no` | Base URL for `dolly-backend` |
| `DOLLY_SCOPE` | — | Entra scope for Texas OBO (`api://dev-gcp.dolly.dolly-backend/.default`) |
| `CONFIRM_KEYS` | — | Required. `kid:base64,kid:base64`, newest first, each at least 32 bytes. Signs with the first key, verifies with any. From the secret `mcp-dolly-keys`. |
| `USER_HASH_KEY` | — | Required. Base64, at least 32 bytes. HMAC key for hashed `NAVident`. From `mcp-dolly-keys`. |
| `MCP_DOLLY_GENERATED_KEYS` | `false` | Local development only: generates both keys per process and logs a warning. Refused in `prod-*` and together with configured keys. |

Startup fails when a key is missing, too short, not base64 or has a bad or duplicate `kid`. Errors never contain key material.

### Rotating `CONFIRM_KEYS`

The first key signs and every key verifies, so a key placed later in the list is verify-only. Pods restart one at a time, so rotate in three rollouts:

1. Add the new key as verify-only: `CONFIRM_KEYS=k1:<old>,k2:<new>`. Roll out.
2. Promote it to signer: `CONFIRM_KEYS=k2:<new>,k1:<old>`. Roll out.
3. Wait at least 5 minutes (token lifetime) after the last pod signing with `k1` stopped, then remove it: `CONFIRM_KEYS=k2:<new>`. Roll out.

While the used-token set is in memory, tokens are also bound to the process that issued them, so a token never survives a restart.

Changing `USER_HASH_KEY` resets every user's quota count; rotate it only if it leaks.

`/ready` returns 503 until Texas and `DOLLY_SCOPE` are configured.

## Authentication

Same model as `mcp-kodeverk`: Texas introspection, `azp` must equal `$NAIS_CLUSTER_NAME:copilot:fleet-gateway`, `NAVident` is required and app tokens are rejected. Each tool call exchanges the user's token via Texas OBO for `DOLLY_SCOPE`. Both apps are in the `nav.no` tenant, so OBO works across clusters.

## Waiting on Dolly

`dolly-backend` must pre-authorize this app. Team Dolly adds to `accessPolicy.inbound.rules` in `apps/dolly-backend/config.yml`:

```yaml
- application: mcp-dolly
  namespace: copilot
  cluster: prod-gcp
```

Until then OBO fails and the gateway route `/mcp/copilot/dolly` stays `enabled: false`.
