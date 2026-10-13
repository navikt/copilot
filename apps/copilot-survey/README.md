# copilot-survey

Nav's user surveys ([#1023](https://github.com/navikt/copilot/issues/1023)):
the definitions, the answers, and the dedup that refuses a second answer. No
ingress. nav-pilot reaches it through copilot-cli, ki-utvikling through
my-copilot. It holds the survey keys and the survey database, and no GitHub
credentials.

## Callers

Texas introspection checks the Entra token, then one of two branches
applies. Anything else is refused.

| Caller | Token | Who answered |
| --- | --- | --- |
| copilot-cli (for nav-pilot) | App token: `idtyp=app` or role `access_as_application`, no NAVident or e-mail, `azp` copilot-cli | GitHub login in `X-On-Behalf-Of` (format-checked); Nav e-mail from copilot-api's `POST /internal/v1/saml/name-id` |
| my-copilot (for ki-utvikling) | User (OBO) token: NAVident and `preferred_username` set, `idtyp` not `app`, `azp` my-copilot. `X-On-Behalf-Of` is refused | `preferred_username` |

The client ids come from `AZURE_APP_PRE_AUTHORIZED_APPS`, which NAIS fills
from `accessPolicy.inbound`. `X-On-Behalf-Of` is honoured on the submit and
status routes only.

