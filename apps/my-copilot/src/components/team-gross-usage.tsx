"use client";

import { useState } from "react";
import { BodyShort, Heading, Search, Table, VStack } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableRow } from "@navikt/ds-react/Table";
import type { TeamGrossOverview, TeamNetOverview } from "@/lib/types";

type Team = TeamGrossOverview["teams"][number] | TeamNetOverview["teams"][number];

export function compareTeamMonth(team: Team, previous: TeamGrossOverview | TeamNetOverview | null): number | null {
  const old = previous?.teams.find((row) => row.team_id === team.team_id);
  if (team.users < 5 || !old || old.users < 5) return null;
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
  comparisonReason = null,
}: {
  data: TeamGrossOverview;
  net: TeamNetOverview | null;
  myTeams: string[] | null;
  previous: TeamGrossOverview | TeamNetOverview | null;
  comparisonReason?: string | null;
}) {
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState("team");
  const [direction, setDirection] = useState<"ascending" | "descending">("ascending");
  const source = net?.teams ?? data.teams;
  const mine = new Set((myTeams ?? []).map((team) => team.toLowerCase()));
  const own = source.filter((team) => mine.has(team.team_slug.toLowerCase()));
  const others = source.filter((team) => !mine.has(team.team_slug.toLowerCase()));
  const filtered = (teams: Team[]) =>
    teams
      .filter((team) => team.team_slug.toLowerCase().includes(search.trim().toLowerCase()))
      .sort((a, b) => {
        const value = (team: Team) =>
          sortKey === "members"
            ? team.users
            : sortKey === "change"
              ? compareTeamMonth(team, previous)
              : sortKey === "average"
                ? amount(team) / team.users
                : amount(team);
        if (sortKey === "team")
          return (direction === "ascending" ? 1 : -1) * a.team_slug.localeCompare(b.team_slug, "nb");
        const av = value(a),
          bv = value(b);
        if (av === null) return bv === null ? a.team_slug.localeCompare(b.team_slug, "nb") : 1;
        if (bv === null) return -1;
        return (direction === "ascending" ? av - bv : bv - av) || a.team_slug.localeCompare(b.team_slug, "nb");
      });
  const dollars = (amount: number) =>
    new Intl.NumberFormat("nb-NO", { style: "currency", currency: "USD" }).format(amount);
  const amount = (team: Team) => ("net_usd" in team ? team.net_usd : team.gross_usd);
  const title = net ? "Forbruk" : "Forbruk før fradrag";

  const hidden = net ?? data;
  const rows = (teams: Team[]) =>
    filtered(teams).map((team) => {
      const change = compareTeamMonth(team, previous);
      const before = change === null ? null : amount(team) - change;
      const highlighted =
        change !== null &&
        before !== null &&
        Math.abs(change) >= 10 &&
        (before === 0 || Math.abs(change) / Math.abs(before) >= 0.1);
      return (
        <TableRow key={team.team_id}>
          <TableDataCell>{team.team_slug}</TableDataCell>
          <TableDataCell align="right">{team.users}</TableDataCell>
          <TableDataCell align="right">{dollars(amount(team))}</TableDataCell>
          <TableDataCell align="right">{dollars(amount(team) / team.users)}</TableDataCell>
          <TableDataCell align="right">
            <span
              title={highlighted ? "Endring på minst 10 % og 10 USD fra forrige måned" : undefined}
              className={
                highlighted
                  ? (change ?? 0) > 0
                    ? "text-[var(--ax-text-danger)]"
                    : "text-[var(--ax-text-success)]"
                  : undefined
              }
            >
              {change === null ? "—" : `${change >= 0 ? "+" : ""}${dollars(change)}`}
            </span>
          </TableDataCell>
        </TableRow>
      );
    });

  const table = (teams: Team[], label: string) => (
    <div className="overflow-x-auto">
      <Table
        size="small"
        aria-label={label}
        sort={{ orderBy: sortKey, direction }}
        onSortChange={(key) => {
          if (!key) return;
          setDirection(key === sortKey && direction === "ascending" ? "descending" : "ascending");
          setSortKey(key);
        }}
      >
        <TableHeader>
          <TableRow>
            <Table.ColumnHeader scope="col" sortable sortKey="team">
              Team
            </Table.ColumnHeader>
            <Table.ColumnHeader
              scope="col"
              align="right"
              sortable
              sortKey="members"
              title="Medlemmer med forbruk denne måneden"
            >
              Medlemmer
            </Table.ColumnHeader>
            <Table.ColumnHeader scope="col" align="right" sortable sortKey="amount">
              {title}
            </Table.ColumnHeader>
            <Table.ColumnHeader scope="col" align="right" sortable sortKey="average">
              Per medlem
            </Table.ColumnHeader>
            <Table.ColumnHeader scope="col" align="right" sortable sortKey="change">
              Endring
            </Table.ColumnHeader>
          </TableRow>
        </TableHeader>
        <TableBody>{rows(teams)}</TableBody>
      </Table>
    </div>
  );

  return (
    <VStack gap="space-32">
      <section aria-labelledby="om-kostnadene">
        <VStack gap="space-8">
          <Heading id="om-kostnadene" level="3" size="small">
            Om kostnadene
          </Heading>
          <BodyShort>
            Forbruk er summen for teamets medlemmer. Per medlem er gjennomsnittet blant medlemmer med forbruk. Medlemmer
            i flere team telles i hvert team. Teambeløpene kan derfor ikke summeres til Navs totale kostnad.
            Lisensutgifter er ikke med.
          </BodyShort>
          {!net && (
            <BodyShort>Beløpene er før fradrag. Fakturert forbruk er ikke tilgjengelig for denne måneden.</BodyShort>
          )}
          {comparisonReason && <BodyShort>{comparisonReason}</BodyShort>}
        </VStack>
      </section>

      <section aria-labelledby="medlemsbruk">
        <VStack gap="space-16">
          <Heading id="medlemsbruk" level="3" size="small">
            Forbruk per team
          </Heading>
          <Search label="Søk etter team" value={search} onChange={setSearch} size="small" className="max-w-xs" />
          {myTeams === null && <BodyShort>Kunne ikke finne dine team. Du kan fortsatt søke i teamlisten.</BodyShort>}
          {myTeams !== null && (
            <section aria-labelledby="mine-team">
              <Heading id="mine-team" level="4" size="xsmall" spacing>
                Mine team
              </Heading>
              {own.length ? (
                filtered(own).length ? (
                  table(own, "Mine team")
                ) : (
                  <BodyShort>Ingen av dine team passer søket.</BodyShort>
                )
              ) : (
                <BodyShort>Ingen av dine team vises denne måneden.</BodyShort>
              )}
            </section>
          )}
          <section aria-labelledby="andre-team">
            <Heading id="andre-team" level="4" size="xsmall" spacing>
              Andre team
            </Heading>
            {filtered(others).length ? table(others, "Andre team") : <BodyShort>Ingen team funnet.</BodyShort>}
          </section>
          <BodyShort>{hidden.small_teams} team med færre enn fem medlemmer med forbruk er skjult.</BodyShort>
        </VStack>
      </section>
    </VStack>
  );
}
