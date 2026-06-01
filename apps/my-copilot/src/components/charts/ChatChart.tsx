"use client";

import type { FeatureChartData } from "@/lib/types";
import { Bar } from "react-chartjs-2";
import { chartColors, getBackgroundColor, chartWrapperClass, NO_DATA_MESSAGE } from "@/lib/chart-utils";

interface ChatChartProps {
  data: FeatureChartData;
}

const options = {
  responsive: true,
  maintainAspectRatio: true,
  interaction: { mode: "index" as const, intersect: false },
  plugins: {
    legend: {
      position: "top" as const,
      labels: { usePointStyle: true, pointStyle: "circle" as const, padding: 20, font: { size: 12 } },
    },
    tooltip: {
      backgroundColor: "rgba(0, 0, 0, 0.8)",
      padding: 12,
      cornerRadius: 8,
    },
  },
  scales: {
    y: {
      beginAtZero: true,
      border: { display: false },
      grid: { color: "rgba(0, 0, 0, 0.06)" },
      ticks: { color: "#6B7280", font: { size: 11 } },
    },
    x: {
      border: { display: false },
      grid: { display: false },
      ticks: { color: "#6B7280", font: { size: 11 } },
    },
  },
};

export default function ChatChart({ data }: ChatChartProps) {
  if (!data || !data.labels || data.labels.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  const chartData = {
    labels: data.labels,
    datasets: [
      {
        label: "Genereringer",
        data: data.generations,
        backgroundColor: getBackgroundColor(chartColors[0], 0.7),
        borderColor: chartColors[0],
        borderWidth: 1,
      },
      {
        label: "Godkjenninger",
        data: data.acceptances,
        backgroundColor: getBackgroundColor(chartColors[1], 0.7),
        borderColor: chartColors[1],
        borderWidth: 1,
      },
      {
        label: "Interaksjoner",
        data: data.interactions,
        backgroundColor: getBackgroundColor(chartColors[2], 0.7),
        borderColor: chartColors[2],
        borderWidth: 1,
      },
    ],
  };

  return (
    <div className={chartWrapperClass}>
      <Bar data={chartData} options={options} />
    </div>
  );
}
