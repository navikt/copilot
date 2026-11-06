import { formatNumber, formatShare } from "@/lib/format";

export const sumKnown = (values: (number | null)[]) => values.reduce<number>((s, v) => s + (v ?? 0), 0);

/** «X har flest Y: n av total, share.» Rows must be sorted, largest first; unknown (null) rows are left out of the total. */
export function topTakeaway(rows: { label: string; value: number | null }[], what: string) {
  const known = rows.filter((r) => r.value != null);
  const total = sumKnown(known.map((r) => r.value));
  const top = known[0];
  if (!top || total === 0) return `Ingen ${what} i perioden.`;
  return `${top.label} har flest ${what}: ${formatNumber(top.value!)} av ${formatNumber(total)}, ${formatShare(top.value! / total)}.`;
}
