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
    {"id": "verdi", "version": 1, "type": "multi", "text": "…", "options": ["A", "B", "C"], "max_choices": 3,
     "skip_if": {"question": "verktoy", "answer": "Jeg bruker ikke AI-kodeverktøy"}},
    {"id": "opplevelse", "version": 1, "type": "text", "text": "…", "max_length": 1000}
  ]
}
```

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
- DPIA / personvernombud has signed off.
- Only then: `SURVEY_KEY_<ID>` (`openssl rand -base64 32`) is added to the
  `copilot-survey` secret, and a pull request sets `"active": true`. Without
  both the survey takes no answers.
- No copilot-survey deploys while it is open (a restart drops queued answers).

## When it closes

The day after `ends`, copilot-survey writes the last answers and deletes the
survey's participation rows. The survey owner deletes `SURVEY_KEY_<ID>` from
the secret the same day; copilot-survey warns at start while it is still there.

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
- `text`: `max_length` from 1 to 2000 characters. At most one per survey:
  it is the one answer that can name its author.
- `skip_if`: skip this (optional) question when an earlier `choice` answer is,
  or `multi` answer includes, `answer`.
- `required`: must be answered unless skipped.
