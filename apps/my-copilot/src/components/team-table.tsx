"use client";

import { useState } from "react";
import { Pagination, Table } from "@navikt/ds-react";
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
      <Table size="small">
        <TableHeader>
          <TableRow>
            <TableHeaderCell>Team</TableHeaderCell>
            <TableHeaderCell align="right">Aktive repoer</TableHeaderCell>
            <TableHeaderCell align="right">Med tilpasninger</TableHeaderCell>
            <TableHeaderCell align="right">Adopsjonsrate</TableHeaderCell>
          </TableRow>
        </TableHeader>
        <TableBody>
          {pageTeams.map((team) => (
            <TableRow key={team.team_slug}>
              <TableDataCell>{team.team_name || team.team_slug}</TableDataCell>
              <TableDataCell align="right">{team.active_repos}</TableDataCell>
              <TableDataCell align="right">{team.repos_with_customizations}</TableDataCell>
              <TableDataCell align="right">{formatAdoptionRate(team.adoption_rate)}</TableDataCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {totalPages > 1 && (
        <div className="flex justify-center mt-4">
          <Pagination
            page={page}
            onPageChange={setPage}
            count={totalPages}
            size="small"
          />
        </div>
      )}
    </div>
  );
}
