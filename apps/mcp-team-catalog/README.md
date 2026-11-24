# mcp-team-catalog

Read-only MCP server exposing Nav's Team Catalog API (`team-catalog-backend`, namespace `org`) behind `fleet-gateway`.

## Privacy and PII Projection

People are projected to `name` and `roles` only.

- Never exposed: `email`, `onLeave`, `startDate`, `endDate`, `stale`, `resourceType`, `teamPercent`, `navIdent`, `contactPersonIdent`, `teamOwnerIdent`, `ownerGroupNavidentList`, `contactAddresses`, `changeStamp`, or `links`.
- List, search, and membership tools omit nested `members` lists and return `memberCount` instead.
- Detail tools (`get_team`, `get_product_area`, `get_cluster`) return `members` with `name` and `roles` only.

## Tools

All tools are read-only (`readOnlyHint: true`, `openWorldHint: false`) and call `GET` endpoints only:

- `list_teams`: list teams with optional `productAreaId`, `clusterId`, `status` (default `ACTIVE`), substring `filter`, `limit` (default 25, max 100), `offset`.
- `search_teams`: search teams by name (`query` 3-64 characters, `limit` default 25, max 50).
- `get_team`: get metadata and minimized members (`name`, `roles`) for a team UUID.
- `get_teams_by_nais_team`: list teams linked to a Nais team slug (`naisTeam`, optional `status` default `ACTIVE`).
- `list_product_areas`: list product areas with optional `status` (default `ACTIVE`), `filter`, `limit` (default 50, max 100), `offset`.
- `get_product_area`: get metadata and minimized members (`name`, `roles`) for a product area UUID.
- `list_clusters`: list clusters with optional `status` (default `ACTIVE`), `filter`, `limit` (default 50, max 100), `offset`.
- `get_cluster`: get metadata and minimized members (`name`, `roles`) for a cluster UUID.
- `get_member_memberships`: get teams, product areas, and clusters for a `navIdent` (`[A-Za-z][0-9]{6}`), returning the person's `name` and their `roles` per unit without nested member lists.
- `get_nais_team`: get a Nais team by `teamId` or search Nais teams by `query` (3-64 characters, `limit` default 25, max 50).

All serialized tool payloads are capped at 32 KiB and set `truncated: true` if items are reduced to fit. When more items exist, the result reports how many remain and provides `nextOffset` in `structuredContent`.

Each tool declares an `outputSchema` and returns `structuredContent` (MCP 2026-07-28), plus the same JSON as a text block for older clients.

Catalog responses (`GET /team`, `GET /productarea`, `GET /cluster`, `GET /naisteam`) are cached in memory for 5 minutes (32 MiB cap). Detail responses with members (`GET /team/{id}`, `GET /productarea/{id}`, `GET /cluster/{id}`), person membership lookups (`GET /member/membership/{id}`) and error responses are never cached. Caller tokens are validated and exchanged via Texas OBO on every call.

## Environment Variables

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listen port |
| `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN`, or `ERROR` |
| `TEAM_CATALOG_URL` | `http://team-catalog-backend.org` | Base URL for `team-catalog-backend` |
| `TEAM_CATALOG_SCOPE` | — | Entra target scope for Texas OBO exchange (e.g. `api://dev-gcp.org.team-catalog-backend/.default`) |
| `NAIS_TOKEN_INTROSPECTION_ENDPOINT` | — | Texas introspection endpoint (injected by Nais) |
| `NAIS_TOKEN_EXCHANGE_ENDPOINT` | — | Texas token exchange endpoint (injected by Nais) |
| `AZURE_APP_PRE_AUTHORIZED_APPS` | — | Nais JSON used to resolve `fleet-gateway` client ID |
| `MCP_LEGACY_PROTOCOL_UNTIL` | — | Optional `YYYY-MM-DD` cutoff for initialize-era MCP clients |

## Authentication Chain

1. Inbound requests from `fleet-gateway` carry an Entra OBO bearer token.
2. `mcp-team-catalog` validates the token via Texas introspection (`NAIS_TOKEN_INTROSPECTION_ENDPOINT`), rejecting app tokens (`idtyp=app` or `access_as_application`), missing `NAVident`, or `azp` not matching `$NAIS_CLUSTER_NAME:copilot:fleet-gateway` from `AZURE_APP_PRE_AUTHORIZED_APPS`.
3. Each `tools/call` exchanges the inbound bearer token via Texas OBO (`NAIS_TOKEN_EXCHANGE_ENDPOINT`) for `TEAM_CATALOG_SCOPE` and calls `team-catalog-backend` with `Authorization: Bearer <obo_token>`, `Nav-Call-Id`, and `Nav-Consumer-Id: mcp-team-catalog`.
