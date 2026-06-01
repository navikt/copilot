"use client";

import type { LinesOfCodeChartData } from "@/lib/types";
import { Line } from "react-chartjs-2";
import {
  chartColors,
  getBackgroundColor,
  commonLineOptions,
  chartWrapperClass,
  NO_DATA_MESSAGE,
} from "@/lib/chart-utils";

interface LinesOfCodeChartProps {
  data: LinesOfCodeChartData;
}

export default function LinesOfCodeChart({ data }: LinesOfCodeChartProps) {
  if (!data || !data.days || data.days.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  const chartData = {
    labels: data.days,
    datasets: [
      {
        label: "Foreslåtte linjer",
        data: data.suggested,
        borderColor: chartColors[0],
        backgroundColor: getBackgroundColor(chartColors[0]),
        fill: true,
        tension: 0.3,
        pointRadius: 2,
      },
      {
        label: "Godkjente linjer",
        data: data.accepted,
        borderColor: chartColors[1],
        backgroundColor: getBackgroundColor(chartColors[1]),
        fill: true,
        tension: 0.3,
        pointRadius: 2,
      },
      {
        label: "Foreslåtte slettinger",
        data: data.deletionsSuggested,
        borderColor: chartColors[4],
        backgroundColor: getBackgroundColor(chartColors[4]),
        fill: false,
        tension: 0.3,
        pointRadius: 2,
        borderDash: [4, 4],
      },
      {
        label: "Godkjente slettinger",
        data: data.deletionsAccepted,
        borderColor: chartColors[3],
        backgroundColor: getBackgroundColor(chartColors[3]),
        fill: false,
        tension: 0.3,
        pointRadius: 2,
        borderDash: [4, 4],
      },
    ],
  };

  return (
    <div className={chartWrapperClass}>
      <Line data={chartData} options={commonLineOptions} />
    </div>
  );
}
