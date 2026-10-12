import { formatPercent } from "./format";
import type { TeamYearMonth, TeamYearOverview } from "./types";

/** A gross month is always preliminary: it is replaced by net once the invoice is read in. */
const basisLabel: Record<TeamYearMonth["basis"], string> = {
  net: "Netto",
  premium: "Premiumforespørsler",
  gross: "Brutto (foreløpig)",
  none: "Ingen data",
};

/** Caveats written as # lines in the CSV. */
export function teamYearCaveats(data: TeamYearOverview): string[] {
  const { membership_from, gross_from, last_usage_day } = data.coverage;
  return [
    "Totalraden er summen av de fakturerte månedene. Brutto er foreløpig og er ikke med i totalen.",
    "Beløp for ulike team kan ikke legges sammen: en person som er med i flere team, telles i hvert av dem.",
    "Netto er fakturert forbruk etter fradrag, fordelt på dagene hver person brukte Copilot. Brutto er listepris før fradrag, og brukes inntil fakturaen for måneden er lest inn.",
    "Januar til mai 2026 ble fakturert som premiumforespørsler. For disse månedene har vi ikke forbruk per dag. Netto er derfor fordelt likt på dagene hver person brukte Copilot, og brutto mangler.",
    ...residualCaveats(data),
    `Teamhistorikken starter ${membership_from || "ukjent dato"}. For dager før det regner vi hver person med i de teamene personen var med i den første dagen vi har registrert.`,
    ...(data.months.some((m) => m.basis === "none")
      ? [
          `Ingen data betyr at måneden mangler teamhistorikk (finnes fra ${membership_from || "ukjent dato"}) eller forbruk per person (finnes fra ${gross_from || "ukjent dato"}), eller at fakturaen for måneden ikke er lest inn ennå.`,
        ]
      : []),
    "Netto uten bruk er fakturert beløp for medlemmer uten registrert bruk i måneden. Beløpet kan ikke fordeles på dager og er derfor ikke med i netto.",
    ...licenceCaveats(data),
    `Tallene går til og med ${last_usage_day || "ukjent dato"}.`,
  ];
}

/** One line listing the months where more than 1 % of the bill could not be assigned to a person. */
function residualCaveats(data: TeamYearOverview): string[] {
  const months = data.months
    .filter((m) => (m.residual_pct ?? 0) > 1)
    .map(
      (m) =>
        `${new Date(`${m.month}-01T00:00:00Z`).toLocaleString("nb-NO", { month: "long", timeZone: "UTC" })} ${formatPercent(m.residual_pct ?? 0)}`
    );
  return months.length
    ? [
        `Andel av Navs nettoforbruk som ikke kunne knyttes til en person, og som derfor ikke er med i tallene for noe team: ${months.join(", ")}.`,
      ]
    : [];
}

function licenceCaveats(data: TeamYearOverview): string[] {
  const { seats_snapshot, team_members_from } = data.coverage;
  if (!seats_snapshot || !team_members_from) {
    return ["Lisenser er ikke lest inn ennå. De vises etter første nattlige innlesing."];
  }
  return [
    "Lisenser er et anslag. Vi regner listepris per måned, 19 USD for Business og 39 USD for Enterprise, for hvert medlem av teamet med lisens, fra måneden lisensen ble tildelt. Hadde noen lisens før det, er den perioden ikke med.",
    `Medlemslister finnes fra ${team_members_from}. For månedene før bruker vi medlemmene teamet hadde den dagen.`,
    "Totalt med lisenser legger lisensene til netto og netto uten bruk, eller til brutto når måneden bruker brutto. Feltet er tomt når en av delene mangler.",
  ];
}

/** «januar» for "2026-01". */
export const monthName = (month: string) =>
  new Date(`${month}-01T00:00:00Z`).toLocaleString("nb-NO", { month: "long", timeZone: "UTC" });

const amount = (value: number | null | undefined) => (value == null ? "" : value.toFixed(2));

/** CSV cells per month plus the total row; the CSV and the PDF report both render these. */
export function teamYearRows(data: TeamYearOverview): string[][] {
  return [
    ...data.months.map((m) => [
      m.month,
      basisLabel[m.basis],
      String(m.users ?? ""),
      amount(m.net_usd),
      amount(m.gross_usd),
      amount(m.no_usage_net_usd),
      String(m.seat_months ?? ""),
      amount(m.licence_usd_estimate),
      amount(m.total_usd),
    ]),
    teamYearTotalRow(data),
  ];
}

export function teamYearCsv(data: TeamYearOverview): string {
  const lines = [
    `# Copilot-forbruk i USD for team ${data.team_slug} (${data.team_id}), ${data.year}`,
    ...teamYearCaveats(data).map((caveat) => `# ${caveat}`),
    "måned,grunnlag,medlemmer_med_forbruk,netto_usd,brutto_usd,netto_uten_bruk_usd,medlemmer_med_lisens,lisenser_usd_anslag,totalt_med_lisenser_usd",
    ...teamYearRows(data).map((row) => row.join(",")),
  ];
  return lines.join("\n") + "\n";
}

/** Sum of the billed months (net and premium). The gross month is preliminary and left out. */
function teamYearTotalRow(data: TeamYearOverview): string[] {
  const billed = data.months.filter((m) => m.basis === "net" || m.basis === "premium");
  const sum = (pick: (m: TeamYearMonth) => number | null) => {
    const values = billed.map(pick);
    return values.length && values.every((v) => v != null) ? values.reduce((a, b) => a! + b!, 0) : null;
  };
  const span = [...new Set([billed[0], billed.at(-1)].filter((m) => m).map((m) => monthName(m!.month)))];
  return [
    span.length ? `Totalt ${span.join("–")}` : "Totalt",
    "",
    "",
    amount(sum((m) => m.net_usd)),
    "",
    amount(sum((m) => m.no_usage_net_usd)),
    "",
    amount(sum((m) => m.licence_usd_estimate)),
    amount(sum((m) => m.total_usd)),
  ];
}

/** The report hides «netto uten bruk» when no month has an amount; the CSV keeps every column. */
export const hasNoUsageAmounts = (rows: string[][]) => rows.some((r) => Number(r[5]) !== 0);