## Endpoints

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/api/v1/surveys/active` | none | Open surveys from [`surveys/`](surveys/README.md) |
| `GET` | `/api/v1/surveys/schema` | none | The JSON Schema of a definition, [`surveys/schema.json`](surveys/schema.json) |
| `POST` | `/api/v1/surveys/{id}/responses` | see above | Submit answers: 201, 409 already answered, 400 invalid, 401 caller refused, 403 no Nav identity, 404 not open, 503 not taking answers |
| `GET` | `/api/v1/surveys/{id}/status` | see above | `{"answered": bool}` for the caller, same hash as a submission: 200, 401 caller refused, 403 no Nav identity, 404 not open, 503 storage unavailable |
| `GET` | `/api/v1/admin/surveys` | admin | All surveys with status (kommende, åpen, stengt) and written answers |
| `GET` | `/api/v1/admin/surveys/{id}/results` | admin | Counts per option and question version, no free text. Fewer than 5 answers: nothing; a count of 1–4: `null`, and so is any count it could be worked out from (a complementary cell, or a question skipped on it) |
| `GET` | `/health`, `/ready`, `/metrics` | none | Probes and Prometheus (`survey_submissions_total{survey,status}`, `survey_cli_nameid_unexpected_domain_total`, `survey_queued_answers{survey}`) |

Admin: a my-copilot user token whose `groups` claim holds `COPILOT_ADMIN_GROUP_ID`. Every route under `/api/v1/admin/` passes that one check. A token copilot-survey does not accept at all gets 401; every accepted caller who is not an admin (a my-copilot user outside the group, copilot-cli) gets 404, as in copilot-api and my-copilot.

Residual risk: results are visible while a survey is open. An admin who polls `/results` during an open survey can see one new answer appear and, if they know who just answered, attribute it. Answers are written as they arrive, so nothing blunts this; only showing results after close would. The owner accepted this risk on 2026-10-13. Mitigations: admin-only access and the k≥5 threshold. The DPO assessment should reflect this residual risk.

No OpenTelemetry, no auto-instrumentation, no body logging, no retry that
buffers a body. Logs carry survey id, status, batch size and error type,
never who answered. A write runs on its own background context, never the
request's, so a caller who hangs up does not cancel it.

## Surveys: data model and retention

Definitions: [`surveys/`](surveys/README.md), one file per survey, validated
at start and in CI.

**Goal: refuse a second answer without storing anything that links an answer
to a person, not even pseudonymously.**

A submission is `{"answers": {…}, "context": {…}}`, strictly validated,
unknown fields refused, body at most 32 KiB. Then:

1. The respondent's Nav e-mail is found: from the my-copilot user's own
   token (`preferred_username`), or for a nav-pilot user from the member's
   SAML SSO identity in navikt, which copilot-api looks up
   (`POST /internal/v1/saml/name-id`; the GitHub App key stays there). A
   GitHub account with no SAML identity gets 403 and is told to answer on
   ki-utvikling. The e-mail is used in memory for step 2 only and never
   stored, cached or logged. The two strings must be the same address for
   one person, or that person can answer twice (see the residual risks).
2. The dedup hash is `HMAC-SHA256(survey key, e-mail)`, the e-mail trimmed
   and lowercased first. A hash
   already written or queued gets 409, from nav-pilot and the web alike.
3. The submission is written at once, in one transaction: its hash to
   `survey_participation` and its answer to `survey_answers`. If the write
   fails, it stays queued in memory and is written with the next submission
   to that survey or at shutdown.

Two tables, nothing shared but the survey id:

| Table | Columns |
| --- | --- |
| `survey_participation` | `survey_id`, `participant_hash`, `closes_on` |
| `survey_answers` | `survey_id`, `answers` (`{question id: value}`, a matrix item under its own id like a scale question, and `{question id}.other` for the text of an «other» option), `question_versions` (`{question id: version}`), `context` (nav-pilot version as year.month, OS, client, local models on/off), `delete_after` |

No row id, no timestamp, no request id, no IP, no login, oid, NAVident,
e-mail or token in either. An answer has no id of its own
that a participant could present, so **an answer cannot be changed or
withdrawn** after it is sent (see «Linkable while open» for what the database
can still tie together); nav-pilot and the web say so before sending.

**Per-survey key lifecycle.** (Close-out owner: whoever owns the survey,
named in its pull request.) Each survey has its own random key,
`SURVEY_KEY_<ID>` (id upper-cased, `-` as `_`; `openssl rand -base64 32`),
in the Nais secret `copilot-survey`, namespace `copilot`, created by the team
before the survey opens. Who can read it: members of the `copilot` Nais team
(namespace secret access). It is never in the database or the image. The day
after the survey closes, copilot-survey deletes that survey's participation rows,
and the team deletes its key from the secret. The running pod read its keys
at start and still holds the deleted one, so the team then restarts it
(`kubectl rollout restart deployment/copilot-survey -n copilot`), after the
close-out has written anything still queued. A pod that starts after the survey
closed loads no key for it. From then on no key exists to recompute a hash,
and no table holds one.

**Pseudonymous while open.** Until the key and the participation rows are
deleted, anyone who holds both the key and database access (members of the
`copilot` Nais team) can test whether a given e-mail answered: Nav e-mails are
enumerable, so this is a trivial dictionary test. Without the key the hash
cannot be reversed (HMAC-SHA256, 256-bit random key). copilot-survey logs a
warning at start for every key whose survey has closed.

**Linkable while open.** Each submission is written at once, its
participation row and its answer in one transaction. While a survey is open,
someone with the key and database access can therefore link an answer to a
participant hash by write time or transaction id, and with the key test
which e-mail that hash belongs to. Until 2026-10-13 submissions were written
in batches of 10, which placed an answer among the 10 participants of its
batch and no fewer. The owner dropped that on 2026-10-13 because the data is
down-classified. The context is still kept coarse (no CPU type, version as
year.month) and exports still suppress small segments, which matters once the
link is gone.

When the survey closes its participation rows are deleted as before, but WAL
and backups keep them, with their commit times, for the backup retention
period.

A failed write keeps the submission queued; the next submission to that
survey or a graceful shutdown (every deploy or restart) writes it, together
with anything else queued. Only a crash while a submission is queued, or a
failing write at shutdown, loses it.

The gauge `survey_queued_answers{survey}` is the queue size per survey.
Team members with Prometheus access can see it, which is fine since the
data is down-classified (2026-10-13). The process cannot count what a
crash loses; the last scraped value before the restart is the nearest
estimate, since submissions and writes between that scrape and the crash
are not in it. A failed write at shutdown is logged (`survey: writing
queued submissions at shutdown failed`, and the count in `survey: queued
submissions lost at shutdown`), not counted: `/metrics` is closed by then,
so a counter would never be scraped and an alert on it could never fire.

After close-out the retained answers have no identifier and no key exists:
they are meant to be anonymous.

Residual risks, for the privacy review:

- Small segments: a rare combination of context values narrows who answered.
  Exports must suppress or merge any segment with fewer than 5 respondents.
- A compromised copilot-cli can submit one answer as any navikt member per
  open survey, which also locks that member out (409). It is the same trust
  copilot-api places in copilot-cli for usage reads. There is no rate limit
  per login; the `CopilotSurveySubmissionBurst` alert in `.nais/app.yaml`
  fires when `survey_submissions_total` grows by more than 20 in 5 minutes,
  all surveys and statuses summed. See the accepted risk in `SECURITY.md`.
- Free text: the answers that can name their author ("as the only Rust dev
  on team X"): a `text` question (at most one per survey) and the short text
  of an «other» option (at most 200 characters each, at most three per
  survey). They are stored in the same row as the rest of that person's
  answers, so one identifying text identifies the whole row. nav-pilot and
  the web ask people not to write anything that identifies anyone, and all
  free text should go through a redaction pass before analysis and never be
  quoted next to its segment values.
- Whether the ingress sees each naisdevice as its own address (in the access
  log) or only the naisdevice gateway's: not verified; check in dev.
- Cloud SQL query insights are off for this instance; keep them off, and keep
  `log_statement` at its default (none).
- Ingress access logs hold the time and source address of each
  `POST /api/v1/surveys/…` on copilot-cli and my-copilot. Neither they nor
  copilot-survey log an identity, hash or answer on this path, but the Nais
  ingress log is outside our control. Ask the platform team to drop or sample
  access logs for this path, or keep their retention short.
- Write timing: each submission is written during its own request, so
  someone with database access (the `copilot` Nais team) and the ingress
  access log can link an answer to the request that sent it, and so to its
  time and source address. Accepted by the owner on 2026-10-13 along with
  writing at once (see "Linkable while open").
- Cloud SQL backups and WAL keep deleted participation rows (and commit
  times) for the backup retention period (7 backups by default).
- Two identities for one person: the web hashes the Entra
  `preferred_username` (the UPN), nav-pilot the SAML `nameId` of the navikt
  SSO identity. Both are trimmed and lowercased; nothing else is folded,
  because Nav publishes no alias scheme we could rely on. The evidence that
  they agree: my-copilot already finds a user's GitHub account by passing
  their `preferred_username` to GitHub's `externalIdentities(userName:)`
  (copilot-api, `SAMLIdentityResolver`). That lookup only finds anyone if
  the Entra SAML app sends the UPN as `nameId` (or as the SCIM `userName`,
  if navikt is provisioned through SCIM). Not verified against the
  directory. Where they can still differ: an SSO identity linked before a
  name change (the UPN changes; the stored `nameId` may keep the old
  one), a `nameId` from
  another domain or format, and whatever GitHub folds when it matches
  `userName` (case, at least). Such a person can answer once from nav-pilot
  and once on the web. `survey_cli_nameid_unexpected_domain_total` counts
  nav-pilot submissions whose `nameId` is not `@nav.no` (no label, nothing
  logged); a counter that stays at 0 rules out the second case only. The
  counter is not an identity, but its increment is timed: with the ingress
  log, it tells that one submission had such a `nameId`. What would measure
  the whole risk needs directory access and the owner's approval: an
  aggregate count, never a list, of navikt SSO identities whose `nameId`
  differs, ignoring case, from the Entra UPN of the same person. Someone
  with read access to both runs it once before launch and records only the
  number.
- Upgrade path, if the separation of the two tables is judged not convincing:
  blind-signed one-time tokens (Privacy Pass style), so the server never sees
  who spends a token.

**Pre-launch gate:** a DPIA / personvernombud check. With this design the
retained data should be anonymous; the privacy officer should confirm that.

Export for analysis goes through `copilot-survey export`, which reads one
JSON row per respondent on stdin and writes them back redacted:

```sh
psql "$DATABASE_URL" -At -c "SELECT json_build_object('answers', answers,
  'question_versions', question_versions, 'context', context)
  FROM survey_answers WHERE survey_id = 'ID'" | go run . export > export.jsonl
