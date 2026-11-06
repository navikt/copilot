import { formatNumber, formatShare } from "@/lib/format";

interface Row {
  label: string;
  value: number | null;
}

/**
 * One sentence about the largest row. The leader is found here, never taken from display order, because panels
 * can share another panel's order. Unknown (null) is not zero: all unknown is «ikke tilgjengelig», all zero is zero,
 * and with some unknown rows the total is only «blant radene med kjente tall». `listedOnly` is for top-N lists,
 * whose total covers the listed rows and not everything that was counted.
 */
export function topTakeaway(rows: Row[], what: string, listedOnly = false) {
  const known = rows.filter((r): r is { label: string; value: number } => r.value != null);
  if (known.length === 0) return `Tall for ${what} er ikke tilgjengelig.`;
  const total = known.reduce((s, r) => s + r.value, 0);
  if (total === 0) return `Alle observerte rader har 0 ${what}.`;
  const max = Math.max(...known.map((r) => r.value));
  const leaders = known.filter((r) => r.value === max).map((r) => r.label);
  const scope = listedOnly
    ? " blant de viste radene"
    : known.length < rows.length
      ? " blant radene med kjente tall"
      : "";
  return `${leaders.join(" og ")} har flest ${what}${scope}: ${formatNumber(max)} av ${formatNumber(total)}${listedOnly || known.length < rows.length ? " i radene" : ""}, ${formatShare(max / total)}.`;
}
