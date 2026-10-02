# Direct-worker feature pilot, frozen protocol

The 2 October `@nav-pilot` feature pair stopped for Luna at a mandatory phase
gate. This is a new cohort, not a continuation or a replacement of that run.
Before any call, both arms use Copilot's default agent, no workspace agent
file or skill, the same sixteen repo instructions and tools, and the same
synthetic task starter and prompt. `prepare.py --direct-worker` records the
supplied files and requested model/effort in each attempt manifest. The
standard `prompt.md` files are unchanged; evaluator tests stay outside the
workspace. No privacy or auth decisions are delegated to the agent: the
synthetic data contains no personal information and there is no external
service or production endpoint. Each run is a fresh noninteractive session
with `--no-ask-user` and a soft `--max-ai-credits 30`. If the default agent
stops to ask for requirements, record that as incomplete; do not resume it.
User-level skills from the host's Copilot home remained available in both
arms. The runs invoked skills, so this is not a skills-free coding baseline.

The frozen protocol ran TypeScript on both arms first, then Kotlin on both
arms from fresh copies. Compare each pair only within the same CLI and cplt
versions. Report full-task completion separately from check counts; do not
compare this cohort's rates with the earlier `@nav-pilot` cohort.

Only evaluator-owned tests establish completion. All results are exploratory;
GPT-6 Sol stays `@nav-pilot`'s default.

## Results

Copilot CLI `1.0.92-0` (`--no-auto-update`) and cplt
`2026.10.01-170947-a36e2bf` were used in both pairs. Each run used
`--max-ai-credits 30` (soft, per session), `--disable-builtin-mcps`, no
custom `--agent`, and Medium effort. cplt used
`--preset strict --proxy-forced --no-audit --no-brief
--no-allow-localhost-any` with explicit denies for the evaluator, manifest
and repository Git directory. The host's cplt config also granted Desktop
read, pnpm-store write and localhost ports 3000/8080; its Copilot home and
authentication were in scope. This is not a network-disabled agent container.

| Task | Model | Session | Usage rows | Credits recorded | Evaluator |
| --- | --- | --- | --- | ---: | --- |
| TS feature | Sol Medium | `cb9a4ad3-7646-465c-ac14-c07de8426b2f` | 16525–16537 | 19.37465 | 1/1 regression, 2/2 acceptance, 1/1 HTTP |
| TS feature | Luna Medium | `cbc2fc27-7740-4abb-be55-d18ca994be6d` | 16538–16545 | 0.6567915 | 1/1 regression, 2/2 acceptance, 1/1 HTTP |
| Kotlin feature | Sol Medium | `264694bf-b1a8-4c63-a809-5c7c7b046558` | 16546–16555 | 15.51321 | 1/1 regression, 2/2 acceptance |
| Kotlin feature | Luna Medium | `a8315410-62fd-4e37-a99c-ca3008f362ac` | 16556–16564 | 0.7138490 | 1/1 regression, 2/2 acceptance |

**36.2585005 AI credits recorded** in this cohort; 80.1609590 across all
ten paid attempts in the pilot. Every recorded call has the requested model
and effort, and none is marked as a subagent. Default-context prices in
`apps/my-copilot/src/lib/model-pricing.ts` reproduce the totals exactly.
The local Copilot session database cannot prove no provider calls were omitted.
Each JSON file contains the preparation manifest, evaluator result, raw
usage rows and source hashes. Each patch is the changed source/public tests
relative to the matching starter; generated dependencies and build files are
excluded. All four patches were inspected. Both TS variants validate before
storing and expose new cases through GET. Both Kotlin variants update through
the repository/service/route layers and preserve the existing GET behavior.
The host's user-level skills, prompts, tool configuration and full transcripts
were not frozen in these artifacts. Reapplying a patch to the versioned
starter reproduces the candidate source; it does not recreate an agent run.

Both Kotlin agents tried to run Gradle inside cplt, but the sandbox blocked
the daemon's localhost connection. This is a shared tool restriction, not a
model failure; the evaluator ran pinned Gradle in a network-disabled Docker
container and passed the hidden and regression tests. Both TS candidates
passed a real Next.js build and production HTTP check in the evaluator.

Four successes on two tasks, once per arm, cannot estimate completion rates
or support the proposed five-percentage-point non-inferiority margin. This
cohort does not test `@nav-pilot` orchestration or warrant changing its
GPT-6 Sol pin. Do not publish it on `/modeller`.
