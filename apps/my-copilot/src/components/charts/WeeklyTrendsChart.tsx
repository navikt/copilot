"use client";

import type { WeeklyTrend } from "@/lib/types";
import React from "react";
import { Line } from "react-chartjs-2";
import { chartColors, getBackgroundColor, commonLineOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";

interface WeeklyTrendsChartProps {
  data: WeeklyTrend[];
  /** Optional map of ISO week label (e.g. "2026-W27") to total AI credits used that week. */
  weeklyCredits?: Record<string, number> | null;
}

const WeeklyTrendsChart: React.FC<WeeklyTrendsChartProps> = ({ data, weeklyCredits }) => {
  if (!data || data.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  const labels = data.map((d) => d.week);

  const hasCredits = !!weeklyCredits && data.some((d) => (weeklyCredits[d.week] ?? 0) > 0);
  const creditsDataset = hasCredits
    ? {
        label: "AI-kreditter brukt",
        data: data.map((d) => weeklyCredits![d.week] ?? 0),
        borderColor: chartColors[5 % chartColors.length],
        backgroundColor: getBackgroundColor(chartColors[5 % chartColors.length]),
        tension: 0.4,
        fill: false,
        stack: undefined as string | undefined,
        borderDash: [5, 3],
        yAxisID: "yCredits",
      }
    : null;

  // Extract unique models across all weeks for stacked view
  const hasModels = data.some((d) => d.models && d.models.length > 0);

  const modelNames = hasModels
    ? [...new Set(data.flatMap((d) => (d.models || []).map((m) => m.model)))]
        .map((model) => ({
          model,
          total: data.reduce((sum, d) => sum + ((d.models || []).find((m) => m.model === model)?.interactions || 0), 0),
        }))
        .sort((a, b) => b.total - a.total)
        .slice(0, 5)
        .map((m) => m.model)
    : [];

  const datasets = hasModels
    ? [
        ...modelNames.map((model, i) => ({
          label: model,
          data: data.map((d) => (d.models || []).find((m) => m.model === model)?.interactions || 0),
          borderColor: chartColors[i % chartColors.length],
          backgroundColor: getBackgroundColor(chartColors[i % chartColors.length]),
          tension: 0.4,
          fill: true,
          stack: "models",
        })),
        ...(creditsDataset ? [creditsDataset] : []),
      ]
    : [
        {
          label: "Forespørsler (IDE + CLI)",
          data: data.map((d) => d.interactions + d.cli_requests),
          borderColor: chartColors[0],
          backgroundColor: getBackgroundColor(chartColors[0]),
          tension: 0.4,
          fill: true,
        },
        {
          label: "Linjer lagt til",
          data: data.map((d) => d.lines_added),
          borderColor: chartColors[1],
          backgroundColor: getBackgroundColor(chartColors[1]),
          tension: 0.4,
        },
        {
          label: "Aksepterte forslag",
          data: data.map((d) => d.acceptances),
          borderColor: chartColors[2],
          backgroundColor: getBackgroundColor(chartColors[2]),
          tension: 0.4,
        },
        ...(creditsDataset ? [creditsDataset] : []),
      ];

  const chartData = { labels, datasets };

  const options = {
    ...commonLineOptions,
    // false: let the aspect-* CSS class on the wrapping div control the
    // canvas size — Chart.js's own aspectRatio handling (used when this is
    // true) ignores the container's CSS and defaults to a 2:1 ratio.
    maintainAspectRatio: false,
    scales: {
      ...commonLineOptions.scales,
      ...(hasModels && {
        y: {
          ...((commonLineOptions.scales as Record<string, unknown>)?.y || {}),
          stacked: true,
        },
      }),
      ...(hasCredits && {
        yCredits: {
          position: "right" as const,
          display: true,
          grid: { drawOnChartArea: false },
          title: { display: true, text: "Kreditter" },
        },
      }),
    },
  };

  return (
    <div className="aspect-[6/1]">
      <Line data={chartData} options={options} />
    </div>
  );
};

export default WeeklyTrendsChart;
