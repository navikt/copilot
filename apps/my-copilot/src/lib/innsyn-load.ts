import {
  getAdminActiveUsers,
  getAdminMonthlyOverview,
  getAdminVolume,
  getBillingModelBreakdown,
} from "./cached-bigquery";
import { buildInnsyn, type InnsynAnswer } from "./innsyn";

/** The same fetches as /admin/kostnad. Admin only: copilot-api answers 404 to everyone else. */
export async function loadInnsyn(year: number, token: string): Promise<InnsynAnswer> {
  const now = new Date();
  const monthsBack = (now.getUTCFullYear() - year) * 12 + now.getUTCMonth() + 1;
  const [admin, active, { breakdown, error }, volume] = await Promise.all([
    getAdminMonthlyOverview(year, token),
    getAdminActiveUsers(token),
    getBillingModelBreakdown(token, monthsBack),
    getAdminVolume(year, token),
  ]);
  if (error) throw new Error(`model breakdown: ${error}`);
  return buildInnsyn({ year, admin, active, breakdown, volume, today: now.toISOString().slice(0, 10) });
}

/** Years with data: 2026 up to this year. */
export function parseInnsynYear(value: string | null | undefined): number | null {
  const thisYear = new Date().getUTCFullYear();
  const year = Number(value || thisYear);
  return Number.isInteger(year) && year >= 2026 && year <= thisYear ? year : null;
}
