# Survey definitions

One survey per file, `<id>.json`. These files are the single source of truth:
copilot-survey serves the open ones at `GET /api/v1/surveys/active`, and
nav-pilot and the web render them from there. Changing a question is a pull
request. `go test` (in CI, `TestShippedSurveys`) refuses a file that does not
fit the format below; unknown fields are refused too.

```json
{
  "id": "q4-2026",
  "series": "utviklerundersokelsen",
  "title": "Shown as the survey's name",
  "active": false,
  "nudge": "calm",
  "intro": "Optional, shown before the first question",
  "starts": "2026-10-01",
  "ends": "2026-11-30",
  "questions": [
    {"id": "tilfredshet", "version": 1, "type": "scale", "construct": "space-satisfaction",
     "text": "…", "min": 1, "max": 5,
     "labels": ["Helt uenig", "Uenig", "Nøytral", "Enig", "Helt enig"], "required": true},
    {"id": "erfaring", "version": 1, "type": "choice", "text": "…", "options": ["0–2", "3–5"]},
    {"id": "verktoy", "version": 1, "type": "multi", "text": "…",
     "options": ["Copilot", "Cursor", "Jeg bruker ikke KI-kodeverktøy"], "other": "Annet", "max_length": 100},
    {"id": "verdi", "version": 1, "type": "multi", "text": "…", "options": ["A", "B", "C"], "max_choices": 3,
     "skip_if": {"question": "verktoy", "answer": "Jeg bruker ikke KI-kodeverktøy"}},
    {"id": "opplevelse", "version": 1, "type": "text", "text": "…", "max_length": 1000}
  ]
}
```

## Schema, for other clients

[`schema.json`](schema.json) is the format as a JSON Schema (draft 2020-12),
served at `GET /api/v1/surveys/schema` on copilot-survey and copilot-cli, no
sign-in. `GET /api/v1/surveys/active` returns `{"surveys": [...]}`, each item
a definition that fits it. CI checks every file here against it
(`TestShippedSurveysMatchSchema`), and checks that copilot-survey's structs,
nav-pilot's and the web's types name the same fields.

- `schema_version` in the schema goes up on every change to the format.
- New fields are optional, so a definition or a client that works today keeps
  working. A client ignores fields it does not know, and skips a survey that
  has a question `type` it does not know (nav-pilot says it needs an upgrade).
- copilot-survey checks more than the schema can say: unique question ids,
  `skip_if` pointing at an earlier `choice` or `multi` question and one of its
  options, `starts` before `ends`, `labels` one per step, at most one `text`
  question. A definition that fits the schema can still be refused at start.
  The reverse holds too: the schema refuses fields that do not belong to a
  question's type (`options` on a `scale`), which copilot-survey ignores; CI
  runs both on every file here.
