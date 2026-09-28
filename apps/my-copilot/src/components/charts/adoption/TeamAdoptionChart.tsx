"use client";

import type { TeamAdoption, AdoptionScope } from "@/lib/types";
import React, { useMemo, useState } from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading, HStack, VStack, ToggleGroup } from "@navikt/ds-react";
import { TooltipItem } from "chart.js";
import { getTopTeamsForChart, getTeamAdoptionRate, getTeamRepoCount } from "@/lib/adoption-utils";

type ViewMode = "absolute" | "percentage";

interface TeamAdoptionChartProps {
  data: TeamAdoption[];
  maxTeams?: number;
}

const TeamAdoptionChart: React.FC<TeamAdoptionChartProps> = ({ data, maxTeams = 15 }) => {
  const [viewMode, setViewMode] = useState<ViewMode>("percentage");
  const [scope, setScope] = useState<AdoptionScope>("active");

  const topTeams = useMemo(
    () => getTopTeamsForChart(data ?? [], scope, viewMode, maxTeams),
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
            return Math.round(getTeamAdoptionRate(t, scope) * 100);
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
            const repoCount = getTeamRepoCount(team, scope);
            const rate = getTeamAdoptionRate(team, scope);
            const ratePercent = Math.round(rate * 100);
            const repoLabel = scope === "active" ? "aktive repo (siste 90 dager)" : "aktive repo";
            return viewMode === "percentage"
              ? `${ratePercent}% (${team.repos_with_customizations} av ${repoCount} ${repoLabel})`
              : `${context.raw} repo med tilpasninger (${ratePercent}% av ${repoCount} ${repoLabel})`;
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
