#!/usr/bin/env python3
"""Generate dashboards/nav-pilot-local.json.

Written as a generator rather than hand-edited JSON because the interesting
part of this dashboard is nine PromQL expressions, and everything around them
is nine hundred lines of Grafana boilerplate that hides them. Here the queries
sit together where they can be read against each other, and the house style is
lifted from `nav-pilot-cli.json` at build time so the two cannot drift.

Two rules are baked in, both bought with a wrong PR:

  sum_over_time, never increase(). A nav-pilot process exports its counters
  once and exits, so increase() has no second sample to difference and returns
  0.0 on every one of these series. Verified against live data: increase()
  totalled 0.0 across five series where sum_over_time totalled 58.

  histogram_quantile over sum_over_time of _bucket is correct *here*, and only
  because these instruments are recorded once per process at exit. Each export
  is one run's own cumulative buckets, so summing them across the window gives
  a valid aggregate histogram. It would be wrong for an instrument recorded at
  startup, which the 10-second PeriodicReader re-exports for the life of the
  process — nav_pilot_version_skew_days is the example not to copy.

  scripts/build-local-dashboard.py            # write the dashboard
  scripts/build-local-dashboard.py --check    # fail if it is out of date
"""
import argparse
import copy
import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
HOUSE = ROOT / "dashboards" / "nav-pilot-cli.json"
OUT = ROOT / "dashboards" / "nav-pilot-local.json"

SEL = '{version=~"$version"}'
SEL_LE0 = '{version=~"$version", le="0"}'


def panels():
    """Every panel, as (id, title, viz, description, [(expr, legend)])."""
    return [
        (1, "Maskiner med lokal kjøring", "stat",
         "Hvor mange som har kjørt minst én lokal sesjon i perioden. Nevneren for alt annet her.",
         [(f"count(count by (device_id) (sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__range])))", "")]),
        (2, "Sesjoner", "stat",
         "Én sesjon er én klientkjøring med lokal worker tilgjengelig, talt når klienten avslutter.",
         [(f"sum(sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__range]))", "")]),
        (3, "Oppgaver sendt lokalt", "stat",
         "Sum av oppgaver hovedagenten faktisk sendte til den lokale modellen.",
         [(f"sum(sum_over_time(nav_pilot_local_dispatches_sum{SEL}[$__range]))", "")]),
        (4, "Andel sesjoner uten dispatch", "stat",
         "Panelet som avgjør om alfaen utvides. Se fordelingen under for hvorfor tallet er "
         "som det er — en null betyr ikke det samme i de to tilfellene.",
         [(f"100 * sum(sum_over_time(nav_pilot_local_dispatches_bucket{SEL_LE0}[$__range]))"
           f" / clamp_min(sum(sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__range])), 1)", "")]),

        (10, "Sesjoner uten dispatch, delt på om klienten så workeren", "timeseries",
         "Den ene grafen som er verdt å bygge dashbordet for. saw_traffic=true betyr at klienten "
         "nådde workeren og lot være å bruke den — det er en dom over modellen. saw_traffic=false "
         "betyr at oppkoblingen aldri kom fram, som er vår feil, ikke modellens. Uten dette skillet "
         "er en null tre forskjellige ting rapportert som én.\n\n"
         "VENTER PÅ DATA: saw_traffic ble lagt til etter at alfaen gikk ut, så panelet er tomt til "
         "folk har oppdatert. Tomt er riktig her — en null vi ikke kan tolke er verre enn ingen strek.",
         [(f'sum by (saw_traffic) (sum_over_time(nav_pilot_local_dispatches_bucket{SEL_LE0}[$__interval]))',
           "så workeren: {{saw_traffic}}")]),
        # Explicit differences, not the raw le series. A bar per cumulative
        # bucket reads as "31 sessions sent 0, 55 sent 5" when it actually means
        # "≤0" and "≤5", and every viewer makes that mistake once.
        (11, "Oppgaver per sesjon", "barchart",
         "Fordeling, aldri gjennomsnitt: de fleste sesjonene sender ingenting og noen få sender "
         "mye, og et snitt av de to beskriver ingen faktisk sesjon. Bøttene er trukket fra "
         "hverandre, så hver søyle er sesjoner i akkurat det intervallet.",
         [(f'sum(sum_over_time(nav_pilot_local_dispatches_bucket{SEL_LE0}[$__range]))', "0 oppgaver"),
          (f'sum(sum_over_time(nav_pilot_local_dispatches_bucket{{version=~"$version", le="5"}}[$__range]))'
           f' - sum(sum_over_time(nav_pilot_local_dispatches_bucket{SEL_LE0}[$__range]))', "1–5"),
          (f'sum(sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__range]))'
           f' - sum(sum_over_time(nav_pilot_local_dispatches_bucket{{version=~"$version", le="5"}}[$__range]))', "6+")]),
        (12, "Sesjoner per intervall, per klient", "timeseries",
         "opencode kjører modellen som subagent under en skyagent; Copilot CLI kjører hele økten lokalt.",
         [(f"sum by (client) (sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__interval]))", "{{client}}")]),

        (20, "Tid til klar", "timeseries",
         "p50 og p95 for oppstarter som faktisk kom opp. Filteret outcome=\"ready\" er ikke pynt: "
         "en oppstart som går i timeout bidrar med ti minutter, og en p95 uten filteret måler "
         "feilene, ikke ventetiden.\n\n"
         "VENTER PÅ DATA: outcome kom inn etter forrige release, så panelet fylles først når folk "
         "har oppdatert.",
         [(f'histogram_quantile(0.5, sum by (le) (sum_over_time(nav_pilot_local_ready_seconds_bucket{{version=~"$version", outcome="ready"}}[$__range])))', "p50"),
          (f'histogram_quantile(0.95, sum by (le) (sum_over_time(nav_pilot_local_ready_seconds_bucket{{version=~"$version", outcome="ready"}}[$__range])))', "p95")]),
        (21, "Hvordan oppstarter endte", "timeseries",
         "ready, failed eller interrupted. Fram til denne fantes ble bare vellykkede oppstarter "
         "registrert, så den trege halen manglet per konstruksjon.\n\n"
         "VENTER PÅ DATA: outcome kom inn etter forrige release og fylles etter hvert som folk oppdaterer.",
         [(f'sum by (outcome) (sum_over_time(nav_pilot_local_ready_seconds_count{SEL}[$__interval]))', "{{outcome}}")]),
        (22, "Modeller i bruk", "piechart",
         "Skal være én. Flere betyr at noen kjører noe vi ikke har målt.",
         [(f"sum by (model) (sum_over_time(nav_pilot_local_dispatches_count{SEL}[$__range]))", "{{model}}")]),
    ]


