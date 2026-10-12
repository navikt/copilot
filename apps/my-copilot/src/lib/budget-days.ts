import { formatPercent } from "./format";
import type { BudgetDay } from "./types";

/** Share in percent of the day's users in the lowest band (0–10 %, or the band it was merged into). */
export function lowestShare(d: BudgetDay): { label: string; pct: number } | null {
  const b = d.bands?.find((x) => x.first === 0);
  return b && d.users ? { label: b.label, pct: (b.users / d.users) * 100 } : null;
}

const short = (day: string) =>
  new Date(`${day}T12:00:00Z`).toLocaleDateString("nb-NO", { day: "numeric", month: "short", timeZone: "UTC" });

/**
 * «Per 10. okt.: 63 % under 10 % (sep.: 58 %)»: the last day of `month` with bands, against the same day
 * of the month before. The comparison is left out when that day has no bands or a different lowest band.
 */
export function budgetComparison(days: BudgetDay[], month: string): string | null {
  const last = days.filter((d) => d.day.startsWith(month) && d.bands).at(-1);
  const now = last && lowestShare(last);
  if (!last || !now) return null;
  const [y, m] = month.split("-").map(Number);
  const prevMonth = new Date(Date.UTC(y, m - 2, 1)).toISOString().slice(0, 7);
  const prev = days.find((d) => d.day === `${prevMonth}-${last.day.slice(8)}`);
  const before = prev && lowestShare(prev);
  const what = now.label.startsWith("0–10") ? "under 10 %" : `på ${now.label}`;
  const cmp =
    before && before.label === now.label
      ? ` (${new Date(Date.UTC(y, m - 2, 1)).toLocaleDateString("nb-NO", { month: "short", timeZone: "UTC" })}: ${formatPercent(before.pct)})`
      : "";
  return `Per ${short(last.day)}: ${formatPercent(now.pct)} ${what}${cmp}`;
}
