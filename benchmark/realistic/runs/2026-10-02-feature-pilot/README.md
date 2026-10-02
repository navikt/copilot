# Exploratory feature task, 2 October 2026

Same TypeScript feature starter and prompt for one Sol Medium and one Luna
Medium attempt, each with a fresh workspace and session. Copilot CLI
`1.0.92-0` (`--no-auto-update`) and cplt
`2026.10.01-170947-a36e2bf` were used for both. cplt ran with
`--preset strict --proxy-forced --no-audit --no-brief
--no-allow-localhost-any`, project-dir set to the attempt workspace, and
explicit denies for its evaluator, manifest and the repo Git directory.
The host cplt configuration also granted Desktop read, pnpm-store write and
localhost ports 3000/8080. Host Copilot auth/config were used.
User-level Copilot skills were available but not frozen; this cohort invoked
skills. Reapplying a patch reproduces source changes, not the model run.
`--max-ai-credits 30` was a soft session limit, and built-in MCP servers
were disabled. The Kotlin/Ktor feature was not run because this pair
exposed a task-protocol problem first.

| Model | Session | Usage rows | Recorded credits | Evaluator |
| --- | --- | ---: | ---: | --- |
| Sol Medium | `29b557c4-5e6d-427f-b328-d90a3654a325` | 16508–16520 | 17.843860 | Completed: 1/1 regression, 2/2 acceptance, 1/1 HTTP |
| Luna Medium | `fbeb7eb0-dc07-4d71-a5e7-fa5b5979ecfb` | 16521–16524 | 0.563142 | Failed: 1/1 regression, 0/2 acceptance, 0/1 HTTP |

The Luna candidate made no source changes. `@nav-pilot` classified the new
endpoint as a full-phase request and stopped after asking about privacy and
access. The Sol candidate implemented the endpoint; its evaluator-owned
tests passed. Neither outcome is a controlled comparison of coding ability:
the persona's phase boundary affected the observed task completion. Do not
resume Luna's session or change its prompt after seeing this result and then
count it as the original attempt. Decide up front whether a later coding
comparison invokes a worker directly, supplies all required answers in a
frozen prompt, or counts phase stops as task failures; rerun *both* arms
under that new protocol.

The Sol agent installed locked dependencies before running local checks and
temporarily edited `tsconfig.json`; it restored that file to its original
hash. The Next.js build generated `next-env.d.ts`. The patch in this folder
contains only the application code and public test changes; generated files
and dependencies are excluded. The two JSON files hold preparation hashes,
evaluator results, raw usage rows and source hashes. Every recorded call
used the requested model and Medium effort, with no subagents. Default-context
prices in `apps/my-copilot/src/lib/model-pricing.ts` reproduce both credit
totals exactly. The local session database does not prove all provider calls
were captured.

These are exploratory observations, not a pin decision or `/modeller` result.
GPT-6 Sol stays `@nav-pilot`'s default.
