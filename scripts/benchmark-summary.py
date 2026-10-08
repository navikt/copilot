#!/usr/bin/env python3
"""Summarise committed benchmark runs into docs/golden-baselines/summary.json.

Reads every docs/golden-baselines/**/*.txt whose header names a suite (written
by `nav-pilot-golden.sh --suite ... --save-baseline`), with the -results.psv,
-attempts.psv and -usage.psv beside it. Older baselines carry no suite and are
left out: they ran other prompt selections and are not comparable.

  scripts/benchmark-summary.py            # write summary.json
  scripts/benchmark-summary.py --check    # fail if summary.json is stale
  scripts/benchmark-summary.py --selftest

A run is refused when its main-agent usage rows (empty agent_id) name any
model but the one in its header: a frontmatter pin once overrode --model, and
every number would have been credited to the wrong model. Subagents may run
on other models; those are listed in subagent_models and still counted in
credits. A run with no usage rows cannot be checked: model_verified is false.

Credits are exact assistant_usage_events (total_nano_aiu / 1e9) summed per
run, retries and subagents included. When tracking was incomplete for any
attempt, or a run has no usage rows, credits is null and usage_complete false:
a partial sum is not an exact number. Wall time is the sum of the run's CLI
calls. `effort` is what was requested ("default" when no --effort was
passed); `ran_at` is the majority effort in the usage rows, "default" when
they carry none.
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
SUITES = {"planning", "review", "norsk", "coding", "research", "kafka", "rust"}
EFFORTS = {"low", "medium", "high", "default"}

# Shown on ki-utvikling.nav.no/modeller. Every hard check a suite records needs
# a line here; an unknown ID stops the summary rather than showing English.
CHECKS = {
    "2": ("t2", "Stiller spørsmål i fase 1 og venter på svar"),
    "3": ("t3", "Tar opp personvern og tilgangskontroll"),
    "4": ("t4", "Planen i fase 2 markerer rød sone. Kjøres bare når fase 1 stilte spørsmål"),
    "5": ("t5", "Velger TokenX, ikke client_credentials, når kallet gjelder en bruker"),
    "7": ("t7", "Jackson-migrering: spør ikke om personvern eller tilgang"),
    "7b": ("t7b", "Nytt fødselsnummer i en Kafka-melding: tar opp personvern"),
    "rv1": ("rv1", "Kotlin: finner alle tre plantede feil"),
    "rv2": ("rv2", "Kotlin: riktig linje for hver feil"),
    "rv3": ("rv3", "TSX: finner alle fire plantede feil"),
    "rv4": ("rv4", "TSX: riktig linje for hver feil"),
    "rv5": ("rv5", "PR med åtte filer: riktig fil og linje for sikkerhets- og personvernfeilene"),
    "rv6": ("rv6", "PR med åtte filer: finner designfeilene (idempotens og dobbel skriving)"),
    "rv7": ("rv7", "PR med åtte filer: SQL og tilgang får høy prioritet, en kosmetisk merknad gjør det ikke"),
    "rv8": ("rv8", "Ren fil: ingen funn med høy prioritet, og svaret sier at ingenting er kritisk"),
    "no1": ("no1", "Ingen nynorske former"),
    "no2": ("no2", "Ingen KI-floskler"),
    "no3": ("no3", "Skriver ikke «AI»"),
    "no4": ("no4", "Holder seg innenfor 30–90 ord"),
    "ko1": ("ko1", "Go: testene er grønne etterpå"),
    "ko2": ("ko2", "Go: endrer bare filen med feilen"),
    "ko3": ("ko3", "TS: testen er grønn etterpå"),
    "ko4": ("ko4", "TS: endrer bare filen med feilen"),
    "ko5": ("ko5", "Go, to filer: testene er grønne etterpå"),
    "ko6": ("ko6", "Go, to filer: endrer bare de to filene med feilen"),
    "re1": ("re1", "Riktig fil og linje for hver bruk av konstanten"),
    "re2": ("re2", "Sier at funksjonen ikke kalles, uten å dikte opp filer"),
    "re3": ("re3", "Oppsummerer i høyst tre punkter"),
    "kf1": ("kf1", "Kafka-konsument: samme hendelse utbetales én gang, og offset commites etter behandling"),
    "kf2": ("kf2", "Kafka-konsument: endrer bare i prosjektet med feilen"),
    "kf3": ("kf3", "Kafka-hendelse: nytt felt, gamle meldinger og ukjente felt leses"),
    "kf4": ("kf4", "Kafka-hendelse: endrer bare i prosjektet med hendelsen"),
    "rs1": ("rs1", "Rust: lånefeilen er rettet, og køen tømmes i riktig rekkefølge"),
    "rs2": ("rs2", "Rust: endrer bare i craten med feilen"),
    "rs3": ("rs3", "Rust: feiltyper med thiserror og en test per feil"),
    "rs4": ("rs4", "Rust: endrer bare i craten med parseren"),
    "re4": ("re4", "Oppsummeringen nevner endepunktet"),
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


def summarise_run(txt):
    h = header(txt)
    base = str(txt)[: -len(".txt")]
    n = int(h["repeats"])
    results = rows(Path(base + "-results.psv"))
    attempts = rows(Path(base + "-attempts.psv"))
    usage = rows(Path(base + "-usage.psv"))
    model = h["model"]

    main_models = {r[5] for r in usage if not r[16]}
    if usage and main_models != {model}:
        raise SystemExit(f"{txt.name}: header says {model!r}, main-agent usage rows say {sorted(main_models)}")

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
    order.sort(key=list(CHECKS).index)

    per_run = defaultdict(int)
    for r in usage:
        per_run[r[1]] += int(r[12] or 0)
    complete = (
        bool(attempts)
        and all(r[6] == "true" for r in attempts)
        and all(str(i) in per_run for i in range(1, n + 1))
    )
    credits = None
    if complete:
        values = [per_run[str(i)] / 1e9 for i in range(1, n + 1)]
        credits = {"median": round(statistics.median(values), 3), "mean": round(statistics.mean(values), 3)}

    wall = defaultdict(int)
    for r in attempts:
        wall[r[1]] += int(r[5])
    wall_values = [wall.get(str(i), 0) / 1000 for i in range(1, n + 1)]

    observed = Counter(r[6] for r in usage if r[6])
    run = {
        "date": h["date"],
        "suite": h["suite"],
        "model": model,
        "model_verified": bool(usage),
        "subagent_models": sorted({r[5] for r in usage if r[16]} - {model}),
        "effort": "default" if h["effort"] == "CLI default" else h["effort"],
        "ran_at": observed.most_common(1)[0][0] if observed else "default",
        "cli_version": (re.search(r"\d+\.\d+\.\d+(-\d+)?", h.get("clientVersion", "")) or [""])[0],
        "n": n,
        "smoke": "smoke" in str(txt.relative_to(REPO)),
        "checks": [{"id": CHECKS[i][0], "description": CHECKS[i][1], "passed": checks[i]} for i in order],
        "credits": credits,
        "usage_complete": complete,
        "wall_seconds": {"median": round(statistics.median(wall_values), 1)},
        "source": str(txt.relative_to(REPO)),
    }
    if run["suite"] not in SUITES or run["effort"] not in EFFORTS or model == "CLI default":
        raise SystemExit(f"{txt.name}: suite {run['suite']!r}, effort {run['effort']!r} or model {model!r} is outside the summary contract")
    return run


def build(directory=BASELINES):
    runs = [
        summarise_run(txt)
        for txt in sorted(directory.rglob("*.txt"))
        # delegation measures which agents a persona calls, not a model, and
        # has no place on ki-utvikling.nav.no/modeller.
        if header(txt).get("suite", "none") not in ("none", "delegation")
    ]
    runs.sort(key=lambda r: (r["date"], r["suite"], r["model"], r["effort"], r["source"]))
    generated = max((r["date"] for r in runs), default="1970-01-01")
    return json.dumps({"generated": generated, "runs": runs}, ensure_ascii=False, indent=2) + "\n"


def selftest():
    global REPO
    hdr = "# agent: code-review\n# suite:        review\n# date: 2026-09-30\n# clientVersion: GitHub Copilot CLI 1.0.90-5.\n# model: gpt-6-sol\n# effort: low\n# repeats: 2\n"
    u = "cr-tsx|{run}|s|1|0|{m}|{e}|0|0|0|0|0|{nano}|0|0|stop||\n"
    ok_usage = u.format(run=1, m="gpt-6-sol", e="low", nano=2_000_000_000) + u.format(run=2, m="gpt-6-sol", e="low", nano=4_000_000_000)
    attempts = "cr-tsx|1|0|pass|9|1000|true|\ncr-tsx|2|0|pass|9|3000|{c}|\n"

    def run_with(usage, complete="true"):
        with tempfile.TemporaryDirectory() as tmp:
            d = Path(tmp)
            (d / "a.txt").write_text(hdr)
            (d / "a-results.psv").write_text(hdr + "rv4|1|pass|x|\nrv4|2|fail|x|y\ncr4|1|soft-pass|x|\n")
            (d / "a-attempts.psv").write_text(attempts.format(c=complete))
            (d / "a-usage.psv").write_text(usage)
            (d / "old.txt").write_text("# agent: nav-pilot\n# repeats: 5\n")
            global REPO
            REPO, saved = d, REPO
            try:
                return json.loads(build(d))["runs"]
            finally:
                REPO = saved

    runs = run_with(ok_usage)
    run = runs[0]
    assert len(runs) == 1, "a baseline without a suite must be left out"
    assert run["checks"] == [{"id": "rv4", "description": CHECKS["rv4"][1], "passed": 1}], run["checks"]
    assert run["credits"] == {"median": 3.0, "mean": 3.0} and run["usage_complete"], run
    assert run["wall_seconds"] == {"median": 2.0} and run["cli_version"] == "1.0.90-5", run
    assert run["effort"] == "low" and run["ran_at"] == "low" and not run["smoke"], run
    # A model that takes no effort: requested low stays low, ran_at says so.
    assert run_with(ok_usage.replace("|low|", "||"))[0]["ran_at"] == "default"
    # Partial usage is not exact usage.
    assert run_with(ok_usage, complete="false")[0]["credits"] is None
    only_run1 = ok_usage.splitlines(keepends=True)[0]
    assert run_with(only_run1)[0]["credits"] is None
    # A subagent on another model is listed, and its credits still count.
    sub = run_with(ok_usage + u.format(run=1, m="gpt-6-luna", e="", nano=1_000_000_000).replace("|stop||", "|stop|sub-1|"))[0]
    assert sub["subagent_models"] == ["gpt-6-luna"] and sub["credits"]["mean"] == 3.5, sub
    # No usage rows: nothing verifies the model.
    assert run_with("")[0]["model_verified"] is False
    # A pin that overrode --model is refused, not credited to the wrong model.
    try:
        run_with(ok_usage.replace("gpt-6-sol", "gpt-5.3-codex", 1))
        raise AssertionError("a run on another model than its header was accepted")
    except SystemExit as e:
        assert "usage rows say" in str(e), e
    print("benchmark-summary selftest passed")


def main():
    out = BASELINES / "summary.json"
    if "--selftest" in sys.argv:
        selftest()
        return
    text = build()
    if "--check" in sys.argv:
        if not out.exists() or out.read_text() != text:
            sys.exit("docs/golden-baselines/summary.json is stale: run scripts/benchmark-summary.py")
        return
    out.write_text(text)
    print(f"wrote {out.relative_to(REPO)}")


if __name__ == "__main__":
    main()
