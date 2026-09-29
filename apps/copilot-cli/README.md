# copilot-cli

Gateway for [nav-pilot](../../cli/nav-pilot). It signs developers in with
GitHub and forwards nav-pilot's requests: Copilot usage to copilot-api, user surveys
to [copilot-survey](../copilot-survey/README.md). See [#337](https://github.com/navikt/copilot/issues/337)
(gateway) and [#1023](https://github.com/navikt/copilot/issues/1023) (surveys).

## Sign-in

| Caller | Token | Checked by | Identity |
| --- | --- | --- | --- |
| nav-pilot (laptop, naisdevice) | GitHub App user token from the device flow | `POST /applications/{client_id}/token` with the app's own credentials: the token must be issued **to this app** (a gh CLI or IDE token is refused), live; then copilot-api must answer that the login from GitHub's answer is an active navikt member (`POST /internal/v1/github/org-membership`, with copilot-cli's M2M token) | `github` + numeric user id |

Every failure is a refusal (fail closed), including copilot-api being down or answering anything but `{"active": true|false}` (502). Outcomes are cached by SHA-256 of
the token (success 5 minutes, never past the token's expiry; refusal 1
minute; copilot-api caches its membership answer 1 more minute). Cache misses are rate limited per token (burst 3, then one per 10
seconds), so one client cannot drain the shared budget, and under a global
ceiling (1/s, burst 10) below the app's 5,000/h GitHub quota, so a flood of
random tokens costs a 429, not the quota. A flood of distinct tokens can
still slow sign-in for others; it never lets anyone in. copilot-cli logs neither tokens nor identities; copilot-api logs the
GitHub login of each usage request it serves on someone's behalf (audit).

**GitHub App prerequisites.** Device flow enabled, and no permissions at all:
no repository, organization or account permissions. copilot-cli only checks
that a token was issued to the App (client id and secret); copilot-api checks
membership with its own GitHub App, which needs *Members: read*. nav-pilot
no longer asks GitHub about membership itself. Older releases still do, with
the user token, as a local hint in `auth login` and `auth status`: once the
App has no *Members: read*, GitHub may answer them as it would an outsider,
so a private member can see "not a member" there. It is only a hint; access
is decided here. User token expiry on:
access tokens last 8 hours, and nav-pilot renews them with the refresh token
it stores beside them.

**Sign-out.** `nav-pilot auth logout` calls `POST /api/v1/auth/revoke` with
the token as bearer. copilot-cli revokes exactly that token at GitHub
(`DELETE /applications/{client_id}/token`, which needs the App's client
secret, so nav-pilot cannot do it itself) and refuses it from its cache at
once. There is no body, so a caller can only revoke a token it holds. The
refresh token is not revoked; nav-pilot deletes both locally either way.

No CORS headers: browsers never call this service, and no app in the cluster
does either.

Downstream calls carry copilot-cli's own M2M token for that service and the
verified GitHub login in `X-On-Behalf-Of`. copilot-api honours the header on
its per-user usage GETs only, copilot-survey on the answer route only. The
public survey definitions go without a token or header.

## Endpoints

These shapes never change, because shipped nav-pilot binaries call them. A new shape
gets a new path (`/api/v2/…`).

| Method | Path | Auth | Forwarded to |
| --- | --- | --- | --- |
| `GET` | `/api/v1/usage` | GitHub | copilot-api `GET /api/v1/copilot/usage/user/{login}` |
| `POST` | `/api/v1/auth/revoke` | GitHub token, no org check | GitHub `DELETE /applications/{client_id}/token` for the bearer token: 204, 401 unknown to this app, 429, 502 GitHub failed, 503 sign-in not configured |
| `GET` | `/api/v1/surveys/active` | none | copilot-survey, same path |
| `GET` | `/api/v1/surveys/schema` | none | copilot-survey, same path: the JSON Schema of the definitions |
| `POST` | `/api/v1/surveys/{id}/responses` | GitHub | copilot-survey, same path: 201, 409 already answered, 400 invalid, 403 no Nav identity, 404 not open, 400 or 413 body over 32 KiB, 503 not taking answers, 502 copilot-survey unreachable or refused the gateway |
| `GET` | `/health`, `/ready`, `/metrics` | none | — (probes and Prometheus) |

Status, body, `Content-Type` and `Cache-Control` come back unchanged. A redirect is
returned, not followed. An unreachable service gives 502, and so does a 401 from it:
that is about copilot-cli's own token, not the caller's. No retry, so no request body is buffered.
Survey data model, key lifecycle and residual risks: [copilot-survey's README](../copilot-survey/README.md).

## Configuration

NAIS injects the Texas variables. The secret `copilot-cli` (namespace
`copilot`), created by hand in the Nais console in each cluster, holds:

| Key | Purpose | How to make it |
| --- | --- | --- |
| `GITHUB_CLIENT_ID` | the nav-pilot GitHub App's client id | the App's settings page |
| `GITHUB_CLIENT_SECRET` | checks that a token was issued to that App | App → *Generate a new client secret* |

Missing GitHub credentials turn sign-in off (503). `GITHUB_APP_*`,
`SURVEY_KEY_*` and the database are no longer used; delete them if they are
still there.

| Variable | Description | Default |
| --- | --- | --- |
| `PORT`, `LOG_LEVEL` | server | `8080`, `INFO` |
| `GITHUB_ORG` | org named in the not-a-member message; copilot-api's `GITHUB_ORG` is the one checked | `navikt` |
| `COPILOT_API_URL` | internal URL of copilot-api | `http://copilot-api` |
| `COPILOT_API_AUDIENCE` | Entra scope for the M2M token | from `NAIS_CLUSTER_NAME` |
| `COPILOT_SURVEY_URL` | internal URL of copilot-survey | `http://copilot-survey` |
| `COPILOT_SURVEY_AUDIENCE` | Entra scope for the M2M token | from `NAIS_CLUSTER_NAME` |

## Development

```bash
mise install
mise check   # fmt, vet, staticcheck, deadcode, lint, test
```
