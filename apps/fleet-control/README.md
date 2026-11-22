# fleet-control

Single source of truth for the MCP registry and fleet-gateway routes (#2334). This first slice serves a read-only internal API. There is no ingress, and it runs in dev-gcp only.

## Access

Every `/internal/v1` request needs an Entra app token (client credentials) from an allowed caller. fleet-control validates it with Texas and maps `azp` to a caller by the full client id from config, never by name.

| Endpoint | fleet-gateway | mcp-registry |
| --- | --- | --- |
| `GET /internal/v1/servers` | no | yes |
| `GET /internal/v1/routes` | yes | no |
| `GET /internal/v1/snapshots/routes` | yes | no |
| `GET /internal/v1/snapshots/registry` | yes | yes |

Missing or invalid token, or a user token: 401. Valid app token from another caller: 403. Texas unreachable or failing: 503. None of them return data.

`accessPolicy.inbound` admits the same two apps at the network layer.

## Configuration

| Variable | Meaning |
| --- | --- |
| `FLEET_GATEWAY_CLIENT_ID` | fleet-gateway's Entra client id in this cluster |
| `MCP_REGISTRY_CLIENT_ID` | mcp-registry's Entra client id in this cluster |
| `NAIS_TOKEN_INTROSPECTION_ENDPOINT` | Texas, set by Nais |
| `SEED_ROUTES_FILE`, `SEED_ALLOWLIST_FILE` | first-run import |

The app refuses to start when a client id or the Texas endpoint is missing, or when both client ids are equal. Client ids are set per cluster in `.nais/<cluster>.yaml`. A change rolls out pod by pod, so for a short time old and new replicas may accept different callers.

## Development

```sh
mise check
```
