"use client";

import { useState, useMemo, useCallback } from "react";
import {
  HStack,
  Pagination,
  Table,
  Search,
  Alert,
  VStack,
  BodyShort,
  Box,
  HGrid,
  Heading,
  Button,
} from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableRow } from "@navikt/ds-react/Table";
import type { TeamUsageSummary, UserMetricsSummary } from "@/lib/types";
import { formatNumber } from "@/lib/format";

function CopyJsonButton({ data, label = "Kopier JSON" }: { data: unknown; label?: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(JSON.stringify(data, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [data]);
  return (
    <Button variant="tertiary-neutral" size="xsmall" onClick={handleCopy}>
      {copied ? "✓ Kopiert" : label}
    </Button>
  );
}

const PAGE_SIZE = 15;

type SortKey = "team_slug" | "avg_active_users" | "total_acceptances" | "total_interactions" | "total_lines_accepted";

interface TeamUsageTableProps {
  teams: TeamUsageSummary[];
  userTeams?: string[];
  userMetrics?: UserMetricsSummary | null;
}

export default function TeamUsageTable({ teams, userTeams = [], userMetrics }: TeamUsageTableProps) {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("avg_active_users");
  const [sortDirection, setSortDirection] = useState<"ascending" | "descending">("descending");

  const userTeamSet = useMemo(() => new Set(userTeams.map((t) => t.toLowerCase())), [userTeams]);

  const filteredTeams = useMemo(() => {
    if (!search.trim()) return teams;
    const query = search.toLowerCase();
    return teams.filter((t) => t.team_slug.toLowerCase().includes(query));
  }, [teams, search]);

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
      {/* Personal stats card */}
      {userMetrics && (
        <Box background="info-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-12">
            <HStack justify="space-between" align="center">
              <Heading size="xsmall" level="3">
                Din bruk siste 7 dager
              </Heading>
              <CopyJsonButton data={userMetrics} label="📋 JSON" />
            </HStack>
            {/* Activity overview */}
            <HGrid columns={{ xs: 2, sm: 3, md: 6 }} gap="space-8">
              <div className="text-center">
                <div className="text-lg font-semibold">{userMetrics.active_days}</div>
                <BodyShort size="small" className="text-gray-600">
                  Aktive dager
                </BodyShort>
              </div>
              <div className="text-center">
                <div className="text-lg font-semibold">{formatNumber(userMetrics.total_interactions)}</div>
                <BodyShort size="small" className="text-gray-600">
                  Forespørsler
                </BodyShort>
              </div>
              <div className="text-center">
                <div className="text-lg font-semibold">{formatNumber(userMetrics.total_acceptances)}</div>
                <BodyShort size="small" className="text-gray-600">
                  Aksepterte forslag
                </BodyShort>
              </div>
              <div className="text-center">
                <div className="text-lg font-semibold">{formatNumber(userMetrics.total_lines_accepted)}</div>
                <BodyShort size="small" className="text-gray-600">
                  Linjer lagt til
                </BodyShort>
              </div>
              <div className="text-center">
                <div className="text-lg font-semibold">{formatNumber(userMetrics.total_lines_deleted)}</div>
                <BodyShort size="small" className="text-gray-600">
                  Linjer slettet
                </BodyShort>
              </div>
              <div className="text-center">
                <div className="text-lg font-semibold">{formatNumber(userMetrics.total_lines_suggested)}</div>
                <BodyShort size="small" className="text-gray-600">
                  Linjer foreslått
                </BodyShort>
              </div>
            </HGrid>
            {/* Chat requests by mode */}
            {(userMetrics.chat_agent_requests > 0 ||
              userMetrics.chat_ask_requests > 0 ||
              userMetrics.chat_edit_requests > 0 ||
              userMetrics.chat_plan_requests > 0) && (
              <div>
                <BodyShort size="small" className="text-gray-600 mb-1">
                  Chat-forespørsler per modus
                </BodyShort>
                <HGrid columns={{ xs: 2, sm: 4 }} gap="space-8">
                  {userMetrics.chat_agent_requests > 0 && (
                    <div className="text-center">
                      <div className="text-sm font-semibold">{formatNumber(userMetrics.chat_agent_requests)}</div>
                      <BodyShort size="small" className="text-gray-500">
                        Agent
                      </BodyShort>
                    </div>
                  )}
                  {userMetrics.chat_ask_requests > 0 && (
                    <div className="text-center">
                      <div className="text-sm font-semibold">{formatNumber(userMetrics.chat_ask_requests)}</div>
                      <BodyShort size="small" className="text-gray-500">
                        Ask
                      </BodyShort>
                    </div>
                  )}
                  {userMetrics.chat_edit_requests > 0 && (
                    <div className="text-center">
                      <div className="text-sm font-semibold">{formatNumber(userMetrics.chat_edit_requests)}</div>
                      <BodyShort size="small" className="text-gray-500">
                        Edit
                      </BodyShort>
                    </div>
                  )}
                  {userMetrics.chat_plan_requests > 0 && (
                    <div className="text-center">
                      <div className="text-sm font-semibold">{formatNumber(userMetrics.chat_plan_requests)}</div>
                      <BodyShort size="small" className="text-gray-500">
                        Plan
                      </BodyShort>
                    </div>
                  )}
                </HGrid>
              </div>
            )}
            {/* CLI token usage */}
            {userMetrics.cli_total_requests > 0 && (
              <div>
                <BodyShort size="small" className="text-gray-600 mb-1">
                  CLI-bruk
                </BodyShort>
                <HGrid columns={{ xs: 3 }} gap="space-8">
                  <div className="text-center">
                    <div className="text-sm font-semibold">{formatNumber(userMetrics.cli_total_requests)}</div>
                    <BodyShort size="small" className="text-gray-500">
                      Forespørsler
                    </BodyShort>
                  </div>
                  <div className="text-center">
                    <div className="text-sm font-semibold">{formatNumber(userMetrics.cli_prompt_tokens)}</div>
                    <BodyShort size="small" className="text-gray-500">
                      Prompt-tokens
                    </BodyShort>
                  </div>
                  <div className="text-center">
                    <div className="text-sm font-semibold">{formatNumber(userMetrics.cli_output_tokens)}</div>
                    <BodyShort size="small" className="text-gray-500">
                      Output-tokens
                    </BodyShort>
                  </div>
                </HGrid>
              </div>
            )}
            {userTeams.length > 0 && (
              <BodyShort size="small" className="text-gray-600">
                Dine team: {userTeams.join(", ")}
              </BodyShort>
            )}
          </VStack>
        </Box>
      )}

      <Alert variant="info" size="small">
        Team med færre enn 5 Copilot-brukere vises ikke (GitHub-begrensning). Data tilgjengelig fra 15. mai 2026.
      </Alert>

      <HStack gap="space-8" align="end">
        <Search
          label="Finn teamet ditt"
          size="small"
          variant="simple"
          value={search}
          onChange={handleSearch}
          className="max-w-xs"
        />
        <CopyJsonButton data={sortedTeams} label="📋 Teams JSON" />
      </HStack>

      <div className="overflow-x-auto">
        <Table size="small" sort={{ orderBy: sortKey, direction: sortDirection }} onSortChange={handleSort}>
          <TableHeader>
            <TableRow>
              <Table.ColumnHeader scope="col" sortKey="team_slug" sortable>
                Team
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="avg_active_users" sortable align="right">
                Aktive brukere
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="total_acceptances" sortable align="right">
                Aksepterte kodeforslag
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="total_interactions" sortable align="right">
                Interaksjoner
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="total_lines_accepted" sortable align="right">
                Linjer akseptert
              </Table.ColumnHeader>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pageTeams.map((team) => {
              const isUserTeam = userTeamSet.has(team.team_slug.toLowerCase());
              return (
                <TableRow key={team.team_slug} className={isUserTeam ? "bg-blue-50 font-medium" : ""}>
                  <TableDataCell>
                    {team.team_slug}
                    {isUserTeam && " ⭐"}
                  </TableDataCell>
                  <TableDataCell align="right">
                    {team.avg_active_users} av {team.total_users}
                  </TableDataCell>
                  <TableDataCell align="right">{formatNumber(team.total_acceptances)}</TableDataCell>
                  <TableDataCell align="right">{formatNumber(team.total_interactions)}</TableDataCell>
                  <TableDataCell align="right">{formatNumber(team.total_lines_accepted)}</TableDataCell>
                </TableRow>
              );
            })}
            {pageTeams.length === 0 && (
              <TableRow>
                <TableDataCell colSpan={5}>
                  <BodyShort className="text-gray-500 py-4 text-center">
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
