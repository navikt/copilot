# fleet-gateway runbook

For the on-call engineer. Cluster `prod-gcp` (dev: `dev-gcp`), namespace `copilot`, app `fleet-gateway`. Commands are read-only unless marked **break-glass**. Never read secrets (`kubectl get secret`, `nais secret`, `printenv`). Post-deploy verification: [t7-prod-test.md](t7-prod-test.md). Design and plans live in issues (#2317 for this page).

Route paths below are written `/mcp/<namespace>/<name>`.

## What it is

OAuth 2.1 authorization server in front of Microsoft Entra ID, plus a reverse proxy that forwards `/mcp/*` calls to team MCP servers with a Texas on-behalf-of token. Two replicas in prod, one to two in dev.

| Dependency | Used for | If it is down |
| --- | --- | --- |
| Entra (`login.microsoftonline.com`) | Login, refresh | Login and refresh fail. Existing access tokens keep working until they expire (Entra `expires_in`, capped at `SESSION_MAX_AGE`, 12 h) |
| Texas (Nais token exchange) | OBO token to the upstream | Every `/mcp/*` call fails |
| Nais Postgres (`fleet-gateway`) | Sessions, consent, DCR clients, counters | `/ready` fails, see below |
| `RT_KEYS` secret `fleet-gateway-rt-keys` | Seals refresh tokens and every store value; also signs the Entra state cookie | Startup fails on Nais without it |
| Route file (`pilot-routes.<env>.yaml`, baked into the image) | Which routes exist, their tools and `enabled` | Startup fails on an invalid file |
| Upstreams (for example `mcp-onboarding`) | The MCP servers | Only that route fails |
| `github.com`, `opencode.ai`, `ki-utvikling.nav.no` | Serve the CIMD client documents | New logins for that client fail |

## Health and logs

- `GET /health`: liveness, always 200 while the process runs.
- `GET /ready`: pings Postgres with a 2 s timeout. 503 `not_ready` when the database is unreachable.

```sh
kubectl config use-context prod-gcp
NS=copilot
URL=https://fleet-gateway.intern.nav.no
kubectl -n $NS get pods -l app=fleet-gateway
curl -s -o /dev/null -w '%{http_code}\n' $URL/health $URL/ready
kubectl -n $NS logs -l app=fleet-gateway --prefix --tail=-1 --since=30m | grep '"audit '
```

Audit lines: `audit mcp` in OpenID AuthZEN shape (`subject.id` NAVident, `resource.id` route, `action.properties.tool`, `decision.decision`, `decision.context.reason`, `context.outcome`; see the README), `audit login`, `audit refresh`, `audit dcr`. A killed route logs `route disabled by kill switch`. Logs are ordinary stdout. Owner decision 2026-11-18: audit lines go to Loki; retention measured at 90 days or more, but the limit is not confirmed by the platform team. A saved query for Grafana Loki or Kibana is not yet recorded: add it here after the drill.

## Kill switches

Both return `503` with `Retry-After: 300` for `/mcp/*`. The check runs before the bearer token is read, so unauthenticated and authenticated callers alike see 503. Login, `/token` and metadata stay up.

### Close one route

1. Set `enabled: false` on the route in `pilot-routes.<env>.yaml`. For a route registered in prod, also remove its entry from `apps/mcp-registry/allowlist.json`: `TestRegistryMatchesProdRoutes` (`registry_consistency_test.go`) fails when a registry entry has no enabled prod route.
2. PR, required checks, merge queue, build, deploy.
3. Confirm: `curl -s -o /dev/null -w '%{http_code}\n' -X POST $URL/mcp/<namespace>/<name>` prints `503`.

Time from merge to 503: **unmeasured, drill needed** (#2294). There is no faster per-route path. For an urgent single-route stop use the global switch.

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

Use alongside blocking the user in Entra when one account must be cut off immediately without waiting up to 12 h (`SESSION_MAX_AGE`) for Entra expiry or refresh.

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

- Outage: `/ready` returns 503, so Nais takes both pods out of the ingress and every call fails until the database is back. The pods are not restarted by liveness. No HA is configured (`.nais/postgres.yaml`).
- Loss or corruption: recreate empty. **Warning:** this drops the `revoked` denylist while client-held refresh tokens stay decryptable with the unchanged `RT_KEYS`, so revoked or already-used tokens can pass replay checks again if Entra still accepts them. Rotate `RT_KEYS` (drop the old key, see above) whenever the database is lost. Everything in it is short-lived (sessions 12 h, consent 30 days, DCR clients up to 180 days). Users log in again; migrations run at startup. OpenCode users may need `opencode mcp logout <name>` first (unverified).
- Backups: Nais Postgres defaults (automated backups, retention) are **unverified**; none are required for this design.

## Post-deploy smoke (three steps)

1. `curl -s -o /dev/null -w '%{http_code}\n' $URL/.well-known/oauth-protected-resource/mcp/<namespace>/<name>` prints `200`.
2. One login with your own user through a real client.
3. One tool call (`whoami`); an `audit mcp` line with `context.outcome` `ok` and `decision.decision` `true` appears. Full version: [t7-prod-test.md](t7-prod-test.md).

## Contacts

- Owner: team copilot (placeholder: on-call rota and phone, to be filled in).
- Slack: #ki-produktutvikling-diskusjon.
- Entra and Texas: Nais platform support (placeholder: channel).
- CIMD documents: `github.com` and `opencode.ai` are third parties; no contact.

## Kill-switch drill

Run in dev first (`kubectl config use-context dev-gcp`, `URL=https://fleet-gateway.intern.dev.nav.no`), then prod in a quiet window with the pilot team told. Use a stopwatch; write each time below and in #2317.

- [ ] Baseline: route returns 200/401 (not 503), `/ready` 200.
- [ ] Route: set `enabled: false` on a dev route, open PR. Same-repo PRs deploy to dev before merge, so the route may already be 503. Start the clock at the shutdown decision (PR opened), not at merge. Stop when the rollout is complete, no old replica serves, and each current pod returns `503` when probed directly (`kubectl port-forward`). Record decision-to-503 and merge-to-503. Time: ____
- [ ] Re-enable via PR; time to recovery: ____
- [x] Global (dev, 2026-11-18, `nais app set env`): 34 s to every pod 503. Start at the command, stop only after the rollout completes and each current pod returns `503` when probed directly; repeated ingress requests do not show which replica answered. Time: 34 s.
- [x] Confirm `/health`, `/ready`, metadata and login still respond.
- [x] Deploy with the switch set (dev rerun of the deploy job only): the switch did not survive; the pod came back serving 401.
- [x] Undo with `GATEWAY_DISABLED-`: 30 s to recovery.
- [ ] Record whether a changed `RT_KEYS` secret needs a restart.
- [ ] Copy the numbers into this page and close the measurement half of #2294.

## Alert drill

Run in dev only. `nais app stop` and `nais app set replicas` ask for interactive confirmation and refuse without a terminal, so the owner runs them. Developer accounts cannot scale with kubectl, and the Alertmanager is not reachable with developer rights (no `pods/portforward` in `nais-system`).

Alerts route per environment to the channel set in Nais Console, not necessarily #github-copilot-alerts.

- [x] `nais app stop fleet-gateway -t copilot -e dev-gcp` (dev-gcp, 2026-11-18, 19:43:39 UTC).
- [x] `FleetGatewayNoReadyPods` reached Slack at 19:47 UTC, about 3.5 min after the stop. The team's dev alert channel was changed in Nais Console during the drill, so treat 3.5 min as an upper bound.
- [x] `nais app set replicas fleet-gateway --min 1 --max 2 -t copilot -e dev-gcp`; `/ready` returned 200 at 19:48:18 UTC.
- [x] Alert resolve time: RESOLVED reached Slack at 19:52 UTC (2026-11-18), about 4 min after restore (`/ready` 200 at 19:48:18 UTC).
