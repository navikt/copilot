#!/usr/bin/env python3
"""Generate dashboards/nais/nav-pilot-local.json.

Written as a generator rather than hand-edited JSON because the interesting
part of this dashboard is nine PromQL expressions, and everything around them
is nine hundred lines of Grafana boilerplate that hides them. Here the queries
sit together where they can be read against each other, and the house style is
lifted from `nais/nav-pilot-cli.json` at build time so the two cannot drift.

Two rules are baked in, both bought with a wrong PR:

  sum_over_time, never increase(). The series carry no per-process label, so
  every run on a device writes into the same series from its own 1, and
  increase() reads that as flat: 0.0. Verified against live data: increase()
  totalled 0.0 across five series where sum_over_time totalled 58.

  sum_over_time is a count because every sample is what its process gained
  since its last export that went through (newOnlyExporter, #1246), so the
  same for histogram_quantile over sum_over_time of _bucket. Before #1246 a
  process re-exported everything every 10 s, and only instruments recorded at
  exit, like these, were counted once.

  scripts/build-local-dashboard.py            # write the dashboard
  scripts/build-local-dashboard.py --check    # fail if it is out of date
"""
import argparse
import copy
import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
HOUSE = ROOT / "dashboards" / "nais" / "nav-pilot-cli.json"
OUT = ROOT / "dashboards" / "nais" / "nav-pilot-local.json"

API_VERSION = "dashboard.grafana.app/v2"  # Grafana 13 v2 schema

SEL = '{version=~"$version"}'
SEL_LE0 = '{version=~"$version", le="0"}'


