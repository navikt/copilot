"use client";

import type { BillingModelBreakdown, BillingMonthlyTrend, BillingModelForecast } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, getBackgroundColor, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { VStack, BodyShort, Box, HGrid, HelpText } from "@navikt/ds-react";
import { formatNumber } from "@/lib/format";
import { LinkableHeading } from "@/components/linkable-heading";

interface BillingModelBreakdownChartProps {
  breakdown: BillingModelBreakdown[];
  trend: BillingMonthlyTrend[];
  forecast?: BillingModelForecast | null;
}

const BillingModelBreakdownChart: React.FC<BillingModelBreakdownChartProps> = ({ breakdown, trend, forecast }) => {
  if (!breakdown || breakdown.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
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
    backgroundColor: getBackgroundColor(chartColors[i % chartColors.length], 0.75),
    borderColor: chartColors[i % chartColors.length],
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
    borderColor: "#1a1a2e",
    backgroundColor: "transparent",
    borderWidth: 2,
    pointRadius: 3,
    type: "line" as const,
    stack: undefined,
    order: 0,
  };

  // Prognosis removed — see "Prognose månedsslutt (USD)" chart above for month-end forecast

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
    plugins: {
      legend: { position: "bottom" as const, labels: { font: { size: 11 } } },
      tooltip: {
        callbacks: {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          label: (ctx: any) => `${ctx.dataset.label}: ${formatNumber(Math.round(ctx.parsed.y ?? 0))} USD`,
        },
      },
    },
    scales: {
      x: { stacked: true },
      y: {
        stacked: true,
        ticks: {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          callback: (v: any) => `${formatNumber(Math.round(Number(v)))} USD`,
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
        <div className="flex items-center gap-2">
          <LinkableHeading size="small" level="3" id="modellkostnad-historikk">
            Modellkostnad historikk
          </LinkableHeading>
          <HelpText title="Modellkostnad — brutto vs netto" placement="top">
            Søylene viser brutto kostnad per modell per måned (før Nav-rabatt). Netto-linjen viser faktisk fakturert
            beløp etter rabatt — derfor er linjen alltid lavere enn toppen av søylene. Inneværende måned viser
            akkumulert brutto hittil; for prognose månedsslutt, se «Prognose månedsslutt (USD)»-grafen over.
          </HelpText>
        </div>

        <BodyShort size="small" className="text-gray-500">
          Søyler = brutto per modell (før rabatt) · Linje = totalt netto fakturert (etter rabatt)
        </BodyShort>

        {(summaryNetAmount !== null || summaryGrossAmount !== null) && (
          <HGrid columns={{ xs: 2, sm: 4 }} gap="space-12">
            <Box background="default" padding="space-12" borderRadius="8" className="border border-gray-200">
              <BodyShort size="small" className="text-gray-500">
                {latestLabel} — {isCurrentMonthLatest ? "netto hittil" : "netto"}
              </BodyShort>
              <BodyShort weight="semibold">
                {summaryNetAmount !== null ? `${formatNumber(Math.round(summaryNetAmount))} USD` : "—"}
              </BodyShort>
            </Box>
            <Box background="default" padding="space-12" borderRadius="8" className="border border-gray-200">
              <BodyShort size="small" className="text-gray-500">
                {isCurrentMonthLatest ? "Brutto hittil" : "Brutto"}
              </BodyShort>
              <BodyShort weight="semibold">
                {summaryGrossAmount !== null ? `${formatNumber(Math.round(summaryGrossAmount))} USD` : "—"}
              </BodyShort>
            </Box>
            <Box background="default" padding="space-12" borderRadius="8" className="border border-gray-200">
              <BodyShort size="small" className="text-gray-500">
                Nav-rabatt
              </BodyShort>
              <BodyShort weight="semibold">{summaryDiscountPct !== null ? `${summaryDiscountPct} %` : "—"}</BodyShort>
            </Box>
            <Box background="default" padding="space-12" borderRadius="8" className="border border-gray-200">
              <BodyShort size="small" className="text-gray-500">
                Modeller i bruk
              </BodyShort>
              <BodyShort weight="semibold">{summaryModels ?? "—"}</BodyShort>
            </Box>
          </HGrid>
        )}

        {latestTopModels.length > 0 && (
          <div className="flex flex-wrap gap-3">
            {latestTopModels.map(({ model, pct }) => (
              <Box
                key={model}
                background="default"
                padding="space-8"
                borderRadius="8"
                className="border border-gray-200 text-sm"
              >
                <span className="font-medium">{model}</span>
                <span className="ml-2 text-gray-500">{pct} %</span>
              </Box>
            ))}
          </div>
        )}

        <Bar data={chartData} options={options} />
      </VStack>
    </Box>
  );
};

export default BillingModelBreakdownChart;
