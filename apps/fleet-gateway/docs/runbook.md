# fleet-gateway runbook

For the on-call engineer. Cluster `prod-gcp` (dev: `dev-gcp`), namespace `copilot`, app `fleet-gateway`. Commands are read-only unless marked **break-glass**. Never read secrets (`kubectl get secret`, `nais secret`, `printenv`). Post-deploy verification: [t7-prod-test.md](t7-prod-test.md). Design and plans live in issues (#2317 for this page).

Route paths below are written `/mcp/<namespace>/<name>`.

## What it is

OAuth 2.1 authorization server in front of Microsoft Entra ID, plus a reverse proxy that forwards `/mcp/*` calls to team MCP servers with a Texas on-behalf-of token. Two replicas in prod, one to two in dev.

| Dependency | Used for | If it is down |
| --- | --- | --- |
| Entra (`login.microsoftonline.com`) | Login, refresh | Login and refresh fail. Existing access tokens keep working until they expire (Entra `expires_in`, capped at `SESSION_MAX_AGE`, 12 h; refresh slides the session up to `SESSION_ABSOLUTE_MAX_AGE`, 7 d from login) |
| Texas (Nais token exchange) | OBO token to the upstream | Every `/mcp/*` call fails |
| Nais Postgres (`fleet-gateway`) | Sessions, consent, DCR clients, counters | `fleet_gateway_store_up` = 0 and alert `FleetGatewayStoreDown`; `/ready` stays 200, see below |
| `RT_KEYS` secret `fleet-gateway-rt-keys` | Seals refresh tokens and every store value; also signs the Entra state cookie | Startup fails on Nais without it |
| Route file (`pilot-routes.<env>.yaml`, baked into the image) | Which routes exist, their tools and `enabled` | Startup fails on an invalid file |
| fleet-control and Texas (only with `ROUTES_SOURCE=fleet-control`) | The route snapshot | A pod with a snapshot keeps it. A new pod starts from the snapshot stored in Postgres (`fleet_gateway_routes_from_stored_snapshot` = 1). With no valid stored snapshot it serves no routes: `/mcp/*` 503 and `/ready` fails until the first valid snapshot. A route revoked after the stored snapshot was taken comes back if a pod restarts during a fleet-control outage: watch for `fleet_gateway_routes_from_stored_snapshot` = 1 and the warning log `route table loaded from stored snapshot, fleet-control unreachable`, which gives the snapshot's age |
| Upstreams (for example `mcp-onboarding`) | The MCP servers | Only that route fails |
| `github.com`, `opencode.ai`, `ki-utvikling.nav.no` | Serve the CIMD client documents | New logins for that client fail |

## Health and logs

- `GET /health`: liveness, always 200 while the process runs.
- `GET /ready`: pings Postgres with a 2 s timeout and sets `fleet_gateway_store_up`, but does not fail on it (#2893). 503 `not_ready` while shutting down, or with `ROUTES_SOURCE=fleet-control` before the first valid route snapshot, live or stored (`reason: no_route_snapshot`).

```sh
kubectl config use-context prod-gcp
NS=copilot
URL=https://fleet-gateway.intern.nav.no
kubectl -n $NS get pods -l app=fleet-gateway
curl -s -o /dev/null -w '%{http_code}\n' $URL/health $URL/ready
kubectl -n $NS logs -l app=fleet-gateway --prefix --tail=-1 --since=30m | grep '"audit '
```

Audit lines: `audit mcp` in OpenID AuthZEN shape (`subject.id` NAVident, `resource.id` route, `action.properties.tool`, `decision.decision`, `decision.context.reason`, `context.outcome`; see the README), `audit login`, `audit refresh`, `audit dcr`. A killed route logs `route disabled by kill switch`. Logs are ordinary stdout. Owner decision 2026-11-18: audit lines go to Loki; retention measured at 90 days or more, but the limit is not confirmed by the platform team. The record also carries agentgateway's names (`jwt.sub`, `mcp.method.name`, `gen_ai.tool.name`, `route`, `request.id`, `http.method`) until 2027-01-31, when the AuthZEN duplicates may be removed (README, "agentgateway field names"). Write new queries against the new names. Loki's `json` parser turns dots into underscores:

```logql
# Calls by one user
{service_name="fleet-gateway"} |= "audit mcp" | json | jwt_sub="Z123456"
# Denied tool calls per tool
sum by (gen_ai_tool_name) (count_over_time({service_name="fleet-gateway"} |= "audit mcp" | json | mcp_method_name="tools/call" | decision_decision="false" [1h]))
```

The stream selector (`service_name`) is not yet verified against the Nais Loki labels: check it after the drill and correct it here.

## Usage statistics

`hack/fleet-gateway-stats.sh [hours]` (default 24) prints prod counts from Loki and Mimir: HTTP 4xx/5xx by status, method and path; `audit refresh` and `audit login` by outcome and `client_id`; total requests by code. It prints counts only, never NAVident. Needs naisdevice. Metrics in Mimir (verified 2026-11-20): `fleet_gateway_http_requests_total`, `fleet_gateway_mcp_requests_total`, `fleet_gateway_mcp_request_duration_seconds` (histogram), `fleet_gateway_rate_limited_total`, `fleet_gateway_routes_loaded`. Loki returns at most 5000 lines per query, so the Loki counts are a floor on busy days.

## Kill switches

Both return `503` with `Retry-After: 300` for `/mcp/*`. The check runs before the bearer token is read, so unauthenticated and authenticated callers alike see 503. Login, `/token` and metadata stay up.

### Close one route

1. Set `enabled: false` on the route in `pilot-routes.<env>.yaml`. For a route registered in prod, also remove its entry from `apps/mcp-registry/allowlist.json`: `TestRegistryMatchesProdRoutes` (`registry_consistency_test.go`) fails when a registry entry has no enabled prod route. Also update the test that lists the route: closing a registered route breaks `TestServersListHandler_KodeverkTitle` in `apps/mcp-registry/handlers_test.go` (kodeverk), or whichever test lists the route you close.
2. PR, required checks, merge queue, build, deploy.
3. Confirm: `curl -s -o /dev/null -w '%{http_code}\n' -X POST $URL/mcp/<namespace>/<name>` prints `503`.

Time from merge to 503: **7 min 4 s** (time to first 503 seen at the ingress) in the prod drill of 2026-11-24 (route kodeverk, #2844 and #2845); decision to 503 was 18 min 9 s, of which 11 min 5 s was PR to merge. There is no faster per-route path. Watch the fleet-gateway build-deploy job after the merge and re-run it if it fails: a failed deploy extends the outage (see the drill below). For an urgent single-route stop use the global switch.

### Close everything

`GATEWAY_DISABLED` is read in `config.go` and is **not** in `.nais/app.yaml`. Break-glass without a merge, through the Nais API with team rights:

```sh
# break-glass: changes the cluster you name. Starts a rolling restart.
nais app set env fleet-gateway GATEWAY_DISABLED=true -t copilot -e <env>
nais app status fleet-gateway -t copilot -e <env>
```

Undo: `nais app set env fleet-gateway GATEWAY_DISABLED- -t copilot -e <env>`.

With RBAC on the deployment, kubectl does the same:

```sh
kubectl -n copilot set env deploy/fleet-gateway GATEWAY_DISABLED=true
kubectl -n copilot rollout status deploy/fleet-gateway --timeout=120s
kubectl -n copilot set env deploy/fleet-gateway GATEWAY_DISABLED-
```

Developer accounts have no `patch` on deployments in dev-gcp: `kubectl set env` fails with `requires container.deployments.update`. Use the Nais CLI.

A normal deploy overwrites the switch silently (measured in dev 2026-11-18: the switch was gone after the deploy job reran). Say in the incident channel that it is on and re-check after every deploy.

Time to 503 for the break-glass path, dev 2026-11-18: rollout complete in 28 s, every current pod returned `503` on `/mcp/*` at 34 s, with `/health`, `/ready` and PRM metadata still 200. It is a rollout, not an instant switch: the old pod serves until the new one is ready. Undo: rollout complete in 27 s, pod back to 401 at 30 s. The Nais CLI says the change is temporary and is overwritten on the next deploy; the rerun confirmed it.

## Rotate `RT_KEYS`

Format: `kid:base64key,...`, 32-byte keys. The first entry encrypts; all entries decrypt (`store_seal.go`, `parseKeyRing`). The Entra state cookie is signed with the first key's HKDF subkey and verified against every entry's. Only session lookup treats an unknown kid as missing (`store_gateway.go`, `getSessionFromStore`; the row is kept). A refresh token with an unknown kid returns `invalid_grant`. Consent, DCR client and MCP session reads with an unknown kid return an error, so callers see 503 until the record expires or is deleted.

A single deploy that prepends a new key is not safe: during the rollout a new pod seals with the new key while an old pod does not know it, so users are logged out. Use two deploys:

1. Create the new key and append it **last** (decrypt only): `old,new`. Update the secret `fleet-gateway-rt-keys` (human-created, owner action), then restart: `kubectl -n copilot rollout restart deploy/fleet-gateway`. Wait for rollout.
2. Promote: `new,old`. Update the secret, restart, wait for rollout.
3. Keep `old` until the longest-lived record sealed with it has expired: consent 30 days (`CONSENT_TTL`), DCR clients 30 to 180 days (reads do not re-seal). Only then remove it: `new`. Sessions (12 h) are not the limit.

Dropping `old` earlier has these effects: sessions and refresh tokens sealed with it fail (users log in again); consent records fail with 503 until they expire or are deleted from the `items` table; DCR clients fail with 503 and must re-register after their rows are deleted; old MCP sessions must be re-initialised. DCR is internal only and off on the future public host.

Whether the pods pick up a changed secret without a restart is unverified; restart explicitly.

## Log everyone out

Remove the current key from `RT_KEYS` (replace it with a fresh key, no old entry) and restart. All sealed values become unreadable. Sessions and refresh tokens fail and users log in again. Consent, DCR client and MCP session reads fail with 503 (see Rotate `RT_KEYS`) until those rows expire or are deleted from the `items` table; DCR clients must re-register. For a single user, see [Lock out one user](#lock-out-one-user) instead.

## Lock out one user

Use alongside blocking the user in Entra when one account must be cut off immediately without waiting for Entra expiry or refresh. Lock-out compares against the original login time, so it kills a session that refresh has kept alive; the `audit refresh` outcome is `user_revoked`. Without it a refreshing client lives up to 7 d (`SESSION_ABSOLUTE_MAX_AGE`) from login, idle sessions end after 12 h (`SESSION_IDLE_TIMEOUT`).

### Prerequisite: `LOCKOUT_KEY`

The gateway reads `LOCKOUT_KEY` (at least 32 random bytes, for example `openssl rand -base64 32`) from a Nais secret, created by the owner in Nais Console for `dev-gcp` and `prod-gcp` as secret `fleet-gateway-lockout-key`; after that, add `- secret: fleet-gateway-lockout-key` under `envFrom` in `.nais/app.yaml` (not done yet, because a missing secret stops the pod from starting). Never rotate or delete `LOCKOUT_KEY`: lockout records are keyed by it, and starting the gateway without it fails by design once any lockout record exists. It is separate from `RT_KEYS` so a global logout or `RT_KEYS` change does not lift lockouts. Without `LOCKOUT_KEY`, the `lockout` command refuses to run. Lockout records written before `LOCKOUT_KEY` existed (keyed from `RT_KEYS`) are not migrated; none exist in prod.

### Run and undo

```sh
kubectl exec -n copilot deploy/fleet-gateway -- /app/main lockout --operator <your-ident> <NAVident>
```

Effect:

- `/mcp/*` rejects the user's sessions with `401` and deletes the session row on lookup.
- `/token` rejects authorization codes (including new logins) and refresh tokens with `invalid_grant` (`session revoked`).
- Remembered consent is deleted lazily on the next login.
- Other users are unaffected.

The command writes the record and queues an audit entry in Postgres. Within about 30 s, the running gateway writes the `audit lockout` line (`outcome=ok`, `action=lock`, `user_key`, `operator`, `at`) to stdout / Loki, where you can grep for it as in [Health and logs](#health-and-logs); plaintext `NAVident` is never logged. The lockout lasts until undone (no expiry); also block the user in Entra if they should not regain access later.

To undo a lockout:

```sh
kubectl exec -n copilot deploy/fleet-gateway -- /app/main lockout --undo --operator <your-ident> <NAVident>
```

This lifts the block for new logins only and queues an `audit lockout` line with `action=undo`, logged by the gateway within about 30 s. Sessions, authorization codes, refresh tokens and remembered consent issued before the lockout stay invalid permanently. `kubectl exec` is the v1 mechanism; an operator API behind internal ingress is tracked in #2435.

## Block one client

Remove the `client_id` from `ALLOWED_CLIENT_IDS` in `.nais/<env>-gcp.yaml` and deploy. Access tokens already issued stay valid until they expire (up to 12 h); the proxy does not recheck the client. Pending authorizations for the removed ID can still issue tokens. DCR re-registration bypasses a client block: `/register` issues a fresh `client_id` without the allowlist. DCR is internal only (off on the future public host, owner decision); to stop it, set `DCR_ENABLED=false` or use the global switch. A DCR client can additionally be deleted from the `items` table by its plain key.

## Postgres outage or loss

- Outage: `/ready` stays 200 (#2893), so pods stay in the ingress and route from the in-memory snapshot. Anything that reads or writes the store (login, token refresh, sessions) fails until the database is back. `FleetGatewayStoreDown` fires after 2 min. The pods are not restarted by liveness. No HA is configured (`.nais/postgres.yaml`).
- Loss or corruption: recreate empty. **Warning:** this drops the `revoked` denylist while client-held refresh tokens stay decryptable with the unchanged `RT_KEYS`, so revoked or already-used tokens can pass replay checks again if Entra still accepts them. Rotate `RT_KEYS` (drop the old key, see above) whenever the database is lost. Everything in it is short-lived (sessions 12 h, consent 30 days, DCR clients up to 180 days). Users log in again; migrations run at startup. OpenCode users may need `opencode mcp logout <name>` first (unverified).
- Backups: Nais Postgres defaults (automated backups, retention) are **unverified**; none are required for this design.

## Hashed store keys rollout

The deploy that hashes the store keys for authorization codes, consent ids and Entra `state` (#2454) can fail one login per user during the rolling update: the login starts on a new pod and finishes on an old one, which cannot find the hashed row. The user logs in again. Nothing to do after the rollout. Remove the plaintext-key fallback in `store_gateway.go` (`shortcut:` comment) once a pending/code TTL has passed.

## Post-deploy smoke (three steps)

1. `curl -s -o /dev/null -w '%{http_code}\n' $URL/.well-known/oauth-protected-resource/mcp/<namespace>/<name>` prints `200`.
2. One login with your own user through a real client.
3. One tool call (`whoami`); an `audit mcp` line with `context.outcome` `ok` and `decision.decision` `true` appears. Full version: [t7-prod-test.md](t7-prod-test.md).

## Route sync failing

`FleetGatewayRouteSyncFailing` fires when route snapshot fetches fail for 15 minutes. `FleetGatewayRouteNeverSynced` fires when a pod with `routes_source` set has run 15 minutes without one successful sync. `FleetGatewayRouteSnapshotStale` cannot catch that case: its age series only appears after the first sync (#2637). Routing fails closed either way.

Search the pod logs for `fleet-control routes not updated`. A refused snapshot (for example `snapshot has no checked_at`) means a contract mismatch, not an outage.

Snapshot contract rule: for one release the gateway accepts the snapshot both with and without a changed field. fleet-control ships the new field first; the gateway requires it only in a later release. See "Snapshot versions" in `apps/fleet-control/README.md`. Incident: #2861 required `checked_at` before dev fleet-control served it, and dev route sync failed from 24 Nov 18:54 to 25 Nov 08:38 (1679 failures, no alert) until #2910.

## Switch prod routes to fleet-control (#2760)

Prod reads routes from `pilot-routes.prod.yaml` today. This section moves it to `ROUTES_SOURCE=fleet-control` and back. Rehearse the whole run in dev first and write the times below.

### Gates (all before the prod switch)

- [ ] The fleet-gateway ROS is revised for fleet-control as route source and approved by the owner.
- [ ] fleet-control is deployed to prod-gcp (#2855).
- [ ] The Nais Console service-account binding for fleet-control prod is done (#2855). Network rules and Entra are not enough: without the binding, snapshot reads work while every team lookup returns 503. Check it with a team lookup: `GET /apis/<group>/<version>/whoami` on fleet-control prod (my-copilot `/fleet` calls it) returns your teams, not 503. Once #2855 adds a service-account line to fleet-control's `/ready`, that line must show the binding.
- [ ] fleet-control prod lists fleet-gateway as inbound. The gateway side comes in prod step 3.

### Seeding

fleet-control seeds itself; no separate tool. The image carries `pilot-routes.prod.yaml` and `allowlist.json` (`apps/fleet-control/Dockerfile`), and `SEED_ROUTES_FILE` points at `pilot-routes.{{env}}.yaml`. `seedFromEnv` imports only when the database has no servers, so it never overwrites. `seed_test.go` parses both the dev and the prod file. After the first prod deploy, freeze `pilot-routes.prod.yaml`: route changes go through fleet-control. A change to the file after seeding does not reach the database.

### Shadow mode: compare before switching

With `ROUTES_SOURCE=git` and `ROUTES_SHADOW_SOURCE=fleet-control` the gateway also polls fleet-control every 30 s. It validates the snapshot by the same rules as a real switch, never serves it, and compares it with the Git table. Upstreams compare in their `.svc.cluster.local` form (#2856); the comparison covers route, team, target, upstream, `enabled`, tools and `passAllTools`.

- `fleet_gateway_routes_shadow_diff`: the number of routes that differ. No series until the first shadow snapshot.
- `fleet_gateway_routes_shadow_failures_total`: failed or rejected shadow fetches. A failure never changes what the gateway serves.
- Log line `fleet-control shadow diff` with the differing route paths, at the first snapshot and whenever the set changes.

Enable it with `routes_shadow_source: fleet-control`, `fleet_control_audience` and the `fleet-control` outbound rule in the environment file. The values must not be combined with `routes_source: fleet-control`; the gateway refuses to start.

### Dev rehearsal

Dev runs on fleet-control. Rehearse shadow mode, the switch and the rollback there. Use a stopwatch and write each time here.

- [ ] Shadow: set `routes_source: git` and `routes_shadow_source: fleet-control` in `.nais/dev-gcp.yaml`, deploy. Deployed: ____ First `fleet-control shadow diff` log: ____ Gauge value: ____ (if not 0, record the routes).
- [ ] Switch: set `routes_source: fleet-control` and remove `routes_shadow_source`, deploy. Merged: ____ `/ready` 200 on all pods: ____ `fleet_gateway_route_snapshot_age_seconds` under 120: ____
- [ ] Roll back: `nais app set env fleet-gateway ROUTES_SOURCE=git -t copilot -e dev-gcp`. Command: ____ Rollout complete: ____ Each pod answers on every route: ____ Duration: ____
- [ ] Back to fleet-control: re-run the dev deploy job, which restores the manifest value. Done: ____
- [ ] Post-deploy smoke on one route. Done: ____

### Prod steps

1. Check every gate above.
2. Deploy fleet-control prod. Its log shows `seed` with `imported` true and the server count. Time: ____
3. Preparation PR, deployed before the watch starts. In `.nais/prod-gcp.yaml` add the `fleet-control` outbound rule, `fleet_control_audience: "api://prod-gcp.copilot.fleet-control/.default"` and `routes_shadow_source: fleet-control`. Shadow mode cannot fetch without the rule and the audience. After the deploy, the first `fleet-control shadow diff` log line must appear on every pod; `fleet_gateway_routes_shadow_failures_total` growing means the rule or inbound is missing. Merged: ____ First shadow log: ____
4. Watch for 24 h: `fleet_gateway_routes_shadow_diff` stays at 0 on every pod and `fleet_gateway_routes_shadow_failures_total` does not grow. Any non-zero diff resets the clock. Start: ____ End: ____
5. Switch PR: replace `routes_shadow_source` with `routes_source: fleet-control`; the outbound rule and audience stay. Merge and watch the deploy job. Merged: ____ Rollout complete: ____
6. Verify: `/ready` 200 on every pod, snapshot age under 120 s, no `fleet-control routes not updated` in the logs, and the post-deploy smoke on each prod route (PRM 200, then one tool call). Done: ____

### Roll back to the Git file (under 5 min)

```sh
nais app set env fleet-gateway ROUTES_SOURCE=git -t copilot -e prod-gcp
kubectl --context prod-gcp -n copilot rollout status deploy/fleet-gateway
```

The global kill-switch drill took 56 s in prod with the same command, so expect about 1 min. The setting does not survive the next deploy: revert the PR from step 5 at once. While the file is the source, routes registered or changed in fleet-control after seeding are not served; they stay in fleet-control and come back at the next switch. Before switching again, run the diff and decide per difference whether the file or fleet-control is right.

## SLO

Proposed defaults (#2893), pending owner approval of the targets. Window: one calendar month.

| SLO | Target | Error budget |
| --- | --- | --- |
| Availability per route | 99.5 % | 0.5 % of requests, about 3.6 h of full outage per month |
| Latency per route, p95 | below 1 s | 5 % of requests may be slower |
| Gateway 5xx rate | below 1 % | 1 % of responses |

**Availability** reads `fleet_gateway_mcp_requests_total{route,outcome}`; it is the only request counter with a route label.

- Counts as bad (the gateway's own fault): `store_error`, `upstream_token_error`, `over_capacity`, `sse_over_capacity`. The capacity outcomes count because we set the capacity (decided 2026-11-25).
- Does not count: `upstream_error` and `upstream_unexpected_sse` (the MCP server failed), `disabled` (kill switch, left out of the total too), `stream_error` (200 already sent), client errors (`unauthorized`, `bad_request`, `tool_denied`, `user_cap`, `user_sse_cap`, `payload_too_large`, `response_too_large`, `upstream_access_missing` and the other 4xx outcomes).

```promql
1 - (
  sum by (route) (increase(fleet_gateway_mcp_requests_total{namespace="copilot", outcome=~"store_error|upstream_token_error|over_capacity|sse_over_capacity"}[30d]))
  / sum by (route) (increase(fleet_gateway_mcp_requests_total{namespace="copilot", outcome!="disabled"}[30d]))
)
```

**Latency**: end-to-end p95 under 1 s for normal tool calls, SSE streams excluded (decided 2026-11-25). `fleet_gateway_mcp_request_duration_seconds` has only a `route` label and times SSE streams until they end, so it cannot exclude them yet. Until #2973 adds a label for that, this SLO is documented here but neither measured nor alerted on. The query once the label exists (label name to be settled in #2973):

```promql
histogram_quantile(0.95, sum by (route, le) (rate(fleet_gateway_mcp_request_duration_seconds_bucket{namespace="copilot", kind!="sse"}[30d])))
```

**5xx rate** reads `fleet_gateway_http_requests_total{code,reason}`. It has no route label, so it is gateway-wide. Deliberate 503s carry a non-empty `reason` (kill switch, caps, store quota, concurrent refresh) and do not count. Upstream 502s cannot be told apart in this counter and do count.

```promql
sum(increase(fleet_gateway_http_requests_total{namespace="copilot", code=~"5..", reason=""}[30d]))
  / sum(increase(fleet_gateway_http_requests_total{namespace="copilot"}[30d]))
```

### Availability burn rate

Two alerts on the availability SLO, both per route and both requiring at least 20 requests in the short window:

- `FleetGatewayAvailabilityBurnFast`: error ratio above 7.2 % (14.4 times budget) over both 1 h and 5 min. At that rate 2 % of the month's budget is gone in an hour.
- `FleetGatewayAvailabilityBurnSlow`: error ratio above 3 % (6 times budget) over both 6 h and 30 min. 5 % of the budget is gone in six hours.

What to do: break `fleet_gateway_mcp_requests_total` down by `outcome` for the route in the alert. `store_error` means Postgres (see "Postgres outage or loss"). `upstream_token_error` means token exchange with Entra ID fails; search pod logs for `audit mcp` lines with that outcome. `over_capacity` or `sse_over_capacity` means the in-flight cap is hit; look for a slow upstream holding requests.

## Contacts

- Owner: team copilot (placeholder: on-call rota and phone, to be filled in).
- Slack: #ki-produktutvikling-diskusjon.
- Entra and Texas: Nais platform support (placeholder: channel).
- CIMD documents: `github.com` and `opencode.ai` are third parties; no contact.

## Kill-switch drill

Run in dev first (`kubectl config use-context dev-gcp`, `URL=https://fleet-gateway.intern.dev.nav.no`), then prod in a quiet window with the pilot team told. Use a stopwatch; write each time below and in #2317.

- [x] Baseline (prod, 2026-11-24, 15:51:24Z): kodeverk 401, `/ready` 200.
- [ ] Route: set `enabled: false` on a dev route, open PR. Same-repo PRs deploy to dev before merge, so the route may already be 503. Start the clock at the shutdown decision (PR opened), not at merge. Stop when the rollout is complete, no old replica serves, and each current pod returns `503` when probed directly (`kubectl port-forward`). Record decision-to-503 and merge-to-503. Prod (kodeverk, #2844), time to first 503 seen at the ingress; rollout complete and the per-pod direct probe not measured in prod: decision (PR opened) 15:52:26Z, merged 16:03:31Z (11 min 5 s), first 503 at 16:10:35Z. Merge-to-503 7 min 4 s, decision-to-503 18 min 9 s. The other 3 prod routes stayed 401.
- [x] Re-enable via PR (prod, #2845 revert merged 16:10:48Z): first 401 at 16:42:33Z, 31 min 45 s. Slow because the fleet-gateway build-deploy failed at the Docker build step and needed a re-run. Expect about 7 min when the deploy succeeds. Lesson: watch the deploy job and re-run it if it fails, because a failed deploy extends the outage.
- [x] Global (dev, 2026-11-18, `nais app set env`): 34 s to every pod 503. Start at the command, stop only after the rollout completes and each current pod returns `503` when probed directly; repeated ingress requests do not show which replica answered. Time: 34 s. Prod (2026-11-24, `nais app set env ... GATEWAY_DISABLED=true -e prod-gcp` at 16:45:08Z): prod: 56 s to first 503 seen at the ingress on all 4 routes, rollout complete at 56 s (`kubectl rollout status`), per-pod direct probe not measured in prod, `/ready` and metadata stayed 200.
- [x] Confirm `/health`, `/ready`, metadata and login still respond.
- [x] Deploy with the switch set (dev rerun of the deploy job only): the switch did not survive; the pod came back serving 401.
- [x] Undo with `GATEWAY_DISABLED-`: 30 s to recovery. Prod (undo at 16:46:11Z): 52 s to first 401 seen at the ingress, env var confirmed removed.
- [ ] Record whether a changed `RT_KEYS` secret needs a restart.
- [ ] Copy the numbers into this page and close the measurement half of #2294.

## Alert drill

Run in dev only. `nais app stop` and `nais app set replicas` ask for interactive confirmation and refuse without a terminal, so the owner runs them. Developer accounts cannot scale with kubectl, and the Alertmanager is not reachable with developer rights (no `pods/portforward` in `nais-system`).

Alerts go to `#github-copilot-alerts`. Nais Console sets the channel per environment; for team copilot it is `#github-copilot-alerts` in both dev and prod (confirmed 2026-11-18). If alerts stop arriving, check the Console setting first.

- [x] `nais app stop fleet-gateway -t copilot -e dev-gcp` (dev-gcp, 2026-11-18, 19:43:39 UTC).
- [x] `FleetGatewayNoReadyPods` reached Slack at 19:47 UTC, about 3.5 min after the stop. The team's dev alert channel was changed in Nais Console during the drill, so treat 3.5 min as an upper bound.
- [x] `nais app set replicas fleet-gateway --min 1 --max 2 -t copilot -e dev-gcp`; `/ready` returned 200 at 19:48:18 UTC.
- [x] Alert resolve time: RESOLVED reached Slack at 19:52 UTC (2026-11-18), about 4 min after restore (`/ready` 200 at 19:48:18 UTC).
