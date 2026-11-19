# mcp-dolly

Read-only MCP server for Dolly, Nav's test data tool (`dolly-backend`, namespace `dolly`, `dev-gcp`). It runs in namespace `copilot` behind `fleet-gateway` and has no ingress.

The server only reads. Tools that create, change or delete test data are left out on purpose: Dolly has no access control on groups yet, so anyone can change anyone's groups. Write tools wait until Dolly has group access control or this server enforces ownership itself (#2457). `POST /api/v1/gruppe/{id}/bestilling/fraidenter` must never be used.

## Tools

All tools call `dolly-backend` with `GET` only. `pageSize` defaults to 20, max 50.

- `dolly_current_user` — `GET /api/v1/bruker/current`
- `dolly_list_groups` — the caller's groups: `GET /api/v1/bruker/current`, then `GET /api/v1/gruppe?brukerId=…&pageNo=&pageSize=`
- `dolly_get_group` — `GET /api/v1/gruppe/{gruppeId}/page/{pageNo}?pageSize=`
- `dolly_search` — `kind: gruppe` calls `GET /api/v1/gruppe/soekGruppe?fragment=`, `kind: bestilling` calls `GET /api/v1/bestilling/soekBestilling?fragment=` (fragment 1–100 characters)
- `dolly_get_order` — `GET /api/v1/bestilling/{bestillingId}`
- `dolly_list_group_orders` — `GET /api/v1/bestilling/gruppe/{gruppeId}?page=&pageSize=`

Upstream bodies are capped at 1 MiB and tool results at 128 KiB. Logs never contain response bodies, idents or tokens.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `DOLLY_URL` | `https://dolly-backend.ekstern.dev.nav.no` | Base URL for `dolly-backend` |
| `DOLLY_SCOPE` | — | Entra scope for Texas OBO (`api://dev-gcp.dolly.dolly-backend/.default`) |

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
