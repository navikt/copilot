"use client";

import { useState } from "react";
import { BodyShort, Button, Heading, HStack, Search, Table, VStack } from "@navikt/ds-react";
import Link from "next/link";
import { TableBody, TableDataCell, TableHeader, TableRow } from "@navikt/ds-react/Table";
import type { TeamGrossOverview, TeamNetOverview } from "@/lib/types";
import { previousMonth } from "@/lib/month-utils";

type Team = TeamGrossOverview["teams"][number] | TeamNetOverview["teams"][number];

export function compareTeamMonth(team: Team, previous: TeamGrossOverview | TeamNetOverview | null): number | null {
  const old = previous?.teams.find((row) => row.team_id === team.team_id);
  if (!old || old.users < 5) return null;
  return "net_usd" in team && "net_usd" in old
    ? team.net_usd - old.net_usd
    : "gross_usd" in team && "gross_usd" in old
      ? team.gross_usd - old.gross_usd
      : null;
}

export default function TeamGrossUsage({
  data,
  net,
  myTeams,
  previous,
}: {
  data: TeamGrossOverview;
  net: TeamNetOverview | null;
  myTeams: string[] | null;
  previous: TeamGrossOverview | TeamNetOverview | null;
}) {
  const [search, setSearch] = useState("");
  const source = net?.teams ?? data.teams;
  const mine = new Set((myTeams ?? []).map((team) => team.toLowerCase()));
  const own = source.filter((team) => mine.has(team.team_slug.toLowerCase()));
  const others = source.filter((team) => !mine.has(team.team_slug.toLowerCase()));
  const filtered = (teams: Team[]) =>
    teams.filter((team) => team.team_slug.toLowerCase().includes(search.trim().toLowerCase()));
  const dollars = (amount: number) =>
    new Intl.NumberFormat("nb-NO", { style: "currency", currency: "USD" }).format(amount);
  const amount = (team: Team) => ("net_usd" in team ? team.net_usd : team.gross_usd);
  const title = net ? "Netto medlemskostnad" : "Brutto medlemsbruk";

  const rows = (teams: Team[]) =>
    filtered(teams).map((team) => {
      const change = compareTeamMonth(team, previous);
      return (
        <TableRow key={team.team_id}>
          <TableDataCell>{team.team_slug}</TableDataCell>
          <TableDataCell align="right">{team.users}</TableDataCell>
          <TableDataCell align="right">{dollars(amount(team))}</TableDataCell>
          <TableDataCell align="right">
            {change === null ? "Ikke tilgjengelig" : `${change >= 0 ? "+" : ""}${dollars(change)}`}
          </TableDataCell>
        </TableRow>
      );
    });

  const table = (teams: Team[], label: string) => (
    <Table size="small" aria-label={label}>
      <TableHeader>
        <TableRow>
          <Table.ColumnHeader scope="col">Team</Table.ColumnHeader>
          <Table.ColumnHeader scope="col" align="right">
            Brukere
          </Table.ColumnHeader>
          <Table.ColumnHeader scope="col" align="right">
            {title}
          </Table.ColumnHeader>
          <Table.ColumnHeader scope="col" align="right">
            Endring fra forrige måned
          </Table.ColumnHeader>
        </TableRow>
      </TableHeader>
      <TableBody>{rows(teams)}</TableBody>
    </Table>
  );

  return (
    <VStack gap="space-32">
      <HStack gap="space-8" align="center" wrap>
        <form action="/innsikt/team" method="get" aria-label="Velg måned">
          <label htmlFor="team-spend-month">Måned</label>{" "}
          <input
            id="team-spend-month"
            name="month"
            type="month"
            defaultValue={data.month}
            max={new Date().toISOString().slice(0, 7)}
          />{" "}
          <Button type="submit" size="small" variant="secondary-neutral">
            Vis måned
          </Button>
        </form>
        {data.month > "2026-05" && <Link href={`/innsikt/team?month=${previousMonth(data.month)}`}>Forrige måned</Link>}
      </HStack>

      <section aria-labelledby="navs-regning">
        <VStack gap="space-8">
          <Heading id="navs-regning" level="3" size="small">
            Navs regning
          </Heading>
          {net ? (
            <BodyShort>
              Fakturert for {net.sku}: {dollars(net.enterprise_net_usd)}. Kjente brukere: {dollars(net.known_net_usd)}.
              Av dette er {dollars(net.unassigned_net_usd)} ikke knyttet til et team, og {dollars(net.residual_net_usd)}{" "}
              har ingen kjent bruker. Lisensutgifter er ikke med. Hentet {net.loaded_at.slice(0, 10)}.
            </BodyShort>
          ) : (
            <BodyShort>
              Fakturerte brukerbeløp er ikke tilgjengelige for {data.month}. Brutto forbruk for ulike brukere:{" "}
              {dollars(data.distinct_gross_usd)}. Uten team: {dollars(data.unassigned_gross_usd)}. Dette er ikke Navs
              fakturerte nettokostnad.
            </BodyShort>
          )}
          <BodyShort>
            Bruksdata til og med {data.last_usage_day} ({data.days_with_usage} dager).
          </BodyShort>
        </VStack>
      </section>

      <section aria-labelledby="medlemsbruk">
        <VStack gap="space-16">
          <Heading id="medlemsbruk" level="3" size="small">
            Medlemsbruk i team
          </Heading>
          <BodyShort>
            Hvert team viser hele bruken til medlemmene sine. En person som tilhører flere team telles flere steder.
            Teambeløp og prosenter kan derfor ikke summeres til Navs regning.
            {net
              ? " Netto per bruker er fakturert; plasseringen i team ved teambytte er beregnet ut fra bruksdager."
              : " Beløpene er brutto, ikke fakturert netto."}
          </BodyShort>
          <Search label="Søk etter team" value={search} onChange={setSearch} size="small" className="max-w-xs" />
          {myTeams === null && <BodyShort>Kunne ikke finne dine team. Du kan fortsatt søke i teamlisten.</BodyShort>}
          {myTeams !== null && (
            <section aria-labelledby="mine-team">
              <Heading id="mine-team" level="4" size="xsmall" spacing>
                Mine team
              </Heading>
              {own.length ? (
                table(own, "Mine team")
              ) : (
                <BodyShort>
                  Ingen av teamene dine vises for denne måneden. Team med færre enn fem betalende brukere skjules.
                </BodyShort>
              )}
            </section>
          )}
          <section aria-labelledby="andre-team">
            <Heading id="andre-team" level="4" size="xsmall" spacing>
              Andre team
            </Heading>
            {filtered(others).length ? table(others, "Andre team") : <BodyShort>Ingen team funnet.</BodyShort>}
          </section>
          <BodyShort>
            {data.small_teams} team med færre enn fem betalende brukere er skjult. Til sammen gjelder det{" "}
            {data.small_teams_users} ulike brukere og {dollars(net?.small_teams_net_usd ?? data.small_teams_gross_usd)}.
            Noen av dem kan også inngå i synlige team. Dette beløpet kan ikke legges til Navs regning.
          </BodyShort>
        </VStack>
      </section>
    </VStack>
  );
}
