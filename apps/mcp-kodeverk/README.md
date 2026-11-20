# mcp-kodeverk

Read-only MCP server exposing Nav's Felles kodeverk API (`kodeverk-api`, namespace `team-rocket`) behind `fleet-gateway`.

## Tools

All tools are read-only (`readOnlyHint: true`, `openWorldHint: false`) and use `GET /api/v1/**` endpoints only:

- `list_kodeverk` — list published kodeverk names with optional substring `filter`, `limit` (default 50, max 200), `offset`.
- `get_kodeverk_koder` — list published code keys for a `kodeverksnavn` with optional `filter`, `limit` (default 50, max 200), `offset`.
- `get_kodeverk_betydninger` — fetch code meanings (`betydninger`) with optional exact `kode`, `ekskluderUgyldige` (default `true`), `oppslagsdato` (`YYYY-MM-DD`), `spraak` (up to 5 `^[a-z]{2}$` codes), paginated over sorted codes (`limit` default 50, max 100; `offset`).
- `list_hierarkier` — list published hierarchy names with optional `filter`, `limit` (default 50, max 200), `offset`.
- `get_hierarki_noder` — fetch hierarchy node tree with optional `spraak`, `maxDepth` (default 3, max 10), and root node `limit` (default 50, max 100).
- `search_kodeverk` — search kodeverk names (when `kodeverksnavn` is omitted) or codes and Norwegian Bokmål (`nb`) terms within a kodeverk (`query` 2–64 characters, `limit` default 25, max 50).

All serialized tool payloads are capped at 32 KiB and set `truncated: true` if items are reduced to fit. When more items exist, the result says how many and gives the next `offset` (also as `nextOffset` in `structuredContent`).

Each tool declares an `outputSchema` and returns `structuredContent` (MCP 2026-07-28), plus the same JSON as a text block for older clients.

Upstream responses are cached in memory for 10 minutes, bounded to 32 MiB and shared across users: kodeverk-api returns the same reference data to every caller. Tokens are still validated and exchanged on every call. Errors are not cached.

## Environment Variables

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listen port |
| `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN`, or `ERROR` |
| `KODEVERK_URL` | `http://kodeverk-api.team-rocket` | Base URL for `kodeverk-api` |
| `KODEVERK_SCOPE` | — | Entra target scope for Texas OBO exchange (e.g. `api://prod-gcp.team-rocket.kodeverk-api/.default`) |
| `NAIS_TOKEN_INTROSPECTION_ENDPOINT` | — | Texas introspection endpoint (injected by Nais) |
| `NAIS_TOKEN_EXCHANGE_ENDPOINT` | — | Texas token exchange endpoint (injected by Nais) |
| `AZURE_APP_PRE_AUTHORIZED_APPS` | — | Nais JSON used to resolve `fleet-gateway` client ID |
| `MCP_LEGACY_PROTOCOL_UNTIL` | — | Optional `YYYY-MM-DD` cutoff for initialize-era MCP clients |

## Authentication Chain

1. Inbound requests from `fleet-gateway` carry an Entra OBO bearer token.
2. `mcp-kodeverk` validates the token via Texas introspection (`NAIS_TOKEN_INTROSPECTION_ENDPOINT`), rejecting app tokens (`idtyp=app` or `access_as_application`), missing `NAVident`, or `azp` not matching `$NAIS_CLUSTER_NAME:copilot:fleet-gateway` from `AZURE_APP_PRE_AUTHORIZED_APPS`.
3. Each `tools/call` exchanges the inbound bearer token via Texas OBO (`NAIS_TOKEN_EXCHANGE_ENDPOINT`) for `KODEVERK_SCOPE` and calls `kodeverk-api` with `Authorization: Bearer <obo_token>`, `Nav-Call-Id`, and `Nav-Consumer-Id: mcp-kodeverk`.

## Dev-GCP Tenant Caveat

`kodeverk-api` in `dev-gcp` uses the `trygdeetaten.no` Entra tenant, whereas `mcp-kodeverk` (and `fleet-gateway`) uses `nav.no`. Because Entra OBO cannot cross tenants, Texas OBO token exchange against `kodeverk-api` in `dev-gcp` is expected to fail unless both apps share the same tenant. For this reason, the `/mcp/copilot/kodeverk` route in `apps/fleet-gateway/pilot-routes.dev.yaml` is disabled (`enabled: false`).
