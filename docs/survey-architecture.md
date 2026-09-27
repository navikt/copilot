# User surveys: gateway, API and survey service

**Status:** proposal. It replaces the survey parts of copilot-cli (#339). Part of #1023.

## The three services

| Service | Role | Holds |
| --- | --- | --- |
| **copilot-gateway** (renamed from copilot-cli) | The front door for clients outside the browser (nav-pilot). It checks the GitHub token and org membership, then calls copilot-api with its own app token and the GitHub login. It has no survey logic. | Secret `copilot-gateway`: `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` (the Nav Copilot (my-copilot) App) |
| **copilot-api** | The central API. It works out who the caller is, finds their Nav e-mail, and calls copilot-survey. | Unchanged: the GitHub App key, BigQuery |
| **copilot-survey** (new) | Owns the survey domain: the definitions (`surveys/*.json`), the dedup HMAC, the two unlinked tables and the batching. | Secret `copilot-survey`: `SURVEY_KEY_<ID>`. Its own Postgres |

## Request paths

### nav-pilot (CLI)

```
nav-pilot            copilot-gateway                 copilot-api                         copilot-survey
   |  GitHub user token   |                              |                                     |
   |--------------------->| POST /applications/{id}/token (App client id+secret): issued to this App?
   |                      | GET /orgs/navikt/members/{login}: member?
   |                      |  app token (azp=gateway) + X-On-Behalf-Of: <login>                 |
   |                      |----------------------------->| trusts the header only from the gateway azp
   |                      |                              | SAML: login -> Nav e-mail (its App)  |
   |                      |                              |  app token (azp=copilot-api)         |
   |                      |                              |  {survey, answers, context, email}   |
   |                      |                              |------------------------------------->| HMAC(key, e-mail) -> participation
   |                      |                              |                                     | answers -> queue, batch of 10
   |<------------------------------ 201 / 409 / 400 / 404 -------------------------------------|
```

### ki-utvikling (web)

```
browser -> my-copilot (Wonderwall session) -> OBO token (aud copilot-api) -> copilot-api
        -> e-mail from the token's preferred_username -> copilot-survey (as above)
```

The web no longer goes through the gateway: it has Entra, and copilot-api already accepts my-copilot's OBO tokens. The gateway drops its Entra path.

### Survey definitions

copilot-survey serves `GET /surveys/active`. copilot-api republishes it as a public route (`/public/v1/surveys/active`, used by my-copilot), and the gateway passes it through for nav-pilot (`GET /api/v1/surveys/active`, no sign-in).

## Trust boundaries

1. **Gateway ingress** (`.intern.nav.no`, naisdevice only). Only GitHub tokens issued to the Nav Copilot App are accepted, and only for navikt members. A flood of tokens is rate limited.
2. **Gateway to copilot-api.** An Entra app token from the gateway (`azp` = copilot-gateway, from `AZURE_APP_PRE_AUTHORIZED_APPS`). copilot-api trusts `X-On-Behalf-Of` only from that `azp`, and today only on GETs. The survey route is the one POST it will be trusted on, and the header is still format-checked.
3. **my-copilot to copilot-api.** An OBO token for a user (NAVident and preferred_username are set).
4. **copilot-api to copilot-survey.** An Entra app token. copilot-survey accepts only `azp` = copilot-api and only app tokens (no user in the token). Its `accessPolicy.inbound` lists only copilot-api, and it has no ingress. So nobody else can send it an e-mail to hash: not the gateway, not my-copilot, not a user.
5. **copilot-survey storage.** The same two unlinked tables as today: `survey_participation(survey_id, participant_hash, closes_on)` and `survey_answers(survey_id, answers, question_versions, context, delete_after)`. Both are written only in shuffled batches of 10 per survey, in one transaction. The survey's hashes and its key are deleted when it closes.

**Confused deputy.** copilot-survey trusts only copilot-api. copilot-api never takes an e-mail from the request: it derives it from the token (web) or from the SAML lookup of the gateway-asserted login (CLI). The gateway derives the login from a GitHub token it has checked. No hop takes an identity the caller could choose.

**Replay.** GitHub and Entra tokens are bearer tokens, valid until they expire. Replaying one submits as that user, which dedup turns into a 409. App tokens between services never leave the cluster. The survey POST is not idempotent by design: a second POST is a 409, which is the replay protection.

## Where the e-mail is

- **Used:** in copilot-api's memory for one request, and in copilot-survey's memory while it computes the HMAC.
- **In transit:** in the body from copilot-api to copilot-survey, over in-cluster HTTP between two pods in the same namespace, restricted by the network policy. It is not TLS today, the same as every other in-cluster call in Nais.
- **Never:** in a URL, a log line, a metric, a trace attribute or a cache. In copilot-api it reaches only the survey handler, and that handler stops the logging middleware and the on-behalf-of audit line from recording the login or path parameters on this route.
- **Traces:** the survey route in copilot-api and copilot-survey records method and route template only, never the body.

## Logs

| Service | Logs on the survey path |
| --- | --- |
| gateway | status and error type. No login, no token |
| copilot-api | status and error type. The on-behalf-of audit line is skipped for this route (it would name the login and the time) |
| copilot-survey | survey id, status, batch size, error type. Never the e-mail, the hash or the answers |
| Nais ingress | the gateway's access log has time and source for `POST /api/v1/surveys/…`. Checked before launch |

## Secrets

| Secret | Namespace | Keys | Who creates it |
| --- | --- | --- | --- |
| `copilot-gateway` | copilot | `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` (Nav Copilot (my-copilot) App, app id 1108263) | the team, in dev and prod |
| `copilot-survey` | copilot | `SURVEY_KEY_<ID>`, one per open survey, `openssl rand -base64 32` | the survey owner, just before launch. Deleted the day after it closes |
| `copilot-cli` | copilot | (old) | deleted after cutover |

copilot-api's secret is unchanged. The gateway does not get the App's private key or copilot-api's billing token.

## Migration (no prod deploy and no survey data yet)

1. **copilot-survey.** New app `apps/copilot-survey`. Survey code moves out of copilot-cli with its tests: `surveys.go`, `store.go`, the definitions, the batching. It gets Postgres, inbound only from copilot-api, and no ingress. The q4-2026 draft (#1054) moves to `apps/copilot-survey/surveys/`, still unmerged.
2. **copilot-api.**
   - `POST /api/v1/surveys/{id}/responses`, taking on-behalf-of from the gateway (POST allowed on this route only) and OBO from my-copilot. The e-mail comes from the token or the SAML lookup.
   - `GET /public/v1/surveys/active`, proxied to copilot-survey.
   - Outbound to copilot-survey.
3. **copilot-gateway.**
   - Rename `apps/copilot-cli` to `apps/copilot-gateway`, including the workflow, `.nais`, docs and the ci.yaml job.
   - Drop the Entra path, the survey code and Postgres.
   - Forward `POST /api/v1/surveys/{id}/responses` and `GET /api/v1/surveys/active` to copilot-api.
   - Ingresses: `copilot-gateway.intern(.dev).nav.no`, plus `copilot-cli.intern(.dev).nav.no` for nav-pilot releases that already ship that default.
   - The startup doesn't depend on the secret's keys: without them GitHub sign-in answers 503.
4. **Clients.**
   - nav-pilot's default URL becomes `https://copilot-gateway.intern.nav.no`. The allowlist is unchanged (`*.nav.no`).
   - The web form (#1076) calls copilot-api with the OBO token it already gets.
5. **Cleanup** (you): delete the `copilot-cli` app and its Postgres instance in dev-gcp, and the `copilot-cli` secret in both clusters.

Each step is its own PR, reviewed by Fable and given the language pass, merged at 0 threads.

## What you need to do

1. Create secret `copilot-gateway` (`GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, the same values as in `copilot-cli`) in dev-gcp and prod-gcp.
2. On the Nav Copilot (my-copilot) App: turn on device flow, and check that user-token expiry is off (or ask for refresh support).
3. Before launch: create secret `copilot-survey` with `SURVEY_KEY_Q4_2026`.
4. After cutover: delete the `copilot-cli` secret (dev and prod), the `copilot-cli` app and its Postgres instance (dev).
5. The launch checklist: `apps/copilot-survey/surveys/README.md`.
