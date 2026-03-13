"use client";

import type { AdoptionTrendData } from "@/lib/types";
import React from "react";
import { Line } from "react-chartjs-2";
import {
  chartColors,
  getBackgroundColor,
  commonLineOptions,
  chartWrapperClass,
  NO_DATA_MESSAGE,
} from "@/lib/chart-utils";

interface AdoptionTrendChartProps {
  data: AdoptionTrendData;
}

const AdoptionTrendChart: React.FC<AdoptionTrendChartProps> = ({ data }) => {
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
        label: "Chat-brukere (30d snitt)",
        data: data.chatUsers,
        borderColor: chartColors[0],
        backgroundColor: getBackgroundColor(chartColors[0]),
        tension: 0.4,
      },
      {
        label: "Agent-brukere (30d snitt)",
        data: data.agentUsers,
        borderColor: chartColors[2],
        backgroundColor: getBackgroundColor(chartColors[2]),
        tension: 0.4,
      },
      {
        label: "CLI-brukere (daglig)",
        data: data.cliUsers,
        borderColor: chartColors[3],
        backgroundColor: getBackgroundColor(chartColors[3]),
        tension: 0.4,
        borderDash: [5, 5],
      },
    ],
  };

  const options = {
    ...commonLineOptions,
    plugins: {
      ...commonLineOptions.plugins,
      title: {
        display: true,
        text: "Adopsjonstrender – Chat, Agent og CLI",
      },
    },
  };

  return (
    <div className={chartWrapperClass}>
      <Line data={chartData} options={options} />
    </div>
  );
};

export default AdoptionTrendChart;
