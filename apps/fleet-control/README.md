# fleet-control

Single source of truth for the MCP registry and fleet-gateway routes (#2334). It serves a read-only internal API for fleet-gateway and mcp-registry, and a write API for teams. There is no ingress, and it runs in dev-gcp only.

## Access

Every `/internal/v1` request needs an Entra app token (client credentials) from an allowed caller. fleet-control validates it with Texas and maps `azp` to a caller by the full client id from config, never by name.

| Endpoint | fleet-gateway | mcp-registry |
| --- | --- | --- |
| `GET /internal/v1/servers` | no | yes |
| `GET /internal/v1/routes` | yes | no |
| `GET /internal/v1/snapshots/routes` | yes | no |
| `GET /internal/v1/snapshots/registry` | no | yes |

Missing or invalid token, or a user token: 401. Valid app token from another caller: 403. Texas unreachable or failing: 503. None of them return data.

`accessPolicy.inbound` admits the same two apps at the network layer.

## Write API

Teams register and change servers themselves. The owning team of a server is its Nais namespace. A change goes through three steps:

1. A team member sends a draft: `PUT /admin/v1/servers/{namespace}/{name}/draft` with `registry` (a server.json entry), `route` (`target`, `upstream_url`, `enabled`, `tools`) or both. The route path is always `/mcp/{namespace}/{name}`. The answer holds `version` and `content_hash`. A newer draft replaces the pending one.
2. Another member of the same team reads it with `GET /admin/v1/servers/{namespace}/{name}` and approves it with `POST .../approve` and `{"version": N, "content_hash": "..."}`. The submitter cannot approve, and a hash that does not match the pending draft is refused. Approval publishes the version, and the next snapshot carries it.
3. To stop, one member is enough: `POST .../revoke` with `{"reason": "..."}`, or `POST .../rollback` back to the previously published version. Rollback keeps a revoked server revoked.

Every request needs a person's Entra token that my-copilot obtained on their behalf (OBO), with `azp` equal to `MY_COPILOT_CLIENT_ID` and a `NAVident`. App tokens are refused. Input is validated like the seed: names, target, upstream URL, tools and the registry entry. Every change writes a row to `audit_events` with the actor and the state before and after, in the same transaction as the change. Writes to one server lock its row, so concurrent approvals publish once.

| Answer | Meaning |
| --- | --- |
| 400 | Invalid name or body |
| 401 | Missing or invalid token, app token, or a token from another client |
| 403 | Not a member of the owning team, or approving your own draft |
| 404 | Unknown server |
| 409 | Not the pending version, wrong hash, wrong state, or registry name taken |
| 503 | Texas, team lookup or Postgres unavailable |

Not built yet: team lookup in the Nais teams API (static list for now), GitHub OIDC from the team's CI, team-level policy, the route owner's approval of another team's agent profile, rate limiting, and `force` for the shrink guard.

## Configuration

| Variable | Meaning |
| --- | --- |
| `FLEET_GATEWAY_CLIENT_ID` | fleet-gateway's Entra client id in this cluster |
| `MCP_REGISTRY_CLIENT_ID` | mcp-registry's Entra client id in this cluster |
| `NAIS_TOKEN_INTROSPECTION_ENDPOINT` | Texas, set by Nais |
| `SEED_ROUTES_FILE`, `SEED_ALLOWLIST_FILE` | first-run import |
| `MY_COPILOT_CLIENT_ID` | my-copilot's Entra client id. Unset: no `/admin` routes |
| `FLEET_CONTROL_STATIC_TEAMS` | JSON of team to NAVidents, until the Nais teams API is wired in. Unset: nobody can write |

The app refuses to start when a client id or the Texas endpoint is missing, or when both client ids are equal. Client ids are set per cluster in `.nais/<cluster>.yaml`. A change rolls out pod by pod, so for a short time old and new replicas may accept different callers.

## Development

```sh
mise check
```
