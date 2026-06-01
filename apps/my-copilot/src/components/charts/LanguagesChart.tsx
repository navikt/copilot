"use client";

import type { LanguageChartData } from "@/lib/types";
import { Line } from "react-chartjs-2";
import {
  chartColors,
  getBackgroundColor,
  commonLineOptions,
  chartWrapperClass,
  NO_DATA_MESSAGE,
} from "@/lib/chart-utils";

interface LanguagesChartProps {
  data: LanguageChartData;
}

export default function LanguagesChart({ data }: LanguagesChartProps) {
  if (!data || !data.days || data.days.length === 0 || !data.languages || data.languages.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  const chartData = {
    labels: data.days,
    datasets: data.languages.map((lang, i) => ({
      label: lang.name,
      data: lang.values,
      borderColor: chartColors[i % chartColors.length],
      backgroundColor: getBackgroundColor(chartColors[i % chartColors.length]),
      fill: false,
      tension: 0.3,
      pointRadius: 2,
    })),
  };

  return (
    <div className={chartWrapperClass}>
      <Line data={chartData} options={commonLineOptions} />
    </div>
  );
}
