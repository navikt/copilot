import { formatUSD } from "./format";
import type { MonthAnnotation } from "./trends";
import type { CostForecast } from "./types";

/** The day Nav went from premium requests to KI-kreditter. The only event that is not read from data. */
export const KI_CREDITS_FROM = "2026-06-01";

/** An event to mark in the cost charts: `text` is the short marker, `label` the full sentence. */
export interface CostEvent {
  date: string;
  text: string;
  label: string;
  /** The pooled credits run out; the forecast chart already says so in its summary. */
  runOut?: boolean;
}

const dayLong = (day: string) =>
  new Date(`${day}T00:00:00Z`).toLocaleDateString("nb-NO", {
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });

/** The switch to KI-kreditter, changes in the personal limit, and the day the pooled credits run out this month. */
export function costEvents(
  forecast: Pick<CostForecast, "limit_changes" | "allowance_run_out_date" | "allowance_used_up"> | null
): CostEvent[] {
  const events: CostEvent[] = [
    { date: KI_CREDITS_FROM, text: "KI-kreditter", label: `${dayLong(KI_CREDITS_FROM)}: overgang til KI-kreditter` },
  ];
  // The first limit came with the switch; only later changes are events of their own.
  for (const c of forecast?.limit_changes ?? []) {
    if (c.date <= KI_CREDITS_FROM) continue;
    events.push({
      date: c.date,
      text: `Grense ${c.usd}`,
      label: `${dayLong(c.date)}: forbruksgrensen per person ble ${formatUSD(c.usd)}`,
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
