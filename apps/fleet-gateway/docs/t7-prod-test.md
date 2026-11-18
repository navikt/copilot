# T7: manual prod verification

Run after each prod deploy of fleet-gateway. Cluster `prod-gcp`, namespace `copilot`.
Step 3 deletes one prod pod. That is the only step that changes prod. Steps 2, 4 and 5 log in and out with your own user, and step 7 writes a temporary local file. Never read secrets (`kubectl get secret`, `nais secret`).

Setup:

```sh
kubectl config use-context prod-gcp
NS=copilot
URL=https://fleet-gateway.intern.nav.no
```

## 1. Rollout is healthy

```sh
kubectl -n $NS rollout status deploy/fleet-gateway --timeout=120s
kubectl -n $NS get pods -l app=fleet-gateway
curl -s -o /dev/null -w '%{http_code}\n' $URL/.well-known/oauth-protected-resource/mcp/copilot/onboarding
```

Expected: rollout complete, pods Ready, curl prints `200`.

## 2. Login via OpenCode and whoami

Add `$URL/mcp/copilot/onboarding` as a remote MCP server in OpenCode, then `opencode mcp auth <name>`. Complete the Entra login in the browser. In a session, call the `whoami` tool.

Expected: login succeeds, `whoami` returns your NAVident.

```sh
kubectl -n $NS logs -l app=fleet-gateway --prefix --tail=-1 --since=10m | grep '"audit mcp"' | tail -5
```

Expected: lines with your NAVident in `subject.id`, `resource.id` `copilot/onboarding`, `decision.decision` `true` and `context.outcome` `ok`. Note `context.protocol_version` and `context.mcp_headers` for each client (Copilot CLI, OpenCode): they show which MCP revision it sends.

## 3. Pod restart keeps the session

Prod mutation: this deletes one gateway pod. The other replica keeps serving, and the deployment starts a replacement.

```sh
kubectl -n $NS get pods -l app=fleet-gateway
kubectl -n $NS delete pod <one-pod-name>
kubectl -n $NS rollout status deploy/fleet-gateway --timeout=120s
```

Call `whoami` again in the same OpenCode session.

Expected: works without a new login (session in Postgres, refresh token client-encrypted).

## 4. Refresh after access-token expiry

Wait for the gateway access token to expire (check `expires_in` in the token response; do not log tokens). Call `whoami`.

Expected: call succeeds, client refreshed silently, no browser prompt.

```sh
kubectl -n $NS logs -l app=fleet-gateway --prefix --tail=-1 --since=10m | grep -i -E 'refresh|token' | tail -10
```

Expected: no errors; no `invalid_grant`.

## 5. Revoke

In OpenCode, log out the server (`opencode mcp logout <name>`), which calls `POST /revoke`. Then call `whoami`.

Expected: the client asks for login. This shows the client dropped its credentials, not that the server revoked them. `/revoke` of a refresh token revokes its whole token family (since #2278), so server-side revocation of the access token and the refresh token is covered by `TestRevoke_RefreshTokenAndAccessToken`, not by this step. Do not copy tokens out of OpenCode to replay them by hand.

```sh
kubectl -n $NS logs -l app=fleet-gateway --prefix --tail=-1 --since=5m | grep -i revoke | tail -5
```

## 6. Kill switch dry run

Do not merge. On a branch, set `enabled: false` on the `/mcp/copilot/onboarding` route in `apps/fleet-gateway/pilot-routes.prod.yaml`. Run `mise check` and `go test ./...` in `apps/fleet-gateway`; both must pass with the route disabled. Push the branch, but do not deploy it.

Expected: tests green.

Optional runtime check in dev: dev loads `pilot-routes.dev.yaml` (`pilot_routes_file` in `.nais/dev-gcp.yaml`), not the prod file. On a separate branch, set `enabled: false` on the route in `apps/fleet-gateway/pilot-routes.dev.yaml` and deploy that branch to dev. Calls to the route are then refused and the log shows `route disabled by kill switch`:

```sh
kubectl --context dev-gcp -n $NS logs -l app=fleet-gateway --prefix --tail=-1 --since=5m | grep 'kill switch'
```

Redeploy main to dev afterwards. Global switch: `GATEWAY_DISABLED=true` stops all routes. Know the rollback path before using it.

## 7. Logs: NAVident only, no tokens

```sh
kubectl -n $NS logs -l app=fleet-gateway --prefix --tail=-1 --since=1h > /tmp/fgw.log
grep -c '"audit mcp"' /tmp/fgw.log
grep -E -i 'eyJ[A-Za-z0-9_-]{10,}|bearer [A-Za-z0-9._~+/=-]{10,}|"(access_token|refresh_token|client_secret|id_token)":"|[A-Za-z0-9_-]{100,}' /tmp/fgw.log | wc -l
grep -E -o '"navident":"[^"]*"' /tmp/fgw.log | sort -u
grep -E -o '"subject":\{"type":"user","id":"[^"]*"' /tmp/fgw.log | sort -u
rm /tmp/fgw.log
```

Expected: audit count above 0. The token grep prints `0`: no JWT (`eyJ...`), no bearer value, no JSON field holding a token, and no long base64url run such as a sealed or Entra refresh token. Field names like `refresh_token_received` do not match. Any match is a leak until shown otherwise. The navident and `subject.id` values are only NAVidents (one letter, six digits). No e-mail, name or token in any line.

## Result

Record date, deployed SHA and pass/fail per step in the PR or issue that tracks the prod rollout (#2276).
