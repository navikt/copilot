// «Månedsinnsikt» on the admin cost page: a few statements about this month against last
// month, each with a threshold, a link to the chart and the lever to pull (owner decision
// 13 October 2026). Pure functions over data the page already fetches. Amounts are internal.
import { formatDate, formatNumber, formatPercent, formatUSD } from "./format";
import { FAMILY_LABELS, modelFamily, type ModelFamily } from "./model-family";
import { monthName } from "./team-year";
import type {
  ActiveUsersMonth,
  AdminMonth,
  BillingModelBreakdown,
  BillingModelDailyCost,
  BillingMonthlyTrend,
  BudgetDay,
  CostForecast,
  HighConsumerMonth,
} from "./types";

/** One finding from copilot-api's GET /api/v1/admin/billing/anomalies. */
export interface Anomaly {
  rule: "A1" | "A3" | "A4" | "A7";
  date: string;
  value: number;
  threshold: number;
  model?: string;
}

/** Distinct users of a model family in one month, from GET /api/v1/admin/billing/model-users. */
export interface ModelFamilyUsers {
  month: string;
  family: ModelFamily;
  users: number;
}

/** M2: the share of the personal limit that counts as close to it, in percent. */
export const LIMIT_PRESSURE_PCT = 90;
/** M2: the rise in that share since the same day last month, in percentage points, before we show it. */
export const LIMIT_PRESSURE_RISE_PP = 1;
/** M3: the change in billed cost between the two last closed months, in percent, before we explain it. */
export const COST_CHANGE_MIN_PCT = 15;
/** M4: the rise in a family's share of billed cost, in percentage points. */
export const MODEL_SHARE_RISE_PP = 10;
/** M4: families with fewer users have no cost per user that means anything. */
export const MODEL_MIN_USERS = 5;
/** M5: the change in the top 5 %'s share of billed cost, in percentage points. */
export const TOP5_CHANGE_PP = 5;
/** M6: the share of billed cost on people with no team, in percent, or its rise in points. */
export const NO_TEAM_SHARE_PCT = 5;
export const NO_TEAM_RISE_PP = 2;

/** Lower edge of each budget band index, as in copilot-api's budgetEdges. */
const BAND_EDGES = [0, 10, 25, 50, 75, 90, 100];

export interface MonthInsight {
  id: "M1" | "M2" | "M3" | "M4" | "M5" | "M6" | "M7";
  text: string;
  /** Shown in a fold under the statement. */
  details?: string[];
  href: string;
  /** «Hva kan vi gjøre?» */
  action: string;
}

export interface MonthInsightData {
  forecast: CostForecast | null;
  /** Last month's billing per model and day, for its run-out date. */
  prevDaily: BillingModelDailyCost[];
  anomalies: Anomaly[] | null;
  budgetDays: BudgetDay[] | null;
  trend: BillingMonthlyTrend[];
  active: ActiveUsersMonth[];
  breakdown: BillingModelBreakdown[];
  familyUsers: ModelFamilyUsers[] | null;
  high: HighConsumerMonth[] | null;
  months: AdminMonth[];
}

const pct = (v: number) => formatPercent(v, v && Math.abs(v) < 10 ? 1 : 0);
const pp = (v: number) =>
  `${v > 0 ? "+" : v < 0 ? "−" : ""}${Math.abs(v).toLocaleString("nb-NO", { maximumFractionDigits: 1 })} prosentpoeng`;
const change = (ratio: number) => `${ratio >= 1 ? "+" : "−"}${formatPercent(Math.abs(ratio - 1) * 100)}`;
const day = (d: string) => `${Number(d.slice(8))}. ${monthName(d.slice(0, 7))}`;
const prevMonth = (m: string) => {
  const [y, mo] = m.split("-").map(Number);
  return mo === 1 ? `${y - 1}-12` : `${y}-${String(mo - 1).padStart(2, "0")}`;
};
const sameDay = (d: string, month: string) => {
  const last = new Date(Date.UTC(Number(month.slice(0, 4)), Number(month.slice(5)), 0)).getUTCDate();
  return `${month}-${String(Math.min(Number(d.slice(8)), last)).padStart(2, "0")}`;
};

/** The first day billed net is more than half of gross: the pooled credits are used up. Same rule as copilot-api's poolNet. */
export function runOutDate(rows: BillingModelDailyCost[]): string | null {
  const byDay = new Map<string, { gross: number; net: number }>();
  for (const r of rows) {
    const d = byDay.get(r.day) ?? { gross: 0, net: 0 };
    d.gross += r.gross_amount;
    d.net += r.net_amount;
    byDay.set(r.day, d);
  }
  return [...byDay.keys()].sort().find((k) => byDay.get(k)!.net > 0.5 * byDay.get(k)!.gross) ?? null;
}

