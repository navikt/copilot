import type { VersionResult } from "@/lib/survey";

export type Segment = { value: number; count: number; share: number };

const isStep = (value: string) => value.trim() !== "" && !Number.isNaN(Number(value));

/**
 * A Likert version as a diverging bar, with the mean. Shares are of every answer, so steps and
 * unsure («Vet ikke», any non-numeric value) sum to 100 %; n and mean count the steps only.
 */
export function likert(v: VersionResult, min: number, max: number) {
  const total = v.n ?? 0;
  const segments: Segment[] = [];
  let sum = 0;
  let n = 0;
  let hidden = false; // legacy payload: an explicit null count makes the mean unknowable
  for (let value = min; value <= max; value++) {
    const o = v.options?.find((x) => Number(x.value) === value);
    if (o && o.count == null) hidden = true;
    const count = o?.count ?? 0;
    sum += value * count;
    n += count;
    segments.push({ value, count, share: total ? count / total : 0 });
  }
  const unsureCount = (v.options ?? []).filter((o) => !isStep(o.value)).reduce((a, o) => a + (o.count ?? 0), 0);
  const unsure = { count: unsureCount, share: total ? unsureCount / total : 0 };
  return { segments, unsure, n, mean: n && !hidden ? sum / n : null };
}

/** The unsure label in these versions' answers, if anyone gave one. */
export function unsureLabel(versions: VersionResult[]): string | undefined {
  return versions.flatMap((v) => v.options ?? []).find((o) => !isStep(o.value) && (o.count ?? 0) > 0)?.value;
}

/**
 * n as shown: for a scale or matrix with an unsure label, answers on the scale, then «· k vet ikke»;
 * any other question, n as is. "–" when hidden.
 */
export function nText(v: VersionResult, fmt: (x: number) => string, unsure?: string): string {
  if (v.n === null) return "–";
  const k = unsure ? (v.options?.find((o) => o.value === unsure)?.count ?? 0) : 0;
  return k ? `${fmt(v.n - k)} · ${fmt(k)} vet ikke` : fmt(v.n);
}
