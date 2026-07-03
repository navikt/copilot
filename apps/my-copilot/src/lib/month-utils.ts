/**
 * Shared month-boundary utilities for the statistikk page and charts.
 *
 * Ensures a single, tested definition of:
 * - "current month" (YYYY-MM)
 * - "previous month" (safe against day-31 overflow)
 * - whether a month's data is "complete" (has all expected days)
 */

/**
 * Returns the current calendar month as YYYY-MM (UTC).
 */
export function currentMonthUTC(): string {
  const d = new Date();
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

/**
 * Returns the previous calendar month relative to `month` (YYYY-MM).
 * Safe against day-31 overflow: sets day to 1 before subtracting.
 */
export function previousMonth(month: string): string {
  const d = new Date(`${month}-01T00:00:00Z`);
  d.setUTCDate(1); // anchor to day 1 to avoid overflow
  d.setUTCMonth(d.getUTCMonth() - 1);
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

/**
 * Returns the number of days in a given month (YYYY-MM).
 */
export function daysInCalendarMonth(month: string): number {
  const [year, m] = month.split("-").map(Number);
  // Day 0 of the next month = last day of the target month
  return new Date(Date.UTC(year, m, 0)).getUTCDate();
}

/**
 * Determines if a month's data is complete.
 * A month is complete if:
 * - it is NOT the current calendar month, OR
 * - it has data for all calendar days in that month
 */
export function isMonthComplete(month: string, daysOfData: number, currentMonth: string): boolean {
  if (month !== currentMonth) return true;
  return daysOfData >= daysInCalendarMonth(month);
}

/**
 * From a sorted array of monthly data, select the latest complete month
 * and the one before it (for MoM comparison).
 */
export function selectCompleteMonths<T extends { month: string; days_in_month: number }>(
  data: T[],
  current: string = currentMonthUTC()
): { completeMonths: T[]; currentMonth: T | null; latestComplete: T | null; prevComplete: T | null } {
  const completeMonths = data.filter((m) => isMonthComplete(m.month, m.days_in_month, current));
  const currentMonth =
    data.find((m) => m.month === current && !isMonthComplete(m.month, m.days_in_month, current)) ?? null;
  const latestComplete = completeMonths.length > 0 ? completeMonths[completeMonths.length - 1] : null;
  const prevComplete = completeMonths.length > 1 ? completeMonths[completeMonths.length - 2] : null;
  return { completeMonths, currentMonth, latestComplete, prevComplete };
}
