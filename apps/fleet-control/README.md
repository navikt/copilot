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
| `GET /internal/v1/agents` | yes | no |

Missing or invalid token, or a user token: 401. Valid app token from another caller: 403. Texas unreachable or failing: 503. None of them return data.

`accessPolicy.inbound` admits the same two apps at the network layer.

## Write API

Teams register and change servers themselves. The owning team of a server is its Nais namespace. A change goes through three steps:

1. A team member sends a draft: `PUT /admin/v1/servers/{namespace}/{name}/draft` with `registry` (a server.json entry), `route` (`target`, `upstream_url`, `enabled`, `tools`) or both. The route path is always `/mcp/{namespace}/{name}`. The answer holds `version` and `content_hash`. A newer draft replaces the pending one.
2. Another member of the same team reads it with `GET /admin/v1/servers/{namespace}/{name}` and approves it with `POST .../approve` and `{"version": N, "content_hash": "..."}`. The submitter cannot approve, and a hash that does not match the pending draft is refused. Approval publishes the version, and the next snapshot carries it.
3. To stop, one member is enough: `POST .../revoke` with `{"reason": "..."}`, or `POST .../rollback` back to the previously published version. Add `"block": true` to a revoke when the version must never return (for example leaked credentials); rollback skips blocked versions and versions that were never approved. Rollback keeps a revoked server revoked.

Every request needs a person's Entra token that my-copilot obtained on their behalf (OBO), with `azp` equal to `MY_COPILOT_CLIENT_ID`, a `NAVident` and a `preferred_username` (used for team membership). App tokens and guest accounts are refused: `idtyp` must be absent or `user`, and `acct` must be 0 when present; without `acct` the token needs `tid` equal to `AZURE_APP_TENANT_ID` and no `#EXT#` in `preferred_username`. Input is validated like the seed: names, target, upstream URL, tools and the registry entry. Every change writes a row to `audit_events` with the actor and the state before and after, in the same transaction as the change. Writes to one server lock its row, so concurrent approvals publish once.

| Answer | Meaning |
| --- | --- |
| 400 | Invalid name or body |
| 401 | Missing or invalid token, app token, or a token from another client |
| 403 | Not a member of the owning team, or approving your own draft |
| 404 | Unknown server |
| 409 | Not the pending version, wrong hash, wrong state, or registry name taken |
| 503 | Texas, team lookup or Postgres unavailable |

Not built yet: team lookup in the Nais teams API (static list for now), GitHub OIDC from the team's CI, team-level policy, the route owner's approval of another team's agent profile, rate limiting, and `force` for the shrink guard.

## Agent profiles (#2582)

An agent profile says which caller app may use which tools on which gateway routes, on behalf of a signed-in user. fleet-gateway does not enforce profiles yet; this slice stores and serves them.

- Identity: cluster, namespace and app of the caller, plus its Entra client id. A client id can be in only one verified live profile; unverified drafts reserve nothing. An identity verifier must match the client id to the declared app; without one (none exists yet) the profile stays unverified and never reaches fleet-gateway. `sts_subject` is reserved for the Nais STS identity and rejected until that issuer is chosen.
- Mode is `delegated` only. fleet-control rejects agent principals, in the API and in the schema.
- Grants list routes (`<namespace>/<name>`) and exact tool names. Every tool must be on the published route. Wildcards are rejected. `max_risk` is `read`.
- Quotas, DPIA owner and reference are required.
- A profile starts as `draft`. A person from each granted route's owning team approves that route with the draft id returned on create (or read with `GET /admin/v1/agents/{namespace}/{name}`, visible only to the owner team and the granted routes' teams; others get 404), and the author cannot approve. A second member of the owner team also approves, with the profile's namespace as `route` (styringsmodell v0.3). Approval re-checks that the route is still published with the granted tools. Once the owner team and every route have approved, the profile is `approved` and expires six months later. The owner team or a route owner can revoke it. Revocation is final. An expired profile is marked `expired` when a new profile needs its name or client id.
- `GET /internal/v1/agents` returns approved, verified profiles that have not expired, checked at read time.
- Every change writes an audit event.

The write routes (`POST /admin/v1/agents`, `.../{namespace}/{name}/approvals`, `.../revoke`) follow the Write API rules above: a person's token from the admin client, teams from `FLEET_CONTROL_STATIC_TEAMS`, dev-* clusters only. `SEED_AGENTS_FILE` imports drafts on startup in dev only (`seed/agents.dev.json`, the Aksel Studio draft). Its `entra_client_id` is the dev client id of `studio-api`. With no identity verifier yet, the profile stays unverified and out of the snapshot. The app refuses `SEED_AGENTS_FILE` outside dev-* clusters.

## Read API for my-copilot