def panels():
    """Every panel, as (id, title, viz, description, [(expr, legend)])."""
    return [
        (1, "Machines running locally", "stat",
         'How many ran at least one local session in the range. The denominator for everything else here.',
         [(f"count(count by (device_id) (sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__range])))", "")]),
        (2, "Sessions", "stat",
         'One session is one client run with a local worker available, counted when the client exits.',
         [(f"sum(sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__range]))", "")]),
        (3, "Tasks sent locally", "stat",
         'Total tasks the main agent actually sent to the local model.',
         [(f"sum(sum_over_time(nav_pilot_local_dispatches_sum{SEL}[$__range]))", "")]),
        (4, "Share of sessions without dispatch", "stat",
         'The panel that decides whether the alpha widens. See the breakdown below for why the number is what it is: a zero does not mean the same thing in the two cases.',
         [(f"100 * sum(sum_over_time(nav_pilot_local_dispatches_bucket{SEL_LE0}[$__range]))"
           f" / clamp_min(sum(sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__range])), 1)", "")]),

        (10, "Sessions without dispatch, by whether the client saw the worker", "timeseries",
         "The one graph worth building the dashboard for. saw_traffic=true means the client reached the worker and chose not to use it: a verdict on the model. saw_traffic=false means the connection never got through, which is our fault, not the model's. Without this split a zero is three different things reported as one.\n\nWAITING FOR DATA: saw_traffic was added after the alpha shipped, so the panel is empty until people upgrade. Empty is correct here: a zero we cannot interpret is worse than no line.",
         [(f'sum by (saw_traffic) (sum_over_time(nav_pilot_local_dispatches_bucket{SEL_LE0}[$__interval]))',
           "saw worker: {{saw_traffic}}")]),
        # Explicit differences, not the raw le series. A bar per cumulative
        # bucket reads as "31 sessions sent 0, 55 sent 5" when it actually means
        # "≤0" and "≤5", and every viewer makes that mistake once.
        (11, "Tasks per session", "barchart",
         'Distribution, never mean: most sessions send nothing and a few send a lot, and an average of the two describes no real session. The buckets are subtracted from each other, so each bar is sessions in exactly that interval.',
         [(f'sum(sum_over_time(nav_pilot_local_dispatches_bucket{SEL_LE0}[$__range]))', "0 tasks"),
          (f'sum(sum_over_time(nav_pilot_local_dispatches_bucket{{version=~"$version", le="5"}}[$__range]))'
           f' - sum(sum_over_time(nav_pilot_local_dispatches_bucket{SEL_LE0}[$__range]))', "1–5"),
          (f'sum(sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__range]))'
           f' - sum(sum_over_time(nav_pilot_local_dispatches_bucket{{version=~"$version", le="5"}}[$__range]))', "6+")]),
        (12, "Sessions per interval, by client", "timeseries",
         'opencode runs the model as a subagent under a cloud agent; Copilot CLI runs the whole session locally.',
         [(f"sum by (client) (sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__interval]))", "{{client}}")]),
        (13, "Dispatch guard, by outcome", "timeseries",
         'What the guard in front of the local model did in sessions with local_dispatch balanced or aggressive. deny_* are things it stopped; dispatched_after_deny still ended in a dispatch; verify_nudge is the reminder when the local model is done and no build or test has run since; create_retry* is follow-up after it created a new file.',
         [(f"sum by (outcome) (sum_over_time(nav_pilot_local_gate_total{SEL}[$__interval]))", "{{outcome}}")]),

        (20, "Time to ready", "timeseries",
         'p50 and p95 for starts that actually came up. The filter outcome="ready" is not decoration: a start that times out contributes ten minutes, and a p95 without the filter measures the failures, not the wait.\n\nWAITING FOR DATA: outcome arrived after the previous release, so the panel fills once people upgrade.',
         [(f'histogram_quantile(0.5, sum by (le) (sum_over_time(nav_pilot_local_ready_seconds_bucket{{version=~"$version", outcome="ready"}}[$__range])))', "p50"),
          (f'histogram_quantile(0.95, sum by (le) (sum_over_time(nav_pilot_local_ready_seconds_bucket{{version=~"$version", outcome="ready"}}[$__range])))', "p95")]),
        (21, "How starts ended", "timeseries",
         'ready, failed or interrupted. Until this existed only successful starts were recorded, so the slow tail was missing by construction.\n\nWAITING FOR DATA: outcome arrived after the previous release and fills as people upgrade. From the same release autostart is counted too, which is the usual way in; before, the histogram only measured starts someone typed by hand.',
         [(f'sum by (outcome) (sum_over_time(nav_pilot_local_ready_seconds_count{SEL}[$__interval]))', "{{outcome}}")]),
        (22, "Models in use", "piechart",
         'Should be one. More means someone runs something we have not measured.',
         [(f"sum by (model) (sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__range]))", "{{model}}")]),

        (30, "decide calls by outcome", "timeseries",
         'decided and below_threshold are answers; no_server means no local server was running, and timeout that it was busy or slow. One run of `alpha decide` is one point. --eval does not count here, only as the command `alpha decide eval`.',
         [(f"sum by (result) (sum_over_time(nav_pilot_decide_result_total{SEL}[$__interval]))", "{{result}}")]),
        (31, "Who calls decide", "piechart",
         'hook: git ran the call (GIT_INDEX_FILE/GIT_EXEC_PATH set). tty: a person at the terminal. script: everything else. Mostly tty means people are trying it; mostly hook means they use it.',
         [(f"sum by (caller) (sum_over_time(nav_pilot_decide_result_total{SEL}[$__range]))", "{{caller}}")]),
        (32, "decide latency, p95 by evidence size", "timeseries",
         'Answered calls only. Waiting for the server lock is included: an agent session on the same server delays decide, and that is the wait a git hook notices.',
         [(f'histogram_quantile(0.95, sum by (evidence_size, le) (sum_over_time(nav_pilot_decide_latency_ms_bucket{SEL}[$__range])))', "p95 {{evidence_size}}"),
          (f'histogram_quantile(0.5, sum by (le) (sum_over_time(nav_pilot_decide_latency_ms_bucket{SEL}[$__range])))', "p50 all")]),
        (33, "Share of choices with p below 0.8, by model", "timeseries",
         'How often the model is unsure of its own choice. If it rises for a model, the questions people ask it are harder than what it was measured on.',
         [(f'100 * sum by (model) (sum_over_time(nav_pilot_decide_p_choice_bucket{{version=~"$version", le="0.8"}}[$__range]))'
           f' / clamp_min(sum by (model) (sum_over_time(nav_pilot_decide_p_choice_count{SEL}[$__range])), 1)', "{{model}}")]),

        (40, "Loop guard triggered, by rule", "timeseries",
         'session=local is the guard in front of the local model, which ends the turn. session=cloud is the postToolUse hook in Copilot sessions against cloud models, which can only rewrite the result. Both are counted when the nav-pilot session ends, not when they happen.',
         [(f"sum by (rule, session) (sum_over_time(nav_pilot_hook_loop_guard_total{SEL}[$__interval]))", "{{session}} {{rule}}")]),
        (41, "Redactions in tool results", "timeseries",
         'Counts, never content. secret and fnr are masked values; injection_note is results that got a note that the text looks like instructions.',
         [(f"sum by (kind) (sum_over_time(nav_pilot_hook_redact_total{SEL}[$__interval]))", "{{kind}}")]),
        (42, "Action check, by outcome and category", "timeseries",
         'Whether the local model got to check a risky shell command before it ran, and what it answered. flagged are commands the model found risky; skipped_* are checks that were not made (deadline, no server, or error).',
         [(f"sum by (outcome, category) (sum_over_time(nav_pilot_hook_action_check_total{SEL}[$__interval]))",
           "{{outcome}} {{category}}")]),
    ]


