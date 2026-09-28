"use client";

import type { LanguageAdoption, AdoptionScope } from "@/lib/types";
import React, { useMemo, useState } from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading, HStack, VStack, ToggleGroup } from "@navikt/ds-react";
import { TooltipItem } from "chart.js";
import { getTopLanguagesForChart, getLanguageAdoptionRate, getLanguageRepoCount } from "@/lib/adoption-utils";

interface LanguageAdoptionChartProps {
  data: LanguageAdoption[];
  maxLanguages?: number;
}

const LanguageAdoptionChart: React.FC<LanguageAdoptionChartProps> = ({ data, maxLanguages = 12 }) => {
  const [scope, setScope] = useState<AdoptionScope>("active");

  const topLanguages = useMemo(
    () => getTopLanguagesForChart(data ?? [], scope, maxLanguages),
    [data, scope, maxLanguages]
  );

  if (!data || data.length === 0) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>
      </Box>
    );
  }

  if (topLanguages.length === 0) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <Heading size="small" level="4">
          Adopsjon etter programmeringsspråk
        </Heading>
        <div className="text-center text-gray-500">Ingen språk har KI-tilpasninger ennå</div>
      </Box>
    );
  }

  const chartData = {
    labels: topLanguages.map((l) => l.language),
    datasets: [
      {
        label: "Adopsjonsrate",
        data: topLanguages.map((l) => getLanguageAdoptionRate(l, scope) * 100),
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
            const repoCount = getLanguageRepoCount(lang, scope);
            const repoLabel = scope === "active" ? "aktive repo" : "repo";
            return `${lang.repos_with_customizations} av ${repoCount} ${repoLabel} (${(context.raw as number).toFixed(0)}%)`;
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
      <VStack gap="space-16">
        <HStack justify="space-between" align="center" gap="space-8" wrap>
          <Heading size="small" level="4">
            Adopsjon etter programmeringsspråk
          </Heading>
          <ToggleGroup size="small" value={scope} onChange={(val) => setScope(val as AdoptionScope)}>
            <ToggleGroup.Item value="active">Aktive repoer</ToggleGroup.Item>
            <ToggleGroup.Item value="all">Alle repoer</ToggleGroup.Item>
          </ToggleGroup>
        </HStack>
        <div style={{ height: Math.max(300, topLanguages.length * 28) }}>
          <Bar data={chartData} options={options} />
        </div>
      </VStack>
    </Box>
  );
};

export default LanguageAdoptionChart;
