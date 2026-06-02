"use client";

import type { WeeklyTrend } from "@/lib/types";
import React from "react";
import { Line } from "react-chartjs-2";
import { chartColors, getBackgroundColor, commonLineOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";

interface WeeklyTrendsChartProps {
  data: WeeklyTrend[];
}

const WeeklyTrendsChart: React.FC<WeeklyTrendsChartProps> = ({ data }) => {
  if (!data || data.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  const labels = data.map((d) => d.week);

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
        ...(data.some((d) => d.prompt_tokens + d.output_tokens > 0)
          ? [
              {
                label: "CLI tokens (inn + ut)",
                data: data.map((d) => d.prompt_tokens + d.output_tokens),
                borderColor: chartColors[6 % chartColors.length],
                backgroundColor: getBackgroundColor(chartColors[6 % chartColors.length]),
                tension: 0.4,
                fill: false,
                stack: undefined as string | undefined,
                borderDash: [5, 3],
                yAxisID: "y1",
              },
            ]
          : []),
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
      ];

  const chartData = { labels, datasets };

  const hasCLITokens = hasModels && data.some((d) => d.prompt_tokens + d.output_tokens > 0);

  const options = {
    ...commonLineOptions,
    maintainAspectRatio: true,
    ...(hasModels && {
      scales: {
        ...commonLineOptions.scales,
        y: {
          ...((commonLineOptions.scales as Record<string, unknown>)?.y || {}),
          stacked: true,
        },
        ...(hasCLITokens && {
          y1: {
            position: "right" as const,
            display: true,
            grid: { drawOnChartArea: false },
            title: { display: true, text: "CLI tokens" },
          },
        }),
      },
    }),
  };

  return (
    <div className="aspect-[3/1]">
      <Line data={chartData} options={options} />
    </div>
  );
};

export default WeeklyTrendsChart;
