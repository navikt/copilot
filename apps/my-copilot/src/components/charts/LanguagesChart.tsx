"use client";

import type { LanguageChartData } from "@/lib/types";
import React from "react";
import { Line } from "react-chartjs-2";
import { chartColors, getBackgroundColor, commonLineOptions, chartWrapperClass } from "@/lib/chart-utils";

interface LanguagesChartProps {
  data: LanguageChartData;
}

const LanguagesChart: React.FC<LanguagesChartProps> = ({ data }) => {
  if (!data || data.languages.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <div className="text-center text-gray-500 py-8">Ingen språkdata tilgjengelig</div>
      </div>
    );
  }

  const datasets = data.languages.map((lang, index) => ({
    label: lang.name,
    data: lang.values,
    borderColor: chartColors[index % chartColors.length],
    backgroundColor: getBackgroundColor(chartColors[index % chartColors.length]),
    tension: 0.4,
  }));

  const languageChartData = {
    labels: data.days,
    datasets,
  };

  const lineOptions = {
    ...commonLineOptions,
    plugins: {
      ...commonLineOptions.plugins,
      title: {
        display: true,
        text: "Topp programmeringsspråk over tid",
      },
    },
  };

  return (
    <div className={chartWrapperClass}>
      <Line data={languageChartData} options={lineOptions} />
    </div>
  );
};

export default LanguagesChart;
