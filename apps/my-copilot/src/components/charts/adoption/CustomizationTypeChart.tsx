"use client";

import type { AdoptionSummary } from "@/lib/types";
import type { CustomizationType } from "@/lib/adoption-utils";
import React from "react";
import { Bar } from "react-chartjs-2";
import { seriesColor, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { BodyShort, Box, Heading, VStack } from "@navikt/ds-react";
import { extractCustomizationTypes } from "@/lib/adoption-utils";

interface CustomizationTypeChartProps {
  data: AdoptionSummary | null;
}

const groupConfig: Record<string, { title: string; color: () => string }> = {
  copilot: { title: "GitHub Copilot", color: seriesColor(0) },
  agentic: { title: "Agentisk og plattform", color: seriesColor(1) },
  "nav-pilot": { title: "nav-pilot", color: seriesColor(4) },
};

function GroupChart({ title, color, items }: { title: string; color: () => string; items: CustomizationType[] }) {
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
    <Box padding="space-16" borderRadius="8" borderWidth="1" borderColor="neutral-subtle" background="default">
      <Heading size="small" level="3" spacing>
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
      <Box padding="space-16" borderRadius="8" borderWidth="1" borderColor="neutral-subtle" background="default">
        <BodyShort>{NO_DATA_MESSAGE}</BodyShort>
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
