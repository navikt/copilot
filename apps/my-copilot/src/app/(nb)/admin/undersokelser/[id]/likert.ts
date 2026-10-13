import type { VersionResult } from "@/lib/survey";

export type Segment = { value: number; count: number | null; share: number };

/**
 * A Likert version as a diverging bar. Hidden cells are merged into one segment of
 * n minus the shown counts, which the shown numbers already give away; they are
 * never drawn at their own width. The mean is only given when nothing is hidden.
 */
export function likert(v: VersionResult, min: number, max: number) {
  const n = v.n ?? 0;
  const segments: Segment[] = [];
  let shown = 0;
  let sum = 0;
  let anyHidden = false;
  for (let value = min; value <= max; value++) {
    const o = v.options?.find((x) => Number(x.value) === value);
    if (o && (o.suppressed || o.count === null)) {
      anyHidden = true;
      segments.push({ value, count: null, share: 0 });
      continue;
    }
    const count = o?.count ?? 0;
    shown += count;
    sum += value * count;
    segments.push({ value, count, share: n ? count / n : 0 });
  }
  const hidden = anyHidden ? { count: n - shown, share: n ? (n - shown) / n : 0 } : null;
  return { segments, hidden, mean: anyHidden || !n ? null : sum / n };
}
