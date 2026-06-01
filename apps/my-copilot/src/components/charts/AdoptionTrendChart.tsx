"use client";

import type { AdoptionTrendData } from "@/lib/types";
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

export default function AdoptionTrendChart({ data }: AdoptionTrendChartProps) {
  if (!data || !data.days || data.days.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  const chartData = {
    labels: data.days,
    datasets: [
      {
        label: "Chat-brukere",
        data: data.chatUsers,
        borderColor: chartColors[0],
        backgroundColor: getBackgroundColor(chartColors[0]),
        fill: true,
        tension: 0.3,
        pointRadius: 2,
      },
      {
        label: "Agent-brukere",
        data: data.agentUsers,
        borderColor: chartColors[1],
        backgroundColor: getBackgroundColor(chartColors[1]),
        fill: true,
        tension: 0.3,
        pointRadius: 2,
      },
      {
        label: "CLI-brukere",
        data: data.cliUsers,
        borderColor: chartColors[2],
        backgroundColor: getBackgroundColor(chartColors[2]),
        fill: true,
        tension: 0.3,
        pointRadius: 2,
      },
    ],
  };

  return (
    <div className={chartWrapperClass}>
      <Line data={chartData} options={commonLineOptions} />
    </div>
  );
}
