# Probe results, 2026-09-29

The measurements behind PR #1303 ("make `mise run all` pass reliably"). Each
section gives the command and its saved output, unedited. Machine: one
macOS laptop, 18 logical CPUs (`hw.ncpu`), CrowdStrike Falcon's endpoint
security extension active, git 2.55.0. Other work was running on it; the
load average at the start of each run is in the output.

## First exec of a new script (`first-exec.go`)

    go run hack/probes/first-exec.go    # three runs

```
# run 1 2026-09-29 13:56:39 load { 1.30 2.28 4.10 }
fresh-parallel  n=40 median=2.107s p90=3.666s max=3.957s
fresh-serial    n=40 median=130ms p90=133ms max=135ms
warm-parallel   n=40 median=115ms p90=134ms max=139ms
writeexec       n=40 median=120ms p90=139ms max=144ms
# run 2 2026-09-29 13:56:59 load { 1.35 2.23 4.03 }
fresh-parallel  n=40 median=2.097s p90=3.677s max=3.97s
fresh-serial    n=40 median=131ms p90=134ms max=135ms
warm-parallel   n=40 median=114ms p90=135ms max=139ms
writeexec       n=40 median=113ms p90=135ms max=138ms
# run 3 2026-09-29 13:57:20 load { 1.25 2.15 3.96 }
fresh-parallel  n=40 median=2.139s p90=3.787s max=4.094s
fresh-serial    n=40 median=126ms p90=194ms max=457ms
warm-parallel   n=40 median=87ms p90=109ms max=114ms
writeexec       n=40 median=118ms p90=138ms max=143ms
```

40 new scripts started at once wait about 2.1s (median) and up to 4.1s.
Started one after another, or after one earlier run of each file, they take
0.09-0.13s. `writeexec`, the way `testhome.WriteExec` writes them, is as
fast as a file that has run before.

## Probe timings in the nav-pilot suite (`probe-timing.patch`, `probe-timing.log`)

`probe-timing.patch` is temporary instrumentation, not part of the change:
it logs every `runStagedProbe` call as `binary budget duration error`.
`probe-timing.log` is its output from one
`NP_PROBE_LOG=... go test -race -skip TestLaunchBudget -count=1 ./...` run
before the fix (five tests failed in that run). Of the 16 probes with a 2s
budget, 6 were killed at 2.00s; the 10 that finished took 0.24-1.76s.

## `echo | grep -q` under pipefail (`pipefail-grep.sh`)

    bash hack/probes/pipefail-grep.sh                  # runs 1-3
    bash hack/probes/pipefail-grep.sh 5000 18          # run 4
    REPEAT=n bash hack/probes/pipefail-grep.sh 1000 6

```
# run 1 2026-09-29 13:57:58 load { 2.39 2.40 3.98 }
pipe: 0 false negatives in 12000 runs
herestring: 0 false negatives in 12000 runs
# run 2 2026-09-29 13:58:37 load { 14.79 5.53 5.05 }
pipe: 0 false negatives in 12000 runs
herestring: 0 false negatives in 12000 runs
# run 3 2026-09-29 13:59:27 load { 29.70 10.91 7.07 }
pipe: 0 false negatives in 12000 runs
herestring: 0 false negatives in 12000 runs
# run 4 2026-09-29 14:00:20 load { 27.80 13.82 8.41 } args 5000 18
pipe: 0 false negatives in 90000 runs
herestring: 0 false negatives in 90000 runs
# run REPEAT=1 2026-09-29 14:06:54 load { 22.04 27.30 17.93 } args 1000 6
body: 26243 bytes
pipe: 0 false negatives in 6000 runs
herestring: 0 false negatives in 6000 runs
# run REPEAT=8 2026-09-29 14:07:31 load { 31.29 29.03 18.94 } args 1000 6
body: 69983 bytes
pipe: 5956 false negatives in 6000 runs
herestring: 0 false negatives in 6000 runs
# NOTE: the REPEAT=1 run above hit a script bug (seq 2 1 counts down on macOS), so its body was 3x (26243 bytes); fixed, rerun below
# run REPEAT=1 2026-09-29 14:08:26 load { 34.40 30.70 20.21 } args 1000 6
body: 8747 bytes
pipe: 0 false negatives in 6000 runs
herestring: 0 false negatives in 6000 runs
```

