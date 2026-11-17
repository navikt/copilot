# T7: manual prod verification

Run after each prod deploy of fleet-gateway. Cluster `prod-gcp`, namespace `copilot`.
Read-only except steps 4 and 7. Never read secrets (`kubectl get secret`, `nais secret`).

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
curl -s -o /dev/null -w '%{http_code}\n' $URL/.well-known/oauth-protected-resource/mcp/onboarding
```

Expected: rollout complete, pods Ready, curl prints `200`.

## 2. Login via OpenCode and whoami

Add `$URL/mcp/onboarding` as a remote MCP server in OpenCode, then `opencode mcp auth <name>`. Complete the Entra login in the browser. In a session, call the `whoami` tool.

Expected: login succeeds, `whoami` returns your NAVident.

```sh
kubectl -n $NS logs deploy/fleet-gateway --since=10m | grep '"audit mcp"' | tail -5
```

Expected: lines with `navident`, `route=/mcp/onboarding`, `outcome` ok.

## 3. Pod restart keeps the session

```sh
kubectl -n $NS rollout restart deploy/fleet-gateway
kubectl -n $NS rollout status deploy/fleet-gateway --timeout=120s
```

Call `whoami` again in the same OpenCode session.

Expected: works without a new login (session in Postgres, refresh token client-encrypted).

## 4. Refresh after access-token expiry

Wait for the gateway access token to expire (check `expires_in` in the token response; do not log tokens). Call `whoami`.

Expected: call succeeds, client refreshed silently, no browser prompt.

```sh
kubectl -n $NS logs deploy/fleet-gateway --since=10m | grep -i -E 'refresh|token' | tail -10
```

Expected: no errors; no `invalid_grant`.

## 5. Revoke

In OpenCode, log out the server (`opencode mcp logout <name>`), which calls `POST /revoke`. Then call `whoami`.

Expected: the call fails with 401 and the client asks for login. Replaying the old refresh token returns `invalid_grant`.

```sh
kubectl -n $NS logs deploy/fleet-gateway --since=5m | grep -i revoke | tail -5
```

## 6. Kill switch dry run

Do not merge. On a branch, set `enabled: false` on the `/mcp/onboarding` route in `apps/fleet-gateway/pilot-routes.prod.yaml`. Run `mise check` and `go test ./...` in `apps/fleet-gateway`; both must pass with the route disabled. Push the branch, but do not deploy it.

Expected: tests green. If deployed to dev instead, calls to the route are refused and the log shows `route disabled by kill switch`:

```sh
kubectl --context dev-gcp -n $NS logs deploy/fleet-gateway --since=5m | grep 'kill switch'
```

Global switch: `GATEWAY_DISABLED=true` stops all routes. Know the rollback path before using it.

## 7. Logs: NAVident only, no tokens

```sh
kubectl -n $NS logs deploy/fleet-gateway --since=1h > /tmp/fgw.log
grep -c '"audit mcp"' /tmp/fgw.log
grep -E -i 'eyJ[A-Za-z0-9_-]{10,}|bearer |refresh_token|access_token|client_secret' /tmp/fgw.log | wc -l
grep -E -o '"navident":"[^"]*"' /tmp/fgw.log | sort -u
rm /tmp/fgw.log
```

Expected: audit count above 0, token grep prints `0`, navident values are only NAVidents (one letter, six digits). No e-mail, name or token in any line.

## Result

Record date, deployed SHA and pass/fail per step in the PR or issue that tracks the prod rollout (#2276).
