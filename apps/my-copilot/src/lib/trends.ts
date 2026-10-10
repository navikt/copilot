import type { BillingModelBreakdown, SegmentChart } from "./types";
import { FAMILY_LABELS, modelFamily, type ModelFamily } from "./model-family";
import { previousMonth } from "./month-utils";

/** The period select on /innsikt/trender. The value is kept in ?periode=. */
export const PERIODS = [
  { value: "3", label: "Siste 3 måneder", months: 3 },
  { value: "6", label: "Siste 6 måneder", months: 6 },
  { value: "12", label: "Siste 12 måneder", months: 12 },
  { value: "alt", label: "Alt", months: null },
] as const;

export type Period = (typeof PERIODS)[number];

export function parsePeriod(value: string | undefined): Period {
  return PERIODS.find((p) => p.value === value) ?? PERIODS[2];
}

/** First month (YYYY-MM) the period covers, counting the current month. null means everything. */
export function periodStart(period: Period, current: string): string | null {
  if (period.months === null) return null;
  let month = current;
  for (let i = 1; i < period.months; i++) month = previousMonth(month);
  return month;
}

/**
 * The months a chart shows: the period, clamped to where the data starts.
 * `clamped` is true when the period reaches back before the data, so the chart can say so.
 */
export function visibleMonths(
  months: string[],
  start: string | null
): { months: string[]; clamped: boolean; first: string | undefined } {
  const sorted = [...new Set(months)].sort();
  const first = sorted[0];
  // Every calendar month from the first to the last, so a month without data shows as a gap.
  const all: string[] = [];
  for (let m = first; m && m <= sorted[sorted.length - 1]; m = nextMonth(m)) all.push(m);
  if (!start) return { months: all, clamped: false, first };
  return { months: all.filter((m) => m >= start), clamped: !!first && first > start, first };
}

export interface FamilyShares {
  months: string[];
  /** Per family, the share of the month's net cost in percent, one value per month. null: no net cost that month. */
  series: { family: ModelFamily; label: string; shares: (number | null)[] }[];
}

function nextMonth(month: string): string {
  const [y, m] = month.split("-").map(Number);
  return m === 12 ? `${y + 1}-01` : `${y}-${String(m + 1).padStart(2, "0")}`;
}

/** Net cost per model family as a share of each month's net cost. A month with no net cost is a gap. */
export function familyShares(rows: BillingModelBreakdown[], months: string[]): FamilyShares {
  const net = new Map<string, Map<ModelFamily, number>>();
  for (const row of rows) {
    const month = row.year_month.slice(0, 7);
    if (!months.includes(month)) continue;
    const byFamily = net.get(month) ?? new Map<ModelFamily, number>();
    const family = modelFamily(row.model);
    byFamily.set(family, (byFamily.get(family) ?? 0) + row.net_amount);
    net.set(month, byFamily);
  }
  const total = (m: string) => [...(net.get(m)?.values() ?? [])].reduce((a, b) => a + b, 0);
  const kept = months;
  const families = (Object.keys(FAMILY_LABELS) as ModelFamily[]).filter((f) =>
    kept.some((m) => (net.get(m)?.get(f) ?? 0) > 0)
  );
  return {
    months: kept,
    series: families.map((family) => ({
      family,
      label: FAMILY_LABELS[family],
      shares: kept.map((m) =>
        total(m) > 0 ? Math.round(((net.get(m)!.get(family) ?? 0) / total(m)) * 1000) / 10 : null
      ),
    })),
  };
}

export interface MonthAnnotation {
  month: string;
  labels: string[];
  /** 1-based number of each event in the shared «Hendelser» list, same order as labels. */
  numbers: number[];
  dataBreak: boolean;
}

/**
 * Annotations grouped per month, only for months the chart shows, numbered by their place in the full list
 * (the page's «Hendelser»). Without `all`, only data breaks (and model events when `models` is set) become markers.
 */
export function annotationsFor(
  annotations: { date: string; label: string; dataBreak?: boolean; model?: boolean }[],
  months: string[],
  all = true,
  /** Also mark model events when `all` is off; set on the model chart. */
  models = false
): MonthAnnotation[] {
  const out = new Map<string, MonthAnnotation>();
  annotations.forEach((a, i) => {
    const month = a.date.slice(0, 7);
    if (!months.includes(month) || (!all && !a.dataBreak && !(models && a.model))) return;
    const entry = out.get(month) ?? { month, labels: [], numbers: [], dataBreak: false };
    entry.labels.push(a.label);
    entry.numbers.push(i + 1);
    entry.dataBreak ||= !!a.dataBreak;
    out.set(month, entry);
  });
  return [...out.values()];
}

/** ?hendelser=alle shows every event as a marker in the charts; otherwise only data breaks, plus model events in the model chart. */
export function showAllEvents(value: string | string[] | undefined): boolean {
  return value === "alle";
}

/** «2026-06» → «jun. 2026». */
export function monthLabel(month: string): string {
  return new Date(`${month}-01T00:00:00Z`).toLocaleDateString("nb-NO", {
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  });
}

/** One series of shares in percent, one value per month. null: no value that month. */
export interface ShareSeries {
  label: string;
  shares: (number | null)[];
}

/**
 * Picks the server's display-ready shares for the months a chart shows. A month the server did not return
 * is null in every band.
 */
export function chartShares(chart: SegmentChart, months: string[]): ShareSeries[] {
  const at = months.map((m) => chart.months.indexOf(m));
  const pick = (shares: (number | null)[]) => at.map((i) => (i < 0 ? null : shares[i]));
  return chart.bands.map((b) => ({ label: b.label, shares: pick(b.shares) }));
}

/** Net up minus down from the server, for the months a chart shows. */
export function chartNet(chart: SegmentChart, months: string[]): (number | null)[] {
  return months.map((m) => {
    const i = chart.months.indexOf(m);
    return i < 0 ? null : (chart.net?.[i] ?? null);
  });
}

/** Change in percentage points from the first to the last month with a value. null with fewer than two. */
export function shareChange(series: ShareSeries): number | null {
  const values = series.shares.filter((v): v is number => v !== null);
  return values.length < 2 ? null : Math.round(values[values.length - 1] - values[0]);
}

/** «+8 pp», «−3 pp», «±0 pp». */
export function formatPp(pp: number): string {
  return `${pp > 0 ? "+" : pp < 0 ? "−" : "±"}${Math.abs(pp)} pp`;
}
