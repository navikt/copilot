import { formatPercent } from "./format";
import type { TeamYearMonth, TeamYearOverview } from "./types";

export const basisLabel: Record<TeamYearMonth["basis"], string> = {
  net: "Netto",
  premium: "Premiumforespørsler",
  gross: "Brutto",
  none: "Ingen data",
};

/** Caveats shown above the panel table and as # lines in the CSV. */
export function teamYearCaveats(data: TeamYearOverview): string[] {
  const { membership_from, gross_from, last_usage_day } = data.coverage;
  return [
    "Beløp for ulike team kan ikke legges sammen, heller ikke for hele året. En person som er med i flere team, telles i hvert av dem.",
    "Netto er fakturert forbruk etter fradrag, fordelt på dagene hver person brukte Copilot. Brutto er listepris før fradrag, og brukes inntil fakturaen for måneden er lest inn.",
    "Januar til mai 2026 ble fakturert som premiumforespørsler. For disse månedene har vi ikke forbruk per dag. Netto er derfor fordelt likt på dagene hver person brukte Copilot, og brutto mangler.",
    ...residualCaveats(data),
    `Teamhistorikken starter ${membership_from || "ukjent dato"}. For dager før det regner vi hver person med i de teamene personen var med i den første dagen vi har registrert.`,
    `Ingen data betyr at måneden mangler teamhistorikk (finnes fra ${membership_from || "ukjent dato"}) eller forbruk per person (finnes fra ${gross_from || "ukjent dato"}).`,
    "Skjult betyr at færre enn fem medlemmer hadde forbruk den måneden.",
    "Netto uten bruk er fakturert beløp for medlemmer uten registrert bruk i måneden. Beløpet kan ikke fordeles på dager og er derfor ikke med i netto.",
    ...licenceCaveats(data),
    `Tallene går til og med ${last_usage_day || "ukjent dato"}.`,
  ];
}

/** One line per month where more than 1 % of the bill could not be assigned to a person. */
function residualCaveats(data: TeamYearOverview): string[] {
  return data.months
    .filter((m) => (m.residual_pct ?? 0) > 1)
    .map((m) => {
      const name = new Date(`${m.month}-01T00:00:00Z`).toLocaleString("nb-NO", { month: "long", timeZone: "UTC" });
      return `${name[0].toUpperCase()}${name.slice(1)}: ${formatPercent(m.residual_pct ?? 0)} av Navs nettoforbruk kunne ikke knyttes til en person og er derfor ikke med i tallene for noe team.`;
    });
}

function licenceCaveats(data: TeamYearOverview): string[] {
  const { seats_snapshot, team_members_from } = data.coverage;
  if (!seats_snapshot || !team_members_from) {
    return ["Lisenser er ikke lest inn ennå. De vises etter første nattlige innlesing."];
  }
  return [
    "Lisenser er et anslag. Vi regner listepris per måned, 19 USD for Business og 39 USD for Enterprise, for hvert medlem av teamet med lisens, fra måneden lisensen ble tildelt. Hadde noen lisens før det, er den perioden ikke med. Har færre enn fem medlemmer lisens, viser vi ikke tallet.",
    `Medlemslister finnes fra ${team_members_from}. For månedene før bruker vi medlemmene teamet hadde den dagen.`,
    "Totalt med lisenser legger lisensene til netto og netto uten bruk, eller til brutto når måneden bruker brutto. Feltet er tomt når en av delene mangler eller er skjult.",
  ];
}

const amount = (value: number | null | undefined) => (value == null ? "" : value.toFixed(2));

export function teamYearCsv(data: TeamYearOverview): string {
  const lines = [
    `# Copilot-forbruk i USD for team ${data.team_slug} (${data.team_id}), ${data.year}`,
    ...teamYearCaveats(data).map((caveat) => `# ${caveat}`),
    "måned,grunnlag,medlemmer_med_forbruk,netto_usd,brutto_usd,netto_uten_bruk_usd,medlemmer_med_lisens,lisenser_usd_anslag,totalt_med_lisenser_usd",
    ...data.months.map((m) =>
      [
        m.month,
        m.hidden ? `${basisLabel[m.basis]} (skjult)` : basisLabel[m.basis],
        m.users ?? "",
        amount(m.net_usd),
        amount(m.gross_usd),
        amount(m.no_usage_net_usd),
        m.seat_months ?? "",
        amount(m.licence_usd_estimate),
        amount(m.total_usd),
      ].join(",")
    ),
  ];
  return lines.join("\n") + "\n";
}
