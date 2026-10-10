import { Alert, Button, HStack, Select, Tag, VStack } from "@navikt/ds-react";
import { List, ListItem } from "@navikt/ds-react/List";
import { Table, TableBody, TableColumnHeader, TableDataCell, TableHeader, TableRow } from "@navikt/ds-react/Table";
import { DownloadIcon } from "@navikt/aksel-icons";
import type { TeamSpend, TeamYearMonth, TeamYearOverview } from "@/lib/types";
import { basisLabel, teamYearCaveats } from "@/lib/team-year";
import { formatUSD } from "@/lib/format";

const basisTag: Record<TeamYearMonth["basis"], "success" | "warning" | "neutral"> = {
  net: "success",
  gross: "warning",
  none: "neutral",
};

const monthName = (month: string) =>
  new Date(`${month}-01T00:00:00Z`).toLocaleDateString("nb-NO", { month: "long", timeZone: "UTC" });

/** Team picker for the year view. A GET form keeps the choice in the URL, like the month picker. */
export function TeamYearPicker({ teams, team, month }: { teams: TeamSpend[]; team: string; month: string }) {
  return (
    <form action="/innsikt/team#hittil-i-ar" method="get" aria-label="Velg team">
      <input type="hidden" name="month" value={month} />
      <HStack gap="space-8" align="end" wrap>
        <Select key={team} label="Team" name="team" defaultValue={team}>
          <option value="">Velg team</option>
          {teams.map((t) => (
            <option key={t.team_id} value={t.team_id}>
              {t.team_slug}
            </option>
          ))}
        </Select>
        <Button type="submit" variant="secondary-neutral">
          Vis
        </Button>
      </HStack>
    </form>
  );
}

export default function TeamYearCost({ data }: { data: TeamYearOverview }) {
  const [additive, ...caveats] = teamYearCaveats(data);
  const cell = (m: TeamYearMonth, value: number | null) =>
    value !== null ? formatUSD(value) : m.hidden ? "Skjult" : "—";
  return (
    <VStack gap="space-16">
      <Alert variant="warning" size="small">
        {additive} Ikke legg sammen årstall for flere team.
      </Alert>
      <div className="overflow-x-auto">
        <Table size="small" aria-label={`Forbruk per måned for ${data.team_slug} i ${data.year}`}>
          <TableHeader>
            <TableRow>
              <TableColumnHeader scope="col">Måned</TableColumnHeader>
              <TableColumnHeader scope="col">Grunnlag</TableColumnHeader>
              <TableColumnHeader scope="col" align="right">
                Medlemmer med forbruk
              </TableColumnHeader>
              <TableColumnHeader scope="col" align="right">
                Netto
              </TableColumnHeader>
              <TableColumnHeader scope="col" align="right">
                Brutto
              </TableColumnHeader>
              <TableColumnHeader scope="col" align="right">
                Netto uten bruk
              </TableColumnHeader>
            </TableRow>
          </TableHeader>
          <TableBody>
            {data.months.map((m) => (
              <TableRow key={m.month}>
                <TableDataCell className="capitalize">{monthName(m.month)}</TableDataCell>
                <TableDataCell>
                  <Tag size="xsmall" variant={basisTag[m.basis]}>
                    {basisLabel[m.basis]}
                  </Tag>
                </TableDataCell>
                <TableDataCell align="right">{m.users ?? (m.hidden ? "Skjult" : "—")}</TableDataCell>
                <TableDataCell align="right">{m.basis === "net" ? cell(m, m.net_usd) : "—"}</TableDataCell>
                <TableDataCell align="right">{m.basis === "none" ? "—" : cell(m, m.gross_usd)}</TableDataCell>
                <TableDataCell align="right">{m.basis === "net" ? cell(m, m.no_usage_net_usd) : "—"}</TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <List size="small">
        {caveats.map((caveat) => (
          <ListItem key={caveat}>{caveat}</ListItem>
        ))}
      </List>
      <div>
        <Button
          as="a"
          href={`/api/team-year?team=${encodeURIComponent(data.team_id)}&year=${data.year}`}
          variant="secondary"
          size="small"
          icon={<DownloadIcon aria-hidden />}
          download
        >
          Last ned CSV
        </Button>
      </div>
    </VStack>
  );
}
