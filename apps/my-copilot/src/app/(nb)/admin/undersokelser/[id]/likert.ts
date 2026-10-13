import type { VersionResult } from "@/lib/survey";

export type Segment = { value: number; count: number; share: number };

/** A Likert version as a diverging bar, with the mean. */
export function likert(v: VersionResult, min: number, max: number) {
  const n = v.n ?? 0;
  const segments: Segment[] = [];
  let sum = 0;
  let hidden = false; // legacy payload: an explicit null count makes the mean unknowable
  for (let value = min; value <= max; value++) {
    const o = v.options?.find((x) => Number(x.value) === value);
    if (o && o.count == null) hidden = true;
    const count = o?.count ?? 0;
    sum += value * count;
    segments.push({ value, count, share: n ? count / n : 0 });
  }
  return { segments, mean: n && !hidden ? sum / n : null };
}