# Panels whose instruments have no series yet. Kept with their intended query
# so they light up once data arrives.
PLACEHOLDERS = {30: 1015, 31: 1015, 32: 1015, 33: 1015, 40: 2915, 42: 2915}


# Panels whose values are not plain counts.
UNITS = {20: "s", 32: "ms", 33: "percent"}


ROWS = [
    ("Adoption", [1, 2, 3, 4]),
    ("Dispatch: is the worker used?", [10, 11, 12, 13]),
    ("Local server", [20, 21, 22]),
    ("alpha decide", [30, 31, 32, 33]),
    ("Hooks", [40, 41, 42]),
]


def house_viz(house, group):
    """Borrow a vizConfig of this type from the CLI dashboard, so the two
    dashboards cannot drift apart on styling."""
    for p in house["elements"].values():
        if p["spec"]["vizConfig"]["group"] == group:
            return copy.deepcopy(p["spec"]["vizConfig"])
    # No panel of that type to copy: a minimal one, still valid.
    return {"group": group, "kind": "VizConfig",
            "spec": {"fieldConfig": {"defaults": {}, "overrides": []}, "options": {}},
            "version": "13.0.1+security-01"}


def build():
    house = json.loads(HOUSE.read_text())["spec"]
    ds = {"name": "${DS_METRICS}"}

    elements = {}
    for pid, title, group, desc, queries in panels():
        if pid in PLACEHOLDERS:
            title = f"{title} (placeholder)"
            desc = (f"Placeholder: no series yet in Mimir. Tracked in #{PLACEHOLDERS[pid]}. "
                    f"The query is the intended one. {desc}")
        viz = house_viz(house, group)
        if group == "timeseries":
            # Half-width charts: legend table below. Counts sum; latency and
            # shares never do, so they show mean, max and last instead.
            exprs = [e for e, _ in queries]
            counter = not any(k in e for e in exprs for k in ("histogram_quantile", " / "))
            viz["spec"].setdefault("options", {})["legend"] = {
                "displayMode": "table", "placement": "bottom", "showLegend": True, "sortDesc": True,
                "calcs": ["sum"] if counter else ["mean", "max", "lastNotNull"],
                "sortBy": "Total" if counter else "Max"}
        if group == "stat" and pid == 4:
            viz["spec"]["fieldConfig"]["defaults"]["unit"] = "percent"
        if pid in UNITS:
            viz["spec"]["fieldConfig"]["defaults"]["unit"] = UNITS[pid]
        elements[f"panel-{pid}"] = {
            "kind": "Panel",
            "spec": {
                "data": {"kind": "QueryGroup", "spec": {
                    "queries": [
                        {"kind": "PanelQuery", "spec": {
                            "hidden": False, "refId": chr(ord("A") + i),
                            "query": {"datasource": ds, "group": "prometheus", "kind": "DataQuery",
                                      "spec": {"expr": expr, "legendFormat": legend}, "version": "v0"},
                        }} for i, (expr, legend) in enumerate(queries)
                    ],
                    "queryOptions": {}, "transformations": []}},
                "description": desc,
                "id": pid,
                "links": [],
                "title": title,
                "vizConfig": viz,
            },
        }

    rows = []
    for n, (title, ids) in enumerate(ROWS):
        # Same grid as the other dashboards: every panel in a row has the same
        # size; stats are one strip, charts go two or three to a line.
        items = []
        k = len(ids) if len(ids) <= 3 else 2 if len(ids) % 2 == 0 else 3
        for i, pid in enumerate(ids):
            if pid < 10:
                width, height, x, y = 24 // len(ids), 4, i * (24 // len(ids)), 0
            else:
                width, height, x, y = 24 // k, 8, (i % k) * (24 // k), (i // k) * 8
            items.append({"kind": "GridLayoutItem", "spec": {
                "element": {"kind": "ElementReference", "name": f"panel-{pid}"},
                "height": height, "width": width, "x": x, "y": y}})
        rows.append({"kind": "RowsLayoutRow", "spec": {
            "collapse": n >= 2, "title": title,  # two rows open, the rest load when expanded
            "layout": {"kind": "GridLayout", "spec": {"items": items}}}})

    # Only the two variables that mean anything here. The CLI dashboard's
    # command/scope filters have no counterpart in the local instruments, and a
    # filter that cannot narrow anything is a control that lies.
    variables = [v for v in house["variables"]
                 if v["spec"]["name"] in ("DS_METRICS", "version")]

    spec = {
        "annotations": house.get("annotations", {"kind": "AnnotationQueries", "spec": []}),
        "cursorSync": house.get("cursorSync", "Off"),
        "description": (
            "Local inference in nav-pilot alpha: who uses it, what they send there, and whether the "
            "server comes up. Every query uses sum_over_time, not increase(): all runs on a machine "
            "share a series and start at their own 1, so increase() gives 0. Each sample is what is "
            "new since the process's last export, so the sum counts each run once."),
        "editable": True,
        "elements": elements,
        "layout": {"kind": "RowsLayout", "spec": {"rows": rows}},
        "links": [],
        "liveNow": False,
        "preload": False,
        "tags": ["nav-pilot", "domain:nav-pilot", "concern:insights", "local-inference", "alpha"],
        "timeSettings": house.get("timeSettings", {"from": "now-7d", "to": "now"}),
        "title": "nav-pilot local inference (nais)",
        "variables": variables,
    }
    # A complete v2 resource, so Grafana imports it with a stable uid.
    return {"apiVersion": API_VERSION, "kind": "Dashboard",
            "metadata": {"name": "copilot-nav-pilot-local"}, "spec": spec}


def main():
    ap = argparse.ArgumentParser(prog="build-local-dashboard")
    ap.add_argument("--check", action="store_true", help="fail if the committed file is stale")
    args = ap.parse_args()

    want = json.dumps(build(), indent=2, ensure_ascii=False) + "\n"
    if args.check:
        have = OUT.read_text() if OUT.exists() else ""
        if have != want:
            print("✗ dashboards/nais/nav-pilot-local.json is out of date — run scripts/build-local-dashboard.py")
            return 1
        print("✓ dashboard is up to date")
        return 0
    OUT.write_text(want)
    print(f"wrote {OUT.relative_to(ROOT)}: {len(build()['spec']['elements'])} panels, {len(ROWS)} rows")
    return 0


if __name__ == "__main__":
    sys.exit(main())
