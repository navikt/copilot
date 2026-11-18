# fleet-gateway runbook

For the on-call engineer. Cluster `prod-gcp` (dev: `dev-gcp`), namespace `copilot`, app `fleet-gateway`. Commands are read-only unless marked **break-glass**. Never read secrets (`kubectl get secret`, `nais secret`, `printenv`). Post-deploy verification: [t7-prod-test.md](t7-prod-test.md). Design and plans live in issues (#2317 for this page).

Route paths below are written `/mcp/<namespace>/<name>`. Until #2350 lands, routes are `/mcp/<name>` (for example `/mcp/onboarding`); the mechanism is the same.

## What it is

OAuth 2.1 authorization server in front of Microsoft Entra ID, plus a reverse proxy that forwards `/mcp/*` calls to team MCP servers with a Texas on-behalf-of token. Two replicas in prod, one to two in dev.

| Dependency | Used for | If it is down |
| --- | --- | --- |
| Entra (`login.microsoftonline.com`) | Login, refresh | Login and refresh fail. Existing access tokens keep working until they expire (about 1 h) |
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

Audit lines: `audit mcp` (route, tool, decision, outcome, NAVident), `audit login`, `audit refresh`, `audit dcr`. A killed route logs `route disabled by kill switch`. Logs are ordinary stdout; retention in the platform log store is unverified (#2317 F1). A saved query for Grafana Loki or Kibana is not yet recorded: add it here after the drill.

## Kill switches

Both return `503` with `Retry-After: 300` for `/mcp/*`. The check runs before the bearer token is read, so unauthenticated and authenticated callers alike see 503. Login, `/token` and metadata stay up.

### Close one route

1. Set `enabled: false` on the route in `pilot-routes.<env>.yaml`.
2. PR, required checks, merge queue, build, deploy.
3. Confirm: `curl -s -o /dev/null -w '%{http_code}\n' -X POST $URL/mcp/<namespace>/<name>` prints `503`.

Time from merge to 503: **unmeasured, drill needed** (#2294). There is no faster per-route path. For an urgent single-route stop use the global switch.

### Close everything

`GATEWAY_DISABLED` is read in `config.go` and is **not** in `.nais/app.yaml`. Break-glass without a merge:

```sh
# break-glass: changes prod. Starts a rolling restart.
kubectl -n copilot set env deploy/fleet-gateway GATEWAY_DISABLED=true
kubectl -n copilot rollout status deploy/fleet-gateway --timeout=120s
```

Undo: `kubectl -n copilot set env deploy/fleet-gateway GATEWAY_DISABLED-`. The next normal deploy (nais apply) overwrites the manifest and reverts the switch silently, so say in the incident channel that it is on and re-check after every deploy.

Time to 503 for the break-glass path: **unmeasured, drill needed**. It is a rollout, not an instant switch: expect old pods to serve until replaced. Whether the Nais deployment controller reverts a `set env` before the next deploy is unverified.

## Rotate `RT_KEYS`

Format: `kid:base64key,...`, 32-byte keys. The first entry encrypts; all entries decrypt (`store_seal.go`, `parseKeyRing`). The Entra state cookie is signed with the first key's HKDF subkey and verified against every entry's. A row sealed with an unknown kid reads as missing and is kept; a refresh token with an unknown kid returns `invalid_grant`, and the client then drops its refresh token.

A single deploy that prepends a new key is not safe: during the rollout a new pod seals with the new key while an old pod does not know it, so users are logged out. Use two deploys:

1. Create the new key and append it **last** (decrypt only): `old,new`. Update the secret `fleet-gateway-rt-keys` (human-created, owner action), then restart: `kubectl -n copilot rollout restart deploy/fleet-gateway`. Wait for rollout.
2. Promote: `new,old`. Update the secret, restart, wait for rollout.
3. After `SESSION_MAX_AGE` (12 h) plus one deploy, remove `old`: `new`. Anything still sealed with `old` stops working; by then those sessions have expired.

Whether the pods pick up a changed secret without a restart is unverified; restart explicitly.

## Log everyone out

Remove the current key from `RT_KEYS` (replace it with a fresh key, no old entry) and restart. All sealed values become unreadable: every session, refresh token and consent record reads as missing, and users log in again. This is the only per-user-agnostic revoke today. **There is no tool to cut off one user** on the gateway side: sessions are keyed by token hash, with no index by NAVident. For a single user, revoke in Entra; the current access token then lives at most about an hour.

## Block one client

Remove the `client_id` from `ALLOWED_CLIENT_IDS` in `.nais/<env>-gcp.yaml` and deploy. Access tokens already issued stay valid for up to an hour; the proxy does not recheck the client. A DCR client can additionally be deleted from the `items` table by its plain key.

## Postgres outage or loss

- Outage: `/ready` returns 503, so Nais takes both pods out of the ingress and every call fails until the database is back. The pods are not restarted by liveness. No HA is configured (`.nais/postgres.yaml`).
- Loss or corruption: recreate empty. Everything in it is short-lived (sessions 12 h, consent 30 days, DCR clients up to 180 days). Users log in again; migrations run at startup. OpenCode users may need `opencode mcp logout <name>` first (unverified).
- Backups: Nais Postgres defaults (automated backups, retention) are **unverified**; none are required for this design.

```sh
kubectl -n copilot get postgres,pods | grep -i fleet-gateway
```

## Post-deploy smoke (three steps)

1. `curl -s -o /dev/null -w '%{http_code}\n' $URL/.well-known/oauth-protected-resource/mcp/<namespace>/<name>` prints `200`.
2. One login with your own user through a real client.
3. One tool call (`whoami`); an `audit mcp` line with `outcome` ok appears. Full version: [t7-prod-test.md](t7-prod-test.md).

## Contacts

- Owner: team copilot (placeholder: on-call rota and phone, to be filled in).
- Slack: #ki-produktutvikling-diskusjon.
- Entra and Texas: Nais platform support (placeholder: channel).
- CIMD documents: `github.com` and `opencode.ai` are third parties; no contact.

## Kill-switch drill

Run in dev first (`kubectl config use-context dev-gcp`, `URL=https://fleet-gateway.intern.dev.nav.no`), then prod in a quiet window with the pilot team told. Use a stopwatch; write each time below and in #2317.

- [ ] Baseline: route returns 200/401 (not 503), `/ready` 200.
- [ ] Route: set `enabled: false` on a dev route, open PR. Start the clock at "merge". Stop at the first `503` from the route. Time: ____
- [ ] Re-enable via PR; time to recovery: ____
- [ ] Global: `kubectl -n copilot set env deploy/fleet-gateway GATEWAY_DISABLED=true`. Start at the command, stop at the first `503` on every replica (loop curl, 1 s apart). Time: ____
- [ ] Confirm `/health`, `/ready`, metadata and login still respond.
- [ ] Undo with `GATEWAY_DISABLED-`; time to recovery: ____
- [ ] Deploy once and check that a leftover `GATEWAY_DISABLED` is reverted or kept (record which).
- [ ] Record whether a changed `RT_KEYS` secret needs a restart.
- [ ] Copy the numbers into this page and close the measurement half of #2294.
