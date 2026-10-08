"""Haiku 5.5 vs GPT-6 Luna Medium: checks, credits, request sizes and list price per arm.

Run: python3 docs/golden-baselines/2026-10-08-haiku-5-5/list-price.py
Same method as 2026-10-06-batch4/list-price.py (cache read and write priced separately),
but each request is priced in the tier its own input_tokens falls in.
"""
import collections, glob, os, statistics as st

D = os.path.dirname(os.path.abspath(__file__))
# USD per 1M tokens: input, cache read, cache write, output. GitHub list price, #1456 / model-pricing.ts.
# (tier limit, price at or under, price above)
P = {
    "claude-haiku-5.5": (100_000, (0.1, 0.01, 0.125, 0.5), (0.5, 0.05, 0.625, 2.5)),
    "gpt-6-luna": (272_000, (0.1, 0.01, 0.125, 0.5), (0.2, 0.02, 0.25, 0.75)),
}


def p95(xs):
    xs = sorted(xs)
    return xs[min(len(xs) - 1, round(0.95 * (len(xs) - 1)))]


def rows(path):
    with open(path) as f:
        return [l.rstrip("\n").split("|") for l in f if l.strip() and not l.startswith("#")]


print("| Arm | Bestått | Median credits | Maks input/forespørsel | p95 input | Andel > 100K | Median $ per kjøring |")
print("| --- | --- | --- | --- | --- | --- | --- |")
for txt in sorted(glob.glob(f"{D}/*.txt")):
    base = txt[:-4]
    usage = rows(base + "-usage.psv")
    res = [r for r in rows(base + "-results.psv") if not r[2].startswith("soft")]
    model = usage[0][5]
    assert {r[5] for r in usage} == {model}, f"{base}: other model in usage rows"
    limit, lo, hi = P[model]
    credits, dollars = collections.defaultdict(int), collections.defaultdict(float)
    sizes = []
    for r in usage:
        inp, out, cr, cw = (int(r[j]) for j in (7, 8, 9, 10))
        assert inp >= cr + cw, "input_tokens must include cache tokens"
        pi, pc, pw, po = hi if inp > limit else lo
        dollars[r[1]] += ((inp - cr - cw) * pi + cr * pc + cw * pw + out * po) / 1e6
        credits[r[1]] += int(r[12] or 0)
        sizes.append(inp)
    passed = sum(r[2] == "pass" for r in res)
    over = sum(s > 100_000 for s in sizes)
    print(f"| {os.path.basename(base)} | {passed}/{len(res)} | {st.median(credits.values()) / 1e9:.2f} "
          f"| {max(sizes):,} | {p95(sizes):,} | {over}/{len(sizes)} | ${st.median(dollars.values()):.4f} |")
