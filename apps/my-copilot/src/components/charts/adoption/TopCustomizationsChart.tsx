"use client";

import type { CustomizationDetail } from "@/lib/types";
import React from "react";
import { Bar } from "react-chartjs-2";
import { chartColors, commonHorizontalBarOptions, NO_DATA_MESSAGE } from "@/lib/chart-utils";
import { Box, Heading, BodyShort, HGrid } from "@navikt/ds-react";

interface TopCustomizationsChartProps {
  data: CustomizationDetail[];
  maxItems?: number;
}

const categoryLabels: Record<string, string> = {
  agents: "Agenter",
  skills: "Skills",
  instructions: "Instruksjoner",
  prompts: "Prompts",
};

const categoryColors: Record<string, string> = {
  agents: chartColors[0],
  skills: chartColors[1],
  instructions: chartColors[2],
  prompts: chartColors[3],
};

function CategoryChart({
  category,
  items,
  maxItems,
}: {
  category: string;
  items: CustomizationDetail[];
  maxItems: number;
}) {
  const top = items.slice(0, maxItems);
  const color = categoryColors[category] ?? chartColors[5];

  const chartData = {
    labels: top.map((item) => item.file_name),
    datasets: [
      {
        data: top.map((item) => item.repo_count),
        backgroundColor: color,
        borderRadius: 4,
        barThickness: 18,
      },
    ],
  };

  const height = Math.max(150, top.length * 32);

  return (
    <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
      <Heading size="small" level="4" spacing>
        {categoryLabels[category] ?? category}
      </Heading>
      {top.length === 0 ? (
        <BodyShort className="text-gray-500">Ingen data</BodyShort>
      ) : (
        <div style={{ height }}>
          <Bar data={chartData} options={commonHorizontalBarOptions} />
        </div>
      )}
    </Box>
  );
}

const TopCustomizationsChart: React.FC<TopCustomizationsChartProps> = ({ data, maxItems = 10 }) => {
  if (!data || data.length === 0) {
    return (
      <Box padding="space-16" borderRadius="8" className="bg-white border border-gray-200">
        <div className="text-center text-gray-500">{NO_DATA_MESSAGE}</div>
      </Box>
    );
  }

  const grouped = Object.groupBy(data, (item) => item.category);

  const categories = ["agents", "skills", "instructions", "prompts"];

  return (
    <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
      {categories.map((category) => (
        <CategoryChart key={category} category={category} items={grouped[category] ?? []} maxItems={maxItems} />
      ))}
    </HGrid>
  );
};

export default TopCustomizationsChart;
