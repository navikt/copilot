import { formatNumber, formatPercent, formatUSD } from "./format";
import { FAMILY_LABELS, modelFamily, type ModelFamily } from "./model-family";
import { runOutDate } from "./month-insights";
import { monthName } from "./team-year";
import type { MonthAnnotation } from "./trends";
import type { ActiveUsersMonth, BillingModelDailyCost, BudgetDay, CostForecast } from "./types";

/** The day Nav went from premium requests to KI-kreditter. The only event that is not read from data. */
export const KI_CREDITS_FROM = "2026-06-01";

/** Window in days before and after a governance event (#1777). */
export const EVENT_WINDOW_DAYS = 14;

/** An event to mark in the cost charts: `text` is the short marker, `label` the full sentence. */
export interface CostEvent {
  date: string;
  text: string;
  label: string;
  /** The pooled credits run out; the forecast chart already says so in its summary. */
  runOut?: boolean;
  /** Personal limit before and after the change, in USD. */
  fromUsd?: number;
  toUsd?: number;
}

export interface ModelMixShift {
  family: ModelFamily;
  label: string;
  beforePct: number;
  afterPct: number;
  diffPp: number;
}

export interface EventEffect {
  event: CostEvent;
  /** One-sentence summary of the before/after effect (#1777). */
  summary: string;
  /** 14-day weekday-adjusted gross cost per billed user before and after, in USD, and percentage change. */
  costPerUser: {
    beforeUsd: number;
    afterUsd: number;
    changePct: number;
  } | null;
  /** Share of users at >= 90 % of their personal limit before and after. */
  atLimit90: {
    beforeDay: string;
    beforePct: number;
    afterDay: string;
    afterPct: number;
    minimum: boolean;
  } | null;
  /** Model family shares (14-day weekday-adjusted gross) before and after. */
  modelMix: ModelMixShift[];
  /** Day the pooled licence credits ran out in the month before/of the event vs. the month of/after. */
  runOut: {
    beforeMonth: string;
    beforeDate: string | null;
    afterMonth: string;
    afterDate: string | null;
  } | null;
}

export interface EventEffectInput {
  events: CostEvent[];
  daily: BillingModelDailyCost[];
  active: ActiveUsersMonth[];
  budgetDays?: BudgetDay[] | null;
  forecast?: Pick<CostForecast, "month" | "allowance_run_out_date" | "allowance_used_up"> | null;
}

const dayLong = (day: string) =>
  new Date(`${day}T00:00:00Z`).toLocaleDateString("nb-NO", {
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });

const dayShort = (day: string) =>
  new Date(`${day}T12:00:00Z`).toLocaleDateString("nb-NO", {
    day: "numeric",
    month: "short",
    timeZone: "UTC",
  });

export const dayMonth = (day: string) => `${Number(day.slice(8))}. ${monthName(day.slice(0, 7))}`;

export function addDays(day: string, delta: number): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + delta);
  return d.toISOString().slice(0, 10);
}

const prevMonth = (m: string) => {
  const [y, mo] = m.split("-").map(Number);
  return mo === 1 ? `${y - 1}-12` : `${y}-${String(mo - 1).padStart(2, "0")}`;
};

const nextMonth = (m: string) => {
  const [y, mo] = m.split("-").map(Number);
  return mo === 12 ? `${y + 1}-01` : `${y}-${String(mo + 1).padStart(2, "0")}`;
};

const isWeekend = (day: string) => {
  const w = new Date(`${day}T00:00:00Z`).getUTCDay();
  return w === 0 || w === 6;
};

