"use client";

import type { GenerationModeTrendData } from "@/lib/types";
import React from "react";
import { Line } from "react-chartjs-2";
import {
  chartColors,
  getBackgroundColor,
  commonLineOptions,
  chartWrapperClass,
  NO_DATA_MESSAGE,
} from "@/lib/chart-utils";
import { formatNumber } from "@/lib/format";

interface GenerationModeChartProps {
  data: GenerationModeTrendData;
}

const GenerationModeChart: React.FC<GenerationModeChartProps> = ({ data }) => {
  if (!data || data.days.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <div className="text-center text-gray-500 py-8">{NO_DATA_MESSAGE}</div>
      </div>
    );
  }

  const chartData = {
    labels: data.days,
    datasets: [
      {
        label: "Bruker",
        data: data.userInitiated,
        borderColor: chartColors[0],
        backgroundColor: getBackgroundColor(chartColors[0]),
        fill: true,
        tension: 0.4,
      },
      {
        label: "Agent",
        data: data.agentInitiated,
        borderColor: chartColors[2],
        backgroundColor: getBackgroundColor(chartColors[2]),
        fill: true,
        tension: 0.4,
      },
    ],
  };

  const options = {
    ...commonLineOptions,
    plugins: {
      ...commonLineOptions.plugins,
      title: {
        display: true,
        text: "Genereringer — bruker vs. agent",
      },
      tooltip: {
        ...commonLineOptions.plugins.tooltip,
        callbacks: {
          label: (context: { dataset: { label?: string }; raw: unknown }) => {
            return ` ${context.dataset.label}: ${formatNumber(context.raw as number)}`;
          },
        },
      },
    },
  };

  return (
    <div className={chartWrapperClass}>
      <Line data={chartData} options={options} />
    </div>
  );
};

export default GenerationModeChart;
