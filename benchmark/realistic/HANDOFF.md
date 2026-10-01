# Restart point for the realistic coding pilot

Branch: `benchmark/realistic-pilot-preflight`. Decision: keep GPT-6 Sol as
`@nav-pilot`'s default. No live model calls have been made for these fixtures.
The exploratory budget is 1,500 AI credits in total, including retries and
subagents. Five attempts per arm cannot support a pin change. Do not publish
these results on `/modeller`.

## Saved state

- `docs/pin-protokoll.md` records the two separate comparisons: Luna Medium
  versus Sol Medium on bounded coding work, then Sol Low versus Sol Medium on
  orchestration tasks if budget remains. The five-percentage-point tolerance
  belongs to a later study with an adequate sample.
- `benchmark/realistic/` has four synthetic Kotlin/Ktor and Next.js tasks.
  Agent-visible starters are separate from acceptance and regression tests.
  `check.py` validates baseline, known-good and invalid controls without
  model calls, or grades an already-completed candidate in network-disabled
  Docker containers.
- `prepare.py` copies a fresh attempt and records file hashes, requested arm
  and prompt. `check.py --attempt` rejects a changed evaluator fixture.
  `probe.py` confirms that a test container can write to its mounted workspace
  and temporary home, cannot see the evaluator files, and cannot connect to
  an external TCP address. None of these scripts starts Copilot CLI.
- The current preparation installs only `nav-pilot.agent.md` and all
  `instructions/*.instructions.md`, with no skills or other agents. This
  selection is recorded in each manifest and is not a full installation.

## Resume here

1. Confirm the branch and worktree are clean. Run
   `python3 -B benchmark/realistic/check.py`,
   `python3 -B benchmark/realistic/probe.py`,
   `python3 -B -m unittest discover -s benchmark/realistic -p 'test_*.py'`
   and `mise run benchmark:check`. Prebuild the TypeScript image and warm
   Kotlin dependencies as described in `README.md` if needed.
2. Resolve #1409's live-agent boundary. Copilot must reach its model, while
   the agent's tools must have no external network, secrets or access to
   evaluator files. The Docker probe proves a container policy only; it does
   not put Copilot's tools in that container. `cplt --preset strict` permits
   provider egress and has not been shown to meet #1409's requirement. Test
   denied tool egress and fixture access through the *actual* runner before
   any paid call.
3. Obtain and verify an enforceable provider-side spending boundary for the
   1,500-credit **total**. Copilot CLI's `--max-ai-credits` is a soft
   per-session cap and can overshoot. Do not run a paid attempt if a hard
   boundary cannot be demonstrated.
4. Finish the runner and evidence record: pinned CLI and artifact versions,
   identical fresh sessions, read-only evaluator inputs, candidate snapshot,
   verified observed model and effort for every call, complete credits for
   retries and subagents, and explicit timeout, CLI-error and untestable
   outcomes. Preserve raw usage and evaluation output outside the workspace.
5. Only after those gates pass, run the small worker comparison on shared
   Kotlin and TypeScript tasks. Keep orchestration separate. Inspect failed
   patches; report full-task success separately from check-level pass rates.
   #1410's sequential-change pilot comes later.

Issues: [#584](https://github.com/navikt/copilot/issues/584) (decision),
[#1409](https://github.com/navikt/copilot/issues/1409) (coding tasks),
[#1410](https://github.com/navikt/copilot/issues/1410) (sequential changes).