/** The switch to KI-kreditter, changes in the personal limit, and the day the pooled credits run out this month. */
export function costEvents(
  forecast: Pick<CostForecast, "limit_changes" | "allowance_run_out_date" | "allowance_used_up"> | null
): CostEvent[] {
  const events: CostEvent[] = [
    { date: KI_CREDITS_FROM, text: "KI-kreditter", label: `${dayLong(KI_CREDITS_FROM)}: overgang til KI-kreditter` },
  ];
  // The first limit came with the switch; only later changes are events of their own.
  const changes = forecast?.limit_changes ?? [];
  for (let i = 0; i < changes.length; i++) {
    const c = changes[i];
    if (c.date <= KI_CREDITS_FROM) continue;
    events.push({
      date: c.date,
      text: `Grense ${c.usd}`,
      label: `${dayLong(c.date)}: forbruksgrensen per person ble ${formatUSD(c.usd)}`,
      fromUsd: changes[i - 1]?.usd,
      toUsd: c.usd,
    });
  }
  const runOut = forecast?.allowance_run_out_date;
  if (runOut) {
    events.push({
      date: runOut,
      text: "Kreditter brukt opp",
      runOut: true,
      label: forecast?.allowance_used_up
        ? `${dayLong(runOut)}: kredittene i lisensene ble brukt opp`
        : `${dayLong(runOut)}: kredittene i lisensene ventes brukt opp`,
    });
  }
  return events.sort((a, b) => a.date.localeCompare(b.date));
}

/** Markers for a chart whose categories are `keys`: months ("2026-06") or days ("2026-10-09"). */
export function eventAnnotations(events: CostEvent[], keys: string[]): MonthAnnotation[] {
  const byKey = new Map<string, MonthAnnotation>();
  for (const e of events) {
    const key = keys.find((k) => e.date.startsWith(k));
    if (!key) continue;
    const m = byKey.get(key) ?? { month: key, labels: [], numbers: [], dataBreak: false };
    m.labels.push(e.label);
    m.text = m.text ? `${m.text} · ${e.text}` : e.text;
    byKey.set(key, m);
  }
  return [...byKey.values()];
}

/** One sentence for the chart summary, naming the events the chart marks. */
export function eventSummary(markers: MonthAnnotation[]): string {
  const labels = markers.flatMap((m) => m.labels);
  return labels.length ? ` Markert i grafen: ${labels.join("; ")}.` : "";
}

/** Months (YYYY-MM) needed to compute 14-day before/after effects and run-out dates for all governance events. */
export function eventMonths(events: CostEvent[]): string[] {
  const out = new Set<string>();
  for (const e of events) {
    if (e.runOut || e.date <= KI_CREDITS_FROM) continue;
    out.add(addDays(e.date, -EVENT_WINDOW_DAYS).slice(0, 7));
    out.add(addDays(e.date, -1).slice(0, 7));
    out.add(e.date.slice(0, 7));
    out.add(addDays(e.date, EVENT_WINDOW_DAYS - 1).slice(0, 7));
    const m = e.date.slice(0, 7);
    out.add(e.date.endsWith("-01") ? prevMonth(m) : nextMonth(m));
  }
  return [...out].sort();
}

/**
 * Weekday-adjusted 14-day total (10 weekdays + 4 weekend days) from daily values.
 * Requires at least 5 weekdays and 2 weekend days in the window.
 */
function adjusted14DayTotal(days: string[], valueForDay: (day: string) => number): number | null {
  const wd = days.filter((d) => !isWeekend(d));
  const we = days.filter((d) => isWeekend(d));
  if (wd.length < 5 || we.length < 2) return null;
  const mean = (arr: string[]) => arr.reduce((s, d) => s + valueForDay(d), 0) / arr.length;
  return 10 * mean(wd) + 4 * mean(we);
}

/** Share of users in bands >= 90 % (band index >= 5). */
function shareAt90(d: BudgetDay): { pct: number; straddle: boolean } {
  const users90 = (d.bands ?? []).filter((b) => b.first >= 5).reduce((s, b) => s + b.users, 0);
  const straddle = (d.bands ?? []).some((b) => b.first < 5 && b.last >= 5 && b.users > 0);
  return { pct: d.users > 0 ? (users90 / d.users) * 100 : 0, straddle };
}

const pctText = (v: number) => formatPercent(v, v > 0 && Math.abs(v) < 10 ? 1 : 0);
const signedPct = (v: number) => `${v >= 0 ? "+" : "−"}${formatPercent(Math.abs(v))}`;

/**
 * Before/after comparison (14 days before vs. 14 days after, weekday-adjusted) for each governance event (#1777).
 * Aggregates only.
 */
