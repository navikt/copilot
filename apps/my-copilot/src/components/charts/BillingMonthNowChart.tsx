"use client";

import type { BillingModelDailyCost, BillingModelForecast } from "@/lib/types";
import { chartColors, getBackgroundColor, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { formatNumber } from "@/lib/format";
import { BodyShort, Box, HGrid, Heading, VStack } from "@navikt/ds-react";
import React from "react";
import { Bar, Line } from "react-chartjs-2";

interface BillingMonthNowChartProps {
  dailyData: BillingModelDailyCost[];
  forecast: BillingModelForecast | null;
}

const BillingMonthNowChart: React.FC<BillingMonthNowChartProps> = ({ dailyData, forecast }) => {
  if (!dailyData || dailyData.length === 0 || !forecast) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  const normalizeModel = (model: string) => model.replace(/^Auto: /, "");
  const labels = [...new Set(dailyData.map((d) => d.day))].sort();
  const modelTotals = new Map<string, number>();
  const byModelByDay = new Map<string, Map<string, number>>();

  for (const row of dailyData) {
    const model = normalizeModel(row.model);
    modelTotals.set(model, (modelTotals.get(model) || 0) + row.net_amount);
    if (!byModelByDay.has(model)) byModelByDay.set(model, new Map());
    const dayMap = byModelByDay.get(model)!;
    dayMap.set(row.day, (dayMap.get(row.day) || 0) + row.net_amount);
  }

  const topModels = [...modelTotals.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
    .map(([model]) => model);

  const stackedDatasets = topModels.map((model, index) => ({
    label: model,
    data: labels.map((day) => Number((byModelByDay.get(model)?.get(day) || 0).toFixed(2))),
    backgroundColor: getBackgroundColor(chartColors[index % chartColors.length], 0.7),
    borderColor: chartColors[index % chartColors.length],
    borderWidth: 1,
    stack: "net",
  }));

  const points = forecast.points ?? [];
  const cumulativeLabels = points.map((p) => p.day.slice(8));
  const actual = points.map((p) => p.actual_cumulative ?? null);
  const projected = points.map((p) => Number(p.projected_cumulative.toFixed(2)));

  const bandUpper = points.map((p) => {
    if (p.is_actual) return p.projected_cumulative;
    const step = Math.max(0, Number(p.day.slice(8)) - forecast.days_elapsed);
    const spread =
      forecast.days_in_month > forecast.days_elapsed
        ? ((forecast.upper_eom_net_amount - forecast.projected_eom_net_amount) /
            (forecast.days_in_month - forecast.days_elapsed)) *
          step
        : 0;
    return Number((p.projected_cumulative + spread).toFixed(2));
  });
  const bandLower = points.map((p) => {
    if (p.is_actual) return p.projected_cumulative;
    const step = Math.max(0, Number(p.day.slice(8)) - forecast.days_elapsed);
    const spread =
      forecast.days_in_month > forecast.days_elapsed
        ? ((forecast.projected_eom_net_amount - forecast.lower_eom_net_amount) /
            (forecast.days_in_month - forecast.days_elapsed)) *
          step
        : 0;
    return Number((p.projected_cumulative - spread).toFixed(2));
  });

  return (
    <VStack gap="space-16">
      <Heading size="small" level="3">
        Måned hittil: modeller og kostnad
      </Heading>
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-8">
            <BodyShort weight="semibold">Daglig netto kostnad per modell (USD)</BodyShort>
            <div className="aspect-[2/1]">
              <Bar
                data={{ labels: labels.map((d) => d.slice(8)), datasets: stackedDatasets }}
                options={{
                  responsive: true,
                  maintainAspectRatio: true,
                  plugins: { legend: { position: "top", labels: { boxWidth: 10, font: { size: 10 } } } },
                  scales: {
                    x: { stacked: true, grid: { display: false } },
                    y: { stacked: true, beginAtZero: true, grid: { color: "rgba(0,0,0,0.06)" } },
                  },
                }}
              />
            </div>
          </VStack>
        </Box>
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-8">
            <BodyShort weight="semibold">Prognose månedsslutt (USD)</BodyShort>
            <BodyShort size="small" className="text-gray-500">
              MTD {formatNumber(Math.round(forecast.actual_mtd_net_amount))} • Prognose{" "}
              {formatNumber(Math.round(forecast.projected_eom_net_amount))} (
              {formatNumber(Math.round(forecast.lower_eom_net_amount))} –{" "}
              {formatNumber(Math.round(forecast.upper_eom_net_amount))})
            </BodyShort>
            <div className="aspect-[2/1]">
              <Line
                data={{
                  labels: cumulativeLabels,
                  datasets: [
                    {
                      label: "Faktisk kumulativ",
                      data: actual,
                      borderColor: "#2563eb",
                      backgroundColor: "transparent",
                      borderWidth: 2,
                      pointRadius: 2,
                      spanGaps: false,
                    },
                    {
                      label: "Prognose kumulativ",
                      data: projected,
                      borderColor: "#16a34a",
                      borderDash: [5, 5],
                      backgroundColor: "transparent",
                      borderWidth: 2,
                      pointRadius: 0,
                    },
                    {
                      label: "Øvre estimat",
                      data: bandUpper,
                      borderColor: "rgba(22, 163, 74, 0.2)",
                      backgroundColor: "rgba(22, 163, 74, 0.15)",
                      pointRadius: 0,
                      fill: "+1",
                    },
                    {
                      label: "Nedre estimat",
                      data: bandLower,
                      borderColor: "rgba(22, 163, 74, 0.2)",
                      backgroundColor: "rgba(22, 163, 74, 0.15)",
                      pointRadius: 0,
                      fill: false,
                    },
                  ],
                }}
                options={{
                  responsive: true,
                  maintainAspectRatio: true,
                  plugins: { legend: { position: "top", labels: { boxWidth: 10, font: { size: 10 } } } },
                  scales: {
                    x: { grid: { display: false } },
                    y: { beginAtZero: true, grid: { color: "rgba(0,0,0,0.06)" } },
                  },
                }}
              />
            </div>
          </VStack>
        </Box>
      </HGrid>
    </VStack>
  );
};

export default BillingMonthNowChart;
