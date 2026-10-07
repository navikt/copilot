"""Batch 4 in GitHub list-price dollars: median per run, from the usage psv files.

Run: python3 docs/golden-baselines/2026-10-06-batch4/list-price.py
Each cell: "all input at input price / cache read and write at their own price".
"""
import collections, os, statistics as st

D = os.path.dirname(os.path.abspath(__file__))
# USD per 1M tokens: input, cached input, cache write, output. GitHub list price 2026-10-07,
# https://docs.github.com/en/copilot/reference/copilot-billing/models-and-pricing (≤ 272K tier).
P = {
    "gpt-6-sol": (2, 0.2, 2.5, 10),
    "gpt-6.1-sol": (2, 0.1, 2.5, 10),
    "claude-opus-5.5": (4, 0.2, 5, 20),
}
N = {"planning": "Planlegging", "coding": "Koding", "review": "Kodegjennomgang",
     "norsk": "Norsk", "research": "Research"}

for suite, name in N.items():
    row = [name]
    for arm, (pi, pc, pw, po) in P.items():
        runs = collections.defaultdict(lambda: [0] * 4)  # input, output, cache read, cache write
        for line in open(f"{D}/{suite}-{arm}-low-usage.psv"):
            if line.startswith("#") or not line.strip():
                continue
            c = line.split("|")
            assert int(c[7]) <= 272_000, "long-context tier not priced"
            for k, j in enumerate((7, 8, 9, 10)):
                runs[c[1]][k] += int(c[j])
        v = list(runs.values())
        assert all(x[0] >= x[2] + x[3] for x in v), "input_tokens must include cache tokens"
        flat = st.median((x[0] * pi + x[1] * po) / 1e6 for x in v)
        cached = st.median(((x[0] - x[2] - x[3]) * pi + x[2] * pc + x[3] * pw + x[1] * po) / 1e6 for x in v)
        row.append(f"${flat:.2f} / ${cached:.2f}")
    print("| " + " | ".join(row) + " |")
