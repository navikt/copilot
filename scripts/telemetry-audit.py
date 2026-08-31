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


def query(expr, timeout=30):
    url = f"{MIMIR}/query?" + urllib.parse.urlencode({"query": expr})
    req = urllib.request.Request(url, headers={"X-Scope-OrgID": ORG})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return json.load(r)["data"]["result"]
    except urllib.error.URLError as e:
        raise SystemExit(
            f"✗ cannot reach Mimir ({e.reason}).\n"
            "  It is only routable from the Nav network — connect to the VPN and try again."
        )


def metric_names(prefix):
    url = f"{MIMIR}/label/__name__/values"
    req = urllib.request.Request(url, headers={"X-Scope-OrgID": ORG})
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            return sorted(n for n in json.load(r)["data"] if n.startswith(prefix))
    except urllib.error.URLError as e:
        raise SystemExit(f"✗ cannot reach Mimir ({e.reason}). Connect to the Nav VPN and try again.")


def total(series):
    return sum(float(s["value"][1]) for s in series)


def main():
    ap = argparse.ArgumentParser(prog="telemetry-audit")
    ap.add_argument("--window", default="24h", help="lookback, e.g. 24h or 7d (default 24h)")
    ap.add_argument("--prefix", default="nav_pilot_", help="metric prefix to audit")
    args = ap.parse_args()
    w = args.window

    names = metric_names(args.prefix)
    if not names:
        raise SystemExit(f"no metrics matching {args.prefix}")

    print(f"{len(names)} metrics matching {args.prefix}, over the last {w}\n")
    print("  exports = how many times a process reported this metric (count_over_time).")
    print("  total   = sum_over_time, which is the real count only for counters:")
    print("            each CLI process starts at zero, exports once and exits, so a")
    print("            series is per-process totals over time, not a running total.")
    print("            For a gauge the same sum means nothing; marked (gauge?).\n")
    hdr = f"{'metric':46} {'series':>7} {'exports':>9} {'total':>13} {'devices':>8}"
    print(hdr)
    print("-" * len(hdr))

    GAUGES = ("_info", "_present", "_available", "_up_to_date", "installed_items")
    silent, unattributed = [], []
    for name in names:
        # sum_over_time, not increase(): a CLI process exports once and exits,
        # leaving series increase() cannot compute a delta from, which it
        # reports as 0.0. Measured against live data, not assumed.
        series = query(f"sum by (device_id) (sum_over_time({name}[{w}]))")
        exports = query(f"sum(count_over_time({name}[{w}]))")
        n_exports = total(exports)
        if not series:
            silent.append(name)
            print(f"{name:46} {0:>7} {'-':>9} {'-':>13} {'-':>8}")
            continue
        with_dev = [s for s in series if s["metric"].get("device_id")]
        if len(with_dev) != len(series):
            unattributed.append(name)
        gauge = any(g in name for g in GAUGES)
        shown = "(gauge?)" if gauge else f"{total(series):.0f}"
        print(f"{name:46} {len(series):>7} {n_exports:>9.0f} {shown:>13} {len(with_dev):>8}")

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
            if not occupied:
                print(f"  {h}: {observations:.0f} observations, all above the highest bucket — the tail is lost")
                continue
            lo, hi = occupied[0], occupied[-1]
            ceiling = finite[-1][0]
            note = ""
            if len(occupied) == 1:
                note = "  ← one bucket holds everything; no quantile can be read from this"
            elif hi < ceiling / 10:
                note = f"  ← nothing above le={hi:g} but buckets run to {ceiling:g}"
            print(f"  {h}: {observations:.0f} observations, occupied le={lo:g}..{hi:g}{note}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
