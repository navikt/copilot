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
  ToggleGroup,
} from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableRow } from "@navikt/ds-react/Table";
import type { TeamUsageSummary, UserMetricsSummary, WeeklyTrend } from "@/lib/types";
import { formatNumber } from "@/lib/format";
import WeeklyTrendsChart from "@/components/charts/WeeklyTrendsChart";

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
  userMetrics?: UserMetricsSummary | null;
  userWeeklyTrends?: WeeklyTrend[] | null;
  allowAllTeams?: boolean;
}

export default function TeamUsageTable({
  teams,
  userTeams = [],
  userMetrics,
  userWeeklyTrends,
  allowAllTeams = false,
}: TeamUsageTableProps) {
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
      {/* Personal stats card */}
      {userMetrics && (
        <Box background="neutral-soft" padding="space-24" borderRadius="12">
          <VStack gap="space-16">
            <HStack justify="space-between" align="center">
              <div>
                <Heading size="small" level="3">
                  Din bruk
                </Heading>
                <BodyShort size="small" className="text-gray-600">
                  Siste {userMetrics.days_in_period} dager ({userMetrics.active_days} aktive)
                </BodyShort>
              </div>
              <CopyJsonButton data={userMetrics} label="📋 JSON" />
            </HStack>

            {/* Primary metrics */}
            <HGrid columns={{ xs: 2, sm: 4 }} gap="space-16">
              <Box background="info-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(userMetrics.total_interactions + userMetrics.cli_total_requests)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Forespørsler
                  </BodyShort>
                </div>
              </Box>
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(userMetrics.total_acceptances)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Aksepterte forslag
                  </BodyShort>
                </div>
              </Box>
              <Box background="accent-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(userMetrics.total_lines_accepted)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Linjer lagt til
                  </BodyShort>
                </div>
              </Box>
              <Box background="warning-soft" padding="space-16" borderRadius="8">
                <div className="text-center">
                  <Heading size="large" level="4">
                    {formatNumber(userMetrics.total_lines_deleted)}
                  </Heading>
                  <BodyShort size="small" className="text-gray-600">
                    Linjer slettet
                  </BodyShort>
                </div>
              </Box>
            </HGrid>

            {/* Feature usage breakdown */}
            <HGrid columns={{ xs: 1, sm: 2 }} gap="space-16">
              {/* Chat modes */}
              {(userMetrics.chat_agent_requests > 0 ||
                userMetrics.chat_ask_requests > 0 ||
                userMetrics.chat_edit_requests > 0 ||
                userMetrics.chat_plan_requests > 0 ||
                userMetrics.chat_custom_requests > 0) && (
                <Box background="info-soft" padding="space-16" borderRadius="8">
                  <VStack gap="space-8">
                    <BodyShort weight="semibold" size="small">
                      IDE-chat
                    </BodyShort>
                    <HGrid columns={2} gap="space-8">
                      {userMetrics.chat_agent_requests > 0 && (
                        <div>
                          <div className="text-sm font-semibold">{formatNumber(userMetrics.chat_agent_requests)}</div>
                          <BodyShort size="small" className="text-gray-500">
                            Agent
                          </BodyShort>
                        </div>
                      )}
                      {userMetrics.chat_ask_requests > 0 && (
                        <div>
                          <div className="text-sm font-semibold">{formatNumber(userMetrics.chat_ask_requests)}</div>
                          <BodyShort size="small" className="text-gray-500">
                            Ask
                          </BodyShort>
                        </div>
                      )}
                      {userMetrics.chat_edit_requests > 0 && (
                        <div>
                          <div className="text-sm font-semibold">{formatNumber(userMetrics.chat_edit_requests)}</div>
                          <BodyShort size="small" className="text-gray-500">
                            Edit
                          </BodyShort>
                        </div>
                      )}
                      {userMetrics.chat_plan_requests > 0 && (
                        <div>
                          <div className="text-sm font-semibold">{formatNumber(userMetrics.chat_plan_requests)}</div>
                          <BodyShort size="small" className="text-gray-500">
                            Plan
                          </BodyShort>
                        </div>
                      )}
                      {userMetrics.chat_custom_requests > 0 && (
                        <div>
                          <div className="text-sm font-semibold">{formatNumber(userMetrics.chat_custom_requests)}</div>
                          <BodyShort size="small" className="text-gray-500">
                            Custom agent
                          </BodyShort>
                        </div>
                      )}
                    </HGrid>
                  </VStack>
                </Box>
              )}
              {/* CLI */}
              {userMetrics.cli_total_requests > 0 && (
                <Box background="accent-soft" padding="space-16" borderRadius="8">
                  <VStack gap="space-8">
                    <BodyShort weight="semibold" size="small">
                      CLI
                    </BodyShort>
                    <HGrid columns={2} gap="space-8">
                      <div>
                        <div className="text-sm font-semibold">{formatNumber(userMetrics.cli_prompts)}</div>
                        <BodyShort size="small" className="text-gray-500">
                          Spørringer
                        </BodyShort>
                      </div>
                      <div>
                        <div className="text-sm font-semibold">{formatNumber(userMetrics.cli_sessions)}</div>
                        <BodyShort size="small" className="text-gray-500">
                          Økter
                        </BodyShort>
                      </div>
                    </HGrid>
                    <BodyShort size="small" className="text-gray-500">
                      {formatNumber(userMetrics.cli_prompt_tokens)} inn / {formatNumber(userMetrics.cli_output_tokens)}{" "}
                      ut tokens
                    </BodyShort>
                  </VStack>
                </Box>
              )}
              {/* Code Review */}
              {userMetrics.days_used_code_review > 0 && (
                <Box background="success-soft" padding="space-16" borderRadius="8">
                  <VStack gap="space-8">
                    <BodyShort weight="semibold" size="small">
                      Code review
                    </BodyShort>
                    <div>
                      <div className="text-sm font-semibold">{userMetrics.days_used_code_review} dager</div>
                      <BodyShort size="small" className="text-gray-500">
                        Brukt Copilot code review
                      </BodyShort>
                    </div>
                  </VStack>
                </Box>
              )}
            </HGrid>

            {/* Model usage breakdown */}
            {userMetrics.top_models && userMetrics.top_models.length > 0 && (
              <Box background="neutral-soft" padding="space-16" borderRadius="8">
                <VStack gap="space-8">
                  <BodyShort weight="semibold" size="small">
                    AI-modeller i bruk
                  </BodyShort>
                  <VStack gap="space-4">
                    {userMetrics.top_models.map((m) => {
                      const totalInteractions = userMetrics.top_models.reduce((s, x) => s + x.interactions, 0);
                      const pct = totalInteractions > 0 ? Math.round((m.interactions / totalInteractions) * 100) : 0;
                      return (
                        <div key={m.model} className="flex items-center gap-2">
                          <div className="flex-1 min-w-0">
                            <div className="flex justify-between items-baseline">
                              <BodyShort size="small" className="truncate">
                                {m.model}
                              </BodyShort>
                              <BodyShort size="small" className="text-gray-500 ml-2 shrink-0">
                                {pct} %
                              </BodyShort>
                            </div>
                            <div className="h-1.5 bg-gray-200 rounded-full mt-0.5">
                              <div className="h-full bg-blue-500 rounded-full" style={{ width: `${pct}%` }} />
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </VStack>
                </VStack>
              </Box>
            )}

            {/* Personal weekly trends */}
            {userWeeklyTrends && userWeeklyTrends.length > 1 && (
              <div>
                <BodyShort weight="semibold" size="small">
                  Ukentlig aktivitet
                </BodyShort>
                <WeeklyTrendsChart data={userWeeklyTrends} />
              </div>
            )}
          </VStack>
        </Box>
      )}

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
                Aktive / totalt
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="total_acceptances" sortable align="right">
                Aksepterte forslag
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="total_interactions" sortable align="right">
                Chat + agent
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="agent_users" sortable align="right">
                Agent-brukere
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col">Modellmiks</Table.ColumnHeader>
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
