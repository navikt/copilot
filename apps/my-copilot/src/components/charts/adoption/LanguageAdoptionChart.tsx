"use client";

import type { LanguageAdoption } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading } from "@navikt/ds-react";
import { TooltipItem } from "chart.js";

interface LanguageAdoptionChartProps {
  data: LanguageAdoption[];
  maxLanguages?: number;
}

const LanguageAdoptionChart: React.FC<LanguageAdoptionChartProps> = ({ data, maxLanguages = 12 }) => {
  if (!data || data.length === 0) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>
      </Box>
    );
  }

  // Get top languages by adoption rate (only those with customizations)
  const topLanguages = data
    .filter((l) => l.repos_with_customizations > 0)
    .sort((a, b) => b.adoption_rate - a.adoption_rate)
    .slice(0, maxLanguages);

  if (topLanguages.length === 0) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <Heading size="small" level="4">
          Adopsjon etter programmeringsspråk
        </Heading>
        <div className="text-center text-gray-500">Ingen språk har AI-tilpasninger ennå</div>
      </Box>
    );
  }

  const chartData = {
    labels: topLanguages.map((l) => l.language),
    datasets: [
      {
        label: "Adopsjonsrate",
        data: topLanguages.map((l) => l.adoption_rate * 100),
        backgroundColor: chartColors[2], // purple
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
            const lang = topLanguages[context.dataIndex];
            return `${lang.repos_with_customizations} av ${lang.total_repos} repo (${(context.raw as number).toFixed(0)}%)`;
          },
        },
      },
    },
    scales: {
      ...commonHorizontalBarOptions.scales,
      x: {
        ...commonHorizontalBarOptions.scales.x,
        max: 100,
        ticks: {
          ...commonHorizontalBarOptions.scales.x.ticks,
          callback: (value: unknown) => `${value}%`,
        },
      },
    },
  };

  return (
    <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
      <Heading size="small" level="4" spacing>
        Adopsjon etter programmeringsspråk
      </Heading>
      <div style={{ height: Math.max(300, topLanguages.length * 28) }}>
        <Bar data={chartData} options={options} />
      </div>
    </Box>
  );
};

export default LanguageAdoptionChart;
