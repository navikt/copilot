import type { BillingModelBreakdown } from "./types";
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
  dataBreak: boolean;
}

/** Annotations grouped per month, only for months the chart shows. */
export function annotationsFor(
  annotations: { date: string; label: string; dataBreak?: boolean }[],
  months: string[]
): MonthAnnotation[] {
  const out = new Map<string, MonthAnnotation>();
  for (const a of annotations) {
    const month = a.date.slice(0, 7);
    if (!months.includes(month)) continue;
    const entry = out.get(month) ?? { month, labels: [], dataBreak: false };
    entry.labels.push(a.label);
    entry.dataBreak ||= !!a.dataBreak;
    out.set(month, entry);
  }
  return [...out.values()];
}

/** «2026-06» → «jun. 2026». */
export function monthLabel(month: string): string {
  return new Date(`${month}-01T00:00:00Z`).toLocaleDateString("nb-NO", {
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  });
}

/** One series of shares in percent, one value per month. null: hidden (under five) or no data. */
export interface ShareSeries {
  label: string;
  shares: (number | null)[];
}

type Counts = { month: string } & Record<string, number | null | string>;

/**
 * Shares in percent (one decimal) of each key per month. The denominator is `totalKey` when given,
 * otherwise the sum of the visible keys. A hidden count stays null, never 0.
 */
export function segmentShares(
  rows: Counts[],
  months: string[],
  keys: { key: string; label: string }[],
  totalKey?: string
): ShareSeries[] {
  const byMonth = new Map(rows.map((r) => [r.month, r]));
  const total = (r: Counts) =>
    totalKey ? (r[totalKey] as number | null) : keys.reduce((sum, k) => sum + ((r[k.key] as number | null) ?? 0), 0);
  return keys.map(({ key, label }) => ({
    label,
    shares: months.map((m) => {
      const r = byMonth.get(m);
      const n = r?.[key] as number | null | undefined;
      const t = r ? total(r) : null;
      return n == null || !t ? null : Math.round((n / t) * 1000) / 10;
    }),
  }));
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
