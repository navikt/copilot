"use client";

import type { TeamAdoption } from "@/lib/types";
import React, { useMemo, useState } from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading, HStack, ToggleGroup } from "@navikt/ds-react";
import { TooltipItem } from "chart.js";

type ViewMode = "absolute" | "percentage";

interface TeamAdoptionChartProps {
  data: TeamAdoption[];
  maxTeams?: number;
}

const TeamAdoptionChart: React.FC<TeamAdoptionChartProps> = ({ data, maxTeams = 15 }) => {
  const [viewMode, setViewMode] = useState<ViewMode>("percentage");

  const topTeams = useMemo(() => {
    if (!data || data.length === 0) return [];
    const filtered = data.filter((t) => t.repos_with_customizations > 0);
    if (viewMode === "percentage") {
      return filtered
        .filter((t) => t.active_repos > 0)
        .sort((a, b) => b.adoption_rate - a.adoption_rate)
        .slice(0, maxTeams);
    }
    return filtered
      .sort((a, b) => b.repos_with_customizations - a.repos_with_customizations)
      .slice(0, maxTeams);
  }, [data, viewMode, maxTeams]);

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
        <div className="text-center text-gray-500">Ingen team har AI-tilpasninger ennå</div>
      </Box>
    );
  }

  const chartData = {
    labels: topTeams.map((t) => t.team_name || t.team_slug),
    datasets: [
      {
        data: topTeams.map((t) =>
          viewMode === "percentage"
            ? Math.round(t.adoption_rate * 100)
            : t.repos_with_customizations,
        ),
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
          callback: (value: string | number) =>
            viewMode === "percentage" ? `${value}%` : value,
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
            const rate = Math.round(team.adoption_rate * 100);
            return viewMode === "percentage"
              ? `${rate}% (${team.repos_with_customizations} av ${team.active_repos} aktive repo)`
              : `${context.raw} repo med tilpasninger (${rate}% av ${team.active_repos} aktive)`;
          },
        },
      },
    },
  };

  return (
    <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
      <HStack justify="space-between" align="center" gap="space-8" className="mb-[--a-spacing-16]">
        <Heading size="small" level="4">
          {viewMode === "percentage" ? "Team med høyest adopsjonsrate" : "Team med flest tilpasninger"}
        </Heading>
        <ToggleGroup
          size="small"
          value={viewMode}
          onChange={(val) => setViewMode(val as ViewMode)}
        >
          <ToggleGroup.Item value="absolute">Antall</ToggleGroup.Item>
          <ToggleGroup.Item value="percentage">Prosent</ToggleGroup.Item>
        </ToggleGroup>
      </HStack>
      <div style={{ height: Math.max(300, topTeams.length * 28) }}>
        <Bar data={chartData} options={options} />
      </div>
    </Box>
  );
};

export default TeamAdoptionChart;
