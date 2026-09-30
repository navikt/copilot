# Local dispatch: how hard to push work to the local worker

Status: design, implemented in part (see [What ships first](#what-ships-first)).
The built-in default is `aggressive` since 30 September 2026: `balanced` failed its
decision rule against same-day controls (see
[Balanced against same-day controls](#balanced-against-same-day-controls)).

## Problem

A developer who has turned local inference on expects mechanical work to go to
the local model. Today that depends on whether the cloud orchestrator reads the
dispatch policy and chooses to follow it. Whether it does depends on the model:

- In August, Sonnet 4.6 dispatched 23 of 24 hybrid samples under the older policy text.
- In September, Sonnet 5 dispatched 1 of 23 across four probes, even after #996 and #997
  (navikt/mlx-workspace pending-tasks §8.8, transcripts in
  `mlx-workspace/.bench-logs/dispatch-probe*`). It scripts the edits itself, one `sed` per
  file even when each site gets a different value. It quotes the persona's Trivial tier as
  its reason, and it calls jobs over the threshold "small".

Prose alone is not a reliable control, and its effect moves with every model
release. This matters more as the tiers go up: a 64 GB or 128 GB machine runs a
worker trusted with more classes, and a user who has paid for that hardware
should not depend on the orchestrator's mood.

## Where dispatch exists at all

Only opencode has a cloud orchestrator with a local worker in one session
(`startLocalDispatch`, `opencode_launch.go`). Copilot CLI points the whole session
at one provider (`COPILOT_PROVIDER_BASE_URL`), so a Copilot session is either all
local or all cloud and there is nothing to dispatch to. pi has no worker either.
Everything below applies to opencode hybrid sessions. The setting is accepted
for every client and does nothing where there is no worker.

## The setting

```toml
local_dispatch = "aggressive"   # off | conservative | balanced | aggressive
```

Per run: `nav-pilot --local-dispatch aggressive`. The flag wins over the file.
The key only matters once `local_enabled` is true, which is what `alpha local init`,
`alpha local setup` and `alpha local on` set. Without a `local_dispatch` key the
built-in default is `aggressive`, for new and existing setups alike (see
[Balanced against same-day controls](#balanced-against-same-day-controls)). An
explicit value wins. `off` means "worker not offered": it does
not turn local inference off (that is `alpha local off`), because a local session
model still works. It stops offering the worker to a cloud orchestrator.

Neither enforced level lowers the sizes. The August routing measurement saved
credits only where the cloud would have needed about 5 steps or more, and #997
measured a one-step dispatch at the same credits and 2–3× the time. What
enforcement adds is that the sizes hold.

| | off | conservative | balanced | aggressive (built-in default) |
|---|---|---|---|---|
| Worker offered to a cloud orchestrator | no | yes | yes | yes |
| Classes sent | none | manifest-trusted | manifest-trusted | manifest-trusted |
| `edit-multi-mechanical` size to send | none | ≥ 10 files or ≥ 20 call sites | ≥ 5 files or ≥ 10 call sites (#997) | same as balanced |
| "If you doubt it, do it yourself" | n/a | yes | no | no |
| Persona tiers vs policy | n/a | the orchestrator's judgement wins | the policy wins, and the text says the tiers do not decide who writes files | same as balanced |
| Gate: multi-file rule (`edit-multi-mechanical` trusted) | no | no | checkpoint: 1 refusal per turn, a retry passes | dispatch-first: 2 refusals per turn, the file passes once sent |
| Gate: create-file rule (`create-file` trusted) | no | no | no | yes |

Rules that hold at every level:

- **Only trusted classes.** A class the manifest does not mark `trusted` in delegate mode is
  never named as work to send, and the gate never denies on its account (the one exception,
  `deny_tmp`, is about a path, not a class). The level changes
  how hard nav-pilot pushes, never what it considers safe to push.
- **Byte-stable prompt.** The policy stays a pure function of (model, level, loop-guard
  thresholds), so the prompt cache holds within a session.
- **A single search-and-replace stays with the orchestrator** at every level. #997 measured
  that dispatching one costs the same credits and takes 2–3× longer.

### Why the default enforces

The first draft kept `balanced` prose-only and put the gate at `aggressive`
alone, to be promoted after measurement. Probe 5 settled it. With a policy
that said in so many words to send new test files, Sonnet 5 dispatched 0 of 6
create-file samples. Across probes 1–5 it dispatched 1 of 29. In 4 of those 6
samples it never mentioned the worker, and in the other 2 it cited the
persona's Compressed tier. A prose-only default does nothing on the model most
developers run, so `balanced`, until 30 September the built-in default for everyone who has turned local
inference on, enforces the rule the evidence supports: the multi-file split,
at the sizes #997 measured.

It does so as a **checkpoint**. The gate cannot tell an ordinary five-file
feature (implementation, test, wiring, docs, config) from a mechanical change,
and the false-positive cell is not measured yet. So at `balanced` the first
refusal in a turn asks for the rest to go to the worker and says that the same
edit again goes through, and that is the only refusal the turn gets. `aggressive`
is the hard form: dispatch first, two refusals per turn. `aggressive` adds new files. `conservative` is the
prose-only level, for a model that dispatches too eagerly (Sonnet 4.6 sent 23
of 24).

### Why new setups get aggressive

Dispatch re-probe 7 (mlx-workspace §8.8, binary 2bcca023 with #1114, Sonnet 5 as
orchestrator) measured both enforced levels after probe 6's fixes:

- `aggressive` dispatched in 17 of 17 valid samples on the large and create-file
  cells, and all 17 passed verification. Counted by dispatched attempts it is 17
  of 19: the other 2 are sessions the harness ended on a rejected `/tmp` write
  (below). The pass is the orchestrator's, not the worker's alone: on create-file
  the orchestrator redid 15 of the 27 worker tasks. That includes r6 (60 call sites in 3
  files), which probe 6 never sent. The small cell had no false positive (0 of 5).
  Cloud cost was 0.83–2.10× the cloud-only control, below it on one create-file
  cell only, and wall time 2.7–3.6×.
- `balanced` dispatched in 2 of 20. That is the checkpoint working as designed:
  one refusal, and the same edit again passes.

Probe 6 kept `balanced` because `aggressive` failed on quality (5 of 6 dispatched
samples passed). On valid samples that line now holds, so a developer who turns local inference on
gets the level that sends work. The price is wall time and, on most cells, cloud
credits; quality held.

At the time, nobody who already had local inference was moved. An upgrade must
not change how an existing session behaves, so the built-in default in code
stayed `balanced`, and only the setup commands wrote `aggressive`, into a config
that had never had local inference configured. The section below superseded this.

`balanced` itself does not pass the measurement plan's decision rule on these
numbers. Its cost was 1.58× control on r4 and 1.61× on r6, which by that rule
drops `balanced` back to prose. But those controls come from probes 4–6, run on
other days, so the finding was to re-run the controls before acting.

### Balanced against same-day controls

This supersedes the two paragraphs above. The controls were re-run on 29 September
([report](https://github.com/navikt/mlx-workspace/blob/main/reports/2026-09-28-balanced-controls/results.md)): 5 controls, then 5 `balanced` samples per cell (r5 stopped at the cost cap
after 3 controls), same binary and worker. The pass rate was the same (15 of 15, and 5 of 5 on the false-positive
cell), but cloud cost was 1.57× control on r4 and 1.49× on r6. Every `balanced`
sample on those two cells cost more than every control sample (exact
Mann–Whitney, 5 against 5, p = 0.008 each). Two of three large rungs over
control fails the decision rule, whatever r5 would show. The small cell also
cost 1.39× with no refusal and no dispatch, so the extra cost is not only
refusals.

Decision (maintainer, 30 September): the built-in default becomes
`aggressive`. This moves existing local setups without a `local_dispatch` key,
which the rule above ("an upgrade must not change how an existing session
behaves") would normally forbid. It is accepted because local inference is an
alpha with few users. An explicit value is kept. The way back is
`nav-pilot config set local_dispatch balanced`. The setup commands no longer
write the key, since the default now does the same. `balanced` itself stays as
it is: going back to prose is a separate decision.

Three more samples were invalid: the orchestrator made a `/tmp` backup for the
break-and-undo check in the verify text, the backup was auto-rejected, and the
session ended. In one of them the production code was left broken. That is
navikt/copilot#1237, fixed before this default shipped: the verify text now keeps the
undo inside the project, and the gate refuses a `/tmp` path (`deny_tmp`), so the
session goes on. See "Checking the worker's result" below.

## Mechanisms, most reliable first

### (b) Enforcement: a pre-tool gate (balanced and aggressive)

**Client capability, checked rather than assumed.**

- *opencode*: a plugin's `tool.execute.before` hook runs before every tool call. Throwing
  from it aborts the call, and the error text reaches the model as the tool result. This
  was measured in #709 (`opencode_sync.go`), and the rtk plugin (`~/.config/opencode/plugins/rtk.ts`,
  no longer installed by nav-pilot since #1332) used the same hook. opencode loads every `*.js`/`*.ts` in `~/.config/opencode/plugins/`
  (`config/plugin.ts`, glob `{plugin,plugins}/*.{ts,js}`). **`--pure` disables all external
  plugins** (`plugin/index.ts`: `flags.pure ? [] : …`), so a `--pure` run has no gate.
  nav-pilot warns at launch when it sees `--pure` on a launch it would gate.
- *Copilot CLI*: preToolUse exists and fails closed, and its deny reason is shown to the
  model. It is not needed: a Copilot session has no worker.

**Shape.** Two pieces, following the rtk precedent:

1. **The plugin**, `~/.config/opencode/plugins/nav-pilot-dispatch-gate.js`. It is a thin
   shim with no logic. It records each session's agent and turn from `chat.message`,
   and takes the agent from `chat.params` only when `chat.message` has not named one.
   Compaction runs its own agent through `chat.params` on the same session. On
   `tool.execute.before` it POSTs `{session, turn, agent, tool, path, create, command,
   subagent, prompt, replaceAll, old}` to the
   URL in `NAV_PILOT_DISPATCH_GATE`, with a 2 s timeout, and throws the returned `deny`
   text, if there is one. It does nothing when:
   - the variable is unset, which covers opencode run outside nav-pilot, cloud-only
     launches, `conservative` and `off`;
   - the session is not top-level (`client.session.get` shows a `parentID`, or the lookup
     fails). A refusal inside a subagent's session fails that whole `task`, and the
     worker's own edits must never pass the gate;
   - the session's agent is unknown or is `local-worker`. The one exception, never a
     refusal: a file the worker creates is reported (`phase: "worker"`) for the retry
     below;
   - anything fails (fail open).

   No file contents leave opencode: `path` is `filePath`, `command` is the bash command,
   `prompt` only for a `task` to `local-worker`, and `old` only for an `edit` with
   `replaceAll`, so the guard can count its matches. The route never logs a body.
2. **The decision**, a route on the session's loop guard (`/nav-pilot/dispatch-gate`). The
   guard is the nav-pilot launch process: it lives exactly as long as the session, already
   listens on a port cplt allows, and knows where the server is. State is in memory and gone when the session ends. There are no
   files, no sandboxed paths, no binary to find on `PATH` (bench runs a pinned binary off
   `PATH`), and no process spawned per tool call.

**Rules** (at `balanced` and `aggressive`, only in the top-level session).
State is **per turn**: the plugin numbers each user message it sees for a
non-worker agent (`chat.message`), and the guard resets a session's counters
when the number changes. Per session would let three unrelated edits over a
morning use up the budget before the big job arrives.

- *Counted files*: `edit` and `write` on a `filePath` that exists (the plugin makes a
  relative path absolute against the session's directory; a path the guard cannot stat
  is allowed), and a bash segment
  (split on `;`, `&&`, `||`, `|` and newlines) that runs `sed -i`/`perl -i` on one literal
  file. Files are keyed by name, so `src/A.kt` in a `sed` and the absolute path in an
  `edit` count once.
- *Scripted edits*: a bash command that runs `sed -i`/`perl -i` inside a `for`, `while` or
  `until` loop, or on a file that is itself computed (`"$f"`). A computed value on one
  literal file (`sed -i "s/v/$NEW/" package.json`) is one counted edit. Probe 4 did a 12-call-site job in one such call:
  `grep -rl … | while read f; do sed -i … "$f"; done`. A literal
  `sed -i 's/a/b/' f1 f2`, or `grep -rl … | xargs sed -i 's/a/b/'`, is one
  search-and-replace. The policy keeps that with the
  orchestrator, so it is not counted.
- *Call sites*: an `edit` or a counted `sed`/`perl` segment adds its call sites to the
  turn's count. An ordinary `edit` is 1. An `edit` with `replaceAll` counts the matches of
  its `oldString` in the file, and a `sed -i`/`perl -pi` with one address-free `s` command
  counts its pattern's matches: every match with `/g`, at most one per line without. The guard reads the file before the edit runs, only under the project root and
  after resolving links. A BRE is turned into RE2; a pattern RE2 cannot compile, several
  commands, an address, a computed value or a file that is not a regular file each count as 1. Probe 6's cell r6 is the reason: 60
  call sites in 3 files, done as two edits and one `sed … /g` on the test file, never
  reached 5 files or 10 calls.
- *Deny* (multi-file rule, only when `edit-multi-mechanical` is trusted): the edit that would make the
  orchestrator's **5th distinct file** this turn, the edit that brings the turn to **10
  call sites** once at least two files are involved (ten edits of one file is fixing a
  test), or any scripted edit.
- *Deny* (create-file rule, `aggressive` only, and only when `create-file` is trusted):
  a `write`, or an `edit` with an empty `oldString`, of a path that does not exist yet,
  which means a new file,
  tests included. Probe 5 is the reason. Without `create-file` trusted, a new file is
  neither denied nor counted.
- *Dispatch first*: a denied file passes once its name has appeared in a `task` prompt to
  `local-worker` this turn. A scripted edit passes once any `task` has gone to
  `local-worker` this turn. If the worker fails or refuses, the policy's own fallback
  ("take the task yourself") works, because the file has already been sent. A plain retry
  does not pass. That would turn the gate into a nudge that Sonnet 5 argues past.
- *Checkpoint* (`balanced`): the same edit again passes, and 1 refusal per turn is the
  budget.
- *Budget* (`aggressive`): at most 2 denies per turn. After that the gate stays quiet until the next
  turn. A misfire costs two tool calls at most.
- *Fail open*: the plugin gets no answer within 2 s, the local server's port does not
  accept a TCP connection (a dial with a 200 ms timeout, and no `ps`/`lsof` on this path),
  or the request is malformed. Every one of these allows the call.
- The route is matched before the guard's proxy fallthrough. It takes neither the
  completion lock nor the server lock, and it never logs a body: `command` can hold a
  token and `prompt` holds code.

**Checking the worker's result** (at `balanced` and `aggressive`, wherever the gate runs).
Probe 6 found the failures that matter in what the orchestrator accepted, not in whether
it dispatched: a grep passed a broken definition, and a new test that was green caught
nothing. So:

- On `tool.execute.after` for a `task` to `local-worker`, the plugin asks the route with
  `phase: "after"` and appends the answer to the task's result: build, run the tests that
  cover the change, and, for a new test, break the code it tests on purpose to see it fail.
  The orchestrator reads it at the moment it decides.
- The route marks the turn unverified. A bash segment that builds or runs tests (Gradle,
  Maven, `go test`, `npm test`, `tsc`, `pytest` and the like, `local.Verifies`) clears it.
- One retry for a new file (mlx-workspace #126, bench-frontier's `retry2`: on create-file
  15/20 verified against 5/20, 322 s against 618 s per verified result). The plugin reports
  the worker's own new files (`phase: "worker"`, from its session) and the worker's session
  on the task's return (`output.metadata.sessionId`). When the worker created a file, the
  first build or test after the return is the check. On its `tool.execute.after` the
  plugin sends the exit code (`output.metadata.exit`). A failure appends: send it back to
  `local-worker` once, in the same task, with the output and «The change is not done yet.
  Fix it. Change nothing else.» A second failure appends: fix it yourself. Two attempts is
  the bound. Outcomes: `create_retry`, `create_retry_passed`, `create_retry_failed`.
- On `experimental.text.complete` for the orchestrator, the plugin asks with
  `phase: "text"`. While the turn is unverified the route answers once per turn with a
  reminder, and the plugin adds it with `client.session.prompt({noReply: true})` as a
  synthetic part, with the session's agent and model. opencode's loop reads a user message
  newer than the last assistant message as more work, so the session answers it before it
  goes idle. This was checked against opencode 1.18.32 in `opencode run`, which exits on
  idle. The reminder is worded for the case where the orchestrator is about to build,
  because text before a tool call triggers it too.
- Temp directories outside the project (#1237). Re-probe 7 found the orchestrator backing
  up a file to `/tmp` before the deliberate break, and once staging drafts in
  `/tmp/navpilot_tests`. opencode asks for `external_directory` there, `opencode run`
  rejects the request, and the session ends: in one sample before the break was undone,
  which left production code broken. The verify text now says to undo the break by
  reversing the edit and never to copy outside the project, and the create-file refusal
  says to keep drafts in the project. The gate also refuses the orchestrator's own edit,
  write or shell command that names a path in `/tmp`, `/var/tmp` or `$TMPDIR`
  (`os.TempDir()`, in either form when it is behind a symlink) outside the project, at
  every budget, as outcome `deny_tmp`. A literal `~`, `$HOME` or `$TMPDIR` counts too,
  although opencode itself skips arguments with a `$`: a backup there is still outside
  the project. In home, the gate refuses the edit and write tools, a redirect, and the
  path arguments of the commands opencode checks (`cd`, `pushd`, `popd`, `rm`, `cp`,
  `mv`, `mkdir`, `touch`, `chmod`, `chown`, `cat`) and of `tee`, `ln`, `rsync` and
  `install` (#1273). Other commands may read from home, since a build reads the JDK and
  the Gradle and Maven caches there. A refusal is a tool result, so the session goes on. A path built
  at run time (`mktemp`, other variables) is not seen. In an interactive session this
  replaces opencode's prompt for those paths.
- The mutation check for new tests is policy text only. nav-pilot cannot tell whether a
  test was run against broken code.
- Everything fails open: no answer, no model recorded for the session, or an error means
  no text and no reminder.

**The deny text**:

> nav-pilot (local_dispatch): this is a mechanical change across several
> files. Send it to `local-worker` first: one task per file, naming each place that
> changes and exactly what it becomes, with a check such as a grep. Once a file has been
> sent to `local-worker`, your own edits to it pass, so you can fix or finish what it
> returns.

**Failure modes, and what covers each:**

| Failure | Covered by |
|---|---|
| False positive on a small edit needing judgement | 5-file / 10-site threshold; 2-deny budget per turn |
| A large search-and-replace in one file, then small edits in two others in the same turn | refused: the site count cannot tell it from r6 done in another order. Known and accepted; the budget bounds it to 1 refusal (balanced) or 2 (aggressive) |
| The worker runs as a background task (opencode's experimental background subagents) | its result arrives later as a message, so no checks are appended and no reminder is sent |
| Worker down | TCP dial to the server fails, so the call is allowed; a dispatch that fails still exempts the file |
| Worker wedged (port open, no answer) | the file passes once sent; the budget caps the cost |
| Orchestrator retries the denied call | the retry is denied again until the budget runs out (2 per turn), then it passes |
| Orchestrator works around the gate with other shell writes, or with `apply_patch` (the edit tool of models on `usePatch`) | not covered; visible as a low dispatch rate with few denies (telemetry) |
| Worker wedged, or another process on the port | `tcpUp` sees an open port, so the gate still denies; the budget caps the cost |
| Cloud session with no local server | no guard, so no `NAV_PILOT_DISPATCH_GATE`, and the plugin is inert |
| opencode run without nav-pilot | the variable is unset, so the plugin is inert |
| `--pure` | plugins are off, so there is no gate; nav-pilot warns at launch |
| Loop guard interaction | none: the cloud orchestrator's calls do not pass the local guard, and the gate cannot deny the same call twice |

### (a) Policy and persona text per level

`LocalDispatchPolicy(m, level, …)`. The policy text and the gate take their rules from
one function, `local.DispatchGateRules`, so the text never describes a rule the gate
does not run.

- `conservative` puts back the "describe fully, and if you doubt it can do the task, do it
  yourself" clause that #996 took out. It raises the split threshold to 10 files or 20
  call sites, and adds that the orchestrator's own tier and judgement decide.
- `balanced` is #997's text plus one paragraph. The paragraph says that nav-pilot
  enforces the split in every tier, that the persona's Trivial and Compressed tiers
  decide phase behaviour and not who writes files, and that the refusal is a checkpoint
  a retry passes.
- `aggressive` says instead that a refused file passes only once it has been sent.
  It adds the create-file sentence when `create-file` is trusted.
- `off`: no policy file, and the worker is neither bound nor materialized.

The persona (`agents/nav-pilot.agent.md`) is a synced static file shared by every
level. It already says the policy's send and keep lines apply in every tier, and
Sonnet 5 quoted the tier anyway. Stripping the tier language from the persona when
dispatch is on would mean transforming a synced agent file per level. It is deferred:
the gate does not depend on the model reading the persona correctly.

### (c) Structural: local-first

The local model runs the session and consults the cloud model for planning or
review, for example through `alpha decide` or a cloud subagent. The pieces exist:
a local session model, and `alpha decide`.

Not now. At the 48 GB tier the quality frontier trusts the worker unaided with
only 1–2 call sites. A local primary would be doing the orchestration that it
measures worst at. Revisit it when a tier's model is trusted in `local` mode
(`capabilities.classes.*.local`) for the classes that make up most sessions.
That verdict is already in the manifest, so the switch can be data-driven.

## Telemetry

Recorded at exit by the launch process, like `nav_pilot_local_dispatches`. Enums only.

- `nav_pilot_local_dispatches` gets a `dispatch_level` attribute
  (`off|conservative|balanced|aggressive`). This gives the dispatch rate per level.
- `nav_pilot_local_gate_total{outcome}` counts gate decisions, with `outcome` one of
  `deny_files`, `deny_sites`, `deny_scripted`, `deny_create`, `deny_tmp`, `dispatched_after_deny`,
  `verify_nudge`, `create_retry`, `create_retry_passed` and `create_retry_failed`. `deny_sites` is a deny from the call-site count alone. `dispatched_after_deny`
  is a `task` to `local-worker` after a deny in the same turn. `verify_nudge` is a reminder
  sent because no build or test ran after the worker returned. Recording `allow` would be one data point
  per tool call and tell us nothing.

What to read from them: `dispatched_after_deny` close to the number of denies
means the gate works. Denies that run out the budget mean the orchestrator
disagrees, so look at the pass rate. A low dispatch rate with few denies means
the orchestrator is working around the gate, for example with a Python heredoc.
The alpha cohort will give fewer than 10 sessions per level for months, so the
bench carries the evidence and fleet telemetry only confirms it.

## Measurement plan

bench-hybrid (mlx-workspace), for each level:

- Cells: `bench/targets/isoppfolgingstilfelle-large.json`, rungs 4–6 (thread-arg, 12–60
  call sites), plus probe 5's create-file cells.
- Arms: `control` (dispatch off), and `hybrid` at `conservative`, `balanced` and
  `aggressive`, passed as `--local-dispatch <level>` before `--`.
- **The bench must not pass `--pure`**, or the gate never loads. Instead, isolate each run
  with `XDG_CONFIG_HOME` pointed at a bench-only directory. opencode and nav-pilot's
  `openCodeConfigDir` both honour it. That keeps the developer's own plugins, such as
  rtk, out of every arm. bench-hybrid hardcodes `~/.config/opencode` (`OPENCODE_CFG`,
  `POLICY_FILE`) and has to follow.
- A false-positive cell: a 3–4 file change that needs a judgement per file. The feared
  failure must be measured, not assumed away.
- Probe 5's create-file cells exercise the create-file rule at `aggressive` only with a
  manifest that trusts `create-file` (`NAV_PILOT_BENCH_MANIFEST`). No shipped manifest
  does yet.
- Record per sample: dispatches (task calls to `local-worker`), gate denies, dispatches after a deny,
  pass/fail, cloud cost, wall time.
- Decision rule: keep enforcement at `balanced` if its pass rate is no lower than
  control, on the large rungs and on the false-positive cell, and its cloud cost is at
  or below control on at least two of the three large rungs, with at least 5 samples per
  cell. Otherwise drop `balanced` back to prose and keep the gate at `aggressive`. Fewer
  samples make it a probe, and the report says so.

## Tiering

Higher tiers carry stronger workers with more trusted classes. The gate keys off
the trusted classes, so it covers new ones as the manifest trusts them, without
a release, as long as nav-pilot has a rule for that class.
`edit-multi-mechanical` (at `balanced`) and `create-file` (at `aggressive`) have
one. `edit-single`, `read-qa` and `debug` get no rule, because a
single edit or a lookup cannot be told apart from the orchestrator's own reasoning
steps.

The default could rise with the model. A manifest field
`recommended_dispatch: "aggressive"` on an entry would set the default when the
user has not chosen one. It is not built yet: there is one trusted model, and a
manifest that can raise pressure on a stranger's orchestrator deserves its own
review. It is also capped by what the binary knows. An unknown value falls back
to `balanced`.

## What ships first

- The `local_dispatch` key, the `--local-dispatch` flag and the four levels.
- Per-level policy text.
- The gate at `balanced` (multi-file rule) and `aggressive` (plus the create-file rule):
  the plugin shim, the guard route, and the rules above. The plugin file is written
  whenever a worker is set up and does nothing without `NAV_PILOT_DISPATCH_GATE`. Only
  `alpha local off` removes it, so a launch can never delete it while another session
  is loading it.
- Telemetry: the level attribute and the gate counter.
- Not yet: `recommended_dispatch`, stripping the persona's tier language, and
  local-first.

## Review record

A Fable adversarial review of the first draft changed these points:

- The gate is per turn, not per session.
- It catches scripted loops (`while read f; do sed -i … "$f"`), the form probe 4
  actually used.
- It is dispatch-first rather than retry-passes.
- `aggressive` keeps balanced's sizes (5 files or 10 call sites), not 3.
- There is no ownership probe on the decision path.
- The bench is isolated with `XDG_CONFIG_HOME` rather than `--pure`, and gets a
  false-positive cell.
- The create-file rule is deferred.

Probe 5 then changed the default: see [Why the default enforces](#why-the-default-enforces).
Dispatch-first, rather than a retry that passes, remains a product choice made here
with the stronger control, and the 2-deny budget per turn bounds what it costs.

A second Fable review, of the code, changed these points:

- The plugin gates top-level sessions only.
- `chat.params` no longer overwrites the agent.
- A new file is not a mechanical edit.
- A `$` on one literal file is a counted edit, not a scripted one.
- Files are keyed by name.
- Backslash escapes in double quotes and perl's `-M`/`-I` are handled.
- `off` no longer binds the worker for a local session.
- A launch never removes the plugin.

A third pass, on the revision, changed these:

- `balanced` became a checkpoint rather than dispatch-first.
- The 10-edit rule now needs two files.
- The plugin keeps only a definite top-level answer.
- Relative paths are resolved in the plugin, and a failed stat allows the call.
- The create rule needs a real create (a `write`, or an `edit` with an empty
  `oldString`).
- A background task's synthetic message no longer starts a new turn.

Probe 6 (mlx-workspace §8.8, 2026-09-28) measured the levels. `aggressive`
dispatched on 6 of 8 valid samples on the large and create-file cells (`balanced`: 2 of 6),
with no false positive on the small cell. But 1 of the 6 dispatched samples failed, one hit
the 20-minute cap, and it cost 1.2–1.6× the control's cloud credits and took 2–3.6× its
time. `balanced` stays the default (superseded by re-probe 7, below), and `aggressive` is documented as an opt-in for
mechanical multi-file edits. The gate's two gaps were fixed here: the call-site count (r6 was never gated) and the check of the
worker's result. Both apply at `balanced` too, where the checkpoint bounds a misfire to
one refusal per turn.

The review also suggested cutting `conservative`, since on Sonnet 5 it and `balanced`
both come out near zero. It stays, because the user asked for a graded setting and
`conservative` is the level for a model that dispatches too eagerly (Sonnet 4.6 sent
23 of 24).

Re-probe 7 (see "Why new setups get aggressive") found the quality line holding at
`aggressive`: 17 of 17 dispatched and verified. New local setups got
`aggressive`; existing ones kept `balanced`. Since the same-day controls (see
"Balanced against same-day controls"), `aggressive` is the built-in default for
everyone without an explicit value.