ROWS = [
    ("Adopsjon", [1, 2, 3, 4]),
    ("Dispatch — brukes workeren?", [10, 11, 12]),
    ("Lokal server", [20, 21, 22]),
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
    house = json.loads(HOUSE.read_text())
    ds = {"name": "${DS_METRICS}"}

    elements = {}
    for pid, title, group, desc, queries in panels():
        viz = house_viz(house, group)
        if group == "stat" and pid == 4:
            viz["spec"]["fieldConfig"]["defaults"]["unit"] = "percent"
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
    for title, ids in ROWS:
        items, x = [], 0
        width = 24 // len(ids)
        for pid in ids:
            items.append({"kind": "GridLayoutItem", "spec": {
                "element": {"kind": "ElementReference", "name": f"panel-{pid}"},
                "height": 8 if pid >= 10 else 4, "width": width, "x": x, "y": 0}})
            x += width
        rows.append({"kind": "RowsLayoutRow", "spec": {
            "collapse": False, "title": title,
            "layout": {"kind": "GridLayout", "spec": {"items": items}}}})

    # Only the two variables that mean anything here. The CLI dashboard's
    # command/scope filters have no counterpart in the local instruments, and a
    # filter that cannot narrow anything is a control that lies.
    variables = [v for v in house["variables"]
                 if v["spec"]["name"] in ("DS_METRICS", "version")]

    return {
        "annotations": house.get("annotations", {"kind": "AnnotationQueries", "spec": []}),
        "cursorSync": house.get("cursorSync", "Off"),
        "description": (
            "Lokal inferens i nav-pilot alpha: hvem bruker den, hva sender de dit, og kommer "
            "serveren opp. Alle spørringer bruker sum_over_time, ikke increase() — en nav-pilot-"
            "prosess eksporterer én gang og avslutter, så increase() har ingen andre sample å "
            "regne differanse fra og gir 0."),
        "editable": True,
        "elements": elements,
        "layout": {"kind": "RowsLayout", "spec": {"rows": rows}},
        "links": [],
        "liveNow": False,
        "preload": False,
        "tags": ["nav-pilot", "lokal-inferens", "alpha"],
        "timeSettings": house.get("timeSettings", {"from": "now-7d", "to": "now"}),
        "title": "nav-pilot — lokal inferens",
        "variables": variables,
    }


def main():
    ap = argparse.ArgumentParser(prog="build-local-dashboard")
    ap.add_argument("--check", action="store_true", help="fail if the committed file is stale")
    args = ap.parse_args()

    want = json.dumps(build(), indent=2, ensure_ascii=False) + "\n"
    if args.check:
        have = OUT.read_text() if OUT.exists() else ""
        if have != want:
            print("✗ dashboards/nav-pilot-local.json is out of date — run scripts/build-local-dashboard.py")
            return 1
        print("✓ dashboard is up to date")
        return 0
    OUT.write_text(want)
    print(f"wrote {OUT.relative_to(ROOT)}: {len(build()['elements'])} panels, {len(ROWS)} rows")
    return 0


if __name__ == "__main__":
    sys.exit(main())
