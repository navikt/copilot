"use client";

import type { EditorChartData } from "@/lib/types";
import React from "react";
import { Line } from "react-chartjs-2";
import { chartColors, getBackgroundColor, commonLineOptions, chartWrapperClass } from "@/lib/chart-utils";

interface EditorsChartProps {
  data: EditorChartData;
}

const EditorsChart: React.FC<EditorsChartProps> = ({ data }) => {
  if (!data || data.editors.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <div className="text-center text-gray-500 py-8">Ingen editordata tilgjengelig</div>
      </div>
    );
  }

  const datasets = data.editors.map((editor, index) => ({
    label: editor.name,
    data: editor.values,
    borderColor: chartColors[index % chartColors.length],
    backgroundColor: getBackgroundColor(chartColors[index % chartColors.length]),
    tension: 0.4,
  }));

  const editorChartData = {
    labels: data.days,
    datasets,
  };

  const lineOptions = {
    ...commonLineOptions,
    plugins: {
      ...commonLineOptions.plugins,
      title: {
        display: true,
        text: "Editorbruk over tid",
      },
    },
  };

  return (
    <div className={chartWrapperClass}>
      <Line data={editorChartData} options={lineOptions} />
    </div>
  );
};

export default EditorsChart;
