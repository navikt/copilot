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
  /** Current logged-in user's total credits consumed this month, if known. */
  currentUserCredits?: number | null;
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

// Mirrors the bucket boundary logic in copilot-api's bigquery_stats.go
// (getCreditsHistogram) — must stay in sync with the backend SQL CASE statement.
function bucketForCredits(credits: number, budget: number): string {
  if (credits === 0) return "0%";
  if (budget <= 0) return "100%+";
  if (credits < budget * 0.1) return "1-9%";
  if (credits < budget * 0.25) return "10-24%";
  if (credits < budget * 0.5) return "25-49%";
  if (credits < budget * 0.75) return "50-74%";
  if (credits < budget) return "75-99%";
  return "100%+";
}

const UsageDistributionChart: React.FC<UsageDistributionChartProps> = ({ distribution, currentUserCredits }) => {
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

  const buckets = distribution.credits_histogram.map((b) => b.bucket);
  const labels = buckets.map((b) => BUCKET_LABELS[b] ?? b);
  const counts = distribution.credits_histogram.map((b) => b.num_users);
  const totalUsers = distribution.num_users;
  const totalSeats = distribution.total_licensed_seats;
  const budgetUsd = distribution.budget_credits * 0.01;
  const adoptionPct = totalSeats > 0 ? Math.round((totalUsers / totalSeats) * 100) : null;

  const currentUserBucket =
    currentUserCredits != null ? bucketForCredits(currentUserCredits, distribution.budget_credits) : null;
  const currentUserBucketIndex = currentUserBucket ? buckets.indexOf(currentUserBucket) : -1;

  const chartData = {
    labels,
    datasets: [
      {
        label: "Antall brukere",
        data: counts,
        backgroundColor: buckets.map((_, i) =>
          i === currentUserBucketIndex
            ? getBackgroundColor(chartColors[1], 0.7)
            : getBackgroundColor(chartColors[3], 0.5)
        ),
        borderColor: buckets.map((_, i) => (i === currentUserBucketIndex ? chartColors[1] : chartColors[3])),
        borderWidth: buckets.map((_, i) => (i === currentUserBucketIndex ? 2 : 1)),
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
    // false: let the aspect-* CSS class on the wrapping div control the
    // canvas size — Chart.js's own aspectRatio handling (used when this is
    // true) ignores the container's CSS and defaults to a 2:1 ratio.
    maintainAspectRatio: false,
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
            const you = ctx.dataIndex === currentUserBucketIndex ? " — inkluderer deg" : "";
            return ` ${formatNumber(value)} brukere (${pct} %)${you}`;
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
      <BodyShort size="small" className="text-gray-600" style={{ marginBottom: "var(--a-spacing-8)" }}>
        {adoptionPct !== null
          ? `${formatNumber(totalUsers)} av ${formatNumber(totalSeats)} lisenser i bruk (${adoptionPct} % adopsjon).`
          : `${formatNumber(totalUsers)} brukere hadde AI-aktivitet denne måneden.`}{" "}
        Budsjett: ${formatNumber(budgetUsd)}/måned ({formatNumber(distribution.budget_credits)} kreditter). Ingen
        enkeltbrukere vises.
        {currentUserBucket && (
          <>
            {" "}
            Du er i intervallet <strong>{BUCKET_LABELS[currentUserBucket] ?? currentUserBucket}</strong> (uthevet).
          </>
        )}
      </BodyShort>
      <div className="aspect-[12/1]">
        <Bar
          data={chartData as Parameters<typeof Bar>[0]["data"]}
          options={options as Parameters<typeof Bar>[0]["options"]}
        />
      </div>
    </div>
  );
};

export default UsageDistributionChart;
