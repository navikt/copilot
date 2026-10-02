# Realistic coding pilot: current state

Keep GPT-6 Sol as `@nav-pilot`'s default. The four synthetic Kotlin/Ktor and
Next.js tasks have offline baseline, known-good and invalid controls. This
pilot compares Luna Medium with Sol Medium on bounded coding; Sol Low versus
Medium on orchestration is a separate question. Do not publish these runs on
`/modeller` or use them to change the pin.

The exploratory target was at most 1,500 AI credits across calls, retries
and subagents. The ten paid attempts recorded 80.1609590 AI credits.
Copilot's `--max-ai-credits` is a soft per-session limit and an in-flight call
can overshoot it. No further paid repetitions are planned.

## Follow-ups

Do not start more paid repetitions by following the earlier run procedure.
Concrete follow-ups are tracked in issues:

- [#1409](https://github.com/navikt/copilot/issues/1409): decide whether its
  original live-runner and network-disabled agent-container requirements
  remain. The manual cplt pilot did not meet them.
- [#1413](https://github.com/navikt/copilot/issues/1413): define and validate
  orchestration tasks offline before considering Sol Low versus Medium.
- [#1414](https://github.com/navikt/copilot/issues/1414): preregister task
  sample, uncertainty and cost for any later pin decision. If unaffordable,
  report "undecided" and keep Sol.
- [#1410](https://github.com/navikt/copilot/issues/1410): sequential changes
  are a separate deferred experiment. Persona-regression follow-ups stay in
  [#584](https://github.com/navikt/copilot/issues/584) and
  [#1408](https://github.com/navikt/copilot/issues/1408).

## First paired run, 1 October 2026

Copilot CLI `1.0.91-0` (`--no-auto-update`), installed cplt
`2026.10.01-052722-908a5db`, `@nav-pilot`, Medium effort, same starter and
prompt in each pair. Fresh session and prepared workspace per attempt;
`--max-ai-credits 30` was a soft session limit. `--disable-builtin-mcps` was
set. cplt enforced `strict --proxy-forced` and denied the evaluator and repo
Git directory. The host's existing cplt configuration additionally granted
Desktop read, pnpm-store write and localhost ports 3000/8080, and made its
Copilot home and auth available. Those grants were visible in the startup
summary. The agent's own test commands ran into setup restrictions: Gradle
could not connect to its localhost daemon; TypeScript first lacked `tsc`
until dependencies were installed. The separate Docker evaluator passed.

| Task | Model | Session | Usage row IDs | Credits recorded | CLI time | Evaluator |
| --- | --- | --- | --- | ---: | ---: | --- |
| `typescript/incident` | Sol Medium | `e3db4667-4803-44f4-b8dd-4cff892d4af0` | 16480–16487 | 13.433600 | 47 s | 1/1 regression, 2/2 acceptance, 1/1 HTTP |
| `typescript/incident` | Luna Medium | `a6786370-dc1a-4360-bb2b-1e54ad6c57ee` | 16488–16495 | 0.767018 | 51 s | 1/1 regression, 2/2 acceptance, 1/1 HTTP |
| `kotlin/cursor-debug` | Sol Medium | `72c60a9b-33ce-46f5-ad8d-e3f35e2b75c3` | 16496–16501 | 10.726110 | 28 s | 2/2 regression, 2/2 acceptance |
| `kotlin/cursor-debug` | Luna Medium | `0d96fe24-be41-419a-9928-31f86913f3ff` | 16502–16507 | 0.5687285 | 28 s | 2/2 regression, 2/2 acceptance |

Total recorded usage: **25.4954565 AI credits**. Every database usage row for
these sessions has the requested model and `medium` effort; none has a
subagent ID. The local Copilot database cannot prove that every provider call
was recorded, so treat those credits as observed usage, not a certified total.
`check.py` reports `model`, `effort` and `credits` as `null` for the same
reason. Raw usage, preparation manifests, evaluator reports, source hashes
and reviewed patches are preserved in
[`runs/2026-10-01-coding-pilot/`](runs/2026-10-01-coding-pilot/README.md).
The local full candidate workspaces remain ignored; the committed patches
allow the source changes to be reconstructed. Published default-context
token prices reproduce every recorded credit total exactly, explaining the
roughly 18-fold observed gap. These are **one attempt per arm per task**:
their four successes say nothing reliable about comparative completion rates
or the five-percentage-point margin.

## Feature attempt, 2 October 2026

The installed client changed to Copilot CLI `1.0.92-0` and cplt
`2026.10.01-170947-a36e2bf`. One TypeScript feature attempt per arm used
matching starter, prompt, tools and Medium effort; their records are in
[`runs/2026-10-02-feature-pilot/`](runs/2026-10-02-feature-pilot/README.md).
Sol completed all evaluator checks at 17.84386 recorded credits. Luna used
0.563142 credits, made no code changes and stopped at the persona's full-phase
question gate; its unchanged starter failed 0/2 acceptance and 0/1 HTTP
checks. This is an observed phase-policy difference, not a controlled test
of coding ability. The Kotlin feature pair was prepared but **not run**.
Stop paid feature calls until the prompt/agent protocol is fixed for both
arms before a fresh pair; do not resume the incomplete Luna session and call
it an independent attempt. Total recorded usage for the six paid sessions is
**43.9024585 AI credits**. A session database cannot prove all provider calls
were recorded.

## Direct-worker feature pair, 2 October 2026

The protocol was fixed *before* the new calls: omit `@nav-pilot` and its
phase gates in both arms, retaining the same sixteen repo instructions and
the same starter/prompt for each task. See
[`runs/2026-10-02-direct-worker/`](runs/2026-10-02-direct-worker/README.md).
All four direct-worker attempts passed the external evaluator. Recorded usage
was 36.2585005 AI credits in this cohort, 80.1609590 across all ten paid
attempts. The workers' Kotlin Gradle commands could not connect to the
daemon inside cplt; the external Docker evaluator ran successfully. None of
these small exploratory results justifies changing `@nav-pilot`'s default or
inferring a completion-rate difference. Stop here before buying repeats.

The three cohorts are separate. The first debugging cohort passed four of
four attempts; the `@nav-pilot` feature pair passed one of two after Luna
stopped at a mandatory phase gate; the direct-worker feature cohort passed
four of four. Those fractions are observations, not estimates of population
success rates. Sol used more recorded credits, primarily because the
published default-context rates are 20 times Luna's. The checks measure
task end state, not the quality of the agent's reasoning or its ability to
complete future tasks. Host-level Copilot skills and settings were available
but not frozen, and a patch recreates source changes rather than the session.

One earlier isolated-HOME diagnostic exited at authentication before a model
call. [cplt PR #686](https://github.com/navikt/cplt/pull/686) fixes that
approach's separate native-addon issue; the paid runs above used the installed
cplt and the host's ordinary authentication.

Issues: [#584](https://github.com/navikt/copilot/issues/584),
[#1409](https://github.com/navikt/copilot/issues/1409),
[#1410](https://github.com/navikt/copilot/issues/1410).
