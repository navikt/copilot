"use client";

import type { EditorChartData } from "@/lib/types";
import { Line } from "react-chartjs-2";
import {
  chartColors,
  getBackgroundColor,
  commonLineOptions,
  chartWrapperClass,
  NO_DATA_MESSAGE,
} from "@/lib/chart-utils";

interface EditorsChartProps {
  data: EditorChartData;
}

export default function EditorsChart({ data }: EditorsChartProps) {
  if (!data || !data.days || data.days.length === 0 || !data.editors || data.editors.length === 0) {
    return <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>;
  }

  const chartData = {
    labels: data.days,
    datasets: data.editors.map((editor, i) => ({
      label: editor.name,
      data: editor.values,
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
