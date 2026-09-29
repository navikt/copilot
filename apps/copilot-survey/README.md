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
from `accessPolicy.inbound`. `X-On-Behalf-Of` is honoured on the submit route
only.

## Endpoints

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `GET` | `/api/v1/surveys/active` | none | Open surveys from [`surveys/`](surveys/README.md) |
| `GET` | `/api/v1/surveys/schema` | none | The JSON Schema of a definition, [`surveys/schema.json`](surveys/schema.json) |
| `POST` | `/api/v1/surveys/{id}/responses` | see above | Submit answers: 201, 409 already answered, 400 invalid, 401 caller refused, 403 no Nav identity, 404 not open, 503 not taking answers |
| `GET` | `/health`, `/ready`, `/metrics` | none | Probes and Prometheus (`survey_submissions_total{survey,status}`) |

No OpenTelemetry, no auto-instrumentation, no body logging, no retry that
buffers a body. Logs carry survey id, status, batch size and error type,
never who answered. A batch write runs on its own background context, never
the request's, so the write of all ten carries nothing from the tenth
submitter's request.

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
   one person, or that person can answer twice. Check with a real user in
   dev that `preferred_username` and the SAML `nameId` agree before launch.
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
| `survey_answers` | `survey_id`, `answers` (`{question id: value}`, and `{question id}.other` for the text of an «other» option), `question_versions` (`{question id: version}`), `context` (nav-pilot version as year.month, OS, client, local models on/off), `delete_after` |

No row id, no timestamp, no request id, no IP, no login, oid, NAVident,
e-mail or token in either. Because nothing links an answer to its
participation row, **an answer cannot be changed or withdrawn** after it is
sent; nav-pilot and the web say so before sending.

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
close-out has written the last batch. A pod that starts after the survey
closed loads no key for it. From then on no key exists to recompute a hash,
and no table holds one.

**Pseudonymous while open.** Until the key and the participation rows are
deleted, anyone who holds both the key and database access (members of the
`copilot` Nais team) can test whether a given e-mail answered: Nav e-mails are
enumerable, so this is a trivial dictionary test. Without the key the hash
cannot be reversed (HMAC-SHA256, 256-bit random key). copilot-survey logs a
warning at start for every key whose survey has closed.

**1 of k.** Because both tables are written only in batches of k = 10 per
survey, in one transaction, with rows shuffled, commit time, transaction id
and row order place an answer among the 10 participants of its batch, no
fewer. That is k before the answer's own content narrows it: someone with the
key and the database can name the 10, and if only one of them uses Windows,
or opencode with local models, the answer with those context values is that
person's. This is why the context is kept coarse (no CPU type, version as
year.month) and why exports suppress small segments.

The exception is the survey's last batch, written when it closes with
whatever is left (1 to 9): its participation rows are deleted in the same
close-out, but WAL and backups keep them for the backup retention period.

Submissions still queued are lost on a restart (at most 9 per survey while
the database is healthy; more if a batch write is failing). Their senders
were told "recorded", and nav-pilot does not ask them again (it remembers
locally that they answered), so those answers are gone for good. The deploy
freeze in [surveys/README.md](surveys/README.md) is what keeps this rare.
Writing the queue early instead would break the 1 of k.

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
- Colluding insiders: k assumes the other 9 in a batch are real respondents.
  One account answers once per survey, but a group of 9 insiders answering
  together could pin the 10th.
- Free text: the answers that can name their author ("as the only Rust dev
  on team X"): a `text` question (at most one per survey) and the short text
  of an «other» option (at most 200 characters each, but a survey can have
  several). They are stored in the same row as the rest of that person's
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
- Batch timing: the tenth submission's request writes the batch (or the
  next submission's, after a failed write). That request is slower than the
  other nine, and the commit follows within milliseconds, so someone with both
  database access (the `copilot` Nais team) and the ingress access log can
  tell who closed batch N and when each batch was written. That places an
  answer in one batch window, never below the 10 of its batch, which is what
  k = 10 already concedes. Accepted (#1107): closing it would need a jittered
  or random-size flush, which holds more answers in memory and loses them all
  on a restart.
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
