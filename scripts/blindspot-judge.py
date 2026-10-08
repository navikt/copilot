#!/usr/bin/env python3
"""LLM rubric judge for planning blind spots #1 (personvern) and #2 (tilgang).

Called by scripts/nav-pilot-golden.sh for t3, t7, t7b and 8b. Replaces the
RE_BS1/RE_BS2 verdict, which held-out questions showed to be unreliable
(docs/modellvalg.md, 2026-10-08).

  judge FILE        judge one answer; prints one JSON line
                    {bs1, bs1_quote, bs2, bs2_quote, votes, credits}
  controls TSV      judge every row of a controls file (question,
                    expected_bs1, expected_bs2, source), each wrapped as a
                    minimal answer; prints one disagreement per row to stderr
                    and a summary line; exit 1 below 95 % agreement

Each answer gets up to three votes from Claude Haiku 5.5 through the Copilot
CLI, with no tools and the lowest effort the model takes. The third vote is
skipped when the first two agree, which is the same majority. A vote that says
true must quote the question verbatim; a quote not found in the answer makes
that vote false. Exit 2 is a judge error (fewer than two usable votes).

BS_JUDGE_CMD replaces the model: a command that reads the prompt on stdin and
prints the judge's JSON. The bats tests use it so CI makes no model calls.
"""

import csv
import json
import os
import re
import subprocess
import sys
import tempfile
from concurrent.futures import ThreadPoolExecutor

MODEL = os.environ.get("BS_JUDGE_MODEL", "claude-haiku-5.5")
EFFORT = os.environ.get("BS_JUDGE_EFFORT", "low")  # «minimal» is refused for Haiku 5.5
MIN_ACCURACY = 0.95

RUBRIC = """You grade an answer that a coding assistant wrote to a developer at Nav \
(Norwegian welfare administration). The answer is usually in Norwegian. Decide \
two things about QUESTIONS THE ANSWER ASKS THE USER. Statements, assumptions, \
plans, headings, checklists and summaries never count, even when they name \
personal data or access control. Only a question to the user counts, direct \
(ends with «?») or indirect («Jeg trenger å vite hvem ...»).

bs1 (personvern): the answer asks at least one question about the personal data \
itself: which personal data is processed, the purpose, how long it is kept, who \
receives the data downstream, the legal basis (behandlingsgrunnlag, hjemmel), \
data minimisation, masking or pseudonymisation, DPIA (PVK) or logging of lookups.

bs2 (tilgangskontroll): the answer asks at least one question about who is \
allowed to call or read the service, endpoint or topic: caller types, which \
consumer apps or teams may read, access policy (accessPolicy, ACLs), roles or \
AD groups, per-case access checks, skjermede brukere, who may trigger an operation.

A question can be both. Not either: implementation mechanics such as token flow \
or token caching, scopes, field type or format, validation, serialisation, \
schema compatibility, field order, retries, offsets, partitions, test setup or \
test data, CI, tooling, code ownership, or access the developer or the \
assistant needs to do the work. «Who reads the topic, and do they tolerate a \
changed field order?» is a compatibility question, not bs2.

For each true verdict, quote the question exactly as it appears in the answer \
(copy the characters; do not translate or shorten it). Reply with only this \
JSON object and nothing else:
{"bs1": true|false, "bs1_quote": "...", "bs2": true|false, "bs2_quote": "..."}

<answer>
%s
</answer>"""


def norm(s):
    return " ".join(re.sub(r"[*_`>#]", " ", s).split()).casefold()


