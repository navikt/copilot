"use client";

import type { AdoptionSummary } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading } from "@navikt/ds-react";
import { extractToolComparison } from "@/lib/adoption-utils";

interface ToolComparisonChartProps {
  data: AdoptionSummary | null;
}

const toolColors: Record<string, string> = {
  "Kun Copilot": chartColors[0],
  Cursor: chartColors[2],
  Claude: chartColors[3],
  Windsurf: chartColors[5],
};

const ToolComparisonChart: React.FC<ToolComparisonChartProps> = ({ data }) => {
  if (!data) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>
      </Box>
    );
  }

  const tools = extractToolComparison(data);

  if (tools.length === 0) {
    return null;
  }

  const chartData = {
    labels: tools.map((t) => t.label),
    datasets: [
      {
        data: tools.map((t) => t.value),
        backgroundColor: tools.map((t) => toolColors[t.label] ?? chartColors[4]),
        borderRadius: 4,
        barThickness: 24,
      },
    ],
  };

  const height = Math.max(120, tools.length * 40);

  return (
    <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
      <Heading size="small" level="4" spacing>
        AI-verktøy i bruk
      </Heading>
      <div style={{ height }}>
        <Bar data={chartData} options={commonHorizontalBarOptions} />
      </div>
    </Box>
  );
};

export default ToolComparisonChart;
