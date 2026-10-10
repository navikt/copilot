"use client";

import type { DailyTrend } from "@/lib/types";
import React from "react";
import { BodyShort } from "@navikt/ds-react";
import { Line } from "react-chartjs-2";
import {
  seriesColor,
  commonLineOptions,
  chartWrapperClass,
  chartBoxClass,
  bottomLegend,
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
        borderColor: seriesColor(0),
        backgroundColor: seriesColor(0, 0.1),
        tension: 0.4,
      },
      {
        label: "Chat (interaksjoner)",
        data: data.map((d) => d.chatUsers),
        borderColor: seriesColor(2),
        backgroundColor: seriesColor(2, 0.1),
        tension: 0.4,
      },
      {
        label: "Agent (genereringer)",
        data: data.map((d) => d.agentUsers),
        borderColor: seriesColor(3),
        backgroundColor: seriesColor(3, 0.1),
        tension: 0.4,
      },
    ],
  };

  const trendOptions = {
    ...commonLineOptions,
    plugins: {
      ...commonLineOptions.plugins,
      legend: bottomLegend,
      title: {
        display: true,
        text: "Daglig aktivitet over tid",
      },
    },
  };

  return (
    <div className={chartWrapperClass}>
      <div className={chartBoxClass}>
        <Line data={trendData} options={trendOptions} />
      </div>
    </div>
  );
};

export default TrendChart;
