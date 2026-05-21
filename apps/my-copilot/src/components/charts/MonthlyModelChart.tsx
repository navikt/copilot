"use client";

import type { MonthlyModelUsage } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, getBackgroundColor, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { VStack, BodyShort, Box, Heading, HGrid } from "@navikt/ds-react";
import { formatNumber } from "@/lib/format";

interface MonthlyModelChartProps {
  data: MonthlyModelUsage[];
}

const MonthlyModelChart: React.FC<MonthlyModelChartProps> = ({ data }) => {
  if (!data || data.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  // Get unique months and top models (by latest month interactions, falling back to total)
  const months = [...new Set(data.map((d) => d.month))].sort();
  const latestMonthStr = months[months.length - 1];
  const latestMonthData = data.filter((d) => d.month === latestMonthStr);

  // Use latest month ranking to pick which models to show (most relevant now)
  const topModels = latestMonthData
    .sort((a, b) => b.interactions - a.interactions)
    .slice(0, 8)
    .map((d) => d.model);

  // Also compute totals for summary cards
  const modelTotals = new Map<string, { interactions: number; tokens: number }>();
  for (const d of data) {
    const existing = modelTotals.get(d.model) || { interactions: 0, tokens: 0 };
    existing.interactions += d.interactions;
    existing.tokens += d.prompt_tokens + d.output_tokens;
    modelTotals.set(d.model, existing);
  }

  // Build stacked bar datasets for interactions
  const interactionDatasets = topModels.map((model, i) => ({
    label: model,
    data: months.map((month) => data.find((d) => d.month === month && d.model === model)?.interactions || 0),
    backgroundColor: getBackgroundColor(chartColors[i % chartColors.length], 0.7),
    borderColor: chartColors[i % chartColors.length],
    borderWidth: 1,
  }));

  // Build stacked bar datasets for tokens (total CLI tokens per month, not per-model)
  // Token data comes from totals_by_cli and is the same for all models in a month
  const monthlyTokens = months.map((month) => {
    const entry = data.find((d) => d.month === month);
    return {
      prompt: entry?.prompt_tokens || 0,
      output: entry?.output_tokens || 0,
    };
  });

  const tokenBarDatasets = [
    {
      label: "Prompt-tokens",
      data: monthlyTokens.map((t) => t.prompt),
      backgroundColor: getBackgroundColor(chartColors[0], 0.7),
      borderColor: chartColors[0],
      borderWidth: 1,
    },
    {
      label: "Output-tokens",
      data: monthlyTokens.map((t) => t.output),
      backgroundColor: getBackgroundColor(chartColors[1], 0.7),
      borderColor: chartColors[1],
      borderWidth: 1,
    },
  ];

  const tokenBarOptions = {
    responsive: true,
    maintainAspectRatio: true,
    plugins: {
      legend: {
        position: "top" as const,
        labels: { usePointStyle: true, pointStyle: "circle", padding: 12, font: { size: 10 } },
      },
    },
    scales: {
      x: { stacked: true, grid: { display: false } },
      y: { stacked: true, beginAtZero: true, grid: { color: "rgba(0,0,0,0.06)" } },
    },
  };

  const barOptions = {
    responsive: true,
    maintainAspectRatio: true,
    plugins: {
      legend: {
        position: "top" as const,
        labels: { usePointStyle: true, pointStyle: "circle", padding: 12, font: { size: 10 } },
      },
    },
    scales: {
      x: { stacked: true, grid: { display: false } },
      y: { stacked: true, beginAtZero: true, grid: { color: "rgba(0,0,0,0.06)" } },
    },
  };

  // Summary: latest month model distribution
  const latestMonth = months[months.length - 1];
  const latestData = data.filter((d) => d.month === latestMonth);
  const totalLatestInteractions = latestData.reduce((s, d) => s + d.interactions, 0);
  // Tokens are per-month totals (from CLI), take from first entry
  const latestEntry = latestData[0];
  const totalLatestTokens = latestEntry ? latestEntry.prompt_tokens + latestEntry.output_tokens : 0;

  return (
    <VStack gap="space-16">
      <Heading size="small" level="3">
        AI-modeller over tid
      </Heading>
      <BodyShort size="small" className="text-gray-500">
        Aktivitet per modell: interaksjoner + kodeforslag + aksepteringer.{" "}
        <a
          href="https://github.com/enterprises/nav/settings/copilot/usage"
          target="_blank"
          rel="noopener noreferrer"
          className="underline"
        >
          Premium-forespørsler (fakturering)
        </a>
      </BodyShort>

      <HGrid columns={{ xs: 2, sm: 4 }} gap="space-8">
        {latestData
          .sort((a, b) => b.interactions - a.interactions)
          .slice(0, 4)
          .map((d) => (
            <Box key={d.model} background="neutral-soft" padding="space-12" borderRadius="8">
              <div className="text-center">
                <BodyShort size="small" weight="semibold" className="truncate" title={d.model}>
                  {d.model}
                </BodyShort>
                <div className="text-lg font-semibold">
                  {totalLatestInteractions > 0 ? Math.round((d.interactions / totalLatestInteractions) * 100) : 0} %
                </div>
                <BodyShort size="small" className="text-gray-500">
                  {formatNumber(d.interactions)} aktiviteter
                </BodyShort>
              </div>
            </Box>
          ))}
      </HGrid>

      <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-8">
            <BodyShort weight="semibold">Aktivitet per modell</BodyShort>
            <div className="aspect-[2/1]">
              <Bar data={{ labels: months, datasets: interactionDatasets }} options={barOptions} />
            </div>
          </VStack>
        </Box>
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-8">
            <BodyShort weight="semibold">CLI token-forbruk over tid</BodyShort>
            <BodyShort size="small" className="text-gray-500">
              Totalt {latestMonth}: {formatNumber(totalLatestTokens)} tokens
            </BodyShort>
            <div className="aspect-[2/1]">
              <Bar data={{ labels: months, datasets: tokenBarDatasets }} options={tokenBarOptions} />
            </div>
          </VStack>
        </Box>
      </HGrid>
    </VStack>
  );
};

export default MonthlyModelChart;
