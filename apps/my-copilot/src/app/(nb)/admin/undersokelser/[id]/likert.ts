import type { VersionResult } from "@/lib/survey";

export type Segment = { value: number; count: number; share: number };

/** A Likert version as a diverging bar, with the mean. */
export function likert(v: VersionResult, min: number, max: number) {
  const n = v.n ?? 0;
  const segments: Segment[] = [];
  let sum = 0;
  for (let value = min; value <= max; value++) {
    const count = v.options?.find((x) => Number(x.value) === value)?.count ?? 0;
    sum += value * count;
    segments.push({ value, count, share: n ? count / n : 0 });
  }
  return { segments, mean: n ? sum / n : null };
}
