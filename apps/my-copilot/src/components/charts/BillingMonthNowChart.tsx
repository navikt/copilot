"use client";

import type { BillingModelDailyCost, BillingModelForecast } from "@/lib/types";
import { chartColors, getBackgroundColor, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { formatNumber } from "@/lib/format";
import { BodyShort, Box, HGrid, VStack } from "@navikt/ds-react";
import React from "react";
import { Bar, Line } from "react-chartjs-2";
import { LinkableHeading } from "@/components/linkable-heading";

interface BillingMonthNowChartProps {
  dailyData: BillingModelDailyCost[];
  forecast: BillingModelForecast | null;
}

const BillingMonthNowChart: React.FC<BillingMonthNowChartProps> = ({ dailyData, forecast }) => {
  if (!dailyData || dailyData.length === 0 || !forecast || !forecast.points?.length) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }
  const monthLabel = new Date(`${forecast.month}-01`).toLocaleDateString("nb-NO", { month: "long", year: "numeric" });

  const normalizeModel = (model: string) => model.replace(/^Auto: /, "");
  const labels = [...new Set(dailyData.map((d) => d.day))].sort();
  const modelTotals = new Map<string, number>();
  const byModelByDay = new Map<string, Map<string, number>>();
  const grossByDay = new Map<string, number>();

  for (const row of dailyData) {
    const model = normalizeModel(row.model);
    modelTotals.set(model, (modelTotals.get(model) || 0) + row.gross_amount);
    if (!byModelByDay.has(model)) byModelByDay.set(model, new Map());
    const dayMap = byModelByDay.get(model)!;
    dayMap.set(row.day, (dayMap.get(row.day) || 0) + row.gross_amount);
    grossByDay.set(row.day, (grossByDay.get(row.day) || 0) + row.gross_amount);
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
    stack: "gross",
  }));

  const daysElapsed = forecast.days_elapsed;
  const daysInMonth = forecast.days_in_month;
  const cumulativeLabels = Array.from({ length: daysInMonth }, (_, index) => String(index + 1).padStart(2, "0"));

  // Use backend's weekday-aware forecast points for the projection line.
  // The backend separates weekday/weekend rates and blends with previous month
  // data early in the billing cycle — this gives a more accurate projection than
  // a simple linear run-rate computed client-side.
  const actualMTDGross = forecast.actual_mtd_net_amount;
  const projectedEOMGross = forecast.projected_eom_net_amount;

  const actual: Array<number | null> = [];
  const projected: number[] = [];
  const bandUpper: number[] = [];
  const bandLower: number[] = [];

  // Compute uncertainty spread per day for the confidence band
  const remainingDays = daysInMonth - daysElapsed;
  const totalSpread = remainingDays > 0 ? forecast.upper_eom_net_amount - forecast.projected_eom_net_amount : 0;
  // Spread grows with sqrt(days) — scale per-day accordingly
  const sqrtTotal = Math.sqrt(Math.max(remainingDays, 1));

  for (const point of forecast.points) {
    if (point.is_actual) {
      actual.push(Number((point.actual_cumulative ?? point.projected_cumulative).toFixed(2)));
      projected.push(Number(point.projected_cumulative.toFixed(2)));
      bandUpper.push(Number(point.projected_cumulative.toFixed(2)));
      bandLower.push(Number(point.projected_cumulative.toFixed(2)));
    } else {
      const dayNum = Number(point.day.slice(8));
      const daysFromActual = dayNum - daysElapsed;
      const spread = totalSpread * (Math.sqrt(daysFromActual) / sqrtTotal);
      actual.push(null);
      projected.push(Number(point.projected_cumulative.toFixed(2)));
      bandUpper.push(Number((point.projected_cumulative + spread).toFixed(2)));
      bandLower.push(Number(Math.max(actualMTDGross, point.projected_cumulative - spread).toFixed(2)));
    }
  }

  return (
    <VStack gap="space-16">
      <LinkableHeading size="small" level="3">
        Måned hittil: modeller og kostnad
      </LinkableHeading>
      <BodyShort size="small" className="text-gray-500">
        Viser {monthLabel}
      </BodyShort>
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-8">
            <BodyShort weight="semibold">Daglig brutto kostnad per modell (USD)</BodyShort>
            <BodyShort size="small" className="text-gray-500">
              Brutto kostnad (før credits/rabatt)
            </BodyShort>
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
              MTD {formatNumber(Math.round(actualMTDGross))} • Prognose {formatNumber(Math.round(projectedEOMGross))} (
              {formatNumber(Math.round(forecast.lower_eom_net_amount))} –{" "}
              {formatNumber(Math.round(forecast.upper_eom_net_amount))})
            </BodyShort>
            <div className="aspect-[2/1]">
              <Line
                data={{
                  labels: cumulativeLabels,
                  datasets: [
                    {
                      label: "Faktisk kumulativ (brutto)",
                      data: actual,
                      borderColor: "#2563eb",
                      backgroundColor: "transparent",
                      borderWidth: 2,
                      pointRadius: 2,
                      spanGaps: false,
                    },
                    {
                      label: "Prognose kumulativ (brutto)",
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
