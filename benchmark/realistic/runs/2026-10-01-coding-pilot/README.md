# Exploratory coding pilot, 1 October 2026

Four independent attempts: one Sol Medium and one Luna Medium on each of the
TypeScript incident and Kotlin cursor tasks. `*.json` contains the preparation
manifest, offline evaluation, raw `assistant_usage_events` rows, and SHA-256
for candidate source files. `*.patch` contains the candidate source changes.
The source fixtures and evaluator tests live under `benchmark/realistic/`.
Generated build files, dependencies, Copilot transcripts and private host
configuration are not included. The ignored local attempt directories contain
the full candidate trees; apply the patches to a prepared workspace to check
the source changes. Host-level Copilot skills and settings were available but
not frozen in the workspace manifest; reapplying a patch does not recreate
the model's run.

| Task | Model | Completed | Recorded credits | Session |
| --- | --- | --- | ---: | --- |
| TS incident | Sol Medium | yes | 13.433600 | `e3db4667-4803-44f4-b8dd-4cff892d4af0` |
| TS incident | Luna Medium | yes | 0.767018 | `a6786370-dc1a-4360-bb2b-1e54ad6c57ee` |
| Kotlin cursor | Sol Medium | yes | 10.726110 | `72c60a9b-33ce-46f5-ad8d-e3f35e2b75c3` |
| Kotlin cursor | Luna Medium | yes | 0.5687285 | `0d96fe24-be41-419a-9928-31f86913f3ff` |

Total recorded usage: **25.4954565 AI credits**. Every recorded call has the
requested model and Medium effort; no subagent calls appear. The totals
exactly match the published default-context token prices in
`apps/my-copilot/src/lib/model-pricing.ts` (30 September 2026): Sol costs
$2 input, $0.20 cached input, $2.50 cache write and $10 output per million
tokens, versus Luna's $0.10, $0.01, $0.125 and $0.50. That 20× price ratio
explains most of the observed credit gap, not a demonstrated quality gap.
The session database alone cannot prove that no provider calls were omitted.

Copilot CLI `1.0.91-0` ran with `--no-auto-update`, `--max-ai-credits 30`
(soft, per session), `--disable-builtin-mcps`, and a new session for each
attempt. The installed cplt was `2026.10.01-052722-908a5db`, launched with
`--preset strict --proxy-forced --no-audit --no-brief
--no-allow-localhost-any`, project-dir set to the prepared workspace, and
explicit denies for its evaluator and manifest plus the repo Git directory.
The host's cplt config also granted Desktop read, pnpm-store write, and
localhost ports 3000/8080. Host Copilot authentication and config were used.
The agent could not run Gradle tests inside that sandbox; the independent
network-disabled Docker evaluator passed both Kotlin candidates. Both TS
agents installed locked dependencies to run local checks; the evaluator
independently built Next.js and exercised its HTTP endpoint.

All four patches were inspected. Both TS candidates return 400 on the invalid
filter and preserve valid responses. Both Kotlin candidates make `after`
exclusive and retain the original route validation. The evaluator-owned
tests passed for each; agent-edited public tests do not count as evidence.

These are two tasks, one attempt per arm. Do not use the four successes or
their check counts as a completion-rate estimate, pin decision, or `/modeller`
result. Sol remains `@nav-pilot`'s default.
