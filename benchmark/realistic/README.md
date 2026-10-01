# Realistic coding fixtures (offline preflight)

These tasks are **not** the published `coding` suite. Each task has an agent-visible
`workspace/` and a short `prompt.md`. `controls/` belongs to the evaluator:
never copy it into the agent workspace. The `initial`, `solution`, and `invalid`
states test that the new requirement starts red, existing behaviour starts
green, a known fix passes both, and a tempting shortcut fails. The control
patches are evaluator-only examples, not required solution shapes.

Warm the pinned Kotlin dependencies using the commands in
[`kotlin/README.md`](kotlin/README.md) and build the locked Next.js
dependency image:

```sh
docker build -f benchmark/realistic/typescript/Dockerfile -t nav-benchmark-next:local .
```

Then run
`python3 -B benchmark/realistic/check.py` at the repository root. The check
uses digest-pinned Node 24 and Gradle 8.14.5/JDK 21 images for all four
fixtures. The Next.js image uses the lockfile and digest-pinned Node base;
its image build needs npm registry access, but tests do not. The test
containers run with network disabled, read-only root
filesystems, no Linux capabilities, `no-new-privileges`, and CPU, process,
and memory limits. Gradle uses the locally warmed cache in offline mode.
Docker must be reachable; image pulls and dependency warming are separate
from the network-disabled preflight. Temporary task copies live in this
directory so Docker Desktop can mount them; they are deleted after each
control. No agent or model is started.

The Kotlin fixtures use pinned Kotlin/Ktor dependencies and test HTTP
behaviour using Ktor's test application. The TypeScript fixtures build
with Next.js 16 and TypeScript 6, check evaluator-owned route-handler tests,
and start the production Next.js server to check the actual HTTP symptom.
They do not cover React components or a production deployment.

For an already-completed candidate workspace, preserve an offline evaluation
record outside the agent workspace:

```sh
python3 -B benchmark/realistic/check.py \
  benchmark/realistic/typescript/incident \
  --candidate /path/to/completed/workspace \
  --output /path/to/new/result.json
```

`result.json` records fixture and candidate hashes, UTC evaluation time,
per-suite passed/total checks, diagnostics, evaluation duration and status.
It deliberately records `model`, `effort` and `credits` as `null` with
`usage_verified: false`: the offline evaluator cannot attest to the model
used or the cost of creating that workspace. A broken build or incomplete
test report is `untestable`, never a failed requirement or a success.

The offline evaluator does not enforce the pilot's 1,500-credit limit and is
not a live runner. Before a paid run, provide a provider-side spending limit,
verify the actual model and effort from complete usage events, and specify
how an isolated agent reaches the model without granting its tools external
network access. Keep the evaluator's tests outside the agent workspace and
record all usage, including subagents; see
[`docs/pin-protokoll.md`](../../docs/pin-protokoll.md).

Prepare an attempt without contacting a model:

```sh
python3 -B benchmark/realistic/prepare.py typescript/incident /path/to/new/attempt \
  --model gpt-6-sol --effort medium
```

The new directory holds `workspace/`, `evaluator/fixture/` and `manifest.json`.
The workspace contains the starter, `nav-pilot.agent.md` and every
`instructions/*.instructions.md`, but no other agents or skills. The manifest
records the prompt, requested arm and SHA-256 of every supplied workspace and
frozen fixture file. It does not attest that the agent ran on that arm. Give
an agent access **only** to `workspace/`; keep the manifest, evaluator fixture
and output outside its mount. Store them in an evaluator-owned, read-only
location during a live run: the local copy alone is not immutable.

After a candidate is finished, evaluate it against the prepared fixture:

```sh
python3 -B benchmark/realistic/check.py --attempt /path/to/attempt \
  --output /path/to/new/result.json
```

This rejects a changed fixture. It cannot verify the model, effort, usage or
the isolation of the agent that produced the candidate. Do not run a paid
attempt until those boundaries and the total spending limit are proven.

Probe the intended tool-container policy without a model call:

```sh
python3 -B benchmark/realistic/probe.py
```

The probe writes in the workspace and temporary home, checks that evaluator
files are absent, and requires an external TCP connection to fail inside a
network-disabled, resource-limited Docker container. It demonstrates the
container policy, not that Copilot CLI can use this container for its tools.
Copilot still needs provider egress outside that boundary. Until a runner
demonstrates this split and verifies complete usage against an enforceable
spending limit, keep live attempts disabled.
