"use client";

import type { BillingModelBreakdown, BillingMonthlyTrend, BillingModelForecast } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { axColor, bottomLegend, chartBoxClass, seriesColor, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { VStack, BodyShort, Box, HGrid, HStack } from "@navikt/ds-react";
import { formatPercent, formatUSD } from "@/lib/format";

interface BillingModelBreakdownChartProps {
  breakdown: BillingModelBreakdown[];
  trend: BillingMonthlyTrend[];
  forecast?: BillingModelForecast | null;
}

const BillingModelBreakdownChart: React.FC<BillingModelBreakdownChartProps> = ({ breakdown, trend, forecast }) => {
  if (!breakdown || breakdown.length === 0) {
    return <BodyShort>{NO_DATA_MESSAGE}</BodyShort>;
  }

  const today = new Date();
  const currentYearMonth = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}`;

  const months = [...new Set(breakdown.map((d) => d.year_month))].sort();

  // Gross by model + month from breakdown (view now sources from daily table, all months are accurate)
  const grossByModelMonth = new Map<string, Map<string, number>>();
  for (const row of breakdown) {
    if (!grossByModelMonth.has(row.model)) grossByModelMonth.set(row.model, new Map());
    grossByModelMonth.get(row.model)!.set(row.year_month, row.gross_amount);
  }

  // Top models by total gross across all months
  const modelTotals = new Map<string, number>();
  for (const [model, byMonth] of grossByModelMonth) {
    for (const [, gross] of byMonth) {
      modelTotals.set(model, (modelTotals.get(model) ?? 0) + gross);
    }
  }
  const topModels = [...modelTotals.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
    .map(([m]) => m);

  const datasets = topModels.map((model, i) => ({
    label: model,
    data: months.map((m) => Math.round((grossByModelMonth.get(model)?.get(m) ?? 0) * 100) / 100),
    backgroundColor: seriesColor(i, 0.75),
    borderColor: seriesColor(i),
    borderWidth: 1,
    stack: "models",
  }));

  // Net line: trend for completed months, forecast MTD net for current month
  const trendByMonth = new Map(trend.map((t) => [t.year_month, t.total_net_amount]));
  const netLine = {
    label: "Totalt netto (etter rabatt)",
    data: months.map((m) => {
      if (m === currentYearMonth && forecast) {
        return Math.round(forecast.actual_mtd_net_amount * 100) / 100;
      }
      return Math.round((trendByMonth.get(m) ?? 0) * 100) / 100;
    }),
    borderColor: () => axColor("text-neutral"),
    backgroundColor: "transparent",
    borderWidth: 2,
    pointRadius: 3,
    type: "line" as const,
    stack: undefined,
    order: 0,
  };

  // The month-end forecast is in BillingMonthNowChart under «Kostnad denne måneden».

  const allDatasets = [...datasets, netLine];

  const chartData = {
    labels: months.map((m) => {
      const [y, mo] = m.split("-");
      return new Date(Number(y), Number(mo) - 1).toLocaleDateString("nb-NO", { month: "short", year: "2-digit" });
    }),
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    datasets: allDatasets as any[],
  };

  const options = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: bottomLegend,
      tooltip: {
        callbacks: {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          label: (ctx: any) => `${ctx.dataset.label}: ${formatUSD(ctx.parsed.y ?? 0)}`,
        },
      },
    },
    scales: {
      x: { stacked: true, grid: { display: false }, ticks: { color: () => axColor("text-neutral-subtle") } },
      y: {
        stacked: true,
        grid: { color: () => axColor("border-neutral-subtle", 0.4) },
        ticks: {
          color: () => axColor("text-neutral-subtle"),
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          callback: (v: any) => formatUSD(Number(v)),
        },
      },
    },
  };

  // Summary cards — show latest month (including partial current month now that views source from daily table)
  const isCurrentMonthLatest = months[months.length - 1] === currentYearMonth;
  const latestMonth = months[months.length - 1];
  const latestTrend = trend.find((t) => t.year_month === latestMonth);
  const latestLabel = latestMonth
    ? new Date(latestMonth + "-01").toLocaleDateString("nb-NO", { month: "long", year: "numeric" })
    : "";

  const summaryNetAmount =
    isCurrentMonthLatest && forecast ? forecast.actual_mtd_net_amount : (latestTrend?.total_net_amount ?? null);
  const latestMonthGross = breakdown
    .filter((r) => r.year_month === latestMonth)
    .reduce((sum, r) => sum + r.gross_amount, 0);
  const summaryGrossAmount = latestMonthGross > 0 ? latestMonthGross : (latestTrend?.total_gross_amount ?? null);
  const summaryModels = latestTrend?.distinct_models ?? null;

  // Top 3 models in latest month
  const latestTopModels = breakdown
    .filter((r) => r.year_month === latestMonth)
    .sort((a, b) => b.gross_amount - a.gross_amount)
    .slice(0, 3)
    .map((r) => ({
      model: r.model,
      pct: Math.round((r.gross_amount / (latestMonthGross || 1)) * 100),
    }));

  // Estimate current month discount from net/gross ratio (forecast net MTD / gross MTD)
  const summaryDiscountPct =
    isCurrentMonthLatest && forecast && latestMonthGross > 0
      ? Math.round((1 - forecast.actual_mtd_net_amount / latestMonthGross) * 100)
      : latestTrend
        ? Math.round(latestTrend.discount_rate_pct)
        : null;

  return (
    <Box background="neutral-soft" padding="space-24" borderRadius="12">
      <VStack gap="space-16">
        <BodyShort size="small" textColor="subtle">
          Linjen er netto fakturert etter rabatt, og ligger derfor under toppen av søylene. Inneværende måned viser
          tallene hittil.
        </BodyShort>

        {(summaryNetAmount !== null || summaryGrossAmount !== null) && (
          <HGrid columns={{ xs: 2, sm: 4 }} gap="space-12">
            {[
              [
                `${latestLabel}, ${isCurrentMonthLatest ? "netto hittil" : "netto"}`,
                summaryNetAmount !== null ? formatUSD(summaryNetAmount) : "—",
              ],
              [
                isCurrentMonthLatest ? "Brutto hittil" : "Brutto",
                summaryGrossAmount !== null ? formatUSD(summaryGrossAmount) : "—",
              ],
              ["Nav-rabatt", summaryDiscountPct !== null ? formatPercent(summaryDiscountPct) : "—"],
              ["Modeller i bruk", summaryModels ?? "—"],
            ].map(([label, value]) => (
              <Box
                key={label}
                background="default"
                padding="space-12"
                borderRadius="8"
                borderWidth="1"
                borderColor="neutral-subtle"
              >
                <BodyShort size="small" textColor="subtle">
                  {label}
                </BodyShort>
                <BodyShort weight="semibold">{value}</BodyShort>
              </Box>
            ))}
          </HGrid>
        )}

        {latestTopModels.length > 0 && (
          <HStack gap="space-12" wrap>
            {latestTopModels.map(({ model, pct }) => (
              <Box
                key={model}
                background="default"
                padding="space-8"
                borderRadius="8"
                borderWidth="1"
                borderColor="neutral-subtle"
              >
                <BodyShort size="small">
                  <span className="font-medium">{model}</span> {formatPercent(pct)}
                </BodyShort>
              </Box>
            ))}
          </HStack>
        )}

        <div className={chartBoxClass}>
          <Bar data={chartData} options={options} />
        </div>
      </VStack>
    </Box>
  );
};

export default BillingModelBreakdownChart;
