#!/usr/bin/env python3
"""Summarise committed benchmark runs into docs/golden-baselines/summary.json.

Reads every docs/golden-baselines/*.txt whose header names a suite (written by
`nav-pilot-golden.sh --suite ... --save-baseline`), with the -results.psv,
-attempts.psv and -usage.psv beside it. Older baselines carry no suite and are
left out: they ran other prompt selections and are not comparable.

  scripts/benchmark-summary.py            # write summary.json
  scripts/benchmark-summary.py --check    # fail if summary.json is stale
  scripts/benchmark-summary.py --selftest

Credits are exact assistant_usage_events (total_nano_aiu / 1e9) summed per
run, retries and subagents included, and null when the run recorded no usage.
Wall time is the sum of the run's CLI calls. Effort is what the usage rows
say the model ran at; a model that takes no effort leaves them empty, and
that is recorded as "default".
"""

import json
import re
import statistics
import sys
import tempfile
from collections import Counter, defaultdict
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
BASELINES = REPO / "docs" / "golden-baselines"
MODELS_GO = REPO / "cli" / "nav-pilot" / "internal" / "domain" / "known_models_gen.go"
SUITES = {"planning", "review", "norsk", "coding"}
EFFORTS = {"low", "medium", "high", "default"}

# Shown on ki-utvikling.nav.no/modeller. Every hard check a suite records needs
# a line here; an unknown ID stops the summary rather than showing English.
CHECKS = {
    "2": ("t2", "Stopper etter fase 1 med spørsmål til brukeren"),
    "3": ("t3", "Tar opp personvern og tilgangskontroll"),
    "4": ("t4", "Fase 2-planen erklærer rød sone"),
    "5": ("t5", "Anbefaler TokenX for brukerkontekst, ikke client_credentials"),
    "rv1": ("rv1", "Kotlin: finner alle tre plantede feil"),
    "rv2": ("rv2", "Kotlin: oppgir riktig linje for hver feil"),
    "rv3": ("rv3", "TSX: finner alle fire plantede feil"),
    "rv4": ("rv4", "TSX: oppgir riktig linje for hver feil"),
    "no1": ("no1", "Ingen nynorske former"),
    "no2": ("no2", "Ingen KI-floskler"),
    "no3": ("no3", "Skriver «KI», ikke «AI»"),
    "no4": ("no4", "Holder lengdegrensen"),
    "ko1": ("ko1", "Go: testene går grønt etterpå"),
    "ko2": ("ko2", "Go: endrer bare filen med feilen"),
    "ko3": ("ko3", "TS: testen går grønt etterpå"),
    "ko4": ("ko4", "TS: endrer bare filen med feilen"),
    "ko5": ("ko5", "Go, to filer: testene går grønt etterpå"),
    "ko6": ("ko6", "Go, to filer: endrer nøyaktig de to filene"),
}


def header(path):
    out = {}
    for line in path.read_text().splitlines():
        m = re.match(r"# ([A-Za-z_]+):\s*(.*)", line)
        if m:
            out.setdefault(m.group(1), m.group(2).strip())
    return out


def rows(path):
    if not path.exists():
        return []
    return [l.split("|") for l in path.read_text().splitlines() if l and not l.startswith("#")]


def labels():
    return dict(re.findall(r'\{ID: "([^"]+)", Label: "([^"]+)"\}', MODELS_GO.read_text()))


def median(values):
    return statistics.median(values) if values else None


