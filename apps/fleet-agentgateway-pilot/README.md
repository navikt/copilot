# fleet-agentgateway-pilot (dev only, spike)

agentgateway v1.7.0 in dev-gcp in front of one route: mcp-onboarding (`/mcp/gw`). Upstream image pinned by digest. No code of our own. Background: copilot-intern `underlag/fleet-agentgateway-spike-2026-11.md`.

## Status 2026-11-24

**Not deployed and not measured.** The four measurements (OBO through real Texas, latency against fleet-gateway, fail closed with Texas down, SSE) are still open. Three blockers stop them:

1. **mcp-onboarding accepts only fleet-gateway.** `apps/mcp-onboarding/entra.go` requires `azp` to be fleet-gateway's client ID, and its inbound rule names only `fleet-gateway`. An OBO token that agentgateway gets through Texas has this app as `azp`. Texas refuses the exchange, or mcp-onboarding answers `403`. To pass, mcp-onboarding must pre-authorize this app in dev and accept its `azp`. That is a change to a security check, so it needs an owner decision and its own review.
2. **No headless dev token.** The repo has no tool that issues an Entra user token for a given audience. The token needs a human browser login, for example through Nais' `azure-token-generator`, which must also be pre-authorized in `accessPolicy.inbound`.
3. **Deploying is a human step.** The workflow deploys only on `workflow_dispatch` or when the PR carries the `deploy-pilot` label, and that label does not exist yet. The agent had no write access to the cluster, so it could neither watch nor remove a deployment.

## Running on Nais without root (checked against the image and source)

- The image `ghcr.io/agentgateway/agentgateway:v1.7.0` is `cgr.dev/chainguard/glibc-dynamic` with `USER 65532` and entrypoint `/app/agentgateway`. That is non-root as is. No capabilities needed. **[read: image config via crane]**
- Ports: the proxy on `8080`, stats on `15020`, readiness on `15021` (`/healthz/ready`). All are above 1024. The admin UI is turned off (`adminAddr: off`).
- Read-only root filesystem: the config comes from a ConfigMap mounted at `/etc/agentgateway`. agentgateway writes a generated session key into its own config file when no key is set, and a ConfigMap is read-only. The pilot sets `session.key: disabled`, so it needs no secret. Anything beyond the spike must set `SESSION_KEY` (env, read in `crates/agentgateway/src/config.rs`) from a Nais secret instead.
- Cluster values come in as `${VAR}`: agentgateway expands environment variables in the config file (`shellexpand::full`, `types/local.rs`). The issuer, audience and JWKS URL are therefore Nais' `AZURE_*` variables, with no build step.
- **Not verified in a pod.** The config has not been loaded by the binary, and startup on Nais has not been seen. The Texas address `127.0.0.1:7164` is an assumption: compare it with `NAIS_TOKEN_EXCHANGE_ENDPOINT` in the pod.

## Config in short

- `jwtAuth` (strict): Entra issuer, `aud` = this app's client ID, JWKS from Entra.
- `mcpAuthorization`: `has(jwt.NAVident) && mcp.tool.name in ["whoami", "list_skills"]`.
- `backendAuth.oauthTokenExchange` to Texas with `target: api://dev-gcp.copilot.mcp-onboarding/.default`, and the raw token passed through `jwt.rawToken.unredacted()`.

## Test plan once the blockers are cleared

1. `kubectl --context dev-gcp -n copilot logs deploy/fleet-agentgateway-pilot`: config loaded, no write error.
2. Without a token, and with a token for the wrong `aud`: `401`.
3. `initialize`, `tools/list` (only `whoami` and `list_skills`), `tools/call whoami` returns the NAVident.
4. `tools/call echo`: denied before mcp-onboarding.
5. Latency: 50 × `tools/call whoami` here and through fleet-gateway (`/mcp/onboarding`), p50/p95.
6. Texas down: point `host` at a closed port and reload (the ConfigMap reloads without a restart). Expect an error and no call to mcp-onboarding.
7. SSE: `GET /mcp` with `Accept: text/event-stream` and a long-lived stream through HAProxy.

## Remove

`kubectl --context dev-gcp -n copilot delete app fleet-agentgateway-pilot configmap fleet-agentgateway-pilot`. Needs write access.