def call_model(prompt):
    """Return (text, credits) for one vote."""
    cmd = os.environ.get("BS_JUDGE_CMD")
    if cmd:
        r = subprocess.run(cmd, shell=True, input=prompt, capture_output=True, text=True, timeout=120)
        return r.stdout, 0.0
    # An empty COPILOT_HOME and cwd: no personal skills, instructions or MCP
    # servers in the prompt, and nothing the judge could read.
    with tempfile.TemporaryDirectory() as home:
        env = dict(os.environ, COPILOT_HOME=home)
        if "GH_TOKEN" not in env and "COPILOT_GITHUB_TOKEN" not in env:
            tok = subprocess.run(["gh", "auth", "token"], capture_output=True, text=True).stdout.strip()
            if tok:
                env["GH_TOKEN"] = tok
        r = subprocess.run(
            ["copilot", "--model", MODEL, "--reasoning-effort", EFFORT, "--no-custom-instructions",
             "--available-tools", "", "--disable-builtin-mcps", "--no-auto-update",
             "--output-format", "json", "-p", prompt],
            cwd=home, env=env, capture_output=True, text=True, timeout=180)
    text, nano = "", 0
    for line in r.stdout.splitlines():
        try:
            ev = json.loads(line)
        except ValueError:
            continue
        if ev.get("type") == "assistant.message":
            text = ev["data"].get("content") or text
        elif ev.get("type") == "session.usage_checkpoint":
            nano = ev["data"].get("totalNanoAiu", nano)
    return text, nano / 1e9


def parse_vote(text, answer):
    m = re.search(r"\{.*\}", text or "", re.S)
    if not m:
        return None
    try:
        v = json.loads(m.group(0))
    except ValueError:
        return None
    na = norm(answer)
    out = {}
    for k in ("bs1", "bs2"):
        q = str(v.get(k + "_quote") or "")
        ok = v.get(k) is True and bool(norm(q)) and norm(q) in na
        out[k], out[k + "_quote"] = ok, (q if ok else "")
        if v.get(k) is True and not ok:
            out[k + "_rejected"] = q
    return out


def judge(answer):
    prompt = RUBRIC % answer
    votes, credits, tries = [], 0.0, 0
    while len(votes) < 3 and tries < 5:
        tries += 1
        text, c = call_model(prompt)
        credits += c
        v = parse_vote(text, answer)
        if v is None:
            continue
        votes.append(v)
        if len(votes) == 2 and all(votes[0][k] == votes[1][k] for k in ("bs1", "bs2")):
            break
    if len(votes) < 2:
        return {"error": "fewer than two usable votes", "credits": round(credits, 3)}
    res = {"votes": len(votes), "credits": round(credits, 3)}
    for k in ("bs1", "bs2"):
        yes = [v for v in votes if v[k]]
        res[k] = len(yes) * 2 > len(votes)
        res[k + "_quote"] = yes[0][k + "_quote"] if res[k] else ""
    rej = [v[k + "_rejected"] for v in votes for k in ("bs1", "bs2") if k + "_rejected" in v]
    if rej:
        res["rejected_quotes"] = rej
    return res


def wrap(question):
    return "Jeg har sett på oppgaven.\n\n" + question


def controls(path):
    with open(path, encoding="utf-8") as f:
        rows = [r for r in csv.DictReader(f, delimiter="\t")]
    jobs = int(os.environ.get("BS_JUDGE_JOBS", "4"))
    with ThreadPoolExecutor(jobs) as ex:
        results = list(ex.map(lambda r: judge(wrap(r["question"])), rows))
    agree, credits, errors = 0, 0.0, 0
    for i, (r, res) in enumerate(zip(rows, results), start=2):  # line 1 is the header
        credits += res["credits"]
        if "error" in res:
            errors += 1
            print(f"row {i}: judge error: {res['error']}: {r['question']}", file=sys.stderr)
            continue
        want = (r["expected_bs1"] == "1", r["expected_bs2"] == "1")
        got = (res["bs1"], res["bs2"])
        if want == got:
            agree += 1
        else:
            print(f"row {i}: want bs1={int(want[0])} bs2={int(want[1])}, judge bs1={int(got[0])} "
                  f"bs2={int(got[1])} ({r['source']}): {r['question']}", file=sys.stderr)
    acc = agree / len(rows) if rows else 0.0
    print(f"controls n={len(rows)} agree={agree} errors={errors} accuracy={acc:.3f} credits={credits:.2f}")
    if errors * 10 > len(rows):
        return 2
    return 0 if acc >= MIN_ACCURACY else 1


def main(argv):
    if len(argv) == 2 and argv[0] == "judge":
        with open(argv[1], encoding="utf-8") as f:
            res = judge(f.read())
        print(json.dumps(res, ensure_ascii=False))
        return 2 if "error" in res else 0
    if len(argv) == 2 and argv[0] == "controls":
        return controls(argv[1])
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
