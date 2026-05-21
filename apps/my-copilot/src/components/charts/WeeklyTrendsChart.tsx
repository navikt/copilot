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

  const chartData = {
    labels,
    datasets: [
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
    ],
  };

  return (
    <div className="aspect-[3/1]">
      <Line data={chartData} options={{ ...commonLineOptions, maintainAspectRatio: true }} />
    </div>
  );
};

export default WeeklyTrendsChart;