def summarise_run(txt, names):
    h = header(txt)
    base = str(txt)[: -len(".txt")]
    n = int(h["repeats"])
    results = rows(Path(base + "-results.psv"))
    attempts = rows(Path(base + "-attempts.psv"))
    usage = rows(Path(base + "-usage.psv"))

    checks, order = defaultdict(int), []
    for r in results:
        rid, status = r[0], r[2]
        if status.startswith("soft"):
            continue
        if rid not in CHECKS:
            raise SystemExit(f"{txt.name}: check {rid!r} has no description in CHECKS")
        if rid not in order:
            order.append(rid)
        checks[rid] += status == "pass"

    credits = None
    if usage:
        per_run = defaultdict(int)
        for r in usage:
            per_run[r[1]] += int(r[12] or 0)
        values = [per_run.get(str(i), 0) / 1e9 for i in range(1, n + 1)]
        credits = {"median": round(median(values), 3), "mean": round(statistics.mean(values), 3)}

    wall = defaultdict(int)
    for r in attempts:
        wall[r[1]] += int(r[5])
    wall_values = [wall.get(str(i), 0) / 1000 for i in range(1, n + 1)]

    ran_at = Counter(r[6] for r in usage if r[6])
    effort = ran_at.most_common(1)[0][0] if ran_at else ("default" if usage else h["effort"])
    effort = "default" if effort == "CLI default" else effort

    model = h["model"]
    label = names.get(model, model)
    if "smoke" in str(txt.relative_to(REPO)):
        label += " (røyktest)"
    run = {
        "date": h["date"],
        "suite": h["suite"],
        "model": model,
        "label": label,
        "effort": effort,
        "cli_version": (re.search(r"\d+\.\d+\.\d+(-\d+)?", h.get("clientVersion", "")) or [""])[0],
        "n": n,
        "checks": [{"id": CHECKS[i][0], "description": CHECKS[i][1], "passed": checks[i]} for i in order],
        "credits": credits,
        "wall_seconds": {"median": round(median(wall_values), 1)},
        "source": str(txt.relative_to(REPO)),
    }
    if run["suite"] not in SUITES or run["effort"] not in EFFORTS:
        raise SystemExit(f"{txt.name}: suite {run['suite']!r} / effort {run['effort']!r} is outside the summary contract")
    return run


def build(directory=BASELINES):
    names = labels()
    runs = [
        summarise_run(txt, names)
        for txt in sorted(directory.rglob("*.txt"))
        if header(txt).get("suite", "none") != "none"
    ]
    runs.sort(key=lambda r: (r["date"], r["suite"], r["model"], r["effort"], r["source"]))
    generated = max((r["date"] for r in runs), default="1970-01-01")
    return json.dumps({"generated": generated, "runs": runs}, ensure_ascii=False, indent=2) + "\n"


def selftest():
    with tempfile.TemporaryDirectory() as tmp:
        d = Path(tmp)
        hdr = "# agent: code-review\n# suite:        review\n# date: 2026-09-30\n# clientVersion: GitHub Copilot CLI 1.0.90-5.\n# model: gpt-6-sol\n# effort: low\n# repeats: 2\n"
        (d / "a.txt").write_text(hdr)
        (d / "a-results.psv").write_text(hdr + "rv4|1|pass|x|\nrv4|2|fail|x|y\ncr4|1|soft-pass|x|\n")
        (d / "a-attempts.psv").write_text("cr-tsx|1|0|pass|9|1000|true|\ncr-tsx|2|0|pass|9|3000|true|\n")
        u = "cr-tsx|{run}|s|1|0|gpt-6-sol|{e}|0|0|0|0|0|{nano}|0|0|stop||\n"
        (d / "a-usage.psv").write_text(u.format(run=1, e="low", nano=2_000_000_000) + u.format(run=2, e="low", nano=4_000_000_000))
        (d / "old.txt").write_text("# agent: nav-pilot\n# repeats: 5\n")
        global REPO
        REPO, saved = d, REPO
        try:
            got = json.loads(build(d))
            (d / "a-usage.psv").write_text(u.format(run=1, e="", nano=1) + u.format(run=2, e="", nano=1))
            no_effort = json.loads(build(d))["runs"][0]["effort"]
        finally:
            REPO = saved
    run = got["runs"][0]
    assert len(got["runs"]) == 1, "a baseline without a suite must be left out"
    assert run["checks"] == [{"id": "rv4", "description": CHECKS["rv4"][1], "passed": 1}], run["checks"]
    assert run["credits"] == {"median": 3.0, "mean": 3.0}, run["credits"]
    assert run["wall_seconds"] == {"median": 2.0} and run["cli_version"] == "1.0.90-5", run
    assert run["label"] == "GPT-6 Sol" and run["effort"] == "low", run
    assert no_effort == "default", no_effort
    print("benchmark-summary selftest passed")


def main():
    out = BASELINES / "summary.json"
    if "--selftest" in sys.argv:
        return selftest()
    text = build()
    if "--check" in sys.argv:
        if not out.exists() or out.read_text() != text:
            sys.exit("docs/golden-baselines/summary.json is stale: run scripts/benchmark-summary.py")
        return
    out.write_text(text)
    print(f"wrote {out.relative_to(REPO)}")


if __name__ == "__main__":
    main()