With the real 8747-byte body the pipe gave no false negatives in 132000
runs, so the one `skills:lint` failure seen in `mise run all` (security-review
and web-design-reviewer, same run) is not reproduced here. With a body
larger than the pipe buffer the same check fails 5956 times in 6000: grep
exits at the match, the writer gets SIGPIPE, pipefail returns 141. The
here-string has no writer and gave 0 in every run.

## git maintenance and TempDir cleanup (`git-cleanup.go`)

    go run hack/probes/git-cleanup.go

```
git version 2.55.0
# run 1 2026-09-29 14:09:03 load { 27.99 29.67 20.27 }
failed deletions: 0 of 300
# run 2 2026-09-29 14:09:17 load { 23.65 28.62 20.06 }
failed deletions: 0 of 300
# run 3, with 24 'yes' processes running, 2026-09-29 14:09:32 load { 21.15 27.84 19.93 }
failed deletions: 0 of 300
# load at end { 42.51 32.95 22.09 }
```

Not reproduced: no failed deletion in 900.

## Interrupting hack/parallel.sh (`parallel-interrupt.py`)

    python3 hack/probes/parallel-interrupt.py <parallel.sh before a2993ff5>
    python3 hack/probes/parallel-interrupt.py

```
# 2026-09-29 14:10:35 parallel.sh before a2993ff5
SIGINT: sections running before 2, exit -2 after 0.0s, still running 2
SIGTERM: sections running before 2, exit -15 after 0.0s, still running 2
# parallel.sh at a2993ff5
SIGINT: sections running before 2, exit 130 after 0.0s, still running 0
SIGTERM: sections running before 2, exit 143 after 0.0s, still running 0
```

## `mise run all` wall times

Load is the 1-minute average, sampled every 5s during each set.

| tree | result | wall time | load min / median / max |
|---|---|---|---|
| origin/main a96ca3b8 | fail (`sync_launch_no_terminal`) | 158s | 13.4 / 19.1 / 24.3 |
| branch at 1d2ae66e (before the lint fix) | pass, fail (skills lint), fail (testhome guard) | 82s, 112s, 104s | 8.8 / 19.5 / 30.8 |
| branch at 7cd8a917 (with the lint fix) | pass, pass, pass | 94s, 93s, 94s | 11.4 / 17.1 / 23.1 |
| 0d9c60a3, rebased on main | pass | 107s | 24.9 at start, 26.6 at end |
| a2993ff5 (interrupt fix) | pass | 87s | 1.8 at start, 8.5 at end |

## Load stress for #1335

Same machine, 2026-09-29. `load-stress.sh TREE OUT 9` runs `mise run check` in a loop in
TREE and nine copies of each test binary at once, each `-count=20`, so 180 runs
per test. Load is the 1-minute average, sampled every 30s. The origin/main
tree is 4329bb8e, the branch is this PR.

| tree | run | load min / median / max | `TestResolveForLaunchWithinMaxAge` | `fetch_typeahead_stderr_redirected` | the three cplt tests |
|---|---|---|---|---|---|
| origin/main | 1 | 21.9 / 25.9 / 28.4 | 62 pass, 118 fail | 163 pass, 17 fail | not in the probe yet |
| branch, first two fixes | 1 | 13.8 / 25.6 / 36.2 | 177 pass, 3 fail | 178 pass, 2 fail | not in the probe yet |
| origin/main | 2 | 6.2 / 11.8 / 19.9 | 179 pass, 1 fail | 180 pass, 0 fail | 125/180, 98/180, 99/180 pass |
| branch | 2 | 17.5 / 26.7 / 46.9 | 180 pass, 0 fail | 180 pass, 0 fail | 180/180 each |

The cplt tests are `TestCpltBuiltinDomainsComeFromCplt`,
`TestDoctorReportsCpltVersion` and `TestUpgradeChecksCpltWhenNavPilotIsCurrent`.

On origin/main every `TestResolveForLaunchWithinMaxAge` failure is the one
from the issue, `cache dated in the future: ... no newer one came in 600ms`,
and every script failure is `pty-run: timed out`, exit 124. The cplt tests
fail on the 2s `cpltCommandTimeout`: the fake cplt did not answer in time, so
doctor and upgrade reported what they report without cplt.

The five branch failures in run 1 all fell in the first one or two of the 20
iterations, while nine e2e binaries built nav-pilot at once: the script, about
4s alone, took 105-128s there. Two were the script at pty-run's 60s, and three
were `TestResolveForLaunchWithinMaxAge` with a local `git fetch` over the
product's own 30s budget (`firstFetchTimeout`). From the third iteration on
there were none. Run 2 had none at a higher peak load.

