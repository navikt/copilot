"use client";

import type { LinesOfCodeChartData } from "@/lib/types";
import React, { useRef, useEffect } from "react";
import { Line } from "react-chartjs-2";
import { Chart as ChartJS } from "chart.js";
import { chartColors, commonLineOptions, chartWrapperClass, NO_DATA_MESSAGE, createGradient } from "@/lib/chart-utils";
import { Heading } from "@navikt/ds-react";

interface LinesOfCodeChartProps {
  data: LinesOfCodeChartData;
}

const LinesOfCodeChart: React.FC<LinesOfCodeChartProps> = ({ data }) => {
  const chartRef = useRef<ChartJS<"line">>(null);
  const hasGradientsRef = useRef(false);

  useEffect(() => {
    const chart = chartRef.current;
    if (!chart || hasGradientsRef.current || !data || data.days.length === 0) return;

    const ctx = chart.ctx;
    chart.data.datasets[0].backgroundColor = createGradient(ctx, chartColors[0]);
    chart.data.datasets[1].backgroundColor = createGradient(ctx, chartColors[1]);
    chart.data.datasets[2].backgroundColor = createGradient(ctx, chartColors[3]);
    chart.data.datasets[3].backgroundColor = createGradient(ctx, chartColors[4]);
    chart.update();
    hasGradientsRef.current = true;
  });

  if (!data || data.days.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <div className="text-center text-gray-500 py-8">{NO_DATA_MESSAGE}</div>
      </div>
    );
  }

  const chartData = {
    labels: data.days,
    datasets: [
      {
        label: "Foreslåtte linjer (lagt til)",
        data: data.suggested,
        borderColor: chartColors[0],
        backgroundColor: chartColors[0].replace("1)", "0.1)"),
        fill: true,
        tension: 0.4,
      },
      {
        label: "Aksepterte linjer (lagt til)",
        data: data.accepted,
        borderColor: chartColors[1],
        backgroundColor: chartColors[1].replace("1)", "0.1)"),
        fill: true,
        tension: 0.4,
      },
      {
        label: "Foreslåtte linjer (slettet)",
        data: data.deletionsSuggested,
        borderColor: chartColors[3],
        backgroundColor: chartColors[3].replace("1)", "0.1)"),
        fill: true,
        tension: 0.4,
        borderDash: [5, 5],
      },
      {
        label: "Aksepterte linjer (slettet)",
        data: data.deletionsAccepted,
        borderColor: chartColors[4],
        backgroundColor: chartColors[4].replace("1)", "0.1)"),
        fill: true,
        tension: 0.4,
        borderDash: [5, 5],
      },
    ],
  };

  const options = {
    ...commonLineOptions,
    plugins: {
      ...commonLineOptions.plugins,
      title: {
        display: true,
        text: "Kodelinjer over tid",
        font: { size: 14 },
      },
    },
  };

  return (
    <div className={chartWrapperClass}>
      <Heading size="small" level="4" className="mb-4">
        Kodelinjer foreslått vs akseptert
      </Heading>
      <Line ref={chartRef} data={chartData} options={options} />
    </div>
  );
};

export default LinesOfCodeChart;
