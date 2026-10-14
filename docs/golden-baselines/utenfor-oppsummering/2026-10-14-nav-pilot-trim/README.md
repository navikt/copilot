# Nav-pilot persona trim experiment

These 11 baseline files and 28 transcripts are excluded from `benchmark-summary` under `utenfor-oppsummering`: persona experiments and broken harness runs. Records were moved without alteration.

## Provenance

| Persona source | SHA-256 |
|---|---|
| Pre-trim: `b8a205ce:agents/nav-pilot.agent.md` | `26e2a00675b8411cf6a6df92cd210dc86f548a0906a3e738acc46ec03962651f` |
| Revised: `424ff204:agents/nav-pilot.agent.md` | `2339a650aeb79e667514c482d82cd22d2ae1531ea7f330081f38b5fdd1311148` |

Both n=3 runs used the same corrected harness, SHA-256 `4969373a74e5056e2572e8fbec0646d9aafc87212aac9510508ff0750a65c5c1`. Their `revision: 424ff204` records the execution checkout, not the pre-trim persona source. After those runs, only this comment was added to `scripts/nav-pilot-golden.sh`, explaining why the committed script hash differs:

```sh
# OpenCode rows contain `unknown`; the sum above is not a cost total for those runs.
```

The failed first n=1 run is retained but superseded: the judge CLI failed, and t2/t4 detectors produced false alarms that were fixed after discovery. Both n=3 runs used those fixes and passed 12/12 hard and 3/3 soft checks. The committed judge calibration record shows 73/75 and a passing status; the live command also reported 0 errors, which the record format does not retain.

OpenCode 1.19.0 was asked for `gpt-6.1-sol`; actual model usage was not independently verified. Runs executed concurrently, so recorded elapsed times are concurrent wall-clock observations. OpenCode usage/cost fields are `unknown`. These records support no cost, latency or effectiveness claims. Worker delegation and Nais integration were untested.

## Reproduce manually

From the repository root, with the corrected harness and OpenCode 1.19.0, copy each persona outside the checkout via `git show`. The commands below make live calls; they are instructions, not part of this archival move. Outputs go to a fresh external directory.

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