The self-service pages in my-copilot read these routes (#2736). They take the same person's token as the write routes and show only what the caller's teams own:

- `GET /admin/v1/me`: the caller's NAVident and teams.
- `GET /admin/v1/servers` and `GET /admin/v1/agents`, optionally `?namespace=`: servers and live agent profiles owned by the caller's teams. Another team's namespace gives an empty list.
- `GET /admin/v1/servers/{namespace}/{name}/audit` and `GET /admin/v1/agents/{namespace}/{name}/audit`: the newest 500 audit events, for members of the owning team. Anyone else gets 404 whether or not the name exists.
- `GET /admin/v1/agents/pending-approval`: drafts with a granted route that the caller's teams own and have not approved yet.

A failed team lookup gives 503, never an unfiltered answer.

## Job API for fleet-runtime

fleet-runtime has no inbound surface. It fetches its work from fleet-control: it claims a job, holds a lease by heartbeat, and reads cancel and kill from the heartbeat answer (#2689). The Go types in `jobs.go` are the contract.

Teams queue jobs with the admin auth above (team member, dev only):

- `POST /admin/v1/jobs/{namespace}` with `kind`, `params` (a JSON object of at most 16 KiB: a reference to the work, not the work), `on_expiry` (`requeue` or `fail`, default `requeue`) and `max_attempts` (1 to 10, default 3).
- `POST /admin/v1/jobs/{namespace}/{id}/cancel` or `.../kill`. A queued job stops at once; a leased one gets the command on its next heartbeat.
- `PUT /admin/v1/jobs/{namespace}/kill-switch` with `{"scope": "team" | "global", "on": true, "reason": "..."}`. While a switch is on, no job in scope is claimed and every leased one gets kill on its next heartbeat. Only `FLEET_CONTROL_OPERATOR_TEAM` may set `global`.

fleet-runtime calls, with an Entra app token (`idtyp=app`) whose `azp` is in `FLEET_RUNTIME_CLIENT_IDS`:

| Route | Body | Answer |
| --- | --- | --- |
| `POST /runtime/v1/claim` | `claim_token` (16 to 128 of `[A-Za-z0-9._-]`) | 200 with `lease_id`, `lease_ttl_seconds`, `lease_expires_at` and one job, or 204 after waiting up to 25 s. The same token returns the same lease with its remaining TTL; 410 once that lease has ended |
| `POST /runtime/v1/heartbeat` | `lease_id` | 200 with `command` (`continue`, `cancel`, `kill`), `lease_ttl_seconds` and `lease_expires_at`. 410: the lease is gone |
| `POST /runtime/v1/complete`, `/fail` | `lease_id`, `code` (`[a-z0-9_]{1,64}`), `message` (at most 500 characters) | 204. A status, never the job's output |

The runtime's deadline is the monotonic time it sent the request plus `lease_ttl_seconds`, so time in transit only shortens the lease. `lease_expires_at` uses fleet-control's clock and is for information only. A heartbeat answer other than 200, a timeout, a network error, or an answer that arrives after the deadline is not an ack. Bound each heartbeat request by the time left. The runtime must stop the job on `kill`, on 410, and when the countdown reaches zero. Lost contact therefore stops the job on both sides. On `cancel` it stops and reports with `fail`. After the first cancel, heartbeats repeat it but never extend the lease past one TTL from that delivery. Then the lease expires and the sweeper marks the job cancelled. Kill ends the lease at once. The lease TTL is `FLEET_RUNTIME_LEASE_TTL` (default 60 s); heartbeat at a third of it. An expired lease returns its job to the queue (`requeue`, until `max_attempts`) or fails it. A background worker expires leases every `FLEET_RUNTIME_SWEEP_INTERVAL` (default 15 s), and each claim sweeps first too. The sweep uses `SKIP LOCKED`, so replicas sweeping at once never expire a lease twice, and writes one audit row per expiry.

Claims lock the job row with `FOR UPDATE SKIP LOCKED`, and a unique index allows one active lease per job, so concurrent claims get one winner. Claims, commands, deliveries, lease expiry, results and kill switches each write `audit_events` in the same transaction. A plain heartbeat that only extends the lease is not audited. The claim long-poll works because the server has no `WriteTimeout`; `ReadTimeout` covers reading the request only.

Not built yet: the public ingress fleet-runtime will reach this through (#2686), LISTEN/NOTIFY instead of polling every second, a four-eyes step for jobs, and a read API for job status.

## Configuration

| Variable | Meaning |
| --- | --- |
| `FLEET_GATEWAY_CLIENT_ID` | fleet-gateway's Entra client id in this cluster |
| `MCP_REGISTRY_CLIENT_ID` | mcp-registry's Entra client id in this cluster |
| `NAIS_TOKEN_INTROSPECTION_ENDPOINT` | Texas, set by Nais |
| `SEED_ROUTES_FILE`, `SEED_ALLOWLIST_FILE` | first-run import |
| `MY_COPILOT_CLIENT_ID` | my-copilot's Entra client id. Unset: no `/admin` routes |
| `FLEET_CONTROL_STATIC_TEAMS` | JSON of team to member emails, matched trimmed and case-insensitively against the token's `preferred_username`, until the Nais teams API is wired in. Read from the `fleet-control-static-teams` secret in dev-gcp; refused outside dev-*. Unset: nobody can write |
| `FLEET_RUNTIME_CLIENT_IDS` | fleet-runtime's Entra client ids, comma-separated. Unset: no `/runtime` routes |
| `FLEET_RUNTIME_LEASE_TTL` | Lease TTL, 10s to 10m. Default 60s |
| `FLEET_RUNTIME_SWEEP_INTERVAL` | Lease sweep interval, 1s to 5m. Default 15s |
| `FLEET_CONTROL_OPERATOR_TEAM` | Team that may set the global job kill switch. Unset: nobody |
| `NAIS_CLUSTER_NAME` | Set by Nais. The write routes and the static team list run only in a `dev-*` cluster; the static list in prod stops startup |

The app refuses to start when a client id or the Texas endpoint is missing, or when both client ids are equal. Client ids are set per cluster in `.nais/<cluster>.yaml`. A change rolls out pod by pod, so for a short time old and new replicas may accept different callers.

## Development

```sh
mise check
```
