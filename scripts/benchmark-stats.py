#!/usr/bin/env python3
"""Exact tests for small benchmark samples, stdlib only (no scipy on the bench machine).

  benchmark-stats.py fisher 5 5 0 5        # one-sided: is 5/5 higher than 0/5?
  benchmark-stats.py mw 1.1,1.2 3.0,3.1    # one-sided exact Mann–Whitney: is the first sample lower?
  benchmark-stats.py holm 0.004 0.03 0.2   # Holm-adjusted p-values, same order
  benchmark-stats.py --selftest

The decision rules these feed are in docs/golden-baselines/2026-11-modellvalg-plan.md.
"""

import sys
from itertools import combinations
from math import comb


def fisher_greater(a, na, b, nb):
    """P(X >= a) for the hypergeometric: a of na successes against b of nb."""
    k, n = a + b, na + nb
    return sum(comb(k, x) * comb(n - k, na - x) for x in range(a, min(k, na) + 1)) / comb(n, na)


def mann_whitney_less(xs, ys):
    """Exact P(U <= observed), U counting pairs where x > y (ties count 1/2). Small x is the alternative."""
    pooled, nx = list(xs) + list(ys), len(xs)
    if comb(len(pooled), nx) > 2_000_000:
        sys.exit("mann_whitney_less: too many splits for exact enumeration")

    def u(idx):
        rest = [pooled[i] for i in range(len(pooled)) if i not in idx]
        return sum((pooled[i] > y) + 0.5 * (pooled[i] == y) for i in idx for y in rest)

    observed = u(set(range(nx)))
    splits = [u(set(c)) for c in combinations(range(len(pooled)), nx)]
    return sum(s <= observed for s in splits) / len(splits)


def holm(ps):
    order = sorted(range(len(ps)), key=lambda i: ps[i])
    out, running = [0.0] * len(ps), 0.0
    for rank, i in enumerate(order):
        running = max(running, min(1.0, (len(ps) - rank) * ps[i]))
        out[i] = running
    return out


def selftest():
    close = lambda got, want: abs(got - want) < 1e-12

    # Not assert: python -O would skip the checks and still print «passed».
    def check(ok, *why):
        if not ok:
            raise AssertionError(why)

    # The p-values the plan quotes for 5 against 5.
    check(close(fisher_greater(5, 5, 0, 5), 1 / 252))
    check(close(fisher_greater(5, 5, 1, 5), 6 / 252))
    check(close(fisher_greater(4, 5, 0, 5), 6 / 252))
    check(close(fisher_greater(5, 5, 2, 5), 21 / 252))
    check(fisher_greater(3, 5, 3, 5) > 0.5)
    # U = 0..5 give cumulative 1, 2, 4, 7, 12, 19 of 252 splits.
    for u, want in enumerate((1, 2, 4, 7, 12, 19)):
        # The largest x sits above u of the y values.
        xs, ys = [1, 2, 3, 4, 5 + u], [v for v in range(5, 11) if v != 5 + u]
        got = mann_whitney_less(xs, ys)
        check(close(got, want / 252), u, got)
    check(close(mann_whitney_less([1, 1], [1, 1]), 1.0))  # all ties: no evidence
    # Holm over eight: full separations survive, a lone 2/252 does not.
    check(all(p < 0.05 for p in holm([1 / 252] * 8)))
    check(holm([2 / 252] + [0.5] * 7)[0] > 0.05)
    check(all(close(g, w) for g, w in zip(holm([0.01, 0.04, 0.03]), [0.03, 0.06, 0.06])))
    print("benchmark-stats selftest passed")


def main(argv):
    if argv[:1] == ["--selftest"]:
        selftest()
        return 0
    if argv[:1] == ["fisher"] and len(argv) == 5:
        return print(f"{fisher_greater(*map(int, argv[1:])):.4f}")
    if argv[:1] == ["mw"] and len(argv) == 3:
        xs, ys = ([float(v) for v in a.split(",")] for a in argv[1:])
        return print(f"{mann_whitney_less(xs, ys):.4f}")
    if argv[:1] == ["holm"] and len(argv) > 1:
        return print(" ".join(f"{p:.4f}" for p in holm([float(p) for p in argv[1:]])))
    sys.exit(__doc__)


if __name__ == "__main__":
    main(sys.argv[1:])
