#!/usr/bin/env python3
"""Run a benchmark matrix: several arms of nav-pilot-golden.sh --suite.

A matrix file has one arm per line, `suite model effort n`, `#` for comments:

    planning  gpt-6-sol    low      5
    review    claude-opus-5.5  high  10
    coding    gpt-6-luna   default  5     # default: no --effort flag

Each arm is saved as docs/golden-baselines/<matrix name>/<suite>-<model>-<effort>.txt
(plus the -results/-attempts/-usage PSVs). An arm whose files exist is skipped,
so after a crash the same command resumes. An arm that crashed mid-run saved
nothing and runs again from the start.

  scripts/benchmark-matrix.py MATRIX --dry-run   # arms, status, estimated credits
  scripts/benchmark-matrix.py MATRIX --jobs 3    # run the pending arms, 3 at a time
  scripts/benchmark-matrix.py MATRIX --keep      # and keep transcripts to read a failure

The estimate multiplies n by the per-run credits of the closest committed run
in summary.json: same suite, model and effort, else same suite and model, else
the suite median across all models. It is an extrapolation, and says which.
"""

import argparse
import json
import os
import re
import statistics
import subprocess
import sys
import tempfile
from collections import Counter
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
# Overridable so tests never write into the checkout.
BASELINES = Path(os.environ.get("BENCHMARK_BASELINES", REPO / "docs" / "golden-baselines"))


def arms(matrix):
    out = []
    for line in matrix.read_text().splitlines():
        fields = line.split("#", 1)[0].split()
        if not fields:
            continue
        if len(fields) != 4 or not fields[3].isdigit():
            sys.exit(f"{matrix}: want `suite model effort n`, got: {line!r}")
        arm = dict(zip(("suite", "model", "effort", "n"), fields[:3] + [int(fields[3])]))
        if any(a["suite"] == arm["suite"] and a["model"] == arm["model"] and a["effort"] == arm["effort"] for a in out):
            sys.exit(f"{matrix}: {arm['suite']} {arm['model']} {arm['effort']} is listed twice; both would save to one file")
        out.append(arm)
    return out


def baseline(outdir, arm):
    return outdir / f"{arm['suite']}-{arm['model']}-{arm['effort']}.txt"


def done(path, n=None):
    """Saved and whole: every prompt has an attempt row in every run."""
    attempts = path.with_name(path.stem + "-attempts.psv")
    if not (path.exists() and path.with_name(path.stem + "-results.psv").exists() and attempts.exists()):
        return False
    rows = [l.split("|") for l in attempts.read_text().splitlines() if l and not l.startswith("#")]
    runs = n or int(re.search(r"^# repeats:\s*(\d+)", path.read_text(), re.M).group(1))
    per_prompt = Counter(r[0] for r in rows)
    return bool(per_prompt) and all(c == runs for c in per_prompt.values())


def per_run_credits(runs, arm):
    def med(rs):
        vals = [r["credits"]["median"] for r in rs if r.get("credits")]
        return statistics.median(vals) if vals else None

    same_suite = [r for r in runs if r["suite"] == arm["suite"]]
    same_model = [r for r in same_suite if r["model"] == arm["model"]]
    exact = [r for r in same_model if r["effort"] == arm["effort"]]
    for rs, how in ((exact, "measured"), (same_model, "other effort"), (same_suite, "other model")):
        if med(rs) is not None:
            return med(rs), how
    return None, "no data"


def run_arm(arm, outdir, logdir, keep):
    cmd = [str(REPO / "scripts" / "nav-pilot-golden.sh"), "--suite", arm["suite"],
           "--model", arm["model"], "--repeat", str(arm["n"]),
           "--save-baseline", str(baseline(outdir, arm))] + (["--keep"] if keep else [])
    if arm["effort"] != "default":
        cmd += ["--effort", arm["effort"]]
    log = logdir / f"{baseline(outdir, arm).stem}.log"
    with log.open("w") as f:
        rc = subprocess.run(cmd, cwd=REPO, stdout=f, stderr=subprocess.STDOUT).returncode
    # 0 green, 1 an assertion failed, 3 not evaluated: all three are results.
    ok = rc in (0, 1, 3) and done(baseline(outdir, arm), arm["n"])
    print(f"{'done' if ok else 'FAILED'} {baseline(outdir, arm).stem} (exit {rc}, log {log})", flush=True)
    return ok


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("matrix", type=Path)
    ap.add_argument("--jobs", type=int, default=2)
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--keep", action="store_true", help="keep transcripts (path in each arm's log)")
    a = ap.parse_args()

    outdir = BASELINES / a.matrix.stem
    todo = arms(a.matrix)
    summary = BASELINES / "summary.json"
    runs = json.loads(summary.read_text())["runs"] if summary.exists() else []

    total = 0.0
    for arm in todo:
        each, how = per_run_credits(runs, arm)
        cost = each * arm["n"] if each is not None else None
        total += cost or 0
        status = "done" if done(baseline(outdir, arm), arm["n"]) else "pending"
        est = f"~{cost:.0f} credits ({arm['n']} × {each:.1f}, {how})" if cost is not None else "no estimate"
        print(f"{status:8} {arm['suite']:9} {arm['model']:18} {arm['effort']:8} n={arm['n']:<3} {est}")
    print(f"estimated total ~{total:.0f} credits, pending and done arms alike; an extrapolation, not a quote")
    if a.dry_run:
        return

    outdir.mkdir(parents=True, exist_ok=True)
    logdir = Path(tempfile.mkdtemp(prefix="benchmark-matrix."))
    pending = [arm for arm in todo if not done(baseline(outdir, arm), arm["n"])]
    with ThreadPoolExecutor(max_workers=a.jobs) as pool:
        results = list(pool.map(lambda arm: run_arm(arm, outdir, logdir, a.keep), pending))
    print(f"{sum(results)}/{len(pending)} arms saved to {outdir.relative_to(REPO)}; "
          "then run scripts/benchmark-summary.py and commit")
    sys.exit(0 if all(results) else 1)


if __name__ == "__main__":
    main()
