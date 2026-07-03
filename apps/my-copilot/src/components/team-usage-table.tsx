"use client";

import { useState, useMemo, useCallback } from "react";
import { HStack, Pagination, Table, Search, Alert, VStack, BodyShort, Button, ToggleGroup } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableRow } from "@navikt/ds-react/Table";
import type { TeamUsageSummary } from "@/lib/types";
import { formatNumber } from "@/lib/format";

function CopyJsonButton({ data, label = "Kopier JSON" }: { data: unknown; label?: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(JSON.stringify(data, null, 2)).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [data]);
  return (
    <Button variant="tertiary-neutral" size="xsmall" onClick={handleCopy}>
      {copied ? "✓ Kopiert" : label}
    </Button>
  );
}

const PAGE_SIZE = 15;

type SortKey = "team_slug" | "avg_active_users" | "total_acceptances" | "total_interactions" | "agent_users";

interface TeamUsageTableProps {
  teams: TeamUsageSummary[];
  userTeams?: string[];
  allowAllTeams?: boolean;
}

export default function TeamUsageTable({ teams, userTeams = [], allowAllTeams = false }: TeamUsageTableProps) {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [showMyTeams, setShowMyTeams] = useState(!allowAllTeams);
  const [sortKey, setSortKey] = useState<SortKey>("total_interactions");
  const [sortDirection, setSortDirection] = useState<"ascending" | "descending">("descending");

  const userTeamSet = useMemo(() => new Set(userTeams.map((t) => t.toLowerCase())), [userTeams]);

  const filteredTeams = useMemo(() => {
    let result = teams;
    if (showMyTeams) {
      result = result.filter((t) => userTeamSet.has(t.team_slug.toLowerCase()));
    }
    if (search.trim()) {
      const query = search.toLowerCase();
      result = result.filter((t) => t.team_slug.toLowerCase().includes(query));
    }
    return result;
  }, [teams, search, showMyTeams, userTeamSet]);

  const sortedTeams = useMemo(() => {
    return [...filteredTeams].sort((a, b) => {
      const aVal = a[sortKey];
      const bVal = b[sortKey];
      if (typeof aVal === "string" && typeof bVal === "string") {
        return sortDirection === "ascending" ? aVal.localeCompare(bVal) : bVal.localeCompare(aVal);
      }
      const diff = (aVal as number) - (bVal as number);
      return sortDirection === "ascending" ? diff : -diff;
    });
  }, [filteredTeams, sortKey, sortDirection]);

  const totalPages = Math.ceil(sortedTeams.length / PAGE_SIZE);
  const pageTeams = sortedTeams.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  function handleSort(key: string) {
    if (sortKey === key) {
      setSortDirection((d) => (d === "ascending" ? "descending" : "ascending"));
    } else {
      setSortKey(key as SortKey);
      setSortDirection("descending");
    }
    setPage(1);
  }

  function handleSearch(value: string) {
    setSearch(value);
    setPage(1);
  }

  return (
    <VStack gap="space-16">
      <Alert variant="info" size="small">
        Team med færre enn 5 Copilot-brukere vises ikke (GitHub-begrensning).
      </Alert>

      <HStack gap="space-8" align="end" wrap>
        {allowAllTeams && userTeams.length > 0 && (
          <ToggleGroup
            size="small"
            value={showMyTeams ? "mine" : "alle"}
            onChange={(val) => {
              setShowMyTeams(val === "mine");
              setPage(1);
            }}
          >
            <ToggleGroup.Item value="alle">Alle team</ToggleGroup.Item>
            <ToggleGroup.Item value="mine">Mine team</ToggleGroup.Item>
          </ToggleGroup>
        )}
        <Search label="Søk" size="small" variant="simple" value={search} onChange={handleSearch} className="max-w-xs" />
        <CopyJsonButton data={sortedTeams} label="📋 JSON" />
      </HStack>

      <div className="overflow-x-auto">
        <Table size="small" sort={{ orderBy: sortKey, direction: sortDirection }} onSortChange={handleSort}>
          <TableHeader>
            <TableRow>
              <Table.ColumnHeader scope="col" sortKey="team_slug" sortable>
                Team
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="avg_active_users" sortable align="right">
                Aktive
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="total_acceptances" sortable align="right">
                Forslag
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="total_interactions" sortable align="right">
                Chat
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="agent_users" sortable align="right">
                Agent
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col">Modeller</Table.ColumnHeader>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pageTeams.map((team) => {
              const isUserTeam = userTeamSet.has(team.team_slug.toLowerCase());
              const adoptionPct =
                team.total_users > 0 ? Math.round((team.avg_active_users / team.total_users) * 100) : 0;
              const topModels = team.top_models || [];
              const modelTotal = topModels.reduce((s, m) => s + m.interactions, 0);
              return (
                <TableRow key={team.team_slug} className={isUserTeam ? "bg-blue-50 font-medium" : ""}>
                  <TableDataCell>
                    {team.team_slug}
                    {isUserTeam && " ⭐"}
                  </TableDataCell>
                  <TableDataCell align="right">
                    {team.avg_active_users} / {team.total_users} ({adoptionPct} %)
                  </TableDataCell>
                  <TableDataCell align="right">{formatNumber(team.total_acceptances)}</TableDataCell>
                  <TableDataCell align="right">{formatNumber(team.total_interactions)}</TableDataCell>
                  <TableDataCell align="right">
                    {team.agent_users} / {team.avg_active_users}
                  </TableDataCell>
                  <TableDataCell>
                    {topModels.length > 0 ? (
                      <div className="flex gap-1 flex-wrap">
                        {topModels.map((m) => (
                          <span key={m.model} className="text-xs bg-gray-100 rounded px-1" title={m.model}>
                            {m.model.replace(/^Auto: /, "")} ({Math.round((m.interactions / modelTotal) * 100)}%)
                          </span>
                        ))}
                      </div>
                    ) : (
                      <span className="text-xs text-gray-400">–</span>
                    )}
                  </TableDataCell>
                </TableRow>
              );
            })}
            {pageTeams.length === 0 && (
              <TableRow>
                <TableDataCell colSpan={6}>
                  <BodyShort className="text-gray-500 text-center">
                    {search ? "Ingen team funnet for søket ditt." : "Ingen teamdata tilgjengelig ennå."}
                  </BodyShort>
                </TableDataCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {totalPages > 1 && (
        <HStack justify="center">
          <Pagination page={page} onPageChange={setPage} count={totalPages} size="small" />
        </HStack>
      )}
    </VStack>
  );
}
