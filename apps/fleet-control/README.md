# fleet-control

Single source of truth for the MCP registry and fleet-gateway routes (#2334). It serves a read-only internal API for fleet-gateway and mcp-registry, and a write API for teams. There is no ingress, and it runs in dev-gcp only.

## Access

Every `/internal/v1` request needs an Entra app token (client credentials) from an allowed caller. fleet-control validates it with Texas and maps `azp` to a caller by the full client id from config, never by name.

| Endpoint                              | fleet-gateway | mcp-registry |
| ------------------------------------- | ------------- | ------------ |
| `GET /internal/v1/snapshots/routes`   | yes           | no           |
| `GET /internal/v1/snapshots/registry` | no            | yes          |

Missing or invalid token, or a user token: 401. Valid app token from another caller: 403. Texas unreachable or failing: 503. None of them return data.

`accessPolicy.inbound` admits the same two apps at the network layer.

## Write API

Teams register and change servers through `/apis/fleet.nav.no/v1alpha1` (see below). Of `/admin/v1`, only the job kill switch is left (#2895); every other `/admin/v1` route answers 404 or 405.

Every request needs a person's Entra token that my-copilot obtained on their behalf (OBO), with `azp` equal to `MY_COPILOT_CLIENT_ID`, a `NAVident` and a `preferred_username` (used for team membership). App tokens and guest accounts are refused: `idtyp` must be absent or `user`, and `acct` must be 0 when present; without `acct` the token needs `tid` equal to `AZURE_APP_TENANT_ID` and no `#EXT#` in `preferred_username`. Input is validated like the seed: names, target, upstream URL, tools and the registry entry. A registry name belongs to the team (#2853): team `<ns>` may use `no.nav.<ns>/<name>` or `io.github.navikt/<ns>`. Other names, vendor names such as `com.microsoft/*` included, belong to the platform team (`FLEET_CONTROL_OPERATOR_TEAM`). Registry names are unique regardless of case. Names published before this rule stay; startup logs them as `published registry names outside their team's prefix`, and a rollback to one is refused. Every change writes a row to `audit_events` with the actor and the state before and after, in the same transaction as the change. Writes to one server lock its row, so concurrent approvals publish once.

| Answer | Meaning                                                                  |
| ------ | ------------------------------------------------------------------------ |
| 400    | Invalid name or body                                                     |
| 401    | Missing or invalid token, app token, or a token from another client      |
| 403    | Not a member of the owning team, or approving your own draft             |
| 404    | Unknown server                                                           |
| 409    | Not the pending version, wrong hash, wrong state, or registry name taken |
| 503    | Texas, team lookup or Postgres unavailable                               |

Not built yet: GitHub OIDC from the team's CI, team-level policy, the route owner's approval of another team's agent profile, rate limiting, and `force` for the shrink guard.

## Agent profiles (#2582)

An agent profile says which caller app may use which tools on which gateway routes, on behalf of a signed-in user. fleet-gateway does not enforce profiles yet; this slice stores and serves them.

- Identity: cluster, namespace and app of the caller, plus its Entra client id. A client id can be in only one verified live profile; unverified drafts reserve nothing. An identity verifier must match the client id to the declared app; without one (none exists yet) the profile stays unverified and never reaches fleet-gateway. `sts_subject` is reserved for the Nais STS identity and rejected until that issuer is chosen.
- Mode is `delegated` only. fleet-control rejects agent principals, in the API and in the schema.
- Grants list routes (`<namespace>/<name>`) and exact tool names. Every tool must be on the published route. Wildcards are rejected. `max_risk` is `read`.
- Quotas, DPIA owner and reference are required.
- A profile starts as `draft`. A person from each granted route's owning team approves that route with the draft id returned on create, and the author cannot approve. A second member of the owner team also approves, with the profile's namespace as `route` (styringsmodell v0.3). Approval re-checks that the route is still published with the granted tools. Once the owner team and every route have approved, the profile is `approved` and expires six months later. The owner team or a route owner can revoke it. Revocation is final. An expired profile is marked `expired` when a new profile needs its name or client id.
- Every change writes an audit event.

Agent profiles are written through `/apis/fleet.nav.no/v1alpha1/namespaces/{namespace}/agentprofiles` (#2748). The pre-v1alpha1 tables `agent_profiles` and `agent_profile_approvals` are dropped in migration `011` (#2895). `SEED_AGENTS_FILE` imports drafts on startup in dev only (`seed/agents.dev.json`, the Aksel Studio draft). Its `entra_client_id` is the dev client id of `studio-api`. With no identity verifier yet, the profile stays unverified and out of the snapshot. The app refuses `SEED_AGENTS_FILE` outside dev-* clusters.

## API v1alpha1 (#2748)

`/apis/fleet.nav.no/v1alpha1` follows the design in copilot-intern `fleet/api-v1alpha1.md` (v0.6). It is Kubernetes-shaped JSON over REST on Postgres: `MCPServer`, `AgentProfile` and `AgentJob` with `metadata`, `spec` and `status` (`metav1.Condition`), errors as `metav1.Status`.

- Writing a spec proposes it. `POST` and `PUT` make a new `generation` only when the spec changes; a label change keeps what is pending. `PUT` needs a fresh `metadata.resourceVersion`, otherwise 409.
- `spec` is always the published generation; the one awaiting approval is in `status.pending`, for the owner team only.
- Another person in the owner team approves with `generation` and `contentHash` (`POST .../approval`). The two-person rule compares the Entra `oid`, which is stored but never shown, except your own in `whoami`. A person token without `oid` gets 401. A seeded submitter, or one from before v1alpha1, cannot be approved: a person submits again.
- `contentHash` is `v1:sha256:` over the normalised spec (sets sorted) in RFC 8785 form. Hashes compare as whole strings. Agent profiles have one hash per grant, which the server's owner team approves.
- `revocation` (one person in the owner team or `FLEET_CONTROL_OPERATOR_TEAM`) is final for that generation; `rollback` never lifts it and refuses revoked or blocked generations and their content with 409. `block: true` stops the content from ever being submitted again.
- `mcpservercatalogentries` shows published, unrevoked servers to every signed-in person, with its own field list. The single entry also lists the live agent profiles with access (`status.consumers`, names only).
- `GET .../namespaces/{team}/apps` lists the team's Nais applications (name and environment) from the Nais Console API, for the registration wizard in `/fleet`. Members only (404 otherwise); 503 where no Console service account is bound.
- Non-members get 404 whether or not a resource exists. A failed team lookup is 503.

### AgentJob

`namespaces/{ns}/agentjobs` has list, `GET`, `POST` and `POST {name}/termination` (`{"mode": "Cancel"|"Kill", "reason": "..."}`). Input, output and `AwaitingInput` come in a later PR, and the kill switch stays on `PUT /admin/v1/jobs/{ns}/kill-switch`.

- **Orderers.** A person in `{ns}` orders through my-copilot and must name a published profile in `{ns}` in `spec.agentProfile`. A person can also order through another app (delegated): an OBO token whose `azp` is the `identity.entraClientID` of a published, verified, unexpired and unrevoked profile in `{ns}`. Otherwise the app gets 401, and 404 in another namespace. Delegated tokens reach only `agentjobs`. The app is `status.orderer`; the person is `status.createdBy`. An `oid` is never shown.
- **Preauthorize each app by hand.** A delegated app needs fleet-control's `accessPolicy.inbound` to list it, or Texas refuses its token. Texas checks only the audience, so this list is the only gate before fleet-control checks the agent profile. Add apps there one at a time after their agent profile is approved. None is listed yet.
- **Names are unique per orderer**: `(team, orderer_client_id, orderer_subject)`, where `('', '')` is a person through my-copilot and `(azp, oid)` a person through an app. Each caller sees and terminates only its own orderer's jobs. Jobs created before v1alpha1 have no name and show as `job-<id>`; clients may not choose that form (422).
- **Retries are safe.** The spec is compared after defaults are filled in (`onExpiry: Requeue`, `maxAttempts: 3`, or 1 when the profile has a grant above `Read`). Same spec gives 200 and the existing job, another spec 409. The name check runs before the kill switch, so a retry gets 200 while the switch is on and a new job 503. `maxAttempts` above 1 on a profile that may write is 422.
- Migration `007` adds the orderer columns with default `''`, so an image from before it keeps inserting jobs.

On every start fleet-control fills `spec` and `spec_hash` on published server versions that lack them.

## Audit identities and retention (#2763, #2762)

Each audit row keeps the NAVident in `actor` and the Entra oid in `actor_oid`, with `actor_kind` and `actor_client_id` beside them. The oid is the stable key if the token format changes. Migration 009 fills `actor_oid` on older rows where another table pairs the NAVident with exactly one oid.

Audit rows are kept for 90 days. Deletion is off by default: set `FLEET_AUDIT_RETENTION_DELETE=true` and each start deletes older rows with the owner role (`PG*`). The runtime role has no DELETE right. The append-only trigger lets a DELETE through only for rows older than 90 days, and only in a session that set `fleet.audit_retention = 'on'`. Export to a long-term store is not built yet, because the target is not decided.

Status does not read audit rows. It reads `resource_status`, which keeps the newest time per resource and action and the revoking role (`platform`, `owner`). A trigger on `audit_events` fills it on every insert, also from older images. It holds no NAVident, so retention never changes what a status shows.

## Snapshot versions (#2764)

`generation` comes from the `snapshots` table, so it only grows, also across restarts. `checked_at` is when the serving replica last confirmed the snapshot is the newest in the database. It stops moving when the replica serves from memory during a database outage.

Publish and rollback return 409 when the routes snapshot would pass 960 KiB, under fleet-gateway's 1 MiB limit, or when the writing team's routes would pass 64 KiB (#2735). Only the writing team's budget is checked, so one team cannot block the others.

`GET /internal/v1/snapshots/routes` and `/registry` carry `schema_version: 1`, and fleet-gateway and mcp-registry refuse other versions. v1alpha1 does not change them. Rules for later changes:

1. New optional fields do not raise `schema_version`. fleet-gateway ignores unknown fields; check mcp-registry before the first addition.
2. Changed or removed fields raise `schema_version`. fleet-control then serves both versions, chosen with `?schema_version=N` (default 1).
3. Deploy the gateway first with support for both versions. Remove the old version from fleet-control only when no gateway asks for it.
4. A consumer may start requiring an optional field only after every fleet-control replica serves it. `checked_at` (#2849) works this way: deploy fleet-control first, then fleet-gateway. Ship the field in one release and require it in a later one, so the gateway accepts the snapshot with and without it in between. #2861 required `checked_at` before dev fleet-control served it, and dev route sync failed for 14 hours on 24–25 Nov 2026 until #2910.
5. Agent profiles in the gateway become their own kind with their own `schema_version: 1`, not a change to `routes`.

## Job API for fleet-runtime

fleet-runtime has no inbound surface. It fetches its work from fleet-control: it claims a job, holds a lease by heartbeat, and reads cancel and kill from the heartbeat answer (#2689). The Go types in `jobs.go` are the contract.

Teams queue jobs through `/apis` `namespaces/{ns}/agentjobs`, which requires an agent profile. Cancel and kill go through `POST {name}/termination`: a queued job stops at once, a leased one gets the command on its next heartbeat. The kill switch keeps its own route, with the admin auth above (team member, dev only):

- `PUT /admin/v1/jobs/{namespace}/kill-switch` with `{"scope": "team" | "team:<name>" | "global", "on": true, "reason": "..."}`. While a switch is on, no job in scope is claimed or created, and every leased one gets kill on its next heartbeat. Only `FLEET_CONTROL_OPERATOR_TEAM` may set `global` or another team's switch (`team:<name>`). A switch the operator team set, only the operator team can turn off.

fleet-runtime calls, with an Entra app token (`idtyp=app`) whose `azp` is in `FLEET_RUNTIME_CLIENT_IDS`. A runtime claims only jobs of the teams listed for it:

| Route                                | Body                                                                       | Answer                                                                                                                                                                                                 |
| ------------------------------------ | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `POST /runtime/v1/claim`             | `claim_token` (16 to 128 of `[A-Za-z0-9._-]`)                              | 200 with `lease_id`, `lease_ttl_seconds`, `lease_expires_at` and one job, or 204 after waiting up to 25 s. The same token returns the same lease with its remaining TTL; 410 once that lease has ended |
| `POST /runtime/v1/heartbeat`         | `lease_id`                                                                 | 200 with `command` (`continue`, `cancel`, `kill`), `lease_ttl_seconds` and `lease_expires_at`. 410: the lease is gone                                                                                  |
| `POST /runtime/v1/complete`, `/fail` | `lease_id`, `code` (`[a-z0-9_]{1,64}`), `message` (at most 500 characters) | 204. A status, never the job's output                                                                                                                                                                  |

The runtime's deadline is the monotonic time it sent the request plus `lease_ttl_seconds`, so time in transit only shortens the lease. `lease_expires_at` uses fleet-control's clock and is for information only. A heartbeat answer other than 200, a timeout, a network error, or an answer that arrives after the deadline is not an ack. Bound each heartbeat request by the time left. The runtime must stop the job on `kill`, on 410, and when the countdown reaches zero. Lost contact therefore stops the job on both sides. On `cancel` it stops and reports with `fail`. After the first cancel, heartbeats repeat it but never extend the lease past one TTL from that delivery. Then the lease expires and the sweeper marks the job cancelled. Kill ends the lease at once. The lease TTL is `FLEET_RUNTIME_LEASE_TTL` (default 60 s); heartbeat at a third of it. An expired lease returns its job to the queue (`requeue`, until `max_attempts`) or fails it. A background worker expires leases every `FLEET_RUNTIME_SWEEP_INTERVAL` (default 15 s), and each claim sweeps first too. The sweep uses `SKIP LOCKED`, so replicas sweeping at once never expire a lease twice, and writes one audit row per expiry.

Claims lock the job row with `FOR UPDATE SKIP LOCKED`, and a unique index allows one active lease per job, so concurrent claims get one winner. Claims, commands, deliveries, lease expiry, results and kill switches each write `audit_events` in the same transaction. A plain heartbeat that only extends the lease is not audited. The claim long-poll works because the server has no `WriteTimeout`; `ReadTimeout` covers reading the request only.

Not built yet: the public ingress fleet-runtime will reach this through (#2686), LISTEN/NOTIFY instead of polling every second, a four-eyes step for jobs, and a read API for job status.

## Configuration

| Variable                                  | Meaning                                                                                                                                                                                                                                                                                                                                                                                     |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `FLEET_GATEWAY_CLIENT_ID`                 | fleet-gateway's Entra client id in this cluster                                                                                                                                                                                                                                                                                                                                             |
| `MCP_REGISTRY_CLIENT_ID`                  | mcp-registry's Entra client id in this cluster                                                                                                                                                                                                                                                                                                                                              |
| `NAIS_TOKEN_INTROSPECTION_ENDPOINT`       | Texas, set by Nais                                                                                                                                                                                                                                                                                                                                                                          |
| `SEED_ROUTES_FILE`, `SEED_ALLOWLIST_FILE` | first-run import                                                                                                                                                                                                                                                                                                                                                                            |
| `READWRITE_PG*`                           | The runtime role's connection, set by Nais. Missing in Nais (`NAIS_CLUSTER_NAME` set): startup fails instead of running as the owner role (#2854). Outside Nais, `DB_URL` is used                                                                                                                                                                                                           |
| `MY_COPILOT_CLIENT_ID`                    | my-copilot's Entra client id. Unset: no `/admin` routes                                                                                                                                                                                                                                                                                                                                     |
| `NAIS_SERVICE_ACCOUNT_TOKEN_PATH`         | Set by Nais when a service account is bound to the workload in Console. When set, team membership comes from the Nais Console GraphQL API (`user(email).teams`), keyed on the token's `preferred_username`, trimmed and lowercased. Answers are cached for one minute; errors are not cached. An unreachable API, a rejected or missing token, a GraphQL error or an empty answer gives 503 |
| `FLEET_CONTROL_STATIC_TEAMS`              | Dev fallback when no service account is bound: JSON of team to member emails, matched trimmed and case-insensitively against the token's `preferred_username`. Read from the `fleet-control-static-teams` secret in dev-gcp; refused outside dev-*. Unset: nobody can write                                                                                                                 |
| `FLEET_RUNTIME_CLIENT_IDS`                | fleet-runtime's Entra client ids and the teams each serves, comma-separated: `<client-id>=<team>[\|<team>...]`. An entry without teams stops startup. Unset: no `/runtime` routes                                                                                                                                                                                                           |
| `FLEET_RUNTIME_LEASE_TTL`                 | Lease TTL, 10s to 10m. Default 60s                                                                                                                                                                                                                                                                                                                                                          |
| `FLEET_RUNTIME_SWEEP_INTERVAL`            | Lease sweep interval, 1s to 5m. Default 15s                                                                                                                                                                                                                                                                                                                                                 |
| `FLEET_CONTROL_OPERATOR_TEAM`             | Team that may set the global job kill switch. Unset: nobody                                                                                                                                                                                                                                                                                                                                 |
| `NAIS_CLUSTER_NAME`                       | Set by Nais. The write routes and the static team list run only in a `dev-*` cluster; the static list in prod stops startup                                                                                                                                                                                                                                                                 |

The app refuses to start when a client id or the Texas endpoint is missing, or when both client ids are equal. Client ids are set per cluster in `.nais/<cluster>.yaml`. A change rolls out pod by pod, so for a short time old and new replicas may accept different callers.

## Migrations

Migrations run at startup under an advisory lock (`db.go`). During a rolling deploy, old and new replicas run against the same schema, so every migration must keep the previous image working:

- A migration only adds: new tables, nullable columns or columns with a default, new indexes, wider checks.
- A destructive change (drop or rename a column or table, `SET NOT NULL`, a narrower check) takes two releases. Release 1 stops the code from using the old shape. Release 2, after release 1 runs everywhere, removes it.
- An image from before the migration must still start, read and write. If it cannot, the migration is wrong, not the image.

Existing migrations, checked against the rule (#2761):

| Migration     | Verdict                                                                                                                                                                    |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 001, 003, 004 | New tables only                                                                                                                                                            |
| 002           | Drops `NOT NULL` and widens the status check (loosening). Adds `servers_published_status`, which old code already satisfied                                                |
| 005           | Additive; new columns nullable or defaulted                                                                                                                                |
| 006           | Revokes rights from the runtime role on `audit_events`. Old code never updated or deleted audit rows                                                                       |
| 007           | Defaulted columns; replaces the `(team, name)` unique index with a wider one (loosening)                                                                                   |
| 008           | New unique index on `lower(registry_name)`. Tightening: an old image that writes a case-only duplicate now gets an error instead of a second row. Intended (#2853)         |
| 010           | Adds `runtime_kill_switches.set_by_operator` with a default. An old image ignores it, so it can clear an operator-set switch (#2854)                                       |
| 011           | **Destructive, the approved exception (#2895).** Drops `agent_profiles`, `agent_profile_approvals` and `server_versions.content_hash` in one release, before prod has data |

### Rollback after 011

A rollback to an image from before migration `011` is no longer possible. That image reads `agent_profiles` and writes `content_hash`, which no longer exist, so it fails on agent profile reads and on every server draft. During the rolling deploy of `011` itself, old replicas fail the same way until they are replaced; this was accepted because fleet-control runs in dev only.

To undo `011`, restore the database from a backup taken before it (see "Backup and restore") together with the older image. Do not recreate the tables by hand.

`011` is the one approved exception to "a migration only adds" (owner decision 2026-11-24, #2895). The rule above applies again to every migration after it.

## Audit retention

`FLEET_AUDIT_RETENTION_DELETE=true` in all clusters: at startup the app deletes `audit_events` older than 90 days. Owner decision 2026-11-24: Loki is the archive. Until #2876 merges, the variable has no effect.

Every committed `audit_events` row is also written to stdout as one JSON line, `audit fleet-control`, so Loki keeps the trail after the database deletes it. The line is logged only after the transaction commits; a rolled-back write logs nothing. Fields, in the fleet-gateway `audit mcp` shape:

- `at`: the row's time
- `subject.type` (`user`, `application`, `system`), `subject.id` (NAVident), `subject.properties.oid`, `subject.properties.client_id`
- `action.name`
- `resource.type`, `resource.id` (`namespace/name`), `resource.properties.namespace`, `.name`, `.generation`
- `context.outcome`: always `committed`

`detail` and tokens are not logged. Find the writes by one person, or to one resource:

```logql
{service_name="fleet-control"} |= "audit fleet-control" | json | subject_id="Z123456"
{service_name="fleet-control"} |= "audit fleet-control" | json | resource_id="team-a/my-server"
```

## Alerts

The PrometheusRule `fleet-control-alerts` in `.nais/app.yaml` links each alert to a section here. Metrics are on `/metrics`, all prefixed `fleet_control_`.

### FleetControlDown

Severity critical. No pod reports `up` for 5 minutes. Gateways stay fail-static (see [While fleet-control is down](#while-fleet-control-is-down)). Run `kubectl -n copilot get pods -l app=fleet-control` and read the pod logs. Usual causes: Postgres unreachable at startup, a missing client id, or a failed migration.

### FleetControlHigh5xxRate

Severity warning. More than 10 % of responses are 5xx over 5 minutes, with at least 20 requests. Break down `fleet_control_http_requests_total` by `route` and `code`. The `route` label is the mux pattern. 503 from the snapshot or admin routes usually means the database or Console is down: check the two alerts below.

### FleetControlConsoleLookupsFailing

Severity warning. Three or more Nais Console team lookups failed in 10 minutes. Team checks fail closed, so writes answer 503. Break down `fleet_control_console_lookups_total` by `outcome` (`ok`, `error`, `503`) and look at `fleet_control_console_lookup_duration_seconds`. `503` points at Console itself; `error` can also be an expired service account token or a changed answer.

### FleetControlDBErrors

Severity warning. More than 5 failed statements in 10 minutes (`fleet_control_db_errors_total`, not counting cancelled requests or constraint violations). Check the Postgres instance in Nais Console and the pod logs. `fleet_control_audit_write_failures_total` counts failed `audit_events` inserts: the write rolls back with them.

### FleetControlSnapshotStale

Severity warning. No successful snapshot build of one kind for 10 minutes (`fleet_control_snapshot_last_success_timestamp_seconds`). Snapshots are built when a gateway or the registry polls. Search the logs for `snapshot rebuild failed`. A build refused by the fail-closed checks (for example a route over the team budget) needs the offending MCPServer fixed or revoked. `fleet_control_snapshot_build_duration_seconds` and `fleet_control_snapshot_bytes` show build time and size.

## Backup and restore

Nais Postgres ([doc.nais.io](https://doc.nais.io/persistence/postgres/reference/recovery-window/)) takes a nightly base backup and archives the write-ahead log continuously, into a bucket in the team's project. That gives point-in-time recovery to any second in the window. `.nais/postgres.yaml` sets `backup.retentionDays: 7` (Nais default 2, range 2 to 30). The file is applied by hand, not by CI: run `nais apply -t copilot -e <env> .nais/postgres.yaml` after changing it. Cost at 5Gi is about 5 x 8 x 0.02 = 0.80 EUR a month.

- RPO: seconds (WAL archive), within 7 days.
- RTO: clone time plus one deploy, about 5 minutes in the dev drill below.

### While fleet-control is down

fleet-gateway is fail-static: it keeps serving its last snapshot of routes and agent profiles and does not drop to an empty route set. Writes (publish, approve, revoke, jobs) fail until fleet-control is back. Do not restart or deploy fleet-gateway during the outage: a new pod has no snapshot, so its MCP routes answer 503 and `/ready` fails until fleet-control answers (#2797).

### Restore runbook

A restore never overwrites the source. It clones into a new database ([how-to](https://doc.nais.io/persistence/postgres/how-to/restore-point-in-time/)).

1. Pick the time just before the bad write, as RFC3339 (`2026-11-16T12:00:00Z`). Check the earliest restore point with `nais postgres get fleet-control --team copilot --environment <env>`.
2. Clone: `nais postgres clone fleet-control fleet-control-restore --team copilot --environment <env> --at "<time>"`. Confirm.
3. Wait until `nais postgres get fleet-control-restore --team copilot --environment <env>` shows Clone status `Completed` and Status `Ready`.
4. Inspect the clone as a personal user: row counts in `servers`, `agent_profile_resources`, `runtime_jobs`, and the latest `audit_events` time.
5. Stop writes: scale fleet-control to 0, or accept that writes made from here on are lost.
6. Change `uses.postgres[].name` in `.nais/app.yaml` to `fleet-control-restore`, open a PR and deploy. Startup runs the migrations (no-op on a clone) and re-applies the `audit_events` revokes.
7. Check `/ready`, then that fleet-gateway fetches a snapshot again: its snapshot age metric drops back.
8. Keep the source until the clone is verified and nothing uses it. Then delete it, or rename back in a later change.

The audit log in the clone ends at the restore time. Events after it exist only in Loki. Find them with `{service_name="fleet-control"} |= "audit fleet-control" | json` over the time since the restore point. Audit rows older than 90 days are pruned again at the next start.

### Restore drill in dev (done 2026-11-25)

Needs a human with write access to dev-gcp. Fill in and commit the result here. Done on 2026-11-25 (#2881) in two runs: a clone-and-verify run, then a repoint run that moved the dev app to a clone and back.

- [x] Date, who: 2026-11-25, an agent on the owner's authorization, dev-gcp only.
- [x] Note a test write and its time T: no write was made to the source. The app itself wrote `snapshots` routes generation 2 at 09:30:25Z, after the clone point, which serves as T.
- [x] Clone started (time): 09:34:33Z, `nais postgres clone fleet-control fleet-control-restore-drill` without `--at`. Clone point 09:24:33Z (latest point, 10 minutes back).
- [x] Clone `Completed` and `Ready` (time): 09:38:35Z (polled every 20 s). Clone duration: about 4 minutes.
- [x] fleet-control deployed against the clone, `/ready` (time): repoint run, clone 09:53:03Z to `Completed` 09:56:50Z (3 min 47 s). Repoint patched 09:57:13Z, rolled out and ready 09:57:49Z (36 s). The repoint used `kubectl patch application` on `spec.uses.postgres[0].name`, not a PR, because CI deploys only from main. Do that in dev only; in prod, follow step 6.
- [x] fleet-gateway snapshot age dropped (time): snapshot age 6 s after the repoint, 0 sync failures.
- [x] Data matches T (yes/no, what differed): yes. Row counts are equal in all 12 tables except `snapshots` (source 3, clone 2); the missing row is routes generation 2 from after the clone point. Migration version `011_drop_legacy.sql` in both. `audit_events` and `resource_status` ACLs are identical (only `app` may write), and the `CheckAuditLocked` query returns owner=false, change rights=false on both for a personal read user. Latest `audit_events` row 2026-11-23 in both.
- [x] Total RTO, step 2 to step 7: about 4 min 46 s (clone start to app ready on the clone), plus lost writes since the clone point: up to about 10 minutes without `--at`.
- [x] Switched back and clone deleted: repoint run switched back to the source in 52 s, snapshot age 18 s, 0 sync failures; that clone was deleted. First run: `nais postgres delete fleet-control-restore-drill` at 09:39:45Z; pods gone and the instance no longer in `nais postgres list` at 09:41:12Z. Source `fleet-control` still `Ready`.
- [x] Surprises or doc fixes: without `--at` the clone point is 10 minutes back, so writes from the last 10 minutes are missing. Pass `--at` for a recent point. `nais postgres psql --access-level read` is enough for step 4.

## Development

```sh
mise check
```