function m1(data: MonthInsightData): MonthInsight | null {
  const f = data.forecast;
  if (!f) return null;
  const trend = f.models.find((m) => m.model === "trend");
  const parts: string[] = [];
  const thisOut = f.allowance_run_out_date;
  const lastOut = runOutDate(data.prevDaily);
  if (thisOut) {
    const verb = f.allowance_used_up ? "gikk tom" : "går tom rundt";
    let cmp = "";
    if (lastOut) {
      const diff = Number(lastOut.slice(8)) - Number(thisOut.slice(8));
      const prev = monthName(lastOut.slice(0, 7));
      cmp =
        diff === 0
          ? `, samme dag som i ${prev}`
          : `, ${Math.abs(diff)} ${Math.abs(diff) === 1 ? "dag" : "dager"} ${diff > 0 ? "tidligere" : "senere"} enn i ${prev}`;
    }
    parts.push(`Kredittene i lisensene ${verb} ${day(thisOut)}${cmp}`);
  } else {
    parts.push("Kredittene i lisensene ventes ikke brukt opp denne måneden");
  }
  if (trend && f.budget_status === "set" && f.budget_usd) {
    parts.push(`forventet faktura er ${formatPercent(trend.budget_pct)} av budsjettet`);
  } else if (trend) {
    parts.push(`forventet faktura er ${formatUSD(trend.net_mid_usd)}`);
  }
  const details: string[] = [];
  if (trend)
    details.push(
      `Forventet faktura (Nav samlet): ${formatUSD(trend.net_mid_usd)}, fra ${formatUSD(trend.net_low_usd)} til ${formatUSD(trend.net_high_usd)}.`
    );
  if (trend?.over_budget) details.push("Forventet faktura er over budsjettet i GitHub.");
  for (const a of (data.anomalies ?? []).filter((a) => a.rule === "A1").sort((a, b) => b.date.localeCompare(a.date)))
    details.push(
      `Uvanlig høyt forbruk ${formatDate(a.date)}: ${formatUSD(a.value)}, mot ${formatUSD(a.threshold)} i median for samme type dag.`
    );
  return {
    id: "M1",
    text: `${parts.join("; ")}.`,
    details,
    href: "#prognose",
    action: trend?.over_budget
      ? "Senk den personlige grensen, eller velg en billigere standardmodell i nav-pilot."
      : "Følg med på om prognosen nærmer seg budsjettet. Da er den personlige grensen spaken.",
  };
}

/** Users in bands from index minIdx up. */
function atLimit(d: BudgetDay, minIdx: number) {
  return (d.bands ?? []).filter((b) => b.first >= minIdx).reduce((s, b) => s + b.users, 0);
}

function m2(data: MonthInsightData): MonthInsight | null {
  const days = (data.budgetDays ?? []).filter((d) => d.bands);
  const now = days.at(-1);
  if (!now) return null;
  // The same day last month, or the nearest earlier day with data.
  const target = sameDay(now.day, prevMonth(now.day.slice(0, 7)));
  const then = days.filter((d) => d.day <= target && d.day.slice(0, 7) === target.slice(0, 7)).at(-1);
  if (!then) return null;
  // Bands under five people are merged; a merged band that straddles 90 % is left out, so the counts are a minimum.
  const idx = BAND_EDGES.indexOf(LIMIT_PRESSURE_PCT);
  const n = atLimit(now, idx);
  const p = atLimit(then, idx);
  const share = (x: number, d: BudgetDay) => (x / d.users) * 100;
  if (share(n, now) - share(p, then) <= LIMIT_PRESSURE_RISE_PP) return null;
  const details: string[] = [];
  const straddle = [now, then].some((d) => d.bands!.some((b) => b.first < idx && b.last >= idx && b.users > 0));
  if (straddle) details.push("Tallene er et minimum: grupper under fem personer er slått sammen med gruppen under.");
  // Rough: everyone in a band whose lower edge, at this month's pace, passes 100 % by month end.
  const month = now.day.slice(0, 7);
  const dim = new Date(Date.UTC(Number(month.slice(0, 4)), Number(month.slice(5)), 0)).getUTCDate();
  const elapsed = Number(now.day.slice(8));
  let clause = "";
  if (elapsed < dim) {
    const needed = (100 * elapsed) / dim;
    const reach = (now.bands ?? []).filter((b) => BAND_EDGES[b.first] >= needed).reduce((s, b) => s + b.users, 0);
    clause = `; ≈${formatNumber(reach)} når grensen før månedsslutt (grovt anslag)`;
    details.push(
      `Anslaget teller dem som allerede har brukt minst ${formatPercent(Math.ceil(needed))} av grensen etter ${elapsed} av ${dim} dager, og regner med samme tempo resten av måneden.`
    );
  }
  return {
    id: "M2",
    text: `${formatNumber(n)} personer (${pct(share(n, now))}) har brukt minst ${formatPercent(LIMIT_PRESSURE_PCT)} av grensen ${day(now.day)}, mot ${formatNumber(p)} (${pct(share(p, then))}) ${day(then.day)}${clause}.`,
    details,
    href: "#budsjett-per-dag",
    action: "Vurder om den personlige grensen bør heves for dem som trenger mer, eller holdes for å styre kostnaden.",
  };
}

