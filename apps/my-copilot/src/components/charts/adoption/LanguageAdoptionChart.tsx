"use client";

import type { LanguageAdoption } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { seriesColor, commonHorizontalBarOptions } from "@/lib/chart-utils";
import { BodyShort, Box, Heading, VStack } from "@navikt/ds-react";
import { TooltipItem } from "chart.js";
import { getTopLanguagesForChart, getLanguageAdoptionRate, getLanguageRepoCount } from "@/lib/adoption-utils";
import { formatPercent } from "@/lib/format";

interface LanguageAdoptionChartProps {
  data: LanguageAdoption[];
  maxLanguages?: number;
}

const SCOPES = ["active", "all"] as const;

// Shows the share for active repos and for all repos side by side, sorted by the active share.
const LanguageAdoptionChart: React.FC<LanguageAdoptionChartProps> = ({ data, maxLanguages = 12 }) => {
  const topLanguages = getTopLanguagesForChart(data, "active", maxLanguages);

  if (topLanguages.length === 0) return <BodyShort>Ingen språk har KI-tilpasninger ennå.</BodyShort>;

  const chartData = {
    labels: topLanguages.map((l) => l.language),
    datasets: SCOPES.map((scope, i) => ({
      label: scope === "active" ? "Aktive repoer" : "Alle repoer",
      data: topLanguages.map((l) => getLanguageAdoptionRate(l, scope) * 100),
      backgroundColor: seriesColor(i === 0 ? 2 : 0),
      borderRadius: 4,
      barThickness: 12,
    })),
  };

  const options = {
    ...commonHorizontalBarOptions,
    plugins: {
      ...commonHorizontalBarOptions.plugins,
      legend: { display: true, position: "bottom" as const },
      tooltip: {
        ...commonHorizontalBarOptions.plugins.tooltip,
        callbacks: {
          label: (context: TooltipItem<"bar">) => {
            const lang = topLanguages[context.dataIndex];
            const repos = getLanguageRepoCount(lang, SCOPES[context.datasetIndex]);
            return `${context.dataset.label}: ${formatPercent(context.raw as number)} av ${repos} repoer`;
          },
        },
      },
    },
    scales: {
      ...commonHorizontalBarOptions.scales,
      x: {
        ...commonHorizontalBarOptions.scales.x,
        max: 100,
        ticks: { ...commonHorizontalBarOptions.scales.x.ticks, callback: (v: unknown) => formatPercent(Number(v)) },
      },
    },
  };

  return (
    <Box padding="space-16" borderRadius="8" borderWidth="1" borderColor="neutral-subtle" background="default">
      <VStack gap="space-16">
        <Heading size="small" level="3">
          Andel repoer med tilpasninger per programmeringsspråk
        </Heading>
        <div style={{ height: Math.max(300, topLanguages.length * 40) }}>
          <Bar data={chartData} options={options} />
        </div>
      </VStack>
    </Box>
  );
};

export default LanguageAdoptionChart;
