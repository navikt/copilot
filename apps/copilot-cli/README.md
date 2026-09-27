# copilot-cli

Gateway for [nav-pilot](../../cli/nav-pilot) and ki-utvikling (my-copilot). It
signs callers in, forwards Copilot usage lookups to copilot-api, and takes
answers to user surveys. See [#337](https://github.com/navikt/copilot/issues/337)
(gateway) and [#1023](https://github.com/navikt/copilot/issues/1023) (surveys).

## Sign-in

Two paths, one normalised identity (`issuer` + the issuer's stable id):

| Caller | Token | Checked by | Identity |
| --- | --- | --- | --- |
| nav-pilot (laptop, naisdevice) | GitHub App user token from the device flow | `POST /applications/{client_id}/token` with the app's own credentials: the token must be issued **to this app** (a gh CLI or IDE token is refused), live, then `GET /orgs/navikt/members/{login}` must answer 204 | `github` + numeric user id |
| my-copilot (in-cluster) | Entra ID OBO token, audience copilot-cli | Texas introspection (signature, issuer, audience, expiry), then: a user token (`NAVident` set, `idtyp` not `app`), from a pre-authorized app (`azp` in `AZURE_APP_PRE_AUTHORIZED_APPS`) | `entra` + `oid` |

A JWT goes to the Entra path, anything else to GitHub. Every failure is a
refusal (fail closed). Outcomes are cached by SHA-256 of the token (success 5
minutes, never past the token's expiry; refusal 1 minute). Cache misses are
of the GitHub path are rate limited globally (1/s, burst 10), below the
app's 5,000/h GitHub quota, so a flood of random tokens costs a 429, not the
quota; the Entra path goes to the local Texas sidecar and is not limited.
copilot-cli logs neither tokens nor identities; copilot-api logs the GitHub
login of each usage request it serves on someone's behalf (audit).

**GitHub App prerequisites.** Device flow enabled; organization permission
*Members: read*; installed on `navikt`. Without the installation, org
membership answers 302 for everyone and every sign-in gets 403. User token
expiry: nav-pilot stores no refresh token yet, so either turn expiry off on
the App or add refresh before rollout.

No CORS headers: browsers never call this service. my-copilot calls it server
to server.

Usage (`/api/v1/usage`) needs the GitHub path: copilot-api keys usage by GitHub
login and trusts `X-On-Behalf-Of` only from copilot-cli, only on GETs.

## Endpoints

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/api/v1/usage` | GitHub | Current month usage summary |
| `GET` | `/api/v1/surveys/active` | none | Open surveys from [`surveys.json`](surveys.json) |
| `POST` | `/api/v1/surveys/{id}/responses` | GitHub or Entra | Submit answers: 201, 409 already answered, 400 invalid, 404 not open |
| `GET` | `/health`, `/ready`, `/metrics` | none | Probes and Prometheus |

## Surveys: data model and retention

Definitions: [`surveys/`](surveys/README.md), one file per survey, validated
at start and in CI.

A submission is `{"answers": {…}, "context": {…}}`, both strictly validated,
unknown fields refused, body at most 32 KiB. One table:

| Column | Content |
| --- | --- |
| `survey_id` | the survey (one wave of a series) |
| `respondent_hash` | while the survey is open: HMAC-SHA256(`SURVEY_HASH_KEY`, survey id, issuer, issuer id), for dedup. The day after it closes: replaced with a random value |
| `answers` | `{question id: value}`: a number for a scale, the option text for a choice, a list for a multi, trimmed text |
| `context` | nav-pilot version (year.month or major.minor only), OS, arch, client, local models on/off: enums only |
| `closes_on`, `delete_after` | the day after the survey ends; that + 180 days |

Primary key `(survey_id, respondent_hash)` is the dedup: a second answer gets
409. Not stored: the GitHub login or id, the Entra oid or NAVident, e-mail, IP,
`device_id`, submission time.

**Pseudonymous while open, unlinkable after.** Whoever holds
`SURVEY_HASH_KEY` and a list of navikt GitHub ids or Entra oids can recompute
a hash, so while a survey is open its rows are pseudonymous to the team, not
anonymous. The hash differs per survey (no joining across surveys or waves),
and the day after the survey closes it is replaced with a random value, after
which no key links a row to anyone. Rows are deleted at `delete_after`.

Residual risks, for the privacy review:

- Small cohorts: a rare combination of context values (say windows, arm64,
  opencode, local models on) narrows who answered; version is coarsened for
  this reason. Report only groups of at least 5.
- The ingress access log has the time and source address of each
  `POST /api/v1/surveys/…`; row order in Postgres follows insertion. Neither
  holds the identity, but together with other logs they narrow it. Keep
  ingress log retention short.
- Cloud SQL backups keep rows (and open-survey hashes) for the backup
  retention period past `delete_after`.
- The same person answering once from nav-pilot and once from the web counts
  twice (different issuers).

Export for analysis:
`SELECT survey_id, answers, context FROM survey_responses WHERE survey_id = $1`,
one JSON row per respondent, keyed by question id; join the question
`version`, `construct` and `reverse` from the definition file in git.

## Configuration

NAIS injects the Entra, Texas and database variables. The rest comes from the
secret `copilot-cli` (namespace `copilot`), created by hand in the Nais console
in each cluster:

| Key | Purpose | How to make it |
| --- | --- | --- |
| `GITHUB_CLIENT_ID` | the nav-pilot GitHub App's client id | from the GitHub App settings page |
| `GITHUB_CLIENT_SECRET` | lets this service check that a token was issued to that app | GitHub App → *Generate a new client secret* |
| `SURVEY_HASH_KEY` | keys the respondent hash | `openssl rand -base64 32` |

Missing GitHub credentials turn the GitHub path off (503), a missing hash key
or database turns survey submissions off (503). Rotating `SURVEY_HASH_KEY`
during an open survey lets people answer it again.

| Variable | Description | Default |
| --- | --- | --- |
| `PORT`, `LOG_LEVEL` | server | `8080`, `INFO` |
| `GITHUB_ORG` | required org membership | `navikt` |
| `COPILOT_API_URL` | internal URL of copilot-api | `http://copilot-api` |
| `COPILOT_API_AUDIENCE` | Entra scope for the M2M token | from `NAIS_CLUSTER_NAME` |
| `DB_URL` | Postgres (NAIS `sqlInstances`) | — |

## Development

```bash
mise install
mise check   # fmt, vet, staticcheck, deadcode, lint, test
```
