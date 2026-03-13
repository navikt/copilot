"use client";

import type { TeamAdoption } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, chartWrapperClass, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Heading } from "@navikt/ds-react";
import { TooltipItem } from "chart.js";

interface TeamAdoptionChartProps {
  data: TeamAdoption[];
  maxTeams?: number;
}

const TeamAdoptionChart: React.FC<TeamAdoptionChartProps> = ({ data, maxTeams = 15 }) => {
  if (!data || data.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <div className="text-center text-gray-500 py-8">{NO_DATA_MESSAGE}</div>
      </div>
    );
  }

  // Get top teams by number of repos with customizations
  const topTeams = data
    .filter((t) => t.repos_with_customizations > 0)
    .sort((a, b) => b.repos_with_customizations - a.repos_with_customizations)
    .slice(0, maxTeams);

  if (topTeams.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <Heading size="small" level="4" className="mb-4">
          Team med flest tilpasninger
        </Heading>
        <div className="text-center text-gray-500 py-8">Ingen team har AI-tilpasninger ennå</div>
      </div>
    );
  }

  const chartData = {
    labels: topTeams.map((t) => t.team_name || t.team_slug),
    datasets: [
      {
        data: topTeams.map((t) => t.repos_with_customizations),
        backgroundColor: chartColors[1], // green
        borderRadius: 4,
        barThickness: 16,
      },
    ],
  };

  const options = {
    ...commonHorizontalBarOptions,
    plugins: {
      ...commonHorizontalBarOptions.plugins,
      tooltip: {
        ...commonHorizontalBarOptions.plugins.tooltip,
        callbacks: {
          label: (context: TooltipItem<"bar">) => {
            const team = topTeams[context.dataIndex];
            const rate = (team.adoption_rate * 100).toFixed(0);
            return `${context.raw} repo med tilpasninger (${rate}% av ${team.active_repos} aktive)`;
          },
        },
      },
    },
  };

  return (
    <div className={chartWrapperClass}>
      <Heading size="small" level="4" className="mb-4">
        Team med flest tilpasninger
      </Heading>
      <div style={{ height: Math.max(300, topTeams.length * 28) }}>
        <Bar data={chartData} options={options} />
      </div>
    </div>
  );
};

export default TeamAdoptionChart;
