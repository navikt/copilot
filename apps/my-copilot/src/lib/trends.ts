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
  if (!start) return { months: sorted, clamped: false, first };
  return { months: sorted.filter((m) => m >= start), clamped: !!first && first > start, first };
}

export interface FamilyShares {
  months: string[];
  /** Per family, the share of the month's net cost in whole percent, one value per month. */
  series: { family: ModelFamily; label: string; shares: number[] }[];
}

/** Net cost per model family as a share of each month's net cost. Months with no net cost are left out. */
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
  const kept = months.filter((m) => [...(net.get(m)?.values() ?? [])].reduce((a, b) => a + b, 0) > 0);
  const families = (Object.keys(FAMILY_LABELS) as ModelFamily[]).filter((f) =>
    kept.some((m) => (net.get(m)?.get(f) ?? 0) > 0)
  );
  return {
    months: kept,
    series: families.map((family) => ({
      family,
      label: FAMILY_LABELS[family],
      shares: kept.map((m) => {
        const byFamily = net.get(m)!;
        const total = [...byFamily.values()].reduce((a, b) => a + b, 0);
        return Math.round(((byFamily.get(family) ?? 0) / total) * 1000) / 10;
      }),
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
