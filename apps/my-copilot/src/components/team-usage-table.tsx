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
  userWeeklyTrends?: WeeklyTrend[] | null;
}

export default function TeamUsageTable({ teams, userTeams = [], userMetrics, userWeeklyTrends }: TeamUsageTableProps) {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [showMyTeams, setShowMyTeams] = useState(false);
  const [sortKey, setSortKey] = useState<SortKey>("avg_active_users");
  const [sortDirection, setSortDirection] = useState<"ascending" | "descending">("descending");

  const userTeamSet = useMemo(() => new Set(userTeams.map((t) => t.toLowerCase())), [userTeams]);

  // Compute comparison stats: how user's teams rank vs all teams
  const teamComparison = useMemo(() => {
    if (userTeams.length === 0 || teams.length < 3) return null;
    const myTeams = teams.filter((t) => userTeamSet.has(t.team_slug.toLowerCase()));
    if (myTeams.length === 0) return null;

    // Per-user activity rate (acceptances per active user per day)
    const activityRates = teams
      .filter((t) => t.avg_active_users > 0 && t.days_with_data > 0)
      .map((t) => t.total_acceptances / t.avg_active_users / t.days_with_data);
    activityRates.sort((a, b) => a - b);

    const bestTeam = myTeams.reduce((best, t) => {
      const rate =
        t.avg_active_users > 0 && t.days_with_data > 0
          ? t.total_acceptances / t.avg_active_users / t.days_with_data
          : 0;
      const bestRate =
        best.avg_active_users > 0 && best.days_with_data > 0
          ? best.total_acceptances / best.avg_active_users / best.days_with_data
          : 0;
      return rate > bestRate ? t : best;
    }, myTeams[0]);

    const bestRate =
      bestTeam.avg_active_users > 0 && bestTeam.days_with_data > 0
        ? bestTeam.total_acceptances / bestTeam.avg_active_users / bestTeam.days_with_data
        : 0;
    const rank = activityRates.filter((r) => r <= bestRate).length;
    const percentile = Math.round((rank / activityRates.length) * 100);

    // Adoption rate (active / total)
    const adoptionRate =
      bestTeam.total_users > 0 ? Math.round((bestTeam.avg_active_users / bestTeam.total_users) * 100) : 0;
    const orgMedianAdoption = (() => {
      const rates = teams.filter((t) => t.total_users > 0).map((t) => t.avg_active_users / t.total_users);
      rates.sort((a, b) => a - b);
      return rates.length > 0 ? Math.round(rates[Math.floor(rates.length / 2)] * 100) : 0;
    })();

    return { bestTeam: bestTeam.team_slug, percentile, adoptionRate, orgMedianAdoption };
  }, [teams, userTeams, userTeamSet]);

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
                userMetrics.chat_plan_requests > 0) && (
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
                    <div>
                      <div className="text-sm font-semibold">
                        {formatNumber(userMetrics.cli_total_requests)} forespørsler
                      </div>
                      <BodyShort size="small" className="text-gray-500">
                        {formatNumber(userMetrics.cli_prompt_tokens)} inn /{" "}
                        {formatNumber(userMetrics.cli_output_tokens)} ut tokens
                      </BodyShort>
                    </div>
                  </VStack>
                </Box>
              )}
            </HGrid>

            {/* Personal weekly trends */}
            {userWeeklyTrends && userWeeklyTrends.length > 1 && (
              <div>
                <BodyShort weight="semibold" size="small" className="mb-2">
                  Ukentlig aktivitet
                </BodyShort>
                <WeeklyTrendsChart data={userWeeklyTrends} />
              </div>
            )}

            {/* Team comparison */}
            {teamComparison && (
              <Box background="success-soft" padding="space-16" borderRadius="8">
                <VStack gap="space-8">
                  <BodyShort weight="semibold" size="small">
                    Ditt beste team: {teamComparison.bestTeam}
                  </BodyShort>
                  <HGrid columns={{ xs: 1, sm: 2 }} gap="space-8">
                    <div>
                      <div className="text-sm font-semibold">Topp {100 - teamComparison.percentile} %</div>
                      <BodyShort size="small" className="text-gray-500">
                        Aktivitet per bruker
                      </BodyShort>
                    </div>
                    <div>
                      <div className="text-sm font-semibold">{teamComparison.adoptionRate} % adopsjon</div>
                      <BodyShort size="small" className="text-gray-500">
                        Org-median: {teamComparison.orgMedianAdoption} %
                      </BodyShort>
                    </div>
                  </HGrid>
                </VStack>
              </Box>
            )}

            {userTeams.length > 0 && !teamComparison && (
              <BodyShort size="small" className="text-gray-500">
                Team: {userTeams.join(", ")}
              </BodyShort>
            )}
          </VStack>
        </Box>
      )}

      <Alert variant="info" size="small">
        Team med færre enn 5 Copilot-brukere vises ikke (GitHub-begrensning).
      </Alert>

      <HStack gap="space-8" align="end" wrap>
        {userTeams.length > 0 && (
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