- To answer, `POST /api/v1/surveys/{id}/responses` with
  `{"answers": {question id: value}, "context": {...}}`: a number for `scale`,
  the option text for `choice`, a list of option texts for `multi`, a string
  for `text`. An `other` label counts as an option; its text goes under
  `"<id>.other"`, and only when the answer includes the label. Leave out a question that is skipped or not answered. The
  caller must be copilot-cli or my-copilot (see [the README](../README.md#callers)).

## Comparing waves

A survey that repeats (a *series*) is compared wave by wave at the aggregate
level: answers are not linked to a person across waves (see
[the data model](../README.md#surveys-data-model-and-retention)).

- Each wave is a new file and a new `id`; `series` groups them.
- A question keeps its `id` across waves. `version` starts at 1 and goes up
  on any change to its text, options, labels or scale that could change how
  people answer it. Same `id` and
  `version` in two waves means the results compare; a higher `version` means
  they do not, and the pull request says why.
- A question that is dropped is simply absent; its `id` is never reused for
  something else.
- `construct` names what a question measures (for analysis) and `reverse`
  marks a scale where agreeing is the negative end. Clients ignore both.

## Before a survey opens

- Its definition is merged here and reviewed by the survey owner.
- A real user in dev has checked that their Entra `preferred_username` and
  their navikt SAML `nameId` are the same address (else one person can answer
  twice, from nav-pilot and the web).
- The ingress access log has been checked for what it records of
  `POST /api/v1/surveys/…` (source address per device, or per naisdevice
  gateway).
- DPIA / personvernombud has signed off, including the residual risks in
  [the data model](../README.md#surveys-data-model-and-retention). One of
  them: a batch is written at the tenth answer (or the next one after a
  failed write), so write timing places an answer in a batch window for
  anyone with ingress log and database access.
- Only then: `SURVEY_KEY_<ID>` (`openssl rand -base64 32`) is added to the
  `copilot-survey` secret, and a pull request sets `"active": true`. Without
  both the survey takes no answers.
- No copilot-survey deploys while it is open (a restart drops queued answers).

## When it closes

The day after `ends`, copilot-survey writes the last answers and deletes the
survey's participation rows. The survey owner deletes `SURVEY_KEY_<ID>` from
the secret the same day, then restarts the pod
(`kubectl rollout restart deployment/copilot-survey -n copilot`): the running
pod read its keys at start and still holds the deleted one. copilot-survey
warns at start while the key is still in the secret.

## Fields

- `id`, `series`, question ids: lowercase letters, digits and `-`.
- `active`: false (or left out) until the survey is to go live. A survey is
  served and takes answers only when `active` is true *and* today is within
  `starts`–`ends`. Setting it true is its own pull request, approved by the
  survey owner, after the checklist below.
- `nudge`: when nav-pilot mentions the survey unasked. `calm` (default) asks
  after a session ends. `start` prints a one-line hint as a session starts,
  pointing at `nav-pilot survey`. `off` leaves it to `nav-pilot survey`.
  `calm` and `start` happen at most three times per person, and never without
  a terminal, in CI or when opted out.
- `starts`, `ends`: first and last day it takes answers (UTC).
- `scale`: whole numbers from `min` to `max` (at most 11 steps). `labels`, if
  given, names every step. Stored as the number.
- `choice`: one option. `multi`: one or more, at most `max_choices` if set.
  At least two options, no duplicates. Stored as the option text.
- `other` (`choice`, `multi`): the label of one more option, shown last, that
  takes a short free text, such as «Annet: ___». Choosing it stores the label,
  like any option; the text is optional, sent and stored as `<id>.other`, at
  most `max_length` characters (1 to 200). At most three per survey. It is free text like a `text`
  answer and treated the same way (see [free text](#free-text)). Not one of
  `options`, and not a `skip_if` answer.
- `text`: `max_length` from 1 to 2000 characters. At most one per survey:
  it is the answer most likely to name its author.
- `skip_if`: skip this (optional) question when an earlier `choice` answer is,
  or `multi` answer includes, `answer`.
- `required`: must be answered unless skipped.

## Free text

`text` answers and `other` texts are the answers that can name their author
(«as the only Rust developer on team X»). Both are trimmed, checked as text
and capped by `max_length`; nav-pilot and the web ask people not to write
anything that identifies anyone; and both go through a redaction pass before
analysis and are never quoted next to their segment values. A survey keeps to
one `text` question and at most three `other` options, each capped at 200
characters: they say what the listed options missed, not how someone feels.

## End-to-end test in dev

`dev-e2e-test.json` is a dummy that ships inactive. In dev-gcp only,
`survey_active_ids: dev-e2e-test` in `.nais/dev-gcp.yaml` sets
`SURVEY_ACTIVE_IDS`, which opens it; copilot-survey ignores the variable in
any other cluster, so it can never open a survey in prod. It also needs
`SURVEY_KEY_DEV_E2E_TEST` in the dev `copilot-survey` secret.

- Test: `NAV_PILOT_COPILOT_CLI_URL=https://copilot-cli.intern.dev.nav.no nav-pilot survey`,
  then the form on ki-utvikling.ekstern.dev.nav.no/nav-pilot/undersokelse.
  The second answer from the same person, on either, gets 409.
- Nothing reaches the database until the tenth answer: one tester's answer
  stays queued in memory. `survey_submissions_total{survey="dev-e2e-test"}`
  counts it, and the next restart logs it as dropped.
- Close it: remove `survey_active_ids` from `.nais/dev-gcp.yaml`.
- Reset: restart the pod (`kubectl rollout restart deployment/copilot-survey -n copilot`
  in dev-gcp) to drop queued answers. To let the same people answer again
  after a batch was written, replace `SURVEY_KEY_DEV_E2E_TEST` with a new key
  and restart; old participation rows then match no one. Written rows go by
  themselves: participation the day after `ends`, answers 180 days later.
