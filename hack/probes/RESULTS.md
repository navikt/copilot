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
