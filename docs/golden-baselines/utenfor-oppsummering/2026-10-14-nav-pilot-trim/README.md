# Nav-pilot persona trim experiment

These 11 baseline files and 28 transcripts are excluded from `benchmark-summary` under `utenfor-oppsummering`: persona experiments and broken harness runs. Records were moved without alteration.

## Provenance

| Persona source | SHA-256 |
|---|---|
| Pre-trim: `b8a205ce:agents/nav-pilot.agent.md` | `26e2a00675b8411cf6a6df92cd210dc86f548a0906a3e738acc46ec03962651f` |
| Revised: `424ff204:agents/nav-pilot.agent.md` | `2339a650aeb79e667514c482d82cd22d2ae1531ea7f330081f38b5fdd1311148` |

Both n=3 runs used the same corrected harness, SHA-256 `4969373a74e5056e2572e8fbec0646d9aafc87212aac9510508ff0750a65c5c1`. Their `revision: 424ff204` records the execution checkout, not the pre-trim persona source. In `3eeca379`, the committed harness differed from the run version only by this post-run comment:

```sh
# OpenCode rows contain `unknown`; the sum above is not a cost total for those runs.
```

Subsequent final-review fixes changed the executable harness and judge: split-line phase detection, substantive red-zone declarations, bounded transport retries, and OpenCode's `tools: {"*": false}` configuration. The spend example now propagates `unknown`; offline tests are wired into mise and both CI workflows. These fixes were not used in the archived runs. Their recorded harness hash identifies the run version, not the current scripts. No live experiment or judge calibration was rerun for these fixes.

The failed first n=1 run is retained but superseded: the judge CLI failed, and t2/t4 detectors produced false alarms that were fixed after discovery. Both n=3 runs used those fixes and passed 12/12 hard and 3/3 soft checks. The committed judge calibration record shows 73/75 and a passing status; the live command also reported 0 errors, which the record format does not retain.

OpenCode 1.19.0 was asked for `gpt-6.1-sol`; actual model usage was not independently verified. Runs executed concurrently, so recorded elapsed times are concurrent wall-clock observations. OpenCode usage/cost fields are `unknown`. These records support no cost, latency or effectiveness claims. Worker delegation and Nais integration were untested.

OpenCode isolates configuration with temporary `HOME` and `XDG_CONFIG_HOME`. Real `XDG_DATA_HOME`, `XDG_CACHE_HOME` and `XDG_STATE_HOME` remain writable to preserve authentication; session data/history can persist there. This is not full filesystem isolation. The judge now also sets `tools: {"*": false}` alongside denied permissions. OpenCode 1.19.0 accepts that field in [`ConfigV1.Info`](https://github.com/anomalyco/opencode/blob/v1.19.0/packages/core/src/v1/config/config.ts#L129), [converts it to permissions](https://github.com/anomalyco/opencode/blob/v1.19.0/packages/opencode/src/config/config.ts#L567-L577), and [filters disabled tools before the model request](https://github.com/anomalyco/opencode/blob/v1.19.0/packages/opencode/src/session/llm/request.ts#L210-L215). This source check does not independently verify a live model request.

## Reproduce manually

From the repository root with OpenCode 1.19.0, copy each persona outside the checkout via `git show`. These commands use the current harness, including the subsequent fixes above; they do not reproduce the archived harness hash. They make live calls. Outputs go to a fresh external directory.

```sh
scratch=$(mktemp -d)
git show b8a205ce:agents/nav-pilot.agent.md > "$scratch/pre-trim.agent.md"
git show 424ff204:agents/nav-pilot.agent.md > "$scratch/trim.agent.md"
NAV_PILOT_GOLDEN_PERSONA="$scratch/pre-trim.agent.md" \
  ./scripts/nav-pilot-golden.sh --suite planning \
  --client opencode --model gpt-6.1-sol --only 2,3,4,5 --repeat 3 \
  --save-baseline "$scratch/pre-trim-opencode-n3.txt"
NAV_PILOT_GOLDEN_PERSONA="$scratch/trim.agent.md" \
  ./scripts/nav-pilot-golden.sh --suite planning \
  --client opencode --model gpt-6.1-sol --only 2,3,4,5 --repeat 3 \
  --save-baseline "$scratch/trim-opencode-n3.txt"
```