const billedMonths = (months: AdminMonth[]) => months.filter((m) => m.basis === "net" || m.basis === "premium");

interface ModelShift {
  family: ModelFamily;
  rise: number;
  perUser: number;
  median: number;
}

/** M4: a family whose share of billed cost rose and whose cost per user is above the median. Never a new family. */
export function modelShifts(data: MonthInsightData): { shifts: ModelShift[]; last: string; prev: string } | null {
  const closed = billedMonths(data.months);
  const last = closed.at(-1)?.month;
  const prev = closed.at(-2)?.month;
  if (!last || !prev || !data.familyUsers) return null;
  const net = (m: string) => {
    const by = new Map<ModelFamily, number>();
    for (const r of data.breakdown)
      if (r.year_month.slice(0, 7) === m)
        by.set(modelFamily(r.model, true), (by.get(modelFamily(r.model, true)) ?? 0) + r.net_amount);
    return by;
  };
  const a = net(prev);
  const b = net(last);
  const total = (by: Map<ModelFamily, number>) => [...by.values()].reduce((s, v) => s + v, 0);
  const ta = total(a);
  const tb = total(b);
  if (!ta || !tb) return null;
  const perUser = new Map<ModelFamily, number>();
  for (const u of data.familyUsers)
    if (u.month === last && u.users >= MODEL_MIN_USERS && b.get(u.family))
      perUser.set(u.family, b.get(u.family)! / u.users);
  const sorted = [...perUser.values()].sort((x, y) => x - y);
  if (!sorted.length) return null;
  const median =
    sorted.length % 2
      ? sorted[(sorted.length - 1) / 2]
      : (sorted[sorted.length / 2 - 1] + sorted[sorted.length / 2]) / 2;
  const shifts: ModelShift[] = [];
  for (const [family, cost] of b) {
    const before = a.get(family) ?? 0;
    if (before <= 0) continue; // new this month: not a shift
    const rise = (cost / tb - before / ta) * 100;
    const pu = perUser.get(family);
    if (rise > MODEL_SHARE_RISE_PP && pu != null && pu > median) shifts.push({ family, rise, perUser: pu, median });
  }
  return { shifts: shifts.sort((x, y) => y.rise - x.rise), last, prev };
}

function m4(data: MonthInsightData): MonthInsight | null {
  const r = modelShifts(data);
  const s = r?.shifts[0];
  if (!r || !s) return null;
  return {
    id: "M4",
    text: `${FAMILY_LABELS[s.family]} fikk ${pp(s.rise)} større andel av det fakturerte beløpet i ${monthName(r.last)} enn i ${monthName(r.prev)}, og koster mer per person enn medianen.`,
    details: r.shifts.map(
      (x) =>
        `${FAMILY_LABELS[x.family]}: ${formatUSD(x.perUser)} per person i ${monthName(r.last)}, median for modellfamiliene ${formatUSD(x.median)}.`
    ),
    href: "#trender",
    action: `Vurder om ${FAMILY_LABELS[s.family]} skal tilbys, eller velg en billigere standardmodell i nav-pilot.`,
  };
}

/**
 * net = billed users × list-price use per user × share billed (net / gross). Each factor is the ratio
 * between the two months, so the three multiply to the change in net exactly.
 */
export function costFactors(a: BillingMonthlyTrend, b: BillingMonthlyTrend, ua: number, ub: number) {
  return {
    users: ub / ua,
    perUser: b.total_gross_amount / ub / (a.total_gross_amount / ua),
    rest: b.total_net_amount / b.total_gross_amount / (a.total_net_amount / a.total_gross_amount),
  };
}

