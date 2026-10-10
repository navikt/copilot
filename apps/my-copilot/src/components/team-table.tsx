"use client";

import { useState } from "react";
import { HStack, Pagination, Table, VStack } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import type { TeamAdoption } from "@/lib/types";
import { formatPercent } from "@/lib/format";

const PAGE_SIZE = 15;

interface TeamTableProps {
  teams: TeamAdoption[];
}

export default function TeamTable({ teams }: TeamTableProps) {
  const [page, setPage] = useState(1);
  const totalPages = Math.ceil(teams.length / PAGE_SIZE);
  const pageTeams = teams.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  return (
    <VStack gap="space-16">
      <div className="overflow-x-auto">
        <Table size="small" aria-label="Tilpasninger per team">
          <TableHeader>
            <TableRow>
              <TableHeaderCell>Team</TableHeaderCell>
              <TableHeaderCell align="right">Repoer</TableHeaderCell>
              <TableHeaderCell align="right">Aktive repoer</TableHeaderCell>
              <TableHeaderCell align="right">Med tilpasninger</TableHeaderCell>
              <TableHeaderCell align="right">Andel av alle</TableHeaderCell>
              <TableHeaderCell align="right">Andel av aktive</TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pageTeams.map((team) => (
              <TableRow key={team.team_slug}>
                <TableDataCell>{team.team_name || team.team_slug}</TableDataCell>
                <TableDataCell align="right">{team.active_repos}</TableDataCell>
                <TableDataCell align="right">{team.recently_active_repos}</TableDataCell>
                <TableDataCell align="right">{team.repos_with_customizations}</TableDataCell>
                <TableDataCell align="right">{formatPercent(team.adoption_pct)}</TableDataCell>
                <TableDataCell align="right">
                  {team.adoption_active_pct === null ? "—" : formatPercent(team.adoption_active_pct)}
                </TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {totalPages > 1 && (
        <HStack justify="center">
          <Pagination page={page} onPageChange={setPage} count={totalPages} size="small" siblingCount={0} />
        </HStack>
      )}
    </VStack>
  );
}
