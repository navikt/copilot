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

**GitHub App prerequisites.** Device flow enabled; organization permissions
*Members: read* and whatever lets it read the org's SAML identities (the
permission copilot-api's App uses for `externalIdentities`; verify in dev);
installed on `navikt`. Without the installation, org
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
| `GET` | `/api/v1/surveys/active` | none | Open surveys from [`surveys/`](surveys/README.md) |
| `POST` | `/api/v1/surveys/{id}/responses` | GitHub or Entra | Submit answers: 201, 409 already answered, 400 invalid, 404 not open |
| `GET` | `/health`, `/ready`, `/metrics` | none | Probes and Prometheus |

## Surveys: data model and retention

Definitions: [`surveys/`](surveys/README.md), one file per survey, validated
at start and in CI.

**Goal: refuse a second answer without storing anything that links an answer
to a person, not even pseudonymously.**

A submission is `{"answers": {…}, "context": {…}}`, strictly validated,
unknown fields refused, body at most 32 KiB. Then:

1. The respondent's Nav e-mail is found: from the Entra token
   (`preferred_username`), or for a GitHub sign-in from the member's SAML SSO
   identity in navikt (`externalIdentities … samlIdentity.nameId`, read with
   the nav-pilot GitHub App's installation token). A GitHub account with no
   SAML identity gets 403 and is told to answer on ki-utvikling. The e-mail is
   used in memory for step 2 only and never stored or logged (the Entra one
   sits in the in-memory token cache for at most 5 minutes). The two strings
   must be the same address for one person, or that person can answer twice:
   check with a real user in dev that `preferred_username` and the SAML
   `nameId` agree before launch.
2. The dedup hash is `HMAC-SHA256(survey key, lowercased e-mail)`. A hash
   already written or queued gets 409, from nav-pilot and the web alike.
3. The submission is queued per survey. Every 10 (k) submissions to one
   survey are written together, in one transaction: their 10 hashes to
   `survey_participation` and their 10 answers to `survey_answers`, each
   shuffled.

Two tables, nothing shared but the survey id:

| Table | Columns |
| --- | --- |
| `survey_participation` | `survey_id`, `participant_hash`, `closes_on` |
| `survey_answers` | `survey_id`, `answers` (`{question id: value}`), `question_versions` (`{question id: version}`), `context` (nav-pilot version as year.month, OS, arch, client, local models on/off), `delete_after` |

No row id, no timestamp, no request id, no IP, no login, oid, NAVident,
e-mail or token in either. Because nothing links an answer to its
participation row, **an answer cannot be changed or withdrawn** after it is
sent; nav-pilot and the web say so before sending.

**Per-survey key lifecycle.** (Close-out owner: whoever owns the survey,
named in its pull request.) Each survey has its own random key,
`SURVEY_KEY_<ID>` (id upper-cased, `-` as `_`; `openssl rand -base64 32`),
in the Nais secret `copilot-cli`, namespace `copilot`, created by the team
before the survey opens. Who can read it: members of the `copilot` Nais team
(namespace secret access). It is never in the database or the image. The day
after the survey closes, copilot-cli deletes that survey's participation rows,
and the team deletes its key from the secret. From then on no key exists to
recompute a hash, and no table holds one.

**Pseudonymous while open.** Until the key and the participation rows are
deleted, anyone who holds both the key and database access (members of the
`copilot` Nais team) can test whether a given e-mail answered: Nav e-mails are
enumerable, so this is a trivial dictionary test. Without the key the hash
cannot be reversed (HMAC-SHA256, 256-bit random key). copilot-cli logs a
warning at start for every key whose survey has closed.

**1 of k.** Because both tables are written only in batches of k = 10 per
survey, in one transaction, with rows shuffled, commit time, transaction id
and row order place an answer among the 10 participants of its batch, no
fewer. Even someone with the key and the database can narrow an answer to
10 named people, not to one. The exception is the survey's last batch,
written when it closes with whatever is left (1 to 9): its participation rows
are deleted in the same close-out, but WAL and backups keep them for the
backup retention period.

Submissions still queued are lost on a restart (at most 9 per survey): their
senders are not recorded and can answer again, but were told "recorded".
Avoid deploying copilot-cli while a survey is open. Writing the queue early
instead would break the 1 of k.

After close-out the retained answers have no identifier and no key exists:
they are meant to be anonymous.

Residual risks, for the privacy review:

- Small segments: a rare combination of context values narrows who answered.
  Exports must suppress or merge any segment with fewer than 5 respondents.
- Free text: the one answer that can name its author ("as the only Rust dev
  on team X"). A survey has at most one text question, nav-pilot asks people
  not to write anything that identifies anyone, and text should go through a
  redaction pass before analysis.
- Whether the ingress sees each naisdevice as its own address (in the access
  log) or only the naisdevice gateway's: not verified; check in dev.
- Cloud SQL query insights are off for this instance; keep them off, and keep
  `log_statement` at its default (none).
- Ingress access logs hold the time and source address of each
  `POST /api/v1/surveys/…`. copilot-cli itself logs no identity, hash or
  answer on this path, but the Nais ingress log is outside its control: ask
  the platform team to drop or sample access logs for this path, or keep their
  retention short.
- Cloud SQL backups and WAL keep deleted participation rows (and batch commit
  times) for the backup retention period (7 backups by default).
- Upgrade path, if the separation of the two tables is judged not convincing:
  blind-signed one-time tokens (Privacy Pass style), so the server never sees
  who spends a token.

**Pre-launch gate:** a DPIA / personvernombud check. With this design the
retained data should be anonymous; the privacy officer should confirm that.

Export for analysis: `SELECT answers, question_versions, context FROM
survey_answers WHERE survey_id = $1`, one JSON row per respondent keyed by
question id; `construct` and `reverse` come from the definition file.

## Configuration

NAIS injects the Entra, Texas and database variables. The rest comes from the
secret `copilot-cli` (namespace `copilot`), created by hand in the Nais console
in each cluster:

| Key | Purpose | How to make it |
| --- | --- | --- |
| `GITHUB_CLIENT_ID` | the nav-pilot GitHub App's client id | the App's settings page |
| `GITHUB_CLIENT_SECRET` | checks that a token was issued to that App | App → *Generate a new client secret* |
| `GITHUB_APP_ID` | the App's id, for its installation token | the App's settings page |
| `GITHUB_APP_PRIVATE_KEY` | signs the App's JWT (PEM) | App → *Generate a private key* |
| `GITHUB_APP_INSTALLATION_ID` | the App's installation on navikt | the installation's URL |
| `SURVEY_KEY_<ID>` | one per survey, see above | `openssl rand -base64 32`; delete at close |

Missing GitHub credentials turn the GitHub path off (503). A survey without
its key, or no database, takes no answers (503). Without the App's
installation credentials, GitHub sign-ins cannot answer (503); the web still
can.

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
