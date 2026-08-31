#!/usr/bin/env python3
"""Audit what nav-pilot telemetry actually looks like in Mimir.

A dashboard is only as good as the series behind it, and the failures that
matter are quiet ones: a metric nothing emits any more, a metric whose points
carry no device_id so every "how many people" panel undercounts, a histogram
whose buckets do not span its real values. None of those raise an error. They
render a panel that looks fine and is wrong.

This asks the questions a panel cannot ask about itself:

  which nav_pilot metrics have data at all
  which carry device_id and version, and how many series are missing them
  how many distinct devices each metric sees
  for histograms, whether the observed values fit inside the buckets

Mimir is reachable from the Nav network only. Off VPN this fails as a bare
connection error rather than an auth error, which reads like the service being
down; the script says so instead.

  scripts/telemetry-audit.py                 # last 24 hours
  scripts/telemetry-audit.py --window 7d
  scripts/telemetry-audit.py --prefix nav_pilot_local
"""
import argparse
import json
import sys
import urllib.error
import urllib.parse
import urllib.request

MIMIR = "https://mimir.nav.cloud.nais.io/prometheus/api/v1"
ORG = "nais"


def _get(url, timeout=30):
    req = urllib.request.Request(url, headers={"X-Scope-OrgID": ORG})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            body = json.load(r)
    # HTTPError subclasses URLError, so it has to be caught first or an auth
    # failure is reported as "connect to the VPN" — the exact misleading
    # message this function exists to avoid.
    except urllib.error.HTTPError as e:
        raise SystemExit(f"✗ Mimir answered {e.code} {e.reason}. The request reached it; this is not the VPN.")
    except urllib.error.URLError as e:
        raise SystemExit(
            f"✗ cannot reach Mimir ({e.reason}).\n"
            "  It is only routable from the Nav network — connect to the VPN and try again."
        )
    if body.get("status") != "success":
        raise SystemExit(f"✗ Mimir rejected the query: {body.get('error', body)}")
    return body["data"]


def query(expr, timeout=30):
    url = f"{MIMIR}/query?" + urllib.parse.urlencode({"query": expr})
    return _get(url, timeout)["result"]


def metric_names(prefix):
    return sorted(n for n in _get(f"{MIMIR}/label/__name__/values") if n.startswith(prefix))


def total(series):
    return sum(float(s["value"][1]) for s in series)


def bucket_notes(rows, observations):
    """Read a cumulative bucket list and say what is wrong with its fit.

    Split out from the printing so it can be checked without a network: the
    partial-overflow case was wrong for a whole review cycle precisely because
    nothing exercised it.
    """
    finite = [r for r in rows if r[0] != float("inf")]
    counts, prev = [], 0.0
    for le, cum in finite:
        counts.append((le, cum - prev))
        prev = cum
    occupied = [le for le, n in counts if n > 0]
    overflow = observations - (finite[-1][1] if finite else 0)
    notes = []
    if overflow > 0:
        notes.append(f"{overflow:.0f} above the top bucket")
    if len(occupied) == 1:
        notes.append("one bucket holds everything")
    elif occupied and finite and occupied[-1] < finite[-1][0] / 10:
        notes.append("buckets run far past the data")
    return notes


def self_check():
    inf = float("inf")
    # 90% inside, 10% past the top boundary: the case that used to print clean.
    rows = [(5.0, 90.0), (10.0, 90.0), (inf, 100.0)]
    assert bucket_notes(rows, 100.0)[0].startswith("10 above"), bucket_notes(rows, 100.0)
    # A clean histogram earns no note at all.
    rows = [(5.0, 30.0), (10.0, 70.0), (25.0, 100.0), (inf, 100.0)]
    assert bucket_notes(rows, 100.0) == [], bucket_notes(rows, 100.0)
    # Everything in one bucket cannot answer a quantile.
    rows = [(5.0, 100.0), (10.0, 100.0), (inf, 100.0)]
    assert "one bucket holds everything" in bucket_notes(rows, 100.0)
    print("✓ self-check passed")
    return 0


