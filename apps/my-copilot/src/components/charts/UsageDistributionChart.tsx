"use client";

import React from "react";
import { Bar } from "react-chartjs-2";
import { BodyShort, Heading } from "@navikt/ds-react";
import { chartColors, getBackgroundColor } from "@/lib/chart-utils";
import { formatNumber } from "@/lib/format";
import type { UsageDistribution } from "@/lib/types";
import type { TooltipItem } from "chart.js";

interface UsageDistributionChartProps {
  distribution: UsageDistribution | null;
}

// Same bucket order used server-side (copilot-api bigquery_stats.go getCreditsHistogram).
// Buckets are % of the enterprise per-user AI credit budget (dynamic — see budget_credits).
const BUCKET_LABELS: Record<string, string> = {
  "0%": "0 %",
  "1-9%": "1-9 %",
  "10-24%": "10-24 %",
  "25-49%": "25-49 %",
  "50-74%": "50-74 %",
  "75-99%": "75-99 %",
  "100%+": "100 %+",
};

const UsageDistributionChart: React.FC<UsageDistributionChartProps> = ({ distribution }) => {
  if (!distribution || distribution.num_users === 0) {
    return <BodyShort className="text-gray-500">Ingen fordelingsdata tilgjengelig ennå.</BodyShort>;
  }

  // Privacy guard: mirrors copilot-api's minUsersForDistribution — too few users
  // to aggregate safely means the backend returns an empty histogram. Don't show
  // the exact (small) user count here either — that alone can aid re-identification.
  if (distribution.credits_histogram.length === 0) {
    return (
      <BodyShort className="text-gray-500">For få brukere til å vise en anonymisert fordeling denne måneden.</BodyShort>
    );
  }

  const labels = distribution.credits_histogram.map((b) => BUCKET_LABELS[b.bucket] ?? b.bucket);
  const counts = distribution.credits_histogram.map((b) => b.num_users);
  const totalUsers = distribution.num_users;
  const totalSeats = distribution.total_licensed_seats;
  const budgetUsd = distribution.budget_credits * 0.01;
  const adoptionPct = totalSeats > 0 ? Math.round((totalUsers / totalSeats) * 100) : null;

  const chartData = {
    labels,
    datasets: [
      {
        label: "Antall brukere",
        data: counts,
        backgroundColor: getBackgroundColor(chartColors[3], 0.5),
        borderColor: chartColors[3],
        borderWidth: 1,
        borderRadius: 2,
        // Histogram bars should touch — this isn't a categorical bar chart,
        // it's counts within contiguous credit ranges.
        barPercentage: 1.0,
        categoryPercentage: 0.95,
      },
    ],
  };

  const options = {
    responsive: true,
    maintainAspectRatio: true,
    plugins: {
      legend: { display: false },
      tooltip: {
        backgroundColor: "rgba(0,0,0,0.8)",
        padding: 10,
        cornerRadius: 6,
        callbacks: {
          label: (ctx: TooltipItem<"bar">) => {
            const value = ctx.parsed.y as number;
            const pct = ((value / totalUsers) * 100).toFixed(0);
            return ` ${formatNumber(value)} brukere (${pct} %)`;
          },
        },
      },
    },
    scales: {
      x: {
        grid: { display: false },
        title: {
          display: true,
          text: "Andel av personlig AI-kredittbudsjett brukt",
          color: "#6B7280",
          font: { size: 10 },
        },
        ticks: { color: "#6B7280", font: { size: 11 } },
      },
      y: {
        beginAtZero: true,
        grid: { color: "rgba(0,0,0,0.06)" },
        ticks: { color: "#6B7280", font: { size: 11 }, precision: 0 },
        title: { display: true, text: "Antall brukere", color: "#6B7280", font: { size: 10 } },
      },
    },
  };

  return (
    <div>
      <Heading size="small" level="4" spacing>
        Fordeling av AI-kredittbruk — {distribution.month}
      </Heading>
      <BodyShort size="small" className="text-gray-600" style={{ marginBottom: "var(--a-spacing-4)" }}>
        {formatNumber(totalUsers)} brukere denne måneden
        {adoptionPct !== null ? ` av ${formatNumber(totalSeats)} tildelte lisenser (${adoptionPct} % adopsjon)` : ""}.
      </BodyShort>
      <BodyShort size="small" className="text-gray-600" style={{ marginBottom: "var(--a-spacing-8)" }}>
        Bøttene viser andel av det personlige AI-kredittbudsjettet (${formatNumber(budgetUsd)}/måned,{" "}
        {formatNumber(distribution.budget_credits)} kreditter — 1 kreditt = $0,01). Ingen enkeltbrukere identifiseres,
        kun antall brukere per intervall.
      </BodyShort>
      <div className="aspect-[8/1.5]">
        <Bar
          data={chartData as Parameters<typeof Bar>[0]["data"]}
          options={options as Parameters<typeof Bar>[0]["options"]}
        />
      </div>
    </div>
  );
};

export default UsageDistributionChart;