### Without load

Two controls that do not depend on load. A `git` on PATH that sleeps 0.8s
before `fetch`:

```
origin/main: --- FAIL: TestResolveForLaunchWithinMaxAge
  cache_test.go:243: cache dated in the future: <nil>, the copy of navikt/x is from 2026-09-30 17:50, and no newer one came in 600ms; want 4bf093c...
branch:      PASS
```

A git wrapper in the script that sleeps 1s after the checkout, before
nav-pilot flushes the typed-ahead keys:

```
origin/main script: pty-run: timed out, exit code 124, want 0 (22.8s)
branch script:      PASS (9.3s)
```

With the flush removed from `quietStdin` (`ioctlSetTermios` in place of
`ioctlSetTermiosFlushIn`), the branch script still fails: `pty-run` times out.

# Probe results, 2026-10-08

Why five journeys (launch_without_cplt, launch_explicit_args,
sync_launch_no_terminal, autonomy_leave_strict, hook_slow_python_fails_open),
the opencode_* journeys and `TestIsCplt` failed in a full
`go test -race -skip TestLaunchBudget ./...` and passed alone. Same machine as
above; load average 12-28, from this work and other sessions.

## The failures

Every one is a deadline of 2-3 s on a local process that was healthy:

| Failure | Deadline that expired | What the journey saw |
|---|---|---|
| launch_without_cplt, launch_explicit_args, sync_launch_no_terminal, `TestIsCplt` | `IsCplt`: `copilot --version`, 2 s | the probe was killed before the fake logged its argv (no `copilot.log`), or it was asked again because a killed probe is not cached |
| autonomy_leave_strict (exit 124) | `cplt config get sandbox.preset`, 2 s | the preset read as unknown, the cursor started on the wrong answer, strict was chosen and the prompt pty-run waited for never came |
| opencode_policy, opencode_mcp_registry | `opencode --version`, 5 s | an unreadable version counts as opencode 2, so the launch stopped with "cplt too old" |
| hook_slow_python_fails_open | Copilot's (here run.py's) 3 s | the hook shell ran 2 more processes (`cat`, `rm`) after the kill at 1 s; it exited after the deadline |

The runtime gate already gives the same binaries 8 s (`cplt --version`) and
30 s (the client), for the reason written next to those constants.

## Process start-to-exit inside a full test run

A temporary test in `internal/testhome`, run beside two suite loops, timed
`exec.Command(...).Run()` every 200 ms for 150 s:

```
sh -c exit               n=172 p50=26ms  p90=155ms p99=3.28s max=5.11s >2s=2
exec warmed script       n=172 p50=23ms  p90=120ms p99=2.06s max=4.01s >2s=2
WriteExec(write+warm)    n=172 p50=133ms p90=495ms p99=8.50s max=8.58s >2s=7
cat fresh file           n=172 p50=8ms   p90=22ms  p99=64ms  max=70ms  >2s=0
```

Split into `Start()` and `Wait()`: `Start()` stayed under 4 ms; the time is
spent after the fork, before the child gets to exit. Reading a new file
(`cat`) is never slow, so it is starting a process, not file I/O.

`spawn-under-cpu.go` tries to reproduce that outside the suite, and does not:

```
# load { 19.97 24.39 22.09 }
as-is        n=256 p50=17ms p90=22ms max=42ms   over-2s=0
cpu-busy     n=215 p50=13ms p90=34ms max=1.374s over-2s=0
new-scripts  n=204 p50=33ms p90=86ms max=395ms  over-2s=0
```

So busy cores alone, or new scripts alone, are not enough; what in a full
test run stalls a process start for seconds is not pinned down. What is
measured is that it happens, and that a 2 s deadline on a healthy local
process does not survive it.

## Before and after

Before: two loops of the suite at once on origin/main, 6 runs each. 10 of 12
runs failed: hook_slow_python_fails_open 4, autonomy_leave_strict 6,
sync_launch_no_terminal 2, `TestIsCplt` 3, plus sync_removed_items_go 2 and
local_dispatch_sites 1 (not investigated here).

Paired: each round runs origin/main and this change at the same time, with
12 extra `yes` processes (`ab.sh 8 12`, load 13-28), 8 rounds:

```
main  2 of 8 rounds failed (launch_explicit_args, hook_slow_python_fails_open)
fix   0 of 8 rounds failed
```
