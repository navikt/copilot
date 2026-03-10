"use client";

import type { ModelChartData } from "@/lib/types";
import React from "react";
import { Doughnut } from "react-chartjs-2";
import { chartColors, commonDonutOptions, chartWrapperClass, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Heading } from "@navikt/ds-react";
import { TooltipItem } from "chart.js";

interface ModelUsageChartProps {
  data: ModelChartData[];
}

const ModelUsageChart: React.FC<ModelUsageChartProps> = ({ data }) => {
  if (!data || data.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <div className="text-center text-gray-500 py-8">{NO_DATA_MESSAGE}</div>
      </div>
    );
  }

  const total = data.reduce((sum, m) => sum + m.generations, 0);

  const chartData = {
    labels: data.map((m) => m.name),
    datasets: [
      {
        data: data.map((m) => m.generations),
        backgroundColor: data.map((_, i) => chartColors[i % chartColors.length]),
        borderColor: data.map((_, i) => chartColors[i % chartColors.length]),
        borderWidth: 0,
        hoverOffset: 4,
      },
    ],
  };

  const options = {
    ...commonDonutOptions,
    plugins: {
      ...commonDonutOptions.plugins,
      tooltip: {
        ...commonDonutOptions.plugins.tooltip,
        callbacks: {
          label: (context: TooltipItem<"doughnut">) => {
            const value = context.raw as number;
            const percentage = ((value / total) * 100).toFixed(1);
            return `${context.label}: ${value} genereringer (${percentage}%)`;
          },
        },
      },
    },
  };

  return (
    <div className={chartWrapperClass}>
      <Heading size="small" level="4" className="mb-4">
        Modellbruk
      </Heading>
      <div className="max-w-md mx-auto">
        <Doughnut data={chartData} options={options} />
      </div>
    </div>
  );
};

export default ModelUsageChart;