```

- Free text: e-mail addresses, Norwegian phone numbers, 11-digit
  fødselsnummer, IP addresses, GitHub @mentions and URLs with a query string
  become tags such as `[e-post]`. Names are not caught: read the free text by
  hand before analysis.
- k ≥ 5: a context value (client, OS, version, local models) held by fewer
  than 5 rows becomes «annet», and a row whose combination of values is still
  shared by fewer than 5 gets «annet» for all of them.

Answers keep their question ids; `construct` and `reverse` come from the
definition file. Analyse the export, not the table.

## Configuration

NAIS injects the Entra, Texas and database variables. The secret
`copilot-survey` (namespace `copilot`), created by hand in the Nais console,
holds one `SURVEY_KEY_<ID>` per open survey and nothing else. A survey without
its key, or no database, takes no answers (503).

| Variable | Description | Default |
| --- | --- | --- |
| `PORT`, `LOG_LEVEL` | server | `8080`, `INFO` |
| `COPILOT_API_URL` | internal URL of copilot-api | `http://copilot-api` |
| `COPILOT_API_AUDIENCE` | Entra scope for the M2M token | from `NAIS_CLUSTER_NAME` |
| `DB_URL` | Postgres (NAIS `sqlInstances`) | — |

## Development

```bash
mise install
mise check   # fmt, vet, staticcheck, deadcode, lint, test
```

`COPILOT_SURVEY_TEST_DB_URL` runs the store test against a real Postgres.
