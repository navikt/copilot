"use client";

import { useState } from "react";
import { BodyShort, Heading, HStack, Table, VStack } from "@navikt/ds-react";
import { Buildings3Icon, CodeIcon, CpuIcon, WrenchIcon } from "@navikt/aksel-icons";
import { TableBody, TableDataCell, TableHeader, TableRow } from "@navikt/ds-react/Table";
import type { TeamGrossOverview, TeamNetOverview } from "@/lib/types";
import { useTeamControls } from "./team-controls";
import TeamUsageValue from "./team-usage-value";

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
  const { search, columns } = useTeamControls();
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
          {(["providers", "categories"] as const).map(
            (column) =>
              columns.includes(column) && (
                <TableDataCell key={column}>
                  <VStack gap="space-4">
                    {data.usage?.[team.team_id]?.[column]?.length
                      ? data.usage[team.team_id][column].map((value) => (
                          <TeamUsageValue
                            key={value}
                            kind={column === "providers" ? "provider" : "category"}
                            value={value}
                          />
                        ))
                      : Array.isArray(data.usage?.[team.team_id]?.[column])
                        ? "Ingen modellinteraksjoner"
                        : "Ikke tilgjengelig"}
                  </VStack>
                </TableDataCell>
              )
          )}
          {columns.includes("feature") && (
            <TableDataCell>
              {data.usage?.[team.team_id]?.feature ? (
                <TeamUsageValue kind="feature" value={data.usage[team.team_id].feature} />
              ) : (
                "—"
              )}
            </TableDataCell>
          )}
          {columns.includes("language") && (
            <TableDataCell>
              {data.usage?.[team.team_id]?.language ? (
                <TeamUsageValue kind="language" value={data.usage[team.team_id].language} />
              ) : (
                "—"
              )}
            </TableDataCell>
          )}
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
            {columns.includes("providers") && (
              <Table.ColumnHeader scope="col">
                <HStack gap="space-8" align="center">
                  <Buildings3Icon aria-hidden fontSize="1.25rem" />
                  Leverandører
                </HStack>
              </Table.ColumnHeader>
            )}
            {columns.includes("categories") && (
              <Table.ColumnHeader scope="col">
                <HStack gap="space-8" align="center">
                  <CpuIcon aria-hidden fontSize="1.25rem" />
                  Modelltyper
                </HStack>
              </Table.ColumnHeader>
            )}
            {columns.includes("feature") && (
              <Table.ColumnHeader scope="col">
                <HStack gap="space-8" align="center">
                  <WrenchIcon aria-hidden fontSize="1.25rem" />
                  Funksjon
                </HStack>
              </Table.ColumnHeader>
            )}
            {columns.includes("language") && (
              <Table.ColumnHeader scope="col">
                <HStack gap="space-8" align="center">
                  <CodeIcon aria-hidden fontSize="1.25rem" />
                  Språk
                </HStack>
              </Table.ColumnHeader>
            )}
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
          {net && (
            <BodyShort>Nettobeløpene er innsamlet {net.loaded_at}. Senere fakturakorreksjoner er ikke med.</BodyShort>
          )}
          {comparisonReason && <BodyShort>{comparisonReason}</BodyShort>}
        </VStack>
      </section>

      <section aria-labelledby="medlemsbruk">
        <VStack gap="space-16">
          <Heading id="medlemsbruk" level="3" size="small">
            Forbruk per team
          </Heading>
          {columns.length > 0 && (
            <BodyShort>
              Leverandører og modelltyper vises for alle synlige team, rangert etter antall brukerinteraksjoner uten
              kostnadsvekting. Uklassifisert betyr at leverandøren eller modelltypen ikke er kjent. Funksjon og språk
              vises bare med minst fem bidragsytere. Funksjon rangeres etter brukerinteraksjoner, språk etter
              kodegenereringer. En strek betyr at data mangler, eller at funksjon eller språk er skjult. Dette er
              bruksmønster, ikke kostnadsfordeling.
            </BodyShort>
          )}
          {columns.length > 0 && !data.usage && (
            <BodyShort>Bruksmønster er ikke tilgjengelig fra datatjenesten. Prøv igjen senere.</BodyShort>
          )}
          {data.usage &&
            (["providers", "categories"] as const).some(
              (column) =>
                columns.includes(column) &&
                filtered(source).some((team) => !Array.isArray(data.usage?.[team.team_id]?.[column]))
            ) && <BodyShort>Leverandør- og modelltypeoversikt mangler for noen team fra datatjenesten.</BodyShort>}
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
