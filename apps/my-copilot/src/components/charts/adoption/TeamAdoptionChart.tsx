"use client";

import type { TeamAdoption, AdoptionScope } from "@/lib/types";
import React, { useMemo, useState } from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading, HStack, VStack, ToggleGroup } from "@navikt/ds-react";
import { TooltipItem } from "chart.js";

// The API sends whole percent per team; the chart only picks and orders rows.
const rate = (t: TeamAdoption, scope: AdoptionScope) =>
  scope === "active" ? (t.adoption_active_pct ?? 0) : t.adoption_pct;
const repoCount = (t: TeamAdoption, scope: AdoptionScope) =>
  scope === "active" ? t.recently_active_repos : t.active_repos;

type ViewMode = "absolute" | "percentage";

interface TeamAdoptionChartProps {
  data: TeamAdoption[];
  maxTeams?: number;
}

const TeamAdoptionChart: React.FC<TeamAdoptionChartProps> = ({ data, maxTeams = 15 }) => {
  const [viewMode, setViewMode] = useState<ViewMode>("percentage");
  const [scope, setScope] = useState<AdoptionScope>("active");

  const topTeams = useMemo(
    () =>
      (data ?? [])
        .filter((t) => t.repos_with_customizations > 0 && (viewMode === "absolute" || repoCount(t, scope) > 0))
        .sort((a, b) =>
          viewMode === "percentage"
            ? rate(b, scope) - rate(a, scope)
            : b.repos_with_customizations - a.repos_with_customizations
        )
        .slice(0, maxTeams),
    [data, viewMode, scope, maxTeams]
  );

  if (!data || data.length === 0) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>
      </Box>
    );
  }

  if (topTeams.length === 0) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <Heading size="small" level="4">
          Team med flest tilpasninger
        </Heading>
        <div className="text-center text-gray-500">Ingen team har KI-tilpasninger ennå</div>
      </Box>
    );
  }

  const chartData = {
    labels: topTeams.map((t) => t.team_name || t.team_slug),
    datasets: [
      {
        data: topTeams.map((t) => {
          if (viewMode === "percentage") {
            return rate(t, scope);
          }
          return t.repos_with_customizations;
        }),
        backgroundColor: chartColors[1], // green
        borderRadius: 4,
        barThickness: 16,
      },
    ],
  };

  const options = {
    ...commonHorizontalBarOptions,
    scales: {
      ...commonHorizontalBarOptions.scales,
      x: {
        ...commonHorizontalBarOptions.scales?.x,
        ...(viewMode === "percentage" ? { max: 100 } : {}),
        ticks: {
          ...commonHorizontalBarOptions.scales?.x?.ticks,
          callback: (value: string | number) => (viewMode === "percentage" ? `${value}%` : value),
        },
      },
    },
    plugins: {
      ...commonHorizontalBarOptions.plugins,
      tooltip: {
        ...commonHorizontalBarOptions.plugins.tooltip,
        callbacks: {
          label: (context: TooltipItem<"bar">) => {
            const team = topTeams[context.dataIndex];
            const repos = repoCount(team, scope);
            const ratePercent = rate(team, scope);
            const repoLabel = scope === "active" ? "aktive repo (siste 90 dager)" : "aktive repo";
            return viewMode === "percentage"
              ? `${ratePercent}% (${team.repos_with_customizations} av ${repos} ${repoLabel})`
              : `${context.raw} repo med tilpasninger (${ratePercent}% av ${repos} ${repoLabel})`;
          },
        },
      },
    },
  };

  return (
    <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
      <VStack gap="space-16">
        <HStack justify="space-between" align="center" gap="space-8" wrap>
          <Heading size="small" level="4">
            {viewMode === "percentage" ? "Team med høyest adopsjonsrate" : "Team med flest tilpasninger"}
          </Heading>
          <HStack gap="space-8">
            <ToggleGroup size="small" value={scope} onChange={(val) => setScope(val as AdoptionScope)}>
              <ToggleGroup.Item value="active">Aktive repoer</ToggleGroup.Item>
              <ToggleGroup.Item value="all">Alle repoer</ToggleGroup.Item>
            </ToggleGroup>
            <ToggleGroup size="small" value={viewMode} onChange={(val) => setViewMode(val as ViewMode)}>
              <ToggleGroup.Item value="absolute">Antall</ToggleGroup.Item>
              <ToggleGroup.Item value="percentage">Prosent</ToggleGroup.Item>
            </ToggleGroup>
          </HStack>
        </HStack>
        <div style={{ height: Math.max(300, topTeams.length * 28) }}>
          <Bar data={chartData} options={options} />
        </div>
      </VStack>
    </Box>
  );
};

export default TeamAdoptionChart;
