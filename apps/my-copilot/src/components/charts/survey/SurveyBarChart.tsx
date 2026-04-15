"use client";

import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions } from "@/lib/chart-utils";
import { Box, Heading } from "@navikt/ds-react";

interface SurveyBarChartProps {
  title: string;
  labels: string[];
  values: number[];
  /** Show percentage labels on bars */
  showPercent?: boolean;
  total?: number;
  height?: number;
  color?: string;
}

export const SurveyBarChart: React.FC<SurveyBarChartProps> = ({
  title,
  labels,
  values,
  showPercent = true,
  total = 163,
  height = 300,
  color = chartColors[0],
}) => {
  if (values.length === 0) {
    return null;
  }

  const safeTotal = total > 0 ? total : 1;
  const maxValue = Math.max(...values) * 1.15;

  const chartData = {
    labels,
    datasets: [
      {
        data: values,
        backgroundColor: color,
        borderRadius: 4,
        barThickness: 22,
      },
    ],
  };

  const options = {
    ...commonHorizontalBarOptions,
    plugins: {
      ...commonHorizontalBarOptions.plugins,
      tooltip: {
        ...commonHorizontalBarOptions.plugins.tooltip,
        callbacks: {
          label: (ctx: { raw: unknown }) => {
            const v = ctx.raw as number;
            return showPercent ? `${v} (${Math.round((v * 100) / safeTotal)} %)` : `${v}`;
          },
        },
      },
    },
    scales: {
      ...commonHorizontalBarOptions.scales,
      x: {
        ...commonHorizontalBarOptions.scales.x,
        max: maxValue,
        ticks: {
          ...commonHorizontalBarOptions.scales.x.ticks,
          callback: (v: string | number) =>
            showPercent ? `${Math.round(((v as number) * 100) / safeTotal)} %` : v,
        },
      },
    },
  };

  return (
    <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
      <Heading size="small" level="3" spacing>
        {title}
      </Heading>
      <div style={{ height }}>
        <Bar data={chartData} options={options} />
      </div>
    </Box>
  );
};
