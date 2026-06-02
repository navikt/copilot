"use client";

import type { MonthlyModelUsage, MonthlyBillingUsage } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, getBackgroundColor, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { VStack, BodyShort, Box, Heading, HGrid } from "@navikt/ds-react";
import { formatNumber } from "@/lib/format";

interface MonthlyModelChartProps {
  data: MonthlyModelUsage[];
  billingData?: MonthlyBillingUsage[];
}

const MonthlyModelChart: React.FC<MonthlyModelChartProps> = ({ data, billingData }) => {
  if (!data || data.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  // === Billing data (premium requests per model — authoritative) ===
  const hasBillingData = billingData && billingData.length > 0;

  // Filter out "Auto:" prefixed duplicates — group them with manual selections
  const normalizeBillingModel = (model: string) => model.replace(/^Auto: /, "");

  const billingMonths = hasBillingData ? [...new Set(billingData.map((d) => d.month))].sort() : [];
  const billingByModel = new Map<string, Map<string, number>>();
  if (hasBillingData) {
    for (const item of billingData) {
      const model = normalizeBillingModel(item.model);
      if (!billingByModel.has(model)) billingByModel.set(model, new Map());
      const monthMap = billingByModel.get(model)!;
      monthMap.set(item.month, (monthMap.get(item.month) || 0) + item.gross_requests);
    }
  }

  // Top billing models by latest month
  const latestBillingMonth = billingMonths[billingMonths.length - 1];
  const billingTopModels = hasBillingData
    ? [...billingByModel.entries()]
        .map(([model, monthMap]) => ({ model, latest: monthMap.get(latestBillingMonth) || 0 }))
        .sort((a, b) => b.latest - a.latest)
        .slice(0, 8)
        .map((d) => d.model)
    : [];

  const billingDatasets = billingTopModels.map((model, i) => ({
    label: model,
    data: billingMonths.map((month) => Math.round(billingByModel.get(model)?.get(month) || 0)),
    backgroundColor: getBackgroundColor(chartColors[i % chartColors.length], 0.7),
    borderColor: chartColors[i % chartColors.length],
    borderWidth: 1,
  }));

  // Billing summary cards
  const billingLatestData = hasBillingData
    ? [...billingByModel.entries()]
        .map(([model, monthMap]) => ({ model, requests: monthMap.get(latestBillingMonth) || 0 }))
        .filter((d) => d.requests > 0)
        .sort((a, b) => b.requests - a.requests)
    : [];
  const totalBillingRequests = billingLatestData.reduce((s, d) => s + d.requests, 0);

  // === Interaction data (IDE-only, from user_metrics) ===
  const months = [...new Set(data.map((d) => d.month))].sort();
  const latestMonthStr = months[months.length - 1];
  const latestMonthData = data.filter((d) => d.month === latestMonthStr);

  const topModels = latestMonthData
    .sort((a, b) => b.interactions - a.interactions)
    .slice(0, 8)
    .map((d) => d.model);

  // Build stacked bar datasets for interactions
  const interactionDatasets = topModels.map((model, i) => ({
    label: model,
    data: months.map((month) => data.find((d) => d.month === month && d.model === model)?.interactions || 0),
    backgroundColor: getBackgroundColor(chartColors[i % chartColors.length], 0.7),
    borderColor: chartColors[i % chartColors.length],
    borderWidth: 1,
  }));

  // Token data
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

  const latestEntry = latestMonthData[0];
  const totalLatestTokens = latestEntry ? latestEntry.prompt_tokens + latestEntry.output_tokens : 0;

  return (
    <VStack gap="space-16">
      <Heading size="small" level="3">
        AI-modeller over tid
      </Heading>

      {/* Billing data: premium requests per model (authoritative) */}
      {hasBillingData && (
        <>
          <BodyShort size="small" className="text-gray-500">
            Premium-forespørsler per modell — inkluderer all bruk (IDE, CLI, agenter).{" "}
            <a
              href="https://github.com/enterprises/nav/settings/copilot/usage"
              target="_blank"
              rel="noopener noreferrer"
              className="underline"
            >
              GitHub faktureringsdata
            </a>
          </BodyShort>

          <HGrid columns={{ xs: 2, sm: 4 }} gap="space-8">
            {billingLatestData.slice(0, 4).map((d) => (
              <Box key={d.model} background="neutral-soft" padding="space-12" borderRadius="8">
                <div className="text-center">
                  <BodyShort size="small" weight="semibold" className="truncate" title={d.model}>
                    {d.model}
                  </BodyShort>
                  <div className="text-lg font-semibold">
                    {totalBillingRequests > 0 ? Math.round((d.requests / totalBillingRequests) * 100) : 0} %
                  </div>
                  <BodyShort size="small" className="text-gray-500">
                    {formatNumber(Math.round(d.requests))} forespørsler
                  </BodyShort>
                </div>
              </Box>
            ))}
          </HGrid>

          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
            <Box background="neutral-soft" padding="space-16" borderRadius="8">
              <VStack gap="space-8">
                <BodyShort weight="semibold">Premium-forespørsler per modell</BodyShort>
                <div className="aspect-[2/1]">
                  <Bar data={{ labels: billingMonths, datasets: billingDatasets }} options={barOptions} />
                </div>
              </VStack>
            </Box>
            <Box background="neutral-soft" padding="space-16" borderRadius="8">
              <VStack gap="space-8">
                <BodyShort weight="semibold">CLI token-forbruk over tid</BodyShort>
                <BodyShort size="small" className="text-gray-500">
                  Totalt {months[months.length - 1]}: {formatNumber(totalLatestTokens)} tokens
                </BodyShort>
                <div className="aspect-[2/1]">
                  <Bar data={{ labels: months, datasets: tokenBarDatasets }} options={barOptions} />
                </div>
              </VStack>
            </Box>
          </HGrid>
        </>
      )}

      {/* Fallback/supplementary: IDE interaction data */}
      {!hasBillingData && (
        <>
          <BodyShort size="small" className="text-gray-500">
            Aktivitet per modell: interaksjoner + kodeforslag + aksepteringer (kun IDE).{" "}
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
            {latestMonthData
              .sort((a, b) => b.interactions - a.interactions)
              .slice(0, 4)
              .map((d) => {
                const totalLatestInteractions = latestMonthData.reduce((s, r) => s + r.interactions, 0);
                return (
                  <Box key={d.model} background="neutral-soft" padding="space-12" borderRadius="8">
                    <div className="text-center">
                      <BodyShort size="small" weight="semibold" className="truncate" title={d.model}>
                        {d.model}
                      </BodyShort>
                      <div className="text-lg font-semibold">
                        {totalLatestInteractions > 0 ? Math.round((d.interactions / totalLatestInteractions) * 100) : 0}{" "}
                        %
                      </div>
                      <BodyShort size="small" className="text-gray-500">
                        {formatNumber(d.interactions)} aktiviteter
                      </BodyShort>
                    </div>
                  </Box>
                );
              })}
          </HGrid>

          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
            <Box background="neutral-soft" padding="space-16" borderRadius="8">
              <VStack gap="space-8">
                <BodyShort weight="semibold">Aktivitet per modell (IDE)</BodyShort>
                <div className="aspect-[2/1]">
                  <Bar data={{ labels: months, datasets: interactionDatasets }} options={barOptions} />
                </div>
              </VStack>
            </Box>
            <Box background="neutral-soft" padding="space-16" borderRadius="8">
              <VStack gap="space-8">
                <BodyShort weight="semibold">CLI token-forbruk over tid</BodyShort>
                <BodyShort size="small" className="text-gray-500">
                  Totalt {months[months.length - 1]}: {formatNumber(totalLatestTokens)} tokens
                </BodyShort>
                <div className="aspect-[2/1]">
                  <Bar data={{ labels: months, datasets: tokenBarDatasets }} options={barOptions} />
                </div>
              </VStack>
            </Box>
          </HGrid>
        </>
      )}
    </VStack>
  );
};

export default MonthlyModelChart;
