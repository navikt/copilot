"use client";

import type { AdoptionSummary } from "@/lib/types";
import type { CustomizationType } from "@/lib/adoption-utils";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading, VStack } from "@navikt/ds-react";
import { extractCustomizationTypes } from "@/lib/adoption-utils";

interface CustomizationTypeChartProps {
  data: AdoptionSummary | null;
}

const groupConfig: Record<string, { title: string; color: string }> = {
  copilot: { title: "GitHub Copilot", color: chartColors[0] },
  agentic: { title: "Agentic & plattform", color: chartColors[1] },
  "nav-pilot": { title: "nav-pilot", color: chartColors[4] },
};

function GroupChart({ title, color, items }: { title: string; color: string; items: CustomizationType[] }) {
  const sorted = [...items].sort((a, b) => b.value - a.value);

  const chartData = {
    labels: sorted.map((t) => t.label),
    datasets: [
      {
        data: sorted.map((t) => t.value),
        backgroundColor: color,
        borderRadius: 4,
        barThickness: 20,
      },
    ],
  };

  const height = Math.max(120, sorted.length * 36);

  return (
    <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
      <Heading size="small" level="4" spacing>
        {title}
      </Heading>
      <div style={{ height }}>
        <Bar data={chartData} options={commonHorizontalBarOptions} />
      </div>
    </Box>
  );
}

const CustomizationTypeChart: React.FC<CustomizationTypeChartProps> = ({ data }) => {
  if (!data) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>
      </Box>
    );
  }

  const allTypes = extractCustomizationTypes(data);

  const groups = Object.entries(groupConfig)
    .map(([key, config]) => ({
      ...config,
      items: allTypes.filter((t) => t.group === key),
    }))
    .filter((g) => g.items.some((i) => i.value > 0));

  return (
    <VStack gap="space-16">
      {groups.map((group) => (
        <GroupChart key={group.title} title={group.title} color={group.color} items={group.items} />
      ))}
    </VStack>
  );
};

export default CustomizationTypeChart;
