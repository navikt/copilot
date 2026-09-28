"use client";

import React from "react";
import { Bar } from "react-chartjs-2";
import { BodyShort, Heading } from "@navikt/ds-react";
import { chartColors, getBackgroundColor } from "@/lib/chart-utils";
import { formatNumber } from "@/lib/format";
import type { UsageDistribution, UsageHistogramBucket } from "@/lib/types";
import type { Chart, Plugin, TooltipItem } from "chart.js";

// "AI" stays in the AI credit wording: AI credits is GitHub's name for the billing unit. Other Norwegian text says KI.

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

// Same threshold as copilot-api's minUsersForDistribution, which suppresses
// bucket counts from 1 to 4.
const MIN_USERS = 5;

export function countText(b: UsageHistogramBucket): string {
  return b.suppressed ? `<${MIN_USERS}` : formatNumber(b.num_users);
}

function monthName(month: string): string {
  const date = new Date(`${month}-01T00:00:00Z`);
  if (Number.isNaN(date.getTime())) return month;
  return new Intl.DateTimeFormat("nb-NO", { month: "long", year: "numeric", timeZone: "UTC" }).format(date);
}

// num_users counts everyone with activity in the month, including people whose
// seat was removed during it; total_licensed_seats is today's seat count. The
// two can't be reconciled, so show both with honest labels and no ratio.
export function populationText(d: UsageDistribution): string {
  const users = `${formatNumber(d.num_users)} brukere med Copilot-aktivitet i ${monthName(d.month)}`;
  return d.total_licensed_seats > 0 ? `${users} · ${formatNumber(d.total_licensed_seats)} lisenser nå.` : `${users}.`;
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

  const histogram = distribution.credits_histogram;
  const buckets = histogram.map((b) => b.bucket);
  const labels = buckets.map((b) => BUCKET_LABELS[b] ?? b);
  // Empty buckets draw no bar (null). Suppressed buckets get a token value so
  // minBarLength gives them a visible stub; labels carry the real text.
  const barValues = histogram.map((b) => (b.suppressed ? 1 : b.num_users || null));
  const totalUsers = distribution.num_users;
  const budgetUsd = distribution.budget_credits * 0.01;

  const currentUserBucket =
    currentUserCredits != null ? bucketForCredits(currentUserCredits, distribution.budget_credits) : null;
  const currentUserBucketIndex = currentUserBucket ? buckets.indexOf(currentUserBucket) : -1;

  const chartData = {
    labels,
    datasets: [
      {
        label: "Antall brukere",
        data: barValues,
        minBarLength: 6,
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

  // Count above every bar, so nobody has to hover, and «Du er her» above the
  // user's own bar even when it is only a stub.
  const barLabels: Plugin<"bar"> = {
    id: "barLabels",
    afterDatasetsDraw(chart: Chart<"bar">) {
      const { ctx, scales } = chart;
      const bars = chart.getDatasetMeta(0).data;
      ctx.save();
      ctx.textAlign = "center";
      ctx.textBaseline = "bottom";
      histogram.forEach((b, i) => {
        const mine = i === currentUserBucketIndex;
        ctx.fillStyle = mine ? "#065F46" : "#374151";
        ctx.font = `${mine ? "bold " : ""}12px sans-serif`;
        // Keep «Du er her» inside the canvas when the last column is narrow.
        const x = Math.min(scales.x.getPixelForValue(i), chart.width - ctx.measureText("Du er her").width / 2 - 2);
        const y = (barValues[i] == null ? scales.y.getPixelForValue(0) : bars[i].y) - 4;
        ctx.fillText(countText(b), x, y);
        if (mine) ctx.fillText("Du er her", x, y - 15);
      });
      ctx.restore();
    },
  };

  const summary = histogram
    .map(
      (b, i) =>
        `${labels[i]}: ${b.suppressed ? `færre enn ${MIN_USERS}` : formatNumber(b.num_users)}${i === currentUserBucketIndex ? " (du er her)" : ""}`
    )
    .join(", ");

  const options = {
    responsive: true,
    // false: let the aspect-* CSS class on the wrapping div control the
    // canvas size — Chart.js's own aspectRatio handling (used when this is
    // true) ignores the container's CSS and defaults to a 2:1 ratio.
    maintainAspectRatio: false,
    // Hovering anywhere in a column shows its tooltip, also for tiny bars.
    interaction: { mode: "index", intersect: false },
    layout: { padding: { top: 36, right: 8 } },
    plugins: {
      legend: { display: false },
      tooltip: {
        backgroundColor: "rgba(0,0,0,0.8)",
        padding: 10,
        cornerRadius: 6,
        callbacks: {
          label: (ctx: TooltipItem<"bar">) => {
            const b = histogram[ctx.dataIndex];
            const you = ctx.dataIndex === currentUserBucketIndex ? ", deg inkludert" : "";
            if (b.suppressed) return ` Færre enn ${MIN_USERS} brukere${you}`;
            const pct = ((b.num_users / totalUsers) * 100).toFixed(0);
            return ` ${formatNumber(b.num_users)} brukere (${pct} %)${you}`;
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
        Fordeling av AI-kredittbruk, {monthName(distribution.month)}
      </Heading>
      <BodyShort size="small" className="text-gray-600" style={{ marginBottom: "var(--a-spacing-8)" }}>
        {populationText(distribution)} Budsjett: ${formatNumber(budgetUsd)}/måned (
        {formatNumber(distribution.budget_credits)} kreditter). Grafen viser bare antall brukere per intervall, og
        intervaller med færre enn {MIN_USERS} brukere vises som «&lt;{MIN_USERS}». Bare du ser hvor du selv ligger.
        {currentUserBucket && (
          <>
            {" "}
            Du er i intervallet <strong>{BUCKET_LABELS[currentUserBucket] ?? currentUserBucket}</strong> (uthevet).
          </>
        )}
      </BodyShort>
      <div className="h-64">
        <Bar
          data={chartData as Parameters<typeof Bar>[0]["data"]}
          options={options as Parameters<typeof Bar>[0]["options"]}
          plugins={[barLabels]}
          role="img"
          aria-label={`Antall brukere per andel av AI-kredittbudsjettet brukt: ${summary}.`}
        />
      </div>
    </div>
  );
};

export default UsageDistributionChart;
