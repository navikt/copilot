"use client";

import type { TeamAdoption } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { seriesColor, commonHorizontalBarOptions } from "@/lib/chart-utils";
import { BodyShort, Box, Heading, VStack } from "@navikt/ds-react";
import { TooltipItem } from "chart.js";
import { formatPercent } from "@/lib/format";

// The API sends whole percent per team. Both shares are drawn side by side; the counts are in the table below.
const activeRate = (t: TeamAdoption) => t.adoption_active_pct ?? 0;

interface TeamAdoptionChartProps {
  data: TeamAdoption[];
  maxTeams?: number;
}

const TeamAdoptionChart: React.FC<TeamAdoptionChartProps> = ({ data, maxTeams = 15 }) => {
  const topTeams = data
    .filter((t) => t.repos_with_customizations > 0)
    .sort((a, b) => activeRate(b) - activeRate(a) || b.adoption_pct - a.adoption_pct)
    .slice(0, maxTeams);

  if (topTeams.length === 0) return <BodyShort>Ingen team har KI-tilpasninger ennå.</BodyShort>;

  const chartData = {
    labels: topTeams.map((t) => t.team_name || t.team_slug),
    datasets: [
      {
        label: "Aktive repoer",
        data: topTeams.map(activeRate),
        backgroundColor: seriesColor(1),
        borderRadius: 4,
        barThickness: 12,
      },
      {
        label: "Alle repoer",
        data: topTeams.map((t) => t.adoption_pct),
        backgroundColor: seriesColor(0),
        borderRadius: 4,
        barThickness: 12,
      },
    ],
  };

  const options = {
    ...commonHorizontalBarOptions,
    scales: {
      ...commonHorizontalBarOptions.scales,
      x: {
        ...commonHorizontalBarOptions.scales.x,
        max: 100,
        ticks: { ...commonHorizontalBarOptions.scales.x.ticks, callback: (v: string | number) => formatPercent(+v) },
      },
    },
    plugins: {
      ...commonHorizontalBarOptions.plugins,
      legend: { display: true, position: "bottom" as const },
      tooltip: {
        ...commonHorizontalBarOptions.plugins.tooltip,
        callbacks: {
          label: (context: TooltipItem<"bar">) => {
            const team = topTeams[context.dataIndex];
            const active = context.datasetIndex === 0;
            const repos = active ? team.recently_active_repos : team.active_repos;
            return `${context.dataset.label}: ${formatPercent(context.raw as number)} av ${repos} repoer`;
          },
        },
      },
    },
  };

  return (
    <Box padding="space-16" borderRadius="8" borderWidth="1" borderColor="neutral-subtle" background="default">
      <VStack gap="space-16">
        <Heading size="small" level="3">
          Team med høyest andel aktive repoer med tilpasninger
        </Heading>
        <div style={{ height: Math.max(300, topTeams.length * 40) }}>
          <Bar data={chartData} options={options} />
        </div>
      </VStack>
    </Box>
  );
};

export default TeamAdoptionChart;
