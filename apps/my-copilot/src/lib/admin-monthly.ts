import { monthName } from "./team-year";
import type { AdminMonth, AdminMonthlyOverview } from "./types";

const basisLabel: Record<AdminMonth["basis"], string> = {
  net: "Fakturert",
  premium: "Premiumforespørsler",
  gross: "Foreløpig forbruk",
  none: "Ingen data",
};

/** CSV column names; the page uses the same order. */
export const adminMonthlyHeader = [
  "måned",
  "grunnlag",
  "fakturert_usd",
  "fordelt_på_personer_usd",
  "ikke_fordelt_usd",
  "ikke_fordelt_prosent",
  "personer_uten_team_usd",
  "foreløpig_forbruk_usd",
  "lisenser",
  "lisenskostnad_usd_anslag",
  "totalt_inkl_lisenser_usd",
  "estimert_fakturert_hittil_usd",
];

/** How to read the numbers: # lines in the CSV, and «Slik leser du tabellen» on the page. */
export function adminMonthlyCaveats(data: AdminMonthlyOverview): string[] {
  const { membership_from, seats_snapshot } = data.coverage;
  const rebuilt = rebuiltSeatMonths(data);
  return [
    "Fakturert er beløpet Nav betaler etter rabatt. Fordelt på personer og ikke fordelt gir til sammen fakturert.",
    "Foreløpig forbruk er forbruket til listepris for måneden som pågår. Det byttes ut med fakturert beløp når fakturaen kommer, rundt den 3. i måneden etter.",
    "Estimert fakturert hittil er et foreløpig anslag for måneder uten faktura: foreløpig forbruk minus den delen kredittene i lisensene dekker. Kredittene deles av alle i Nav. For måneden som pågår er det samme anslag som «Forbruk hittil» øverst på siden, og det er dette beløpet, ikke foreløpig forbruk, som er med i totalt inkl. lisenser.",
    "Totalraden er summen av de fakturerte månedene. Måneden som pågår er ikke med.",
    "Januar til mai 2026 ble fakturert som premiumforespørsler, ikke KI-kreditter. Totalraden legger dem sammen med månedene fra juni.",
    `Personer uten team er fakturert beløp for personer som ikke var med i noe team de dagene de brukte Copilot. Teamhistorikken starter ${membership_from || "ukjent dato"}, så feltet er tomt for månedene før.`,
    seats_snapshot
      ? "Lisenskostnad er et anslag: antall lisenser ganger listepris, uten rabatt. Listeprisen er 19 USD for Business og 39 USD for Enterprise."
      : "Lisensene er ikke lest inn ennå. De vises etter første nattlige innlesing.",
    ...(seats_snapshot && rebuilt.length
      ? [
          `Lisenser og lisenskostnad for ${monthSpan(rebuilt)} er beregnet ut fra dagens lisenser, for lavt for tidligere måneder. Daglige tellinger finnes fra ${data.coverage.seats_from}.`,
        ]
      : []),
  ];
}

/** Months whose seats are rebuilt from today's seats: before the month of the first daily snapshot. */
export function rebuiltSeatMonths(data: AdminMonthlyOverview): string[] {
  const from = data.coverage.seats_from?.slice(0, 7);
  // A first snapshot after the 1st leaves the start of its month rebuilt too.
  const partial = data.coverage.seats_from && !data.coverage.seats_from.endsWith("-01");
  return data.months
    .filter((m) => m.seats != null && (!from || m.month < from || (partial && m.month === from)))
    .map((m) => m.month);
}

const monthSpan = (months: string[]) => [...new Set([months[0], months.at(-1)!])].map(monthName).join("–");

/** The last day and month the numbers cover; in the CSV, and under «Kilde og metode» on the page. */
export function adminMonthlyCoverage(data: AdminMonthlyOverview): string {
  const { last_gross_day, last_net_month } = data.coverage;
  return `Siste dag med foreløpig forbruk: ${last_gross_day || "ukjent"}. Siste fakturerte måned: ${last_net_month ? monthName(last_net_month) : "ingen"}.`;
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
      amount(m.estimated_net_usd),
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
    "",
  ];
}

export function adminMonthlyCsv(data: AdminMonthlyOverview): string {
  return (
    [
      `# Copilot-kostnad i USD for hele Nav, ${data.year}`,
      "# Ny kolonne fra oktober 2026: estimert_fakturert_hittil_usd, for måneden som pågår.",
      "# Nye kolonnenavn fra oktober 2026: netto_usd heter fakturert_usd, restbeløp_* heter ikke_fordelt_*, uten_team_usd heter personer_uten_team_usd, brutto_usd heter foreløpig_forbruk_usd, lisenser_usd_anslag heter lisenskostnad_usd_anslag og totalt_med_lisenser_usd heter totalt_inkl_lisenser_usd. I grunnlag heter Netto nå Fakturert og Brutto (foreløpig) nå Foreløpig forbruk.",
      ...[...adminMonthlyCaveats(data), adminMonthlyCoverage(data)].map((c) => `# ${c}`),
      adminMonthlyHeader.join(","),
      ...adminMonthlyRows(data).map((row) => row.join(",")),
    ].join("\n") + "\n"
  );
}
