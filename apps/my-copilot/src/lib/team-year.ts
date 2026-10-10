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
    "Teambeløp kan ikke summeres på tvers av team. En person som er med i flere team, telles i hvert av dem.",
    "Netto er fakturert forbruk etter fradrag, fordelt på dagene personen brukte Copilot. Brutto er listepris før fradrag og brukes til fakturaen for måneden er lest inn.",
    `Ingen data betyr at teamhistorikk (fra ${membership_from || "ukjent"}) eller forbruk per person (fra ${gross_from || "ukjent"}) mangler for måneden.`,
    "Skjult betyr at færre enn fem medlemmer hadde forbruk.",
    "Netto uten bruk er fakturert beløp for medlemmer uten registrert bruk i måneden. Det kan ikke fordeles på dager og er ikke med i netto.",
    "Lisenser er ikke med.",
    `Data til og med ${last_usage_day || "ukjent dato"}.`,
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
