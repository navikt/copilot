"use client";

import { useState } from "react";
import { HStack, Pagination, Table } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { sortTeamsByAdoption, formatAdoptionRate } from "@/lib/adoption-utils";
import type { TeamAdoption } from "@/lib/types";

const PAGE_SIZE = 15;

interface TeamTableProps {
  teams: TeamAdoption[];
}

export default function TeamTable({ teams }: TeamTableProps) {
  const [page, setPage] = useState(1);
  const sortedTeams = sortTeamsByAdoption(teams);
  const totalPages = Math.ceil(sortedTeams.length / PAGE_SIZE);
  const pageTeams = sortedTeams.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  return (
    <div>
      <div className="overflow-x-auto">
        <Table size="small">
          <TableHeader>
            <TableRow>
              <TableHeaderCell>Team</TableHeaderCell>
              <TableHeaderCell align="right">Aktive repoer</TableHeaderCell>
              <TableHeaderCell align="right">Nylig aktive</TableHeaderCell>
              <TableHeaderCell align="right">Med tilpasninger</TableHeaderCell>
              <TableHeaderCell align="right">Adopsjonsrate</TableHeaderCell>
              <TableHeaderCell align="right">Rate (aktive)</TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pageTeams.map((team) => (
              <TableRow key={team.team_slug}>
                <TableDataCell>{team.team_name || team.team_slug}</TableDataCell>
                <TableDataCell align="right">{team.active_repos}</TableDataCell>
                <TableDataCell align="right">{team.recently_active_repos}</TableDataCell>
                <TableDataCell align="right">{team.repos_with_customizations}</TableDataCell>
                <TableDataCell align="right">{formatAdoptionRate(team.adoption_rate)}</TableDataCell>
                <TableDataCell align="right">
                  {team.recently_active_repos > 0 ? formatAdoptionRate(team.adoption_rate_active_only) : "—"}
                </TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {totalPages > 1 && (
        <HStack justify="center" className="mt-(--a-spacing-16)">
          <Pagination page={page} onPageChange={setPage} count={totalPages} size="small" />
        </HStack>
      )}
    </div>
  );
}
