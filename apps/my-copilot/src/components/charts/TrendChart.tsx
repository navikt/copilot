"use client";

import type { DailyTrend } from "@/lib/types";
import React from "react";
import { BodyShort } from "@navikt/ds-react";
import { Line } from "react-chartjs-2";
import {
  chartColors,
  getBackgroundColor,
  commonLineOptions,
  chartWrapperClass,
  NO_DATA_MESSAGE,
} from "@/lib/chart-utils";

interface TrendChartProps {
  data: DailyTrend[];
}

const TrendChart: React.FC<TrendChartProps> = ({ data }) => {
  if (!data || data.length === 0) {
    return (
      <div className={chartWrapperClass}>
        <BodyShort>{NO_DATA_MESSAGE}</BodyShort>
      </div>
    );
  }

  const labels = data.map((d) => d.day);

  const trendData = {
    labels,
    datasets: [
      {
        label: "Kodeforslag (genereringer)",
        data: data.map((d) => d.codeCompletionUsers),
        borderColor: chartColors[0],
        backgroundColor: getBackgroundColor(chartColors[0]),
        tension: 0.4,
      },
      {
        label: "Chat (interaksjoner)",
        data: data.map((d) => d.chatUsers),
        borderColor: chartColors[2],
        backgroundColor: getBackgroundColor(chartColors[2]),
        tension: 0.4,
      },
      {
        label: "Agent (genereringer)",
        data: data.map((d) => d.agentUsers),
        borderColor: chartColors[3],
        backgroundColor: getBackgroundColor(chartColors[3]),
        tension: 0.4,
      },
    ],
  };

  const trendOptions = {
    ...commonLineOptions,
    plugins: {
      ...commonLineOptions.plugins,
      title: {
        display: true,
        text: "Daglig aktivitet over tid",
      },
    },
  };

  return (
    <div className={chartWrapperClass}>
      <Line data={trendData} options={trendOptions} />
    </div>
  );
};

export default TrendChart;
