/**
 * Format a number with Norwegian locale formatting
 * Uses space as thousands separator
 * @param value - The number to format
 * @returns Formatted string with space as thousands separator
 */
export function formatNumber(value: number): string {
  return new Intl.NumberFormat("nb-NO", { maximumFractionDigits: 0 }).format(value);
}

/**
 * Format a percentage value
 * @param value - The percentage value (e.g., 25 for 25%)
 * @returns Formatted string with % symbol
 */
export function formatPercentage(value: number): string {
  return `${value}%`;
}

export function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString("nb-NO", {
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
