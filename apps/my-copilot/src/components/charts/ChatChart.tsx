"use client";

import type { FeatureChartData } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { Chart as ChartJS, CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend } from "chart.js";
import { chartWrapperClass, NO_DATA_MESSAGE, chartColors, getBackgroundColor } from "@/lib/chart-utils";
import { formatNumber } from "@/lib/format";

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend);

interface ChatChartProps {
  data: FeatureChartData;
}

const ChatChart: React.FC<ChatChartProps> = ({ data }) => {
  if (!data || data.labels.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <div className="text-center text-gray-500 py-8">{NO_DATA_MESSAGE}</div>
      </div>
    );
  }

  const acceptanceRates = data.generations.map((gen, i) =>
    gen > 0 ? Math.round((data.acceptances[i] / gen) * 100) : 0
  );

  const chartData = {
    labels: data.labels,
    datasets: [
      {
        label: "Aksepteringsrate",
        data: acceptanceRates,
        backgroundColor: acceptanceRates.map((rate) =>
          rate >= 30
            ? getBackgroundColor(chartColors[1], 0.8)
            : rate >= 15
              ? getBackgroundColor(chartColors[3], 0.7)
              : getBackgroundColor(chartColors[4], 0.6)
        ),
        borderColor: acceptanceRates.map((rate) =>
          rate >= 30 ? chartColors[1] : rate >= 15 ? chartColors[3] : chartColors[4]
        ),
        borderWidth: 1,
        borderRadius: 4,
        barPercentage: 0.6,
        categoryPercentage: 0.8,
      },
    ],
  };

  const barOptions = {
    indexAxis: "y" as const,
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { display: false },
      title: {
        display: true,
        text: "Aksepteringsrate per funksjon",
        font: { size: 14 },
      },
      tooltip: {
        backgroundColor: "rgba(0, 0, 0, 0.8)",
        padding: 12,
        cornerRadius: 8,
        callbacks: {
          label: (context: { dataIndex: number; raw: unknown }) => {
            const idx = context.dataIndex;
            return [
              ` Aksepteringsrate: ${context.raw}%`,
              ` Genererte: ${formatNumber(data.generations[idx])}`,
              ` Aksepterte: ${formatNumber(data.acceptances[idx])}`,
            ];
          },
        },
      },
    },
    scales: {
      x: {
        beginAtZero: true,
        max: 100,
        border: { display: false },
        grid: { color: "rgba(0, 0, 0, 0.06)" },
        ticks: {
          color: "#6B7280",
          font: { size: 11 },
          callback: (value: string | number) => `${value}%`,
        },
      },
      y: {
        border: { display: false },
        grid: { display: false },
        ticks: {
          color: "#374151",
          font: { size: 12 },
        },
      },
    },
  };

  const chartHeight = Math.max(250, data.labels.length * 50);

  return (
    <div className={chartWrapperClass}>
      <div style={{ height: chartHeight }}>
        <Bar data={chartData} options={barOptions} />
      </div>
    </div>
  );
};

export default ChatChart;
