import { monthName } from "./team-year";
import type { AdminMonth, AdminMonthlyOverview } from "./types";

const basisLabel: Record<AdminMonth["basis"], string> = {
  net: "Netto",
  premium: "Premiumforespørsler",
  gross: "Brutto (foreløpig)",
  none: "Ingen data",
};

/** CSV column names; the page uses the same order. */
export const adminMonthlyHeader = [
  "måned",
  "grunnlag",
  "netto_usd",
  "fordelt_på_personer_usd",
  "restbeløp_usd",
  "restbeløp_prosent",
  "uten_team_usd",
  "brutto_usd",
  "lisenser",
  "lisenser_usd_anslag",
  "totalt_med_lisenser_usd",
];

/** Caveats written as # lines in the CSV and listed under the table. */
export function adminMonthlyCaveats(data: AdminMonthlyOverview): string[] {
  const { membership_from, last_gross_day, last_net_month, seats_snapshot } = data.coverage;
  return [
    "Totalraden er summen av de fakturerte månedene. Brutto er foreløpig og er ikke med i totalen.",
    "Januar til mai 2026 ble fakturert som premiumforespørsler, ikke KI-kreditter. Totalraden legger disse månedene sammen med netto fra juni.",
    "Netto er Navs faktura etter fradrag. Fordelt på personer og restbeløpet blir til sammen netto. Restbeløpet er den delen av fakturaen som ikke kan knyttes til en person.",
    `Uten team er netto for personer som ikke var med i noe team de dagene de brukte Copilot. Teamhistorikken starter ${membership_from || "ukjent dato"}, så feltet er tomt for månedene før.`,
    "Brutto er listepris før fradrag. Den brukes for måneden som pågår og for forrige måned til fakturaen er lest inn, vanligvis den 3. i måneden.",
    seats_snapshot
      ? "Lisenser er et anslag: antall lisenser som gjaldt måneden, til listepris 19 USD for Business og 39 USD for Enterprise. Rabatt er ikke med, og lisenser som ble sagt opp før første innlesing, mangler."
      : "Lisenser er ikke lest inn ennå. De vises etter første nattlige innlesing.",
    `Siste dag med brutto: ${last_gross_day || "ukjent"}. Siste måned med netto: ${last_net_month ? monthName(last_net_month) : "ingen"}.`,
  ];
}

const amount = (value: number | null | undefined) => (value == null ? "" : value.toFixed(2));

const billed = (m: AdminMonth) => m.basis === "net" || m.basis === "premium";

/** Cells per month with the total row last; the CSV and the page both render these. */
export function adminMonthlyRows(data: AdminMonthlyOverview): string[][] {
  return [
    ...data.months.map((m) => [
      m.month,
      basisLabel[m.basis],
      amount(m.enterprise_net_usd),
      amount(m.assigned_net_usd),
      amount(m.residual_net_usd),
      amount(m.residual_pct),
      amount(m.no_team_net_usd),
      amount(m.gross_usd),
      String(m.seats ?? ""),
      amount(m.licence_usd_estimate),
      amount(m.total_usd),
    ]),
    adminMonthlyTotalRow(data.months.filter(billed)),
  ];
}

/** Sum of the billed months. A column is empty when no billed month has a value. */
function adminMonthlyTotalRow(months: AdminMonth[]): string[] {
  const sum = (pick: (m: AdminMonth) => number | null) => {
    const values = months.map(pick).filter((v): v is number => v != null);
    return values.length ? values.reduce((a, b) => a + b, 0) : null;
  };
  const net = sum((m) => m.enterprise_net_usd);
  const residual = sum((m) => m.residual_net_usd);
  const span = [...new Set([months[0], months.at(-1)].filter((m) => m).map((m) => monthName(m!.month)))];
  return [
    span.length ? `Totalt ${span.join("–")}` : "Totalt",
    "",
    amount(net),
    amount(sum((m) => m.assigned_net_usd)),
    amount(residual),
    net && residual != null ? amount((residual / net) * 100) : "",
    amount(sum((m) => m.no_team_net_usd)),
    "",
    "",
    months.every((m) => m.licence_usd_estimate != null) ? amount(sum((m) => m.licence_usd_estimate)) : "",
    months.every((m) => m.total_usd != null) ? amount(sum((m) => m.total_usd)) : "",
  ];
}

export function adminMonthlyCsv(data: AdminMonthlyOverview): string {
  return (
    [
      `# Copilot-kostnad i USD for hele Nav, ${data.year}`,
      ...adminMonthlyCaveats(data).map((c) => `# ${c}`),
      adminMonthlyHeader.join(","),
      ...adminMonthlyRows(data).map((row) => row.join(",")),
    ].join("\n") + "\n"
  );
}