function m3(data: MonthInsightData, modelMix: boolean): MonthInsight | null {
  const closed = billedMonths(data.months);
  const last = closed.at(-1)?.month;
  const prev = closed.at(-2)?.month;
  if (!last || !prev) return null;
  const t = (m: string) => data.trend.find((x) => x.year_month.slice(0, 7) === m);
  const u = (m: string) => data.active.find((x) => x.month === m && x.billed)?.active_users;
  const [ta, tb, ua, ub] = [t(prev), t(last), u(prev), u(last)];
  if (!ta || !tb || !ua || !ub || !ta.total_net_amount || !ta.total_gross_amount || !tb.total_gross_amount) return null;
  const total = tb.total_net_amount / ta.total_net_amount;
  if (Math.abs(total - 1) * 100 <= COST_CHANGE_MIN_PCT) return null;
  const { users, perUser, rest } = costFactors(ta, tb, ua, ub);
  return {
    id: "M3",
    text: `Det fakturerte beløpet endret seg ${change(total)} fra ${monthName(prev)} til ${monthName(last)}: fakturerte personer ${change(users)}, forbruk per person til listepris ${change(perUser)}, andel over kredittene i lisensene ${change(rest)}.`,
    details: [
      `Fakturert: ${formatUSD(ta.total_net_amount)} i ${monthName(prev)}, ${formatUSD(tb.total_net_amount)} i ${monthName(last)}. Fakturerte personer: ${formatNumber(ua)} og ${formatNumber(ub)}.`,
      `Forbruk per person er listepris per fakturert person${modelMix ? ", og endringen henger sammen med modellvalget under" : ""}.`,
      "Andel over kredittene i lisensene er den delen av forbruket som blir fakturert. Den stiger når kredittene går tom tidligere.",
      "De tre tallene ganget sammen gir den samlede endringen.",
    ],
    href: "#per-maned",
    // The lever follows the factor that moved most.
    action:
      users >= perUser && users >= rest
        ? "Flere brukere er ønsket. Hold den personlige grensen som den er."
        : perUser >= rest
          ? "Senk den personlige grensen, eller velg en billigere standardmodell i nav-pilot."
          : "Senk den personlige grensen, så går kredittene i lisensene senere tom.",
  };
}

function m5(data: MonthInsightData): MonthInsight | null {
  const closed = (data.high ?? []).filter((h) => h.basis !== "hittil");
  const [a, b] = closed.slice(-2);
  const sa = a?.tiers.find((t) => t.top_pct === 5)?.net_share_pct;
  const sb = b?.tiers.find((t) => t.top_pct === 5)?.net_share_pct;
  if (sa == null || sb == null || Math.abs(sb - sa) <= TOP5_CHANGE_PP) return null;
  return {
    id: "M5",
    text: `De 5 % som bruker mest sto for ${pct(sb)} av det fakturerte beløpet i ${monthName(b.month)}, ${pp(sb - sa)} fra ${monthName(a.month)}.`,
    href: "#storforbrukere",
    action: "Den personlige grensen tar toppen av forbruket hos de største brukerne.",
  };
}

function m6(data: MonthInsightData): MonthInsight | null {
  const closed = billedMonths(data.months).filter((m) => m.enterprise_net_usd && m.no_team_net_usd != null);
  const [a, b] = closed.slice(-2);
  if (!a || !b) return null;
  const sa = (a.no_team_net_usd! / a.enterprise_net_usd!) * 100;
  const sb = (b.no_team_net_usd! / b.enterprise_net_usd!) * 100;
  if (sb <= NO_TEAM_SHARE_PCT && sb - sa <= NO_TEAM_RISE_PP) return null;
  return {
    id: "M6",
    text: `Personer uten team sto for ${pct(sb)} av det fakturerte beløpet i ${monthName(b.month)}, mot ${pct(sa)} i ${monthName(a.month)}.`,
    details: [`${formatUSD(b.no_team_net_usd!)} i ${monthName(b.month)}. Det kan ikke fordeles på et team.`],
    href: "#per-maned",
    action: "Den personlige grensen gjelder også dem. Be dem registrere seg i et team, så får kostnaden en eier.",
  };
}

/** A7, unchanged from «Varsler». */
export function dataGapText(a: Anomaly): string {
  if (/^\d{4}-\d{2}$/.test(a.date))
    return `Fakturert forbruk for ${a.date} er ikke lest inn, selv om det skal være klart innen den ${a.threshold}.`;
  if (a.threshold > 0)
    return a.date
      ? `Ingen nye forbruksdata siden ${formatDate(a.date)}. Vanligvis kommer det nye data innen ${a.threshold} døgn.`
      : "Det finnes ingen forbruksdata.";
  return `Forbruksdata mangler for ${formatDate(a.date)}.`;
}

function m7(data: MonthInsightData): MonthInsight[] {
  return (data.anomalies ?? [])
    .filter((a) => a.rule === "A7")
    .map((a) => ({
      id: "M7",
      text: dataGapText(a),
      href: /^\d{4}-\d{2}$/.test(a.date) ? "/innsikt/team" : "/innsikt/bruk#kostnad",
      action: "Sjekk innlesingen før du stoler på tallene over.",
    }));
}

/** Data gaps first: the other statements mean nothing without data. */
export function monthInsights(data: MonthInsightData): MonthInsight[] {
  const mix = !!modelShifts(data)?.shifts.length;
  return [...m7(data), m1(data), m2(data), m3(data, mix), m4(data), m5(data), m6(data)].filter(
    (x): x is MonthInsight => x != null
  );
}
