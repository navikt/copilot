# copilot-cli

CLI gateway for [nav-pilot](../../cli/nav-pilot) — lets developers fetch their
personal GitHub Copilot usage data from the terminal without going through
the my-copilot web UI.

See [issue #337](https://github.com/navikt/copilot/issues/337) for the full
PRD and architecture rationale.

## Architecture

```
nav-pilot ──(GitHub token)──▶ copilot-cli ──(M2M token via Texas)──▶ copilot-api
```

1. The developer authenticates via GitHub (device flow, handled in nav-pilot)
   and sends their GitHub token to copilot-cli as a Bearer token.
2. copilot-cli validates the token against `GET api.github.com/user` and
   checks navikt org membership (`GET /orgs/navikt/members/{user}`), caching
   a positive result for 5 minutes.
3. copilot-cli exchanges its own workload identity for an M2M access token
   via the Texas sidecar (`NAIS_TOKEN_ENDPOINT`), scoped to copilot-api.
4. copilot-cli forwards the request to copilot-api with the M2M token and an
   `X-On-Behalf-Of: <github-username>` header. copilot-api only trusts this
   header when the M2M token's `azp` claim matches copilot-cli's client ID.

This keeps GitHub tokens off copilot-api entirely, and keeps copilot-api's
Azure AD auth model (used by my-copilot) unchanged.

## Endpoints

| Method | Path            | Auth              | Description                     |
| ------ | --------------- | ----------------- | -------------------------------- |
| `GET`  | `/api/v1/usage` | GitHub Bearer token | Current month usage summary    |
| `GET`  | `/health`       | none               | Liveness                         |
| `GET`  | `/ready`        | none               | Readiness                        |
| `GET`  | `/metrics`      | none               | Prometheus metrics               |

## Configuration

| Variable              | Description                                          | Default              |
| --------------------- | ----------------------------------------------------- | --------------------- |
| `PORT`                | Server port                                            | `8080`                |
| `LOG_LEVEL`           | Log level                                              | `INFO`                |
| `GITHUB_ORG`          | Required GitHub org membership                         | `navikt`              |
| `COPILOT_API_URL`     | Internal URL of copilot-api                            | `http://copilot-api`  |
| `COPILOT_API_AUDIENCE`| Entra ID scope for the M2M token (defaults from `NAIS_CLUSTER_NAME`) | derived |
| `NAIS_TOKEN_ENDPOINT` | Texas sidecar client_credentials endpoint (injected by NAIS) | — |

## Development

```bash
mise install
mise check   # fmt, vet, staticcheck, lint, test
mise dev     # run locally on :8080
```

Note: without a Texas sidecar (`NAIS_TOKEN_ENDPOINT` unset), calls to
`/api/v1/usage` will fail once GitHub auth succeeds, since no M2M token can
be minted locally. GitHub token validation and org-membership checks work
without any additional setup.