def main():
    ap = argparse.ArgumentParser(prog="telemetry-audit")
    ap.add_argument("--window", default="24h", help="lookback, e.g. 24h or 7d (default 24h)")
    ap.add_argument("--prefix", default="nav_pilot_", help="metric prefix to audit")
    ap.add_argument("--self-check", action="store_true", help="check the bucket arithmetic, no network")
    args = ap.parse_args()
    if args.self_check:
        return self_check()
    w = args.window

    names = metric_names(args.prefix)
    if not names:
        raise SystemExit(f"no metrics matching {args.prefix}")

    print(f"{len(names)} metrics matching {args.prefix}, over the last {w}\n")
    print("  exports  how many samples reached Mimir (count_over_time).")
    print("  total    sum_over_time. A real count ONLY for an instrument recorded")
    print("           once per process, near exit: each sample is then one run's own")
    print("           value. An instrument set at startup is re-exported every 10s")
    print("           for the life of the process, and summing those snapshots means")
    print("           nothing. max/series tells you which you are looking at: a small")
    print("           max with far more exports than a person could plausibly have")
    print("           produced is re-export, not volume.")
    print("  max      the largest value any single series reached (max_over_time).\n")
    hdr = f"{'metric':46} {'series':>7} {'exports':>9} {'total':>13} {'max':>7} {'devices':>8}"
    print(hdr)
    print("-" * len(hdr))

    silent, unattributed = [], []
    for name in names:
        # sum_over_time, not increase(): a process exports its counter and exits,
        # so increase() has no second sample to difference and returns 0.0 on
        # every one of these series. Measured against live data, not assumed.
        series = query(f"sum by (device_id) (sum_over_time({name}[{w}]))")
        exports = total(query(f"sum(count_over_time({name}[{w}]))"))
        peak = query(f"max(max_over_time({name}[{w}]))")
        if not series:
            silent.append(name)
            print(f"{name:46} {0:>7} {'-':>9} {'-':>13} {'-':>7} {'-':>8}")
            continue
        with_dev = [s for s in series if s["metric"].get("device_id")]
        if len(with_dev) != len(series):
            unattributed.append(name)
        peak_s = f"{total(peak):.0f}" if peak else "-"
        print(f"{name:46} {len(series):>7} {exports:>9.0f} {total(series):>13.0f} {peak_s:>7} {len(with_dev):>8}")

    if silent:
        print(f"\nNo data in this window ({len(silent)}):")
        for n in silent:
            print(f"  {n}")
        print("  Either nobody exercised the path, or nothing emits it any more.")

    if unattributed:
        print(f"\nNo device_id on any series ({len(unattributed)} metrics):")
        for n in unattributed:
            print(f"  {n}")
        print("  These collapse to one series for the whole fleet. Nothing over them")
        print("  can answer how many people, which machines, or who is affected.")

    # The denominator every adoption panel needs, from a metric that does carry
    # device_id, so 'how many devices' has an answer even while the counters
    # above do not.
    for probe in ("nav_pilot_info", "nav_pilot_config_info"):
        if probe in names:
            devices = query(f"count(count by (device_id) (sum_over_time({probe}[{w}])))")
            if devices:
                print(f"\nDistinct devices seen by {probe}: {total(devices):.0f}")
                break

    # Histograms: do the buckets span the values people actually produce? A
    # histogram whose observations all land in the first bucket cannot answer a
    # quantile question, and one that overflows le=+Inf has lost its tail.
    hists = sorted({n[: -len("_bucket")] for n in names if n.endswith("_bucket")})
    if hists:
        print("\nHistogram bucket fit:")
        for h in hists:
            buckets = query(f"sum by (le) (sum_over_time({h}_bucket[{w}]))")
            rows = sorted(((float(b["metric"]["le"]), float(b["value"][1])) for b in buckets),
                          key=lambda x: x[0])
            if not rows or not rows[-1][1]:
                print(f"  {h}: no observations")
                continue
            observations = rows[-1][1]
            finite = [r for r in rows if r[0] != float("inf")]
            # Per-bucket counts from the cumulative ones, so "where do values
            # actually land" is a question about buckets rather than about
            # everything below a boundary.
            counts, prev = [], 0.0
            for le, cum in finite:
                counts.append((le, cum - prev))
                prev = cum
            occupied = [le for le, n in counts if n > 0]
            ceiling = finite[-1][0]
            # Observations past the highest finite boundary. Reporting this only
            # when *every* value overflows misses the case the check exists for:
            # a histogram that fits 90% of its values and loses the other 10%
            # off the top, which is where the tail a quantile needs lives.
            overflow = observations - finite[-1][1]
            if not occupied:
                print(f"  {h}: {observations:.0f} observations, all above le={ceiling:g} — the whole tail is lost")
                continue
            lo, hi = occupied[0], occupied[-1]
            notes = []
            if overflow > 0:
                notes.append(f"{overflow:.0f} ({overflow / observations:.0%}) above le={ceiling:g}, so the tail is lost")
            if len(occupied) == 1:
                notes.append("one bucket holds everything; no quantile can be read from this")
            elif hi < ceiling / 10:
                notes.append(f"nothing above le={hi:g} but buckets run to {ceiling:g}")
            note = ("  ← " + "; ".join(notes)) if notes else ""
            print(f"  {h}: {observations:.0f} observations, occupied le={lo:g}..{hi:g}{note}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
