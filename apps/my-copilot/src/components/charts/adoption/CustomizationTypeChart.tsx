"use client";

import type { AdoptionSummary } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading } from "@navikt/ds-react";

interface CustomizationTypeChartProps {
  data: AdoptionSummary | null;
}

const CustomizationTypeChart: React.FC<CustomizationTypeChartProps> = ({ data }) => {
  if (!data) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>
      </Box>
    );
  }

  const customizationTypes = [
    { label: "copilot-instructions.md", value: data.repos_with_copilot_instructions },
    { label: "AGENTS.md", value: data.repos_with_agents_md },
    { label: ".github/agents/", value: data.repos_with_agents },
    { label: ".github/instructions/", value: data.repos_with_instructions },
    { label: ".github/prompts/", value: data.repos_with_prompts },
    { label: ".github/skills/", value: data.repos_with_skills },
    { label: "mcp.json", value: data.repos_with_mcp_config },
    { label: ".copilot/", value: data.repos_with_copilot_dir },
  ].sort((a, b) => b.value - a.value);

  const chartData = {
    labels: customizationTypes.map((t) => t.label),
    datasets: [
      {
        data: customizationTypes.map((t) => t.value),
        backgroundColor: chartColors[0],
        borderRadius: 4,
        barThickness: 20,
      },
    ],
  };

  return (
    <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
      <Heading size="small" level="4" spacing>
        Copilot-tilpasninger etter type
      </Heading>
      <div style={{ height: 300 }}>
        <Bar data={chartData} options={commonHorizontalBarOptions} />
      </div>
    </Box>
  );
};

export default CustomizationTypeChart;
