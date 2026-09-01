#!/usr/bin/env python3
"""Ensidig ikke-underlegenhetstest for test 3 i golden-suiten.

Newcombe hybrid score-intervall for differansen mellom to andeler, ensidig 95 %
(z = 1,6449), mot en margin paa delta = 0,10. Tallene som mates inn staar i
tabellen i docs/nav-pilot-benchmark-og-beslutninger-2026-08.md, seksjon 1, og er
talt opp under selve kjoeringen. De er ikke gjenskapbare fra baseline-filene
alene; se forbeholdet i samme seksjon.

Kjoer: python3 scripts/ikke-underlegenhet.py
"""
import math

Z = 1.6449  # ensidig 95 %
DELTA = 0.10

REFERANSE = ("Claude Sonnet 4.6", 48, 50)  # bestaatt, kjoeringer
KANDIDATER = [("GPT-5.6 Sol", 49, 50), ("GPT-5.6 Luna", 49, 50), ("GPT-5.6 Terra", 40, 45)]


def wilson(x, n, z=Z):
    p = x / n
    nevner = 1 + z * z / n
    senter = p + z * z / (2 * n)
    halv = z * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n))
    return (senter - halv) / nevner, (senter + halv) / nevner


def newcombe(x1, n1, x2, n2, z=Z):
    """Nedre og oevre grense for p1 - p2 (kandidat minus referanse)."""
    p1, p2 = x1 / n1, x2 / n2
    l1, u1 = wilson(x1, n1, z)
    l2, u2 = wilson(x2, n2, z)
    d = p1 - p2
    return d, d - math.sqrt((p1 - l1) ** 2 + (u2 - p2) ** 2), d + math.sqrt((u1 - p1) ** 2 + (p2 - l2) ** 2)


def main():
    navn_ref, x_ref, n_ref = REFERANSE
    print(f"Referanse: {navn_ref} {x_ref}/{n_ref} = {x_ref / n_ref:.3f}\n")
    for navn, x, n in KANDIDATER:
        d, lo, hi = newcombe(x, n, x_ref, n_ref)
        ok = lo > -DELTA
        print(
            f"{navn:16s} {x}/{n} = {x / n:.3f}  diff {d * 100:+6.2f} pp"
            f"  [{lo * 100:+6.2f}, {hi * 100:+6.2f}] pp"
            f"  ikke-underlegen ved delta={DELTA:.2f}: {'ja' if ok else 'nei'}"
        )


if __name__ == "__main__":
    main()
    # Selvsjekk: tallene som staar i benchmark-dokumentet.
    assert [round(newcombe(x, n, 48, 50)[1] * 100, 1) for _, x, n in KANDIDATER] == [-5.0, -5.0, -17.5]
