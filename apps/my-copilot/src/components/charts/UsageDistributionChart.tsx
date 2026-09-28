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
// Non-breaking space, so «100 %+» never wraps between number and sign.
const BUCKET_LABELS: Record<string, string> = {
  "0%": "0\u00a0%",
  "1-9%": "1-9\u00a0%",
  "10-24%": "10-24\u00a0%",
  "25-49%": "25-49\u00a0%",
  "50-74%": "50-74\u00a0%",
  "75-99%": "75-99\u00a0%",
  "100%+": "100\u00a0%+",
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
  const users = `${formatNumber(d.num_users)} brukere hadde Copilot-aktivitet i ${monthName(d.month)}.`;
  return d.total_licensed_seats > 0
    ? `${users} Nav har ${formatNumber(d.total_licensed_seats)} lisenser i dag.`
    : users;
}

interface BarLabelOptions {
  texts: string[];
  mine: number;
}

// Count above every bar, so nobody has to hover, and «Du er her» above the
// user's own bar even when it is only a stub. Reads its data from options, so
// it follows prop changes (react-chartjs-2 only registers plugins once).
const barLabels: Plugin<"bar", BarLabelOptions> = {
  id: "barLabels",
  afterDatasetsDraw(chart: Chart<"bar">, _args, { texts, mine }: BarLabelOptions) {
    const { ctx, scales } = chart;
    const bars = chart.getDatasetMeta(0).data;
    ctx.save();
    ctx.textAlign = "center";
    ctx.textBaseline = "bottom";
    texts.forEach((text, i) => {
      const isMine = i === mine;
      ctx.fillStyle = isMine ? "#065F46" : "#374151";
      ctx.font = `${isMine ? "bold " : ""}12px sans-serif`;
      let x = scales.x.getPixelForValue(i);
      // Keep «Du er her» inside the canvas when the last column is narrow.
      if (isMine) x = Math.min(x, chart.width - ctx.measureText("Du er her").width / 2 - 2);
      // Null (empty) bars sit at the base pixel, so this works for them too.
      const y = bars[i].y - 4;
      ctx.fillText(text, x, y);
      if (isMine) ctx.fillText("Du er her", x, y - 15);
    });
    ctx.restore();
  },
};

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

  const summary = histogram
    .map(
      (b, i) =>
        `${labels[i]}: ${b.suppressed ? "færre enn fem" : formatNumber(b.num_users)} brukere${i === currentUserBucketIndex ? ", her er du" : ""}`
    )
    .join(". ");

  const options = {
    responsive: true,
    // false: let the height class on the wrapping div control the canvas
    // size — Chart.js's own aspectRatio handling (used when this is true)
    // ignores the container's CSS and defaults to a 2:1 ratio.
    maintainAspectRatio: false,
    // Hovering anywhere in a column shows its tooltip, also for tiny bars.
    interaction: { mode: "index", intersect: false },
    layout: { padding: { top: 36, right: 8 } },
    plugins: {
      legend: { display: false },
      barLabels: { texts: histogram.map(countText), mine: currentUserBucketIndex },
      tooltip: {
        backgroundColor: "rgba(0,0,0,0.8)",
        padding: 10,
        cornerRadius: 6,
        callbacks: {
          label: (ctx: TooltipItem<"bar">) => {
            const b = histogram[ctx.dataIndex];
            const you = ctx.dataIndex === currentUserBucketIndex ? ", du er en av dem" : "";
            if (b.suppressed) return ` Færre enn fem brukere${you}`;
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
          text: "Andel av eget AI-kredittbudsjett som er brukt",
          color: "#6B7280",
          font: { size: 10 },
        },
        ticks: { color: "#6B7280", font: { size: 11 } },
      },
      y: {
        beginAtZero: true,
        // Keeps «<5» stubs short if every non-empty bucket is suppressed.
        suggestedMax: MIN_USERS,
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
        {populationText(distribution)}{" "}
        {`Hver bruker har ${formatNumber(distribution.budget_credits)} AI-kreditter (${formatNumber(budgetUsd)} dollar) i måneden.`}{" "}
        Søylene viser hvor mange brukere som ligger i hvert intervall. Intervaller med færre enn fem brukere er merket
        &lt;{MIN_USERS}. Bare du ser hvor du selv ligger.
        {currentUserBucket && (
          <>
            {" "}
            Du er i intervallet <strong>{BUCKET_LABELS[currentUserBucket] ?? currentUserBucket}</strong>.
          </>
        )}
      </BodyShort>
      <div className="h-64">
        <Bar
          data={chartData as Parameters<typeof Bar>[0]["data"]}
          options={options as Parameters<typeof Bar>[0]["options"]}
          plugins={[barLabels]}
          role="img"
          aria-label={`Søylediagram: antall brukere per andel av AI-kredittbudsjettet som er brukt. ${summary}.`}
        />
      </div>
    </div>
  );
};

export default UsageDistributionChart;
