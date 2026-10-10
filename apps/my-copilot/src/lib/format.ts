/**
 * Format a number with Norwegian locale formatting
 * Uses space as thousands separator
 * @param value - The number to format
 * @returns Formatted string with space as thousands separator
 */
export function formatNumber(value: number): string {
  return new Intl.NumberFormat("nb-NO", { maximumFractionDigits: 0 }).format(value);
}

/** Percent in nb-NO style: formatPercent(12) → "12 %", formatPercent(12.34, 1) → "12,3 %", with a non-breaking space. */
export function formatPercent(value: number, decimals = 0): string {
  const n = new Intl.NumberFormat("nb-NO", { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
  return `${n.format(value)} %`;
}

/** A 0–1 share as percent: formatShare(0.123, 1) → "12,3 %". */
export const formatShare = (share: number, decimals = 0) => formatPercent(share * 100, decimals);

/** Whole US dollars: formatUSD(1234.4) → "1 234 USD", with non-breaking spaces. */
export function formatUSD(value: number): string {
  return `${formatNumber(value)} USD`;
}

/**
 * Format a duration in minutes to a human-readable Norwegian string.
 * Returns "–" for null, zero or negative durations (e.g. a nullable median
 * with no qualifying pull requests).
 *
 * IMPORTANT: Rounds total minutes FIRST, then splits into hours/minutes.
 * The naive approach (Math.round on the remainder) caused "1t 60m" when
 * minutes % 60 was between 59.5 and 60 (e.g. 119.7 → floor(1.99)=1h, round(59.7)=60m).
 * @param minutes - Duration in minutes, or null
 * @returns Formatted string, e.g. "3t 12m", "45 min", "2d 4t" or "–"
 */
export function formatMinutes(minutes: number | null): string {
  if (minutes == null || minutes <= 0) return "–";
  if (minutes < 60) return `${Math.round(minutes)} min`;
  const totalMinutes = Math.round(minutes);
  const hours = Math.floor(totalMinutes / 60);
  const mins = totalMinutes % 60;
  if (hours < 24) return mins > 0 ? `${hours}t ${mins}m` : `${hours}t`;
  const days = Math.floor(hours / 24);
  const remainingHours = hours % 24;
  return remainingHours > 0 ? `${days}d ${remainingHours}t` : `${days}d`;
}

/**
 * "2026-09-26" → "26. september 2026", always in Oslo time. A bare date is read as noon UTC so it never moves a day.
 * A timestamp keeps its time, also BigQuery's "2026-10-03 04:12:33.1+00" form.
 */
export function formatDate(dateStr: string, locale: "nb-NO" | "en-GB" = "nb-NO"): string {
  const iso =
    dateStr.length === 10 ? `${dateStr}T12:00:00Z` : dateStr.replace(" ", "T").replace(/([+-]\d\d)$/, "$1:00");
  return new Date(iso).toLocaleDateString(locale, {
    timeZone: "Europe/Oslo",
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}

/**
 * Compute the ISO 8601 week label for a date string ("YYYY-MM-DD"), matching
 * BigQuery's FORMAT_DATE('%G-W%V', day) used server-side for weekly trends
 * (e.g. copilot-api's GetUserWeeklyTrends). Must stay in sync with that format
 * so client-side aggregation (e.g. summing daily credits into weeks) lines up
 * with the week labels returned by the backend.
 * @param dateStr - Date in "YYYY-MM-DD" format
 * @returns ISO week label, e.g. "2026-W01"
 */
export function isoWeekLabel(dateStr: string): string {
  const date = new Date(`${dateStr}T00:00:00Z`);
  const target = new Date(date.valueOf());
  const dayNr = (date.getUTCDay() + 6) % 7; // Monday = 0 .. Sunday = 6
  target.setUTCDate(target.getUTCDate() - dayNr + 3); // Thursday of the same ISO week
  const firstThursday = new Date(Date.UTC(target.getUTCFullYear(), 0, 4));
  const firstThursdayDayNr = (firstThursday.getUTCDay() + 6) % 7;
  firstThursday.setUTCDate(firstThursday.getUTCDate() - firstThursdayDayNr + 3);
  const week = 1 + Math.round((target.getTime() - firstThursday.getTime()) / (7 * 24 * 3600 * 1000));
  return `${target.getUTCFullYear()}-W${String(week).padStart(2, "0")}`;
}
