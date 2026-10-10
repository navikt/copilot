import type { TeamYearMonth, TeamYearOverview } from "./types";

export const basisLabel: Record<TeamYearMonth["basis"], string> = {
  net: "Netto",
  gross: "Brutto",
  none: "Ingen data",
};

/** Caveats shown above the panel table and as # lines in the CSV. */
export function teamYearCaveats(data: TeamYearOverview): string[] {
  const { membership_from, gross_from, last_usage_day } = data.coverage;
  return [
    "Beløp for ulike team kan ikke legges sammen, heller ikke for hele året. En person som er med i flere team, telles i hvert av dem.",
    "Netto er fakturert forbruk etter fradrag, fordelt på dagene hver person brukte Copilot. Brutto er listepris før fradrag, og brukes inntil fakturaen for måneden er lest inn.",
    `Ingen data betyr at måneden mangler teamhistorikk (finnes fra ${membership_from || "ukjent dato"}) eller forbruk per person (finnes fra ${gross_from || "ukjent dato"}).`,
    "Skjult betyr at færre enn fem medlemmer hadde forbruk den måneden.",
    "Netto uten bruk er fakturert beløp for medlemmer uten registrert bruk i måneden. Beløpet kan ikke fordeles på dager og er derfor ikke med i netto.",
    "Lisenser er ikke med.",
    `Tallene går til og med ${last_usage_day || "ukjent dato"}.`,
  ];
}

const amount = (value: number | null) => (value === null ? "" : value.toFixed(2));

export function teamYearCsv(data: TeamYearOverview): string {
  const lines = [
    `# Copilot-forbruk i USD for team ${data.team_slug} (${data.team_id}), ${data.year}`,
    ...teamYearCaveats(data).map((caveat) => `# ${caveat}`),
    "måned,grunnlag,medlemmer_med_forbruk,netto_usd,brutto_usd,netto_uten_bruk_usd",
    ...data.months.map((m) =>
      [
        m.month,
        m.hidden ? `${basisLabel[m.basis]} (skjult)` : basisLabel[m.basis],
        m.users ?? "",
        amount(m.net_usd),
        amount(m.gross_usd),
        amount(m.no_usage_net_usd),
      ].join(",")
    ),
  ];
  return lines.join("\n") + "\n";
}
