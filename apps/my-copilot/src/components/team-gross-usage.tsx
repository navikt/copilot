"use client";

import { useState } from "react";
import { BodyShort, Search, Table, VStack } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableRow } from "@navikt/ds-react/Table";
import type { TeamGrossOverview, TeamNetOverview } from "@/lib/types";

export default function TeamGrossUsage({ data, net }: { data: TeamGrossOverview; net: TeamNetOverview | null }) {
  const [search, setSearch] = useState("");
  const teams = (net?.teams ?? data.teams).filter((team) =>
    team.team_slug.toLowerCase().includes(search.trim().toLowerCase())
  );
  const dollars = (amount: number) =>
    new Intl.NumberFormat("nb-NO", { style: "currency", currency: "USD" }).format(amount);

  return (
    <VStack gap="space-16">
      <BodyShort>
        {net ? `Netto AI-kostnad (${net.sku})` : "Brutto AI-bruk"} i {data.month}, bruksdata til og med{" "}
        {data.last_usage_day} ({data.days_with_usage} dager med data). Hvert team får hele bruken til medlemmene sine.
        En person i flere team telles i hvert av dem, så teambeløpene kan ikke summeres til Navs regning.{" "}
        {net
          ? "Netto per bruker er fakturert. Fordelingen mellom team ved teambytte er beregnet ut fra registrerte bruksdager."
          : "Dette er ikke fakturert netto kostnad."}
      </BodyShort>
      <form action="/innsikt/team" method="get">
        <label htmlFor="team-gross-month">Måned</label>{" "}
        <input
          id="team-gross-month"
          name="month"
          type="month"
          defaultValue={data.month}
          max={new Date().toISOString().slice(0, 7)}
        />{" "}
        <button type="submit">Vis måned</button>
      </form>
      <BodyShort>
        {net
          ? `Navs fakturerte AI-bruk: ${dollars(net.enterprise_net_usd)}. Kjente brukere: ${dollars(net.known_net_usd)}. Ikke knyttet til team: ${dollars(net.unassigned_net_usd)}, hvorav ${dollars(net.no_usage_net_usd)} mangler bruksdager. Uten kjent bruker: ${dollars(net.residual_net_usd)}.`
          : `Navs unike brutto brukerforbruk: ${dollars(data.distinct_gross_usd)}. Uten team: ${dollars(data.unassigned_gross_usd)}.`}{" "}
        {data.small_teams} team med færre enn fem brukere vises samlet:{" "}
        {dollars(net?.small_teams_net_usd ?? data.small_teams_gross_usd)} fra {data.small_teams_users} ulike brukere.
        Disse brukerne kan også inngå i synlige team.
      </BodyShort>
      <Search label="Søk etter team" value={search} onChange={setSearch} size="small" className="max-w-xs" />
      <Table size="small">
        <TableHeader>
          <TableRow>
            <Table.ColumnHeader scope="col">Team</Table.ColumnHeader>
            <Table.ColumnHeader scope="col" align="right">
              Brukere
            </Table.ColumnHeader>
            <Table.ColumnHeader scope="col" align="right">
              {net ? "Netto medlemskostnad" : "Brutto medlemsbruk"}
            </Table.ColumnHeader>
          </TableRow>
        </TableHeader>
        <TableBody>
          {teams.map((team) => (
            <TableRow key={team.team_id}>
              <TableDataCell>{team.team_slug}</TableDataCell>
              <TableDataCell align="right">{team.users}</TableDataCell>
              <TableDataCell align="right">{dollars("net_usd" in team ? team.net_usd : team.gross_usd)}</TableDataCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </VStack>
  );
}
