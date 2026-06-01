"use client";

import type { LanguageData } from "@/lib/types";
import { Doughnut } from "react-chartjs-2";
import { chartColors, commonDonutOptions, chartWrapperClass, NO_DATA_MESSAGE } from "@/lib/chart-utils";

interface LanguageDistributionChartProps {
  data: LanguageData[];
}

export default function LanguageDistributionChart({ data }: LanguageDistributionChartProps) {
  if (!data || data.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  const sorted = [...data].sort((a, b) => b.generations - a.generations).slice(0, 10);

  const chartData = {
    labels: sorted.map((d) => d.name),
    datasets: [
      {
        data: sorted.map((d) => d.generations),
        backgroundColor: sorted.map((_, i) => chartColors[i % chartColors.length].replace("1)", "0.7)")),
        borderColor: sorted.map((_, i) => chartColors[i % chartColors.length]),
        borderWidth: 1,
      },
    ],
  };

  return (
    <div className={chartWrapperClass}>
      <Doughnut data={chartData} options={commonDonutOptions} />
    </div>
  );
}