export function eventEffects(input: EventEffectInput): EventEffect[] {
  const usersByMonth = new Map(input.active.map((a) => [a.month, a.active_users]));
  const fallbackUsers = input.active.at(-1)?.active_users ?? 0;
  const usersForDay = (day: string) => usersByMonth.get(day.slice(0, 7)) ?? fallbackUsers;

  const grossByDay = new Map<string, number>();
  const familyByDay = new Map<string, Map<ModelFamily, number>>();
  let hasNamedModels = false;
  for (const r of input.daily) {
    if (r.day < KI_CREDITS_FROM) continue;
    grossByDay.set(r.day, (grossByDay.get(r.day) ?? 0) + r.gross_amount);
    if (r.model && r.model !== "alle") {
      hasNamedModels = true;
      const fam = modelFamily(r.model, true);
      const byFam = familyByDay.get(r.day) ?? new Map<ModelFamily, number>();
      byFam.set(fam, (byFam.get(fam) ?? 0) + r.gross_amount);
      familyByDay.set(r.day, byFam);
    }
  }
  const allDays = [...grossByDay.keys()].sort();
  const budgetDays = (input.budgetDays ?? [])
    .filter((d) => d.bands && d.users > 0)
    .sort((a, b) => a.day.localeCompare(b.day));

  const monthRunOut = (m: string): string | null => {
    const rows = input.daily.filter((r) => r.day.startsWith(m));
    const fromRows = runOutDate(rows);
    if (fromRows) return fromRows;
    if (input.forecast?.month === m && input.forecast.allowance_run_out_date) {
      return input.forecast.allowance_run_out_date;
    }
    return null;
  };

  const out: EventEffect[] = [];
  for (const event of input.events) {
    if (event.runOut || event.date <= KI_CREDITS_FROM) continue;
    const beforeStart = addDays(event.date, -EVENT_WINDOW_DAYS);
    const beforeEnd = addDays(event.date, -1);
    const afterStart = event.date;
    const afterEnd = addDays(event.date, EVENT_WINDOW_DAYS - 1);

    const beforeDays = allDays.filter((d) => d >= beforeStart && d <= beforeEnd);
    const afterDays = allDays.filter((d) => d >= afterStart && d <= afterEnd);

    const beforePerUser = adjusted14DayTotal(beforeDays, (d) => {
      const u = usersForDay(d);
      return u > 0 ? (grossByDay.get(d) ?? 0) / u : 0;
    });
    const afterPerUser = adjusted14DayTotal(afterDays, (d) => {
      const u = usersForDay(d);
      return u > 0 ? (grossByDay.get(d) ?? 0) / u : 0;
    });

    const costPerUser =
      beforePerUser != null && afterPerUser != null && beforePerUser > 0
        ? {
            beforeUsd: beforePerUser,
            afterUsd: afterPerUser,
            changePct: ((afterPerUser - beforePerUser) / beforePerUser) * 100,
          }
        : null;

    // Budget share at >= 90 %: for a mid-month change, compare the last day before the change with the last day
    // of the window in the same month (before the 1st-of-month reset) if available, otherwise the last day in the window.
    // For a 1st-of-month change, compare the end of the 14-day after window with the same day-of-month in the previous month.
    let beforeB: BudgetDay | undefined;
    let afterB: BudgetDay | undefined;
    if (event.date.endsWith("-01")) {
      afterB = budgetDays.filter((d) => d.day >= afterStart && d.day <= afterEnd).at(-1);
      if (afterB) {
        const pm = prevMonth(event.date.slice(0, 7));
        const target = `${pm}-${afterB.day.slice(8)}`;
        beforeB =
          budgetDays.filter((d) => d.day.startsWith(pm) && d.day <= target).at(-1) ??
          budgetDays.filter((d) => d.day >= beforeStart && d.day <= beforeEnd).at(-1);
      }
    } else {
      beforeB = budgetDays.filter((d) => d.day >= beforeStart && d.day <= beforeEnd).at(-1);
      const eventMonth = event.date.slice(0, 7);
      afterB =
        budgetDays.filter((d) => d.day >= afterStart && d.day <= afterEnd && d.day.startsWith(eventMonth)).at(-1) ??
        budgetDays.filter((d) => d.day >= afterStart && d.day <= afterEnd).at(-1);
    }

    let atLimit90: EventEffect["atLimit90"] = null;
    if (beforeB && afterB) {
      const b90 = shareAt90(beforeB);
      const a90 = shareAt90(afterB);
      atLimit90 = {
        beforeDay: beforeB.day,
        beforePct: b90.pct,
        afterDay: afterB.day,
        afterPct: a90.pct,
        minimum: b90.straddle || a90.straddle,
      };
    }

    const modelMix: ModelMixShift[] = [];
    if (hasNamedModels && beforeDays.length >= 7 && afterDays.length >= 7) {
      const families = Object.keys(FAMILY_LABELS) as ModelFamily[];
      const beforeFam = new Map<ModelFamily, number>();
      const afterFam = new Map<ModelFamily, number>();
      for (const f of families) {
        const b = adjusted14DayTotal(beforeDays, (d) => familyByDay.get(d)?.get(f) ?? 0) ?? 0;
        const a = adjusted14DayTotal(afterDays, (d) => familyByDay.get(d)?.get(f) ?? 0) ?? 0;
        if (b > 0 || a > 0) {
          beforeFam.set(f, b);
          afterFam.set(f, a);
        }
      }
      const totalB = [...beforeFam.values()].reduce((s, v) => s + v, 0);
      const totalA = [...afterFam.values()].reduce((s, v) => s + v, 0);
      if (totalB > 0 && totalA > 0) {
        for (const f of beforeFam.keys()) {
          const beforePct = ((beforeFam.get(f) ?? 0) / totalB) * 100;
          const afterPct = ((afterFam.get(f) ?? 0) / totalA) * 100;
          if (beforePct >= 1 || afterPct >= 1) {
            modelMix.push({
              family: f,
              label: FAMILY_LABELS[f],
              beforePct,
              afterPct,
              diffPp: afterPct - beforePct,
            });
          }
        }
        modelMix.sort((a, b) => Math.abs(b.diffPp) - Math.abs(a.diffPp));
      }
    }

    const eventMonth = event.date.slice(0, 7);
    const beforeMonth = event.date.endsWith("-01") ? prevMonth(eventMonth) : eventMonth;
    const afterMonth = event.date.endsWith("-01") ? eventMonth : nextMonth(eventMonth);
    const beforeRunOut = monthRunOut(beforeMonth);
    const afterRunOut = monthRunOut(afterMonth);
    const runOut =
      beforeRunOut || afterRunOut
        ? { beforeMonth, beforeDate: beforeRunOut, afterMonth, afterDate: afterRunOut }
        : null;

    if (!costPerUser && !atLimit90 && !modelMix.length && !runOut) continue;

    const prefix =
      event.fromUsd != null && event.toUsd != null
        ? `Etter at grensen gikk fra ${formatNumber(event.fromUsd)} til ${formatUSD(event.toUsd)} ${dayShort(event.date)}`
        : event.toUsd != null
          ? `Etter at grensen ble ${formatUSD(event.toUsd)} ${dayShort(event.date)}`
          : `Etter ${event.text.toLowerCase()} ${dayShort(event.date)}`;

    const clauses: string[] = [];
    if (costPerUser) clauses.push(`kostnad per person ${signedPct(costPerUser.changePct)}`);
    if (atLimit90)
      clauses.push(`andel nær grensen fra ${pctText(atLimit90.beforePct)} til ${pctText(atLimit90.afterPct)}`);
    if (!clauses.length && runOut?.beforeDate && runOut?.afterDate) {
      clauses.push(`kredittene i lisensene gikk tom ${dayMonth(runOut.afterDate)} mot ${dayMonth(runOut.beforeDate)}`);
    }

    out.push({
      event,
      summary: `${prefix}: ${clauses.join(", ")}.`,
      costPerUser,
      atLimit90,
      modelMix,
      runOut,
    });
  }
  return out.sort((a, b) => b.event.date.localeCompare(a.event.date));
}
