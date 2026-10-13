import {
  getAdminActiveUsers,
  getAdminMonthlyOverview,
  getAdminVolume,
  getBillingModelBreakdown,
} from "./cached-bigquery";
import { buildRapport, type Rapport } from "./rapport";

/** The same fetches as /admin/kostnad. Admin only: copilot-api answers 404 to everyone else. */
export async function loadRapport(year: number, token: string): Promise<Rapport> {
  const now = new Date();
  // ponytail: the model breakdown caps at 36 months, so reports for years before that lose «Modeller»; add a year filter to the endpoint then.
  const monthsBack = Math.min((now.getUTCFullYear() - year) * 12 + now.getUTCMonth() + 1, 36);
  const [admin, active, { breakdown, error }, volume] = await Promise.all([
    getAdminMonthlyOverview(year, token),
    getAdminActiveUsers(token),
    getBillingModelBreakdown(token, monthsBack),
    getAdminVolume(year, token),
  ]);
  if (error) throw new Error(`model breakdown: ${error}`);
  return buildRapport({ year, admin, active, breakdown, volume, today: now.toISOString().slice(0, 10) });
}

/** Years with data: 2026 up to this year. */
export function parseRapportYear(value: string | null | undefined): number | null {
  const thisYear = new Date().getUTCFullYear();
  const year = Number(value || thisYear);
  return Number.isInteger(year) && year >= 2026 && year <= thisYear ? year : null;
}
